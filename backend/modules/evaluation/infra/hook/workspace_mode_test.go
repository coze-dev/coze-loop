// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestHookWorkspaceModeNeedsNoEndpointRegistration(t *testing.T) {
	for _, raw := range []string{
		`{"admission_enabled":true,"workspace_allowlist":["7590120691407168258",42]}`,
		`{"admission_enabled":true,"workspace_allowlist":[]}`,
	} {
		t.Run(raw, func(t *testing.T) {
			cfg, err := NewRuntimeConfigProvider(&hookConfigLoader{value: raw}, true).GetRuntimeConfig(context.Background())
			require.NoError(t, err)
			require.True(t, cfg.WorkerEnabled)
		})
	}
}

type workspaceSecretFunc func(context.Context, int64) (string, error)

func (f workspaceSecretFunc) GetWorkspaceSigningSecret(ctx context.Context, spaceID int64) (string, error) {
	return f(ctx, spaceID)
}

func TestHookWorkspaceSigningUsesServerWorkspaceAndKeepsAdmittedRuns(t *testing.T) {
	for _, id := range []int64{42, 43} {
		for _, env := range []entity.HookEnvironment{entity.HookEnvironmentProd, entity.HookEnvironmentPPE, entity.HookEnvironmentBOE} {
			t.Run(strconv.FormatInt(id, 10)+string(env), func(t *testing.T) {
				loader := &hookConfigLoader{value: `{"admission_enabled":true,"workspace_allowlist":[42,43]}`}
				r := NewKeyResolver(NewRuntimeConfigProvider(loader, true), nil, workspaceSecretFunc(func(_ context.Context, got int64) (string, error) {
					require.Equal(t, id, got)
					return "space-secret-" + strconv.FormatInt(got, 10), nil
				}))
				b := entity.HookKeyBinding{WorkspaceID: id, URL: "https://new-service.example/a%2Fb?q=x%2Fy", Environment: env}
				if env != entity.HookEnvironmentProd {
					b.Lane = "lane_from_form"
				}
				key, err := r.Resolve(context.Background(), b)
				require.NoError(t, err)
				require.Equal(t, "fornax-space-"+strconv.FormatInt(id, 10), key.KeyID)
				require.Equal(t, "space-secret-"+strconv.FormatInt(id, 10), string(key.Secret))
				loader.value = `{"admission_enabled":false,"workspace_allowlist":[]}`
				after, err := r.Resolve(context.Background(), b)
				require.NoError(t, err, "admission changes must not stop admitted Runs")
				require.Equal(t, key, after)
			})
		}
	}
}

func TestHookWorkspaceSigningReadFailuresNeverFallback(t *testing.T) {
	for _, mode := range []string{"empty", "blank", "error", "mode_changed", "config_broken", "canceled", "missing"} {
		t.Run(mode, func(t *testing.T) {
			loader := &hookConfigLoader{value: `{"workspace_allowlist":[42]}`}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var provider workspaceSecretFunc = func(context.Context, int64) (string, error) {
				switch mode {
				case "empty":
					return "", nil
				case "blank":
					return " \t", nil
				case "error":
					return "", errors.New("private secret detail")
				case "mode_changed":
					loader.value = `{` + hookRuntimeSecurity + `}`
				case "config_broken":
					loader.value = `{"workspace_allowlist":null}`
				case "canceled":
					cancel()
				}
				return "private secret detail", nil
			}
			if mode == "missing" {
				provider = nil
			}
			r := NewKeyResolver(NewRuntimeConfigProvider(loader, true), hookSecretFunc(func(context.Context, string) ([]byte, error) {
				t.Fatal("workspace signing fell back to legacy key")
				return nil, nil
			}), provider)
			key, err := r.Resolve(ctx, entity.HookKeyBinding{WorkspaceID: 42, URL: "https://example.com", Environment: entity.HookEnvironmentProd})
			require.EqualError(t, err, "hook signing key unavailable")
			require.Empty(t, key)
		})
	}
}

func TestHookWorkspaceEndpointPolicyDerivedFromForm(t *testing.T) {
	p := NewRuntimeConfigProvider(&hookConfigLoader{value: `{"workspace_allowlist":[]}`}, true)
	resolver, ok := any(p).(interface {
		ResolveEndpointPolicy(context.Context, EndpointTarget) (EndpointPolicy, error)
	})
	require.True(t, ok, "runtime must resolve unregistered targets for admitted Runs")
	u, err := url.Parse("https://NEW-service.example:8443/a%2Fb?x=1")
	require.NoError(t, err)
	policy, err := resolver.ResolveEndpointPolicy(context.Background(), EndpointTarget{WorkspaceID: 42, URL: u, Environment: "PPE", Lane: "ppe_new"})
	require.NoError(t, err)
	require.Equal(t, "new-service.example", policy.Host)
	require.Equal(t, uint16(8443), policy.Port)
	require.Equal(t, "PPE", policy.Environment)
	require.Equal(t, "ppe_new", policy.Lane)
	for _, ip := range []string{"10.1.2.3", "192.168.1.1", "100.64.2.3", "fd12::1", "203.0.113.1"} {
		require.NoError(t, ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr(ip)}), ip)
	}
	for _, ip := range []string{"127.0.0.1", "169.254.169.254", "100.100.100.200", "fd00:ec2::254", "::1", "::ffff:127.0.0.1", "0.0.0.0", "224.0.0.1", "fe80::1"} {
		require.Error(t, ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr(ip)}), ip)
	}
}

