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
		ExperimentID:   100,
		ItemID:         200,
		InvokeID:       "300",
		DatasetID:      400,
		DatasetVersion: 500,
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
	impl.EmitInvokeFinished(eval_metrics.SandboxAgentInvokeTags{ExperimentID: 1, InvokeID: "x"}, nil, 0, submitTime)
	if len(fm.records) != 1 {
		t.Fatalf("want 1 record, got %d", len(fm.records))
	}
	rec := fm.records[0]
	if rec.tags["success"] != "true" || rec.tags["error_type"] != "-" {
		t.Fatalf("success/error_type wrong: %+v", rec.tags)
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
}

func TestEmitStepStarted(t *testing.T) {
	impl, fm := newFakeImpl(t)
	impl.EmitStepStarted(eval_metrics.SandboxAgentStepTags{
		ExperimentID: 1, ItemID: 2, InvokeID: "3", DatasetID: 4, DatasetVersion: 5, StepName: "plan",
	})
	if len(fm.records) != 1 {
		t.Fatalf("want 1 record, got %d", len(fm.records))
	}
	rec := fm.records[0]
	if rec.tags["step_name"] != "plan" || rec.tags["item_id"] != "2" || rec.tags["invoke_id"] != "3" {
		t.Fatalf("step tags wrong: %+v", rec.tags)
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
