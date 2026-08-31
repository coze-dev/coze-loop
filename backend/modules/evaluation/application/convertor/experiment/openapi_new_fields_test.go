// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domainCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domainEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	domain_filter "github.com/coze-dev/coze-loop/backend/kitex_gen/stone/fornax/ml_flow/domain/filter"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestDomainEvaluatorIDVersionListDTO2OpenAPI(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, DomainEvaluatorIDVersionListDTO2OpenAPI(nil))
	})

	t.Run("empty list returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, DomainEvaluatorIDVersionListDTO2OpenAPI([]*domainEvaluator.EvaluatorIDVersionItem{}))
	})

	t.Run("nil entries are skipped", func(t *testing.T) {
		t.Parallel()
		items := []*domainEvaluator.EvaluatorIDVersionItem{
			nil,
			{EvaluatorID: gptr.Of(int64(7)), Version: gptr.Of("v1")},
			nil,
		}
		got := DomainEvaluatorIDVersionListDTO2OpenAPI(items)
		if assert.Len(t, got, 1) {
			assert.Equal(t, int64(7), got[0].GetEvaluatorID())
			assert.Equal(t, "v1", got[0].GetVersion())
		}
	})

	t.Run("with run config and score weight", func(t *testing.T) {
		t.Parallel()
		jsonValue := "{\"k\":\"v\"}"
		env := "production"
		weight := 0.5
		items := []*domainEvaluator.EvaluatorIDVersionItem{
			{
				EvaluatorID:        gptr.Of(int64(1)),
				Version:            gptr.Of("v1"),
				EvaluatorVersionID: gptr.Of(int64(11)),
				ScoreWeight:        gptr.Of(weight),
				RunConfig: &domainEvaluator.EvaluatorRunConfig{
					Env:                   &env,
					EvaluatorRuntimeParam: &domainCommon.RuntimeParam{JSONValue: &jsonValue},
				},
			},
		}
		got := DomainEvaluatorIDVersionListDTO2OpenAPI(items)
		if assert.Len(t, got, 1) {
			assert.Equal(t, int64(1), got[0].GetEvaluatorID())
			assert.Equal(t, "v1", got[0].GetVersion())
			assert.Equal(t, int64(11), got[0].GetEvaluatorVersionID())
			assert.InDelta(t, weight, got[0].GetScoreWeight(), 1e-9)
			if assert.NotNil(t, got[0].RunConfig) {
				assert.Equal(t, env, got[0].RunConfig.GetEnv())
				if assert.NotNil(t, got[0].RunConfig.EvaluatorRuntimeParam) {
					assert.Equal(t, jsonValue, got[0].RunConfig.EvaluatorRuntimeParam.GetJSONValue())
				}
			}
		}
	})

	t.Run("entry without run config", func(t *testing.T) {
		t.Parallel()
		items := []*domainEvaluator.EvaluatorIDVersionItem{
			{
				EvaluatorID:        gptr.Of(int64(1)),
				Version:            gptr.Of("v1"),
				EvaluatorVersionID: gptr.Of(int64(11)),
			},
		}
		got := DomainEvaluatorIDVersionListDTO2OpenAPI(items)
		if assert.Len(t, got, 1) {
			assert.Nil(t, got[0].RunConfig)
		}
	})
}

func TestDomainExptTemplateMetaDTO2OpenAPI(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, DomainExptTemplateMetaDTO2OpenAPI(nil))
	})

	t.Run("Desc maps to Description; Visibility dropped", func(t *testing.T) {
		t.Parallel()
		exptType := domainExpt.ExptType_Offline
		from := &domainExpt.ExptTemplateMeta{
			ID:          gptr.Of(int64(1)),
			WorkspaceID: gptr.Of(int64(2)),
			Name:        gptr.Of("tpl"),
			Desc:        gptr.Of("hello"),
			ExptType:    &exptType,
		}
		got := DomainExptTemplateMetaDTO2OpenAPI(from)
		if assert.NotNil(t, got) {
			assert.Equal(t, int64(1), got.GetID())
			assert.Equal(t, int64(2), got.GetWorkspaceID())
			assert.Equal(t, "tpl", got.GetName())
			assert.Equal(t, "hello", got.GetDescription())
			assert.Equal(t, openapiExperiment.ExperimentTypeOffline, got.GetExptType())
		}
	})

	t.Run("nil expt type stays nil", func(t *testing.T) {
		t.Parallel()
		from := &domainExpt.ExptTemplateMeta{ID: gptr.Of(int64(42))}
		got := DomainExptTemplateMetaDTO2OpenAPI(from)
		if assert.NotNil(t, got) {
			assert.Nil(t, got.ExptType)
		}
	})
}

