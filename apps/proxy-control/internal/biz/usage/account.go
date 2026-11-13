package usage

import (
	"math"
	"time"
)

type Grant struct {
	ID                   int64
	GrantedBytes         int64
	UsedBytes            int64
	OverdraftOffsetBytes int64
	StartsAt             time.Time
	ExpiresAt            *time.Time
	ActivatedAt          *time.Time
	Source               string
	dirty                bool
}

func (g *Grant) activeAt(at time.Time) bool {
	return !g.StartsAt.After(at) && (g.ExpiresAt == nil || g.ExpiresAt.After(at))
}

type Account struct {
	ID             string
	Overdraft      int64
	WarningSince   *time.Time
	LastReceivedAt *time.Time
	Admission      int64
	Warning        int64
	Grants         []Grant
}

func loadAccount(tx Transaction, id string, now time.Time) (*Account, error) {
	a, err := tx.LoadAccount(id, now)
	if err != nil {
		return nil, err
	}
	if err := loadThresholds(tx, &a, now); err != nil {
		return nil, err
	}
	return &a, a.activate(tx, now)
}

func loadThresholds(tx Transaction, a *Account, now time.Time) error {
	settings, err := tx.Thresholds(a.ID, now)
	if err != nil {
		return err
	}
	a.Admission, a.Warning = resolveThresholds(settings)
	return nil
}

func resolveThresholds(settings []Threshold) (admission, warning int64) {
	admission, warning = 10*1024*1024, 1000*1024*1024
	var admissionSet, warningSet bool
	for _, setting := range settings {
		if !admissionSet && setting.AdmissionThresholdBytes != nil {
			admission, admissionSet = *setting.AdmissionThresholdBytes, true
		}
		if !warningSet && setting.WarningThresholdBytes != nil {
			warning, warningSet = *setting.WarningThresholdBytes, true
		}
	}
	return admission, warning
}

func (a *Account) available(now time.Time) (int64, error) {
	total := -a.Overdraft
	for _, g := range a.Grants {
		if g.activeAt(now) && g.ActivatedAt != nil {
			remaining := g.GrantedBytes - g.UsedBytes
			if total > math.MaxInt64-remaining {
				return 0, errByteOverflow
			}
			total += remaining
		}
	}
	return total, nil
}

func (a *Account) activate(tx Transaction, now time.Time) error {
	// Activation is chronological, independently of the expiry order used for allocation.
	var due []*Grant
	for i := range a.Grants {
		g := &a.Grants[i]
		if !g.StartsAt.After(now) && g.ActivatedAt == nil {
			due = append(due, g)
		}
	}
	sortActivations(due)
	for _, g := range due {
		if !g.activeAt(now) {
			// Keep the grant available to historical samples without offsetting
			// current debt after the whole activation window was missed.
			g.ActivatedAt = &now
			g.dirty = true
			continue
		}
		before, err := a.available(now)
		if err != nil {
			return err
		}
		offset := a.activateGrant(g, now)
		if offset > 0 {
			after, err := a.available(now)
			if err != nil {
				return err
			}
			if err := tx.WriteLedger(a.ID, g.ID, "overdraft_offset", -offset, before, after, nil, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Account) activateGrant(g *Grant, now time.Time) int64 {
	offset := min(a.Overdraft, g.GrantedBytes-g.UsedBytes)
	a.Overdraft -= offset
	g.UsedBytes += offset
	g.OverdraftOffsetBytes += offset
	g.ActivatedAt = &now
	g.dirty = true
	return offset
}

func (a *Account) save(tx Transaction, now time.Time) error {
	available, err := a.available(now)
	if err != nil {
		return err
	}
	if available >= a.Warning {
		a.WarningSince = nil
	} else if a.WarningSince == nil {
		a.WarningSince = &now
	}
	for _, g := range a.Grants {
		if !g.dirty {
			continue
		}
		if err := tx.SaveGrant(g); err != nil {
			return err
		}
	}
	return tx.SaveAccount(*a, now)
}

func (a *Account) correctExpiry(tx Transaction, g *Grant, debit int64, now time.Time) error {
	if g.ExpiresAt == nil || g.ExpiresAt.After(now) {
		return nil
	}
	original, found, err := tx.ExpiryEntry(g.ID)
	if err != nil || !found {
		return err
	}
	available, err := a.available(now)
	if err != nil {
		return err
	}
	return tx.WriteLedger(a.ID, g.ID, "grant_expire_correction", debit, available, available, &original, now)
}
