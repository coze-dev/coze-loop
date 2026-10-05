// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "errors"

var ErrHookFinalizationUnsettled = errors.New("hook normal finalization requires settled original-run records")
var ErrHookFinalizationUnsupported = errors.New("hook finalization mode is not supported")

type HookFinalizationCounts struct {
	Pending, Processing, Success, Fail, Terminated int32
}
type HookFinalizationStats struct {
	Key               HookRunKey
	ExecutionScope    string
	Items, Turns      HookFinalizationCounts
	ItemIDs           []int64
	NeverAdmitted     bool
	ActiveTermination bool
}

func (s HookFinalizationStats) NormalStatus() ExptStatus {
	if s.Items.Fail > 0 || s.Turns.Fail > 0 {
		return ExptStatus_Failed
	}
	return ExptStatus_Success
}

type HookFinalizationSource struct {
	Key        HookRunKey
	Managed    bool
	Experiment *Experiment
	RunLog     *ExptRunLog
}

// The authoritative completion may report a run-level failure after items succeed.
func (s HookFinalizationStats) AllowsTerminalStatus(status ExptStatus) bool {
	if s.NeverAdmitted || s.ActiveTermination {
		return status == ExptStatus_Terminated || status == ExptStatus_SystemTerminated
	}
	return status == s.NormalStatus() || status == ExptStatus_Failed
}
