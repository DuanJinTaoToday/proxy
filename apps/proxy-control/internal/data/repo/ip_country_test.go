package repo

import (
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/internal/conf"
)

const countryCSV = `8.8.8.0,8.8.8.255,US
113.28.0.0,113.28.0.255,HK
182.93.0.0,182.93.0.255,MO
203.69.0.0,203.69.0.255,TW
223.5.5.0,223.5.5.255,CN
2001:db8::,2001:db8::ff,US
`

func TestIPCountryRanges(t *testing.T) {
	db, err := readIPCountry(strings.NewReader(countryCSV))
	if err != nil {
		t.Fatal(err)
	}
	for address, want := range map[string]string{
		"8.8.7.255":     "",
		"8.8.8.0":       "US",
		"8.8.8.255":     "US",
		"8.8.9.0":       "",
		"113.28.0.15":   "HK",
		"182.93.0.20":   "MO",
		"203.69.0.25":   "TW",
		"223.5.5.5":     "CN",
		"2001:db8::1":   "US",
		"2001:db8::100": "",
	} {
		if got := db.CountryCode(netip.MustParseAddr(address)); got != want {
			t.Errorf("country code for %s = %q, want %q", address, got, want)
		}
	}
}

func TestIPCountryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "country.csv.gz")
	var data bytes.Buffer
	gzipWriter := gzip.NewWriter(&data)
	if _, err := io.WriteString(gzipWriter, countryCSV); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := loadIPCountry(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := db.CountryCode(netip.MustParseAddr("223.5.5.5")); got != "CN" {
		t.Errorf("country code = %q, want CN", got)
	}
}

func TestUnavailableIPCountryFile(t *testing.T) {
	dir := t.TempDir()
	malformed := filepath.Join(dir, "bad.csv")
	contents := countryCSV + "invalid range\n"
	if err := os.WriteFile(malformed, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", filepath.Join(dir, "missing.csv"), malformed} {
		config := &conf.Data{Repo: &conf.Data_Repo{IpCountryFile: path}}
		db := ProvideIPCountryLookup(config, slog.New(slog.DiscardHandler))
		if got := db.CountryCode(netip.MustParseAddr("223.5.5.5")); got != "" {
			t.Errorf("unavailable database %q returned %q", path, got)
		}
	}
}
