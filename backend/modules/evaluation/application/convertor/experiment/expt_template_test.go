// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	common_eval "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	common_obs "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	evaluatorpkg "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	filterdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ConvertCreateExptTemplateReq 须把请求里的 expt_source 合并进 TemplateConf，否则落库 template_conf 会丢失 expt_source。
func TestConvertCreateExptTemplateReq_ExptSourceInTemplateConf(t *testing.T) {
	t.Parallel()

	req := &expt.CreateExperimentTemplateRequest{
		WorkspaceID: 1,
		Meta: &domain_expt.ExptTemplateMeta{
			Name:     gptr.Of("n"),
			ExptType: gptr.Of(domain_expt.ExptType_Offline),
		},
		TripleConfig: &domain_expt.ExptTuple{
			EvalSetID:        gptr.Of(int64(1)),
			EvalSetVersionID: gptr.Of(int64(1)),
			TargetID:         gptr.Of(int64(2)),
			TargetVersionID:  gptr.Of(int64(2)),
		},
		ExptSource: &domain_expt.ExptSource{
			SourceType: gptr.Of(domain_expt.SourceType_Evaluation),
			SourceID:   gptr.Of("pipe-9"),
		},
	}

	param, err := ConvertCreateExptTemplateReq(req)
	if assert.NoError(t, err) && assert.NotNil(t, param.TemplateConf) && assert.NotNil(t, param.TemplateConf.ExptSource) {
		assert.Equal(t, entity.SourceType_Evaluation, param.TemplateConf.ExptSource.SourceType)
		assert.Equal(t, "pipe-9", param.TemplateConf.ExptSource.SourceID)
	}
	if assert.NotNil(t, param.ExptSource) {
		assert.Equal(t, entity.SourceType_Evaluation, param.ExptSource.SourceType)
		assert.Equal(t, "pipe-9", param.ExptSource.SourceID)
	}
}

func TestBuildTemplateConfForCreate_WithExptSourceOnly(t *testing.T) {
	t.Parallel()

	req := &expt.CreateExperimentTemplateRequest{}
	param := &entity.CreateExptTemplateParam{
		ExptSource: &entity.ExptSource{SourceType: entity.SourceType_Evaluation, SourceID: "src-1"},
	}

	conf := buildTemplateConfForCreate(param, req, nil, nil, nil)
	if assert.NotNil(t, conf) {
		assert.Nil(t, conf.ConnectorConf.TargetConf)
		assert.Nil(t, conf.ConnectorConf.EvaluatorsConf)
		if assert.NotNil(t, conf.ExptSource) {
			assert.Equal(t, entity.SourceType_Evaluation, conf.ExptSource.SourceType)
			assert.Equal(t, "src-1", conf.ExptSource.SourceID)
		}
		assert.Nil(t, conf.ItemRetryNum)
	}
}

func TestBuildEvaluatorIDVersionItemsDTO_ScoreWeightAndRunConfigFallback(t *testing.T) {
	t.Parallel()

	env := "prod"
	template := &entity.ExptTemplate{
		TripleConfig: &entity.ExptTemplateTuple{
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{
				EvaluatorID:        1,
				Version:            "v1",
				EvaluatorVersionID: 101,
			}},
		},
		TemplateConf: &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{
					EvaluatorID:        1,
					Version:            "v1",
					EvaluatorVersionID: 101,
					ScoreWeight:        gptr.Of(0.7),
					RunConf: &entity.EvaluatorRunConfig{
						Env:                   gptr.Of(env),
						EvaluatorRuntimeParam: &entity.RuntimeParam{JSONValue: gptr.Of(`{"k":1}`)},
					},
				}}},
			},
		},
	}

	items := buildEvaluatorIDVersionItemsDTO(template)
	if assert.Len(t, items, 1) {
		item := items[0]
		assert.Equal(t, int64(1), item.GetEvaluatorID())
		assert.Equal(t, "v1", item.GetVersion())
		assert.Equal(t, int64(101), item.GetEvaluatorVersionID())
		assert.Equal(t, 0.7, item.GetScoreWeight())
		if assert.NotNil(t, item.GetRunConfig()) {
			assert.Equal(t, env, item.GetRunConfig().GetEnv())
			if assert.NotNil(t, item.GetRunConfig().GetEvaluatorRuntimeParam()) {
				assert.Equal(t, `{"k":1}`, item.GetRunConfig().GetEvaluatorRuntimeParam().GetJSONValue())
			}
		}
	}
}

