package service

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/wallet"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Wallet) GetOperatorStaticIPExchangeQuota(ctx context.Context, req *v1.WalletTenantRequest) (*v1.OperatorStaticIPExchangeQuota, error) {
	noStore(ctx)
	quota, err := s.biz.OperatorExchangeQuota(ctx, bearerToken(ctx), req.GetTenantId())
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	return operatorExchangeQuotaMessage(quota), nil
}

func (s *Wallet) ListOperatorStaticIPExchanges(ctx context.Context, req *v1.OperatorStaticIPExchangesRequest) (*v1.OperatorStaticIPExchanges, error) {
	return s.operatorStaticIPExchanges(ctx, req, false)
}

func (s *Wallet) ListOperatorStaticIPExchangeQueue(ctx context.Context, req *v1.OperatorStaticIPExchangesRequest) (*v1.OperatorStaticIPExchanges, error) {
	return s.operatorStaticIPExchanges(ctx, req, true)
}

func (s *Wallet) operatorStaticIPExchanges(ctx context.Context, req *v1.OperatorStaticIPExchangesRequest, queue bool) (*v1.OperatorStaticIPExchanges, error) {
	noStore(ctx)
	filter := wallet.ExchangeOperationsFilter{TenantID: req.GetTenantId()}
	if cursor := req.GetCursor(); cursor != "" {
		if len(cursor) > 400 {
			return nil, wallet.ErrInvalid
		}
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, wallet.ErrInvalid
		}
		parts := strings.Split(string(raw), "\n")
		if len(parts) != 3 {
			return nil, wallet.ErrInvalid
		}
		filter.BeforeTime, err = time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return nil, wallet.ErrInvalid
		}
		filter.BeforeTenant, filter.BeforeKey = parts[1], parts[2]
	}
	page, err := s.biz.OperatorExchangeHistory(ctx, bearerToken(ctx), filter, queue)
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	response := &v1.OperatorStaticIPExchanges{}
	if len(page.Records) > wallet.PageSize {
		page.Records = page.Records[:wallet.PageSize]
		last := page.Records[len(page.Records)-1]
		response.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(
			last.CreatedAt.Format(time.RFC3339Nano) + "\n" + last.TenantID + "\n" + last.IdempotencyKey))
	}
	for _, record := range page.Records {
		response.Exchanges = append(response.Exchanges, operatorExchangeMessage(record))
	}
	return response, nil
}

func (s *Wallet) ListOperatorStaticIPExchangeAudit(ctx context.Context, req *v1.OperatorStaticIPExchangeAuditRequest) (*v1.OperatorStaticIPExchangeAuditPage, error) {
	noStore(ctx)
	page, err := s.biz.OperatorExchangeAudit(ctx, bearerToken(ctx), req.GetTenantId(), req.GetBeforeId())
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	response := &v1.OperatorStaticIPExchangeAuditPage{}
	if len(page) > wallet.PageSize {
		page = page[:wallet.PageSize]
		response.NextBeforeId = page[len(page)-1].ID
	}
	for _, audit := range page {
		response.Entries = append(response.Entries, operatorExchangeAuditMessage(audit))
	}
	return response, nil
}

func (s *Wallet) AdjustOperatorStaticIPExchangeQuota(ctx context.Context, req *v1.AdjustOperatorStaticIPExchangeQuotaRequest) (*v1.OperatorStaticIPExchangeAuditEntry, error) {
	noStore(ctx)
	audit, err := s.biz.AdjustExchangeQuota(ctx, bearerToken(ctx), wallet.ExchangeQuotaAdjustment{
		TenantID: req.GetTenantId(), Delta: req.GetDelta(), RequestKey: req.GetRequestKey(), Evidence: req.GetEvidence(),
	})
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	return operatorExchangeAuditMessage(audit), nil
}

func (s *Wallet) ResolveOperatorStaticIPExchange(ctx context.Context, req *v1.ResolveOperatorStaticIPExchangeRequest) (*v1.OperatorStaticIPExchange, error) {
	noStore(ctx)
	record, err := s.biz.ResolveExchange(ctx, bearerToken(ctx), wallet.ExchangeOperationsResolution{
		TenantID: req.GetTenantId(), IdempotencyKey: req.GetIdempotencyKey(), RequestKey: req.GetRequestKey(),
		Outcome: req.GetOutcome(), Evidence: req.GetEvidence(), ExpectedIP: req.GetExpectedIp(), Correction: req.GetCorrection(),
	})
	if err != nil {
		return nil, wallet.PublicError(err)
	}
	return operatorExchangeMessage(record), nil
}

func operatorExchangeMessage(record wallet.ExchangeOperationsRecord) *v1.OperatorStaticIPExchange {
	exchange, _ := exchangeMessage(record.StaticExchange, nil)
	message := &v1.OperatorStaticIPExchange{Exchange: exchange, TenantId: record.TenantID,
		SupplierOrderId: record.SupplierOrderID, SupplierItemId: record.SupplierItemID, OldIp: record.OldIP,
		NewIp: record.NewIP, Resolution: record.Resolution, InQueue: record.InQueue,
		ResolutionConflict: record.ResolutionConflict, Corrected: record.Corrected}
	if record.UnknownAt != nil {
		message.UnknownAt = timestamppb.New(*record.UnknownAt)
	}
	return message
}

func operatorExchangeQuotaMessage(quota wallet.ExchangeOperationsQuota) *v1.OperatorStaticIPExchangeQuota {
	return &v1.OperatorStaticIPExchangeQuota{Total: quota.Total, Remaining: quota.Remaining(), Reserved: quota.Reserved, Used: quota.Used}
}

func operatorExchangeAuditMessage(audit wallet.ExchangeOperationsAudit) *v1.OperatorStaticIPExchangeAuditEntry {
	return &v1.OperatorStaticIPExchangeAuditEntry{Id: audit.ID, Kind: audit.Kind, IdempotencyKey: audit.IdempotencyKey,
		RequestKey: audit.RequestKey, ActorId: audit.ActorID, Evidence: audit.Evidence, Outcome: audit.Outcome,
		SupplierRecordId: audit.SupplierRecordID, ObservedIp: audit.ObservedIP, CreatedAt: timestamppb.New(audit.CreatedAt),
		Before: operatorExchangeQuotaMessage(wallet.ExchangeOperationsQuota{Total: audit.QuotaBefore, Reserved: audit.ReservedBefore, Used: audit.UsedBefore}),
		After:  operatorExchangeQuotaMessage(wallet.ExchangeOperationsQuota{Total: audit.QuotaAfter, Reserved: audit.ReservedAfter, Used: audit.UsedAfter}),
	}
}
