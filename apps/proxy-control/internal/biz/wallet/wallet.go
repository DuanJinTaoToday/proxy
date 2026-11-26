package wallet

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

type Action string

const (
	Recharge Action = "recharge"
	Debit    Action = "debit"
	Hold     Action = "hold"
	Release  Action = "release"
	Refund   Action = "refund"
)

const (
	TrafficPackage = "traffic_package"
	StaticPurchase = "static_purchase"
	StaticRenewal  = "static_renewal"

	Pending   = "pending"
	Succeeded = "succeeded"
	Partial   = "partial"
	Failed    = "failed"
)

var (
	ErrInvalid        = errors.BadRequest("INVALID_WALLET_REQUEST", "invalid wallet request")
	ErrMissing        = errors.NotFound("WALLET_RECORD_NOT_FOUND", "wallet record not found")
	ErrReplay         = errors.Conflict("WALLET_REQUEST_CONFLICT", "request key is already used for a different operation")
	ErrBalanceChanged = errors.Conflict("WALLET_BALANCE_CHANGED", "confirm the current balance before recharging")
	ErrQuoteExpired   = errors.Conflict("WALLET_QUOTE_EXPIRED", "recharge confirmation has expired")
	ErrState          = errors.Conflict("WALLET_ORDER_STATE", "operation conflicts with order state")
	ErrOverflow       = errors.Conflict("WALLET_BALANCE_LIMIT", "wallet balance exceeds the supported amount")
)

type Tenant struct {
	ID, Name, PrimaryAccountName, Email string
	RegisteredAt                        time.Time
	AvailableMinor                      int64
	AvailableTrafficBytes               int64
	TotalRechargeMinor, TotalSpendMinor string
}

type RechargeQuote struct {
	ID, TenantID, Actor, RequestKey, Reason string
	AmountMinor, ExpectedBeforeMinor        int64
	ExpiresAt                               time.Time
}

type Entry struct {
	ID, OrderID, BeforeMinor, AfterMinor int64
	Kind, Actor, OperatorName, Reason    string
	CreatedAt                            time.Time
}

type Order struct {
	ID                                                      int64
	Kind, Status                                            string
	LockedMinor, DebitedMinor, ReleasedMinor, RefundedMinor int64
	DeliveredBytes, DeliveredCount, ConfirmedMinor          int64
	CreatedAt                                               time.Time
	SettledAt                                               *time.Time
	Items                                                   []OrderItem `gorm:"-"`
	Entries                                                 []Entry     `gorm:"-"`
	SummaryItem                                             *OrderItem  `gorm:"-"`
	ItemCount                                               int64       `gorm:"-"`
}

type OrderItem struct {
	StaticRegion, StaticIPType                   string
	OrderID, CapacityBytes, UnitMinor            int64
	Position, TermDays, Quantity, DeliveredCount int32
	Specification, Reference                     string
	Outcome                                      string
}

func (o Order) UnsettledMinor() int64 {
	if o.Status == Pending {
		return o.DebitedMinor - o.ConfirmedMinor - o.ReleasedMinor
	}
	return 0
}

func (o Order) SettledMinor() int64 {
	return o.DebitedMinor - o.ReleasedMinor - o.RefundedMinor - o.UnsettledMinor()
}

// Command amounts are in fen. Debit creates a completed order and Hold creates
// a pending order. Release finalizes a hold: Amount is the amount retained
// (zero on failure). Refund returns part or all of an already settled charge.
type Command struct {
	TenantID, Key  string
	Action         Action
	Amount         int64
	Actor, Reason  string
	ExpectedBefore *int64
	QuoteExpiresAt time.Time `json:"-"`
	OrderID        int64
	Kind           string
	Items          []OrderItem
	ItemDeliveries []int32
}

type Filter struct {
	BeforeID              int64
	Limit                 int
	Kind, Status, Product string
	StartDate, EndDate    string
	OrderQuery            string
}

