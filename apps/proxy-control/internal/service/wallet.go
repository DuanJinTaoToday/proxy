package service

import (
	"context"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Wallet struct {
	biz *wallet.Service
}

func NewWallet(biz *wallet.Service) *Wallet { return &Wallet{biz: biz} }

func (s *Wallet) FindTenants(ctx context.Context, req *v1.FindWalletTenantRequest) (*v1.WalletTenant, error) {
	noStore(ctx)
	tenant, err := s.biz.FindTenant(ctx, bearerToken(ctx), req.GetQuery())
	return tenantMessage(tenant, true), wallet.PublicError(err)
}

func (s *Wallet) GetTenant(ctx context.Context, req *v1.WalletTenantRequest) (*v1.WalletTenant, error) {
	return s.getTenant(ctx, req.GetTenantId(), true)
}

func (s *Wallet) GetCustomerWallet(ctx context.Context, _ *v1.BaseRequest) (*v1.WalletTenant, error) {
	noStore(ctx)
	tenant, err := s.biz.CustomerWallet(ctx, bearerToken(ctx))
	return tenantMessage(tenant, false), wallet.PublicError(err)
}

func (s *Wallet) PurchaseTrafficPackage(ctx context.Context, req *v1.PurchaseTrafficPackageRequest) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	order, err := s.biz.PurchasePackage(ctx, bearerToken(ctx), wallet.PackagePurchase{
		PackageID: req.GetPackageId(), Version: req.GetPackageVersion(), Key: req.GetRequestKey(),
	})
	return orderMessage(order), wallet.PublicError(err)
}

func (s *Wallet) getTenant(ctx context.Context, id string, operations bool) (*v1.WalletTenant, error) {
	noStore(ctx)
	tenant, err := s.biz.Tenant(ctx, bearerToken(ctx), id, operations)
	return tenantMessage(tenant, operations), wallet.PublicError(err)
}

func (s *Wallet) QuoteWalletRecharge(ctx context.Context, req *v1.QuoteWalletRechargeRequest) (*v1.WalletRechargeQuote, error) {
	noStore(ctx)
	tenant, quote, after, err := s.biz.QuoteRecharge(ctx, bearerToken(ctx), req.GetTenantId(), req.GetRequestKey(), req.GetReason(), req.GetAmountMinor())
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	return &v1.WalletRechargeQuote{Tenant: tenantMessage(tenant, true), AmountMinor: quote.AmountMinor,
		Reason: quote.Reason, ExpectedAfterMinor: after, ConfirmationToken: quote.ID, ExpiresAt: timestamppb.New(quote.ExpiresAt)}, nil
}

func (s *Wallet) RechargeWallet(ctx context.Context, req *v1.RechargeWalletRequest) (*v1.WalletEntry, error) {
	noStore(ctx)
	entry, err := s.biz.Recharge(ctx, bearerToken(ctx), req.GetTenantId(), req.GetRequestKey(), req.GetConfirmationToken())
	return entryMessage(entry, true), wallet.PublicError(err)
}

func (s *Wallet) ListOperatorWalletEntries(ctx context.Context, req *v1.OperatorWalletEntriesRequest) (*v1.WalletEntries, error) {
	return s.entries(ctx, req.GetTenantId(), true, wallet.Filter{
		BeforeID: req.GetBeforeId(), Kind: req.GetKind(), StartDate: req.GetStartDate(), EndDate: req.GetEndDate(),
	})
}

func (s *Wallet) ListCustomerWalletEntries(ctx context.Context, req *v1.CustomerWalletEntriesRequest) (*v1.WalletEntries, error) {
	return s.entries(ctx, "", false, wallet.Filter{
		BeforeID: req.GetBeforeId(), Kind: req.GetKind(), StartDate: req.GetStartDate(), EndDate: req.GetEndDate(),
	})
}

func (s *Wallet) entries(ctx context.Context, id string, operations bool, filter wallet.Filter) (*v1.WalletEntries, error) {
	noStore(ctx)
	entries, err := s.biz.Entries(ctx, bearerToken(ctx), id, operations, filter)
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	response := &v1.WalletEntries{}
	if len(entries) > wallet.PageSize {
		entries = entries[:wallet.PageSize]
		response.NextBeforeId = entries[len(entries)-1].ID
	}
	for _, entry := range entries {
		response.Entries = append(response.Entries, entryMessage(entry, operations))
	}
	return response, nil
}

func (s *Wallet) ListOperatorPurchaseOrders(ctx context.Context, req *v1.OperatorPurchaseOrdersRequest) (*v1.PurchaseOrders, error) {
	return s.orders(ctx, req.GetTenantId(), true, wallet.Filter{
		BeforeID: req.GetBeforeId(), OrderQuery: req.GetOrderQuery(), Kind: req.GetKind(), Status: req.GetStatus(),
	})
}

