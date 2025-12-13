// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildTrajectoryFromSpans_Empty(t *testing.T) {
	t.Parallel()
	var spans SpanList
	traj := BuildTrajectoryFromSpans(spans)
	assert.Nil(t, traj)
}

func TestBuildTrajectoryFromSpans_ComplexTree(t *testing.T) {
	t.Parallel()
	traceID := "trace1"

	// 根 agent
	root := &Span{
		SpanID:         "r",
		ParentID:       "", // 作为root
		TraceID:        traceID,
		SpanName:       "root-agent",
		SpanType:       "agent",
		StartTime:      0,       // us
		DurationMicros: 3000000, // 3s
		Input:          "root-in",
		Output:         "root-out",
		TagsString:     map[string]string{},
		TagsLong:       map[string]int64{},
	}

	// 分支1：普通->model
	p1 := &Span{
		SpanID:    "p1",
		ParentID:  "r",
		TraceID:   traceID,
		SpanName:  "parser-1",
		SpanType:  "parser",
		StartTime: 100, // us
	}
	m1 := &Span{
		SpanID:         "m1",
		ParentID:       "p1",
		TraceID:        traceID,
		SpanName:       "model-1",
		SpanType:       "model",
		StartTime:      110,
		DurationMicros: 2000000, // 2s
		TagsLong: map[string]int64{
			"input_tokens":                 50,
			"output_tokens":                20,
			"latency_first_resp":           500000, // 0.5s
			"reasoning_tokens":             5,
			"input_cached_tokens":          3,
			"input_creation_cached_tokens": 2,
		},
	}

	// 分支2：tool 直接子节点，且报错
	t1 := &Span{
		SpanID:         "t1",
		ParentID:       "r",
		TraceID:        traceID,
		SpanName:       "tool-1",
		SpanType:       "tool",
		StartTime:      200,
		DurationMicros: 1000000, // 1s
		StatusCode:     500,
		TagsString: map[string]string{
			"error": "tool failed",
		},
	}

	// 分支3：子 agent
	a1 := &Span{
		SpanID:         "a1",
		ParentID:       "r",
		TraceID:        traceID,
		SpanName:       "sub-agent",
		SpanType:       "agent",
		StartTime:      300,
		DurationMicros: 3000000, // 3s
	}
	// a1的子节点：普通->model
	o2 := &Span{
		SpanID:    "o2",
		ParentID:  "a1",
		TraceID:   traceID,
		SpanName:  "remote-1",
		SpanType:  "remote",
		StartTime: 310,
	}
	m2 := &Span{
		SpanID:         "m2",
		ParentID:       "o2",
		TraceID:        traceID,
		SpanName:       "model-2",
		SpanType:       "model",
		StartTime:      320,
		DurationMicros: 1500000, // 1.5s
		StatusCode:     400,
		TagsString: map[string]string{
			"error": "model failed",
		},
		TagsLong: map[string]int64{
			"input_tokens":  100,
			"output_tokens": 60,
		},
	}
	// a1另一个子节点：tool，无错
	t2 := &Span{
		SpanID:         "t2",
		ParentID:       "a1",
		TraceID:        traceID,
		SpanName:       "tool-2",
		SpanType:       "tool",
		StartTime:      315,
		DurationMicros: 2000000, // 2s
		StatusCode:     0,
	}

	spans := SpanList{root, p1, m1, t1, a1, o2, m2, t2}
	traj := BuildTrajectoryFromSpans(spans)
	assert.NotNil(t, traj)
	assert.NotNil(t, traj.RootStep)
	assert.NotNil(t, traj.ID)
	assert.Equal(t, traceID, *traj.ID)

	// RootStep 校验
	assert.NotNil(t, traj.RootStep.BasicInfo)
	assert.Equal(t, "0", traj.RootStep.BasicInfo.StartedAt)   // 0us -> 0ms
	assert.Equal(t, "3000", traj.RootStep.BasicInfo.Duration) // 3s -> 3000ms

	// AgentSteps 包含 root 和 a1
	assert.Equal(t, 2, len(traj.AgentSteps))
	// 找到 root 对应的 AgentStep
	var rootAgentStep, a1AgentStep *AgentStep
	for _, s := range traj.AgentSteps {
		if s != nil && s.ID != nil {
			switch *s.ID {
			case "r":
				rootAgentStep = s
			case "a1":
				a1AgentStep = s
			default:
			}
		}
	}
	assert.NotNil(t, rootAgentStep)
	assert.NotNil(t, a1AgentStep)

	// root 的子步骤应包含：p1(普通)、m1(model 首个)、t1(tool 首个)、a1(agent 首个)
	ids := make([]string, 0)
	for _, st := range rootAgentStep.Steps {
		if st != nil && st.ID != nil {
			ids = append(ids, *st.ID)
		}
	}
	assert.ElementsMatch(t, []string{"p1", "m1", "t1", "a1"}, ids)

	// a1 的子步骤应包含：o2(普通)、m2(model 首个)、t2(tool 首个)
	ids = ids[:0]
	for _, st := range a1AgentStep.Steps {
		if st != nil && st.ID != nil {
			ids = append(ids, *st.ID)
		}
	}
	assert.ElementsMatch(t, []string{"o2", "m2", "t2"}, ids)

	// RootStep 的 MetricsInfo 聚合所有 AgentStep 的 Steps（去重）
	assert.NotNil(t, traj.RootStep.MetricsInfo)
	mi := traj.RootStep.MetricsInfo
	assert.NotNil(t, mi.LlmDuration)
	assert.Equal(t, "3500", *mi.LlmDuration) // 2000(m1)+1500(m2)
	assert.NotNil(t, mi.ToolDuration)
	assert.Equal(t, "3000", *mi.ToolDuration) // 1000(t1)+2000(t2)
	assert.NotNil(t, mi.ModelErrorRate)
	assert.InDelta(t, 0.5, *mi.ModelErrorRate, 1e-9) // 1/2
	assert.NotNil(t, mi.ToolErrorRate)
	assert.InDelta(t, 0.5, *mi.ToolErrorRate, 1e-9) // 1/2
	assert.NotNil(t, mi.ToolStepProportion)
	assert.InDelta(t, 0.5, *mi.ToolStepProportion, 1e-9) // 2/(2+2)
	assert.NotNil(t, mi.InputTokens)
	assert.Equal(t, int32(150), *mi.InputTokens)
	assert.NotNil(t, mi.OutputTokens)
	assert.Equal(t, int32(80), *mi.OutputTokens)
	// 错误分布
	assert.Contains(t, mi.ToolErrors, int32(500))
	assert.Equal(t, []string{"t1"}, mi.ToolErrors[500])
	assert.Contains(t, mi.ModelErrors, int32(400))
	assert.Equal(t, []string{"m2"}, mi.ModelErrors[400])

	// 各 AgentStep 的 MetricsInfo
	assert.NotNil(t, rootAgentStep.MetricsInfo)
	assert.Equal(t, "2000", *rootAgentStep.MetricsInfo.LlmDuration)
	assert.Equal(t, "1000", *rootAgentStep.MetricsInfo.ToolDuration)
	assert.InDelta(t, 0.0, *rootAgentStep.MetricsInfo.ModelErrorRate, 1e-9)
	assert.InDelta(t, 1.0, *rootAgentStep.MetricsInfo.ToolErrorRate, 1e-9)
	assert.Equal(t, int32(50), *rootAgentStep.MetricsInfo.InputTokens)
	assert.Equal(t, int32(20), *rootAgentStep.MetricsInfo.OutputTokens)

	assert.NotNil(t, a1AgentStep.MetricsInfo)
	assert.Equal(t, "1500", *a1AgentStep.MetricsInfo.LlmDuration)
	assert.Equal(t, "2000", *a1AgentStep.MetricsInfo.ToolDuration)
	assert.InDelta(t, 1.0, *a1AgentStep.MetricsInfo.ModelErrorRate, 1e-9)
	assert.InDelta(t, 0.0, *a1AgentStep.MetricsInfo.ToolErrorRate, 1e-9)
	assert.Equal(t, int32(100), *a1AgentStep.MetricsInfo.InputTokens)
	assert.Equal(t, int32(60), *a1AgentStep.MetricsInfo.OutputTokens)

	// MarshalString
	s, err := traj.MarshalString()
	assert.NoError(t, err)
	assert.NotEmpty(t, s)
}

