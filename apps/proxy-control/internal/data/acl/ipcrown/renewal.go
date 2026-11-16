package ipcrown

import (
	"context"
	"strconv"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
)

type renewedAllocation struct {
	ID      supplierID `json:"id"`
	Expires string     `json:"stoptime"`
}

func (c *Client) Renew(ctx context.Context, ids []string, term string) (wallet.SupplierRenewal, error) {
	numericIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		value, err := strconv.ParseInt(id, 10, 64)
		if err != nil || value <= 0 {
			return wallet.SupplierRenewal{}, wallet.ErrInvalid
		}
		numericIDs = append(numericIDs, value)
	}
	var rows []renewedAllocation
	code, err := c.call(ctx, "/order/renew", map[string]any{
		"cid": staticCategory, "ids": numericIDs, "time": term,
	}, &rows)
	if err != nil {
		return wallet.SupplierRenewal{}, err
	}
	// Insufficient supplier funds is an explicit refusal. Other batch
	// failures may omit individual results and remain pending verification.
	if code == -4 && len(ids) == 1 {
		return wallet.SupplierRenewal{Rejected: true}, nil
	}
	if code != 0 {
		return wallet.SupplierRenewal{}, wallet.ErrStaticUnavailable
	}
	result := wallet.SupplierRenewal{Expiries: make(map[string]time.Time, len(rows))}
	for _, row := range rows {
		expiry, err := parseSupplierExpiry(row.Expires)
		if err == nil && row.ID != "" {
			result.Expiries[string(row.ID)] = expiry
		}
	}
	return result, nil
}