func TestMapDomainExptTypeToOpenAPI(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, mapDomainExptTypeToOpenAPI(nil))
	})

	t.Run("online", func(t *testing.T) {
		t.Parallel()
		v := domainExpt.ExptType_Online
		got := mapDomainExptTypeToOpenAPI(&v)
		if assert.NotNil(t, got) {
			assert.Equal(t, openapiExperiment.ExperimentTypeOnline, *got)
		}
	})

	t.Run("offline (default branch)", func(t *testing.T) {
		t.Parallel()
		v := domainExpt.ExptType_Offline
		got := mapDomainExptTypeToOpenAPI(&v)
		if assert.NotNil(t, got) {
			assert.Equal(t, openapiExperiment.ExperimentTypeOffline, *got)
		}
	})

	t.Run("unknown numeric falls back to offline (default branch)", func(t *testing.T) {
		t.Parallel()
		v := domainExpt.ExptType(99)
		got := mapDomainExptTypeToOpenAPI(&v)
		if assert.NotNil(t, got) {
			assert.Equal(t, openapiExperiment.ExperimentTypeOffline, *got)
		}
	})
}

// TestDomainExperimentDTO2OpenAPI_NewFields 验证新增字段（item_retry_num /
// evaluator_id_version_list / expt_template_meta）从 domain DTO 透传到 openapi。
func TestDomainExperimentDTO2OpenAPI_NewFields(t *testing.T) {
	t.Parallel()

	itemRetry := int32(2)
	exptType := domainExpt.ExptType_Offline
	weight := 0.7

	dto := &domainExpt.Experiment{
		ID:           gptr.Of(int64(1)),
		Name:         gptr.Of("x"),
		ItemRetryNum: &itemRetry,
		EvaluatorIDVersionList: []*domainEvaluator.EvaluatorIDVersionItem{
			{
				EvaluatorID:        gptr.Of(int64(5)),
				Version:            gptr.Of("v1"),
				EvaluatorVersionID: gptr.Of(int64(55)),
				ScoreWeight:        gptr.Of(weight),
			},
		},
		ExptTemplateMeta: &domainExpt.ExptTemplateMeta{
			ID:       gptr.Of(int64(100)),
			Name:     gptr.Of("tpl"),
			Desc:     gptr.Of("desc"),
			ExptType: &exptType,
		},
	}

	converted := DomainExperimentDTO2OpenAPI(dto)
	if assert.NotNil(t, converted) {
		assert.Equal(t, itemRetry, converted.GetItemRetryNum())
		if assert.Len(t, converted.EvaluatorIDVersionList, 1) {
			assert.Equal(t, int64(5), converted.EvaluatorIDVersionList[0].GetEvaluatorID())
			assert.InDelta(t, weight, converted.EvaluatorIDVersionList[0].GetScoreWeight(), 1e-9)
		}
		if assert.NotNil(t, converted.ExptTemplateMeta) {
			assert.Equal(t, int64(100), converted.ExptTemplateMeta.GetID())
			assert.Equal(t, "tpl", converted.ExptTemplateMeta.GetName())
			assert.Equal(t, "desc", converted.ExptTemplateMeta.GetDescription())
			assert.Equal(t, openapiExperiment.ExperimentTypeOffline, converted.ExptTemplateMeta.GetExptType())
		}
	}
}

