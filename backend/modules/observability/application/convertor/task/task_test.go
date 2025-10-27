// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"

	kitCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	kitTask "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	entityCommon "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestTaskDOs2DTOs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	t.Run("empty input", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, TaskDOs2DTOs(ctx, nil, nil))
	})

	t.Run("normal conversion", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		run1 := &entity.TaskRun{
			ID:          1,
			TaskID:      100,
			WorkspaceID: 200,
			TaskType:    kitTask.TaskRunTypeNewData,
			RunStatus:   kitTask.TaskStatusRunning,
			RunDetail: &entity.RunDetail{
				SuccessCount: 3,
				FailedCount:  1,
				TotalCount:   4,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		run2 := &entity.TaskRun{
			ID:          2,
			TaskID:      100,
			WorkspaceID: 200,
			TaskType:    kitTask.TaskRunTypeBackFill,
			RunStatus:   kitTask.TaskStatusPending,
			RunDetail: &entity.RunDetail{
				SuccessCount: 1,
				FailedCount:  2,
				TotalCount:   3,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		userInfos := map[string]*entityCommon.UserInfo{
			"creator": {
				Name:   "Alice",
				UserID: "creator",
			},
		}

		taskDO := &entity.ObservabilityTask{
			ID:          100,
			WorkspaceID: 200,
			Name:        "task-name",
			Description: ptr.Of("desc"),
			TaskType:    kitTask.TaskTypeAutoEval,
			TaskStatus:  kitTask.TaskStatusRunning,
			SpanFilter:  &filter.SpanFilterFields{},
			EffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(time.Hour).UnixMilli(),
				EndAt:   now.Add(2 * time.Hour).UnixMilli(),
			},
			BackfillEffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(-2 * time.Hour).UnixMilli(),
				EndAt:   now.Add(-time.Hour).UnixMilli(),
			},
			Sampler: &entity.Sampler{
				SampleRate:    0.5,
				SampleSize:    10,
				IsCycle:       true,
				CycleCount:    2,
				CycleInterval: 3,
				CycleTimeUnit: kitTask.TimeUnitDay,
			},
			TaskConfig: &entity.TaskConfig{},
			CreatedAt:  now,
			UpdatedAt:  now,
			CreatedBy:  "creator",
			UpdatedBy:  "updater",
			TaskRuns:   []*entity.TaskRun{run1, run2},
		}

		tasks := TaskDOs2DTOs(ctx, []*entity.ObservabilityTask{taskDO}, userInfos)
		if assert.Len(t, tasks, 1) {
			got := tasks[0]
			assert.Equal(t, taskDO.ID, got.GetID())
			assert.Equal(t, taskDO.Name, got.GetName())
			assert.Equal(t, taskDO.Description, got.Description)
			assert.Equal(t, int64(7), *got.TaskDetail.TotalCount)
			assert.Equal(t, int64(4), *got.TaskDetail.SuccessCount)
			assert.Equal(t, int64(3), *got.TaskDetail.FailedCount)
			assert.Equal(t, "Alice", got.BaseInfo.GetCreatedBy().GetName())
			assert.Equal(t, "updater", got.BaseInfo.GetUpdatedBy().GetUserID())
		}
	})
}

