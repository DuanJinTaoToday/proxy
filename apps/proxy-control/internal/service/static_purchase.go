package service

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/product"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Wallet) ListStaticOffers(ctx context.Context, req *v1.StaticOffersRequest) (*v1.StaticOffers, error) {
	noStore(ctx)
	offers, err := s.biz.StaticOffers(ctx, req.GetShareMode(), req.GetPurposeId())
	if errors.Is(err, product.ErrSaleClosed) {
		return &v1.StaticOffers{}, nil
	}
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	result := &v1.StaticOffers{Open: true, QuotedAt: timestamppb.Now()}
	for _, offer := range offers {
		result.Offers = append(result.Offers, staticOfferMessage(offer))
	}
	return result, nil
}

func staticOfferMessage(offer wallet.StaticOffer) *v1.StaticOffer {
	return &v1.StaticOffer{NodeId: offer.NodeID, Region: offer.Region, IpType: offer.IPType, Term: offer.Term,
		TermDays: offer.TermDays, UnitMinor: offer.UnitMinor, Available: offer.Available, ShareMode: offer.ShareMode, PurposeId: offer.PurposeID,
		Continent: offer.Continent, Line: offer.Line}
}

func (s *Wallet) QuoteStaticPurchase(ctx context.Context, req *v1.StaticPurchaseQuoteRequest) (*v1.StaticPurchaseQuote, error) {
	noStore(ctx)
	selection := wallet.StaticPurchaseRequest{
		NodeID: req.GetNodeId(), Term: req.GetTerm(), Quantity: req.GetQuantity(),
		ShareMode: req.GetShareMode(), PurposeID: req.GetPurposeId(),
	}
	for _, line := range req.GetLines() {
		selection.Lines = append(selection.Lines, wallet.StaticPurchaseLine{
			NodeID: line.GetNodeId(), Quantity: line.GetQuantity(),
		})
	}
	quote, err := s.biz.QuoteStatic(ctx, bearerToken(ctx), selection)
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	response := &v1.StaticPurchaseQuote{
		ConfirmationToken: quote.ConfirmationToken, ExpiresAt: timestamppb.New(quote.ExpiresAt),
	}
	if len(req.GetLines()) == 0 {
		response.Offer = staticOfferMessage(quote.Lines[0].Offer)
		response.Quantity = quote.Lines[0].Quantity
		return response, nil
	}
	for _, line := range quote.Lines {
		response.Lines = append(response.Lines, &v1.StaticPurchaseQuoteLine{
			Offer: staticOfferMessage(line.Offer), Quantity: line.Quantity,
		})
	}
	return response, nil
}

func (s *Wallet) PurchaseStaticIP(ctx context.Context, req *v1.PurchaseStaticIPRequest) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	purchase := wallet.StaticPurchaseRequest{
		NodeID: req.GetNodeId(), Term: req.GetTerm(), Quantity: req.GetQuantity(), UnitMinor: req.GetUnitMinor(),
		Key: req.GetRequestKey(), ConfirmationToken: req.GetConfirmationToken(), ShareMode: req.GetShareMode(), PurposeID: req.GetPurposeId(),
	}
	for _, line := range req.GetLines() {
		purchase.Lines = append(purchase.Lines, wallet.StaticPurchaseLine{
			NodeID: line.GetNodeId(), Quantity: line.GetQuantity(), UnitMinor: line.GetUnitMinor(),
		})
	}
	order, err := s.biz.PurchaseStatic(ctx, bearerToken(ctx), purchase)
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
		result.Allocations = append(result.Allocations, staticIPMessage(ip))
	}
	return result, nil
}

func staticIPMessage(ip wallet.StaticAllocation) *v1.StaticIP {
	return &v1.StaticIP{AllocationId: ip.AllocationID, OrderId: ip.OrderID, Ip: ip.IP, Port: ip.Port,
		Username: ip.Username, Password: ip.Password, ExpiresAt: timestamppb.New(ip.ExpiresAt),
		Region: ip.Region, IpType: ip.IPType, PendingRenewalOrderId: ip.PendingRenewalOrderID}
}

func (s *Wallet) GetStaticIPConfig(ctx context.Context, req *v1.StaticIPConfigRequest) (*v1.StaticIP, error) {
	noStore(ctx)
	ips, err := s.biz.StaticCredentials(ctx, bearerToken(ctx), []string{req.GetAllocationId()}, true)
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	return staticIPMessage(ips[0]), nil
}

func (s *Wallet) ExportStaticIPs(ctx context.Context, req *v1.StaticIPExportRequest) ([]byte, error) {
	noStore(ctx)
	ips, err := s.biz.StaticCredentials(ctx, bearerToken(ctx), req.GetAllocationIds(), true)
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	var result strings.Builder
	for _, ip := range ips {
		result.WriteString(net.JoinHostPort(ip.IP, strconv.Itoa(int(ip.Port))) + ":" + ip.Username + ":" + ip.Password + "\n")
	}
	return []byte(result.String()), nil
}

func (s *Wallet) ResolveStaticPurchase(ctx context.Context, req *v1.ResolveStaticPurchaseRequest) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	order, err := s.biz.ResolveStatic(ctx, bearerToken(ctx), req.GetTenantId(), req.GetOrderId(),
		req.GetSupplierOrderId(), req.GetEvidence(), req.GetCloseRemaining(), req.GetConfirmedUndeliverable())
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
		var renewals []string
		if item.PendingRenewalItems != "" {
			renewals = strings.Split(item.PendingRenewalItems, ",")
		}
		result.Purchases = append(result.Purchases, &v1.PendingStaticPurchase{OrderId: item.OrderID,
			TenantId: item.TenantID, SupplierOrderId: item.SupplierOrderID, DeliveredCount: item.DeliveredCount,
			CreatedAt: timestamppb.New(item.CreatedAt), CanClose: item.CanClose, PendingRenewalItems: renewals})
	}
	return result, nil
}

func (s *Wallet) ClaimStaticIPExpiryNotices(ctx context.Context, _ *v1.BaseRequest) (*v1.StaticIPExpiryNotices, error) {
	noStore(ctx)
	count, err := s.biz.ClaimStaticIPExpiryNotices(ctx, bearerToken(ctx))
	return &v1.StaticIPExpiryNotices{Count: count}, wallet.PublicError(err)
}
