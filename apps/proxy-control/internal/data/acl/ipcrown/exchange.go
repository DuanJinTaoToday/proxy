package ipcrown

import (
	"context"
	"net/http"
	"strconv"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
)

func (c *Client) Exchange(ctx context.Context, id string, nodeID int64) (wallet.SupplierExchange, error) {
	itemID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || itemID <= 0 || nodeID <= 0 {
		return wallet.SupplierExchange{}, wallet.ErrInvalid
	}
	var rows []allocation
	code, httpStatus, err := c.callWithStatus(ctx, "/order/exchange", map[string]any{
		"type": "same", "ids": []int64{itemID}, "list": []map[string]int64{{"nid": nodeID, "number": 1}}, "isrand": 4,
	}, &rows)
	if err != nil || httpStatus != http.StatusOK {
		return wallet.SupplierExchange{}, wallet.ErrStaticUnavailable
	}
	switch code {
	case -4, -5, -8: // Insufficient supplier funds, no inventory, or rejected by the supplier's concurrency limit.
		return wallet.SupplierExchange{Rejected: true}, nil
	case 0:
		if len(rows) != 1 {
			return wallet.SupplierExchange{}, wallet.ErrStaticUnavailable
		}
		delivery := parseDelivery("", rows, 0)
		if len(delivery.Allocations) == 1 && delivery.Allocations[0].SupplierItemID == id {
			return wallet.SupplierExchange{Allocation: delivery.Allocations[0]}, nil
		}
	}
	return wallet.SupplierExchange{}, wallet.ErrStaticUnavailable
}