func TestHookWorkspaceAllowlistStrictAndAdmissionOnly(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `42`, `"42"`, `[null]`, `[0]`, `[-1]`, `["+1"]`, `[" 1"]`, `["1e2"]`, `[1.0]`, `[1e2]`, `[true]`, `["9223372036854775808"]`, `[42,"042"]`} {
		_, err := NewRuntimeConfigProvider(&hookConfigLoader{value: `{"workspace_allowlist":` + raw + `}`}, true).GetRuntimeConfig(context.Background())
		require.Error(t, err, raw)
	}
	for _, tc := range []struct {
		raw                 string
		configured, allowed bool
	}{
		{`{"admission_enabled":true,` + hookRuntimeSecurity + `}`, false, true},
		{`{"admission_enabled":true,"workspace_allowlist":[42]}`, true, true},
		{`{"admission_enabled":true,"workspace_allowlist":[43]}`, true, false},
		{`{"admission_enabled":true,"workspace_allowlist":[]}`, true, false},
		{`{"admission_enabled":false,"workspace_allowlist":[42]}`, true, false},
	} {
		cfg, err := NewRuntimeConfigProvider(&hookConfigLoader{value: tc.raw}, true).GetRuntimeConfig(context.Background())
		require.NoError(t, err)
		require.Equal(t, tc.configured, cfg.WorkspaceAllowlistConfigured)
		require.Equal(t, tc.allowed, cfg.AllowsWorkspace(42))
		require.False(t, cfg.AllowsWorkspace(0))
		require.True(t, cfg.WorkerEnabled)
	}
}

func TestHookWorkspaceHTTPUsesSPIv1WithoutRegistration(t *testing.T) {
	for _, env := range []entity.HookEnvironment{entity.HookEnvironmentProd, entity.HookEnvironmentPPE, entity.HookEnvironmentBOE} {
		t.Run(string(env), func(t *testing.T) {
			var calls atomic.Int32
			tr, input, _ := httpTLSFixture(t, func(w http.ResponseWriter, req *http.Request) {
				calls.Add(1)
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				digest := sha256.Sum256(body)
				message := strings.Join([]string{"POST", req.RequestURI, req.Header.Get("X-Fornax-Hook-Timestamp"), req.Header.Get("X-Fornax-Hook-Nonce"), "op", hex.EncodeToString(digest[:])}, "\n")
				mac := hmac.New(sha256.New, []byte("workspace-test-sk"))
				_, _ = mac.Write([]byte(message))
				require.Equal(t, hex.EncodeToString(mac.Sum(nil)), req.Header.Get("X-Fornax-Hook-Signature"))
				require.Equal(t, "fornax-space-1", req.Header.Get("X-Fornax-Hook-Key-Id"))
				require.NotContains(t, string(body), "workspace-test-sk")
				for _, values := range req.Header {
					for _, value := range values {
						require.NotContains(t, value, "workspace-test-sk")
					}
				}
				if env == entity.HookEnvironmentProd {
					require.Empty(t, req.Header.Get("x-tt-env"))
				} else {
					require.Equal(t, "lane_from_form", req.Header.Get("x-tt-env"))
				}
				require.Equal(t, env == entity.HookEnvironmentPPE, req.Header.Get("x-use-ppe") == "1")
				writeHTTPSuccess(w)
			})
			loader := &hookConfigLoader{value: `{"admission_enabled":true,"workspace_allowlist":[1]}`}
			cfg := NewRuntimeConfigProvider(loader, true)
			keys := NewKeyResolver(cfg, nil, workspaceSecretFunc(func(_ context.Context, id int64) (string, error) {
				require.Equal(t, int64(1), id)
				return "workspace-test-sk", nil
			}))
			wired := NewHTTPTransport(keys, nil, cfg.ResolveEndpointPolicy)
			wired.dial, wired.lookup, wired.rootCAs = tr.dial, tr.lookup, tr.rootCAs
			input.Config.Environment = gptr.Of(env)
			if env != entity.HookEnvironmentProd {
				input.Config.Lane = gptr.Of("lane_from_form")
			}
			got := wired.Invoke(context.Background(), input)
			require.NotNil(t, got.Response)
			require.Equal(t, "succeeded", string(got.Response.GetStatus()))
			input.Request.EventType = gptr.Of("experiment.run.after")
			input.Request.Context.TerminalStatus = gptr.Of("success")
			for _, revoked := range []string{
				`{"admission_enabled":true,"workspace_allowlist":[]}`,
				`{"admission_enabled":false,"workspace_allowlist":[1]}`,
				`{"admission_enabled":false,"workspace_allowlist":[]}`,
			} {
				loader.value = revoked
				runtime, err := cfg.GetRuntimeConfig(context.Background())
				require.NoError(t, err)
				require.False(t, runtime.AllowsWorkspace(1))
				got = wired.Invoke(context.Background(), input)
				require.NotNil(t, got.Response, "after must still execute following admission revocation")
			}
			require.Equal(t, int32(4), calls.Load())
		})
	}
}