func TestBuildOpenAPIEvaluatorIDVersionListFromExperiment(t *testing.T) {
	t.Parallel()

	t.Run("nil experiment returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, buildOpenAPIEvaluatorIDVersionListFromExperiment(nil))
	})

	t.Run("nil eval conf returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, buildOpenAPIEvaluatorIDVersionListFromExperiment(&entity.Experiment{}))
	})

	t.Run("nil EvaluatorsConf returns nil", func(t *testing.T) {
		t.Parallel()
		exp := &entity.Experiment{
			EvalConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{},
			},
		}
		assert.Nil(t, buildOpenAPIEvaluatorIDVersionListFromExperiment(exp))
	})

	t.Run("empty EvaluatorConf returns nil", func(t *testing.T) {
		t.Parallel()
		exp := &entity.Experiment{
			EvalConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{},
				},
			},
		}
		assert.Nil(t, buildOpenAPIEvaluatorIDVersionListFromExperiment(exp))
	})

	t.Run("nil entry is skipped", func(t *testing.T) {
		t.Parallel()
		exp := &entity.Experiment{
			EvalConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{
						EvaluatorConf: []*entity.EvaluatorConf{
							nil,
							{EvaluatorVersionID: 7, EvaluatorID: 70, Version: "v1"},
						},
					},
				},
			},
		}
		got := buildOpenAPIEvaluatorIDVersionListFromExperiment(exp)
		if assert.Len(t, got, 1) {
			assert.Equal(t, int64(70), got[0].GetEvaluatorID())
			assert.Equal(t, "v1", got[0].GetVersion())
			assert.Equal(t, int64(7), got[0].GetEvaluatorVersionID())
		}
	})

	t.Run("with run config and score weight", func(t *testing.T) {
		t.Parallel()
		env := "test"
		jsonValue := "{\"a\":1}"
		weight := 0.25
		exp := &entity.Experiment{
			EvalConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{
						EvaluatorConf: []*entity.EvaluatorConf{
							{
								EvaluatorVersionID: 7,
								EvaluatorID:        70,
								Version:            "v1",
								ScoreWeight:        &weight,
								RunConf: &entity.EvaluatorRunConfig{
									Env:                   &env,
									EvaluatorRuntimeParam: &entity.RuntimeParam{JSONValue: &jsonValue},
								},
							},
						},
					},
				},
			},
		}
		got := buildOpenAPIEvaluatorIDVersionListFromExperiment(exp)
		if assert.Len(t, got, 1) {
			assert.InDelta(t, weight, got[0].GetScoreWeight(), 1e-9)
			if assert.NotNil(t, got[0].RunConfig) {
				assert.Equal(t, env, got[0].RunConfig.GetEnv())
				if assert.NotNil(t, got[0].RunConfig.EvaluatorRuntimeParam) {
					assert.Equal(t, jsonValue, got[0].RunConfig.EvaluatorRuntimeParam.GetJSONValue())
				}
			}
		}
	})

	t.Run("without run config or score weight", func(t *testing.T) {
		t.Parallel()
		exp := &entity.Experiment{
			EvalConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{
						EvaluatorConf: []*entity.EvaluatorConf{
							{EvaluatorVersionID: 7, EvaluatorID: 70, Version: "v1"},
						},
					},
				},
			},
		}
		got := buildOpenAPIEvaluatorIDVersionListFromExperiment(exp)
		if assert.Len(t, got, 1) {
			assert.Nil(t, got[0].ScoreWeight)
			assert.Nil(t, got[0].RunConfig)
		}
	})
}

// TestOpenAPIExptDO2DTO_NewFields 验证 entity 路径下的 4 个新字段（item_retry_num /
// evaluator_id_version_list / expt_template_meta）正确填充；ScoreWeight 仍通过 list 透出。
func TestOpenAPIExptDO2DTO_NewFields(t *testing.T) {
	t.Parallel()

	start := time.Unix(100, 0)
	end := time.Unix(200, 0)
	weight := 0.7
	env := "prod"
	jsonValue := "{\"a\":1}"

	experiment := &entity.Experiment{
		ID:        10,
		Name:      "exp",
		CreatedBy: "creator",
		Status:    entity.ExptStatus_Success,
		StartAt:   &start,
		EndAt:     &end,
		ExptType:  entity.ExptType_Offline,
		EvalConf: &entity.EvaluationConfiguration{
			ItemConcurNum: gptr.Of(3),
			ItemRetryNum:  gptr.Of(5),
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{
					EnableScoreWeight: true,
					EvaluatorConf: []*entity.EvaluatorConf{
						{
							EvaluatorVersionID: 7,
							EvaluatorID:        70,
							Version:            "v1",
							ScoreWeight:        &weight,
							RunConf: &entity.EvaluatorRunConfig{
								Env:                   &env,
								EvaluatorRuntimeParam: &entity.RuntimeParam{JSONValue: &jsonValue},
							},
						},
					},
				},
			},
		},
		ExptTemplateMeta: &entity.ExptTemplateMeta{
			ID:          11,
			WorkspaceID: 22,
			Name:        "tpl",
			Desc:        "hello",
			ExptType:    entity.ExptType_Offline,
		},
	}

	converted := OpenAPIExptDO2DTO(experiment)
	if assert.NotNil(t, converted) {
		// item_retry_num
		assert.Equal(t, int32(5), converted.GetItemRetryNum())
		// evaluator_id_version_list — 含 score_weight + run_config
		if assert.Len(t, converted.EvaluatorIDVersionList, 1) {
			it := converted.EvaluatorIDVersionList[0]
			assert.Equal(t, int64(70), it.GetEvaluatorID())
			assert.Equal(t, "v1", it.GetVersion())
			assert.Equal(t, int64(7), it.GetEvaluatorVersionID())
			assert.InDelta(t, weight, it.GetScoreWeight(), 1e-9)
			if assert.NotNil(t, it.RunConfig) {
				assert.Equal(t, env, it.RunConfig.GetEnv())
			}
		}
		// expt_template_meta — Desc 映射到 Description
		if assert.NotNil(t, converted.ExptTemplateMeta) {
			assert.Equal(t, int64(11), converted.ExptTemplateMeta.GetID())
			assert.Equal(t, int64(22), converted.ExptTemplateMeta.GetWorkspaceID())
			assert.Equal(t, "tpl", converted.ExptTemplateMeta.GetName())
			assert.Equal(t, "hello", converted.ExptTemplateMeta.GetDescription())
			assert.Equal(t, openapiExperiment.ExperimentTypeOffline, converted.ExptTemplateMeta.GetExptType())
		}
	}
}

