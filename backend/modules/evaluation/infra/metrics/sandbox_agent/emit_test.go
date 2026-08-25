// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package sandbox_agent

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
)

// fakeMeter / fakeMetric 只做录制, 用来断言 emit 携带的 tag 与 suffix.
type fakeMeter struct {
	m *fakeMetric
}

func (f *fakeMeter) NewMetric(name string, types []metrics.MetricType, tagNames []string) (metrics.Metric, error) {
	f.m = &fakeMetric{name: name, tagNames: tagNames}
	return f.m, nil
}

type emittedRecord struct {
	tags   map[string]string
	values []emittedValue
}

type emittedValue struct {
	mType  metrics.MetricType
	suffix string
	v      int64
}

type fakeMetric struct {
	mu       sync.Mutex
	name     string
	tagNames []string
	records  []emittedRecord
}

func (m *fakeMetric) Emit(tags []metrics.T, values ...*metrics.Value) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tagMap := make(map[string]string, len(tags))
	for _, t := range tags {
		tagMap[t.Name] = t.Value
	}
	rec := emittedRecord{tags: tagMap}
	for _, v := range values {
		val := int64(0)
		if p := v.GetValue(); p != nil {
			val = *p
		}
		rec.values = append(rec.values, emittedValue{
			mType:  v.GetType(),
			suffix: v.GetSuffix(),
			v:      val,
		})
	}
	m.records = append(m.records, rec)
}

// newFakeImpl 每次都返回一个独立实例，避免依赖 NewSandboxAgentMetrics 的 sync.Once。
func newFakeImpl(t *testing.T) (*metricsImpl, *fakeMetric) {
	t.Helper()
	fm := &fakeMeter{}
	m, err := fm.NewMetric(metricName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, metricTagNames())
	if err != nil {
		t.Fatalf("NewMetric err: %v", err)
	}
	return &metricsImpl{metric: m}, fm.m
}

func TestEmitInvokeStarted(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitInvokeStarted(eval_metrics.SandboxAgentInvokeTags{
		SpaceID:         7,
		ExperimentID:    100,
		ExperimentRunID: 101,
		ItemID:          200,
		InvokeID:        "300",
		DatasetID:       400,
		DatasetVersion:  500,
		AgentName:       "my-agent",
		ApplicationID:   "app-1",
	})
	if len(fm.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(fm.records))
	}
	rec := fm.records[0]
	if rec.tags["experiment_id"] != "100" || rec.tags["item_id"] != "200" || rec.tags["invoke_id"] != "300" {
		t.Fatalf("id tags wrong: %+v", rec.tags)
	}
	if rec.tags["dataset_id"] != "400" || rec.tags["dataset_version"] != "500" {
		t.Fatalf("dataset tags wrong: %+v", rec.tags)
	}
	if rec.tags["space_id"] != "7" || rec.tags["experiment_run_id"] != "101" {
		t.Fatalf("space_id/experiment_run_id tags wrong: %+v", rec.tags)
	}
	if rec.tags["agent_name"] != "my-agent" || rec.tags["application_id"] != "app-1" {
		t.Fatalf("agent_name/application_id tags wrong: %+v", rec.tags)
	}
	if rec.tags["success"] != "-" || rec.tags["error_type"] != "-" {
		t.Fatalf("success/error_type should be placeholder on started, got: %+v", rec.tags)
	}
	if len(rec.values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(rec.values))
	}
	if rec.values[0].suffix != "invoke_started" || rec.values[0].mType != metrics.MetricTypeCounter || rec.values[0].v != 1 {
		t.Fatalf("value wrong: %+v", rec.values[0])
	}
}

