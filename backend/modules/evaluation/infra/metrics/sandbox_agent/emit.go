// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"strconv"
	"sync"
	"time"

	imetrics "github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
)

const (
	sandboxAgentMtrName = "evaluation_target_sandbox_agent"

	stepSuffix       = ".step"
	invokeSuffix     = ".invoke"
	experimentSuffix = ".experiment"

	startedSuffix  = "_started"
	finishedSuffix = "_finished"
	durationSuffix = "_duration"
)

const (
	tagSpaceID          = "space_id"
	tagExperimentID     = "experiment_id"
	tagExperimentRunID  = "experiment_run_id"
	tagItemID           = "item_id"
	tagInvokeID         = "invoke_id"
	tagDatasetID        = "dataset_id"
	tagDatasetVersionID = "dataset_version"
	tagStepName         = "step_name"
	tagSuccess          = "success"
	tagErrorType        = "error_type"
)

// tagPlaceholder metric 库要求所有声明的 tag 都有值；缺失时统一用 "-" 占位，避免打点被丢弃。
const tagPlaceholder = "-"

func sandboxAgentMtrTags() []string {
	return []string{
		tagSpaceID,
		tagExperimentID,
		tagExperimentRunID,
		tagItemID,
		tagInvokeID,
		tagDatasetID,
		tagDatasetVersionID,
		tagStepName,
		tagSuccess,
		tagErrorType,
	}
}

var (
	sandboxAgentMetricsOnce = sync.Once{}
	sandboxAgentMetricsImpl metrics.SandboxAgentMetrics
)

func NewSandboxAgentMetrics(meter imetrics.Meter) metrics.SandboxAgentMetrics {
	sandboxAgentMetricsOnce.Do(func() {
		if meter == nil {
			return
		}
		metric, err := meter.NewMetric(sandboxAgentMtrName,
			[]imetrics.MetricType{imetrics.MetricTypeCounter, imetrics.MetricTypeTimer},
			sandboxAgentMtrTags())
		if err != nil {
			return
		}
		sandboxAgentMetricsImpl = &SandboxAgentMetricsImpl{metric: metric}
	})
	return sandboxAgentMetricsImpl
}

type SandboxAgentMetricsImpl struct {
	metric imetrics.Metric
}

func (e *SandboxAgentMetricsImpl) safe() bool {
	return e != nil && e.metric != nil
}

func buildTags(t metrics.SandboxAgentTags, withStep, withOutcome bool) []imetrics.T {
	stepName := tagPlaceholder
	if withStep && t.StepName != "" {
		stepName = t.StepName
	}
	success := tagPlaceholder
	errorType := tagPlaceholder
	if withOutcome {
		success = strconv.FormatBool(t.Success)
		if t.ErrorType != "" {
			errorType = t.ErrorType
		} else if t.Success {
			// 成功时 error_type 用占位符 "-" 与方案对齐
			errorType = tagPlaceholder
		} else {
			errorType = "unknown"
		}
	}
	return []imetrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(t.SpaceID, 10)},
		{Name: tagExperimentID, Value: formatInt64OrDash(t.ExperimentID)},
		{Name: tagExperimentRunID, Value: formatInt64OrDash(t.ExperimentRunID)},
		{Name: tagItemID, Value: formatInt64OrDash(t.ItemID)},
		{Name: tagInvokeID, Value: formatInt64OrDash(t.InvokeID)},
		{Name: tagDatasetID, Value: formatInt64OrDash(t.DatasetID)},
		{Name: tagDatasetVersionID, Value: formatInt64OrDash(t.DatasetVersionID)},
		{Name: tagStepName, Value: stepName},
		{Name: tagSuccess, Value: success},
		{Name: tagErrorType, Value: errorType},
	}
}

func formatInt64OrDash(v int64) string {
	if v == 0 {
		return tagPlaceholder
	}
	return strconv.FormatInt(v, 10)
}

func (e *SandboxAgentMetricsImpl) EmitStepStarted(t metrics.SandboxAgentTags) {
	if !e.safe() {
		return
	}
	e.metric.Emit(buildTags(t, true, false),
		imetrics.Counter(1, imetrics.WithSuffix(stepSuffix+startedSuffix)))
}

func (e *SandboxAgentMetricsImpl) EmitStepFinished(t metrics.SandboxAgentTags, start time.Time) {
	if !e.safe() {
		return
	}
	tags := buildTags(t, true, true)
	e.metric.Emit(tags,
		imetrics.Counter(1, imetrics.WithSuffix(stepSuffix+finishedSuffix)),
		imetrics.Timer(time.Since(start).Milliseconds(), imetrics.WithSuffix(stepSuffix+durationSuffix)),
	)
}

func (e *SandboxAgentMetricsImpl) EmitInvokeStarted(t metrics.SandboxAgentTags) {
	if !e.safe() {
		return
	}
	e.metric.Emit(buildTags(t, false, false),
		imetrics.Counter(1, imetrics.WithSuffix(invokeSuffix+startedSuffix)))
}

func (e *SandboxAgentMetricsImpl) EmitInvokeFinished(t metrics.SandboxAgentTags, start time.Time) {
	if !e.safe() {
		return
	}
	tags := buildTags(t, false, true)
	e.metric.Emit(tags,
		imetrics.Counter(1, imetrics.WithSuffix(invokeSuffix+finishedSuffix)),
		imetrics.Timer(time.Since(start).Milliseconds(), imetrics.WithSuffix(invokeSuffix+durationSuffix)),
	)
}

func (e *SandboxAgentMetricsImpl) EmitExperimentStarted(t metrics.SandboxAgentTags) {
	if !e.safe() {
		return
	}
	e.metric.Emit(buildTags(t, false, false),
		imetrics.Counter(1, imetrics.WithSuffix(experimentSuffix+startedSuffix)))
}

func (e *SandboxAgentMetricsImpl) EmitExperimentFinished(t metrics.SandboxAgentTags, start time.Time) {
	if !e.safe() {
		return
	}
	tags := buildTags(t, false, true)
	e.metric.Emit(tags,
		imetrics.Counter(1, imetrics.WithSuffix(experimentSuffix+finishedSuffix)),
		imetrics.Timer(time.Since(start).Milliseconds(), imetrics.WithSuffix(experimentSuffix+durationSuffix)),
	)
}
