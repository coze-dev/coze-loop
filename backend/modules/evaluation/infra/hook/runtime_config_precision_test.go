// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confviper "github.com/coze-dev/coze-loop/backend/pkg/conf/viper"
	"github.com/stretchr/testify/require"
)

func runtimePrecisionConfig(idJSON string) string {
	return fmt.Sprintf(`{"endpoint_policy":[{"workspace_id":%s,"host":"example.com","port":443,"environment":"Prod"}],"signing_key_refs":[{"workspace_id":%s,"host":"example.com","port":443,"environment":"Prod","key_id":"synthetic-key","key_ref":"synthetic-ref"}]}`, idJSON, idJSON)
}

func TestHookRuntimeWorkspaceIDPrecisionBySource(t *testing.T) {
	for _, tt := range []struct {
		name, source, idJSON string
		wantID               int64
		reject               bool
	}{
		{"TCC raw integer", "raw", "7590120691407168258", 7590120691407168258, false},
		{"TCC raw string", "raw", `"7590120691407168258"`, 7590120691407168258, false},
		{"JSON file unsafe integer", "json", "7590120691407168258", 0, true},
		{"JSON file exact string", "json", `"7590120691407168258"`, 7590120691407168258, false},
		{"YAML file integer", "yaml", "7590120691407168258", 7590120691407168258, false},
		{"YAML file floating ID", "yaml", "7590120691407168258.0", 0, true},
		{"JSON file safe float boundary", "json", "9007199254740991", 9007199254740991, false},
		{"JSON file ambiguous float boundary", "json", "9007199254740992", 0, true},
		{"JSON file rounded down neighbor", "json", "9007199254740993", 0, true},
		{"raw exact boundary integer", "raw", "9007199254740993", 9007199254740993, false},
		{"raw max i64", "raw", "9223372036854775807", 9223372036854775807, false},
		{"JSON file max i64 string", "json", `"9223372036854775807"`, 9223372036854775807, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			security := runtimePrecisionConfig(tt.idJSON)
			var loader conf.IConfigLoader = &hookConfigLoader{value: security}
			if tt.source != "raw" {
				dir := t.TempDir()
				name, body := "hook.json", `{"lifecycle_hook":`+security+`}`
				if tt.source == "yaml" {
					name, body = "hook.yaml", "lifecycle_hook: "+security+"\n"
				}
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0600))
				var err error
				loader, err = confviper.NewFileConfLoader(name, confviper.WithConfigPath(dir))
				require.NoError(t, err)
			}
			cfg := NewRuntimeConfigProvider(loader, false)
			policies, err := cfg.EndpointPolicies(context.Background())
			if tt.reject {
				require.Error(t, err, "an already-rounded map must not authorize any workspace")
				require.Empty(t, policies)
				runtime, err := cfg.GetRuntimeConfig(context.Background())
				require.Error(t, err)
				require.Equal(t, entity.HookRuntimeConfig{}, runtime)
				return
			}
			require.NoError(t, err)
			require.Len(t, policies, 1)
			require.Equal(t, tt.wantID, policies[0].WorkspaceID)
			key, err := NewKeyResolver(cfg, hookSecretFunc(func(_ context.Context, ref string) ([]byte, error) {
				require.Equal(t, "synthetic-ref", ref)
				return []byte("synthetic-only"), nil
			})).Resolve(context.Background(), entity.HookKeyBinding{WorkspaceID: tt.wantID, URL: "https://example.com", Environment: entity.HookEnvironmentProd})
			require.NoError(t, err)
			require.Equal(t, "synthetic-key", key.KeyID)
		})
	}
}

func TestHookRuntimeWorkspaceIDRejectsInvalidRepresentations(t *testing.T) {
	for _, idJSON := range []string{`""`, `"0"`, `"-1"`, `"+42"`, `" 42"`, `"42 "`, `"1e3"`, `"1.5"`, `"9223372036854775808"`, "0", "-1", "1.5", "1e3", "null", "true", "9223372036854775808"} {
		cfg := NewRuntimeConfigProvider(&hookConfigLoader{value: runtimePrecisionConfig(idJSON)}, false)
		_, err := cfg.EndpointPolicies(context.Background())
		require.Error(t, err, idJSON)
	}
}

func TestHookRuntimeMapRejectsUnsafeNumericWorkspaceIDs(t *testing.T) {
	for _, value := range []any{float64(9007199254740992), float32(16777216), 42.5, math.Inf(1), math.NaN()} {
		entry := map[string]any{"workspace_id": value, "host": "example.com", "port": 443, "environment": "Prod"}
		key := map[string]any{"workspace_id": value, "host": "example.com", "port": 443, "environment": "Prod", "key_id": "synthetic-key", "key_ref": "synthetic-ref"}
		cfg := NewRuntimeConfigProvider(&hookConfigLoader{value: map[string]any{"endpoint_policy": []map[string]any{entry}, "signing_key_refs": []map[string]any{key}}}, false)
		_, err := cfg.EndpointPolicies(context.Background())
		require.Error(t, err)
	}
	for _, value := range []any{int64(7590120691407168258), json.Number("7590120691407168258"), "7590120691407168258"} {
		entry := map[string]any{"workspace_id": value, "host": "example.com", "port": 443, "environment": "Prod"}
		key := map[string]any{"workspace_id": value, "host": "example.com", "port": 443, "environment": "Prod", "key_id": "synthetic-key", "key_ref": "synthetic-ref"}
		cfg := NewRuntimeConfigProvider(&hookConfigLoader{value: map[string]any{"endpoint_policy": []map[string]any{entry}, "signing_key_refs": []map[string]any{key}}}, false)
		policies, err := cfg.EndpointPolicies(context.Background())
		require.NoError(t, err)
		require.Equal(t, int64(7590120691407168258), policies[0].WorkspaceID)
	}
}