func TestBuildEvaluatorIDVersionItemsDTO_EntityScoreWeightWins(t *testing.T) {
	t.Parallel()

	template := &entity.ExptTemplate{
		TripleConfig: &entity.ExptTemplateTuple{
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{
				EvaluatorID:        1,
				Version:            "v1",
				EvaluatorVersionID: 101,
				ScoreWeight:        0.9,
			}},
		},
		TemplateConf: &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{
					EvaluatorVersionID: 101,
					ScoreWeight:        gptr.Of(0.4),
				}}},
			},
		},
	}

	items := buildEvaluatorIDVersionItemsDTO(template)
	if assert.Len(t, items, 1) {
		assert.Equal(t, 0.9, items[0].GetScoreWeight())
	}
}

func TestBuildTemplateFieldMappingDTO_ItemRetryAndRunConfig(t *testing.T) {
	t.Parallel()

	template := &entity.ExptTemplate{
		FieldMappingConfig: &entity.ExptFieldMapping{
			ItemConcurNum:      gptr.Of(3),
			TargetRuntimeParam: &entity.RuntimeParam{JSONValue: gptr.Of(`{"debug":true}`)},
			EvaluatorFieldMapping: []*entity.EvaluatorFieldMapping{{
				EvaluatorID:        1,
				Version:            "v1",
				EvaluatorVersionID: 101,
				FromEvalSet:        []*entity.ExptTemplateFieldMapping{{FieldName: "input", FromFieldName: "question"}},
				FromTarget:         []*entity.ExptTemplateFieldMapping{{FieldName: "output", FromFieldName: "answer"}},
			}},
		},
		TemplateConf: &entity.ExptTemplateConfiguration{
			EvaluatorsConcurNum: gptr.Of(5),
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{
					EvaluatorVersionID: 101,
					RunConf: &entity.EvaluatorRunConfig{
						Env:                   gptr.Of("staging"),
						EvaluatorRuntimeParam: &entity.RuntimeParam{JSONValue: gptr.Of(`{"r":1}`)},
					},
				}}},
			},
		},
	}

	dto := buildTemplateFieldMappingDTO(template)
	if assert.NotNil(t, dto) {
		assert.Equal(t, int32(3), dto.GetItemConcurNum())
		assert.Equal(t, int32(0), dto.GetItemRetryNum())
		if assert.NotNil(t, dto.GetTargetRuntimeParam()) {
			assert.Equal(t, `{"debug":true}`, dto.GetTargetRuntimeParam().GetJSONValue())
		}
		if assert.Len(t, dto.GetEvaluatorFieldMapping(), 1) {
			item := dto.GetEvaluatorFieldMapping()[0].GetEvaluatorIDVersionItem()
			if assert.NotNil(t, item) {
				assert.Equal(t, int64(1), item.GetEvaluatorID())
				assert.Equal(t, "v1", item.GetVersion())
				assert.Equal(t, int64(101), item.GetEvaluatorVersionID())
				if assert.NotNil(t, item.GetRunConfig()) {
					assert.Equal(t, "staging", item.GetRunConfig().GetEnv())
				}
			}
		}
	}
}

func TestBuildTemplateScoreWeightConfigDTO_EnableFlagOnly(t *testing.T) {
	t.Parallel()

	template := &entity.ExptTemplate{
		TemplateConf: &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EnableScoreWeight: true},
			},
		},
	}

	cfg := buildTemplateScoreWeightConfigDTO(template)
	if assert.NotNil(t, cfg) {
		assert.True(t, cfg.GetEnableWeightedScore())
		assert.Nil(t, cfg.EvaluatorScoreWeights)
	}
}

