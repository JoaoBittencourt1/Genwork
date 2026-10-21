package platform_test

import (
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientAddressResolver(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		expectedAddr string
	}{
		{"direct peer", "203.0.113.7:5000", "", "203.0.113.7"},
		{"spoofed header from untrusted peer is ignored", "203.0.113.7:5000", "198.51.100.1", "203.0.113.7"},
		{"trusted proxy forwards client", "10.0.0.2:5000", "198.51.100.1", "198.51.100.1"},
		{"right-most untrusted entry wins over forged left entries", "10.0.0.2:5000", "1.2.3.4, 198.51.100.1", "198.51.100.1"},
		{"chain of trusted proxies", "10.0.0.2:5000", "198.51.100.1, 10.0.0.3", "198.51.100.1"},
		{"garbage entry falls back to peer", "10.0.0.2:5000", "not-an-ip", "10.0.0.2"},
		{"trusted proxy without header", "10.0.0.2:5000", "", "10.0.0.2"},
	}
	resolver := platform.NewClientAddressResolver(trusted)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/", nil)
			request.RemoteAddr = testCase.remoteAddr
			if testCase.forwardedFor != "" {
				request.Header.Set("X-Forwarded-For", testCase.forwardedFor)
			}
			if got := resolver.Resolve(request).String(); got != testCase.expectedAddr {
				t.Errorf("Resolve() = %s, want %s", got, testCase.expectedAddr)
			}
		})
	}
}
