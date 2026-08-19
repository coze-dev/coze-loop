// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package step_event

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
)

// fakeMeter / fakeMetric 只做录制，用来断言 emit 携带的 tag 与 suffix。
type fakeMeter struct {
	err error
	m   *fakeMetric
}

func (f *fakeMeter) NewMetric(name string, types []metrics.MetricType, tagNames []string) (metrics.Metric, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.m = &fakeMetric{name: name, types: types, tagNames: tagNames}
	return f.m, nil
}

type emittedValue struct {
	mType  metrics.MetricType
	suffix string
	v      int64
}

type emittedRecord struct {
	tags   map[string]string
	values []emittedValue
}

type fakeMetric struct {
	mu       sync.Mutex
	name     string
	types    []metrics.MetricType
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
		rec.values = append(rec.values, emittedValue{mType: v.GetType(), suffix: v.GetSuffix(), v: val})
	}
	m.records = append(m.records, rec)
}

// newFakeImpl 每次返回独立实例，绕开 NewStepEventMetrics 的 sync.Once。
func newFakeImpl(t *testing.T) (*metricsImpl, *fakeMetric) {
	t.Helper()
	fm := &fakeMeter{}
	m, err := fm.NewMetric(metricName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, metricTagNames())
	require.NoError(t, err)
	return &metricsImpl{metric: m}, fm.m
}

// TestMetricTagNames_IsTheClosedLowCardinalitySet 是本包最重要的一条测试：
// tag 闭集必须恰好是 spec D3 定的 6 个低基数维度，且**不含**任何无界高基数标识。
// 有人日后想「顺手加个 invoke_id 方便定位」时，这条会挡住他。
func TestMetricTagNames_IsTheClosedLowCardinalitySet(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"step_name", "success", "error_type", "error_code", "agent_type", "round"}, metricTagNames())

	forbidden := []string{"invoke_id", "experiment_id", "item_id", "item_key", "log_id", "dataset_id", "dataset_version", "model_name", "event_type", "target_id", "dataset_key"}
	for _, name := range metricTagNames() {
		assert.NotContains(t, forbidden, name, "high-cardinality or redundant tag leaked into the closed tag set")
	}
}

func TestEmitStepStarted(t *testing.T) {
	t.Parallel()

	impl, fm := newFakeImpl(t)
	impl.EmitStepStarted(eval_metrics.StepEventTags{StepName: "agent_run", AgentType: "claude_code", Round: 2})

	require.Len(t, fm.records, 1)
	rec := fm.records[0]
	assert.Equal(t, map[string]string{
		"step_name":  "agent_run",
		"agent_type": "claude_code",
		"round":      "2",
		// started 指标下这三个恒为占位符：阶段刚开始，成败尚未发生。
		"success":    "-",
		"error_type": "-",
		"error_code": "-",
	}, rec.tags)
	require.Len(t, rec.values, 1)
	assert.Equal(t, emittedValue{mType: metrics.MetricTypeCounter, suffix: suffixStarted, v: 1}, rec.values[0])
}