func TestHookWorkspaceHTTPChecksAllDNSAddressesAndActualPeer(t *testing.T) {
	for _, mode := range []string{"private", "mixed_metadata", "peer_mismatch", "wrong_workspace", "provider_error"} {
		t.Run(mode, func(t *testing.T) {
			var requests, dials atomic.Int32
			fixture, input, _ := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); writeHTTPSuccess(w) })
			cfg := NewRuntimeConfigProvider(&hookConfigLoader{value: `{"workspace_allowlist":[1]}`}, true)
			tr := NewHTTPTransport(NewKeyResolver(cfg, nil, workspaceSecretFunc(func(context.Context, int64) (string, error) { return "workspace-test-sk", nil })), nil, cfg.ResolveEndpointPolicy)
			tr.rootCAs = fixture.rootCAs
			tr.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
				ips := []netip.Addr{netip.MustParseAddr("10.1.2.3")}
				if mode == "mixed_metadata" {
					ips = append(ips, netip.MustParseAddr("169.254.169.254"))
				}
				return ips, nil
			}
			tr.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
				dials.Add(1)
				peer, err := netip.ParseAddrPort(address)
				require.NoError(t, err)
				require.Equal(t, "10.1.2.3", peer.Addr().String())
				conn, err := fixture.dial(ctx, network, netip.AddrPortFrom(netip.MustParseAddr("203.0.113.10"), peer.Port()).String())
				if err != nil {
					return nil, err
				}
				addressIP := "10.1.2.3"
				if mode == "peer_mismatch" {
					addressIP = "169.254.169.254"
				}
				conn.(*httpTestPeerConn).peer = &net.TCPAddr{IP: net.ParseIP(addressIP), Port: int(peer.Port())}
				return conn, nil
			}
			if mode == "wrong_workspace" {
				input.WorkspaceID = 2
			}
			if mode == "provider_error" {
				tr.targetPolicy = func(context.Context, EndpointTarget) (EndpointPolicy, error) {
					return EndpointPolicy{}, errors.New("unavailable")
				}
				tr.policies = fixture.policies
			}
			got := tr.Invoke(context.Background(), input)
			if mode == "private" {
				require.NotNil(t, got.Response)
				require.Equal(t, int32(1), requests.Load())
			} else {
				require.Equal(t, entity.HookSecurityError, got.Outcome.Code)
				require.Zero(t, requests.Load())
				if mode != "peer_mismatch" {
					require.Zero(t, dials.Load())
				}
			}
		})
	}
}

func TestHookWorkspaceLegacyModeSwitchFailsDuringSecretRead(t *testing.T) {
	loader := &hookConfigLoader{value: `{` + hookRuntimeSecurity + `}`}
	r := NewKeyResolver(NewRuntimeConfigProvider(loader, true), hookSecretFunc(func(context.Context, string) ([]byte, error) {
		loader.value = `{"workspace_allowlist":[42],` + hookRuntimeSecurity + `}`
		return []byte("old-secret"), nil
	}))
	key, err := r.Resolve(context.Background(), entity.HookKeyBinding{WorkspaceID: 42, URL: "https://example.com", Environment: entity.HookEnvironmentProd})
	require.EqualError(t, err, "hook signing key unavailable")
	require.Empty(t, key)
}

func TestHookWorkspaceLegacyPolicyFingerprintUnchanged(t *testing.T) {
	legacyJSON := `{"WorkspaceID":42,"Host":"example.com","Port":443,"Environment":"Prod","Lane":"","PrivateCIDRs":null}`
	want := sha256.Sum256([]byte(legacyJSON))
	got, err := endpointPolicyFingerprint(EndpointPolicy{WorkspaceID: 42, Host: "example.com", Port: 443, Environment: "Prod"})
	require.NoError(t, err)
	require.Equal(t, hex.EncodeToString(want[:]), got)
}
