package server

import (
	"io"
	stdhttp "net/http"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func ReportingDecoder(maxBytes int64) http.DecodeRequestFunc {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	return func(req *stdhttp.Request, body any) error {
		message, ok := body.(proto.Message)
		switch body.(type) {
		case *v1.StickySessionRequest, *v1.CreateSubaccountRequest, *v1.UpdateSubaccountRequest:
			return decodeLimitedProto(req, message, "SUBACCOUNT_REQUEST", "subaccount request", "invalid subaccount body")
		case *v1.QuoteWalletRechargeRequest, *v1.RechargeWalletRequest, *v1.PurchaseTrafficPackageRequest, *v1.PurchaseStaticIPRequest, *v1.ResolveStaticPurchaseRequest:
			return decodeLimitedProto(req, message, "WALLET_REQUEST", "wallet request", "invalid wallet body")
		case *v1.UpdateTrafficPackageRequest, *v1.SetProductSaleRequest:
			return decodeLimitedProto(req, message, "PRODUCT_REQUEST", "product request", "invalid product configuration body")
		case *v1.SetLowBalanceThresholdRequest:
			return decodeLimitedProto(req, message, "TRAFFIC_ALERT_REQUEST", "traffic alert request", "invalid traffic alert body")
		case *v1.SubmitBatchRequest, *v1.ReportLifecycleRequest, *v1.ReportUnfinishedSessionsRequest, *v1.CompleteReplayRequest:
		default:
			return DecodeProxyAuthRequest(req, body)
		}
		if !ok {
			return DecodeProxyAuthRequest(req, body)
		}
		if req.ContentLength > maxBytes {
			return errors.New(413, "BATCH_TOO_LARGE", "Recorder batch exceeds the body limit")
		}
		data, err := io.ReadAll(io.LimitReader(req.Body, maxBytes+1))
		if int64(len(data)) > maxBytes {
			return errors.New(413, "BATCH_TOO_LARGE", "Recorder batch exceeds the body limit")
		}
		if err != nil || protojson.Unmarshal(data, message) != nil {
			return errors.BadRequest("INVALID_BATCH", "invalid Recorder batch body")
		}
		return nil
	}
}

func decodeLimitedProto(req *stdhttp.Request, message proto.Message, reason, name, invalidMessage string) error {
	const maxBody = 8 << 10
	if req.ContentLength > maxBody {
		return errors.New(413, reason+"_TOO_LARGE", name+" exceeds the body limit")
	}
	data, err := io.ReadAll(io.LimitReader(req.Body, maxBody+1))
	if len(data) > maxBody {
		return errors.New(413, reason+"_TOO_LARGE", name+" exceeds the body limit")
	}
	if err != nil || protojson.Unmarshal(data, message) != nil {
		return errors.BadRequest("INVALID_"+reason, invalidMessage)
	}
	return nil
}

func ControlResponseEncoder(w stdhttp.ResponseWriter, req *stdhttp.Request, response any) error {
	switch response.(type) {
	case *v1.StickySession, *v1.Subaccounts, *v1.Subaccount, *v1.SubaccountPassword, *v1.SubaccountProxyConfig,
		*v1.AuthSnapshot, *v1.RefreshAuthStateResponse, *v1.Products, *v1.Product, *v1.TrafficPackage, *v1.TrafficPackages,
		*v1.WalletTenant, *v1.WalletRechargeQuote, *v1.WalletEntry, *v1.WalletEntries, *v1.PurchaseOrder, *v1.PurchaseOrders,
		*v1.SubmitBatchResponse, *v1.QueryUsageResponse, *v1.ListRecordAnomaliesResponse, *v1.ListUsageGapsResponse,
		*v1.DailyUsageResponse, *v1.TrafficAlert, *v1.StaticOffers, *v1.StaticIPs, *v1.PendingStaticPurchases:
		_, dailyUsage := response.(*v1.DailyUsageResponse)
		data, err := (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: dailyUsage}).Marshal(response.(proto.Message))
		if err != nil {
			return err
		}
		contentType := "application/json"
		if req.Header.Get("Accept") == "application/protojson" {
			contentType = "application/protojson"
		}
		w.Header().Set("Content-Type", contentType)
		_, err = w.Write(data)
		return err
	default:
		return http.DefaultResponseEncoder(w, req, response)
	}
}
