// Package ipcrown implements the static residential IP supplier contract.
package ipcrown

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"github.com/google/uuid"
)

const (
	staticCategory = 200001
	sharedCategory = 28
)

func categoryFor(shareMode string) int {
	if shareMode == "shared" {
		return sharedCategory
	}
	return staticCategory
}

type Client struct {
	baseURL, partner, secret string
	http                     *http.Client
}

func New(baseURL, partner, secret string) wallet.StaticSupplier {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || partner == "" || secret == "" {
		return nil
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), partner: partner, secret: secret, http: &http.Client{
		Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

type price struct {
	Term   string          `json:"price_type"`
	Months int32           `json:"buytime"`
	Amount json.RawMessage `json:"price"`
}

type node struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	ISP       string  `json:"isp"`
	Continent string  `json:"continent"`
	Line      string  `json:"line"`
	Available int32   `json:"max"`
	Prices    []price `json:"prices"`
}

func (c *Client) Offers(ctx context.Context, shareMode string, purposeID int32) ([]wallet.StaticOffer, error) {
	if shareMode == "" {
		shareMode = "exclusive"
	}
	params := map[string]any{"cid": categoryFor(shareMode)}
	if purposeID != 0 {
		params["pid"] = int(purposeID)
	}
	var catalog struct{ Nodes []node }
	code, err := c.call(ctx, "/nodes-and-continents", params, &catalog)
	if err != nil || code != 0 {
		return nil, wallet.ErrStaticUnavailable
	}
	var offers []wallet.StaticOffer
	for _, node := range catalog.Nodes {
		for _, price := range node.Prices {
			amount, err := amountMinor(price.Amount)
			days := termDays(price)
			if err != nil || amount <= 0 || node.ID <= 0 || days <= 0 || price.Term == "" {
				continue
			}
			offers = append(offers, wallet.StaticOffer{NodeID: node.ID, Region: node.Title, IPType: node.ISP,
				Term: price.Term, TermDays: days, UnitMinor: amount, Available: max(0, node.Available),
				ShareMode: shareMode, PurposeID: purposeID, Continent: node.Continent, Line: node.Line})
		}
	}
	return offers, nil
}

func termDays(price price) int32 {
	switch price.Term {
	case "test2h":
		return 0
	case "rp":
		return 1
	case "zp":
		return 7
	case "np":
		return 360
	default:
		if price.Months < 1 || price.Months > 120 {
			return 0
		}
		return price.Months * 30
	}
}

func amountMinor(value json.RawMessage) (int64, error) {
	whole, fraction, _ := strings.Cut(strings.Trim(string(value), "\""), ".")
	if whole == "" || len(fraction) > 2 || strings.HasPrefix(whole, "-") {
		return 0, wallet.ErrStaticUnavailable
	}
	return strconv.ParseInt(whole+fraction+strings.Repeat("0", 2-len(fraction)), 10, 64)
}

type supplierID string

func (id *supplierID) UnmarshalJSON(data []byte) error {
	stringID := strings.Trim(string(data), "\"")
	if stringID == "null" || stringID == "0" {
		stringID = ""
	}
	*id = supplierID(stringID)
	return nil
}

type allocation struct {
	ID            supplierID `json:"id"`
	NodeID        int64      `json:"nid"`
	ProxyID       supplierID `json:"proxyid"`
	DetailProxyID supplierID `json:"proxy_id"`
	IP            string     `json:"ip"`
	Port          int32      `json:"port"`
	Username      string     `json:"p_user"`
	Password      string     `json:"p_pass"`
	Expires       string     `json:"stoptime"`
	Status        int        `json:"status"`
}

type delivery struct {
	OrderID supplierID   `json:"order_id"`
	Items   []allocation `json:"list"`
	Details []allocation `json:"proxy_list"`
}

func (c *Client) Place(ctx context.Context, p wallet.StaticPurchaseRequest) (wallet.StaticDelivery, error) {
	type supplierPurchaseLine struct {
		NodeID   int64 `json:"nid"`
		Quantity int32 `json:"number"`
	}
	lines := p.LineItems()
	list := make([]supplierPurchaseLine, 0, len(lines))
	for _, line := range lines {
		list = append(list, supplierPurchaseLine{NodeID: line.NodeID, Quantity: line.Quantity})
	}
	var result delivery
	params := map[string]any{"cid": categoryFor(p.ShareMode), "time": p.Term, "list": list}
	if p.PurposeID != 0 {
		params["pid"] = int(p.PurposeID)
	}
	code, err := c.call(ctx, "/order/place", params, &result)
	if err != nil {
		return wallet.StaticDelivery{}, err
	}
	if code == -5 {
		return wallet.StaticDelivery{Rejected: true}, nil
	}
	if code != 0 || result.OrderID == "" {
		return wallet.StaticDelivery{}, wallet.ErrStaticUnavailable
	}
	return parseDelivery(result.OrderID, result.Items, p.NodeID), nil
}

func (c *Client) Detail(ctx context.Context, orderID string) (wallet.StaticDelivery, error) {
	var result delivery
	code, err := c.call(ctx, "/order/detail", map[string]any{"order_id": orderID}, &result)
	if err != nil || code != 0 || string(result.OrderID) != orderID {
		return wallet.StaticDelivery{}, wallet.ErrStaticUnavailable
	}
	return parseDelivery(result.OrderID, result.Details, 0), nil
}

func parseDelivery(id supplierID, items []allocation, nodeID int64) wallet.StaticDelivery {
	result := wallet.StaticDelivery{SupplierOrderID: string(id)}
	// The supplier returns a wall-clock timestamp without an offset.

	for _, ip := range items {
		expires, err := parseSupplierExpiry(ip.Expires)
		_, addressErr := netip.ParseAddr(ip.IP)
		if err != nil || addressErr != nil || ip.ID == "" || ip.Port <= 0 || ip.Port > 65535 ||
			ip.Username == "" || ip.Password == "" || ip.Status == 2 || (nodeID != 0 && ip.NodeID != nodeID) ||
			strings.ContainsAny(ip.Username+ip.Password, "\r\n\x00") {
			continue
		}
		proxyID := ip.ProxyID
		if proxyID == "" {
			proxyID = ip.DetailProxyID
		}
		result.Allocations = append(result.Allocations, wallet.StaticAllocation{NodeID: ip.NodeID, SupplierItemID: string(ip.ID), ProxyID: string(proxyID),
			IP: ip.IP, Port: ip.Port, Username: ip.Username, Password: ip.Password, ExpiresAt: expires})
	}
	return result
}

func (c *Client) call(ctx context.Context, path string, params map[string]any, result any) (int, error) {
	code, _, err := c.callWithStatus(ctx, path, params, result)
	return code, err
}

func (c *Client) callWithStatus(ctx context.Context, path string, params map[string]any, result any) (int, int, error) {
	return c.callWithCount(ctx, path, params, result, nil)
}

func (c *Client) callWithCount(ctx context.Context, path string, params map[string]any, result any, count **int) (int, int, error) {
	params["timestamp"] = time.Now().Unix()
	params["nonce"] = strings.ReplaceAll(uuid.NewString(), "-", "")
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		value := params[key]
		switch typed := value.(type) {
		case string, int, int64:
			parts = append(parts, fmt.Sprintf("%s=%v", key, typed))
		default:
			encoded, err := json.Marshal(value)
			if err != nil {
				return 0, 0, wallet.ErrStaticUnavailable
			}
			parts = append(parts, key+"="+string(encoded))
		}
	}
	parts = append(parts, "key="+c.secret)
	// IPCrown requires uppercase MD5 for signing its requests.
	sum := md5.Sum([]byte(strings.Join(parts, "&"))) //nolint:gosec // Vendor protocol.
	params["sign"] = strings.ToUpper(hex.EncodeToString(sum[:]))
	body, err := json.Marshal(params)
	if err != nil {
		return 0, 0, wallet.ErrStaticUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/open/api"+path, bytes.NewReader(body))
	if err != nil {
		return 0, 0, wallet.ErrStaticUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Partner-ID", c.partner)
	response, err := c.http.Do(request)
	if err != nil {
		return 0, 0, wallet.ErrStaticUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	var envelope struct {
		Code  *int
		Count *int
		Data  json.RawMessage
	}
	const limit = 4 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(data) > limit || json.Unmarshal(data, &envelope) != nil || envelope.Code == nil {
		return 0, 0, wallet.ErrStaticUnavailable
	}
	if *envelope.Code != 0 {
		return *envelope.Code, response.StatusCode, nil
	}
	if response.StatusCode != http.StatusOK || len(envelope.Data) == 0 || json.Unmarshal(envelope.Data, result) != nil {
		return 0, 0, wallet.ErrStaticUnavailable
	}
	if count != nil {
		*count = envelope.Count
	}
	return 0, response.StatusCode, nil
}

func parseSupplierExpiry(value string) (time.Time, error) {
	return time.ParseInLocation(time.DateTime, value, time.FixedZone("CST", 8*60*60))
}
