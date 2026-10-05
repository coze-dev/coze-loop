// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"maps"
	"math"
	"strings"
	"unicode/utf8"
)

// HookScheduleSeed is private recovery data captured before the first snapshot.
type HookScheduleSeed struct {
	Version        int               `json:"version"`
	Key            HookRunKey        `json:"run_key"`
	ExecutionScope string            `json:"execution_scope"`
	Mode           ExptRunMode       `json:"mode"`
	CreatedAt      int64             `json:"created_at"`
	ItemRetryTimes int               `json:"item_retry_times"`
	Session        *Session          `json:"session"`
	Ext            map[string]string `json:"ext"`
}

// Event returns a fresh original event; it neither refreshes policy nor grants admission.
func (s *HookScheduleSeed) Event(key HookRunKey, scope string, mode ExptRunMode, creator string) (*ExptScheduleEvent, error) {
	if s == nil || s.Version != 1 || !validHookRunKey(key) || !hookStorageASCII(scope, 128) ||
		s.Key != key || s.ExecutionScope != scope || s.Mode != mode ||
		(mode != EvaluationModeSubmit && mode != EvaluationModeTrialRun && mode != EvaluationModeFailRetry && mode != EvaluationModeRetryAll && mode != EvaluationModeRetryItems && mode != EvaluationModeAppend) || s.CreatedAt <= 0 || s.CreatedAt > 253402300799 ||
		s.ItemRetryTimes < 0 || s.ItemRetryTimes > math.MaxInt32 || s.Session == nil || s.Session.UserID != creator ||
		strings.TrimSpace(creator) == "" || len(creator) > 128 || !utf8.ValidString(creator) || s.Session.AppID < 0 ||
		(s.Ext[RetryYieldExtKey] != "true" && s.Ext[RetryYieldExtKey] != "false") {
		return nil, ErrHookStoreCorrupt
	}
	for key, value := range s.Ext {
		if !utf8.ValidString(key) || !utf8.ValidString(value) {
			return nil, ErrHookStoreCorrupt
		}
	}
	session := *s.Session
	typ := ExptType_Offline
	if mode == EvaluationModeAppend {
		typ = ExptType_Online
	}
	return &ExptScheduleEvent{SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID,
		ExptRunMode: mode, ExptType: typ, CreatedAt: s.CreatedAt, ItemRetryTimes: s.ItemRetryTimes,
		Session: &session, Ext: maps.Clone(s.Ext)}, nil
}

func cloneHookScheduleSeed(s *HookScheduleSeed) *HookScheduleSeed {
	if s == nil {
		return nil
	}
	out := *s
	out.Ext = maps.Clone(s.Ext)
	if s.Session != nil {
		session := *s.Session
		out.Session = &session
	}
	return &out
}
