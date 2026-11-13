package usage

import (
	"context"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/go-kratos/kratos/v3/errors"
)

type Scope struct {
	Type, ID string
}

type Settings struct {
	Configured Threshold
	Effective  *Threshold
}

var groupIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Limit configuration to 32-bit thousandths so routine Recorder deltas
// cannot overflow billable bytes because of an extreme configured multiplier.
const maxRateMilli = math.MaxUint32

func (s *Service) validateScope(tx Transaction, scope *Scope) error {
	switch scope.Type {
	case "global":
		if scope.ID == "" || scope.ID == "_" {
			scope.ID = ""
			return nil
		}
	case "tenant":
		if s.access.HasTenant(scope.ID) {
			return nil
		}
	case "group":
		if groupIDPattern.MatchString(scope.ID) {
			exists, err := tx.GroupExists(scope.ID)
			if err != nil {
				return err
			}
			if exists {
				return nil
			}
		}
	}
	return errors.BadRequest("INVALID_SCOPE", "provide a global scope or an existing tenant or group")
}

func (s *Service) BillingSettings(ctx context.Context, token string, scope Scope, tenant string, update *Threshold) (Settings, error) {
	permission := access.ReadOperations
	if update != nil {
		permission = access.ManageBilling
	}
	actor, err := s.access.RequireActor(token, "", permission)
	if err != nil {
		return Settings{}, err
	}
	if tenant == "" && scope.Type == "tenant" {
		tenant = scope.ID
	}
	if tenant != "" && !s.access.HasTenant(tenant) {
		return Settings{}, errors.BadRequest("INVALID_TENANT", "unknown tenant_id")
	}
	var result Settings
	err = s.repo.Transact(ctx, func(tx Transaction) error {
		// Serialize settings and membership writes through validation and commit.
		if err := s.validateScope(tx, &scope); err != nil {
			return err
		}
		if err := tx.LockBillingConfig(update != nil); err != nil {
			return err
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		if update != nil {
			if err := tx.SaveBillingSettings(scope, *update, actor, now); err != nil {
				return err
			}
			ids := s.access.TenantIDs()
			if scope.Type == "tenant" {
				ids = []string{scope.ID}
			}
			if scope.Type != "tenant" {
				settings := []Threshold{*update}
				if scope.Type == "group" {
					global, err := tx.BillingSettings(Scope{Type: "global"})
					if err != nil {
						return err
					}
					settings = append(settings, global)
				}
				if err := validateThresholds(settings); err != nil {
					return err
				}
			}
			for _, id := range ids {
				settings, err := tx.Thresholds(id, now)
				if err != nil {
					return err
				}
				if err := validateThresholds(settings); err != nil {
					return err
				}
			}
		}
		result.Configured, err = tx.BillingSettings(scope)
		if err != nil || tenant == "" {
			return err
		}
		settings, err := tx.Thresholds(tenant, now)
		if err != nil {
			return err
		}
		admission, warning := resolveThresholds(settings)
		result.Effective = &Threshold{AdmissionThresholdBytes: &admission, WarningThresholdBytes: &warning}
		return nil
	})
	if err == nil && update != nil {
		s.invalidateBillingBalances(scope)
	}
	return result, err
}

func validateThresholds(settings []Threshold) error {
	admission, warning := resolveThresholds(settings)
	if warning <= admission {
		return errors.BadRequest("INVALID_THRESHOLDS", "effective warning threshold must exceed the admission threshold")
	}
	return nil
}

func (s *Service) invalidateBillingBalances(scope Scope) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if scope.Type == "tenant" {
		delete(s.cache, scope.ID)
		return
	}
	clear(s.cache)
}

func (s *Service) CreateBillingRate(ctx context.Context, token string, scope Scope, upload, download *uint64, effective *time.Time) (RateVersion, error) {
	actor, err := s.access.RequireActor(token, "", access.ManageBilling)
	if err != nil {
		return RateVersion{}, err
	}
	if upload == nil || download == nil || *upload > maxRateMilli || *download > maxRateMilli {
		return RateVersion{}, errors.BadRequest("INVALID_RATE", "provide both ratios in thousandths from 0 to 4294967295")
	}
	var result RateVersion
	err = s.repo.Transact(ctx, func(tx Transaction) error {
		if err := tx.LockBillingConfig(true); err != nil {
			return err
		}
		if err := s.validateScope(tx, &scope); err != nil {
			return err
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		at := now
		if effective != nil {
			at = effective.UTC().Truncate(time.Microsecond)
			if at.Before(now) {
				return errors.BadRequest("INVALID_EFFECTIVE_FROM", "effective_from cannot be in the past")
			}
		}
		result = RateVersion{ScopeType: scope.Type, ScopeID: scope.ID, EffectiveFrom: at,
			Rate: Rate{UploadRatioMilli: int64(*upload), DownloadRatioMilli: int64(*download)}}
		id, err := tx.InsertBillingRate(result, actor, now)
		if err == nil && id == 0 {
			return errors.Conflict("RATE_VERSION_EXISTS", "a rate already exists for this scope and time")
		}
		result.ID = id
		return err
	})
	return result, err
}

func (s *Service) CreateTenantGroup(ctx context.Context, token, id, name string) error {
	if err := s.access.Require(token, "", access.ManageBilling); err != nil {
		return err
	}
	if !groupIDPattern.MatchString(id) || strings.TrimSpace(name) == "" || len(name) > 256 || strings.ContainsRune(name, 0) {
		return errors.BadRequest("INVALID_GROUP", "provide a valid group_id and name")
	}
	return s.repo.Transact(ctx, func(tx Transaction) error {
		created, err := tx.InsertGroup(id, name, time.Now().UTC())
		if err == nil && !created {
			return errors.Conflict("GROUP_EXISTS", "group_id already exists")
		}
		return err
	})
}

func (s *Service) SetTenantGroup(ctx context.Context, token, tenant, group string) (time.Time, error) {
	actor, err := s.access.RequireActor(token, "", access.ManageBilling)
	if err != nil {
		return time.Time{}, err
	}
	if !s.access.HasTenant(tenant) {
		return time.Time{}, errors.BadRequest("INVALID_TENANT", "unknown tenant_id")
	}
	var at time.Time
	err = s.repo.Transact(ctx, func(tx Transaction) error {
		if err := tx.LockBillingConfig(true); err != nil {
			return err
		}
		if group != "" {
			scope := Scope{Type: "group", ID: group}
			if err := s.validateScope(tx, &scope); err != nil {
				return err
			}
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		at, err = tx.AppendGroupMembership(tenant, group, actor, now)
		if err != nil {
			return err
		}
		settings, err := tx.Thresholds(tenant, at)
		if err != nil {
			return err
		}
		return validateThresholds(settings)
	})
	if err == nil {
		s.invalidateBillingBalances(Scope{Type: "tenant", ID: tenant})
	}
	return at, err
}
