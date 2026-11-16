package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Wallet) ListStaticOffers(ctx context.Context, _ *v1.BaseRequest) (*v1.StaticOffers, error) {
	noStore(ctx)
	offers, err := s.biz.StaticOffers(ctx)
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	result := &v1.StaticOffers{}
	for _, offer := range offers {
		result.Offers = append(result.Offers, &v1.StaticOffer{NodeId: offer.NodeID, Region: offer.Region,
			IpType: offer.IPType, Term: offer.Term, TermDays: offer.TermDays, UnitMinor: offer.UnitMinor, Available: offer.Available})
	}
	return result, nil
}

func (s *Wallet) PurchaseStaticIP(ctx context.Context, req *v1.PurchaseStaticIPRequest) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	order, err := s.biz.PurchaseStatic(ctx, bearerToken(ctx), wallet.StaticPurchaseRequest{
		NodeID: req.GetNodeId(), Term: req.GetTerm(), Quantity: req.GetQuantity(), UnitMinor: req.GetUnitMinor(), Key: req.GetRequestKey(),
	})
	return orderMessage(order), wallet.PublicError(err)
}

func (s *Wallet) ListStaticIPs(ctx context.Context, _ *v1.BaseRequest) (*v1.StaticIPs, error) {
	noStore(ctx)
	allocations, err := s.biz.StaticAllocations(ctx, bearerToken(ctx))
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	result := &v1.StaticIPs{}
	for _, ip := range allocations {
		result.Allocations = append(result.Allocations, &v1.StaticIP{SupplierItemId: ip.SupplierItemID, OrderId: ip.OrderID,
			Ip: ip.IP, Port: ip.Port, Username: ip.Username, Password: ip.Password, ExpiresAt: timestamppb.New(ip.ExpiresAt)})
	}
	return result, nil
}

func (s *Wallet) ResolveStaticPurchase(ctx context.Context, req *v1.ResolveStaticPurchaseRequest) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	order, err := s.biz.ResolveStatic(ctx, bearerToken(ctx), req.GetTenantId(), req.GetOrderId(),
		req.GetSupplierOrderId(), req.GetEvidence(), req.GetCloseRemaining())
	return orderMessage(order), wallet.PublicError(err)
}

func (s *Wallet) ListPendingStaticPurchases(ctx context.Context, req *v1.PendingStaticPurchasesRequest) (*v1.PendingStaticPurchases, error) {
	noStore(ctx)
	pending, err := s.biz.PendingStaticPurchases(ctx, bearerToken(ctx), req.GetAfterId())
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	result := &v1.PendingStaticPurchases{}
	if len(pending) > wallet.PageSize {
		pending = pending[:wallet.PageSize]
		result.NextAfterId = pending[len(pending)-1].OrderID
	}
	for _, item := range pending {
		result.Purchases = append(result.Purchases, &v1.PendingStaticPurchase{OrderId: item.OrderID,
			TenantId: item.TenantID, SupplierOrderId: item.SupplierOrderID, DeliveredCount: item.DeliveredCount,
			CreatedAt: timestamppb.New(item.CreatedAt), CanClose: item.CanClose})
	}
	return result, nil
}
