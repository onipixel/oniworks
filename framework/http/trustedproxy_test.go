package http

import (
	"net/http/httptest"
	"testing"
)

func reqWithForwarded(remoteAddr, xff, xrip string) *Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = remoteAddr
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	if xrip != "" {
		r.Header.Set("X-Real-IP", xrip)
	}
	return newRequest(r, nil)
}

// TestIPIgnoresForwardedByDefault is the spoofing regression: with no trusted
// proxies configured, a client-supplied X-Forwarded-For must be ignored.
func TestIPIgnoresForwardedByDefault(t *testing.T) {
	trustedProxyConfig.Store(nil) // default
	r := reqWithForwarded("203.0.113.9:5555", "1.2.3.4", "5.6.7.8")
	if got := r.IP(); got != "203.0.113.9" {
		t.Fatalf("got %q, want direct peer 203.0.113.9 (forwarded headers must be ignored)", got)
	}
}

// TestIPHonorsForwardedFromTrustedProxy verifies that when the peer is a
// configured trusted proxy, the forwarded client IP is used.
func TestIPHonorsForwardedFromTrustedProxy(t *testing.T) {
	t.Cleanup(func() { trustedProxyConfig.Store(nil) })
	if err := SetTrustedProxies("10.0.0.0/8"); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}
	r := reqWithForwarded("10.0.0.5:5555", "1.2.3.4, 10.0.0.5", "")
	if got := r.IP(); got != "1.2.3.4" {
		t.Fatalf("got %q, want forwarded client 1.2.3.4", got)
	}
}

// TestIPRejectsForwardedFromUntrustedPeer verifies a non-trusted peer's
// forwarded header is ignored even when some proxies are trusted.
func TestIPRejectsForwardedFromUntrustedPeer(t *testing.T) {
	t.Cleanup(func() { trustedProxyConfig.Store(nil) })
	_ = SetTrustedProxies("10.0.0.0/8")
	r := reqWithForwarded("203.0.113.9:5555", "1.2.3.4", "")
	if got := r.IP(); got != "203.0.113.9" {
		t.Fatalf("got %q, want direct peer 203.0.113.9 (untrusted peer)", got)
	}
}

// TestSetTrustedProxiesWildcard verifies "*" trusts all peers.
func TestSetTrustedProxiesWildcard(t *testing.T) {
	t.Cleanup(func() { trustedProxyConfig.Store(nil) })
	_ = SetTrustedProxies("*")
	r := reqWithForwarded("203.0.113.9:5555", "1.2.3.4", "")
	if got := r.IP(); got != "1.2.3.4" {
		t.Fatalf("got %q, want 1.2.3.4 with wildcard trust", got)
	}
}

// TestSetTrustedProxiesBareIP verifies a bare IP is accepted and matched.
func TestSetTrustedProxiesBareIP(t *testing.T) {
	t.Cleanup(func() { trustedProxyConfig.Store(nil) })
	if err := SetTrustedProxies("192.168.1.10"); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}
	r := reqWithForwarded("192.168.1.10:9999", "8.8.8.8", "")
	if got := r.IP(); got != "8.8.8.8" {
		t.Fatalf("got %q, want 8.8.8.8", got)
	}
}

// TestSetTrustedProxiesInvalid verifies a bad CIDR returns an error.
func TestSetTrustedProxiesInvalid(t *testing.T) {
	t.Cleanup(func() { trustedProxyConfig.Store(nil) })
	if err := SetTrustedProxies("not-an-ip/99"); err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}