func TestEmitStepFinished(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tags       eval_metrics.StepEventTags
		success    bool
		errorCode  int32
		durationMS int64

		wantTags     map[string]string
		wantDuration int64
	}{
		{
			name:       "success",
			tags:       eval_metrics.StepEventTags{StepName: "install", AgentType: "codex", Round: 0},
			success:    true,
			durationMS: 1500,
			wantTags: map[string]string{
				"step_name": "install", "agent_type": "codex", "round": "0",
				"success": "true", "error_type": "-", "error_code": "-",
			},
			wantDuration: 1500,
		},
		{
			// 默认分类是 engineering（spec D4 的默认值反转）。
			name:       "failure_with_code_defaults_to_engineering",
			tags:       eval_metrics.StepEventTags{StepName: "setup", AgentType: "codex", Round: 1},
			success:    false,
			errorCode:  600123,
			durationMS: 42,
			wantTags: map[string]string{
				"step_name": "setup", "agent_type": "codex", "round": "1",
				"success": "false", "error_type": "engineering", "error_code": "600123",
			},
			wantDuration: 42,
		},
		{
			name:    "failure_without_code_is_unknown",
			tags:    eval_metrics.StepEventTags{StepName: "evaluate", AgentType: "kimi_code", Round: 3},
			success: false,
			wantTags: map[string]string{
				"step_name": "evaluate", "agent_type": "kimi_code", "round": "3",
				"success": "false", "error_type": "unknown", "error_code": "-",
			},
		},
		{
			// 沙箱时钟异常产出的负耗时 clamp 到 0。
			name:       "negative_duration_is_clamped",
			tags:       eval_metrics.StepEventTags{StepName: "finish", AgentType: "nca"},
			success:    true,
			durationMS: -5,
			wantTags: map[string]string{
				"step_name": "finish", "agent_type": "nca", "round": "0",
				"success": "true", "error_type": "-", "error_code": "-",
			},
			wantDuration: 0,
		},
		{
			// 外部来源的 tag 值过白名单：中文 / 空格 / 换行 → '_'；空值 → 占位符。
			// 白名单是**按字节**做的（与前人一致），所以一个 3 字节的汉字变成 3 个 '_'：
			// "阶段 名\n" = 3+3+1+3+1 = 11 字节。
			name:    "external_tag_values_are_sanitized",
			tags:    eval_metrics.StepEventTags{StepName: "阶段 名\n", AgentType: ""},
			success: true,
			wantTags: map[string]string{
				"step_name": "___________", "agent_type": "-", "round": "0",
				"success": "true", "error_type": "-", "error_code": "-",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl, fm := newFakeImpl(t)
			impl.EmitStepFinished(tt.tags, tt.success, tt.errorCode, tt.durationMS)

			require.Len(t, fm.records, 1)
			rec := fm.records[0]
			assert.Equal(t, tt.wantTags, rec.tags)
			require.Len(t, rec.values, 2)
			assert.Equal(t, emittedValue{mType: metrics.MetricTypeCounter, suffix: suffixFinished, v: 1}, rec.values[0])
			assert.Equal(t, emittedValue{mType: metrics.MetricTypeTimer, suffix: suffixDuration, v: tt.wantDuration}, rec.values[1])
		})
	}
}

// TestNewStepEventMetrics_DegradesToNoop 覆盖降级路径：埋点组件缺失 / 构造失败一律 no-op + warn，
// 绝不 panic、绝不返回 error——否则一个启动期的埋点配置问题会拖垮整个服务。
func TestNewStepEventMetrics_DegradesToNoop(t *testing.T) {
	t.Parallel()

	// 走 newStepEventMetrics（非 once 版本），否则同包其它用例会互相污染单例。
	assert.IsType(t, &noopMetrics{}, newStepEventMetrics(nil), "nil meter must degrade to no-op")
	assert.IsType(t, &noopMetrics{}, newStepEventMetrics(&fakeMeter{err: errors.New("boom")}), "NewMetric error must degrade to no-op")
	assert.IsType(t, &noopMetrics{}, newStepEventMetrics(&nilReturningMeter{}), "nil metric must degrade to no-op")
	assert.IsType(t, &metricsImpl{}, newStepEventMetrics(&fakeMeter{}))

	n := &noopMetrics{}
	assert.NotPanics(t, func() {
		n.EmitStepStarted(eval_metrics.StepEventTags{StepName: "install"})
		n.EmitStepFinished(eval_metrics.StepEventTags{StepName: "install"}, false, 1, -1)
	})

	// once 版本同样不返回 nil interface。
	assert.NotNil(t, NewStepEventMetrics(nil))
}

// nilReturningMeter 复现「NewMetric 不报错但返回 nil」这种实现层坑。
type nilReturningMeter struct{}

func (nilReturningMeter) NewMetric(string, []metrics.MetricType, []string) (metrics.Metric, error) {
	return nil, nil
}

// TestMetricsImpl_NilMetricIsSafe 覆盖「metric 为 nil 时 emit 不 panic」。
func TestMetricsImpl_NilMetricIsSafe(t *testing.T) {
	t.Parallel()

	var impl *metricsImpl
	assert.NotPanics(t, func() {
		impl.EmitStepStarted(eval_metrics.StepEventTags{})
		impl.EmitStepFinished(eval_metrics.StepEventTags{}, true, 0, 0)
	})

	empty := &metricsImpl{}
	assert.NotPanics(t, func() {
		empty.EmitStepStarted(eval_metrics.StepEventTags{})
		empty.EmitStepFinished(eval_metrics.StepEventTags{}, true, 0, 0)
	})
}
