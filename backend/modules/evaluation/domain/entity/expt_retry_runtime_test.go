// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetryRuntimeOverrideForcesFreshTarget(t *testing.T) {
	event := &ExptItemEvalEvent{ExptRunMode: EvaluationModeFailRetry, Ext: map[string]string{"retry_runtime_param": `{"binary_version":"v0.0.250-rc.1"}`}}
	require.True(t, event.IgnoreExistedTargetResult())
	require.True(t, event.IgnoreExistedEvaluatorResult(context.Background()))
	cloned := event.Clone()
	cloned.RetryTimes = 1
	require.False(t, cloned.IgnoreExistedTargetResult(), "automatic retry may reuse a target already run with this override")
	require.Equal(t, event.Ext, cloned.Ext)
	require.False(t, (&ExptItemEvalEvent{ExptRunMode: EvaluationModeFailRetry}).IgnoreExistedTargetResult())
}

func TestParseRetryRuntimeOverride(t *testing.T) {
	for _, raw := range []string{``, `null`, `{}`, `[]`, `{"model_name":"changed"}`, `{"binary_version":12}`, `{"binary_version":null}`, `{"binary_version":""}`, `{"binary_version":"../latest"}`, `{"binary_version":" v1"}`, `{"cua_package_url":"https://example.com/cua.zip"}`, `{"cua_package_sha256":"abc"}`, `{"cua_package_url":"file:///tmp/cua.zip","cua_package_sha256":"` + strings.Repeat("a", 64) + `"}`, `{"cua_package_url":"https://example.com/cua.zip","cua_package_sha256":"bad"}`} {
		t.Run(raw, func(t *testing.T) {
			_, err := ParseRetryRuntimeOverride(map[string]string{RetryRuntimeParamExtKey: raw})
			require.Error(t, err)
		})
	}
	_, err := ParseRetryRuntimeOverride(map[string]string{RetryRuntimeParamExtKey: `{"binary_version":"v0.0.250-rc.20261109","cua_package_url":"https://example.com/cua.zip","cua_package_sha256":"` + strings.Repeat("a", 64) + `"}`})
	require.NoError(t, err)
}

func TestMergeRetryRuntimeOverridePreservesFrozenParameters(t *testing.T) {
	original := `{"model_name":"model","effort":"max","context_window":"256k","large_id":7590132477565303810,"model_config":{"temperature":0.5},"binary_version":"old"}`
	ext := map[string]string{RetryRuntimeParamExtKey: `{"binary_version":"v0.0.250-rc.1"}`}
	merged, err := MergeRetryRuntimeOverride(original, ext)
	require.NoError(t, err)
	require.JSONEq(t, strings.Replace(original, `"old"`, `"v0.0.250-rc.1"`, 1), merged)
	require.Contains(t, merged, `7590132477565303810`, "large IDs must not be rounded through float64")
	unchanged, err := MergeRetryRuntimeOverride(original, nil)
	require.NoError(t, err)
	require.Equal(t, original, unchanged)
	_, err = MergeRetryRuntimeOverride(`[]`, ext)
	require.Error(t, err)
	for _, original := range []string{"", "null"} {
		merged, err := MergeRetryRuntimeOverride(original, ext)
		require.NoError(t, err)
		require.JSONEq(t, ext[RetryRuntimeParamExtKey], merged)
	}
}