func (s *Wallet) ListCustomerPurchaseOrders(ctx context.Context, req *v1.CustomerPurchaseOrdersRequest) (*v1.PurchaseOrders, error) {
	return s.orders(ctx, "", false, wallet.Filter{
		BeforeID: req.GetBeforeId(), OrderQuery: req.GetOrderQuery(), Kind: req.GetKind(),
		Status: req.GetStatus(), Product: req.GetProduct(), StartDate: req.GetStartDate(), EndDate: req.GetEndDate(),
	})
}

func (s *Wallet) orders(ctx context.Context, id string, operations bool, filter wallet.Filter) (*v1.PurchaseOrders, error) {
	noStore(ctx)
	page, err := s.biz.Orders(ctx, bearerToken(ctx), id, operations, filter)
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	orders := page.Orders
	response := &v1.PurchaseOrders{PendingCount: page.PendingCount}
	if len(orders) > wallet.PageSize {
		orders = orders[:wallet.PageSize]
		response.NextBeforeId = orders[len(orders)-1].ID
	}
	for _, order := range orders {
		response.Orders = append(response.Orders, orderMessage(order))
	}
	return response, nil
}

func (s *Wallet) GetOperatorPurchaseOrder(ctx context.Context, req *v1.OperatorPurchaseOrderRequest) (*v1.PurchaseOrder, error) {
	return s.getOrder(ctx, req.GetTenantId(), true, req.GetOrderId())
}

func (s *Wallet) GetCustomerPurchaseOrder(ctx context.Context, req *v1.CustomerPurchaseOrderRequest) (*v1.PurchaseOrder, error) {
	return s.getOrder(ctx, "", false, req.GetOrderId())
}

func (s *Wallet) getOrder(ctx context.Context, id string, operations bool, orderID int64) (*v1.PurchaseOrder, error) {
	noStore(ctx)
	order, err := s.biz.Order(ctx, bearerToken(ctx), id, operations, orderID)
	return orderMessage(order), wallet.PublicError(err)
}

func tenantMessage(tenant wallet.Tenant, operations bool) *v1.WalletTenant {
	message := &v1.WalletTenant{TenantId: tenant.ID, Name: tenant.Name, PrimaryAccountName: tenant.PrimaryAccountName,
		AvailableMinor: tenant.AvailableMinor, AvailableTrafficBytes: tenant.AvailableTrafficBytes,
		TotalRechargeMinor: tenant.TotalRechargeMinor, TotalSpendMinor: tenant.TotalSpendMinor}
	if operations {
		message.Email = tenant.Email
		if !tenant.RegisteredAt.IsZero() {
			message.RegisteredAt = timestamppb.New(tenant.RegisteredAt)
		}
	}
	return message
}

func entryMessage(entry wallet.Entry, operations bool) *v1.WalletEntry {
	message := &v1.WalletEntry{
		Id: entry.ID, OrderId: entry.OrderID, Kind: entry.Kind,
		BeforeMinor: entry.BeforeMinor, AfterMinor: entry.AfterMinor,
		CreatedAt: timestamppb.New(entry.CreatedAt),
	}
	if operations {
		message.OperatorId, message.OperatorName, message.Reason = entry.Actor, entry.OperatorName, entry.Reason
	}
	return message
}

func orderMessage(order wallet.Order) *v1.PurchaseOrder {
	message := &v1.PurchaseOrder{
		Id: order.ID, Kind: order.Kind, Status: order.Status,
		LockedMinor: order.LockedMinor, DebitedMinor: order.DebitedMinor,
		ReleasedMinor: order.ReleasedMinor, RefundedMinor: order.ReleasedMinor + order.RefundedMinor,
		DeliveredBytes: order.DeliveredBytes, DeliveredCount: order.DeliveredCount,
		CreatedAt: timestamppb.New(order.CreatedAt), SettledMinor: order.SettledMinor(), UnsettledMinor: order.UnsettledMinor(),
	}
	if order.Kind == wallet.StaticPurchase || order.Kind == wallet.StaticRenewal {
		message.PrepaidMinor = order.DebitedMinor
	}
	for _, item := range order.Items {
		line := orderItemMessage(item)
		if order.Kind == wallet.StaticRenewal {
			line.AllocationId = item.Reference
			line.Outcome = item.Outcome
		}
		message.Items = append(message.Items, line)
	}
	for _, entry := range order.Entries {
		message.WalletEntries = append(message.WalletEntries, entryMessage(entry, false))
	}
	if order.SummaryItem != nil {
		message.SummaryItem = orderItemMessage(*order.SummaryItem)
		message.ItemCount = order.ItemCount
	}
	return message
}

func orderItemMessage(item wallet.OrderItem) *v1.PurchaseOrderItem {
	return &v1.PurchaseOrderItem{
		Position: item.Position, Specification: item.Specification, CapacityBytes: item.CapacityBytes,
		TermDays: item.TermDays, UnitMinor: item.UnitMinor, Quantity: item.Quantity, DeliveredCount: item.DeliveredCount,
	}
}