func TestTaskConfigDTO2DO(t *testing.T) {
	t.Parallel()

	schema := &dataset.FieldSchema{
		Key:         gptr.Of("field_key"),
		Name:        gptr.Of("Field"),
		Description: gptr.Of("desc"),
	}

	dto := &kitTask.TaskConfig{
		AutoEvaluateConfigs: []*kitTask.AutoEvaluateConfig{
			{
				EvaluatorVersionID: 1,
				EvaluatorID:        2,
				FieldMappings: []*kitTask.EvaluateFieldMapping{
					{
						FieldSchema:        schema,
						TraceFieldKey:      "trace.input",
						TraceFieldJsonpath: "$.result",
					},
					{
						FieldSchema:        schema,
						TraceFieldKey:      "trace.other",
						TraceFieldJsonpath: "$.result",
					},
					{
						FieldSchema:        schema,
						TraceFieldKey:      "trace.array",
						TraceFieldJsonpath: "$.result[0]",
					},
				},
			},
		},
		DataReflowConfig: []*kitTask.DataReflowConfig{
			{
				DatasetID:   gptr.Of(int64(10)),
				DatasetName: gptr.Of("dataset"),
				DatasetSchema: gptr.Of(dataset.DatasetSchema{
					FieldSchemas: []*dataset.FieldSchema{schema},
				}),
				FieldMappings: []*dataset.FieldMapping{
					{
						FieldSchema:        schema,
						TraceFieldKey:      "trace.field",
						TraceFieldJsonpath: "$.value",
					},
				},
			},
		},
	}

	cfg := TaskConfigDTO2DO(dto)
	if assert.NotNil(t, cfg) && assert.Len(t, cfg.AutoEvaluateConfigs, 1) {
		mappings := cfg.AutoEvaluateConfigs[0].FieldMappings
		if assert.Len(t, mappings, 3) {
			assert.Equal(t, "result", ptr.From(mappings[0].EvalSetName))
			assert.Equal(t, "result_", ptr.From(mappings[1].EvalSetName))
			assert.Equal(t, "result_0", ptr.From(mappings[2].EvalSetName))
		}
	}

	if assert.Len(t, cfg.DataReflowConfig, 1) {
		reflow := cfg.DataReflowConfig[0]
		assert.Equal(t, int64(10), ptr.From(reflow.DatasetID))
		assert.Equal(t, "dataset", ptr.From(reflow.DatasetName))
		assert.Equal(t, "trace.field", reflow.FieldMappings[0].TraceFieldKey)
	}
}

func TestTaskDTO2DO(t *testing.T) {
	t.Parallel()

	now := time.Now()
	spanFilter := &filter.SpanFilterFields{}
	dto := &kitTask.Task{
		ID:          gptr.Of(int64(11)),
		Name:        "dto",
		Description: gptr.Of("dto-desc"),
		WorkspaceID: gptr.Of(int64(22)),
		TaskType:    kitTask.TaskTypeAutoEval,
		TaskStatus:  gptr.Of(kitTask.TaskStatusRunning),
		Rule: &kitTask.Rule{
			SpanFilters: spanFilter,
			EffectiveTime: &kitTask.EffectiveTime{
				StartAt: gptr.Of(now.Add(time.Hour).UnixMilli()),
				EndAt:   gptr.Of(now.Add(2 * time.Hour).UnixMilli()),
			},
			Sampler: &kitTask.Sampler{
				SampleRate:    gptr.Of(float64(0.3)),
				SampleSize:    gptr.Of(int64(5)),
				IsCycle:       gptr.Of(true),
				CycleCount:    gptr.Of(int64(1)),
				CycleInterval: gptr.Of(int64(2)),
				CycleTimeUnit: gptr.Of(kitTask.TimeUnitWeek),
			},
			BackfillEffectiveTime: &kitTask.EffectiveTime{
				StartAt: gptr.Of(now.Add(-2 * time.Hour).UnixMilli()),
				EndAt:   gptr.Of(now.Add(-time.Hour).UnixMilli()),
			},
		},
		TaskConfig: &kitTask.TaskConfig{},
		TaskDetail: &kitTask.RunDetail{
			SuccessCount: gptr.Of(int64(1)),
			FailedCount:  gptr.Of(int64(2)),
			TotalCount:   gptr.Of(int64(3)),
		},
		BaseInfo: &kitCommon.BaseInfo{
			CreatedBy: &kitCommon.UserInfo{UserID: gptr.Of("creator")},
			UpdatedBy: &kitCommon.UserInfo{UserID: gptr.Of("updater")},
		},
	}

	overrideSpan := &filter.SpanFilterFields{
		PlatformType: gptr.Of(kitCommon.PlatformType("coze")),
	}

	entityTask := TaskDTO2DO(dto, "override", overrideSpan)
	if assert.NotNil(t, entityTask) {
		assert.Equal(t, int64(11), entityTask.ID)
		assert.Equal(t, "override", entityTask.CreatedBy)
		assert.Equal(t, overrideSpan, entityTask.SpanFilter)
		assert.NotZero(t, entityTask.CreatedAt.Unix())
		assert.Equal(t, int64(1), entityTask.TaskDetail.SuccessCount)
		assert.Equal(t, float64(0.3), entityTask.Sampler.SampleRate)
	}
}

