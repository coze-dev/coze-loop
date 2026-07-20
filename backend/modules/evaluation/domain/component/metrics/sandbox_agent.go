// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import "time"

// InvokeTags 一次沙箱 agent 评测对象执行涉及的可枚举与非枚举 tag 集合。
// 缺失字段由实现层统一填充为 "-"。
type SandboxAgentInvokeTags struct {
	ExperimentID    int64
	ItemID          int64
	InvokeID        string
	DatasetID       int64
	DatasetVersion  int64
}

// SandboxAgentExperimentTags 沙箱 agent 评测实验级 tag 集合。
type SandboxAgentExperimentTags struct {
	ExperimentID   int64
	DatasetID      int64
	DatasetVersion int64
}

//go:generate mockgen -destination=mocks/sandbox_agent.go -package=mocks . SandboxAgentMetrics
type SandboxAgentMetrics interface {
	// EmitInvokeStarted 一次 target invocation 提交时打点，仅 counter。
	EmitInvokeStarted(tags SandboxAgentInvokeTags)
	// EmitInvokeFinished 回调回写时打点，counter + duration。
	// submitTime 为提交时间戳（AsyncCtx.AsyncUnixMS 语义），用于计算 duration。
	EmitInvokeFinished(tags SandboxAgentInvokeTags, err error, errCode int32, submitTime time.Time)
	// EmitExperimentStarted 实验进入 Processing 状态时打点，仅 counter。
	EmitExperimentStarted(tags SandboxAgentExperimentTags)
	// EmitExperimentFinished 实验终态时打点，counter + duration。
	// startTime / endTime 分别对应 experiment.StartAt / experiment.EndAt。
	EmitExperimentFinished(tags SandboxAgentExperimentTags, err error, startTime, endTime time.Time)
}
