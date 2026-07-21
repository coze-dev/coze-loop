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

// SandboxAgentStepTags 沙箱内部 step 事件的 tag 集合。
// step_name 是可枚举维度，其它 tag 与 invoke 复用同一命名。
type SandboxAgentStepTags struct {
	ExperimentID   int64
	ItemID         int64
	InvokeID       string
	DatasetID      int64
	DatasetVersion int64
	StepName       string
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
	// EmitStepStarted 沙箱内部 step 开始事件，仅 counter，来源：openapi ReportEvalTargetStepMetric。
	EmitStepStarted(tags SandboxAgentStepTags)
	// EmitStepFinished 沙箱内部 step 结束事件，counter + duration，来源同上。
	// durationMS 由沙箱侧上报（step 内计时更准确）；err/errCode 用于分类。
	EmitStepFinished(tags SandboxAgentStepTags, err error, errCode int32, durationMS int64)
}
