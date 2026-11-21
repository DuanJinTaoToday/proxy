package wallet

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v3/errors"
)

func validStaticCatalog(shareMode string, purposeID int32) bool {
	switch shareMode {
	case "", "exclusive":
		return purposeID == 0
	case "shared":
		return purposeID == 0 || purposeID == 8 || purposeID == 9 || purposeID == 10
	default:
		return false
	}
}

type sharedStaticSupplier interface {
	OffersFor(context.Context, string, int32) ([]StaticOffer, error)
	RenewShared(context.Context, []string, string) (SupplierRenewal, error)
}

func SupplierStaticOffers(ctx context.Context, supplier StaticSupplier, shareMode string, purposeID int32) ([]StaticOffer, error) {
	if shareMode != "shared" {
		return supplier.Offers(ctx)
	}
	if shared, ok := supplier.(sharedStaticSupplier); ok {
		return shared.OffersFor(ctx, shareMode, purposeID)
	}
	return nil, ErrStaticUnavailable
}

func SupplierStaticRenew(ctx context.Context, supplier StaticSupplier, ids []string, term, shareMode string) (SupplierRenewal, error) {
	if shareMode != "shared" {
		return supplier.Renew(ctx, ids, term)
	}
	if shared, ok := supplier.(sharedStaticSupplier); ok {
		return shared.RenewShared(ctx, ids, term)
	}
	return SupplierRenewal{}, ErrStaticUnavailable
}

func (p StaticPurchaseRequest) OfferReference() string {
	if p.ShareMode == "shared" {
		return fmt.Sprintf("%d/%s/shared/%d", p.NodeID, p.Term, p.PurposeID)
	}
	return fmt.Sprintf("%d/%s", p.NodeID, p.Term)
}

func ParseStaticPurchaseReference(reference string) (nodeID int64, shareMode string, purposeID int32, ok bool) {
	parts := strings.Split(reference, "/")
	if len(parts) != 2 && len(parts) != 4 {
		return 0, "", 0, false
	}
	nodeID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || nodeID <= 0 || parts[1] == "" {
		return 0, "", 0, false
	}
	if len(parts) == 2 {
		return nodeID, "", 0, true
	}
	purpose, err := strconv.ParseInt(parts[3], 10, 32)
	if err != nil || parts[2] != "shared" || !validStaticCatalog(parts[2], int32(purpose)) {
		return 0, "", 0, false
	}
	return nodeID, parts[2], int32(purpose), true
}

func CheckStaticStock(quoted, current int32) error {
	if quoted == 0 || quoted == current {
		return nil
	}
	return errors.Conflict("STATIC_STOCK_CHANGED", "confirm the current stock").WithMetadata(map[string]string{
		"available": strconv.FormatInt(int64(current), 10),
	})
}
