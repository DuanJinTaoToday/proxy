package product

import (
	"context"
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

const (
	Dynamic           = "dynamic"
	Static            = "static"
	BaselinePackageID = "5gb"
)

var (
	ErrInvalidPackage     = errors.BadRequest("INVALID_TRAFFIC_PACKAGE", "invalid traffic package configuration")
	ErrInvalidSale        = errors.BadRequest("INVALID_PRODUCT_SALE", "opening a product requires acceptance evidence and quote confirmation")
	ErrNotFound           = errors.NotFound("PACKAGE_NOT_FOUND", "traffic package not found")
	ErrSaleClosed         = errors.Conflict("PRODUCT_SALE_CLOSED", "product is not open for purchase")
	ErrQuoteNotReady      = errors.Conflict("PRODUCT_QUOTE_NOT_READY", "product quotes are not ready")
	ErrPackageUnavailable = errors.Conflict("PACKAGE_UNAVAILABLE", "traffic package is not available for purchase")
	ErrPackageChanged     = errors.Conflict("PACKAGE_CHANGED", "traffic package changed; confirm the latest configuration")
)

type Package struct {
	ID            string `json:"id"`
	CapacityBytes int64  `json:"capacity_bytes"`
	PriceMinor    int64  `json:"price_minor"`
	Listed        bool   `json:"listed"`
	Version       int64  `json:"version"`
}

func (p Package) Available() bool { return p.Listed && p.PriceMinor > 0 }

func (p Package) discountPercent(base Package) int32 {
	if base.PriceMinor <= 0 || base.CapacityBytes <= 0 {
		return 0
	}
	original := new(big.Int).Mul(big.NewInt(base.PriceMinor), big.NewInt(p.CapacityBytes))
	current := new(big.Int).Mul(big.NewInt(p.PriceMinor), big.NewInt(base.CapacityBytes))
	saving := new(big.Int).Sub(original, current)
	if saving.Sign() <= 0 {
		return 0
	}
	saving.Mul(saving, big.NewInt(100))
	saving.Add(saving, new(big.Int).Rsh(new(big.Int).Set(original), 1))
	return int32(saving.Quo(saving, original).Int64())
}

// DefaultTierPrice computes the initial total for an untouched tier from the
// base price per byte. It rounds to the nearest minor unit, with ties rounded up.
func (p Package) DefaultTierPrice(base Package) (int64, error) {
	rates := map[string]int64{"50gb": 90, "200gb": 80}
	rate := rates[p.ID]
	if rate == 0 || p.Version != 1 || p.PriceMinor != 0 {
		return 0, nil
	}
	numerator := new(big.Int).Mul(big.NewInt(base.PriceMinor), big.NewInt(p.CapacityBytes))
	numerator.Mul(numerator, big.NewInt(rate))
	denominator := new(big.Int).Mul(big.NewInt(base.CapacityBytes), big.NewInt(100))
	numerator.Add(numerator, new(big.Int).Rsh(new(big.Int).Set(denominator), 1))
	result := numerator.Quo(numerator, denominator)
	if !result.IsInt64() || result.Sign() <= 0 {
		return 0, ErrInvalidPackage
	}
	return result.Int64(), nil
}

func DynamicQuoteReady(packages []Package) bool {
	listed := false
	for _, pkg := range packages {
		if pkg.Listed {
			if !pkg.Available() {
				return false
			}
			listed = true
		}
	}
	return listed
}

type Sale struct {
	Kind               string `json:"kind"`
	Open               bool   `json:"open"`
	QuoteReady         bool   `json:"quote_ready"`
	AcceptanceEvidence string `json:"acceptance_evidence"`
}

type Product struct {
	Sale
	Description string
	CanPurchase bool
	Packages    []PublicPackage
}

type PublicPackage struct {
	Package
	DiscountPercent int32
}

type Repository interface {
	Catalog(context.Context) ([]Sale, []Package, error)
	Packages(context.Context) ([]Package, error)
	UpdatePackage(context.Context, string, Package) (Package, error)
	SetSale(context.Context, string, Sale) (Sale, []Package, error)
}

type Service struct {
	store  Repository
	access *access.Service
}

func NewService(store Repository, accessService *access.Service) *Service {
	return &Service{store: store, access: accessService}
}

func (s *Service) Products(ctx context.Context) ([]Product, error) {
	sales, packages, err := s.store.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	products := make([]Product, 0, len(sales))
	for _, sale := range sales {
		products = append(products, present(sale, packages))
	}
	return products, nil
}

func present(sale Sale, packages []Package) Product {
	item := Product{Sale: sale, Description: "购买后获得指定租期的静态住宅 IP。"}
	if sale.Kind == Dynamic {
		item.Description = "通过 SOCKS5 隧道使用动态住宅代理，按实际用量扣减流量。"
		if sale.Open {
			var base Package
			for _, pkg := range packages {
				if pkg.ID == BaselinePackageID {
					base = pkg
					break
				}
			}
			for _, pkg := range packages {
				if pkg.Available() {
					item.Packages = append(item.Packages, PublicPackage{Package: pkg, DiscountPercent: pkg.discountPercent(base)})
				}
			}
		}
	}
	item.CanPurchase = sale.Open && (sale.Kind == Static || len(item.Packages) > 0)
	return item
}

func (s *Service) Packages(ctx context.Context, token string) ([]Package, error) {
	if _, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling); err != nil {
		return nil, err
	}
	return s.store.Packages(ctx)
}

func (s *Service) UpdatePackage(ctx context.Context, token string, pkg Package, listed *bool) (Package, error) {
	actor, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return Package{}, err
	}
	if listed == nil || pkg.ID == "" || len(pkg.ID) > 32 || strings.ContainsRune(pkg.ID, 0) ||
		pkg.CapacityBytes <= 0 || pkg.PriceMinor < 0 || pkg.Version <= 0 || (*listed && pkg.PriceMinor == 0) {
		return Package{}, ErrInvalidPackage
	}
	pkg.Listed = *listed
	return s.store.UpdatePackage(ctx, actor, pkg)
}

func (s *Service) SetSale(ctx context.Context, token string, sale Sale, open *bool) (Product, error) {
	actor, err := s.access.AuthorizeOperator(ctx, token, access.ManageBilling)
	if err != nil {
		return Product{}, err
	}
	sale.AcceptanceEvidence = strings.TrimSpace(sale.AcceptanceEvidence)
	if open == nil || (sale.Kind != Dynamic && sale.Kind != Static) ||
		utf8.RuneCountInString(sale.AcceptanceEvidence) > 1000 || strings.ContainsRune(sale.AcceptanceEvidence, 0) ||
		(*open && (!sale.QuoteReady || sale.AcceptanceEvidence == "")) {
		return Product{}, ErrInvalidSale
	}
	sale.Open = *open
	if !sale.Open {
		sale.QuoteReady = false
		sale.AcceptanceEvidence = ""
	}
	saved, packages, err := s.store.SetSale(ctx, actor, sale)
	return present(saved, packages), err
}

func PublicError(err error) error {
	if err == nil {
		return nil
	}
	if internal := errors.FromError(err); internal.Code < 500 || internal.Reason == "PRODUCT_UNAVAILABLE" {
		return err
	}
	return errors.InternalServer("PRODUCT_ERROR", "could not access product configuration")
}