// TestOpenAPIExptDO2DTO_NoNewFields_WhenEntityEmpty 防止把新字段意外置为非 nil。
func TestOpenAPIExptDO2DTO_NoNewFields_WhenEntityEmpty(t *testing.T) {
	t.Parallel()

	experiment := &entity.Experiment{
		ID:     1,
		Status: entity.ExptStatus_Pending,
		EvalConf: &entity.EvaluationConfiguration{
			// no ItemRetryNum, no EvaluatorsConf
			ConnectorConf: entity.Connector{},
		},
	}
	converted := OpenAPIExptDO2DTO(experiment)
	if assert.NotNil(t, converted) {
		assert.Nil(t, converted.ItemRetryNum)
		assert.Nil(t, converted.EvaluatorIDVersionList)
		assert.Nil(t, converted.ExptTemplateMeta)
	}
}

// TestOpenAPIExptDO2DTO_ItemCentricFields 验证 entity 路径 (GetExperimentsOApi 单实验 Get 用) 回填
// item-centric 多评测集字段: eval_set_source_type(110) / eval_set_details(112) / evaluators_concur_num(113) / total_item_count(114)。
func TestOpenAPIExptDO2DTO_ItemCentricFields(t *testing.T) {
	t.Parallel()

	t.Run("MultiSet 全填", func(t *testing.T) {
		experiment := &entity.Experiment{
			ID:                1,
			Status:            entity.ExptStatus_Success,
			ExptType:          entity.ExptType_Offline,
			EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
			TotalItemCount:    42,
			EvalSetDetails: []*entity.ExptEvalSetDetail{
				{EvalSetID: 100, EvalSetVersionID: 1000, IsPrimary: true, ItemCount: 30},
				{EvalSetID: 200, EvalSetVersionID: 2000, IsPrimary: false, ItemCount: 12},
			},
			EvalConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: gptr.Of(5)},
				},
			},
		}
		converted := OpenAPIExptDO2DTO(experiment)
		if assert.NotNil(t, converted) {
			assert.Equal(t, openapiExperiment.ExptEvalSetSourceTypeMultiSetConfig, converted.GetEvalSetSourceType())
			assert.Equal(t, int64(42), converted.GetTotalItemCount())
			assert.Equal(t, int32(5), converted.GetEvaluatorsConcurNum())
			if assert.Len(t, converted.EvalSetDetails, 2) {
				assert.Equal(t, int64(100), converted.EvalSetDetails[0].GetEvalSetID())
				assert.Equal(t, int64(1000), converted.EvalSetDetails[0].GetEvalSetVersionID())
				assert.True(t, converted.EvalSetDetails[0].GetIsPrimary())
				assert.Equal(t, int32(30), converted.EvalSetDetails[0].GetItemCount())
				assert.Equal(t, int32(12), converted.EvalSetDetails[1].GetItemCount())
			}
		}
	})

	t.Run("SingleSet 旧实验 → 仅 single_set, 无 details/total_item_count", func(t *testing.T) {
		experiment := &entity.Experiment{
			ID:                2,
			Status:            entity.ExptStatus_Success,
			ExptType:          entity.ExptType_Offline,
			EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet,
		}
		converted := OpenAPIExptDO2DTO(experiment)
		if assert.NotNil(t, converted) {
			assert.Equal(t, openapiExperiment.ExptEvalSetSourceTypeSingleSet, converted.GetEvalSetSourceType())
			assert.Nil(t, converted.EvalSetDetails, "SingleSet 不应填 eval_set_details")
			assert.Nil(t, converted.TotalItemCount, "SingleSet 不应回显 total_item_count")
		}
	})
}