func TestTemplateToSubmitExperimentRequest_TargetAndWeightedScore(t *testing.T) {
	t.Parallel()

	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{ID: 10, ExptType: entity.ExptType_Online},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:               1,
			EvalSetVersionID:        2,
			TargetID:                0,
			TargetVersionID:         0,
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 101, ScoreWeight: 0.6}, nil},
		},
		TemplateConf: &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EnableScoreWeight: true},
			},
		},
	}

	req := TemplateToSubmitExperimentRequest(template, "submit", 100)
	if assert.NotNil(t, req) {
		assert.Equal(t, int64(100), req.WorkspaceID)
		assert.Equal(t, "submit", req.GetName())
		assert.Equal(t, int64(10), req.GetExptTemplateID())
		assert.Equal(t, int64(1), req.GetEvalSetID())
		assert.Equal(t, int64(2), req.GetEvalSetVersionID())
		assert.Nil(t, req.TargetID)
		assert.Nil(t, req.TargetVersionID)
		assert.Equal(t, []int64{101}, req.EvaluatorVersionIds)
		assert.True(t, req.GetEnableWeightedScore())
	}
}

func TestToExptTemplateDTO_LatestTimeCronAndSource(t *testing.T) {
	t.Parallel()

	queryAndOr := "and"
	platformType := "app"
	spanListType := "selected"
	frequency := "daily"
	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{ID: 1, WorkspaceID: 2, Name: "tpl", Desc: "desc", ExptType: entity.ExptType_Online},
		TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 3, EvalSetVersionID: 4, TargetID: 5, TargetVersionID: 6},
		EvalSet:      &entity.EvaluationSet{ID: 100},
		ExptInfo: &entity.ExptInfo{
			CreatedExptCount:    7,
			LatestExptID:        8,
			LatestExptStatus:    entity.ExptStatus_Success,
			LatestExptStartTime: 0,
			CronActivate:        true,
		},
		ExptSource: &entity.ExptSource{
			SourceType: entity.SourceType_Evaluation,
			SourceID:   "source-1",
			SpanFilterFields: &entity.SpanFilterFieldsDO{
				PlatformType: &platformType,
				SpanListType: &spanListType,
				Filters: &entity.FilterFieldsDO{
					QueryAndOr: &queryAndOr,
				},
			},
			Scheduler: &entity.ExptSchedulerDO{
				Enabled:   gptr.Of(true),
				Frequency: &frequency,
				TriggerAt: gptr.Of(int64(123)),
			},
		},
	}

	dto := ToExptTemplateDTO(template)
	if assert.NotNil(t, dto) {
		assert.NotNil(t, dto.GetTripleConfig())
		assert.Nil(t, dto.GetTripleConfig().EvalSet)
		if assert.NotNil(t, dto.GetExptInfo()) {
			assert.True(t, dto.GetExptInfo().GetCronActivate())
			assert.Nil(t, dto.GetExptInfo().LatestExptStartTime)
		}
		if assert.NotNil(t, dto.GetExptSource()) {
			assert.Equal(t, domain_expt.SourceType(entity.SourceType_Evaluation), dto.GetExptSource().GetSourceType())
			assert.Equal(t, "source-1", dto.GetExptSource().GetSourceID())
			assert.NotNil(t, dto.GetExptSource().GetScheduler())
			assert.NotNil(t, dto.GetExptSource().GetSpanFilterFields())
		}
	}

	template.Meta.ExptType = entity.ExptType_Offline
	template.ExptInfo.LatestExptStartTime = 999
	dto = ToExptTemplateDTO(template)
	if assert.NotNil(t, dto) && assert.NotNil(t, dto.GetTripleConfig()) {
		assert.NotNil(t, dto.GetTripleConfig().EvalSet)
		if assert.NotNil(t, dto.GetExptInfo()) && assert.NotNil(t, dto.GetExptInfo().LatestExptStartTime) {
			assert.Equal(t, int64(999), dto.GetExptInfo().GetLatestExptStartTime())
		}
	}
}

func TestConvertUpdateExptTemplateReq_CronActivatePointer(t *testing.T) {
	t.Parallel()

	req := &expt.UpdateExperimentTemplateRequest{
		TemplateID:  1,
		WorkspaceID: 2,
		ExptInfo:    &domain_expt.ExptInfo{CronActivate: gptr.Of(false)},
	}

	param, err := ConvertUpdateExptTemplateReq(req)
	assert.NoError(t, err)
	if assert.NotNil(t, param) && assert.NotNil(t, param.CronActivate) {
		assert.False(t, *param.CronActivate)
	}

	param, err = ConvertUpdateExptTemplateReq(&expt.UpdateExperimentTemplateRequest{TemplateID: 1, WorkspaceID: 2})
	assert.NoError(t, err)
	assert.Nil(t, param.CronActivate)
}

