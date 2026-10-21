package platform

import (
	"net/http"
	"net/netip"
	"strings"
)

type ClientAddressResolver struct {
	trustedProxies []netip.Prefix
}

func NewClientAddressResolver(trustedProxies []netip.Prefix) ClientAddressResolver {
	return ClientAddressResolver{trustedProxies: trustedProxies}
}

// Resolve trusts X-Forwarded-For only when the TCP peer is a configured proxy, and then
// takes the right-most entry that is not itself a trusted proxy: entries to its left
// are supplied by the client and can be forged.
func (resolver ClientAddressResolver) Resolve(r *http.Request) netip.Addr {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	peerAddr := peer.Addr().Unmap()
	if !resolver.isTrusted(peerAddr) {
		return peerAddr
	}
	entries := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for index := len(entries) - 1; index >= 0; index-- {
		entry := strings.TrimSpace(entries[index])
		if entry == "" {
			continue
		}
		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return peerAddr
		}
		addr = addr.Unmap()
		if !resolver.isTrusted(addr) {
			return addr
		}
	}
	return peerAddr
}

func (resolver ClientAddressResolver) isTrusted(addr netip.Addr) bool {
	for _, prefix := range resolver.trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
