// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"bytes"
	"encoding/json"
	"errors"
)

var ErrInvalidHookWakeEvent = errors.New("invalid hook wake event")

// HookWakeEvent is only a hint to inspect persisted work; it carries no claim.
type HookWakeEvent struct {
	Run            HookRunKey
	OperationID    string
	ExecutionScope string
}

type hookWakePayload struct {
	WorkspaceID    int64  `json:"workspace_id"`
	ExperimentID   int64  `json:"experiment_id"`
	RunID          int64  `json:"run_id"`
	OperationID    string `json:"operation_id"`
	ExecutionScope string `json:"execution_scope"`
}

func validHookWakeEvent(event HookWakeEvent) bool {
	return event.Run.WorkspaceID > 0 && event.Run.ExperimentID > 0 && event.Run.RunID > 0 &&
		hookStorageASCII(event.OperationID, 128) && hookStorageASCII(event.ExecutionScope, 128)
}

func EncodeHookWakeEvent(event HookWakeEvent) ([]byte, error) {
	if !validHookWakeEvent(event) {
		return nil, ErrInvalidHookWakeEvent
	}
	payload := hookWakePayload{event.Run.WorkspaceID, event.Run.ExperimentID, event.Run.RunID, event.OperationID, event.ExecutionScope}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return nil, ErrInvalidHookWakeEvent
	}
	return bytes.TrimSuffix(body.Bytes(), []byte("\n")), nil
}

func DecodeHookWakeEvent(body []byte) (HookWakeEvent, error) {
	if len(body) == 0 || len(body) > 1024 {
		return HookWakeEvent{}, ErrInvalidHookWakeEvent
	}
	// Reuse the strict object scanner for UTF-8, duplicate keys and trailing data.
	if _, err := normalizeHookParameters(string(body)); err != nil {
		return HookWakeEvent{}, ErrInvalidHookWakeEvent
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || len(fields) != 5 {
		return HookWakeEvent{}, ErrInvalidHookWakeEvent
	}
	for _, key := range []string{"workspace_id", "experiment_id", "run_id", "operation_id", "execution_scope"} {
		if _, ok := fields[key]; !ok {
			return HookWakeEvent{}, ErrInvalidHookWakeEvent
		}
	}
	var payload hookWakePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return HookWakeEvent{}, ErrInvalidHookWakeEvent
	}
	event := HookWakeEvent{Run: HookRunKey{payload.WorkspaceID, payload.ExperimentID, payload.RunID}, OperationID: payload.OperationID, ExecutionScope: payload.ExecutionScope}
	if !validHookWakeEvent(event) {
		return HookWakeEvent{}, ErrInvalidHookWakeEvent
	}
	return event, nil
}
