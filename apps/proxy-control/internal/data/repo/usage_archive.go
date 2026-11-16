package repo

import "time"

// Ingestion and partition maintenance acquire this lock before touching any
// usage partition. CREATE/DROP PARTITION locks the parent table.
func (t usageTx) LockArchive() (string, error) {
	var state struct{ BeforeDate string }
	err := t.db.Raw(`SELECT to_char(before_date, 'YYYY-MM-DD') AS before_date
		FROM usage_archive_state WHERE singleton FOR UPDATE`).Take(&state).Error
	return state.BeforeDate, err
}

func partitionName(date string) (string, error) {
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return "", err
	}
	return "usage_minutely_" + day.Format("20060102"), nil
}

func (t usageTx) ArchiveDay(date string) error {
	if err := t.db.Exec(`INSERT INTO usage_daily
		(tenant_id, gateway_id, usage_date, client_id, input_bytes, output_bytes, billable_bytes, record_count, last_received_at)
		SELECT tenant_id, gateway_id, usage_date, client_id,
			SUM(input_bytes), SUM(output_bytes), SUM(billable_bytes), SUM(record_count), MAX(last_received_at)
		FROM usage_minutely WHERE usage_date = ?
		GROUP BY tenant_id, gateway_id, usage_date, client_id
		ON CONFLICT (tenant_id, gateway_id, usage_date, client_id) DO UPDATE SET
			input_bytes = usage_daily.input_bytes + EXCLUDED.input_bytes,
			output_bytes = usage_daily.output_bytes + EXCLUDED.output_bytes,
			billable_bytes = usage_daily.billable_bytes + EXCLUDED.billable_bytes,
			record_count = usage_daily.record_count + EXCLUDED.record_count,
			last_received_at = GREATEST(usage_daily.last_received_at, EXCLUDED.last_received_at)`, date).Error; err != nil {
		return err
	}
	name, err := partitionName(date)
	if err != nil {
		return err
	}
	return t.db.Exec("DROP TABLE " + name).Error
}

func (t usageTx) DatesBefore(cutoff string) ([]string, error) {
	var dates []string
	err := t.db.Raw(`SELECT DISTINCT to_char(usage_date, 'YYYY-MM-DD')
			FROM usage_minutely WHERE usage_date < ? ORDER BY 1`, cutoff).Scan(&dates).Error
	return dates, err
}

func (t usageTx) SetArchiveCutoff(cutoff string) error {
	return t.db.Exec("UPDATE usage_archive_state SET before_date = ? WHERE singleton", cutoff).Error
}
