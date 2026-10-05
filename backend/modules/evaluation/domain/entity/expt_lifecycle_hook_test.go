// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
)

func TestLifecycleHookDefaultsAndOptionalStages(t *testing.T) {
	for _, raw := range []string{"null", "{}", `{"before":null,"after":null}`} {
		t.Run(raw, func(t *testing.T) {
			var input *LifecycleHookConf
			require.NoError(t, json.Unmarshal([]byte(raw), &input))
			got, err := ResolveLifecycleHookConf(nil, input)
			require.NoError(t, err)
			if got != nil {
				require.Nil(t, got.Before)
				require.Nil(t, got.After)
			}
		})
	}
	input := &LifecycleHookConf{Before: &HookConfig{}, After: &HookConfig{}}
	got, err := ResolveLifecycleHookConf(nil, input)
	require.NoError(t, err)
	want := &HookConfig{
		Enabled: gptr.Of(false), AccessProtocol: gptr.Of(HookAccessProtocolHTTP),
		Environment: gptr.Of(HookEnvironmentProd), InvokeHTTPInfo: &HookHTTPInfo{Method: gptr.Of("post")},
		ParametersJSON: gptr.Of("{}"), TimeoutSeconds: gptr.Of(int32(180)),
		Retry:     &HookRetryConf{Enabled: gptr.Of(true), MaxRetries: gptr.Of(int32(1))},
		OnFailure: gptr.Of(HookFailurePolicyBlock),
	}
	require.Equal(t, want, got.Before)
	want.OnFailure = nil
	require.Equal(t, want, got.After)
	require.Equal(t, &LifecycleHookConf{Before: &HookConfig{}, After: &HookConfig{}}, input)
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	require.JSONEq(t, `{"before":{},"after":{}}`, string(encoded))
}

func TestLifecycleHookTimeoutAndRetry(t *testing.T) {
	for _, tc := range []struct{ input, want int32 }{{0, 180}, {1, 1}, {1200, 1200}} {
		original := tc.input
		got, err := ResolveLifecycleHookConf(nil, &LifecycleHookConf{Before: &HookConfig{TimeoutSeconds: &tc.input}})
		require.NoError(t, err)
		require.Equal(t, tc.want, *got.Before.TimeoutSeconds)
		require.Equal(t, original, tc.input)
	}
	for _, enabled := range []bool{false, true} {
		for _, count := range []int32{1, 10} {
			got, err := ResolveLifecycleHookConf(nil, &LifecycleHookConf{Before: &HookConfig{Retry: &HookRetryConf{Enabled: &enabled, MaxRetries: &count}}})
			require.NoError(t, err)
			require.Equal(t, count, *got.Before.Retry.MaxRetries)
			wantEffective := int32(0)
			if enabled {
				wantEffective = count
			}
			require.Equal(t, wantEffective, got.Before.Retry.EffectiveMaxRetries())
		}
		for _, count := range []int32{-1, 0, 11} {
			_, err := ResolveLifecycleHookConf(nil, &LifecycleHookConf{Before: &HookConfig{Retry: &HookRetryConf{Enabled: &enabled, MaxRetries: &count}}})
			require.Error(t, err)
		}
	}
	for _, retry := range []*HookRetryConf{nil, {}, {Enabled: gptr.Of(false)}} {
		got, err := ResolveLifecycleHookConf(nil, &LifecycleHookConf{Before: &HookConfig{Retry: retry}})
		require.NoError(t, err)
		require.Equal(t, int32(1), *got.Before.Retry.MaxRetries)
	}
	require.Equal(t, int32(1), (*HookRetryConf)(nil).EffectiveMaxRetries())
	require.Equal(t, int32(1), (&HookRetryConf{}).EffectiveMaxRetries())
}

func TestLifecycleHookParameters(t *testing.T) {
	keys := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf(`"k%d":null`, i)
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"empty", "", true}, {"whitespace", " \t\n", true},
		{"lexical numbers and Unicode", ` {"数":9007199254740993,"e":1.2300e+20,"v":[false,null,{"𠮷":"\u4f60"}]} `, true},
		{"16KiB", `{"x":"` + strings.Repeat("a", 16376) + `"}`, true},
		{"over16KiB", `{"x":"` + strings.Repeat("a", 16377) + `"}`, false},
		{"UTF8 bytes", `{"x":"` + strings.Repeat("中", 5460) + `"}`, false},
		{"invalid UTF8", "{\"x\":\"\xff\"}", false},
		{"rootnull", "null", false}, {"array", "[]", false}, {"scalar", "true", false},
		{"invalid", "{", false}, {"trailing object", "{} {}", false}, {"trailing junk", "{}x", false},
		{"duplicate", `{"a":1,"a":2}`, false}, {"escaped duplicate", `{"a":1,"\u0061":2}`, false},
		{"nested duplicate", `{"x":[{"a":1,"a":2}]}`, false},
		{"same key different objects", `{"a":{"x":1},"b":{"x":2}}`, true},
		{"256keys", keys(256), true}, {"257keys", keys(257), false},
		{"total nested keys", `{"parent":` + keys(256) + `}`, false},
		{"depth8", strings.Repeat(`{"x":`, 8) + "0" + strings.Repeat("}", 8), true},
		{"depth9", strings.Repeat(`{"x":`, 9) + "0" + strings.Repeat("}", 9), false},
		{"array depth8", `{"x":` + strings.Repeat("[", 7) + "0" + strings.Repeat("]", 7) + "}", true},
		{"array depth9", `{"x":` + strings.Repeat("[", 8) + "0" + strings.Repeat("]", 8) + "}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := &LifecycleHookConf{Before: &HookConfig{ParametersJSON: &tc.raw}}
			got, err := ResolveLifecycleHookConf(nil, input)
			if !tc.valid {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			want := tc.raw
			if tc.name == "empty" || tc.name == "whitespace" {
				want = "{}"
			}
			require.Equal(t, want, *got.Before.ParametersJSON)
			require.Equal(t, tc.raw, *input.Before.ParametersJSON)
		})
	}
}