func TestGetDirectChildren_Sorting(t *testing.T) {
	t.Parallel()
	parent := &Span{SpanID: "p"}
	c1 := &Span{SpanID: "c1", ParentID: "p", StartTime: 200}
	c2 := &Span{SpanID: "c2", ParentID: "p", StartTime: 100}
	c3 := &Span{SpanID: "c3", ParentID: "p", StartTime: 150}
	spanMap := map[string]*Span{
		"p":  parent,
		"c1": c1,
		"c2": c2,
		"c3": c3,
	}
	children := getDirectChildren(parent, spanMap)
	ids := []string{children[0].SpanID, children[1].SpanID, children[2].SpanID}
	assert.Equal(t, []string{"c2", "c3", "c1"}, ids)
}

func TestGetStepType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, StepTypeAgent, getStepType(&Span{SpanType: "agent"}))
	assert.Equal(t, StepTypeModel, getStepType(&Span{SpanType: "model"}))
	assert.Equal(t, StepTypeTool, getStepType(&Span{SpanType: "tool"}))
	// default 分支：父ID为空且不是上述类型，按 tool 处理
	assert.Equal(t, StepTypeTool, getStepType(&Span{SpanType: "unknown", ParentID: ""}))
	// 非 root 的其它类型，返回原始 SpanType
	assert.Equal(t, StepType("unknown"), getStepType(&Span{SpanType: "unknown", ParentID: "x"}))
}

