// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
)

func TestHookProjectionCompletionDisplayCannotOverrideOutcome(t *testing.T) {
	key := HookRunKey{1, 2, 3}
	valid := HookCompleteAttemptInput{
		HookRenewAttemptInput: HookRenewAttemptInput{HookAttemptScope: HookAttemptScope{Key: key, OperationID: "op", Phase: HookPhaseBefore, ExecutionScope: "local", SnapshotHash: strings.Repeat("a", 64)}, HookAttemptIdentity: HookAttemptIdentity{Token: HookClaimToken{Run: key, OperationID: "op", Phase: HookPhaseBefore, Attempt: 1, Generation: 1}, Owner: "worker", DeliveryID: "delivery"}},
		Config:                &HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}, Outcome: HookOutcome{Code: HookFailed, HTTPStatus: 200, Retryable: true}, CompletedAt: time.Now(), DisplayErrorCode: "错误😀", ErrorMessage: "业务消息",
	}
	require.NoError(t, valid.Validate())
	for name, change := range map[string]func(*HookCompleteAttemptInput){
		"non200":            func(in *HookCompleteAttemptInput) { in.Outcome.HTTPStatus = 403 },
		"success":           func(in *HookCompleteAttemptInput) { in.Outcome.Code = HookSucceeded },
		"platform category": func(in *HookCompleteAttemptInput) { in.Outcome.Code = HookSecurityError },
		"mixed result":      func(in *HookCompleteAttemptInput) { in.ResultRedacted = []byte(`{}`) },
		"long code":         func(in *HookCompleteAttemptInput) { in.DisplayErrorCode = strings.Repeat("😀", 33) },
		"blank code":        func(in *HookCompleteAttemptInput) { in.DisplayErrorCode = " " },
		"invalid code":      func(in *HookCompleteAttemptInput) { in.DisplayErrorCode = string([]byte{255}) },
		"blank message":     func(in *HookCompleteAttemptInput) { in.ErrorMessage = " " },
		"message bytes":     func(in *HookCompleteAttemptInput) { in.ErrorMessage = strings.Repeat("中", 683) },
	} {
		t.Run(name, func(t *testing.T) { in := valid; change(&in); require.Error(t, in.Validate()) })
	}
}