func TestLifecycleHookEndpointAndEnums(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"enabled needs URL", `{"before":{"enabled":true}}`, false},
		{"disabled no URL", `{"before":{"enabled":false}}`, true},
		{"Prod HTTPS", `{"before":{"enabled":true,"invoke_http_info":{"url":"https://example.org/hook?q=1"}}}`, true},
		{"PPE", `{"before":{"enabled":true,"environment":"PPE","lane":"test_lane","invoke_http_info":{"url":"https://example.org"}}}`, true},
		{"BOE", `{"after":{"enabled":true,"environment":"BOE","lane":"test_lane","invoke_http_info":{"url":"https://example.org"}}}`, true},
		{"syntax only", `{"before":{"enabled":true,"invoke_http_info":{"url":"https://127.0.0.1:8443/hook"}}}`, true},
		{"IPv6", `{"before":{"enabled":true,"invoke_http_info":{"url":"https://[::1]:8443/hook"}}}`, true},
		{"continue", `{"before":{"on_failure":"continue"}}`, true},
		{"after empty policy", `{"after":{"on_failure":""}}`, true},
		{"rpc", `{"before":{"access_protocol":"rpc"}}`, false},
		{"protocol enum", `{"before":{"access_protocol":"other"}}`, false},
		{"empty protocol", `{"before":{"access_protocol":""}}`, false},
		{"method", `{"before":{"invoke_http_info":{"method":"get"}}}`, false},
		{"environment", `{"before":{"environment":"prod"}}`, false},
		{"Prod lane", `{"before":{"lane":"test_lane"}}`, false},
		{"PPE lane missing", `{"before":{"enabled":true,"environment":"PPE","invoke_http_info":{"url":"https://example.org"}}}`, false},
		{"BOE lane whitespace", `{"before":{"enabled":true,"environment":"BOE","lane":" ","invoke_http_info":{"url":"https://example.org"}}}`, false},
		{"disabled PPE draft", `{"before":{"environment":"PPE"}}`, true},
		{"after block", `{"after":{"on_failure":"block"}}`, false},
		{"after continue", `{"after":{"on_failure":"continue"}}`, false},
		{"unknown failure", `{"before":{"on_failure":"ignore"}}`, false},
		{"negative timeout", `{"before":{"timeout_seconds":-1}}`, false},
		{"excess timeout", `{"before":{"timeout_seconds":1201}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input LifecycleHookConf
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &input))
			got, err := ResolveLifecycleHookConf(nil, &input)
			if tc.valid {
				require.NoError(t, err)
				require.NotNil(t, got)
			} else {
				require.Error(t, err)
				require.Nil(t, got)
			}
		})
	}
	for _, endpoint := range []string{"http://example.org", "/hook", "https://", "https://user:pass@example.org", "https://example.org/#fragment", "https://example.org/#", "https://example.org:70000", "https://example.org:", "https://example.org:bad", "https://exa mple.org", "https://example.org/" + strings.Repeat("x", 2048)} {
		for _, enabled := range []bool{false, true} {
			_, err := ResolveLifecycleHookConf(nil, &LifecycleHookConf{Before: &HookConfig{Enabled: &enabled, InvokeHTTPInfo: &HookHTTPInfo{URL: &endpoint}}})
			require.Error(t, err, endpoint)
		}
	}
	for _, raw := range []string{`{"before":{"parameters_json":{}}}`, `{"before":{"enabled":"false"}}`, `{"before":{"retry":{"max_retries":1.5}}}`} {
		var input LifecycleHookConf
		require.Error(t, json.Unmarshal([]byte(raw), &input))
	}
}

func TestLifecycleHookStageReplacementAndInputIsolation(t *testing.T) {
	baseRaw := `{"before":{"enabled":true,"invoke_http_info":{"url":"https://example.org/before"},"parameters_json":"{\"old\":1}","timeout_seconds":600,"on_failure":"continue"},"after":{"enabled":true,"environment":"PPE","lane":"test_lane","invoke_http_info":{"url":"https://example.org/after"},"parameters_json":"{\"after\":true}","retry":{"enabled":false,"max_retries":7}}}`
	var base LifecycleHookConf
	require.NoError(t, json.Unmarshal([]byte(baseRaw), &base))
	for _, raw := range []string{"null", "{}", `{"before":null,"after":null}`} {
		var override *LifecycleHookConf
		require.NoError(t, json.Unmarshal([]byte(raw), &override))
		got, err := ResolveLifecycleHookConf(&base, override)
		require.NoError(t, err)
		require.Equal(t, int32(600), *got.Before.TimeoutSeconds)
		require.Equal(t, `{"old":1}`, *got.Before.ParametersJSON)
		require.Equal(t, `{"after":true}`, *got.After.ParametersJSON)
		*got.Before.Enabled = false
		*got.Before.InvokeHTTPInfo.URL = "https://changed.org"
		*got.Before.ParametersJSON = "{}"
		*got.Before.TimeoutSeconds = 1
		*got.Before.OnFailure = HookFailurePolicyBlock
		*got.After.Environment = HookEnvironmentBOE
		*got.After.Lane = "changed"
		*got.After.Retry.Enabled = true
		*got.After.Retry.MaxRetries = 1
		encoded, err := json.Marshal(base)
		require.NoError(t, err)
		require.JSONEq(t, baseRaw, string(encoded))
	}
	override := &LifecycleHookConf{Before: &HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"new":2}`)}}
	got, err := ResolveLifecycleHookConf(&base, override)
	require.NoError(t, err)
	require.False(t, *got.Before.Enabled)
	require.Nil(t, got.Before.InvokeHTTPInfo.URL)
	require.Equal(t, int32(180), *got.Before.TimeoutSeconds)
	require.Equal(t, HookFailurePolicyBlock, *got.Before.OnFailure)
	require.Equal(t, `{"new":2}`, *got.Before.ParametersJSON)
	require.Equal(t, `{"after":true}`, *got.After.ParametersJSON)
	*got.Before.ParametersJSON = "{}"
	require.Equal(t, `{"new":2}`, *override.Before.ParametersJSON)

	bad := &LifecycleHookConf{Before: &HookConfig{}, After: &HookConfig{ParametersJSON: gptr.Of(`{"duplicate":1,"duplicate":2}`)}}
	got, err = ResolveLifecycleHookConf(&base, bad)
	require.Error(t, err)
	require.Nil(t, got)
	require.Equal(t, &HookConfig{}, bad.Before)
	encoded, err := json.Marshal(base)
	require.NoError(t, err)
	require.JSONEq(t, baseRaw, string(encoded))
}

