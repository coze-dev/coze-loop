// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/stretchr/testify/require"
)

type runtimePolicySequence struct {
	conf.IConfigLoader
	values []string
	reads  int
}

func (s *runtimePolicySequence) Get(_ context.Context, key string) any {
	if key != "lifecycle_hook" {
		panic("unexpected config key")
	}
	i := min(s.reads, len(s.values)-1)
	s.reads++
	return s.values[i]
}

const runtimePolicyV1 = `{"endpoint_policy":[{"workspace_id":1,"host":"example.com","port":443,"environment":"Prod","private_cidrs":["10.1.0.0/16"]}],"signing_key_refs":[{"workspace_id":1,"host":"example.com","port":443,"environment":"Prod","key_id":"synthetic-key-v1","key_ref":"synthetic-ref-v1"}]}`

func TestHookTransportRejectsPolicyKeyVersionMix(t *testing.T) {
	rotated := strings.ReplaceAll(runtimePolicyV1, "-v1", "-v2")
	for _, tt := range []struct {
		name, later string
		reject      bool
	}{
		{"stable", runtimePolicyV1, false},
		{"rotated key same policy", rotated, false},
		{"CIDR moved and key rotated", strings.ReplaceAll(rotated, "10.1.0.0/16", "10.2.0.0/16"), true},
		{"CIDR narrowed", strings.ReplaceAll(runtimePolicyV1, "10.1.0.0/16", "10.1.0.0/24"), true},
		{"CIDR broadened", strings.ReplaceAll(runtimePolicyV1, "10.1.0.0/16", "10.0.0.0/8"), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loader := &runtimePolicySequence{values: []string{runtimePolicyV1, tt.later, tt.later}}
			cfg := NewRuntimeConfigProvider(loader, true)
			resolver := NewKeyResolver(cfg, hookSecretFunc(func(context.Context, string) ([]byte, error) { return []byte("synthetic-only"), nil }))
			transport := NewHTTPTransport(resolver, cfg.EndpointPolicies)
			lookups, dials := 0, 0
			transport.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
				lookups++
				return []netip.Addr{netip.MustParseAddr("10.1.2.3")}, nil
			}
			transport.dial = func(_ context.Context, network, address string) (net.Conn, error) {
				dials++
				require.Equal(t, "tcp", network)
				require.Equal(t, "10.1.2.3:443", address)
				return nil, errors.New("synthetic dial boundary; no socket created")
			}
			got := transport.Invoke(context.Background(), httpTestInput())
			if tt.reject {
				require.Zero(t, lookups, "mismatched authorization must stop this invocation before DNS")
				require.Zero(t, dials)
				require.Equal(t, entity.HookSecurityError, got.Outcome.Code)
			} else {
				require.Equal(t, 1, lookups)
				require.Equal(t, 1, dials)
				require.Equal(t, entity.HookTransportError, got.Outcome.Code)
			}
		})
	}
}

func TestHookTransportPolicyChangeRequiresWholeNewInvocation(t *testing.T) {
	v2 := strings.ReplaceAll(strings.ReplaceAll(runtimePolicyV1, "10.1.0.0/16", "10.2.0.0/16"), "-v1", "-v2")
	loader := &runtimePolicySequence{values: []string{runtimePolicyV1, v2, v2}}
	cfg := NewRuntimeConfigProvider(loader, true)
	var refs, addresses []string
	tr := NewHTTPTransport(NewKeyResolver(cfg, hookSecretFunc(func(_ context.Context, ref string) ([]byte, error) {
		refs = append(refs, ref)
		return []byte("synthetic-only"), nil
	})), cfg.EndpointPolicies)
	tr.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("10.2.3.4")}, nil
	}
	tr.dial = func(_ context.Context, _, address string) (net.Conn, error) {
		addresses = append(addresses, address)
		return nil, errors.New("no socket")
	}
	got := tr.Invoke(context.Background(), httpTestInput())
	require.Equal(t, entity.HookSecurityError, got.Outcome.Code)
	require.Empty(t, addresses)
	got = tr.Invoke(context.Background(), httpTestInput())
	require.Equal(t, entity.HookTransportError, got.Outcome.Code)
	require.Equal(t, []string{"10.2.3.4:443"}, addresses)
	require.Equal(t, []string{"synthetic-ref-v2", "synthetic-ref-v2"}, refs)
}

func TestHookPolicyFingerprintCoversCanonicalEgressBinding(t *testing.T) {
	policy := EndpointPolicy{WorkspaceID: 1, Host: "example.com", Port: 443, Environment: "PPE", Lane: "lane-a", PrivateCIDRs: []netip.Prefix{netip.MustParsePrefix("10.1.0.0/16"), netip.MustParsePrefix("10.2.0.0/16")}}
	base, err := endpointPolicyFingerprint(policy)
	require.NoError(t, err)
	require.Len(t, base, 64)
	for _, change := range []func(*EndpointPolicy){
		func(p *EndpointPolicy) { p.WorkspaceID = 2 },
		func(p *EndpointPolicy) { p.Host = "other.example" },
		func(p *EndpointPolicy) { p.Port = 444 },
		func(p *EndpointPolicy) { p.Environment = "BOE" },
		func(p *EndpointPolicy) { p.Lane = "lane-b" },
		func(p *EndpointPolicy) { p.PrivateCIDRs = nil },
		func(p *EndpointPolicy) { p.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix("10.1.0.0/24")} },
	} {
		other := policy
		change(&other)
		fingerprint, err := endpointPolicyFingerprint(other)
		require.NoError(t, err)
		require.NotEqual(t, base, fingerprint)
	}
	canonicalEquivalent := policy
	canonicalEquivalent.Host = "EXAMPLE.com."
	canonicalEquivalent.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix("10.2.0.0/16"), netip.MustParsePrefix("10.1.0.0/16"), netip.MustParsePrefix("10.1.0.0/16")}
	fingerprint, err := endpointPolicyFingerprint(canonicalEquivalent)
	require.NoError(t, err)
	require.Equal(t, base, fingerprint)
	require.Equal(t, "10.2.0.0/16", canonicalEquivalent.PrivateCIDRs[0].String(), "fingerprinting must not mutate policy storage")
	_, err = endpointPolicyFingerprint(EndpointPolicy{})
	require.Error(t, err)
}