func TestBuildModelInfo(t *testing.T) {
	t.Parallel()
	model := &Span{
		TagsLong: map[string]int64{
			"input_tokens":                 10,
			"output_tokens":                20,
			"latency_first_resp":           123456, // us
			"reasoning_tokens":             7,
			"input_cached_tokens":          3,
			"input_creation_cached_tokens": 2,
		},
	}
	mi := buildModelInfo(model)
	assert.NotNil(t, mi)
	assert.Equal(t, int64(10), mi.InputTokens)
	assert.Equal(t, int64(20), mi.OutputTokens)
	assert.Equal(t, strconv.FormatInt(123456/1000, 10), mi.LatencyFirstResp)
	assert.Equal(t, int64(7), mi.ReasoningTokens)
	assert.Equal(t, int64(3), mi.InputReadCachedTokens)
	assert.Equal(t, int64(2), mi.InputCreationCachedTokens)
}

func TestCalculateMetricsInfo_Empty(t *testing.T) {
	t.Parallel()
	mi := calculateMetricsInfo(nil, nil)
	assert.Nil(t, mi)
}

func TestBuildTrajectoryFromSpans_NoRoot(t *testing.T) {
	t.Parallel()
	traceID := "traceX"
	// 不包含 parent==""/"0" 的 root
	p := &Span{SpanID: "p", ParentID: "x", TraceID: traceID, SpanType: "parser", StartTime: 10}
	a := &Span{SpanID: "a", ParentID: "p", TraceID: traceID, SpanType: "agent", StartTime: 20, DurationMicros: 1000000}
	tool := &Span{SpanID: "t", ParentID: "a", TraceID: traceID, SpanType: "tool", StartTime: 30, DurationMicros: 500000}
	spans := SpanList{p, a, tool}
	traj := BuildTrajectoryFromSpans(spans)
	assert.NotNil(t, traj)
	assert.Nil(t, traj.RootStep)
	assert.NotNil(t, traj.ID)
	assert.Equal(t, traceID, *traj.ID)
	assert.Equal(t, 1, len(traj.AgentSteps))
	assert.Equal(t, "a", *traj.AgentSteps[0].ID)
	// agent 步的 metrics 应存在
	assert.NotNil(t, traj.AgentSteps[0].MetricsInfo)
}

func TestBuildRootMetricsInfo_Dedup(t *testing.T) {
	t.Parallel()
	// 构造两个 AgentStep，包含重复的同一 model Step ID
	mSpan := &Span{SpanID: "m", ParentID: "p", SpanType: "model", DurationMicros: 1000000}
	toolSpan := &Span{SpanID: "t", ParentID: "p", SpanType: "tool", DurationMicros: 1000000}
	modelStep := buildStep(mSpan)
	toolStep := buildStep(toolSpan)
	as1 := &AgentStep{Steps: []*Step{modelStep, toolStep}}
	// 重复加入同一 ID 的 modelStep
	as2 := &AgentStep{Steps: []*Step{modelStep}}
	mi := buildRootMetricsInfo([]*AgentStep{as1, as2})
	assert.NotNil(t, mi)
	// 因去重，model 时长只计算一次
	assert.NotNil(t, mi.LlmDuration)
	assert.Equal(t, "1000", *mi.LlmDuration)
	assert.NotNil(t, mi.ToolDuration)
	assert.Equal(t, "1000", *mi.ToolDuration)
}