func TestFillCreateTemplateMeta_CronActivate(t *testing.T) {
	t.Parallel()

	param := &entity.CreateExptTemplateParam{}
	fillCreateTemplateMeta(param, &expt.CreateExperimentTemplateRequest{
		Meta:     &domain_expt.ExptTemplateMeta{Name: gptr.Of("tpl"), ExptType: gptr.Of(domain_expt.ExptType_Offline)},
		ExptInfo: &domain_expt.ExptInfo{CronActivate: gptr.Of(true)},
	})
	assert.True(t, param.CronActivate)

	param = &entity.CreateExptTemplateParam{}
	fillCreateTemplateMeta(param, &expt.CreateExperimentTemplateRequest{
		Meta: &domain_expt.ExptTemplateMeta{Name: gptr.Of("tpl")},
	})
	assert.False(t, param.CronActivate)
}

func TestAppendEvaluatorIDVersionItemsFallbacks(t *testing.T) {
	t.Parallel()

	runCfgBuilder := func(evalVerID int64) *evaluatorpkg.EvaluatorRunConfig {
		if evalVerID != 101 {
			return nil
		}
		return &evaluatorpkg.EvaluatorRunConfig{
			Env: gptr.Of("prod"),
			EvaluatorRuntimeParam: &common_eval.RuntimeParam{JSONValue: gptr.Of(`{"x":1}`)},
		}
	}

	t.Run("evaluators fallback to template conf score", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Evaluators: []*entity.Evaluator{{
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{EvaluatorID: 1, ID: 101, Version: "v1"},
			}},
			TemplateConf: &entity.ExptTemplateConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 101, ScoreWeight: gptr.Of(0.5)}}},
				},
			},
		}
		var dst []*evaluatorpkg.EvaluatorIDVersionItem
		appendEvaluatorIDVersionItemsFromEvaluators(template, &dst, runCfgBuilder)
		if assert.Len(t, dst, 1) {
			assert.Equal(t, 0.5, dst[0].GetScoreWeight())
			assert.NotNil(t, dst[0].GetRunConfig())
		}
	})

	t.Run("version ref fallback to template conf score", func(t *testing.T) {
		template := &entity.ExptTemplate{
			EvaluatorVersionRef: []*entity.ExptTemplateEvaluatorVersionRef{{EvaluatorID: 1, EvaluatorVersionID: 101}},
			TemplateConf: &entity.ExptTemplateConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 101, ScoreWeight: gptr.Of(0.3)}}},
				},
			},
		}
		var dst []*evaluatorpkg.EvaluatorIDVersionItem
		appendEvaluatorIDVersionItemsFromVersionRef(template, &dst, runCfgBuilder)
		if assert.Len(t, dst, 1) {
			assert.Equal(t, 0.3, dst[0].GetScoreWeight())
			assert.NotNil(t, dst[0].GetRunConfig())
		}
	})
}

