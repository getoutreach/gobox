// Copyright 2026 Outreach Corporation. All Rights Reserved.

package clientip

import (
	"net/http"
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFrom(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        []string
		want       string
	}{
		{name: "direct untrusted peer ignores the header", remoteAddr: "198.51.100.7:4242", xff: []string{"203.0.113.1"}, want: "198.51.100.7"},
		{name: "proxy appended the real caller", remoteAddr: "10.1.2.3:4242", xff: []string{"198.51.100.7"}, want: "198.51.100.7"},
		{name: "forged leftmost entry is skipped", remoteAddr: "10.1.2.3:4242", xff: []string{"203.0.113.1, 198.51.100.7"}, want: "198.51.100.7"},
		{name: "forged private entry is skipped", remoteAddr: "10.1.2.3:4242", xff: []string{"10.9.9.9, 198.51.100.7"}, want: "198.51.100.7"},
		{
			name:       "chain of trusted proxies",
			remoteAddr: "10.1.2.3:4242",
			xff:        []string{"198.51.100.7, 10.4.4.4, 192.168.5.5"},
			want:       "198.51.100.7",
		},
		{name: "repeated headers are one list", remoteAddr: "10.1.2.3:4242", xff: []string{"203.0.113.1", "198.51.100.7"}, want: "198.51.100.7"},
		{name: "all trusted returns the furthest hop", remoteAddr: "10.1.2.3:4242", xff: []string{"172.16.4.4, 10.5.5.5"}, want: "172.16.4.4"},
		{name: "no header returns the peer", remoteAddr: "10.1.2.3:4242", want: "10.1.2.3"},
		{
			name:       "garbage stops the walk at the last trusted hop",
			remoteAddr: "10.1.2.3:4242",
			xff:        []string{"198.51.100.7, bogus, 10.5.5.5"},
			want:       "10.5.5.5",
		},
		{name: "empty entry stops the walk", remoteAddr: "10.1.2.3:4242", xff: []string{"198.51.100.7,"}, want: "10.1.2.3"},
		{name: "entry with a port", remoteAddr: "10.1.2.3:4242", xff: []string{"198.51.100.7:5555"}, want: "198.51.100.7"},
		{
			name:       "ipv6 unique local peer is trusted",
			remoteAddr: "[fd00::1]:4242",
			xff:        []string{"[2001:4860::8888]:5555"},
			want:       "2001:4860::8888",
		},
		{name: "loopback peer is trusted", remoteAddr: "127.0.0.1:4242", xff: []string{"198.51.100.7"}, want: "198.51.100.7"},
		{
			name:       "ipv4-mapped peer is unmapped before the trust check",
			remoteAddr: "[::ffff:10.1.2.3]:4242",
			xff:        []string{"198.51.100.7"},
			want:       "198.51.100.7",
		},
		{name: "link-local is not trusted", remoteAddr: "10.1.2.3:4242", xff: []string{"fe80::1%eth0"}, want: "fe80::1"},
		{name: "carrier-grade NAT is not trusted", remoteAddr: "100.64.0.1:4242", xff: []string{"203.0.113.1"}, want: "100.64.0.1"},
		{name: "synthetic request without a port", remoteAddr: "198.51.100.7", want: "198.51.100.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &http.Request{RemoteAddr: tt.remoteAddr, Header: http.Header{}}
			for _, value := range tt.xff {
				r.Header.Add("X-Forwarded-For", value)
			}
			assert.Equal(t, From(r).String(), tt.want)
		})
	}
}

func TestFromInvalidPeer(t *testing.T) {
	r := &http.Request{Header: http.Header{"X-Forwarded-For": {"198.51.100.7"}}}
	assert.Assert(t, !From(r).IsValid())
}

func TestFromWithTrusted(t *testing.T) {
	privateLinkLocalLoopback := func(addr netip.Addr) bool {
		return addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLoopback()
	}
	tests := []struct {
		name       string
		trusted    func(netip.Addr) bool
		remoteAddr string
		xff        []string
		want       string
	}{
		{
			name:       "a custom set can trust link-local hops",
			trusted:    privateLinkLocalLoopback,
			remoteAddr: "10.1.2.3:4242",
			xff:        []string{"198.51.100.7, fe80::1"},
			want:       "198.51.100.7",
		},
		{
			name:       "the default does not trust link-local hops",
			remoteAddr: "10.1.2.3:4242",
			xff:        []string{"198.51.100.7, fe80::1"},
			want:       "fe80::1",
		},
		{
			name:       "trusting everything returns the leftmost entry",
			trusted:    func(netip.Addr) bool { return true },
			remoteAddr: "198.51.100.7:4242",
			xff:        []string{"203.0.113.1, 192.0.2.9"},
			want:       "203.0.113.1",
		},
		{
			name:       "trusting nothing returns the peer",
			trusted:    func(netip.Addr) bool { return false },
			remoteAddr: "10.1.2.3:4242",
			xff:        []string{"198.51.100.7"},
			want:       "10.1.2.3",
		},
		{
			name:       "a nil predicate keeps the default",
			trusted:    nil,
			remoteAddr: "198.51.100.7:4242",
			xff:        []string{"203.0.113.1"},
			want:       "198.51.100.7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &http.Request{RemoteAddr: tt.remoteAddr, Header: http.Header{}}
			for _, value := range tt.xff {
				r.Header.Add("X-Forwarded-For", value)
			}
			var opts []Option
			if tt.trusted != nil {
				opts = append(opts, WithTrusted(tt.trusted))
			}
			assert.Equal(t, From(r, opts...).String(), tt.want)
		})
	}
}

func TestFromWithTrustedRelay(t *testing.T) {
	// The ALB appended the relay's edge; the relay appended the viewer before that.
	const viaRelay = "203.0.113.1, 198.51.100.7, 52.85.10.20"
	verified := func(*http.Request) bool { return true }
	tests := []struct {
		name       string
		verified   func(*http.Request) bool
		remoteAddr string
		xff        string
		want       string
	}{
		{name: "a verified relay's hop is skipped", verified: verified, remoteAddr: "10.1.2.3:4242", xff: viaRelay, want: "198.51.100.7"},
		{name: "an unverified relay's hop is the caller", verified: func(*http.Request) bool { return false }, remoteAddr: "10.1.2.3:4242", xff: viaRelay, want: "52.85.10.20"},
		{name: "no verifier leaves the hop as the caller", remoteAddr: "10.1.2.3:4242", xff: viaRelay, want: "52.85.10.20"},
		{name: "only one hop is skipped", verified: verified, remoteAddr: "10.1.2.3:4242", xff: "198.51.100.7, 52.85.10.21, 52.85.10.20", want: "52.85.10.21"},
		{name: "an untrusted peer is never skipped", verified: verified, remoteAddr: "198.51.100.9:4242", xff: viaRelay, want: "198.51.100.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &http.Request{RemoteAddr: tt.remoteAddr, Header: http.Header{"X-Forwarded-For": {tt.xff}}}
			var opts []Option
			if tt.verified != nil {
				opts = append(opts, WithTrustedRelay(tt.verified))
			}
			assert.Equal(t, From(r, opts...).String(), tt.want)
		})
	}
}
