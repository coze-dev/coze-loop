// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confviper "github.com/coze-dev/coze-loop/backend/pkg/conf/viper"
	"github.com/stretchr/testify/require"
)

// Get is the current-value path on both file and TCC loaders; UnmarshalKey in
// Commercial caches values for ten seconds and must not cache Hook revocations.
type hookConfigLoader struct {
	conf.IConfigLoader
	value any
}

func (l *hookConfigLoader) Get(_ context.Context, key string) any {
	if key != "lifecycle_hook" {
		panic("wrong runtime configuration key")
	}
	return l.value
}

const hookRuntimeSecurity = `"endpoint_policy":[{"workspace_id":42,"host":"Example.COM.","port":443,"environment":"Prod","private_cidrs":["10.1.0.0/16"]}],"signing_key_refs":[{"workspace_id":42,"host":"example.com","port":443,"environment":"Prod","key_id":"hook-v1","key_ref":"dedicated-ref-v1"}]`

func TestHookRuntimeDefaultsAndWorkerInstallation(t *testing.T) {
	for _, installed := range []bool{false, true} {
		p := NewRuntimeConfigProvider(&hookConfigLoader{value: `{}`}, installed)
		got, err := p.GetRuntimeConfig(context.Background())
		require.NoError(t, err)
		require.Equal(t, entity.HookRuntimeConfig{AdmissionEnabled: false, WorkerEnabled: installed, WorkerConcurrency: 8, WorkspaceConcurrency: 2, ScanIntervalSeconds: 10, ScanBatchSize: 100, LeaseSeconds: 30, RenewSeconds: 10, IdentityEnrichmentTimeoutMS: 500, RetentionDays: 30}, got)
		_, err = p.EndpointPolicies(context.Background())
		require.Error(t, err)
	}
	loader := &hookConfigLoader{value: `{"worker_enabled":true}`}
	p := NewRuntimeConfigProvider(loader, false)
	got, err := p.GetRuntimeConfig(context.Background())
	require.NoError(t, err)
	require.False(t, got.WorkerEnabled)
	loader.value = `{"worker_enabled":false}`
	got, err = NewRuntimeConfigProvider(loader, true).GetRuntimeConfig(context.Background())
	require.NoError(t, err)
	require.False(t, got.WorkerEnabled)
}

