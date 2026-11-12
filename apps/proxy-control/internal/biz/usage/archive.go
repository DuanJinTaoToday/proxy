package usage

import (
	"context"
	"time"
)

func (s *Service) Archive(ctx context.Context, now time.Time) error {
	cutoff := now.In(shanghai).AddDate(0, 0, -60).Format(time.DateOnly)
	return s.repo.Transact(ctx, func(tx Transaction) error {
		before, err := tx.LockArchive()
		if err != nil || cutoff <= before {
			return err
		}
		dates, err := tx.DatesBefore(cutoff)
		if err != nil {
			return err
		}
		for _, date := range dates {
			if err := tx.ArchiveDay(date); err != nil {
				return err
			}
		}
		return tx.SetArchiveCutoff(cutoff)
	})
}

func (s *Service) SettleGrants(ctx context.Context, now time.Time) error {
	ids, err := s.repo.DueTenants(ctx, now)
	if err != nil {
		return err
	}
	for _, id := range ids {
		err := s.repo.Transact(ctx, func(tx Transaction) error {
			a, err := loadAccount(tx, id, now)
			if err != nil {
				return err
			}
			available, err := a.available(now)
			if err != nil {
				return err
			}
			for _, g := range a.Grants {
				if g.ExpiresAt == nil || g.ExpiresAt.After(now) {
					continue
				}
				if err := tx.WriteExpiry(id, g.ID, g.UsedBytes-g.GrantedBytes, available, now); err != nil {
					return err
				}
			}
			return a.save(tx, now)
		})
		if err != nil {
			return err
		}
		s.refreshBalances(ctx, []string{id})
	}
	return nil
}