func TestLifecycleHookDisabledPreservesProvidedConfig(t *testing.T) {
	input := &LifecycleHookConf{Before: &HookConfig{
		Enabled: gptr.Of(false), AccessProtocol: gptr.Of(HookAccessProtocolHTTP),
		InvokeHTTPInfo: &HookHTTPInfo{Method: gptr.Of("post"), URL: gptr.Of("https://example.org")},
		Environment:    gptr.Of(HookEnvironmentBOE), Lane: gptr.Of("draft_lane"),
		ParametersJSON: gptr.Of(`{"user":9007199254740993}`), TimeoutSeconds: gptr.Of(int32(1200)),
		Retry:     &HookRetryConf{Enabled: gptr.Of(false), MaxRetries: gptr.Of(int32(10))},
		OnFailure: gptr.Of(HookFailurePolicyContinue),
	}}
	got, err := ResolveLifecycleHookConf(nil, input)
	require.NoError(t, err)
	require.Equal(t, input, got)
	require.Equal(t, int32(0), got.Before.Retry.EffectiveMaxRetries())
	*got.Before.InvokeHTTPInfo.Method = "changed"
	*got.Before.AccessProtocol = HookAccessProtocolRPC
	require.Equal(t, "post", *input.Before.InvokeHTTPInfo.Method)
	require.Equal(t, HookAccessProtocolHTTP, *input.Before.AccessProtocol)
}

func TestLifecycleHookURLHostSyntax(t *testing.T) {
	for _, raw := range []string{"https://[not-an-ip]/hook", "https://::1/hook", "https://[127.0.0.1]/hook"} {
		t.Run(raw, func(t *testing.T) {
			_, err := ResolveLifecycleHookConf(nil, &LifecycleHookConf{Before: &HookConfig{InvokeHTTPInfo: &HookHTTPInfo{URL: &raw}}})
			require.Error(t, err)
		})
	}
	limitURL := "https://example.org/" + strings.Repeat("a", 2028)
	require.Len(t, limitURL, 2048)
	_, err := ResolveLifecycleHookConf(nil, &LifecycleHookConf{Before: &HookConfig{InvokeHTTPInfo: &HookHTTPInfo{URL: &limitURL}}})
	require.NoError(t, err)
}
