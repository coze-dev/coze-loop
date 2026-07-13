// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import "time"

// SandboxAgentTags 打点通用维度。所有字段为空时使用默认占位（"-" / "0"）。
type SandboxAgentTags struct {
	SpaceID          int64
	ExperimentID     int64
	ExperimentRunID  int64
	ItemID           int64
	InvokeID         int64
	DatasetID        int64
	DatasetVersionID int64
	StepName         string
	Success          bool
	// ErrorType 由调用侧决定；成功时应传空串，失败时传如 "engineering" / "non_engineering" / "unknown"。
	ErrorType string
}

//go:generate mockgen -destination=mocks/sandbox_agent.go -package=mocks . SandboxAgentMetrics
type SandboxAgentMetrics interface {
	EmitStepStarted(tags SandboxAgentTags)
	EmitStepFinished(tags SandboxAgentTags, start time.Time)

	EmitInvokeStarted(tags SandboxAgentTags)
	EmitInvokeFinished(tags SandboxAgentTags, start time.Time)

	EmitExperimentStarted(tags SandboxAgentTags)
	EmitExperimentFinished(tags SandboxAgentTags, start time.Time)
}