func (f Filter) validOrderQuery() bool {
	return len(f.OrderQuery) <= 19 &&
		strings.IndexFunc(f.OrderQuery, func(r rune) bool { return r < '0' || r > '9' }) == -1
}

func (f Filter) validDates() bool {
	for _, date := range []string{f.StartDate, f.EndDate} {
		if date != "" {
			parsed, err := time.Parse(time.DateOnly, date)
			if err != nil || parsed.Year() == 0 || parsed.Format(time.DateOnly) != date {
				return false
			}
		}
	}
	return f.StartDate == "" || f.EndDate == "" || f.StartDate <= f.EndDate
}

type OrderPage struct {
	Orders       []Order
	PendingCount int64
}

type PackagePurchase struct {
	PackageID string
	Version   int64
	Key       string
}

// PurchasedTrafficDebtOffset applies at most the purchased amount to existing debt.
func PurchasedTrafficDebtOffset(before, capacity int64) int64 {
	// Clamp before negating so even the minimum signed balance is safe.
	return -max(min(before, 0), -capacity)
}

type Repository interface {
	CommissionRepository
	StaticRenewalRepository
	StaticExchangeRepository
	StaticRepository
	FindTenant(context.Context, string, string) (Tenant, error)
	Tenant(context.Context, string) (Tenant, error)
	CustomerWallet(context.Context, string) (Tenant, error)
	Entries(context.Context, string, Filter) ([]Entry, error)
	Orders(context.Context, string, Filter) (OrderPage, error)
	OrderDetail(context.Context, string, int64) (Order, error)
	CreateRechargeQuote(context.Context, RechargeQuote) error
	RechargeQuote(context.Context, string, string, string, string) (RechargeQuote, error)
	ApplyRecharge(context.Context, Command) (Entry, error)
	PurchasePackage(context.Context, string, PackagePurchase) (Order, error)
}

func IsKind(kind string) bool {
	return kind == TrafficPackage || kind == StaticPurchase || kind == StaticRenewal
}

func IsStatus(status string) bool {
	return status == Pending || status == Succeeded || status == Partial || status == Failed
}

func (c Command) Validate() error {
	if c.TenantID == "" || len(c.TenantID) > 64 || c.Key == "" || len(c.Key) > 128 ||
		strings.ContainsRune(c.Key+c.Reason+c.TenantID, 0) || c.Amount < 0 {
		return ErrInvalid
	}
	if c.Action == Recharge {
		if c.Actor == "" || strings.TrimSpace(c.Reason) == "" || len(c.Reason) > 500 ||
			c.ExpectedBefore == nil || *c.ExpectedBefore < 0 || c.Amount == 0 ||
			c.OrderID != 0 || c.Kind != "" || len(c.Items)+len(c.ItemDeliveries) != 0 {
			return ErrInvalid
		}
		return nil
	}
	if c.Actor != "" || c.Reason != "" || c.ExpectedBefore != nil {
		return ErrInvalid
	}
	switch c.Action {
	case Debit:
		if c.OrderID == 0 && c.Kind == TrafficPackage && c.validItems() && len(c.ItemDeliveries) == 0 {
			return nil
		}
	case Hold:
		if c.OrderID == 0 && (c.Kind == StaticPurchase || c.Kind == StaticRenewal) &&
			c.validItems() && len(c.ItemDeliveries) == 0 {
			return nil
		}
	case Release:
		if c.OrderID > 0 && c.Kind == "" && len(c.Items) == 0 && len(c.ItemDeliveries) > 0 {
			return nil
		}
	case Refund:
		if c.OrderID > 0 && c.Kind == "" && c.Amount > 0 && len(c.Items)+len(c.ItemDeliveries) == 0 {
			return nil
		}
	}
	return ErrInvalid
}

