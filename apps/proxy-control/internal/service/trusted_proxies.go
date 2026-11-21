package service

import (
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

type trustedProxies []netip.Prefix

func parseTrustedProxies(cidrs []string) (trustedProxies, error) {
	proxies := make(trustedProxies, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid server.http.trusted_proxy_cidrs entry %q: %w", cidr, err)
		}
		if prefix.Addr().Is4In6() {
			if prefix.Bits() < 96 {
				return nil, fmt.Errorf("invalid IPv4-mapped proxy CIDR %q", cidr)
			}
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		proxies = append(proxies, prefix.Masked())
	}
	return proxies, nil
}

func (proxies trustedProxies) contains(ip netip.Addr) bool {
	return slices.ContainsFunc(proxies, func(prefix netip.Prefix) bool {
		return prefix.Contains(ip)
	})
}

func (proxies trustedProxies) sourceIP(request *http.Request) netip.Addr {
	if request == nil {
		return netip.Addr{}
	}
	socket, err := netip.ParseAddrPort(request.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	peer := socket.Addr().WithZone("").Unmap()
	if !proxies.contains(peer) {
		return peer
	}
	// Proxies append their peer on the right. Stop at the first untrusted hop
	// without parsing the potentially client-supplied values to its left.
	for _, header := range slices.Backward(request.Header.Values("X-Forwarded-For")) {
		for {
			comma := strings.LastIndexByte(header, ',')
			ip, err := netip.ParseAddr(strings.TrimSpace(header[comma+1:]))
			if err != nil || ip.Zone() != "" {
				return netip.Addr{}
			}
			ip = ip.Unmap()
			if !proxies.contains(ip) {
				return ip
			}
			if comma < 0 {
				break
			}
			header = header[:comma]
		}
	}
	return netip.Addr{}
}