func TestFilterFieldDO2DTO_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.Nil(t, filterFieldDO2DTO(nil))

	t.Run("full nested filter field is converted", func(t *testing.T) {
		fieldType := "string"
		queryType := "eq"
		queryAndOr := "and"
		subQueryAndOr := "or"
		dto := filterFieldDO2DTO(&entity.FilterFieldDO{
			FieldName:  gptr.Of("trace_id"),
			FieldType:  &fieldType,
			Values:     []string{"v1", "v2"},
			QueryType:  &queryType,
			QueryAndOr: &queryAndOr,
			SubFilter: &entity.FilterFieldsDO{
				QueryAndOr: &subQueryAndOr,
				FilterFields: []*entity.FilterFieldDO{{
					FieldName: gptr.Of("span_id"),
					Values:    []string{"child"},
				}},
			},
		})
		if assert.NotNil(t, dto) {
			assert.Equal(t, "trace_id", dto.GetFieldName())
			assert.Equal(t, []string{"v1", "v2"}, dto.GetValues())
			if assert.NotNil(t, dto.FieldType) {
				assert.Equal(t, filterdto.FieldType(fieldType), dto.GetFieldType())
			}
			if assert.NotNil(t, dto.QueryType) {
				assert.Equal(t, filterdto.QueryType(queryType), dto.GetQueryType())
			}
			if assert.NotNil(t, dto.QueryAndOr) {
				assert.Equal(t, filterdto.QueryRelation(queryAndOr), dto.GetQueryAndOr())
			}
			if assert.NotNil(t, dto.SubFilter) {
				if assert.NotNil(t, dto.SubFilter.QueryAndOr) {
					assert.Equal(t, filterdto.QueryRelation(subQueryAndOr), dto.SubFilter.GetQueryAndOr())
				}
				if assert.Len(t, dto.SubFilter.GetFilterFields(), 1) {
					assert.Equal(t, "span_id", dto.SubFilter.GetFilterFields()[0].GetFieldName())
				}
			}
		}
	})

	t.Run("optional fields can stay nil", func(t *testing.T) {
		dto := filterFieldDO2DTO(&entity.FilterFieldDO{FieldName: gptr.Of("status"), Values: []string{"ok"}})
		if assert.NotNil(t, dto) {
			assert.Equal(t, "status", dto.GetFieldName())
			assert.Equal(t, []string{"ok"}, dto.GetValues())
			assert.Nil(t, dto.FieldType)
			assert.Nil(t, dto.QueryType)
			assert.Nil(t, dto.QueryAndOr)
			assert.Nil(t, dto.SubFilter)
		}
	})
}

func TestFilterFieldsDO2DTO_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.Nil(t, filterFieldsDO2DTO(nil))

	t.Run("nil children are filtered out", func(t *testing.T) {
		dto := filterFieldsDO2DTO(&entity.FilterFieldsDO{
			FilterFields: []*entity.FilterFieldDO{nil, {FieldName: gptr.Of("trace_id"), Values: []string{"1"}}},
		})
		if assert.NotNil(t, dto) {
			assert.Nil(t, dto.QueryAndOr)
			if assert.Len(t, dto.GetFilterFields(), 1) {
				assert.Equal(t, "trace_id", dto.GetFilterFields()[0].GetFieldName())
			}
		}
	})

	t.Run("empty filter fields still returns dto", func(t *testing.T) {
		dto := filterFieldsDO2DTO(&entity.FilterFieldsDO{})
		if assert.NotNil(t, dto) {
			assert.Nil(t, dto.QueryAndOr)
			assert.Nil(t, dto.FilterFields)
		}
	})
}

func TestSpanFilterFieldsDO2DTO_PartialFields(t *testing.T) {
	t.Parallel()

	assert.Nil(t, spanFilterFieldsDO2DTO(nil))

	platformType := "app"
	spanListType := "selected"
	dto := spanFilterFieldsDO2DTO(&entity.SpanFilterFieldsDO{
		PlatformType: &platformType,
		SpanListType: &spanListType,
		Filters: &entity.FilterFieldsDO{
			FilterFields: []*entity.FilterFieldDO{{FieldName: gptr.Of("trace_id"), Values: []string{"1"}}},
		},
	})
	if assert.NotNil(t, dto) {
		if assert.NotNil(t, dto.PlatformType) {
			assert.Equal(t, common_obs.PlatformType(platformType), dto.GetPlatformType())
		}
		if assert.NotNil(t, dto.SpanListType) {
			assert.Equal(t, common_obs.SpanListType(spanListType), dto.GetSpanListType())
		}
		if assert.NotNil(t, dto.Filters) {
			assert.Len(t, dto.Filters.GetFilterFields(), 1)
		}
	}
}

func TestToExptTemplateDTOs_PreserveNilEntries(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ToExptTemplateDTOs(nil))
	assert.Nil(t, ToExptTemplateDTOs([]*entity.ExptTemplate{}))

	dtos := ToExptTemplateDTOs([]*entity.ExptTemplate{nil, {
		Meta: &entity.ExptTemplateMeta{ID: 10, WorkspaceID: 20, Name: "tpl"},
	}})
	if assert.Len(t, dtos, 2) {
		assert.Nil(t, dtos[0])
		if assert.NotNil(t, dtos[1]) && assert.NotNil(t, dtos[1].Meta) {
			assert.Equal(t, int64(10), dtos[1].GetMeta().GetID())
			assert.Equal(t, int64(20), dtos[1].GetMeta().GetWorkspaceID())
		}
	}
}

