package usage

import (
	"context"
	"time"
)

func (s *Service) Archive(ctx context.Context, now time.Time) error {
	cutoff := usageArchiveCutoff(now).Format(time.DateOnly)
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

func usageArchiveCutoff(now time.Time) time.Time {
	local := now.In(shanghai)
	return time.Date(local.Year(), local.Month(), local.Day()-60, 0, 0, 0, 0, shanghai)
}
