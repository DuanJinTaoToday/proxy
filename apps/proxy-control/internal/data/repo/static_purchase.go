package repo

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"math"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func NewWalletRepositoryWithStatic(db *gorm.DB, supplier wallet.StaticSupplier, passwordKey string) (*WalletRepository, error) {
	repository := NewWalletRepository(db)
	if supplier == nil && passwordKey == "" {
		return repository, nil
	}
	key, err := base64.RawURLEncoding.DecodeString(passwordKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("ipcrown.password_key must be a base64url-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	repository.staticCipher, err = cipher.NewGCMWithRandomNonce(block)
	repository.static = supplier
	return repository, err
}

type storedStaticIP struct {
	wallet.StaticAllocation
	PasswordCiphertext []byte
}

func staticPasswordScope(tenant, item string) []byte {
	return []byte(tenant + ":" + item)
}

func (r *WalletRepository) StaticOffers(ctx context.Context) ([]wallet.StaticOffer, error) {
	if r.db == nil {
		return nil, errWalletUnavailable
	}
	if err := RequireProductSale(ctx, r.db, product.Static); err != nil {
		return nil, err
	}
	if r.static == nil {
		return nil, wallet.ErrStaticUnavailable
	}
	offers, err := r.static.Offers(ctx)
	if err != nil {
		return nil, wallet.ErrStaticUnavailable
	}
	if err := RequireProductSale(ctx, r.db, product.Static); err != nil {
		return nil, err
	}
	return offers, nil
}

func (r *WalletRepository) PurchaseStatic(ctx context.Context, tenant string, p wallet.StaticPurchaseRequest) (wallet.Order, error) {
	if r.db == nil {
		return wallet.Order{}, errWalletUnavailable
	}
	fingerprint, err := walletFingerprint(p)
	if err != nil {
		return wallet.Order{}, err
	}
	previous, exists, err := walletReplay(r.db.WithContext(ctx), tenant, p.Key, fingerprint)
	if err != nil {
		return wallet.Order{}, err
	}
	if exists {
		return r.Order(ctx, tenant, previous.OrderID)
	}

	offers, err := r.StaticOffers(ctx)
	if err != nil {
		return wallet.Order{}, err
	}
	var selected wallet.StaticOffer
	for _, offer := range offers {
		if offer.NodeID == p.NodeID && offer.Term == p.Term {
			selected = offer
			break
		}
	}
	if selected.NodeID == 0 {
		return wallet.Order{}, wallet.ErrStaticOfferMissing
	}
	if err := p.CheckOffer(selected); err != nil {
		return wallet.Order{}, err
	}
	if p.UnitMinor > math.MaxInt64/int64(p.Quantity) {
		return wallet.Order{}, wallet.ErrInvalid
	}
	command := wallet.Command{
		TenantID: tenant, Key: p.Key, Action: wallet.Hold, Kind: wallet.StaticPurchase,
		Amount: p.UnitMinor * int64(p.Quantity),
		Items: []wallet.OrderItem{{
			Reference: fmt.Sprintf("%d/%s", p.NodeID, p.Term), Specification: selected.Region + " · " + selected.IPType,
			TermDays: selected.TermDays, UnitMinor: p.UnitMinor, Quantity: p.Quantity,
		}},
	}
	created := false
	entry, err := r.apply(ctx, command, p, func(tx *gorm.DB, _ *wallet.Command) error {
		return RequireProductSale(ctx, tx, product.Static)
	}, func(tx *gorm.DB, e wallet.Entry) error {
		created = true
		return tx.Exec("INSERT INTO static_purchases (tenant_id, order_id) VALUES (?, ?)", tenant, e.OrderID).Error
	})
	if err != nil {
		return wallet.Order{}, err
	}
	if !created {
		return r.Order(ctx, tenant, entry.OrderID)
	}

	// After commit the request key can only read the order. An ambiguous
	// supplier response is reconciled by operations without resubmitting.
	process, cancel := context.WithTimeout(context.WithoutCancel(ctx), 25*time.Second)
	defer cancel()
	delivery, err := r.static.Place(process, p)
	if err == nil {
		if err := r.recordStaticDelivery(process, tenant, entry.OrderID, delivery, "", "", delivery.Rejected, false); err != nil {
			return wallet.Order{}, err
		}
	}
	return r.Order(process, tenant, entry.OrderID)
}

func (r *WalletRepository) StaticAllocations(ctx context.Context, tenant string) ([]wallet.StaticAllocation, error) {
	if r.db == nil {
		return nil, errWalletUnavailable
	}
	var rows []storedStaticIP
	if err := r.db.WithContext(ctx).Table("static_ip_allocations").Where("tenant_id = ?", tenant).
		Order("expires_at DESC, supplier_item_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	allocations := make([]wallet.StaticAllocation, 0, len(rows))
	for _, row := range rows {
		if r.staticCipher == nil {
			return nil, errWalletUnavailable
		}
		password, err := r.staticCipher.Open(nil, nil, row.PasswordCiphertext, staticPasswordScope(tenant, row.SupplierItemID))
		if err != nil {
			return nil, err
		}
		row.Password = string(password)
		allocations = append(allocations, row.StaticAllocation)
	}
	return allocations, nil
}

func (r *WalletRepository) ResolveStatic(ctx context.Context, tenant string, id int64, operator, supplierID, evidence string, closeRemaining bool) (wallet.Order, error) {
	if r.db == nil {
		return wallet.Order{}, errWalletUnavailable
	}
	var supplierOrderID string
	err := scanWalletRecord(r.db.WithContext(ctx).Raw(`
        SELECT COALESCE(s.supplier_order_id, '') FROM static_purchases s
        JOIN purchase_orders o ON o.id = s.order_id AND o.tenant_id = s.tenant_id
        WHERE s.tenant_id = ? AND s.order_id = ? AND o.status = 'pending'
        AND o.created_at <= clock_timestamp() - interval '24 hours'`, tenant, id), &supplierOrderID)
	if err != nil {
		return wallet.Order{}, err
	}
	if supplierID == "" {
		supplierID = supplierOrderID
	}
	if supplierOrderID != "" && supplierOrderID != supplierID {
		return wallet.Order{}, wallet.ErrState
	}
	var delivery wallet.StaticDelivery
	if supplierID != "" {
		if r.static == nil {
			return wallet.Order{}, wallet.ErrStaticUnavailable
		}
		delivery, err = r.static.Detail(ctx, supplierID)
		if err != nil && !closeRemaining {
			return wallet.Order{}, wallet.ErrStaticUnavailable
		}
	} else if !closeRemaining {
		return wallet.Order{}, wallet.ErrInvalid
	}
	if err := r.recordStaticDelivery(ctx, tenant, id, delivery, operator, evidence, closeRemaining, closeRemaining); err != nil {
		return wallet.Order{}, err
	}
	return r.Order(ctx, tenant, id)
}

func (r *WalletRepository) recordStaticDelivery(ctx context.Context, tenant string, id int64, delivery wallet.StaticDelivery, operator, evidence string, closeRemaining, requireDeadline bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := findPurchaseOrder(tx, tenant, id, true)
		if err != nil {
			return err
		}
		if order.Kind != wallet.StaticPurchase || order.Status != wallet.Pending || len(order.Items) != 1 {
			return wallet.ErrState
		}
		item := order.Items[0]
		if requireDeadline && time.Since(order.CreatedAt) < 72*time.Hour {
			return wallet.ErrState
		}
		if delivery.SupplierOrderID != "" {
			result := tx.Exec(`UPDATE static_purchases SET supplier_order_id = ?
                WHERE tenant_id = ? AND order_id = ? AND (supplier_order_id IS NULL OR supplier_order_id = ?)`,
				delivery.SupplierOrderID, tenant, id, delivery.SupplierOrderID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return wallet.ErrState
			}
		}
		for _, ip := range delivery.Allocations {
			if r.staticCipher == nil {
				return errWalletUnavailable
			}
			password := r.staticCipher.Seal(nil, nil, []byte(ip.Password), staticPasswordScope(tenant, ip.SupplierItemID))
			result := tx.Exec(`INSERT INTO static_ip_allocations (supplier_item_id, tenant_id, order_id, ip, port, username, password_ciphertext, expires_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (supplier_item_id) DO UPDATE SET expires_at = EXCLUDED.expires_at
                WHERE static_ip_allocations.tenant_id = EXCLUDED.tenant_id AND static_ip_allocations.order_id = EXCLUDED.order_id`,
				ip.SupplierItemID, tenant, id, ip.IP, ip.Port, ip.Username, password, ip.ExpiresAt)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return wallet.ErrState
			}
		}
		var count int32
		if err := scanWalletRecord(tx.Raw("SELECT COUNT(*) FROM static_ip_allocations WHERE tenant_id = ? AND order_id = ?", tenant, id), &count); err != nil {
			return err
		}
		if count > item.Quantity {
			return wallet.ErrState
		}
		if count < item.Quantity && !closeRemaining {
			order.DeliveredCount = int64(count)
			order.Items[0].DeliveredCount = count
			_, err := saveSettlementOrder(tx, tenant, order)
			return err
		}
		if operator != "" {
			if err := tx.Exec(`INSERT INTO static_purchase_resolutions (tenant_id, order_id, operator_id, evidence)
                VALUES (?, ?, ?, ?)`, tenant, id, operator, evidence).Error; err != nil {
				return err
			}
		}
		var before int64
		if err := scanWalletRecord(tx.Raw("SELECT available_minor FROM wallets WHERE tenant_id = ? FOR UPDATE", tenant), &before); err != nil {
			return err
		}
		command := wallet.Command{TenantID: tenant, OrderID: id, Key: uuid.NewString(), Action: wallet.Release,
			Amount: item.UnitMinor * int64(count), ItemDeliveries: []int32{count}}
		fingerprint, err := walletFingerprint(command)
		if err != nil {
			return err
		}
		_, err = applyWalletCommand(tx, command, order, before, fingerprint)
		return err
	})
}

func (r *WalletRepository) PendingStaticPurchases(ctx context.Context, afterID int64) ([]wallet.PendingStaticPurchase, error) {
	if r.db == nil {
		return nil, errWalletUnavailable
	}
	var purchases []wallet.PendingStaticPurchase
	err := r.db.WithContext(ctx).Raw(`
        SELECT o.id AS order_id, o.tenant_id, COALESCE(s.supplier_order_id, '') AS supplier_order_id,
            o.created_at, o.delivered_count, o.created_at <= clock_timestamp() - interval '72 hours' AS can_close
        FROM purchase_orders o JOIN static_purchases s ON s.tenant_id = o.tenant_id AND s.order_id = o.id
        WHERE o.status = 'pending' AND o.created_at <= clock_timestamp() - interval '24 hours' AND o.id > ?
        ORDER BY o.id LIMIT ?`, afterID, wallet.PageSize+1).Scan(&purchases).Error
	return purchases, err
}
