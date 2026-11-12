package usage

func resolveRates(tx Transaction, records []record) ([]Rate, error) {
	samples := make([]RateSample, 0, len(records))
	for _, r := range records {
		samples = append(samples, RateSample{TenantID: r.tenantID, RecordedAt: r.Time})
	}
	return tx.Rates(samples)
}
