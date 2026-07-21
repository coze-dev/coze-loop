// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package sandbox_agent 沙箱 agent 评测对象的稳定性指标上报。
//
// 指标名: evaluation_target_sandbox_agent
// 类型: [Counter, Timer]，通过 suffix 复用
// 参考: modules/evaluation/Trae评测迁移fornax稳定性技术方案.docx
package sandbox_agent

import (
	"strconv"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
)

const (
	metricName = "evaluation_target_sandbox_agent"

	// invoke suffixes — 单次 target invocation 的生命周期
	suffixInvokeStarted  = "invoke_started"
	suffixInvokeFinished = "invoke_finished"
	suffixInvokeDuration = "invoke_duration"

	// experiment suffixes — 沙箱 agent 评测实验的生命周期
	suffixExperimentStarted  = "experiment_started"
	suffixExperimentFinished = "experiment_finished"
	suffixExperimentDuration = "experiment_duration"

	// step suffixes — 沙箱内部编排流程 step，来源 openapi ReportEvalTargetStepMetric
	suffixStepStarted  = "step_started"
	suffixStepFinished = "step_finished"
	suffixStepDuration = "step_duration"

	tagExperimentID    = "experiment_id"
	tagItemID          = "item_id"
	tagInvokeID        = "invoke_id"
	tagDatasetID       = "dataset_id"
	tagDatasetVersion  = "dataset_version"
	tagStepName        = "step_name"
	tagSuccess         = "success"
	tagErrorType       = "error_type"

	// tag 空值占位，遵循 fornax 平台约定
	tagValuePlaceholder = "-"
)

func metricTagNames() []string {
	return []string{
		tagExperimentID,
		tagItemID,
		tagInvokeID,
		tagDatasetID,
		tagDatasetVersion,
		tagStepName,
		tagSuccess,
		tagErrorType,
	}
}

var (
	once sync.Once
	impl eval_metrics.SandboxAgentMetrics
)

// NewSandboxAgentMetrics 构造一个进程级单例指标上报器。meter 为 nil 时返回 no-op 实现。
func NewSandboxAgentMetrics(meter metrics.Meter) eval_metrics.SandboxAgentMetrics {
	once.Do(func() {
		if meter == nil {
			impl = &noopMetrics{}
			return
		}
		m, err := meter.NewMetric(metricName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, metricTagNames())
		if err != nil || m == nil {
			impl = &noopMetrics{}
			return
		}
		impl = &metricsImpl{metric: m}
	})
	return impl
}

type metricsImpl struct {
	metric metrics.Metric
}

func (m *metricsImpl) EmitInvokeStarted(tags eval_metrics.SandboxAgentInvokeTags) {
	if m == nil || m.metric == nil {
		return
	}
	m.metric.Emit(m.buildInvokeTags(tags, "", ""),
		metrics.Counter(1, metrics.WithSuffix(suffixInvokeStarted)))
}

func (m *metricsImpl) EmitInvokeFinished(tags eval_metrics.SandboxAgentInvokeTags, err error, errCode int32, submitTime time.Time) {
	if m == nil || m.metric == nil {
		return
	}
	success := successTag(err, errCode)
	errType := ClassifyErrorType(err, errCode)
	durationMS := durationMS(submitTime)
	m.metric.Emit(m.buildInvokeTags(tags, success, errType),
		metrics.Counter(1, metrics.WithSuffix(suffixInvokeFinished)),
		metrics.Timer(durationMS, metrics.WithSuffix(suffixInvokeDuration)))
}

func (m *metricsImpl) EmitExperimentStarted(tags eval_metrics.SandboxAgentExperimentTags) {
	if m == nil || m.metric == nil {
		return
	}
	m.metric.Emit(m.buildExperimentTags(tags, "", ""),
		metrics.Counter(1, metrics.WithSuffix(suffixExperimentStarted)))
}

func (m *metricsImpl) EmitExperimentFinished(tags eval_metrics.SandboxAgentExperimentTags, err error, startTime, endTime time.Time) {
	if m == nil || m.metric == nil {
		return
	}
	success := successTag(err, 0)
	errType := ClassifyErrorType(err, 0)
	var durMS int64
	if !startTime.IsZero() && !endTime.IsZero() && !endTime.Before(startTime) {
		durMS = endTime.Sub(startTime).Milliseconds()
	}
	m.metric.Emit(m.buildExperimentTags(tags, success, errType),
		metrics.Counter(1, metrics.WithSuffix(suffixExperimentFinished)),
		metrics.Timer(durMS, metrics.WithSuffix(suffixExperimentDuration)))
}

func (m *metricsImpl) EmitStepStarted(tags eval_metrics.SandboxAgentStepTags) {
	if m == nil || m.metric == nil {
		return
	}
	m.metric.Emit(m.buildStepTags(tags, "", ""),
		metrics.Counter(1, metrics.WithSuffix(suffixStepStarted)))
}