func TestTemplateToSubmitExperimentRequest_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.Nil(t, TemplateToSubmitExperimentRequest(nil, "submit", 100))

	t.Run("nil triple config keeps base fields", func(t *testing.T) {
		req := TemplateToSubmitExperimentRequest(&entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: 10},
		}, "submit", 100)
		if assert.NotNil(t, req) {
			assert.Equal(t, int64(100), req.WorkspaceID)
			assert.Equal(t, "submit", req.GetName())
			assert.Equal(t, int64(10), req.GetExptTemplateID())
			assert.Nil(t, req.EvalSetID)
			assert.Nil(t, req.EnableWeightedScore)
		}
	})

	t.Run("target field mapping and concur config are propagated", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: 11, ExptType: entity.ExptType_Offline},
			TripleConfig: &entity.ExptTemplateTuple{
				EvalSetID:               1,
				EvalSetVersionID:        2,
				TargetID:                3,
				TargetVersionID:         4,
				EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{EvaluatorID: 7, Version: "v7", EvaluatorVersionID: 17, ScoreWeight: 0.2}, nil, {EvaluatorVersionID: 0}},
			},
			FieldMappingConfig: &entity.ExptFieldMapping{
				ItemConcurNum:      gptr.Of(5),
				TargetRuntimeParam: &entity.RuntimeParam{JSONValue: gptr.Of(`{"mode":"prod"}`)},
				TargetFieldMapping: &entity.TargetFieldMapping{FromEvalSet: []*entity.ExptTemplateFieldMapping{{FieldName: "input", FromFieldName: "question"}}},
				EvaluatorFieldMapping: []*entity.EvaluatorFieldMapping{{
					EvaluatorID:        7,
					Version:            "v7",
					EvaluatorVersionID: 17,
					FromTarget:         []*entity.ExptTemplateFieldMapping{{FieldName: "answer", FromFieldName: "output"}},
				}},
			},
			TemplateConf: &entity.ExptTemplateConfiguration{
				EvaluatorsConcurNum: gptr.Of(6),
			},
		}

		goReq := TemplateToSubmitExperimentRequest(template, "submit-full", 200)
		if assert.NotNil(t, goReq) {
			assert.Equal(t, int64(200), goReq.WorkspaceID)
			assert.Equal(t, int64(3), goReq.GetTargetID())
			assert.Equal(t, int64(4), goReq.GetTargetVersionID())
			assert.Equal(t, []int64{17}, goReq.EvaluatorVersionIds)
			assert.Equal(t, int32(6), goReq.GetEvaluatorsConcurNum())
			assert.Equal(t, domain_expt.ExptType_Offline, goReq.GetExptType())
			assert.True(t, goReq.GetEnableWeightedScore())
			if assert.NotNil(t, goReq.TargetFieldMapping) {
				assert.Len(t, goReq.TargetFieldMapping.FromEvalSet, 1)
			}
			if assert.NotNil(t, goReq.TargetRuntimeParam) {
				assert.Equal(t, `{"mode":"prod"}`, goReq.TargetRuntimeParam.GetJSONValue())
			}
			assert.Equal(t, int32(5), goReq.GetItemConcurNum())
		}
	})
}

func TestBuildScoreWeightsFromTemplateConf_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.Nil(t, buildScoreWeightsFromTemplateConf(&entity.ExptTemplate{}))

	weights := buildScoreWeightsFromTemplateConf(&entity.ExptTemplate{
		TemplateConf: &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{
					nil,
					{EvaluatorVersionID: 1},
					{EvaluatorVersionID: 2, ScoreWeight: gptr.Of(0.0)},
					{EvaluatorVersionID: 3, ScoreWeight: gptr.Of(-0.1)},
					{EvaluatorVersionID: 4, ScoreWeight: gptr.Of(0.4)},
					{EvaluatorVersionID: 4, ScoreWeight: gptr.Of(0.6)},
				}},
			},
		},
	})
	if assert.Len(t, weights, 1) {
		assert.Equal(t, 0.6, weights[4])
	}
}