func TestEmitInvokeFinished_Success(t *testing.T) {
	impl, fm := newFakeImpl(t)
	submitTime := time.Now().Add(-1500 * time.Millisecond)
	impl.EmitInvokeFinished(eval_metrics.SandboxAgentInvokeTags{
		SpaceID:         9,
		ExperimentID:    1,
		ExperimentRunID: 2,
		InvokeID:        "x",
		AgentName:       "agent-a",
		ApplicationID:   "app-a",
	}, nil, 0, submitTime)
	if len(fm.records) != 1 {
		t.Fatalf("want 1 record, got %d", len(fm.records))
	}
	rec := fm.records[0]
	if rec.tags["success"] != "true" || rec.tags["error_type"] != "-" {
		t.Fatalf("success/error_type wrong: %+v", rec.tags)
	}
	if rec.tags["space_id"] != "9" || rec.tags["experiment_run_id"] != "2" {
		t.Fatalf("space_id/experiment_run_id tags wrong: %+v", rec.tags)
	}
	if rec.tags["agent_name"] != "agent-a" || rec.tags["application_id"] != "app-a" {
		t.Fatalf("agent_name/application_id tags wrong: %+v", rec.tags)
	}
	if len(rec.values) != 2 {
		t.Fatalf("want counter+timer, got %d", len(rec.values))
	}
	var counterSeen, timerSeen bool
	for _, v := range rec.values {
		switch v.suffix {
		case "invoke_finished":
			if v.mType != metrics.MetricTypeCounter || v.v != 1 {
				t.Fatalf("finished counter wrong: %+v", v)
			}
			counterSeen = true
		case "invoke_duration":
			if v.mType != metrics.MetricTypeTimer || v.v < 1000 {
				t.Fatalf("duration timer too small: %+v", v)
			}
			timerSeen = true
		}
	}
	if !counterSeen || !timerSeen {
		t.Fatalf("expected both finished and duration, got %+v", rec.values)
	}
}

func TestEmitInvokeFinished_Engineering(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitInvokeFinished(eval_metrics.SandboxAgentInvokeTags{}, errors.New("boom"), 601200701, time.Time{})
	if len(fm.records) != 1 {
		t.Fatalf("want 1 record")
	}
	rec := fm.records[0]
	if rec.tags["success"] != "false" || rec.tags["error_type"] != "engineering" {
		t.Fatalf("unexpected tags: %+v", rec.tags)
	}
}

func TestEmitInvokeFinished_NonEngineering(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitInvokeFinished(eval_metrics.SandboxAgentInvokeTags{}, nil, 601299999, time.Time{})
	rec := fm.records[0]
	if rec.tags["error_type"] != "non_engineering" {
		t.Fatalf("want non_engineering, got %s", rec.tags["error_type"])
	}
}

func TestEmitExperimentStartedFinished(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitExperimentStarted(eval_metrics.SandboxAgentExperimentTags{ExperimentID: 42, DatasetID: 7, DatasetVersion: 8})
	if len(fm.records) != 1 {
		t.Fatalf("want 1 record")
	}
	start := fm.records[0]
	if start.tags["experiment_id"] != "42" || start.tags["dataset_id"] != "7" || start.tags["dataset_version"] != "8" {
		t.Fatalf("started tags wrong: %+v", start.tags)
	}
	if start.tags["item_id"] != "-" || start.tags["invoke_id"] != "-" {
		t.Fatalf("started should placeholder item/invoke, got %+v", start.tags)
	}
	// 实验层不携带 invoke-only 的四个 tag, 全部走占位符。
	for _, k := range []string{"space_id", "experiment_run_id", "agent_name", "application_id"} {
		if start.tags[k] != "-" {
			t.Fatalf("experiment tag %s should be placeholder, got %q", k, start.tags[k])
		}
	}
	if start.values[0].suffix != "experiment_started" {
		t.Fatalf("want experiment_started, got %s", start.values[0].suffix)
	}

	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	impl.EmitExperimentFinished(eval_metrics.SandboxAgentExperimentTags{ExperimentID: 42}, nil, base, base.Add(2*time.Second))
	if len(fm.records) != 2 {
		t.Fatalf("want 2 records after finished")
	}
	fin := fm.records[1]
	if fin.tags["success"] != "true" {
		t.Fatalf("success expected true")
	}
	var counterSeen, timerSeen bool
	for _, v := range fin.values {
		switch v.suffix {
		case "experiment_finished":
			counterSeen = v.mType == metrics.MetricTypeCounter && v.v == 1
		case "experiment_duration":
			timerSeen = v.mType == metrics.MetricTypeTimer && v.v == 2000
		}
	}
	if !counterSeen || !timerSeen {
		t.Fatalf("expected finished+duration, got %+v", fin.values)
	}
}