// TestOpenAPIExptDO2DTO_EvalSetConfigsEcho 验证 MultiSetConfig 实验经 OpenAPI 读路径 (GetExperimentsOApi)
// 回显 eval_set_configs(111): 含 item_filter + version_id→version 字符串反查 + evaluator_confs 字段映射;
// SingleSet 旧实验不回显。
func TestOpenAPIExptDO2DTO_EvalSetConfigsEcho(t *testing.T) {
	t.Parallel()

	t.Run("MultiSetConfig 回显 item_filter + version 字符串", func(t *testing.T) {
		experiment := &entity.Experiment{
			ID:                1,
			Status:            entity.ExptStatus_Success,
			ExptType:          entity.ExptType_Offline,
			EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
			TotalItemCount:    7,
			// evaluator version_id → version 字符串反查源
			Evaluators: []*entity.Evaluator{
				{EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 5001, Version: "0.0.1"}},
			},
			// eval_set version_id → version 字符串反查源
			EvalSetDetails: []*entity.ExptEvalSetDetail{
				{
					EvalSetID: 100, EvalSetVersionID: 1000, IsPrimary: true, ItemCount: 4,
					EvalSet: &entity.EvaluationSet{ID: 100, EvaluationSetVersion: &entity.EvaluationSetVersion{Version: "0.0.1"}},
				},
				{
					EvalSetID: 200, EvalSetVersionID: 2000, IsPrimary: false, ItemCount: 3,
					EvalSet: &entity.EvaluationSet{ID: 200, EvaluationSetVersion: &entity.EvaluationSetVersion{Version: "0.0.2"}},
				},
			},
			EvalConf: &entity.EvaluationConfiguration{
				EvalSetConfigs: []*entity.EvalSetConfig{
					{
						EvalSetID: 100, EvalSetVersionID: 1000,
						ItemFilter: &entity.ExptItemFilter{
							QueryAndOr: "and",
							FilterFields: []*entity.ExptItemFilterField{
								{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"1", "2", "3"}},
							},
						},
						EvaluatorConfs: []*entity.ExptEvaluatorConf{
							{
								EvaluatorID: 50, EvaluatorVersionID: 5001, Alias: "evaluator_1", ScoreWeight: gptr.Of(0.6),
								FromEvalSet: []*entity.FieldConf{{FieldName: "input", FromField: "input"}},
								FromTarget:  []*entity.FieldConf{{FieldName: "output", FromField: "actual_output"}},
							},
						},
					},
					{
						EvalSetID: 200, EvalSetVersionID: 2000,
						ItemFilter: &entity.ExptItemFilter{
							QueryAndOr: "and",
							FilterFields: []*entity.ExptItemFilterField{
								{FieldName: "category", FieldType: "tag", QueryType: "eq", Values: []string{"baike"}},
							},
						},
					},
				},
			},
		}

		converted := OpenAPIExptDO2DTO(experiment)
		if assert.NotNil(t, converted) && assert.Len(t, converted.EvalSetConfigs, 2) {
			c0 := converted.EvalSetConfigs[0]
			assert.Equal(t, int64(100), c0.GetEvalSetID())
			assert.Equal(t, "0.0.1", c0.GetEvalSetVersion()) // set version_id 1000 反查
			if assert.NotNil(t, c0.ItemFilter) && assert.Len(t, c0.ItemFilter.FilterFields, 1) {
				assert.Equal(t, "item_id", c0.ItemFilter.FilterFields[0].FieldName)
				assert.Equal(t, "long", c0.ItemFilter.FilterFields[0].FieldType)
				assert.Equal(t, "in", c0.ItemFilter.FilterFields[0].GetQueryType())
				assert.Equal(t, []string{"1", "2", "3"}, c0.ItemFilter.FilterFields[0].Values)
			}
			if assert.Len(t, c0.EvaluatorConfs, 1) {
				ec := c0.EvaluatorConfs[0]
				assert.Equal(t, int64(50), ec.GetEvaluatorID())
				assert.Equal(t, "0.0.1", ec.GetVersion()) // evaluator version_id 5001 反查
				assert.Equal(t, "evaluator_1", ec.GetAlias())
				assert.InDelta(t, 0.6, ec.GetScoreWeight(), 1e-9)
				if assert.Len(t, ec.FromEvalSet, 1) {
					assert.Equal(t, "input", ec.FromEvalSet[0].GetFieldName())
					assert.Equal(t, "input", ec.FromEvalSet[0].GetFromFieldName())
				}
				if assert.Len(t, ec.FromTarget, 1) {
					assert.Equal(t, "output", ec.FromTarget[0].GetFieldName())
					assert.Equal(t, "actual_output", ec.FromTarget[0].GetFromFieldName())
				}
			}
			// 第二集: tag 条件圈选
			c1 := converted.EvalSetConfigs[1]
			assert.Equal(t, "0.0.2", c1.GetEvalSetVersion())
			if assert.NotNil(t, c1.ItemFilter) && assert.Len(t, c1.ItemFilter.FilterFields, 1) {
				assert.Equal(t, "category", c1.ItemFilter.FilterFields[0].FieldName)
				assert.Equal(t, "tag", c1.ItemFilter.FilterFields[0].FieldType)
				assert.Equal(t, "eq", c1.ItemFilter.FilterFields[0].GetQueryType())
			}
		}
	})

	t.Run("SingleSet 旧实验不回显 eval_set_configs", func(t *testing.T) {
		experiment := &entity.Experiment{
			ID: 2, Status: entity.ExptStatus_Success, ExptType: entity.ExptType_Offline,
			EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet,
			EvalConf: &entity.EvaluationConfiguration{
				EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 100, EvalSetVersionID: 1000}},
			},
		}
		converted := OpenAPIExptDO2DTO(experiment)
		if assert.NotNil(t, converted) {
			assert.Nil(t, converted.EvalSetConfigs, "SingleSet 不应回显 eval_set_configs")
		}
	})
}