func TestGetScoreWeightFromTemplateConf_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.Zero(t, getScoreWeightFromTemplateConf(nil, 1))
	assert.Zero(t, getScoreWeightFromTemplateConf(&entity.ExptTemplate{}, 1))

	template := &entity.ExptTemplate{
		TemplateConf: &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{
					nil,
					{EvaluatorVersionID: 1, ScoreWeight: gptr.Of(0.0)},
					{EvaluatorVersionID: 2, ScoreWeight: gptr.Of(0.5)},
				}},
			},
		},
	}
	assert.Zero(t, getScoreWeightFromTemplateConf(template, 1))
	assert.Equal(t, 0.5, getScoreWeightFromTemplateConf(template, 2))
	assert.Zero(t, getScoreWeightFromTemplateConf(template, 3))
}

func TestBuildTemplateScoreWeightConfigDTO_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.Nil(t, buildTemplateScoreWeightConfigDTO(&entity.ExptTemplate{}))

	t.Run("template conf weights win over triple config fallback", func(t *testing.T) {
		cfg := buildTemplateScoreWeightConfigDTO(&entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{EvaluatorVersionID: 10, ScoreWeight: 0.3}}},
			TemplateConf: &entity.ExptTemplateConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 10, ScoreWeight: gptr.Of(0.7)}}},
				},
			},
		})
		if assert.NotNil(t, cfg) {
			assert.True(t, cfg.GetEnableWeightedScore())
			assert.Equal(t, 0.7, cfg.EvaluatorScoreWeights[10])
		}
	})

	t.Run("triple config fallback is used when template conf has no valid weights", func(t *testing.T) {
		cfg := buildTemplateScoreWeightConfigDTO(&entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{EvaluatorVersionID: 20, ScoreWeight: 0.5}, nil, {EvaluatorVersionID: 0, ScoreWeight: 0.9}}},
			TemplateConf: &entity.ExptTemplateConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 20, ScoreWeight: gptr.Of(0.0)}}},
				},
			},
		})
		if assert.NotNil(t, cfg) {
			assert.True(t, cfg.GetEnableWeightedScore())
			assert.Equal(t, 0.5, cfg.EvaluatorScoreWeights[20])
		}
	})
}

func TestToExptTemplateDTO_MetaAndSchedulerEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil meta keeps dto meta nil", func(t *testing.T) {
		dto := ToExptTemplateDTO(&entity.ExptTemplate{})
		assert.NotNil(t, dto)
		assert.Nil(t, dto.Meta)
	})

	t.Run("scheduler keeps timing fields when frequency is nil", func(t *testing.T) {
		template := &entity.ExptTemplate{
			Meta: &entity.ExptTemplateMeta{ID: 1},
			ExptSource: &entity.ExptSource{
				SourceType: entity.SourceType_Evaluation,
				SourceID:   "source-1",
				Scheduler: &entity.ExptSchedulerDO{
					Enabled:   gptr.Of(true),
					TriggerAt: gptr.Of(int64(123)),
					StartTime: gptr.Of(int64(456)),
					EndTime:   gptr.Of(int64(789)),
				},
			},
		}
		dto := ToExptTemplateDTO(template)
		if assert.NotNil(t, dto) && assert.NotNil(t, dto.GetExptSource()) && assert.NotNil(t, dto.GetExptSource().GetScheduler()) {
			scheduler := dto.GetExptSource().GetScheduler()
			assert.True(t, scheduler.GetEnabled())
			assert.Equal(t, int64(123), scheduler.GetTriggerAt())
			assert.Equal(t, int64(456), scheduler.GetStartTime())
			assert.Equal(t, int64(789), scheduler.GetEndTime())
			assert.Nil(t, scheduler.Frequency)
		}
	})
}