func (c Command) validItems() bool {
	if c.Amount <= 0 || len(c.Items) == 0 || len(c.Items) > 1000 {
		return false
	}
	remaining := c.Amount
	for _, item := range c.Items {
		quantity := int64(item.Quantity)
		if quantity <= 0 || item.UnitMinor <= 0 || item.UnitMinor > remaining/quantity ||
			item.OrderID != 0 || item.Position != 0 || item.DeliveredCount != 0 ||
			len(item.Specification) > 500 || len(item.Reference) > 128 || strings.ContainsRune(item.Specification+item.Reference, 0) {
			return false
		}
		if c.Kind == TrafficPackage {
			if len(c.Items) != 1 || item.Quantity != 1 || item.CapacityBytes <= 0 || item.TermDays != 0 {
				return false
			}
		} else if item.CapacityBytes != 0 || item.TermDays <= 0 ||
			(c.Kind == StaticRenewal && (item.Reference == "" || item.Quantity != 1)) {
			return false
		}
		remaining -= item.UnitMinor * quantity
	}
	return remaining == 0
}

func balanceAfter(before, delta int64) (int64, error) {
	if delta < -before {
		err := errors.Conflict("WALLET_INSUFFICIENT_FUNDS", "wallet balance is insufficient")
		return 0, err.WithMetadata(map[string]string{"shortfall_minor": strconv.FormatInt(-delta-before, 10)})
	}
	if delta > math.MaxInt64-before {
		return 0, ErrOverflow
	}
	return before + delta, nil
}

type Settlement struct {
	AfterMinor int64
	Order      Order
}

// Settle calculates the wallet and order changes using trusted state read under
// the wallet lock. The repository validates the command before reaching this step.
func (c Command) Settle(before int64, order Order) (Settlement, error) {
	var delta int64
	switch c.Action {
	case Recharge:
		if !c.QuoteExpiresAt.IsZero() && !time.Now().Before(c.QuoteExpiresAt) {
			return Settlement{}, ErrQuoteExpired
		}
		if before != *c.ExpectedBefore {
			return Settlement{}, ErrBalanceChanged
		}
		delta = c.Amount
	case Debit, Hold:
		status := Succeeded
		if c.Action == Hold {
			status = Pending
		}
		order = Order{
			Kind: c.Kind, Status: status, LockedMinor: c.Amount, DebitedMinor: c.Amount,
			Items: append([]OrderItem(nil), c.Items...),
		}
		if c.Action == Debit {
			order.Items[0].DeliveredCount = 1
			order.DeliveredBytes = order.Items[0].CapacityBytes
		}
		delta = -c.Amount
	case Release:
		if order.Status != Pending || len(c.ItemDeliveries) != len(order.Items) {
			return Settlement{}, ErrState
		}
		var retained int64
		order.DeliveredCount = 0
		for i, count := range c.ItemDeliveries {
			item := &order.Items[i]
			if count < 0 || count > item.Quantity {
				return Settlement{}, ErrState
			}
			item.DeliveredCount = count
			order.DeliveredCount += int64(count)
			retained += int64(count) * item.UnitMinor
		}
		if retained != c.Amount {
			return Settlement{}, ErrState
		}
		order.Status = Partial
		switch c.Amount {
		case 0:
			order.Status = Failed
		case order.DebitedMinor:
			order.Status = Succeeded
		}
		order.ReleasedMinor = order.DebitedMinor - c.Amount
		delta = order.ReleasedMinor
	case Refund:
		if (order.Status != Succeeded && order.Status != Partial) ||
			c.Amount > order.DebitedMinor-order.ReleasedMinor-order.RefundedMinor {
			return Settlement{}, ErrState
		}
		order.RefundedMinor += c.Amount
		delta = c.Amount
	default:
		return Settlement{}, ErrInvalid
	}
	after, err := balanceAfter(before, delta)
	if err != nil {
		return Settlement{}, err
	}
	return Settlement{AfterMinor: after, Order: order}, nil
}