// TestOpenAPIExptDO2DTO_OnlineExperimentHidesEvalSet — 校验线上实验不返回 eval_set（保留原有契约）
func TestOpenAPIExptDO2DTO_OnlineExperimentHidesEvalSet(t *testing.T) {
	t.Parallel()

	experiment := &entity.Experiment{
		ID:       1,
		Status:   entity.ExptStatus_Success,
		ExptType: entity.ExptType_Online,
		EvalSet:  &entity.EvaluationSet{ID: 9},
	}
	converted := OpenAPIExptDO2DTO(experiment)
	if assert.NotNil(t, converted) {
		assert.Nil(t, converted.EvalSet)
	}
}

// TestOpenAPIEvalSetConfigsDTO2Domain_ItemFilter 验证 OpenAPI item-centric 多评测集配置里的
// item_filter (题目圈选) 被原样透传到内部 domain EvalSetConfig (与内部 EvalSetConfig.item_filter 同型)。
func TestOpenAPIEvalSetConfigsDTO2Domain_ItemFilter(t *testing.T) {
	t.Parallel()

	itemFilter := &domain_filter.Filter{
		QueryAndOr: gptr.Of(domain_filter.QueryRelation("and")),
		FilterFields: []*domain_filter.FilterField{
			{
				FieldName: "item_id",
				FieldType: domain_filter.FieldType("long"),
				QueryType: gptr.Of(domain_filter.QueryType("in")),
				Values:    []string{"1", "2"},
			},
		},
	}
	confs := []*openapiExperiment.OpenAPIEvalSetConfig{
		{
			EvalSetID:      gptr.Of(int64(100)),
			EvalSetVersion: gptr.Of("v1.0.0"),
			ItemFilter:     itemFilter,
		},
		{
			// 不传 item_filter = 全集, 转换后应为 nil。
			EvalSetID:      gptr.Of(int64(200)),
			EvalSetVersion: gptr.Of("v2.0.0"),
		},
	}
	evalSetVersionIDMap := map[int64]int64{100: 1001, 200: 2002}

	dos := OpenAPIEvalSetConfigsDTO2Domain(confs, evalSetVersionIDMap, map[string]int64{})
	assert.Len(t, dos, 2)

	// 第一集: item_filter 原指针透传, 字段保真。
	assert.Same(t, itemFilter, dos[0].ItemFilter)
	assert.Equal(t, int64(1001), dos[0].GetEvalSetVersionID())
	assert.Len(t, dos[0].ItemFilter.FilterFields, 1)
	assert.Equal(t, "item_id", dos[0].ItemFilter.FilterFields[0].FieldName)
	assert.Equal(t, "long", dos[0].ItemFilter.FilterFields[0].FieldType)
	assert.Equal(t, "in", dos[0].ItemFilter.FilterFields[0].GetQueryType())
	assert.Equal(t, []string{"1", "2"}, dos[0].ItemFilter.FilterFields[0].Values)

	// 第二集: 不传 item_filter → nil (全集语义)。
	assert.Nil(t, dos[1].ItemFilter)
}

