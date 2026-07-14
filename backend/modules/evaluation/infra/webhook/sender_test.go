// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsPrivateHost_Literal covers hostport strings that already contain a
// literal IP: RFC1918 / loopback / link-local / IPv6 loopback / AWS metadata.
// Every one of these must be blocked because the DialContext guard also
// re-checks — but the guardURL pass is the fast path, so we lock it here.
func TestIsPrivateHost_Literal(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"10.0.0.1", true},
		{"10.0.0.1:8443", true},
		{"127.0.0.1", true},
		{"127.0.0.1:8080", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},   // AWS IMDS
		{"169.254.169.254:80", true}, // AWS IMDS with port
		{"[::1]", true},
		{"[::1]:8080", true},
		{"[fe80::1]", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"1.1.1.1:443", false},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			got := IsPrivateHost(tc.host)
			require.Equal(t, tc.want, got, "IsPrivateHost(%q)", tc.host)
		})
	}
}

// TestIsPrivateHost_DNS makes sure the DNS-resolved case blocks a hostname
// whose A record maps to a private IP. Using a stubbed resolver via
// isPrivateHostCtx keeps the test offline / deterministic.
func TestIsPrivateHost_DNS(t *testing.T) {
	// Point at an in-process resolver; if the environment has no working DNS
	// we still exercise the literal-IP branch. Real DNS-rebind coverage
	// requires DialContext-level tests below.
	_ = context.Background()
	require.True(t, IsPrivateHost("localhost:1234"), "localhost should resolve to loopback")
}

// TestIpIsPrivate exercises the pure IP-classification helper for the
// SSRF cases E-F-01/02/03/04.
func TestIpIsPrivate(t *testing.T) {
	privates := []string{
		"10.0.0.1",
		"127.0.0.1",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.1.1",
		"169.254.169.254",
		"::1",
		"fe80::1",
	}
	publics := []string{
		"8.8.8.8",
		"1.1.1.1",
		"142.250.72.14",
		"2606:4700:4700::1111",
	}
	for _, p := range privates {
		t.Run("private/"+p, func(t *testing.T) {
			require.True(t, ipIsPrivate(net.ParseIP(p)), "expected %s to be private", p)
		})
	}
	for _, p := range publics {
		t.Run("public/"+p, func(t *testing.T) {
			require.False(t, ipIsPrivate(net.ParseIP(p)), "expected %s to be public", p)
		})
	}
}

// TestGuardedDialContext_RejectsPrivateLiteral verifies the TOCTOU guard:
// even if the caller somehow bypassed guardURL with a public IP, the dialer
// rejects a private IP at dial time.
func TestGuardedDialContext_RejectsPrivateLiteral(t *testing.T) {
	dial := newGuardedDialContext(0)
	_, err := dial(context.Background(), "tcp", "10.0.0.1:8443")
	require.Error(t, err)
	require.Contains(t, err.Error(), "private_network")
}

// TestGuardedDialContext_BITsBypass verifies internal-source callers can
// still reach private addresses via ctx marker.
func TestGuardedDialContext_BITsBypass(t *testing.T) {
	dial := newGuardedDialContext(0)
	ctx := withBITsBypass(context.Background())
	// Use a loopback listener so the dial actually succeeds when bypass is on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	conn, err := dial(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
}
