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
	"github.com/stretchr/testify/require"

	kitCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	kitTask "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	entityCommon "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
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
			BackfillDetail: &entity.BackfillDetail{
				SuccessCount:      3,
				FailedCount:       1,
				TotalCount:        4,
				BackfillStatus:    kitTask.RunStatusRunning,
				LastSpanPageToken: "abc",
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
			SpanFilter: &entity.SpanFilterFields{
				PlatformType: kitCommon.PlatformTypeCozeloop,
				SpanListType: kitCommon.SpanListTypeRootSpan,
				Filters: loop_span.FilterFields{
					QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumAnd),
					FilterFields: []*loop_span.FilterField{},
				},
			},
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
				CycleTimeUnit: entity.TimeUnitDay,
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
		EvaluationExperimentConfig: &kitTask.EvaluationExperimentConfig{
			ItemConcurrencyCount: gptr.Of(int32(3)),
			ItemMaxRetryCount:    gptr.Of(int32(4)),
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
		if assert.NotNil(t, cfg.EvaluationExperimentConfig) {
			assert.Equal(t, int32(3), ptr.From(cfg.EvaluationExperimentConfig.ItemConcurrencyCount))
			assert.Equal(t, int32(4), ptr.From(cfg.EvaluationExperimentConfig.ItemMaxRetryCount))
		}
		mappings := cfg.AutoEvaluateConfigs[0].FieldMappings
		if assert.Len(t, mappings, 3) {
			assert.Equal(t, "result", ptr.From(mappings[0].EvalSetName))
			assert.Equal(t, "result_", ptr.From(mappings[1].EvalSetName))
			assert.Equal(t, "result_0", ptr.From(mappings[2].EvalSetName))
			assert.Equal(t, "result", ptr.From(mappings[0].DatasetKey))
			assert.Equal(t, "result_", ptr.From(mappings[1].DatasetKey))
			assert.Equal(t, "result_0", ptr.From(mappings[2].DatasetKey))
		}
	}

	if assert.Len(t, cfg.DataReflowConfig, 1) {
		reflow := cfg.DataReflowConfig[0]
		assert.Equal(t, int64(10), ptr.From(reflow.DatasetID))
		assert.Equal(t, "dataset", ptr.From(reflow.DatasetName))
		assert.Equal(t, "trace.field", reflow.FieldMappings[0].TraceFieldKey)
	}
}

func TestTaskConfigDO2DTO_EvaluationExperimentConfig(t *testing.T) {
	t.Parallel()

	do := &entity.TaskConfig{
		EvaluationExperimentConfig: &entity.EvaluationExperimentConfig{
			ItemConcurrencyCount: gptr.Of(int32(5)),
			ItemMaxRetryCount:    gptr.Of(int32(6)),
			SourceTargetID:       gptr.Of("target-123"),
			ExptTemplateID:       gptr.Of(int64(999)),
			SourceTargetVersion:  gptr.Of("v1.0"),
			FullEvalSetFieldMappings: []*entity.EvaluateFieldMapping{
				{
					FieldSchema: &dataset.FieldSchema{
						Key:  gptr.Of("key1"),
						Name: gptr.Of("field1"),
					},
					TraceFieldKey:      "Input",
					TraceFieldJsonpath: "$.input",
					EvalSetName:        gptr.Of("field1"),
				},
			},
		},
	}
	dto := TaskConfigDO2DTO(do)
	if assert.NotNil(t, dto) {
		ec := dto.EvaluationExperimentConfig
		if assert.NotNil(t, ec) {
			assert.Equal(t, int32(5), ptr.From(ec.ItemConcurrencyCount))
			assert.Equal(t, int32(6), ptr.From(ec.ItemMaxRetryCount))
			assert.Equal(t, "target-123", ptr.From(ec.SourceTargetID))
			assert.Equal(t, int64(999), ptr.From(ec.ExptTemplateID))
			assert.Equal(t, "v1.0", ptr.From(ec.SourceTargetVersion))
			if assert.Len(t, ec.FullEvalSetFieldMappings, 1) {
				fm := ec.FullEvalSetFieldMappings[0]
				assert.Equal(t, "Input", fm.TraceFieldKey)
				assert.Equal(t, "$.input", fm.TraceFieldJsonpath)
				assert.Equal(t, "field1", ptr.From(fm.EvalSetName))
			}
		}
	}

	assert.Nil(t, EvaluationExperimentConfigDO2DTO(nil))
}