// TestOpenAPIEvalSetConfigsDTO2Domain_EvaluatorFilter 验证 OpenAPI evaluator 级 filter/filter_mode
// (行级过滤: 命中才执行本 binding) 被原样透传到内部 domain ExptEvaluatorConf
// (与内部 ExptEvaluatorConf.filter/filter_mode 同型)。
func TestOpenAPIEvalSetConfigsDTO2Domain_EvaluatorFilter(t *testing.T) {
	t.Parallel()

	evalFilter := &domain_filter.Filter{
		QueryAndOr: gptr.Of(domain_filter.QueryRelation("and")),
		FilterFields: []*domain_filter.FilterField{
			{
				FieldName: "tag_key",
				FieldType: domain_filter.FieldType("tag"),
				QueryType: gptr.Of(domain_filter.QueryType("eq")),
				Values:    []string{"hard"},
			},
		},
	}
	confs := []*openapiExperiment.OpenAPIEvalSetConfig{
		{
			EvalSetID:      gptr.Of(int64(100)),
			EvalSetVersion: gptr.Of("v1.0.0"),
			EvaluatorConfs: []*openapiExperiment.OpenAPIExptEvaluatorConf{
				{
					// 带 filter + filter_mode=1 (Include)
					EvaluatorID: gptr.Of(int64(11)),
					Version:     gptr.Of("e-v1"),
					Filter:      evalFilter,
					FilterMode:  gptr.Of(int32(1)),
				},
				{
					// 带 filter_mode=2 (Exclude), 不带 filter
					EvaluatorID: gptr.Of(int64(12)),
					Version:     gptr.Of("e-v2"),
					FilterMode:  gptr.Of(int32(2)),
				},
				{
					// 缺省: 不带 filter / filter_mode → Filter nil, FilterMode 0
					EvaluatorID: gptr.Of(int64(13)),
					Version:     gptr.Of("e-v3"),
				},
			},
		},
	}
	evalSetVersionIDMap := map[int64]int64{100: 1001}
	evaluatorVersionIDMap := map[string]int64{
		"11_e-v1": 111,
		"12_e-v2": 122,
		"13_e-v3": 133,
	}

	dos := OpenAPIEvalSetConfigsDTO2Domain(confs, evalSetVersionIDMap, evaluatorVersionIDMap)
	assert.Len(t, dos, 1)
	assert.Len(t, dos[0].EvaluatorConfs, 3)

	ec0 := dos[0].EvaluatorConfs[0]
	// filter 原指针透传, 字段保真; filter_mode=1
	assert.Same(t, evalFilter, ec0.Filter)
	assert.Len(t, ec0.Filter.FilterFields, 1)
	assert.Equal(t, "tag_key", ec0.Filter.FilterFields[0].FieldName)
	assert.Equal(t, "eq", ec0.Filter.FilterFields[0].GetQueryType())
	assert.Equal(t, int32(1), ec0.GetFilterMode())
	assert.Equal(t, int64(111), ec0.GetEvaluatorVersionID())

	ec1 := dos[0].EvaluatorConfs[1]
	// 无 filter → nil; filter_mode=2 透传
	assert.Nil(t, ec1.Filter)
	assert.Equal(t, int32(2), ec1.GetFilterMode())

	ec2 := dos[0].EvaluatorConfs[2]
	// 缺省: filter nil, filter_mode 默认 0
	assert.Nil(t, ec2.Filter)
	assert.False(t, ec2.IsSetFilterMode())
	assert.Equal(t, int32(0), ec2.GetFilterMode())
}

// 中心化调度读视图在 OpenAPI 面的回显（IDL 116~118）。
//
// 补这三个字段的动因与 run_mode_config 相同：写侧能配、读侧没有，OpenAPI 调用方
// 查不到自己的实验有没有被中心调度纳管、申报了多少额度 —— 两套读模型的不对称。
func TestDomainExperimentDTO2OpenAPI_SchedulingReadView(t *testing.T) {
	t.Parallel()

	out := DomainExperimentDTO2OpenAPI(&domainExpt.Experiment{
		ID:            gptr.Of(int64(1)),
		PriorityLevel: gptr.Of(int32(9)),
		SchedulerMode: gptr.Of("enforce"),
		ExpectedQuotaConsumption: &domainExpt.ExpectedQuotaConsumption{
			Resources: []*domainExpt.ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "default", Amount: 1},
				{Category: "model", ResourceKey: "gpt5.5", Amount: 1000},
			},
		},
	})

	assert.NotNil(t, out)
	assert.Equal(t, int32(9), out.GetPriorityLevel())
	assert.Equal(t, "enforce", out.GetSchedulerMode())

	res := out.GetExpectedQuotaConsumption().GetResources()
	assert.Len(t, res, 2)
	assert.Equal(t, "sandbox", res[0].GetCategory())
	assert.Equal(t, "default", res[0].GetResourceKey())
	assert.Equal(t, int64(1), res[0].GetAmount())
	assert.Equal(t, "model", res[1].GetCategory())
	assert.Equal(t, int64(1000), res[1].GetAmount())
}