func TestSpanFilterPO2DO(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	valid := &filter.SpanFilterFields{
		PlatformType: gptr.Of(kitCommon.PlatformType("loop")),
		SpanListType: gptr.Of(kitCommon.SpanListType("root")),
	}
	data, err := sonic.Marshal(valid)
	assert.NoError(t, err)

	result := SpanFilterPO2DO(ctx, gptr.Of(string(data)))
	assert.Equal(t, valid, result)

	invalid := "{" // invalid json
	assert.Nil(t, SpanFilterPO2DO(ctx, &invalid))
}

func TestCheckEffectiveTime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	getCode := func(err error) int32 {
		statusErr, ok := errorx.FromStatusError(err)
		if !ok {
			return 0
		}
		return statusErr.Code()
	}

	futureStart := now.Add(2 * time.Hour).UnixMilli()
	futureEnd := now.Add(3 * time.Hour).UnixMilli()

	cases := []struct {
		name        string
		effective   *kitTask.EffectiveTime
		status      kitTask.TaskStatus
		current     *entity.EffectiveTime
		wantStart   int64
		wantEnd     int64
		wantErrCode int32
	}{
		{
			name:        "nil current",
			effective:   &kitTask.EffectiveTime{StartAt: gptr.Of(futureStart), EndAt: gptr.Of(futureEnd)},
			status:      kitTask.TaskStatusUnstarted,
			current:     nil,
			wantErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name:        "start after end",
			effective:   &kitTask.EffectiveTime{StartAt: gptr.Of(futureEnd), EndAt: gptr.Of(futureStart)},
			status:      kitTask.TaskStatusUnstarted,
			current:     &entity.EffectiveTime{StartAt: futureStart, EndAt: futureEnd},
			wantErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name:        "update start in past",
			effective:   &kitTask.EffectiveTime{StartAt: gptr.Of(now.Add(-time.Hour).UnixMilli()), EndAt: gptr.Of(futureEnd)},
			status:      kitTask.TaskStatusRunning,
			current:     &entity.EffectiveTime{StartAt: futureStart, EndAt: futureEnd},
			wantErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name:        "update end in past",
			effective:   &kitTask.EffectiveTime{StartAt: gptr.Of(futureStart), EndAt: gptr.Of(now.Add(-time.Hour).UnixMilli())},
			status:      kitTask.TaskStatusRunning,
			current:     &entity.EffectiveTime{StartAt: futureStart, EndAt: futureEnd},
			wantErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name:      "unstarted updates both",
			effective: &kitTask.EffectiveTime{StartAt: gptr.Of(futureStart), EndAt: gptr.Of(futureEnd)},
			status:    kitTask.TaskStatusUnstarted,
			current:   &entity.EffectiveTime{StartAt: 100, EndAt: 200},
			wantStart: futureStart,
			wantEnd:   futureEnd,
		},
		{
			name:      "running keeps start",
			effective: &kitTask.EffectiveTime{StartAt: gptr.Of(futureEnd), EndAt: gptr.Of(futureEnd + 1000)},
			status:    kitTask.TaskStatusRunning,
			current:   &entity.EffectiveTime{StartAt: 111, EndAt: 222},
			wantStart: 111,
			wantEnd:   futureEnd + 1000,
		},
		{
			name:        "invalid status",
			effective:   &kitTask.EffectiveTime{StartAt: gptr.Of(futureStart), EndAt: gptr.Of(futureEnd)},
			status:      kitTask.TaskStatus("unknown"),
			current:     &entity.EffectiveTime{StartAt: futureStart, EndAt: futureEnd},
			wantErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := CheckEffectiveTime(ctx, tc.effective, tc.status, tc.current)
			if tc.wantErrCode != 0 {
				assert.NotNil(t, err)
				assert.Equal(t, tc.wantErrCode, getCode(err))
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			if assert.NotNil(t, got) {
				assert.Equal(t, tc.wantStart, got.StartAt)
				assert.Equal(t, tc.wantEnd, got.EndAt)
			}
		})
	}
}

func TestCheckTaskStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	getCode := func(err error) int32 {
		statusErr, ok := errorx.FromStatusError(err)
		if !ok {
			return 0
		}
		return statusErr.Code()
	}

	cases := []struct {
		name        string
		status      kitTask.TaskStatus
		current     kitTask.TaskStatus
		want        kitTask.TaskStatus
		wantErrCode int32
	}{
		{"unstarted ok", kitTask.TaskStatusUnstarted, kitTask.TaskStatusUnstarted, kitTask.TaskStatusUnstarted, 0},
		{"unstarted invalid", kitTask.TaskStatusUnstarted, kitTask.TaskStatusRunning, "", obErrorx.CommercialCommonInvalidParamCodeCode},
		{"running ok", kitTask.TaskStatusRunning, kitTask.TaskStatusPending, kitTask.TaskStatusRunning, 0},
		{"running invalid", kitTask.TaskStatusRunning, kitTask.TaskStatusSuccess, "", obErrorx.CommercialCommonInvalidParamCodeCode},
		{"pending ok", kitTask.TaskStatusPending, kitTask.TaskStatusRunning, kitTask.TaskStatusPending, 0},
		{"disabled ok", kitTask.TaskStatusDisabled, kitTask.TaskStatusPending, kitTask.TaskStatusDisabled, 0},
		{"success ok", kitTask.TaskStatusSuccess, kitTask.TaskStatusRunning, kitTask.TaskStatusSuccess, 0},
		{"pending no transition", kitTask.TaskStatusPending, kitTask.TaskStatusDisabled, "", 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := CheckTaskStatus(ctx, tc.status, tc.current)
			if tc.wantErrCode != 0 {
				assert.Equal(t, tc.wantErrCode, getCode(err))
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetLastPartAfterDot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"foo.bar.baz", "baz"},
		{"foo.bar.", "bar"},
		{"no_dot", "no_dot"},
		{"array[0]", "array_0"},
		{"prefix.value[2]", "value_2"},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.want, getLastPartAfterDot(tc.input))
	}
}

func TestProcessBracket(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "field_1", processBracket("field[1]"))
	assert.Equal(t, "field", processBracket("field"))
}

func TestToJSONString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", ToJSONString(context.Background(), nil))

	type sample struct {
		A int    `json:"a"`
		B string `json:"b"`
	}

	jsonStr := ToJSONString(context.Background(), sample{A: 1, B: "value"})
	assert.Equal(t, "{\"a\":1,\"b\":\"value\"}", jsonStr)
}

func TestBuildTaskRunBaseInfo(t *testing.T) {
	t.Parallel()

	now := time.Now()
	run := &entity.TaskRun{CreatedAt: now, UpdatedAt: now}
	base := buildTaskRunBaseInfo(run, nil)
	if assert.NotNil(t, base) {
		assert.Equal(t, now.UnixMilli(), ptr.From(base.CreatedAt))
		assert.Equal(t, "", ptr.From(base.CreatedBy.UserID))
		assert.Equal(t, "", ptr.From(base.UpdatedBy.UserID))
	}
}
