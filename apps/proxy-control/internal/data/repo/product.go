package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/data/repo/model"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errProductUnavailable = kerrors.ServiceUnavailable("PRODUCT_UNAVAILABLE", "product catalog unavailable")

type ProductRepository struct{ db *gorm.DB }

func ProvideProductRepository(resources *data.Data) *ProductRepository {
	db := resources.UsageDB()
	if db == nil {
		db = resources.ProxyAuthDB()
	}
	return NewProductRepository(db)
}

func NewProductRepository(db *gorm.DB) *ProductRepository { return &ProductRepository{db: db} }

func (r *ProductRepository) Catalog(ctx context.Context) ([]product.Sale, []product.Package, error) {
	if r.db == nil {
		return nil, nil, errProductUnavailable
	}
	var sales []product.Sale
	var packages []product.Package
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []model.ProductSale
		if err := tx.Order("kind").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 2 {
			return errProductUnavailable
		}
		for _, row := range rows {
			sales = append(sales, product.Sale(row))
		}
		var err error
		packages, err = listTrafficPackages(tx)
		return err
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return sales, packages, err
}

func (r *ProductRepository) Packages(ctx context.Context) ([]product.Package, error) {
	if r.db == nil {
		return nil, errProductUnavailable
	}
	return listTrafficPackages(r.db.WithContext(ctx))
}

func (r *ProductRepository) Changes(ctx context.Context) ([]product.Change, error) {
	if r.db == nil {
		return nil, errProductUnavailable
	}
	var rows []struct {
		ID               int64
		TargetKind       string
		TargetID         string
		OperatorUsername string
		CreatedAt        time.Time
		BeforeValue      []byte
		AfterValue       []byte
	}
	err := r.db.WithContext(ctx).Raw(`SELECT changes.id, changes.target_kind, changes.target_id, changes.created_at,
		operators.username AS operator_username, changes.before_value, changes.after_value
		FROM product_change_log AS changes JOIN operators ON operators.id = changes.operator_id
		ORDER BY changes.id DESC LIMIT 50`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	changes := make([]product.Change, 0, len(rows))
	for _, row := range rows {
		change := product.Change{
			ID: row.ID, TargetKind: row.TargetKind, TargetID: row.TargetID,
			OperatorUsername: row.OperatorUsername, CreatedAt: row.CreatedAt,
		}
		if err := json.Unmarshal(row.BeforeValue, &change.Before); err != nil {
			return nil, fmt.Errorf("decode old product values: %w", err)
		}
		if err := json.Unmarshal(row.AfterValue, &change.After); err != nil {
			return nil, fmt.Errorf("decode new product values: %w", err)
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func listTrafficPackages(db *gorm.DB) ([]product.Package, error) {
	var rows []model.TrafficPackage
	if err := db.Order("capacity_bytes, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	packages := make([]product.Package, 0, len(rows))
	for _, row := range rows {
		packages = append(packages, packageFromModel(row))
	}
	return packages, nil
}

func (r *ProductRepository) UpdatePackage(ctx context.Context, actor string, next product.Package) (product.Package, error) {
	if r.db == nil {
		return product.Package{}, errProductUnavailable
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the sale row first, as in checkout and sale updates.
		if _, err := readSale(tx, product.Dynamic, "UPDATE"); err != nil {
			return err
		}
		before, err := readTrafficPackage(tx, next.ID)
		if err != nil {
			return err
		}
		if err := requirePackageVersion(before, next.Version); err != nil {
			return err
		}
		if next == before {
			return nil
		}
		next.Version++
		if err := savePackage(tx, actor, before, next); err != nil {
			return err
		}
		if next.ID == product.BaselinePackageID && before.PriceMinor == 0 && next.PriceMinor > 0 {
			return priceDefaultTiers(tx, actor, next)
		}
		return nil
	})
	return next, err
}

func priceDefaultTiers(tx *gorm.DB, actor string, base product.Package) error {
	packages, err := listTrafficPackages(tx)
	if err != nil {
		return err
	}
	for _, before := range packages {
		price, err := before.DefaultTierPrice(base)
		if err != nil {
			return err
		}
		if price == 0 {
			continue
		}
		next := before
		next.PriceMinor = price
		next.Version++
		if err := savePackage(tx, actor, before, next); err != nil {
			return err
		}
	}
	return nil
}

func savePackage(tx *gorm.DB, actor string, before, next product.Package) error {
	err := tx.Table(model.TableNameTrafficPackage).Where("id = ?", next.ID).
		Updates(map[string]any{"capacity_bytes": next.CapacityBytes, "price_minor": next.PriceMinor, "listed": next.Listed, "version": next.Version}).Error
	if err != nil {
		return err
	}
	return logProductChange(tx, "package", next.ID, actor, before, next)
}

func (r *ProductRepository) SetSale(ctx context.Context, actor string, next product.Sale) (product.Sale, []product.Package, error) {
	if r.db == nil {
		return product.Sale{}, nil, errProductUnavailable
	}
	var packages []product.Package
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		before, err := readSale(tx, next.Kind, "UPDATE")
		if err != nil {
			return err
		}
		if next.Kind == product.Dynamic && next.Open {
			packages, err = listTrafficPackages(tx)
			if err != nil {
				return err
			}
			if !product.DynamicQuoteReady(packages) {
				return product.ErrQuoteNotReady
			}
		}
		if next == before {
			return nil
		}
		err = tx.Table(model.TableNameProductSale).Where("kind = ?", next.Kind).
			Updates(map[string]any{"open": next.Open, "quote_ready": next.QuoteReady, "acceptance_evidence": next.AcceptanceEvidence}).Error
		if err != nil {
			return err
		}
		return logProductChange(tx, "sale", next.Kind, actor, before, next)
	})
	return next, packages, err
}

func readSale(tx *gorm.DB, kind, strength string) (product.Sale, error) {
	var row model.ProductSale
	err := tx.Clauses(clause.Locking{Strength: strength}).Where("kind = ?", kind).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = errProductUnavailable
	}
	return product.Sale(row), err
}

func readTrafficPackage(tx *gorm.DB, id string) (product.Package, error) {
	var row model.TrafficPackage
	err := tx.Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = product.ErrNotFound
	}
	if err != nil {
		return product.Package{}, err
	}
	return packageFromModel(row), nil
}