// legacy 实验也回显 mode/priority，向量省略。
func TestDomainExperimentDTO2OpenAPI_SchedulingReadView_Legacy(t *testing.T) {
	t.Parallel()

	out := DomainExperimentDTO2OpenAPI(&domainExpt.Experiment{
		ID:            gptr.Of(int64(2)),
		PriorityLevel: gptr.Of(int32(1)),
		SchedulerMode: gptr.Of("legacy"),
	})

	assert.NotNil(t, out)
	assert.Equal(t, "legacy", out.GetSchedulerMode(),
		"legacy 也要回显——'为什么我的实验没进中心调度'是最高频疑问，回显 legacy 可一眼确认")
	assert.Equal(t, int32(1), out.GetPriorityLevel())
	assert.Nil(t, out.ExpectedQuotaConsumption, "legacy 确实没申报向量，省略比返回空结构更如实")
}

func TestExpectedQuotaConsumptionDomain2OpenAPI(t *testing.T) {
	t.Parallel()

	// nil / 空 / 全 nil 元素都返回 nil，让 optional 字段省略。
	assert.Nil(t, ExpectedQuotaConsumptionDomain2OpenAPI(nil))
	assert.Nil(t, ExpectedQuotaConsumptionDomain2OpenAPI(&domainExpt.ExpectedQuotaConsumption{}))
	assert.Nil(t, ExpectedQuotaConsumptionDomain2OpenAPI(&domainExpt.ExpectedQuotaConsumption{
		Resources: []*domainExpt.ExpectedResourceConsumption{nil, nil},
	}))

	// 混入 nil 元素时跳过，不 panic 也不产出空壳资源。
	got := ExpectedQuotaConsumptionDomain2OpenAPI(&domainExpt.ExpectedQuotaConsumption{
		Resources: []*domainExpt.ExpectedResourceConsumption{
			nil,
			{Category: "evaluator", ResourceKey: "*", Amount: 3},
		},
	})
	assert.NotNil(t, got)
	assert.Len(t, got.GetResources(), 1)
	assert.Equal(t, "evaluator", got.GetResources()[0].GetCategory())
	assert.Equal(t, int64(3), got.GetResources()[0].GetAmount())
}

// TestExpectedQuotaConsumption_SourceSurvivesRoundTrip
// ★ source 必须在四个转换点上都不丢。
//
// 为什么值得端到端钉：转换层有**四个**独立的搬运点（内部 DTO↔DO 两个、OpenAPI↔domain 两个），
// 每个都是逐字段手写赋值。漏掉 source 的后果极隐蔽 —— 申报方明明传了来源，
// 落库时被吞掉，于是不同来源的用量记进同一个账本条目、互相挤占，
// 而接口回显看起来"没申报过来源"，与调用方真的没传完全一样。
func TestExpectedQuotaConsumption_SourceSurvivesRoundTrip(t *testing.T) {
	t.Run("内部 DTO→DO→DTO", func(t *testing.T) {
		dto := &domainExpt.ExpectedQuotaConsumption{
			Resources: []*domainExpt.ExpectedResourceConsumption{
				{Category: "model", ResourceKey: "kimi-k3", Amount: 6, Source: gptr.Of("litellm")},
				{Category: "model", ResourceKey: "kimi-k3", Amount: 6}, // 同资源、无来源
			},
		}

		do := ExpectedQuotaConsumptionDTO2DO(dto)
		if assert.NotNil(t, do) && assert.Len(t, do.Resources, 2) {
			assert.Equal(t, "litellm", do.Resources[0].Source, "source 必须搬进 DO")
			assert.Empty(t, do.Resources[1].Source, "没申报来源时 DO 侧应为空串，不得凭空造值")
		}

		back := expectedQuotaConsumptionDO2DTO(do)
		if assert.NotNil(t, back) && assert.Len(t, back.Resources, 2) {
			assert.Equal(t, "litellm", back.Resources[0].GetSource(), "source 必须回显")
			assert.Empty(t, back.Resources[1].GetSource())
		}
	})

	t.Run("OpenAPI↔domain 双向", func(t *testing.T) {
		domainDTO := &domainExpt.ExpectedQuotaConsumption{
			Resources: []*domainExpt.ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "mac", Amount: 1, Source: gptr.Of("self-hosted")},
			},
		}

		openapiDTO := ExpectedQuotaConsumptionDomain2OpenAPI(domainDTO)
		if assert.NotNil(t, openapiDTO) && assert.Len(t, openapiDTO.GetResources(), 1) {
			assert.Equal(t, "self-hosted", openapiDTO.GetResources()[0].GetSource(),
				"source 必须搬到 OpenAPI DTO —— sandbox 也能有来源，本字段不是模型专用")
		}

		back := ExpectedQuotaConsumptionOpenAPI2Domain(openapiDTO)
		if assert.NotNil(t, back) && assert.Len(t, back.GetResources(), 1) {
			assert.Equal(t, "self-hosted", back.GetResources()[0].GetSource())
		}
	})
}
