// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

type hookSecretFunc func(context.Context, string) ([]byte, error)

func (f hookSecretFunc) ReadSigningSecret(ctx context.Context, ref string) ([]byte, error) {
	return f(ctx, ref)
}

func TestHookKeyResolverBindingRotationAndOwnedSecret(t *testing.T) {
	loader := &hookConfigLoader{value: `{` + hookRuntimeSecurity + `}`}
	config := NewRuntimeConfigProvider(loader, true)
	secret := []byte("dedicated-test-secret-v1")
	r := NewKeyResolver(config, hookSecretFunc(func(_ context.Context, ref string) ([]byte, error) {
		switch ref {
		case "dedicated-ref-v1":
			return secret, nil
		case "dedicated-ref-v2":
			return []byte("dedicated-test-secret-v2"), nil
		default:
			t.Fatalf("untrusted key reference: %q", ref)
			return nil, nil
		}
	}))
	binding := entity.HookKeyBinding{WorkspaceID: 42, URL: "https://EXAMPLE.com.:443/path?x=1", Environment: entity.HookEnvironmentProd}
	got, err := r.Resolve(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, "hook-v1", got.KeyID)
	require.Equal(t, "dedicated-test-secret-v1", string(got.Secret))
	secret[0] = 'x'
	require.Equal(t, "dedicated-test-secret-v1", string(got.Secret))
	loader.value = `{` + strings.ReplaceAll(strings.ReplaceAll(hookRuntimeSecurity, "hook-v1", "hook-v2"), "dedicated-ref-v1", "dedicated-ref-v2") + `}`
	got, err = r.Resolve(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, "hook-v2", got.KeyID)
	require.Equal(t, "dedicated-test-secret-v2", string(got.Secret))
	loader.value = `{}`
	got, err = r.Resolve(context.Background(), binding)
	require.Error(t, err)
	require.Equal(t, entity.HookSigningKey{}, got)
}

func TestHookKeyResolverRejectsUnboundRequestsBeforeSecretRead(t *testing.T) {
	r := NewKeyResolver(NewRuntimeConfigProvider(&hookConfigLoader{value: `{` + hookRuntimeSecurity + `}`}, false), hookSecretFunc(func(context.Context, string) ([]byte, error) {
		t.Fatal("unbound request read a secret")
		return nil, nil
	}))
	for _, change := range []func(*entity.HookKeyBinding){
		func(b *entity.HookKeyBinding) { b.WorkspaceID = 0 },
		func(b *entity.HookKeyBinding) { b.WorkspaceID = 43 },
		func(b *entity.HookKeyBinding) { b.URL = "https://evil.example/path" },
		func(b *entity.HookKeyBinding) { b.URL = "https://example.com:444/path" },
		func(b *entity.HookKeyBinding) { b.URL = "http://example.com/path" },
		func(b *entity.HookKeyBinding) { b.URL = "https://user:password@example.com/path" },
		func(b *entity.HookKeyBinding) { b.URL = "https://example.com/path#" },
		func(b *entity.HookKeyBinding) { b.URL = "https://example.com/%zz" },
		func(b *entity.HookKeyBinding) { b.Environment = entity.HookEnvironmentPPE; b.Lane = "ppe_test" },
		func(b *entity.HookKeyBinding) { b.Environment = entity.HookEnvironmentBOE; b.Lane = "boe_test" },
		func(b *entity.HookKeyBinding) { b.Environment = "canary" },
		func(b *entity.HookKeyBinding) { b.Lane = "ppe_test" },
	} {
		binding := entity.HookKeyBinding{WorkspaceID: 42, URL: "https://example.com/path", Environment: entity.HookEnvironmentProd}
		change(&binding)
		got, err := r.Resolve(context.Background(), binding)
		require.EqualError(t, err, "hook signing key unavailable")
		require.Equal(t, entity.HookSigningKey{}, got)
	}
}

func TestHookKeyResolverMissingSecretAndRevocationDuringRead(t *testing.T) {
	for _, mode := range []string{"empty", "error", "revoked", "rotated", "policy changed", "canceled", "typed nil"} {
		t.Run(mode, func(t *testing.T) {
			loader := &hookConfigLoader{value: `{` + hookRuntimeSecurity + `}`}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var secrets hookSecretFunc = func(context.Context, string) ([]byte, error) {
				switch mode {
				case "empty":
					return nil, nil
				case "error":
					return nil, fmt.Errorf("sensitive upstream detail")
				case "revoked":
					loader.value = `{}`
				case "rotated":
					loader.value = `{` + strings.ReplaceAll(hookRuntimeSecurity, "dedicated-ref-v1", "dedicated-ref-v2") + `}`
				case "policy changed":
					loader.value = `{` + strings.ReplaceAll(hookRuntimeSecurity, "10.1.0.0/16", "10.1.1.0/24") + `}`
				case "canceled":
					cancel()
				}
				return []byte("must-not-escape"), nil
			}
			if mode == "typed nil" {
				secrets = nil
			}
			r := NewKeyResolver(NewRuntimeConfigProvider(loader, true), secrets)
			got, err := r.Resolve(ctx, entity.HookKeyBinding{WorkspaceID: 42, URL: "https://example.com", Environment: entity.HookEnvironmentProd})
			require.EqualError(t, err, "hook signing key unavailable")
			require.Equal(t, entity.HookSigningKey{}, got)
		})
	}
}

