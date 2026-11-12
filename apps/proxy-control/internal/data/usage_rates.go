package data

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/usage"
)

// Resolve rates in one query so a full batch does not spend a database round
// trip per Recorder line. Ordinal preserves the original allocation order.
func (t usageTx) Rates(records []usage.RateSample) ([]usage.Rate, error) {
	if len(records) == 0 {
		return nil, nil
	}
	type sampleTime struct {
		TenantID   string    `json:"tenant_id"`
		RecordedAt time.Time `json:"recorded_at"`
		Ordinal    int       `json:"ordinal"`
	}
	samples := make([]sampleTime, 0, len(records))
	for i, record := range records {
		samples = append(samples, sampleTime{record.TenantID, record.RecordedAt.Truncate(time.Microsecond), i})
	}
	input, err := json.Marshal(samples)
	if err != nil {
		return nil, err
	}
	var rates []usage.Rate
	err = t.db.Raw(`SELECT rate.id, rate.upload_ratio_milli, rate.download_ratio_milli
		FROM jsonb_to_recordset(?::jsonb) AS sample(tenant_id text, recorded_at timestamptz, ordinal integer)
		CROSS JOIN LATERAL (
			SELECT id, upload_ratio_milli, download_ratio_milli
			FROM billing_rates WHERE effective_from <= sample.recorded_at AND (`+scopeFilter+`) ORDER BY `+scopeOrder+`, effective_from DESC LIMIT 1
		) rate ORDER BY sample.ordinal`, string(input)).Scan(&rates).Error
	if err != nil {
		return nil, err
	}
	if len(rates) != len(records) {
		return nil, errors.New("no effective billing rate")
	}
	return rates, nil
}
