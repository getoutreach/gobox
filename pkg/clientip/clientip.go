// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Works out which IP address actually made a request.

// Package clientip finds the IP that made a request, ignoring X-Forwarded-For entries the caller could forge.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Option changes how From resolves the client IP.
type Option func(*options)

// options is the configuration From builds from its Option values.
type options struct {
	trusted func(netip.Addr) bool
}

// WithTrusted sets which addresses count as our own proxies, replacing the default of private and loopback addresses.
func WithTrusted(trusted func(netip.Addr) bool) Option {
	return func(o *options) {
		if trusted != nil {
			o.trusted = trusted
		}
	}
}

// From returns the rightmost X-Forwarded-For entry our proxies didn't add. Entries left of it are caller-written.
func From(r *http.Request, opts ...Option) netip.Addr {
	o := options{trusted: isTrusted}
	for _, opt := range opts {
		opt(&o)
	}

	addr := parseHop(r.RemoteAddr)
	if !addr.IsValid() || !o.trusted(addr) {
		return addr
	}

	hops := forwardedHops(r.Header)
	for i := len(hops) - 1; i >= 0; i-- {
		hop := parseHop(hops[i])
		if !hop.IsValid() {
			// Our proxies only append valid IPs, so this entry is forged; use the last proxy hop.
			return addr
		}
		addr = hop
		if !o.trusted(addr) {
			return addr
		}
	}
	return addr
}

// First returns the leftmost X-Forwarded-For entry, or the peer when there is none.
func First(r *http.Request) netip.Addr {
	if hops := forwardedHops(r.Header); len(hops) > 0 {
		return parseHop(hops[0])
	}
	return parseHop(r.RemoteAddr)
}

func forwardedHops(header http.Header) []string {
	var hops []string
	for _, value := range header.Values("X-Forwarded-For") {
		for _, hop := range strings.Split(value, ",") {
			hops = append(hops, strings.TrimSpace(hop))
		}
	}
	return hops
}

// parseHop accepts an optional port, as in RemoteAddr. Anything unparsable becomes an invalid Addr.
func parseHop(hop string) netip.Addr {
	if host, _, err := net.SplitHostPort(hop); err == nil {
		hop = host
	}
	addr, err := netip.ParseAddr(hop)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap().WithZone("")
}

// isTrusted treats private and loopback IPs as our proxies, the same set Rails trusts by default.
func isTrusted(addr netip.Addr) bool {
	return addr.IsPrivate() || addr.IsLoopback()
}