func TestNoopWhenMeterNil(t *testing.T) {
	// impl.metric == nil 场景不 panic
	empty := &metricsImpl{}
	empty.EmitInvokeStarted(eval_metrics.SandboxAgentInvokeTags{})
	empty.EmitInvokeFinished(eval_metrics.SandboxAgentInvokeTags{}, nil, 0, time.Now())
	empty.EmitExperimentStarted(eval_metrics.SandboxAgentExperimentTags{})
	empty.EmitExperimentFinished(eval_metrics.SandboxAgentExperimentTags{}, nil, time.Now(), time.Now())
	empty.EmitStepStarted(eval_metrics.SandboxAgentStepTags{})
	empty.EmitStepFinished(eval_metrics.SandboxAgentStepTags{}, nil, 0, 0)
	empty.EmitE2EStarted(eval_metrics.SandboxAgentE2ETags{})
	empty.EmitE2EFinished(eval_metrics.SandboxAgentE2ETags{}, nil, time.Now())

	// nil receiver 也不 panic
	var nilImpl *metricsImpl
	nilImpl.EmitE2EStarted(eval_metrics.SandboxAgentE2ETags{})
	nilImpl.EmitE2EFinished(eval_metrics.SandboxAgentE2ETags{}, nil, time.Now())

	// noop 实现完整覆盖(不 panic 即可)
	n := &noopMetrics{}
	n.EmitInvokeStarted(eval_metrics.SandboxAgentInvokeTags{})
	n.EmitInvokeFinished(eval_metrics.SandboxAgentInvokeTags{}, nil, 0, time.Time{})
	n.EmitExperimentStarted(eval_metrics.SandboxAgentExperimentTags{})
	n.EmitExperimentFinished(eval_metrics.SandboxAgentExperimentTags{}, nil, time.Time{}, time.Time{})
	n.EmitStepStarted(eval_metrics.SandboxAgentStepTags{})
	n.EmitStepFinished(eval_metrics.SandboxAgentStepTags{}, nil, 0, 0)
	n.EmitE2EStarted(eval_metrics.SandboxAgentE2ETags{})
	n.EmitE2EFinished(eval_metrics.SandboxAgentE2ETags{}, nil, time.Time{})
}

func TestEmitStepStarted(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitStepStarted(eval_metrics.SandboxAgentStepTags{
		ExperimentID: 1, ItemID: 2, InvokeID: "3", DatasetID: 4, DatasetVersion: 5, StepName: "plan",
		TargetID: 6, ItemKey: "item-key", DatasetKey: "ds-key",
	})
	if len(fm.records) != 1 {
		t.Fatalf("want 1 record, got %d", len(fm.records))
	}
	rec := fm.records[0]
	if rec.tags["step_name"] != "plan" || rec.tags["item_id"] != "2" || rec.tags["invoke_id"] != "3" {
		t.Fatalf("step tags wrong: %+v", rec.tags)
	}
	if rec.tags["target_id"] != "6" || rec.tags["item_key"] != "item-key" || rec.tags["dataset_key"] != "ds-key" {
		t.Fatalf("new step tags wrong: %+v", rec.tags)
	}
	// Step 层同样不携带 invoke-only 的四个 tag。
	for _, k := range []string{"space_id", "experiment_run_id", "agent_name", "application_id"} {
		if rec.tags[k] != "-" {
			t.Fatalf("step tag %s should be placeholder, got %q", k, rec.tags[k])
		}
	}
	if rec.tags["success"] != "-" || rec.tags["error_type"] != "-" {
		t.Fatalf("started should not carry success/error_type, got %+v", rec.tags)
	}
	if rec.values[0].suffix != "step_started" || rec.values[0].mType != metrics.MetricTypeCounter {
		t.Fatalf("value wrong: %+v", rec.values[0])
	}
}

func TestEmitStepFinished_Success(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitStepFinished(eval_metrics.SandboxAgentStepTags{StepName: "act"}, nil, 0, 750)
	if len(fm.records) != 1 {
		t.Fatalf("want 1 record")
	}
	rec := fm.records[0]
	if rec.tags["success"] != "true" || rec.tags["error_type"] != "-" {
		t.Fatalf("finished success tags wrong: %+v", rec.tags)
	}
	var counterSeen, timerSeen bool
	for _, v := range rec.values {
		switch v.suffix {
		case "step_finished":
			counterSeen = v.mType == metrics.MetricTypeCounter && v.v == 1
		case "step_duration":
			timerSeen = v.mType == metrics.MetricTypeTimer && v.v == 750
		}
	}
	if !counterSeen || !timerSeen {
		t.Fatalf("expected counter+timer, got %+v", rec.values)
	}
}

func TestEmitStepFinished_EngineeringFailure(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitStepFinished(
		eval_metrics.SandboxAgentStepTags{StepName: "call_model"},
		&noopMetricsErr{},
		601200701,
		1000,
	)
	rec := fm.records[0]
	if rec.tags["success"] != "false" || rec.tags["error_type"] != "engineering" {
		t.Fatalf("tags wrong: %+v", rec.tags)
	}
}

func TestEmitStepFinished_NegativeDurationClamped(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitStepFinished(eval_metrics.SandboxAgentStepTags{}, nil, 0, -1)
	for _, v := range fm.records[0].values {
		if v.suffix == "step_duration" && v.v != 0 {
			t.Fatalf("negative duration should clamp to 0, got %d", v.v)
		}
	}
}

