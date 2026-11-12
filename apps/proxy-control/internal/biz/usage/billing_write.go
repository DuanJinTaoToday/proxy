package usage

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

type BalanceChange struct {
	Kind, ExternalRef, Reason string
	Bytes                     uint64
	GrantID                   int64
	StartsAt, ExpiresAt       *time.Time
}

func (s *Service) ChangeBalance(ctx context.Context, token, tenant string, change BalanceChange) (LedgerEntry, error) {
	actor, err := s.access.RequireActor(token, tenant, access.ManageBilling)
	if err != nil {
		return LedgerEntry{}, err
	}
	if strings.TrimSpace(change.ExternalRef) == "" || len(change.ExternalRef) > 256 || change.Bytes == 0 || change.Bytes > math.MaxInt64 ||
		strings.ContainsRune(change.ExternalRef, 0) || strings.ContainsRune(change.Reason, 0) {
		return LedgerEntry{}, errors.BadRequest("INVALID_BALANCE_CHANGE", "provide an external reference of at most 256 bytes and a positive byte amount")
	}
	switch change.Kind {
	case "recharge", "adjust_add":
		if change.GrantID != 0 {
			return LedgerEntry{}, errors.BadRequest("INVALID_GRANT_ID", "grant_id is only allowed for subtraction")
		}
	case "adjust_subtract":
		if change.GrantID <= 0 {
			return LedgerEntry{}, errors.BadRequest("INVALID_GRANT_ID", "subtraction requires a grant_id")
		}
	default:
		return LedgerEntry{}, errors.BadRequest("INVALID_ADJUSTMENT_MODE", "choose ADD or SUBTRACT")
	}
	var result LedgerEntry
	err = s.repo.Transact(ctx, func(tx Transaction) error {
		now := time.Now().UTC().Truncate(time.Microsecond)
		a, err := tx.LoadAccount(tenant, now)
		if err != nil {
			return err
		}
		result, err = tx.FindBalanceChange(tenant, change.ExternalRef)
		if err != nil || result.ID != 0 {
			return err
		}
		now = time.Now().UTC().Truncate(time.Microsecond)
		if err := loadThresholds(tx, &a, now); err != nil {
			return err
		}
		if err := a.activate(tx, now); err != nil {
			return err
		}
		before, err := a.available(now)
		if err != nil {
			return err
		}
		entry := LedgerEntry{Kind: change.Kind, ExternalRef: change.ExternalRef, Reason: change.Reason,
			Operator: actor, CreatedAt: now, AvailableBefore: before, DeltaBytes: int64(change.Bytes)}
		var offset int64
		if change.Kind == "adjust_subtract" {
			addedDebt, err := subtractGrant(tx, &a, change.GrantID, int64(change.Bytes), now)
			if err != nil {
				return err
			}
			entry.OverdraftAddedBytes = &addedDebt
			entry.GrantID, entry.DeltaBytes = change.GrantID, -entry.DeltaBytes
		} else {
			grant, err := newGrant(change, now)
			if err != nil {
				return err
			}
			if err := a.checkGrantCapacity(grant, now); err != nil {
				return err
			}
			if grant.activeAt(now) {
				offset = a.activateGrant(&grant, now)
				grant.dirty = false // CreateGrant persists activation together with the new grant.
			}
			grant.ID, err = tx.CreateGrant(tenant, actor, grant, now)
			if err != nil {
				return err
			}
			entry.GrantID = grant.ID
			a.Grants = append(a.Grants, grant)
		}
		entry.AvailableAfter, err = a.available(now)
		if err != nil {
			return err
		}
		result, err = tx.WriteBalanceChange(tenant, entry)
		if err != nil {
			return err
		}
		if offset > 0 {
			if err := tx.WriteLedger(tenant, entry.GrantID, "overdraft_offset", -offset, entry.AvailableAfter, entry.AvailableAfter, nil, now); err != nil {
				return err
			}
		}
		return a.save(tx, now)
	})
	if err == errByteOverflow {
		return LedgerEntry{}, errors.BadRequest("BALANCE_OVERFLOW", "resulting balance exceeds storage range")
	}
	if err != nil {
		return LedgerEntry{}, err
	}
	s.refreshBalances(ctx, []string{tenant})
	return result, nil
}

func newGrant(change BalanceChange, now time.Time) (Grant, error) {
	g := Grant{GrantedBytes: int64(change.Bytes), StartsAt: now, Source: "adjustment"}
	if change.Kind == "recharge" {
		g.Source = "recharge"
	}
	if change.StartsAt != nil {
		g.StartsAt = change.StartsAt.UTC().Truncate(time.Microsecond)
	}
	if change.ExpiresAt != nil {
		expiry := change.ExpiresAt.UTC().Truncate(time.Microsecond)
		if !expiry.After(g.StartsAt) || !expiry.After(now) {
			return Grant{}, errors.BadRequest("INVALID_GRANT_WINDOW", "expires_at must be after starts_at and the current time")
		}
		g.ExpiresAt = &expiry
	}
	return g, nil
}

func (a *Account) checkGrantCapacity(grant Grant, now time.Time) error {
	// The overlapping total can increase only when a grant starts. Existing
	// debt may be repaid before then, so it cannot provide storage headroom.
	start := now
	if grant.StartsAt.After(start) {
		start = grant.StartsAt
	}
	starts := []time.Time{start}
	for _, existing := range a.Grants {
		if existing.StartsAt.After(starts[0]) && grant.activeAt(existing.StartsAt) {
			starts = append(starts, existing.StartsAt)
		}
	}
	for _, at := range starts {
		total := grant.GrantedBytes
		for _, existing := range a.Grants {
			if !existing.activeAt(at) {
				continue
			}
			remaining := existing.GrantedBytes - existing.UsedBytes
			if total > math.MaxInt64-remaining {
				return errByteOverflow
			}
			total += remaining
		}
	}
	return nil
}

func subtractGrant(tx Transaction, a *Account, id, amount int64, now time.Time) (int64, error) {
	for i := range a.Grants {
		g := &a.Grants[i]
		if g.ID != id || (g.ExpiresAt != nil && !now.Before(*g.ExpiresAt)) {
			continue
		}
		debit := min(amount, g.GrantedBytes-g.UsedBytes)
		debt := amount - debit
		if a.Overdraft > math.MaxInt64-debt {
			return 0, errByteOverflow
		}
		g.GrantedBytes -= debit
		a.Overdraft += debt
		return debt, tx.SaveGrantAmount(*g)
	}
	return 0, errors.NotFound("GRANT_NOT_FOUND", "unexpired grant not found for tenant")
}