func packageFromModel(row model.TrafficPackage) product.Package {
	return product.Package{
		ID: row.ID, CapacityBytes: row.CapacityBytes, PriceMinor: row.PriceMinor,
		Listed: *row.Listed, Version: *row.Version,
	}
}

// RequireProductSale holds the shared sale lock until the caller's checkout transaction ends.
func RequireProductSale(ctx context.Context, tx *gorm.DB, kind string) error {
	sale, err := readSale(tx.WithContext(ctx), kind, "SHARE")
	if err != nil {
		return err
	}
	if !sale.Open {
		return product.ErrSaleClosed
	}
	return nil
}

// LockTrafficPackage returns the confirmed purchase snapshot under the dynamic sale lock.
// Call it inside the checkout transaction before changing the wallet or granting traffic.
func LockTrafficPackage(ctx context.Context, tx *gorm.DB, id string, version int64) (product.Package, error) {
	if err := RequireProductSale(ctx, tx, product.Dynamic); err != nil {
		return product.Package{}, err
	}
	pkg, err := readTrafficPackage(tx.WithContext(ctx), id)
	if err != nil {
		return product.Package{}, err
	}
	if !pkg.Available() {
		return product.Package{}, product.ErrPackageUnavailable
	}
	return pkg, requirePackageVersion(pkg, version)
}

func requirePackageVersion(pkg product.Package, version int64) error {
	if pkg.Version == version {
		return nil
	}
	return product.ErrPackageChanged.WithMetadata(map[string]string{
		"id":             pkg.ID,
		"version":        strconv.FormatInt(pkg.Version, 10),
		"capacity_bytes": strconv.FormatInt(pkg.CapacityBytes, 10),
		"price_minor":    strconv.FormatInt(pkg.PriceMinor, 10),
		"listed":         strconv.FormatBool(pkg.Listed),
	})
}

func logProductChange(tx *gorm.DB, kind, id, actor string, before, after any) error {
	oldValue, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("encode old product values: %w", err)
	}
	newValue, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode new product values: %w", err)
	}
	return tx.Exec(`INSERT INTO product_change_log (target_kind, target_id, operator_id, before_value, after_value)
		VALUES (?, ?, ?, ?::jsonb, ?::jsonb)`, kind, id, actor, string(oldValue), string(newValue)).Error
}
