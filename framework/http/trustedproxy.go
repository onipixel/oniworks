package http

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"
)

// proxyConfig holds the set of trusted upstream proxies. Forwarding headers
// (X-Forwarded-For / X-Real-IP) are only honored when the direct peer matches.
type proxyConfig struct {
	nets     []*net.IPNet
	trustAll bool
}

// trustedProxyConfig is read on every Request.IP() call and swapped atomically
// by SetTrustedProxies. nil means "trust no proxies" (the secure default).
var trustedProxyConfig atomic.Pointer[proxyConfig]

// SetTrustedProxies configures which upstream proxy addresses are allowed to
// set X-Forwarded-For / X-Real-IP. Accepts individual IPs ("10.0.0.1") or CIDR
// ranges ("10.0.0.0/8"). Pass the single value "*" to trust all proxies — only
// safe when the app is never directly reachable by clients.
//
// Call once at startup. With no trusted proxies configured, forwarding headers
// are ignored and Request.IP() returns the direct peer address.
func SetTrustedProxies(cidrs ...string) error {
	if len(cidrs) == 1 && strings.TrimSpace(cidrs[0]) == "*" {
		trustedProxyConfig.Store(&proxyConfig{trustAll: true})
		return nil
	}
	cfg := &proxyConfig{}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if !strings.Contains(c, "/") {
			// Bare IP — convert to a single-host CIDR.
			if ip := net.ParseIP(c); ip != nil {
				if ip.To4() != nil {
					c += "/32"
				} else {
					c += "/128"
				}
			}
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			return fmt.Errorf("http: invalid trusted proxy %q: %w", c, err)
		}
		cfg.nets = append(cfg.nets, n)
	}
	trustedProxyConfig.Store(cfg)
	return nil
}

// ipInNets reports whether ipStr falls within any of the given networks.
func ipInNets(ipStr string, nets []*net.IPNet) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