func TestHookRuntimeCapacityMatchesWorker(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"maximum", `{"worker_concurrency":128,"workspace_concurrency":32,"scan_interval_seconds":3600,"scan_batch_size":100}`, true},
		{"worker_overflow", `{"worker_concurrency":129}`, false},
		{"workspace_overflow", `{"worker_concurrency":128,"workspace_concurrency":33}`, false},
		{"interval_overflow", `{"scan_interval_seconds":3601}`, false},
		{"page_overflow", `{"scan_batch_size":101}`, false},
		{"space_exceeds_process", `{"worker_concurrency":2,"workspace_concurrency":3}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewRuntimeConfigProvider(&hookConfigLoader{value: tc.raw}, true).GetRuntimeConfig(context.Background())
			if !tc.valid {
				require.Error(t, err, "provider must reject, not clamp, unsupported capacity")
				require.Equal(t, entity.HookRuntimeConfig{}, got)
				return
			}
			require.NoError(t, err)
			require.NoError(t, entity.ValidateHookWorkerConfig(got))
			require.Equal(t, int32(128), got.WorkerConcurrency)
			require.Equal(t, int32(32), got.WorkspaceConcurrency)
			require.Equal(t, int32(3600), got.ScanIntervalSeconds)
			require.Equal(t, int32(100), got.ScanBatchSize)
		})
	}
}

func TestHookRuntimeLeaseRangeUnchanged(t *testing.T) {
	for _, raw := range []string{
		`{"lease_seconds":3,"renew_seconds":1}`,
		`{"lease_seconds":20,"renew_seconds":5}`,
		`{"lease_seconds":60,"renew_seconds":20}`,
		`{"lease_seconds":2147483647,"renew_seconds":1073741823}`,
	} {
		got, err := NewRuntimeConfigProvider(&hookConfigLoader{value: raw}, true).GetRuntimeConfig(context.Background())
		require.NoError(t, err)
		require.NoError(t, entity.ValidateHookLeaseTiming(got.LeaseSeconds, got.RenewSeconds))
	}
}

func TestHookRuntimeOverridesAndCurrentPolicies(t *testing.T) {
	loader := &hookConfigLoader{value: `{"admission_enabled":true,"worker_concurrency":4,"workspace_concurrency":1,"scan_interval_seconds":5,"scan_batch_size":25,"lease_seconds":20,"renew_seconds":5,"identity_enrichment_timeout_ms":100,"retention_days":10,` + hookRuntimeSecurity + `}`}
	p := NewRuntimeConfigProvider(loader, true)
	got, err := p.GetRuntimeConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, entity.HookRuntimeConfig{AdmissionEnabled: true, WorkerEnabled: true, WorkerConcurrency: 4, WorkspaceConcurrency: 1, ScanIntervalSeconds: 5, ScanBatchSize: 25, LeaseSeconds: 20, RenewSeconds: 5, IdentityEnrichmentTimeoutMS: 100, RetentionDays: 10}, got)
	policies, err := p.EndpointPolicies(context.Background())
	require.NoError(t, err)
	require.Len(t, policies, 1)
	require.Equal(t, "example.com", policies[0].Host)
	require.Equal(t, "10.1.0.0/16", policies[0].PrivateCIDRs[0].String())
	policies[0].Host = "changed.example"
	policies[0].PrivateCIDRs = nil
	policies, err = p.EndpointPolicies(context.Background())
	require.NoError(t, err)
	require.Equal(t, "example.com", policies[0].Host)
	require.Len(t, policies[0].PrivateCIDRs, 1)
	loader.value = `{"worker_enabled":false}`
	got, err = p.GetRuntimeConfig(context.Background())
	require.NoError(t, err)
	require.False(t, got.WorkerEnabled)
	require.False(t, got.AdmissionEnabled)
	_, err = p.EndpointPolicies(context.Background())
	require.Error(t, err)
}

func TestHookRuntimeStrictFailClosed(t *testing.T) {
	for _, raw := range []any{
		nil, ``, `null`, `[]`, `{"admission_enabled":"true"}`, `{"admission_enabled":true}`, `{"unknown":true}`, `{"worker_enabled":true,"worker_enabled":false}`, `{} {}`, "{\"x\":\"\xff\"}",
		`{"worker_concurrency":0}`, `{"worker_concurrency":-1}`, `{"worker_concurrency":2147483648}`, `{"workspace_concurrency":0}`, `{"workspace_concurrency":9}`, `{"scan_batch_size":101}`, `{"scan_interval_seconds":0}`, `{"scan_batch_size":0}`, `{"lease_seconds":0}`, `{"lease_seconds":20}`, `{"renew_seconds":0}`, `{"renew_seconds":15}`, `{"identity_enrichment_timeout_ms":501}`, `{"identity_enrichment_timeout_ms":0}`, `{"retention_days":0}`,
		`{"endpoint_policy":[null]}`, `{"signing_key_refs":[null]}`,
		`{` + strings.ReplaceAll(hookRuntimeSecurity, `"port":443`, `"port":0`) + `}`,
		`{` + strings.ReplaceAll(hookRuntimeSecurity, `"environment":"Prod"`, `"environment":"PPE"`) + `}`,
		`{` + strings.ReplaceAll(hookRuntimeSecurity, `"10.1.0.0/16"`, `"0.0.0.0/0"`) + `}`,
		`{` + strings.ReplaceAll(hookRuntimeSecurity, `"10.1.0.0/16"`, `"10.1.0.1/16"`) + `}`,
		`{` + strings.ReplaceAll(hookRuntimeSecurity, `"key_id":"hook-v1"`, `"key_id":"bad key"`) + `}`,
		`{` + strings.ReplaceAll(hookRuntimeSecurity, `"key_ref":"dedicated-ref-v1"`, `"key_ref":""`) + `}`,
	} {
		p := NewRuntimeConfigProvider(&hookConfigLoader{value: raw}, true)
		got, err := p.GetRuntimeConfig(context.Background())
		require.Error(t, err, "%q", raw)
		require.Equal(t, entity.HookRuntimeConfig{}, got)
		policies, err := p.EndpointPolicies(context.Background())
		require.Error(t, err)
		require.Empty(t, policies)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewRuntimeConfigProvider(&hookConfigLoader{value: `{}`}, true).GetRuntimeConfig(ctx)
	require.Error(t, err)
	var loader *hookConfigLoader
	_, err = NewRuntimeConfigProvider(loader, true).GetRuntimeConfig(context.Background())
	require.Error(t, err)
}

func TestHookRuntimeFileLoaderMapShape(t *testing.T) {
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(`{`+hookRuntimeSecurity+`}`), &value))
	p := NewRuntimeConfigProvider(&hookConfigLoader{value: value}, false)
	policies, err := p.EndpointPolicies(context.Background())
	require.NoError(t, err)
	require.Equal(t, "example.com", policies[0].Host)
}

func TestHookRuntimeExistingFileLoader(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hook.json"), []byte(`{"lifecycle_hook":{`+hookRuntimeSecurity+`}}`), 0600))
	loader, err := confviper.NewFileConfLoader("hook.json", confviper.WithConfigPath(dir))
	require.NoError(t, err)
	config := NewRuntimeConfigProvider(loader, false)
	policies, err := config.EndpointPolicies(context.Background())
	require.NoError(t, err)
	require.Equal(t, "example.com", policies[0].Host)
	runtime, err := config.GetRuntimeConfig(context.Background())
	require.NoError(t, err)
	require.False(t, runtime.AdmissionEnabled)
	require.False(t, runtime.WorkerEnabled)
}
