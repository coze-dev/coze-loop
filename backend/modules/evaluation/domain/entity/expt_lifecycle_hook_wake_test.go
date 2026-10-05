// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const wakeFixture = `{"workspace_id":1,"experiment_id":2,"run_id":9007199254740993,"operation_id":"hook_42","execution_scope":"ppe_hooks"}`

func TestHookWakeCodecMinimalPayload(t *testing.T) {
	want := HookWakeEvent{Run: HookRunKey{1, 2, 9007199254740993}, OperationID: "hook_42", ExecutionScope: "ppe_hooks"}
	event, err := DecodeHookWakeEvent([]byte(wakeFixture))
	require.NoError(t, err)
	require.Equal(t, want, event)
	body, err := EncodeHookWakeEvent(want)
	require.NoError(t, err)
	require.JSONEq(t, wakeFixture, string(body))
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	require.Equal(t, "9007199254740993", string(fields["run_id"]))
}

func TestHookWakeCodecRejectsMalformedOrSensitivePayload(t *testing.T) {
	cases := map[string]string{
		"empty": "", "null": "null", "array": "[]", "missing": `{}`,
		"trailing": wakeFixture + `{}`, "malformed": `{secret@example.com`,
		"oversize":          strings.Repeat(" ", 1025) + wakeFixture,
		"invalid_utf8":      strings.Replace(wakeFixture, "hook_42", "hook_\xff", 1),
		"duplicate":         strings.Replace(wakeFixture, `"workspace_id":1`, `"workspace_id":9,"workspace_id":1`, 1),
		"escaped_duplicate": strings.Replace(wakeFixture, `"workspace_id":1`, `"workspace_id":9,"workspace_\u0069d":1`, 1),
		"wrong_case":        strings.Replace(wakeFixture, "workspace_id", "Workspace_ID", 1),
	}
	for _, field := range []string{"workspace_id", "experiment_id", "run_id"} {
		for _, value := range []string{`"1"`, "null", "true", "{}", "[]", "0", "-1", "1.5", "1e2", "9223372036854775808"} {
			original := `"` + field + `":1`
			if field == "experiment_id" {
				original = `"experiment_id":2`
			}
			if field == "run_id" {
				original = `"run_id":9007199254740993`
			}
			cases[field+"/"+value] = strings.Replace(wakeFixture, original, `"`+field+`":`+value, 1)
		}
	}
	for _, field := range []string{"operation_id", "execution_scope"} {
		old := `"hook_42"`
		if field == "execution_scope" {
			old = `"ppe_hooks"`
		}
		for _, value := range []string{`""`, `null`, `1`, `true`, `{}`, `[]`, `" leading"`, `"embedded space"`, `"line\nfeed"`, `"中文"`, `"` + strings.Repeat("a", 129) + `"`} {
			cases[field+"/"+value] = strings.Replace(wakeFixture, old, value, 1)
		}
	}
	for _, field := range []string{"config", "parameters", "email", "secret", "response", "attempt", "generation", "phase", "status", "run", "authorization"} {
		cases["extra/"+field] = strings.TrimSuffix(wakeFixture, "}") + `,"` + field + `":"secret@example.com"}`
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			event, err := DecodeHookWakeEvent([]byte(raw))
			require.ErrorIs(t, err, ErrInvalidHookWakeEvent)
			require.Equal(t, HookWakeEvent{}, event)
			require.NotContains(t, err.Error(), "secret@example.com")
		})
	}
}

func TestHookWakeEncodeValidatesIdentity(t *testing.T) {
	valid := HookWakeEvent{Run: HookRunKey{1, 2, 3}, OperationID: strings.Repeat("a", 128), ExecutionScope: strings.Repeat("b", 128)}
	_, err := EncodeHookWakeEvent(valid)
	require.NoError(t, err)
	for name, mutate := range map[string]func(*HookWakeEvent){
		"workspace":         func(e *HookWakeEvent) { e.Run.WorkspaceID = 0 },
		"experiment":        func(e *HookWakeEvent) { e.Run.ExperimentID = -1 },
		"run":               func(e *HookWakeEvent) { e.Run.RunID = 0 },
		"operation":         func(e *HookWakeEvent) { e.OperationID = "" },
		"scope":             func(e *HookWakeEvent) { e.ExecutionScope = "" },
		"scope_length":      func(e *HookWakeEvent) { e.ExecutionScope += "b" },
		"operation_length":  func(e *HookWakeEvent) { e.OperationID += "a" },
		"operation_control": func(e *HookWakeEvent) { e.OperationID = "op\nsecret" },
		"scope_space":       func(e *HookWakeEvent) { e.ExecutionScope = "ppe hooks" },
	} {
		t.Run(name, func(t *testing.T) {
			event := valid
			mutate(&event)
			body, err := EncodeHookWakeEvent(event)
			require.ErrorIs(t, err, ErrInvalidHookWakeEvent)
			require.Nil(t, body)
		})
	}
}
