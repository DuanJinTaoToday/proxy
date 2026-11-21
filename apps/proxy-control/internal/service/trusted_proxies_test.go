package service

import (
	"net/http"
	"testing"
)

func TestSourceIP(t *testing.T) {
	proxies, err := parseTrustedProxies([]string{"10.0.0.2/32", "10.0.0.3/32", "2001:db8::10/128", "::ffff:10.0.0.4/128"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, peer string
		forwarded  []string
		want       string
	}{
		{name: "direct peer ignores all headers", peer: "192.0.2.25:54632", forwarded: []string{"223.5.5.5", "10.0.0.2"}, want: "192.0.2.25"},
		{name: "single proxy", peer: "10.0.0.2:8000", forwarded: []string{"192.0.2.25"}, want: "192.0.2.25"},
		{name: "missing forwarded chain", peer: "10.0.0.2:8000", want: ""},
		{name: "stop at nearest untrusted hop", peer: "10.0.0.2:8000", forwarded: []string{"unparseable, 223.5.5.5, 192.0.2.25, 10.0.0.3"}, want: "192.0.2.25"},
		{name: "multiple forwarded header fields", peer: "10.0.0.2:8000", forwarded: []string{"223.5.5.5, 192.0.2.25", "10.0.0.3"}, want: "192.0.2.25"},
		{name: "malformed hop in trusted suffix", peer: "10.0.0.2:8000", forwarded: []string{"223.5.5.5, invalid, 10.0.0.3"}, want: ""},
		{name: "no untrusted hop", peer: "10.0.0.2:8000", forwarded: []string{"10.0.0.3"}, want: ""},
		{name: "IPv6 proxy chain", peer: "[2001:db8::10]:8000", forwarded: []string{"223.5.5.5, 2001:db8::20"}, want: "2001:db8::20"},
		{name: "IPv4 mapped IPv6 peer and hop", peer: "[::ffff:10.0.0.4]:8000", forwarded: []string{"::ffff:192.0.2.25"}, want: "192.0.2.25"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &http.Request{RemoteAddr: tt.peer, Header: make(http.Header)}
			for _, field := range tt.forwarded {
				request.Header.Add("X-Forwarded-For", field)
			}
			request.Header.Set("X-Real-IP", "223.5.5.5")
			request.Header.Set("Forwarded", "for=223.5.5.5")
			got := proxies.sourceIP(request)
			if tt.want == "" && !got.IsValid() {
				return
			}
			if got.String() != tt.want {
				t.Errorf("source IP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInvalidTrustedProxyCIDR(t *testing.T) {
	if _, err := parseTrustedProxies([]string{"localhost"}); err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}