func (m *metricsImpl) EmitStepFinished(tags eval_metrics.SandboxAgentStepTags, err error, errCode int32, durationMS int64) {
	if m == nil || m.metric == nil {
		return
	}
	success := successTag(err, errCode)
	errType := ClassifyErrorType(err, errCode)
	if durationMS < 0 {
		durationMS = 0
	}
	m.metric.Emit(m.buildStepTags(tags, success, errType),
		metrics.Counter(1, metrics.WithSuffix(suffixStepFinished)),
		metrics.Timer(durationMS, metrics.WithSuffix(suffixStepDuration)))
}

func (m *metricsImpl) buildInvokeTags(t eval_metrics.SandboxAgentInvokeTags, success, errType string) []metrics.T {
	return []metrics.T{
		{Name: tagExperimentID, Value: int64Tag(t.ExperimentID)},
		{Name: tagItemID, Value: int64Tag(t.ItemID)},
		{Name: tagInvokeID, Value: stringTag(t.InvokeID)},
		{Name: tagDatasetID, Value: int64Tag(t.DatasetID)},
		{Name: tagDatasetVersion, Value: int64Tag(t.DatasetVersion)},
		{Name: tagStepName, Value: tagValuePlaceholder},
		{Name: tagSuccess, Value: fallback(success)},
		{Name: tagErrorType, Value: fallback(errType)},
	}
}

func (m *metricsImpl) buildExperimentTags(t eval_metrics.SandboxAgentExperimentTags, success, errType string) []metrics.T {
	return []metrics.T{
		{Name: tagExperimentID, Value: int64Tag(t.ExperimentID)},
		{Name: tagItemID, Value: tagValuePlaceholder},
		{Name: tagInvokeID, Value: tagValuePlaceholder},
		{Name: tagDatasetID, Value: int64Tag(t.DatasetID)},
		{Name: tagDatasetVersion, Value: int64Tag(t.DatasetVersion)},
		{Name: tagStepName, Value: tagValuePlaceholder},
		{Name: tagSuccess, Value: fallback(success)},
		{Name: tagErrorType, Value: fallback(errType)},
	}
}

func (m *metricsImpl) buildStepTags(t eval_metrics.SandboxAgentStepTags, success, errType string) []metrics.T {
	return []metrics.T{
		{Name: tagExperimentID, Value: int64Tag(t.ExperimentID)},
		{Name: tagItemID, Value: int64Tag(t.ItemID)},
		{Name: tagInvokeID, Value: stringTag(t.InvokeID)},
		{Name: tagDatasetID, Value: int64Tag(t.DatasetID)},
		{Name: tagDatasetVersion, Value: int64Tag(t.DatasetVersion)},
		{Name: tagStepName, Value: stringTag(t.StepName)},
		{Name: tagSuccess, Value: fallback(success)},
		{Name: tagErrorType, Value: fallback(errType)},
	}
}

func int64Tag(v int64) string {
	if v == 0 {
		return tagValuePlaceholder
	}
	return strconv.FormatInt(v, 10)
}

func stringTag(v string) string {
	if v == "" {
		return tagValuePlaceholder
	}
	return v
}

func fallback(v string) string {
	if v == "" {
		return tagValuePlaceholder
	}
	return v
}

// successTag 严格按 docx 定义: true/false，非 *_finished/*_duration 场景传空串走占位。
func successTag(err error, errCode int32) string {
	if err == nil && errCode == 0 {
		return "true"
	}
	return "false"
}

// durationMS 计算提交到当前时间的毫秒差；submitTime 为零值时返回 0。
func durationMS(submitTime time.Time) int64 {
	if submitTime.IsZero() {
		return 0
	}
	elapsed := time.Since(submitTime)
	if elapsed < 0 {
		return 0
	}
	return elapsed.Milliseconds()
}

type noopMetrics struct{}

func (n *noopMetrics) EmitInvokeStarted(_ eval_metrics.SandboxAgentInvokeTags)                                     {}
func (n *noopMetrics) EmitInvokeFinished(_ eval_metrics.SandboxAgentInvokeTags, _ error, _ int32, _ time.Time)     {}
func (n *noopMetrics) EmitExperimentStarted(_ eval_metrics.SandboxAgentExperimentTags)                             {}
func (n *noopMetrics) EmitExperimentFinished(_ eval_metrics.SandboxAgentExperimentTags, _ error, _, _ time.Time)   {}
func (n *noopMetrics) EmitStepStarted(_ eval_metrics.SandboxAgentStepTags)                                         {}
func (n *noopMetrics) EmitStepFinished(_ eval_metrics.SandboxAgentStepTags, _ error, _ int32, _ int64)             {}
