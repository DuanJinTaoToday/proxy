package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
)

type Product struct{ biz *product.Service }

func NewProduct(biz *product.Service) *Product { return &Product{biz: biz} }

func (s *Product) ListProducts(ctx context.Context, _ *v1.BaseRequest) (*v1.Products, error) {
	noStore(ctx)
	items, err := s.biz.Products(ctx)
	if err != nil {
		return nil, product.PublicError(err)
	}
	response := &v1.Products{}
	for _, item := range items {
		response.Products = append(response.Products, productMessage(item))
	}
	return response, nil
}

func (s *Product) ListTrafficPackages(ctx context.Context, _ *v1.BaseRequest) (*v1.TrafficPackages, error) {
	noStore(ctx)
	items, err := s.biz.Packages(ctx, bearerToken(ctx))
	if err != nil {
		return nil, product.PublicError(err)
	}
	response := &v1.TrafficPackages{}
	for _, item := range items {
		response.Packages = append(response.Packages, packageMessage(item))
	}
	return response, nil
}

func (s *Product) UpdateTrafficPackage(ctx context.Context, req *v1.UpdateTrafficPackageRequest) (*v1.TrafficPackage, error) {
	noStore(ctx)
	item, err := s.biz.UpdatePackage(ctx, bearerToken(ctx), product.Package{
		ID: req.GetId(), CapacityBytes: req.GetCapacityBytes(), PriceMinor: req.GetPriceMinor(),
		Version: req.GetVersion(),
	}, req.Listed)
	return packageMessage(item), product.PublicError(err)
}

func (s *Product) SetProductSale(ctx context.Context, req *v1.SetProductSaleRequest) (*v1.Product, error) {
	noStore(ctx)
	item, err := s.biz.SetSale(ctx, bearerToken(ctx), product.Sale{
		Kind: req.GetKind(), QuoteReady: req.GetQuoteReady(), AcceptanceEvidence: req.GetAcceptanceEvidence(),
	}, req.Open)
	return productMessage(item), product.PublicError(err)
}

func packageMessage(item product.Package) *v1.TrafficPackage {
	return &v1.TrafficPackage{Id: item.ID, CapacityBytes: item.CapacityBytes, PriceMinor: item.PriceMinor, Listed: item.Listed, Version: item.Version}
}

func productMessage(item product.Product) *v1.Product {
	response := &v1.Product{Kind: item.Kind, Description: item.Description, Open: item.Open, CanPurchase: item.CanPurchase}
	for _, pkg := range item.Packages {
		message := packageMessage(pkg.Package)
		message.DiscountPercent = pkg.DiscountPercent
		response.Packages = append(response.Packages, message)
	}
	return response
}
