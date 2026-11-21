package repo

import (
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"slices"
	"strings"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/biz/access"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
)

type ipCountryRange struct {
	first, last netip.Addr
	code        string
}

type ipCountryDB struct {
	ranges []ipCountryRange
}

func ProvideIPCountryLookup(config *conf.Data, logger *slog.Logger) access.IPCountryLookup {
	path := config.GetRepo().GetIpCountryFile()
	if path != "" {
		db, err := loadIPCountry(path)
		if err == nil {
			return db
		}
		logger.Warn("load offline IP country database", "error", err)
	}
	return ipCountryDB{}
}

func loadIPCountry(path string) (ipCountryDB, error) {
	file, err := os.Open(path)
	if err != nil {
		return ipCountryDB{}, err
	}
	defer func() { _ = file.Close() }()
	if strings.HasSuffix(path, ".gz") {
		compressed, err := gzip.NewReader(file)
		if err != nil {
			return ipCountryDB{}, err
		}
		defer func() { _ = compressed.Close() }()
		return readIPCountry(compressed)
	}
	return readIPCountry(file)
}

func readIPCountry(data io.Reader) (ipCountryDB, error) {
	csvReader := csv.NewReader(data)
	csvReader.FieldsPerRecord = 3
	csvReader.ReuseRecord = true
	var db ipCountryDB
	for {
		row, err := csvReader.Read()
		switch {
		case errors.Is(err, io.EOF):
			return db, nil
		case err != nil:
			return ipCountryDB{}, fmt.Errorf("read IP country database: %w", err)
		}
		first, firstErr := netip.ParseAddr(row[0])
		last, lastErr := netip.ParseAddr(row[1])
		code := row[2]
		if firstErr != nil || lastErr != nil || first.Zone() != "" || last.Zone() != "" ||
			first.BitLen() != last.BitLen() || first.Compare(last) > 0 {
			return ipCountryDB{}, fmt.Errorf("invalid IP range in IP country database at record %d", len(db.ranges)+1)
		}
		if size := len(db.ranges); size > 0 && first.Compare(db.ranges[size-1].last) <= 0 {
			return ipCountryDB{}, fmt.Errorf("unordered or overlapping IP ranges in IP country database at record %d", size+1)
		}
		if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
			return ipCountryDB{}, fmt.Errorf("invalid country code in IP country database at record %d", len(db.ranges)+1)
		}
		db.ranges = append(db.ranges, ipCountryRange{first: first, last: last, code: code})
	}
}

func (db ipCountryDB) CountryCode(ip netip.Addr) string {
	if !ip.IsValid() {
		return ""
	}
	idx, _ := slices.BinarySearchFunc(db.ranges, ip, func(entry ipCountryRange, ip netip.Addr) int {
		return entry.last.Compare(ip)
	})
	if idx < len(db.ranges) && db.ranges[idx].first.BitLen() == ip.BitLen() && db.ranges[idx].first.Compare(ip) <= 0 {
		return db.ranges[idx].code
	}
	return ""
}
