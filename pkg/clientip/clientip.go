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
	trusted       func(netip.Addr) bool
	relayVerified func(*http.Request) bool
}

// WithTrusted sets which addresses count as our own proxies, replacing the default of private and loopback addresses.
func WithTrusted(trusted func(netip.Addr) bool) Option {
	return func(o *options) {
		if trusted != nil {
			o.trusted = trusted
		}
	}
}

// WithTrustedRelay makes From also skip the one public hop of a relay we run, such as a CDN edge, when verified accepts the request.
func WithTrustedRelay(verified func(*http.Request) bool) Option {
	return func(o *options) {
		o.relayVerified = verified
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

	skipRelayHop := o.relayVerified != nil && o.relayVerified(r)
	hops := forwardedHops(r.Header)
	for i := len(hops) - 1; i >= 0; i-- {
		hop := parseHop(hops[i])
		if !hop.IsValid() {
			// Our proxies only append valid IPs, so this entry is forged; use the last proxy hop.
			return addr
		}
		addr = hop
		if o.trusted(addr) {
			continue
		}
		if skipRelayHop {
			skipRelayHop = false
			continue
		}
		return addr
	}
	return addr
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