// noopMetricsErr 用于 step finished 失败路径测试.
type noopMetricsErr struct{}

func (n *noopMetricsErr) Error() string { return "boom" }

// ============================
// e2e (turn 粒度端到端) 打点测试
// ============================

func TestEmitE2EStarted(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitE2EStarted(eval_metrics.SandboxAgentE2ETags{
		SpaceID:         7,
		ExperimentID:    100,
		ExperimentRunID: 101,
		ItemID:          200,
		TurnID:          201,
		DatasetID:       400,
		DatasetVersion:  500,
		TargetID:        600,
		ItemKey:         "item-key",
		DatasetKey:      "ds-key",
		AgentName:       "my-agent",
		ApplicationID:   "app-1",
	})
	if len(fm.records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(fm.records))
	}
	rec := fm.records[0]
	// 关键 tag 齐全: space_id / experiment_id / experiment_run_id / item_id / turn_id
	// dataset_* / target_id / agent_name / application_id / item_key / dataset_key
	if rec.tags["space_id"] != "7" || rec.tags["experiment_id"] != "100" || rec.tags["experiment_run_id"] != "101" {
		t.Fatalf("id tags wrong: %+v", rec.tags)
	}
	if rec.tags["item_id"] != "200" || rec.tags["turn_id"] != "201" {
		t.Fatalf("item/turn tags wrong: %+v", rec.tags)
	}
	if rec.tags["dataset_id"] != "400" || rec.tags["dataset_version"] != "500" || rec.tags["target_id"] != "600" {
		t.Fatalf("dataset/target tags wrong: %+v", rec.tags)
	}
	if rec.tags["agent_name"] != "my-agent" || rec.tags["application_id"] != "app-1" {
		t.Fatalf("agent/app tags wrong: %+v", rec.tags)
	}
	if rec.tags["item_key"] != "item-key" || rec.tags["dataset_key"] != "ds-key" {
		t.Fatalf("item_key/dataset_key tags wrong: %+v", rec.tags)
	}
	// e2e 层不携带 invoke/step 特有的 tag
	if rec.tags["invoke_id"] != "-" || rec.tags["step_name"] != "-" {
		t.Fatalf("invoke_id/step_name should be placeholder: %+v", rec.tags)
	}
	// started: 不携带 success / error_type
	if rec.tags["success"] != "-" || rec.tags["error_type"] != "-" {
		t.Fatalf("started should carry placeholder success/error_type, got: %+v", rec.tags)
	}
	if rec.tags["error_code"] != "-" {
		t.Fatalf("error_code should be placeholder, got %q", rec.tags["error_code"])
	}
	if len(rec.values) != 1 {
		t.Fatalf("expected 1 value on started, got %d", len(rec.values))
	}
	v := rec.values[0]
	if v.suffix != "e2e_started" || v.mType != metrics.MetricTypeCounter || v.v != 1 {
		t.Fatalf("started value wrong: %+v", v)
	}
}

// 空 tag 场景: 数字字段走占位符 `-`, 字符串走 sanitize -> `-`
func TestEmitE2EStarted_EmptyTagsPlaceholder(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitE2EStarted(eval_metrics.SandboxAgentE2ETags{})
	rec := fm.records[0]
	for _, k := range []string{"space_id", "experiment_id", "experiment_run_id", "item_id", "turn_id", "dataset_id", "dataset_version", "target_id", "agent_name", "application_id", "item_key", "dataset_key"} {
		if rec.tags[k] != "-" {
			t.Fatalf("expected placeholder for %s, got %q", k, rec.tags[k])
		}
	}
}

func TestEmitE2EFinished_Success(t *testing.T) {
	impl, fm := newFakeImpl(t)
	start := time.Now().Add(-1500 * time.Millisecond)
	impl.EmitE2EFinished(eval_metrics.SandboxAgentE2ETags{
		SpaceID:         9,
		ExperimentID:    1,
		ExperimentRunID: 2,
		ItemID:          10,
		TurnID:          20,
		AgentName:       "agent-a",
		ApplicationID:   "app-a",
	}, nil, start)
	if len(fm.records) != 1 {
		t.Fatalf("want 1 record, got %d", len(fm.records))
	}
	rec := fm.records[0]
	if rec.tags["success"] != "true" || rec.tags["error_type"] != "-" {
		t.Fatalf("success/error_type wrong: %+v", rec.tags)
	}
	if rec.tags["turn_id"] != "20" || rec.tags["item_id"] != "10" {
		t.Fatalf("item/turn wrong: %+v", rec.tags)
	}
	if len(rec.values) != 2 {
		t.Fatalf("want counter+timer, got %d", len(rec.values))
	}
	var counterSeen, timerSeen bool
	for _, v := range rec.values {
		switch v.suffix {
		case "e2e_finished":
			if v.mType != metrics.MetricTypeCounter || v.v != 1 {
				t.Fatalf("finished counter wrong: %+v", v)
			}
			counterSeen = true
		case "e2e_duration":
			if v.mType != metrics.MetricTypeTimer || v.v < 1000 {
				t.Fatalf("duration too small: %+v", v)
			}
			timerSeen = true
		}
	}
	if !counterSeen || !timerSeen {
		t.Fatalf("expected both finished+duration, got %+v", rec.values)
	}
}

