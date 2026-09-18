// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// EvalTargetExecutionContext binds an invocation to the initiator of its persisted run.
type EvalTargetExecutionContext struct {
	WorkspaceID           int64
	EvalTargetRecordID    int64
	ExperimentID          int64
	ExperimentRunID       int64
	ExperimentWorkspaceID int64
	InitiatorUserID       string
}
