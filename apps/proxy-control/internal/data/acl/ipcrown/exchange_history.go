package ipcrown

import (
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
)

type exchangeHistoryRow struct {
	ID      supplierID `json:"id"`
	OrderID supplierID `json:"proxyindex_id"`
	ItemID  supplierID `json:"proxylist_id"`
	OldNode int64      `json:"old_nid"`
	Node    int64      `json:"nid"`
	OldIP   string     `json:"old_ip"`
	IP      string     `json:"ip"`
	Created string     `json:"create_time"`
}

func (c *Client) ExchangeHistory(ctx context.Context, oldIP string, around time.Time) ([]wallet.StaticExchangeHistoryRecord, error) {
	day := around.In(time.FixedZone("CST", 8*60*60))
	var data json.RawMessage
	var count *int
	const pageSize = 100
	code, status, err := c.callWithCount(ctx, "/order/exchange-records", map[string]any{
		"ip": oldIP, "page": 1, "limit": pageSize,
		"start_time": day.AddDate(0, 0, -1).Format(time.DateOnly), "end_time": day.AddDate(0, 0, 1).Format(time.DateOnly),
	}, &data, &count)
	if err != nil || code != 0 || status != http.StatusOK || count == nil || *count < 0 || *count > pageSize {
		return nil, wallet.ErrStaticUnavailable
	}
	rows := decodeExchangeHistoryRows(data)
	if rows == nil || len(rows) != *count {
		return nil, wallet.ErrStaticUnavailable
	}
	seen := make(map[string]bool, len(rows))
	history := make([]wallet.StaticExchangeHistoryRecord, 0, len(rows))
	for _, row := range rows {
		id := string(row.ID)
		numericID, idErr := strconv.ParseInt(id, 10, 64)
		created, timeErr := parseSupplierExpiry(row.Created)
		ip, ipErr := netip.ParseAddr(row.IP)
		oldIP, oldIPErr := netip.ParseAddr(row.OldIP)
		if idErr != nil || numericID <= 0 || seen[id] || timeErr != nil || ipErr != nil || oldIPErr != nil ||
			ip.Zone() != "" || oldIP.Zone() != "" || row.OrderID == "" || row.ItemID == "" || row.OldNode <= 0 || row.Node <= 0 {
			return nil, wallet.ErrStaticUnavailable
		}
		seen[id] = true
		history = append(history, wallet.StaticExchangeHistoryRecord{
			ID: id, SupplierOrderID: string(row.OrderID), SupplierItemID: string(row.ItemID),
			OldIP: oldIP.Unmap().String(), IP: ip.Unmap().String(), OldNodeID: row.OldNode, NodeID: row.Node, CreatedAt: created,
		})
	}
	return history, nil
}

func decodeExchangeHistoryRows(data json.RawMessage) []exchangeHistoryRow {
	var rows []exchangeHistoryRow
	if json.Unmarshal(data, &rows) == nil {
		return rows
	}
	var envelope map[string][]exchangeHistoryRow
	if json.Unmarshal(data, &envelope) != nil {
		return nil
	}
	for _, field := range []string{"list", "rows", "data", "records"} {
		if rows := envelope[field]; rows != nil {
			return rows
		}
	}
	return nil
}