func TestConvertTemplateConfToDTO_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("target and evaluator mappings are converted with runtime param", func(t *testing.T) {
		env := "prod"
		weight := 0.6
		conf := &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				TargetConf: &entity.TargetConf{IngressConf: &entity.TargetIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{
						FieldName: "input",
						FromField: "question",
					}, {
						FieldName: "const_field",
						Value:     "constant",
					}}},
					CustomConf: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{
						FieldName: "ignored",
						Value:     "x",
					}, {
						FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
						Value:     `{"debug":true}`,
					}}},
				}},
				EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{
					EvaluatorID:        0,
					EvaluatorVersionID: 0,
					Version:            "",
				}, {
					EvaluatorID:        7,
					EvaluatorVersionID: 8,
					Version:            "v8",
					ScoreWeight:        &weight,
					RunConf: &entity.EvaluatorRunConfig{
						Env:                   gptr.Of(env),
						EvaluatorRuntimeParam: &entity.RuntimeParam{JSONValue: gptr.Of(`{"k":1}`)},
					},
					IngressConf: &entity.EvaluatorIngressConf{
						EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{
							FieldName: "eval_field",
							FromField: "dataset_field",
						}}},
						TargetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{
							FieldName: "target_field",
							Value:     "const-target",
						}}},
					},
				}, {
					EvaluatorVersionID: 9,
					IngressConf: &entity.EvaluatorIngressConf{
						EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{
							FieldName: "eval_only",
							FromField: "source_only",
						}}},
					},
				}}},
			},
		}

		targetMapping, evaluatorMappings, runtimeParam := convertTemplateConfToDTO(conf)
		if assert.NotNil(t, targetMapping) && assert.Len(t, targetMapping.FromEvalSet, 2) {
			assert.Equal(t, "input", targetMapping.FromEvalSet[0].GetFieldName())
			assert.Equal(t, "question", targetMapping.FromEvalSet[0].GetFromFieldName())
			assert.Equal(t, "const_field", targetMapping.FromEvalSet[1].GetFieldName())
			assert.Equal(t, "constant", targetMapping.FromEvalSet[1].GetConstValue())
		}
		if assert.NotNil(t, runtimeParam) {
			assert.Equal(t, `{"debug":true}`, runtimeParam.GetJSONValue())
		}
		if assert.Len(t, evaluatorMappings, 2) {
			first := evaluatorMappings[0]
			if assert.NotNil(t, first.GetEvaluatorIDVersionItem()) {
				assert.Equal(t, int64(7), first.GetEvaluatorIDVersionItem().GetEvaluatorID())
				assert.Equal(t, "v8", first.GetEvaluatorIDVersionItem().GetVersion())
				assert.Equal(t, int64(8), first.GetEvaluatorIDVersionItem().GetEvaluatorVersionID())
				assert.Equal(t, weight, first.GetEvaluatorIDVersionItem().GetScoreWeight())
				if assert.NotNil(t, first.GetEvaluatorIDVersionItem().GetRunConfig()) {
					assert.Equal(t, env, first.GetEvaluatorIDVersionItem().GetRunConfig().GetEnv())
					assert.Equal(t, `{"k":1}`, first.GetEvaluatorIDVersionItem().GetRunConfig().GetEvaluatorRuntimeParam().GetJSONValue())
				}
			}
			if assert.Len(t, first.FromEvalSet, 1) {
				assert.Equal(t, "eval_field", first.FromEvalSet[0].GetFieldName())
				assert.Equal(t, "dataset_field", first.FromEvalSet[0].GetFromFieldName())
			}
			if assert.Len(t, first.FromTarget, 1) {
				assert.Equal(t, "target_field", first.FromTarget[0].GetFieldName())
				assert.Equal(t, "const-target", first.FromTarget[0].GetConstValue())
			}
			second := evaluatorMappings[1]
			if assert.NotNil(t, second.GetEvaluatorIDVersionItem()) {
				assert.Equal(t, int64(9), second.GetEvaluatorIDVersionItem().GetEvaluatorVersionID())
				assert.Zero(t, second.GetEvaluatorIDVersionItem().GetEvaluatorID())
				assert.Empty(t, second.GetEvaluatorIDVersionItem().GetVersion())
			}
			if assert.Len(t, second.FromEvalSet, 1) {
				assert.Equal(t, "eval_only", second.FromEvalSet[0].GetFieldName())
			}
			assert.Nil(t, second.FromTarget)
		}
	})

	t.Run("missing target and evaluator ingress keeps outputs nil or empty", func(t *testing.T) {
		conf := &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{
					EvaluatorID:        1,
					EvaluatorVersionID: 2,
					Version:            "v2",
				}}},
			},
		}

		targetMapping, evaluatorMappings, runtimeParam := convertTemplateConfToDTO(conf)
		assert.Nil(t, targetMapping)
		assert.Nil(t, runtimeParam)
		assert.Nil(t, evaluatorMappings)
	})
}