func TestHookKeyResolverPPELanesAndBOEAreDistinct(t *testing.T) {
	for _, env := range []entity.HookEnvironment{entity.HookEnvironmentPPE, entity.HookEnvironmentBOE} {
		p := NewRuntimeConfigProvider(&hookConfigLoader{value: `{` + strings.ReplaceAll(hookRuntimeSecurity, `"environment":"Prod"`, `"environment":"`+string(env)+`","lane":"lane-a"`) + `}`}, false)
		r := NewKeyResolver(p, hookSecretFunc(func(context.Context, string) ([]byte, error) { return []byte("key"), nil }))
		binding := entity.HookKeyBinding{WorkspaceID: 42, URL: "https://example.com", Environment: env, Lane: "lane-a"}
		key, err := r.Resolve(context.Background(), binding)
		require.NoError(t, err)
		require.Equal(t, "hook-v1", key.KeyID)
		binding.Lane = "lane-b"
		_, err = r.Resolve(context.Background(), binding)
		require.Error(t, err)
		binding.Environment, binding.Lane = entity.HookEnvironmentProd, ""
		_, err = r.Resolve(context.Background(), binding)
		require.Error(t, err)
	}
}

func TestHookRuntimeRejectsAmbiguousOrUnpairedSecurity(t *testing.T) {
	policy := `{"workspace_id":42,"host":"example.com","port":443,"environment":"Prod"}`
	key := `{"workspace_id":42,"host":"example.com","port":443,"environment":"Prod","key_id":"v1","key_ref":"ref"}`
	for _, raw := range []string{
		`{"endpoint_policy":[` + policy + `,` + strings.ReplaceAll(policy, "example.com", "EXAMPLE.com.") + `],"signing_key_refs":[` + key + `]}`,
		`{"endpoint_policy":[` + policy + `],"signing_key_refs":[` + key + `,` + key + `]}`,
		`{"endpoint_policy":[` + policy + `]}`,
		`{"signing_key_refs":[` + key + `]}`,
		`{"endpoint_policy":[` + policy + `],"signing_key_refs":[` + strings.ReplaceAll(key, `"workspace_id":42`, `"workspace_id":43`) + `]}`,
		`{` + strings.ReplaceAll(hookRuntimeSecurity, `"Example.COM."`, `"127.0.0.1"`) + `}`,
	} {
		p := NewRuntimeConfigProvider(&hookConfigLoader{value: raw}, true)
		_, err := p.GetRuntimeConfig(context.Background())
		require.Error(t, err)
	}
}

func TestHookConfiguredTransportSignsWithResolvedKey(t *testing.T) {
	var received atomic.Int32
	tr, input, _ := httpTLSFixture(t, func(w http.ResponseWriter, req *http.Request) {
		received.Add(1)
		raw, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		hash := sha256.Sum256(raw)
		message := strings.Join([]string{"POST", req.RequestURI, req.Header.Get("X-Fornax-Hook-Timestamp"), req.Header.Get("X-Fornax-Hook-Nonce"), "op", hex.EncodeToString(hash[:])}, "\n")
		mac := hmac.New(sha256.New, []byte("integration-test-key"))
		_, _ = mac.Write([]byte(message))
		require.Equal(t, "tls-key", req.Header.Get("X-Fornax-Hook-Key-Id"))
		require.Equal(t, hex.EncodeToString(mac.Sum(nil)), req.Header.Get("X-Fornax-Hook-Signature"))
		writeHTTPSuccess(w)
	})
	u, err := url.Parse(*input.Config.InvokeHTTPInfo.URL)
	require.NoError(t, err)
	loader := &hookConfigLoader{value: fmt.Sprintf(`{"endpoint_policy":[{"workspace_id":%d,"host":"example.com","port":%s,"environment":"Prod"}],"signing_key_refs":[{"workspace_id":%d,"host":"example.com","port":%s,"environment":"Prod","key_id":"tls-key","key_ref":"tls-ref"}]}`, input.WorkspaceID, u.Port(), input.WorkspaceID, u.Port())}
	config := NewRuntimeConfigProvider(loader, true)
	tr.keys = NewKeyResolver(config, hookSecretFunc(func(_ context.Context, ref string) ([]byte, error) {
		require.Equal(t, "tls-ref", ref)
		return []byte("integration-test-key"), nil
	}))
	tr.policies = config.EndpointPolicies
	got := tr.Invoke(context.Background(), input)
	require.NotNil(t, got.Response)
	require.Equal(t, "succeeded", string(got.Response.GetStatus()))
	require.Equal(t, int32(1), received.Load())
	loader.value = `{}`
	got = tr.Invoke(context.Background(), input)
	require.Equal(t, entity.HookSecurityError, got.Outcome.Code)
	require.Equal(t, int32(1), received.Load())
}