func TestTaskDTO2DO(t *testing.T) {
	t.Parallel()

	now := time.Now()
	spanFilter := &filter.SpanFilterFields{
		PlatformType: gptr.Of(kitCommon.PlatformTypeCozeloop),
		SpanListType: gptr.Of(kitCommon.SpanListTypeRootSpan),
		Filters: &filter.FilterFields{
			QueryAndOr:   ptr.Of(filter.QueryRelationAnd),
			FilterFields: []*filter.FilterField{},
		},
	}
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

	entityTask := TaskDTO2DO(dto)
	if assert.NotNil(t, entityTask) {
		assert.Equal(t, int64(11), entityTask.ID)
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

func TestSourceInfoListDO2DTO(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, SourceInfoListDO2DTO(nil))
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, SourceInfoListDO2DTO([]*entity.SourceInfo{}))
	})

	t.Run("list with nil elements skips them", func(t *testing.T) {
		t.Parallel()
		input := []*entity.SourceInfo{
			nil,
			{Name: gptr.Of("sdk-go"), Version: gptr.Of("1.0.0")},
			nil,
		}
		result := SourceInfoListDO2DTO(input)
		if assert.Len(t, result, 1) {
			assert.Equal(t, gptr.Of("sdk-go"), result[0].Name)
			assert.Equal(t, gptr.Of("1.0.0"), result[0].Version)
		}
	})

	t.Run("normal conversion", func(t *testing.T) {
		t.Parallel()
		input := []*entity.SourceInfo{
			{Name: gptr.Of("sdk-go"), Version: gptr.Of("1.0.0")},
			{Name: gptr.Of("sdk-python"), Version: gptr.Of("2.0.0")},
		}
		result := SourceInfoListDO2DTO(input)
		if assert.Len(t, result, 2) {
			assert.Equal(t, gptr.Of("sdk-go"), result[0].Name)
			assert.Equal(t, gptr.Of("1.0.0"), result[0].Version)
			assert.Equal(t, gptr.Of("sdk-python"), result[1].Name)
			assert.Equal(t, gptr.Of("2.0.0"), result[1].Version)
		}
	})
}

func TestSpanFilterDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns empty struct", func(t *testing.T) {
		t.Parallel()
		result := SpanFilterDTO2DO(nil)
		assert.NotNil(t, result)
		assert.Equal(t, new(entity.SpanFilterFields), result)
	})

	t.Run("normal conversion", func(t *testing.T) {
		t.Parallel()
		dto := &filter.SpanFilterFields{
			PlatformType: gptr.Of(kitCommon.PlatformTypeCozeloop),
			SpanListType: gptr.Of(kitCommon.SpanListTypeRootSpan),
			Filters: &filter.FilterFields{
				QueryAndOr:   ptr.Of(filter.QueryRelationAnd),
				FilterFields: []*filter.FilterField{},
			},
		}
		result := SpanFilterDTO2DO(dto)
		if assert.NotNil(t, result) {
			assert.Equal(t, kitCommon.PlatformTypeCozeloop, string(result.PlatformType))
			assert.Equal(t, kitCommon.SpanListTypeRootSpan, string(result.SpanListType))
		}
	})
}

func TestSourceInfoListDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, SourceInfoListDTO2DO(nil))
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, SourceInfoListDTO2DO([]*kitTask.SourceInfo{}))
	})

	t.Run("skips nil elements", func(t *testing.T) {
		t.Parallel()
		result := SourceInfoListDTO2DO([]*kitTask.SourceInfo{
			nil,
			{Name: gptr.Of("svc"), Version: gptr.Of("v1")},
		})
		if assert.Len(t, result, 1) {
			assert.Equal(t, "svc", ptr.From(result[0].Name))
			assert.Equal(t, "v1", ptr.From(result[0].Version))
		}
	})

	t.Run("normal conversion", func(t *testing.T) {
		t.Parallel()
		result := SourceInfoListDTO2DO([]*kitTask.SourceInfo{
			{Name: gptr.Of("a"), Version: gptr.Of("1.0")},
			{Name: gptr.Of("b"), Version: gptr.Of("2.0")},
		})
		if assert.Len(t, result, 2) {
			assert.Equal(t, "a", ptr.From(result[0].Name))
			assert.Equal(t, "1.0", ptr.From(result[0].Version))
			assert.Equal(t, "b", ptr.From(result[1].Name))
			assert.Equal(t, "2.0", ptr.From(result[1].Version))
		}
	})
}