// startTime 为零值: duration 应记 0
func TestEmitE2EFinished_ZeroStart(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitE2EFinished(eval_metrics.SandboxAgentE2ETags{TurnID: 1}, nil, time.Time{})
	rec := fm.records[0]
	for _, v := range rec.values {
		if v.suffix == "e2e_duration" && v.v != 0 {
			t.Fatalf("zero start should give 0 duration, got %d", v.v)
		}
	}
}

// 未来时间: duration 应 clamp 到 0
func TestEmitE2EFinished_FutureStartClamped(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitE2EFinished(eval_metrics.SandboxAgentE2ETags{}, nil, time.Now().Add(5*time.Second))
	rec := fm.records[0]
	for _, v := range rec.values {
		if v.suffix == "e2e_duration" && v.v != 0 {
			t.Fatalf("future start should clamp to 0, got %d", v.v)
		}
	}
}

// 失败场景: error_type 走 ClassifyErrorType, success=false
func TestEmitE2EFinished_Failure(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitE2EFinished(eval_metrics.SandboxAgentE2ETags{TurnID: 5}, &noopMetricsErr{}, time.Now().Add(-10*time.Millisecond))
	rec := fm.records[0]
	if rec.tags["success"] != "false" {
		t.Fatalf("want success=false, got %q", rec.tags["success"])
	}
	if rec.tags["error_type"] == "-" || rec.tags["error_type"] == "" {
		t.Fatalf("failure should classify error_type, got %q", rec.tags["error_type"])
	}
}

// item_key / agent_name 等含非法字符时走 sanitizeTagValue: 非白名单字符 → '_'
func TestEmitE2EStarted_SanitizesTagValues(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitE2EStarted(eval_metrics.SandboxAgentE2ETags{
		ItemKey:       "key with space",
		DatasetKey:    "中文key",
		AgentName:     "agent name",
		ApplicationID: "app id",
	})
	rec := fm.records[0]
	if rec.tags["item_key"] != "key_with_space" {
		t.Fatalf("item_key not sanitized: %q", rec.tags["item_key"])
	}
	if rec.tags["agent_name"] != "agent_name" {
		t.Fatalf("agent_name not sanitized: %q", rec.tags["agent_name"])
	}
	if rec.tags["application_id"] != "app_id" {
		t.Fatalf("application_id not sanitized: %q", rec.tags["application_id"])
	}
	// 中文字符每个 UTF-8 字节各转成 '_'
	if rec.tags["dataset_key"] == "-" || rec.tags["dataset_key"] == "中文key" {
		t.Fatalf("dataset_key not sanitized: %q", rec.tags["dataset_key"])
	}
}

// metricTagNames 必须包含 turn_id (新增)
func TestMetricTagNamesIncludesTurnID(t *testing.T) {
	names := metricTagNames()
	for _, n := range names {
		if n == "turn_id" {
			return
		}
	}
	t.Fatalf("metricTagNames should include turn_id, got %v", names)
}

// invoke / experiment / step 三个既有 build*Tags 层也要携带 turn_id 占位符 (回归)
func TestBuildTags_TurnIDPlaceholderForNonE2E(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitInvokeStarted(eval_metrics.SandboxAgentInvokeTags{})
	impl.EmitExperimentStarted(eval_metrics.SandboxAgentExperimentTags{})
	impl.EmitStepStarted(eval_metrics.SandboxAgentStepTags{})
	if len(fm.records) != 3 {
		t.Fatalf("want 3 records, got %d", len(fm.records))
	}
	for i, rec := range fm.records {
		if rec.tags["turn_id"] != "-" {
			t.Fatalf("rec[%d] non-e2e turn_id should be placeholder, got %q", i, rec.tags["turn_id"])
		}
	}
}