func TestTaskConfigDTO2DO_FullEvalSetAndSourceInfo(t *testing.T) {
	t.Parallel()

	schema := &dataset.FieldSchema{
		Key:         gptr.Of("output_key"),
		Name:        gptr.Of("Output"),
		ContentType: gptr.Of("Text"),
	}

	dto := &kitTask.TaskConfig{
		AutoEvaluateConfigs: []*kitTask.AutoEvaluateConfig{
			{
				EvaluatorVersionID: 1,
				EvaluatorID:        2,
				FieldMappings: []*kitTask.EvaluateFieldMapping{
					{
						FieldSchema:        schema,
						TraceFieldKey:      "Input",
						TraceFieldJsonpath: "$.input",
						EvalSetName:        gptr.Of("input_field"),
					},
				},
			},
		},
		EvaluationExperimentConfig: &kitTask.EvaluationExperimentConfig{
			ItemConcurrencyCount: gptr.Of(int32(5)),
			ItemMaxRetryCount:    gptr.Of(int32(3)),
			ExptTemplateID:       gptr.Of(int64(888)),
			SourceTargetID:       gptr.Of("target-1"),
			SourceTargetVersion:  gptr.Of("v2"),
			FullEvalSetFieldMappings: []*kitTask.EvaluateFieldMapping{
				{
					FieldSchema:        schema,
					TraceFieldKey:      "Output",
					TraceFieldJsonpath: "$.output",
					EvalSetName:        gptr.Of("output_field"),
				},
				{
					FieldSchema:        schema,
					TraceFieldKey:      "Output",
					TraceFieldJsonpath: "$.output",
				},
			},
		},
		SourceInfo: []*kitTask.SourceInfo{
			{Name: gptr.Of("my-service"), Version: gptr.Of("1.0.0")},
		},
		IsWorkflowScheduled: gptr.Of(true),
	}

	cfg := TaskConfigDTO2DO(dto)
	require.NotNil(t, cfg)

	ec := cfg.EvaluationExperimentConfig
	if assert.NotNil(t, ec) {
		assert.Equal(t, int32(5), ptr.From(ec.ItemConcurrencyCount))
		assert.Equal(t, int32(3), ptr.From(ec.ItemMaxRetryCount))
		assert.Equal(t, int64(888), ptr.From(ec.ExptTemplateID))
		assert.Equal(t, "target-1", ptr.From(ec.SourceTargetID))
		assert.Equal(t, "v2", ptr.From(ec.SourceTargetVersion))
		if assert.Len(t, ec.FullEvalSetFieldMappings, 2) {
			assert.Equal(t, "output_field", ptr.From(ec.FullEvalSetFieldMappings[0].EvalSetName))
			assert.Equal(t, "output", ptr.From(ec.FullEvalSetFieldMappings[0].DatasetKey))
			assert.Equal(t, "output", ptr.From(ec.FullEvalSetFieldMappings[1].DatasetKey))
		}
	}

	if assert.Len(t, cfg.SourceInfo, 1) {
		assert.Equal(t, "my-service", ptr.From(cfg.SourceInfo[0].Name))
		assert.Equal(t, "1.0.0", ptr.From(cfg.SourceInfo[0].Version))
	}

	assert.Equal(t, true, ptr.From(cfg.IsWorkflowScheduled))

	if assert.Len(t, cfg.AutoEvaluateConfigs, 1) {
		fm := cfg.AutoEvaluateConfigs[0].FieldMappings
		if assert.Len(t, fm, 1) {
			assert.Equal(t, "input_field", ptr.From(fm[0].EvalSetName))
		}
	}

	assert.Nil(t, TaskConfigDTO2DO(nil))
}
