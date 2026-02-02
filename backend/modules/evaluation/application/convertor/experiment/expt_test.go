// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestConvertCreateExptTemplateReq_FieldMappingAndRuntimeAndRunConfig(t *testing.T) {
	req := &expt.CreateExperimentTemplateRequest{
		WorkspaceID: 100,
		Meta: &domain_expt.ExptTemplateMeta{
			Name:     gptr.Of("tpl"),
			Desc:     gptr.Of("desc"),
			ExptType: gptr.Of(domain_expt.ExptType_Offline),
		},
		TripleConfig: &domain_expt.ExptTuple{
			EvalSetID:        gptr.Of(int64(1)),
			EvalSetVersionID: gptr.Of(int64(11)),
			TargetID:         gptr.Of(int64(2)),
			TargetVersionID:  gptr.Of(int64(22)),
			EvaluatorIDVersionItems: []*evaluatordto.EvaluatorIDVersionItem{
				{
					EvaluatorID:        gptr.Of(int64(10)),
					Version:            gptr.Of("v1"),
					EvaluatorVersionID: gptr.Of(int64(1001)),
					ScoreWeight:        gptr.Of(0.5),
				},
			},
		},
		FieldMappingConfig: &domain_expt.ExptFieldMapping{
			TargetFieldMapping: &domain_expt.TargetFieldMapping{
				FromEvalSet: []*domain_expt.FieldMapping{
					{
						FieldName:     gptr.Of("output"),
						FromFieldName: gptr.Of("col1"),
						ConstValue:    gptr.Of(""),
					},
				},
			},
			TargetRuntimeParam: &common.RuntimeParam{
				JSONValue: gptr.Of(`{"rt":"v"}`),
			},
			EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
				{
					EvaluatorVersionID: 1001,
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("col1"),
						},
					},
					EvaluatorIDVersionItem: &evaluatordto.EvaluatorIDVersionItem{
						EvaluatorID:        gptr.Of(int64(10)),
						Version:            gptr.Of("v1"),
						EvaluatorVersionID: gptr.Of(int64(1001)),
						RunConfig: &evaluatordto.EvaluatorRunConfig{
							Env: gptr.Of("prod"),
							EvaluatorRuntimeParam: &common.RuntimeParam{
								JSONValue: gptr.Of(`{"k":"v"}`),
							},
						},
					},
				},
			},
			ItemConcurNum: gptr.Of(int32(3)),
		},
		DefaultEvaluatorsConcurNum: gptr.Of(int32(5)),
	}

	param, err := ConvertCreateExptTemplateReq(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), param.SpaceID)
	assert.Equal(t, "tpl", param.Name)
	assert.Equal(t, entity.ExptType_Offline, param.ExptType)

	// triple config
	assert.Equal(t, int64(1), param.EvalSetID)
	assert.Equal(t, int64(11), param.EvalSetVersionID)
	assert.Equal(t, int64(2), param.TargetID)
	assert.Equal(t, int64(22), param.TargetVersionID)
	if assert.Len(t, param.EvaluatorIDVersionItems, 1) {
		item := param.EvaluatorIDVersionItems[0]
		assert.Equal(t, int64(10), item.EvaluatorID)
		assert.Equal(t, "v1", item.Version)
		assert.Equal(t, int64(1001), item.EvaluatorVersionID)
		assert.Equal(t, 0.5, item.ScoreWeight)
	}

	// TemplateConf: Target runtime param
	assert.NotNil(t, param.TemplateConf)
	assert.NotNil(t, param.TemplateConf.ConnectorConf.TargetConf)
	assert.NotNil(t, param.TemplateConf.ConnectorConf.TargetConf.IngressConf)
	if assert.NotNil(t, param.TemplateConf.ConnectorConf.TargetConf.IngressConf.CustomConf) {
		// runtime_param 被放到了 CustomConf 里，对应 consts.FieldAdapterBuiltinFieldNameRuntimeParam
		found := false
		for _, fc := range param.TemplateConf.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs {
			if fc.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
				found = true
				assert.Equal(t, `{"rt":"v"}`, fc.Value)
			}
		}
		assert.True(t, found, "runtime_param field should exist in CustomConf")
	}

	// TemplateConf: Evaluator run_config、权重
	evConfs := param.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf
	if assert.Len(t, evConfs, 1) {
		ec := evConfs[0]
		assert.Equal(t, int64(1001), ec.EvaluatorVersionID)
		assert.Equal(t, int64(10), ec.EvaluatorID)
		assert.Equal(t, "v1", ec.Version)
		// run_config
		if assert.NotNil(t, ec.RunConf) {
			assert.Equal(t, "prod", gptr.Indirect(ec.RunConf.Env))
			if assert.NotNil(t, ec.RunConf.EvaluatorRuntimeParam) {
				assert.Equal(t, `{"k":"v"}`, gptr.Indirect(ec.RunConf.EvaluatorRuntimeParam.JSONValue))
			}
		}
		// score_weight
		if assert.NotNil(t, ec.ScoreWeight) {
			assert.Equal(t, 0.5, *ec.ScoreWeight)
		}
	}
	assert.Equal(t, 3, gptr.Indirect(param.TemplateConf.ItemConcurNum))
	assert.Equal(t, 5, gptr.Indirect(param.TemplateConf.EvaluatorsConcurNum))
}

func TestConvertUpdateExptTemplateReq_FieldMappingAndRuntimeAndRunConfig(t *testing.T) {
	req := &expt.UpdateExperimentTemplateRequest{
		WorkspaceID: 100,
		TemplateID:  2001,
		Meta: &domain_expt.ExptTemplateMeta{
			Name:     gptr.Of("tpl2"),
			Desc:     gptr.Of("desc2"),
			ExptType: gptr.Of(domain_expt.ExptType_Online),
		},
		TripleConfig: &domain_expt.ExptTuple{
			EvalSetVersionID: gptr.Of(int64(21)),
			TargetVersionID:  gptr.Of(int64(31)),
			EvaluatorIDVersionItems: []*evaluatordto.EvaluatorIDVersionItem{
				{
					EvaluatorID:        gptr.Of(int64(20)),
					Version:            gptr.Of("v2"),
					EvaluatorVersionID: gptr.Of(int64(2002)),
					ScoreWeight:        gptr.Of(0.8),
				},
			},
		},
		FieldMappingConfig: &domain_expt.ExptFieldMapping{
			TargetFieldMapping: &domain_expt.TargetFieldMapping{
				FromEvalSet: []*domain_expt.FieldMapping{
					{
						FieldName:     gptr.Of("output2"),
						FromFieldName: gptr.Of("col2"),
					},
				},
			},
			TargetRuntimeParam: &common.RuntimeParam{
				JSONValue: gptr.Of(`{"rt2":"v2"}`),
			},
			EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
				{
					EvaluatorVersionID: 2002,
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input2"),
							FromFieldName: gptr.Of("col2"),
						},
					},
					EvaluatorIDVersionItem: &evaluatordto.EvaluatorIDVersionItem{
						EvaluatorID:        gptr.Of(int64(20)),
						Version:            gptr.Of("v2"),
						EvaluatorVersionID: gptr.Of(int64(2002)),
						RunConfig: &evaluatordto.EvaluatorRunConfig{
							Env: gptr.Of("staging"),
							EvaluatorRuntimeParam: &common.RuntimeParam{
								JSONValue: gptr.Of(`{"k2":"v2"}`),
							},
						},
					},
				},
			},
			ItemConcurNum: gptr.Of(int32(7)),
		},
		DefaultEvaluatorsConcurNum: gptr.Of(int32(9)),
	}

	param, err := ConvertUpdateExptTemplateReq(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(2001), param.TemplateID)
	assert.Equal(t, int64(100), param.SpaceID)
	assert.Equal(t, "tpl2", param.Name)
	assert.Equal(t, entity.ExptType_Online, param.ExptType)
	assert.Equal(t, int64(21), param.EvalSetVersionID)
	assert.Equal(t, int64(31), param.TargetVersionID)

	// EvaluatorIDVersionItems
	if assert.Len(t, param.EvaluatorIDVersionItems, 1) {
		it := param.EvaluatorIDVersionItems[0]
		assert.Equal(t, int64(20), it.EvaluatorID)
		assert.Equal(t, "v2", it.Version)
		assert.Equal(t, int64(2002), it.EvaluatorVersionID)
		assert.Equal(t, 0.8, it.ScoreWeight)
	}

	// TemplateConf
	assert.NotNil(t, param.TemplateConf)
	assert.Equal(t, 7, gptr.Indirect(param.TemplateConf.ItemConcurNum))
	assert.Equal(t, 9, gptr.Indirect(param.TemplateConf.EvaluatorsConcurNum))

	// Target runtime param in TemplateConf
	tc := param.TemplateConf.ConnectorConf
	assert.NotNil(t, tc.TargetConf)
	if assert.NotNil(t, tc.TargetConf.IngressConf) && tc.TargetConf.IngressConf.CustomConf != nil {
		found := false
		for _, fc := range tc.TargetConf.IngressConf.CustomConf.FieldConfs {
			if fc.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
				found = true
				assert.Equal(t, `{"rt2":"v2"}`, fc.Value)
			}
		}
		assert.True(t, found, "runtime_param field should exist in TargetConf.CustomConf")
	}

	// EvaluatorConf with run_config and score_weight
	evConfs := tc.EvaluatorsConf.EvaluatorConf
	if assert.Len(t, evConfs, 1) {
		ec := evConfs[0]
		assert.Equal(t, int64(2002), ec.EvaluatorVersionID)
		assert.Equal(t, int64(20), ec.EvaluatorID)
		assert.Equal(t, "v2", ec.Version)
		if assert.NotNil(t, ec.RunConf) {
			assert.Equal(t, "staging", gptr.Indirect(ec.RunConf.Env))
			if assert.NotNil(t, ec.RunConf.EvaluatorRuntimeParam) {
				assert.Equal(t, `{"k2":"v2"}`, gptr.Indirect(ec.RunConf.EvaluatorRuntimeParam.JSONValue))
			}
		}
		if assert.NotNil(t, ec.ScoreWeight) {
			assert.Equal(t, 0.8, *ec.ScoreWeight)
		}
	}
}

func TestConvertUpdateExptTemplateMetaReq(t *testing.T) {
	req := &expt.UpdateExperimentTemplateMetaRequest{
		WorkspaceID: 100,
		TemplateID:  2001,
		Meta: &domain_expt.ExptTemplateMeta{
			Name:     gptr.Of("tpl-meta"),
			Desc:     gptr.Of("meta-desc"),
			ExptType: gptr.Of(domain_expt.ExptType_Online),
		},
	}

	param, err := ConvertUpdateExptTemplateMetaReq(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(2001), param.TemplateID)
	assert.Equal(t, int64(100), param.SpaceID)
	assert.Equal(t, "tpl-meta", param.Name)
	assert.Equal(t, "meta-desc", param.Description)
	assert.Equal(t, entity.ExptType_Online, param.ExptType)

	// nil meta 时不应 panic，字段保持默认值
	req.Meta = nil
	param, err = ConvertUpdateExptTemplateMetaReq(req)
	assert.NoError(t, err)
	assert.Equal(t, "", param.Name)
	assert.Equal(t, "", param.Description)
}

func TestToExptTemplateDTO_WithRunConfAndScoreWeight(t *testing.T) {
	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          1,
			WorkspaceID: 100,
			Name:        "tpl",
			Desc:        "desc",
			ExptType:    entity.ExptType_Offline,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        10,
			EvalSetVersionID: 11,
			TargetID:         20,
			TargetVersionID:  21,
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{
				{
					EvaluatorID:        1,
					Version:            "v1",
					EvaluatorVersionID: 101,
					ScoreWeight:        0.6,
				},
			},
		},
		TemplateConf: &entity.ExptTemplateConfiguration{
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{
					EvaluatorConf: []*entity.EvaluatorConf{
						{
							EvaluatorVersionID: 101,
							EvaluatorID:        1,
							Version:            "v1",
							RunConf: &entity.EvaluatorRunConfig{
								Env: gptr.Of("prod"),
								EvaluatorRuntimeParam: &entity.RuntimeParam{
									JSONValue: gptr.Of(`{"foo":"bar"}`),
								},
							},
							ScoreWeight: gptr.Of(0.6),
						},
					},
				},
			},
		},
		BaseInfo: &entity.BaseInfo{
			CreatedAt: gptr.Of[int64](1),
			UpdatedAt: gptr.Of[int64](2),
			DeletedAt: gptr.Of[int64](0),
			CreatedBy: &entity.UserInfo{UserID: gptr.Of("u1")},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of("u2")},
		},
		ExptInfo: &entity.ExptInfo{
			CreatedExptCount: 3,
			LatestExptID:     99,
			LatestExptStatus: entity.ExptStatus_Success,
		},
	}

	dto := ToExptTemplateDTO(template)
	if assert.NotNil(t, dto) {
		assert.NotNil(t, dto.Meta)
		assert.Equal(t, int64(1), gptr.Indirect(dto.Meta.ID))
		assert.NotNil(t, dto.TripleConfig)
		if assert.Len(t, dto.TripleConfig.EvaluatorIDVersionItems, 1) {
			item := dto.TripleConfig.EvaluatorIDVersionItems[0]
			assert.Equal(t, int64(1), gptr.Indirect(item.EvaluatorID))
			assert.Equal(t, "v1", gptr.Indirect(item.Version))
			assert.Equal(t, int64(101), gptr.Indirect(item.EvaluatorVersionID))
			assert.Equal(t, 0.6, gptr.Indirect(item.ScoreWeight))
			if assert.NotNil(t, item.RunConfig) && assert.NotNil(t, item.RunConfig.EvaluatorRuntimeParam) {
				assert.Equal(t, `{"foo":"bar"}`, gptr.Indirect(item.RunConfig.EvaluatorRuntimeParam.JSONValue))
			}
		}
		assert.NotNil(t, dto.BaseInfo)
		assert.NotNil(t, dto.GetExptInfo())
		assert.Equal(t, int64(3), gptr.Indirect(dto.GetExptInfo().CreatedExptCount))
	}
}

func TestBuildEvaluatorScoreWeights(t *testing.T) {
	// 正常情况
	items := []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 1, Version: "v1", ScoreWeight: 0.5},
		{EvaluatorID: 2, Version: "v2", ScoreWeight: 0.8},
		// 无效项应被忽略
		{EvaluatorID: 0, Version: "v1", ScoreWeight: 1},
		{EvaluatorID: 3, Version: "", ScoreWeight: 1},
		{EvaluatorID: 4, Version: "v4", ScoreWeight: 0},
	}
	weights := buildEvaluatorScoreWeights(items)
	if assert.NotNil(t, weights) {
		assert.Len(t, weights, 2)
		assert.Equal(t, 0.5, weights["1#v1"])
		assert.Equal(t, 0.8, weights["2#v2"])
	}

	// 空或全部无效时返回 nil
	assert.Nil(t, buildEvaluatorScoreWeights(nil))
	assert.Nil(t, buildEvaluatorScoreWeights([]*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 0, Version: "v1", ScoreWeight: 1},
	}))
}

func TestBuildEvaluatorConfsFromItemsAndApplyScoreWeights(t *testing.T) {
	// items 中有 evaluator_version_id，触发按 versionID 构建的分支
	items := []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 101},
		{EvaluatorID: 2, Version: "v2", EvaluatorVersionID: 0}, // 无效，应该被忽略
	}
	fieldMappings := []*entity.EvaluatorConf{
		{
			EvaluatorVersionID: 101,
			IngressConf:        &entity.EvaluatorIngressConf{TargetAdapter: &entity.FieldAdapter{}},
			RunConf: &entity.EvaluatorRunConfig{
				Env: gptr.Of("prod"),
			},
		},
	}

	confs := buildEvaluatorConfsFromItems(items, fieldMappings)
	if assert.Len(t, confs, 1) {
		assert.Equal(t, int64(101), confs[0].EvaluatorVersionID)
		assert.Equal(t, int64(1), confs[0].EvaluatorID)
		assert.Equal(t, "v1", confs[0].Version)
		assert.NotNil(t, confs[0].IngressConf)
		if assert.NotNil(t, confs[0].RunConf) {
			assert.Equal(t, "prod", gptr.Indirect(confs[0].RunConf.Env))
		}
	}

	// 应用 score weight
	weights := map[string]float64{
		"1#v1": 0.5,
	}
	applyScoreWeightsToEvaluatorConfs(weights, confs)
	if assert.Len(t, confs, 1) && assert.NotNil(t, confs[0].ScoreWeight) {
		assert.Equal(t, 0.5, *confs[0].ScoreWeight)
	}

	// 当 items 没有有效 versionID 时，退化为直接透传字段映射
	itemsNoVer := []*entity.EvaluatorIDVersionItem{
		{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 0},
	}
	confs2 := buildEvaluatorConfsFromItems(itemsNoVer, fieldMappings)
	// 期望直接返回 fieldMappings（过滤掉 nil）
	if assert.Len(t, confs2, 1) {
		assert.Equal(t, int64(101), confs2[0].EvaluatorVersionID)
	}
}

func TestBuildTemplateConfForCreate(t *testing.T) {
	param := &entity.CreateExptTemplateParam{
		TargetVersionID: 21,
	}
	req := &expt.CreateExperimentTemplateRequest{
		DefaultEvaluatorsConcurNum: gptr.Of(int32(5)),
	}
	targetIngress := &entity.TargetIngressConf{}
	evaluatorConfs := []*entity.EvaluatorConf{
		{EvaluatorVersionID: 101},
	}
	itemConcurNum := gptr.Of(int32(3))

	conf := buildTemplateConfForCreate(param, req, targetIngress, evaluatorConfs, itemConcurNum)
	if assert.NotNil(t, conf) {
		assert.Equal(t, 3, gptr.Indirect(conf.ItemConcurNum))
		assert.Equal(t, 5, gptr.Indirect(conf.EvaluatorsConcurNum))
		assert.NotNil(t, conf.ConnectorConf.TargetConf)
		assert.Equal(t, int64(21), conf.ConnectorConf.TargetConf.TargetVersionID)
		if assert.NotNil(t, conf.ConnectorConf.EvaluatorsConf) {
			assert.Len(t, conf.ConnectorConf.EvaluatorsConf.EvaluatorConf, 1)
		}
	}

	// target & evaluator 都为空时，只返回基础并发配置
	conf2 := buildTemplateConfForCreate(param, req, nil, nil, nil)
	if assert.NotNil(t, conf2) {
		assert.Nil(t, conf2.ConnectorConf.TargetConf)
		assert.Nil(t, conf2.ConnectorConf.EvaluatorsConf)
	}
}

func TestToTargetFieldMappingDOForTemplate(t *testing.T) {
	// 有 mapping 和 runtime param
	mapping := &domain_expt.TargetFieldMapping{
		FromEvalSet: []*domain_expt.FieldMapping{
			{
				FieldName:     gptr.Of("f1"),
				FromFieldName: gptr.Of("src1"),
				ConstValue:    gptr.Of("v1"),
			},
		},
	}
	rt := &entity.RuntimeParam{JSONValue: gptr.Of(`{"k":"v"}`)}
	conf := toTargetFieldMappingDOForTemplate(mapping, rt)
	if assert.NotNil(t, conf) {
		if assert.NotNil(t, conf.EvalSetAdapter) && assert.Len(t, conf.EvalSetAdapter.FieldConfs, 1) {
			f := conf.EvalSetAdapter.FieldConfs[0]
			assert.Equal(t, "f1", f.FieldName)
			assert.Equal(t, "src1", f.FromField)
			assert.Equal(t, "v1", f.Value)
		}
		if assert.NotNil(t, conf.CustomConf) && assert.Len(t, conf.CustomConf.FieldConfs, 1) {
			f := conf.CustomConf.FieldConfs[0]
			assert.Equal(t, consts.FieldAdapterBuiltinFieldNameRuntimeParam, f.FieldName)
			assert.Equal(t, `{"k":"v"}`, f.Value)
		}
	}

	// mapping 为 nil 但 runtime 有值
	conf2 := toTargetFieldMappingDOForTemplate(nil, rt)
	if assert.NotNil(t, conf2.CustomConf) {
		assert.Len(t, conf2.CustomConf.FieldConfs, 1)
	}
}

func TestToEvaluatorFieldMappingDOForTemplate_RunConfAndDefaults(t *testing.T) {
	// 含 EvaluatorIDVersionItem + RunConfig
	runParam := &common.RuntimeParam{JSONValue: gptr.Of(`{"rk":"rv"}`)}
	mappings := []*domain_expt.EvaluatorFieldMapping{
		{
			EvaluatorVersionID: 101,
			FromEvalSet: []*domain_expt.FieldMapping{
				{
					FieldName:     gptr.Of("input"),
					FromFieldName: gptr.Of("col1"),
				},
			},
			FromTarget: []*domain_expt.FieldMapping{
				{
					FieldName:     gptr.Of("output"),
					FromFieldName: gptr.Of("col2"),
				},
			},
			EvaluatorIDVersionItem: &evaluatordto.EvaluatorIDVersionItem{
				EvaluatorID:        gptr.Of(int64(1)),
				Version:            gptr.Of("v1"),
				EvaluatorVersionID: gptr.Of(int64(101)),
				RunConfig: &evaluatordto.EvaluatorRunConfig{
					Env:                   gptr.Of("prod"),
					EvaluatorRuntimeParam: runParam,
				},
			},
		},
	}

	confs := toEvaluatorFieldMappingDoForTemplate(mappings, nil)
	if assert.Len(t, confs, 1) {
		ec := confs[0]
		assert.Equal(t, int64(101), ec.EvaluatorVersionID)
		assert.Equal(t, int64(1), ec.EvaluatorID)
		assert.Equal(t, "v1", ec.Version)
		if assert.NotNil(t, ec.RunConf) {
			assert.Equal(t, "prod", gptr.Indirect(ec.RunConf.Env))
			if assert.NotNil(t, ec.RunConf.EvaluatorRuntimeParam) {
				assert.Equal(t, `{"rk":"rv"}`, gptr.Indirect(ec.RunConf.EvaluatorRuntimeParam.JSONValue))
			}
		}
		if assert.NotNil(t, ec.IngressConf) {
			assert.Len(t, ec.IngressConf.EvalSetAdapter.FieldConfs, 1)
			assert.Len(t, ec.IngressConf.TargetAdapter.FieldConfs, 1)
		}
	}

	// nil mapping 返回 nil
	assert.Nil(t, toEvaluatorFieldMappingDoForTemplate(nil, nil))
}

func TestBuildTemplateFieldMappingDTO_WithRunConf(t *testing.T) {
	// 准备 TemplateConf 中带 RunConf 的 EvaluatorConf
	templateConf := &entity.ExptTemplateConfiguration{
		ConnectorConf: entity.Connector{
			EvaluatorsConf: &entity.EvaluatorsConf{
				EvaluatorConf: []*entity.EvaluatorConf{
					{
						EvaluatorVersionID: 101,
						EvaluatorID:        1,
						Version:            "v1",
						RunConf: &entity.EvaluatorRunConfig{
							Env: gptr.Of("prod"),
							EvaluatorRuntimeParam: &entity.RuntimeParam{
								JSONValue: gptr.Of(`{"a":"b"}`),
							},
						},
					},
				},
			},
		},
	}

	template := &entity.ExptTemplate{
		FieldMappingConfig: &entity.ExptFieldMapping{
			EvaluatorFieldMapping: []*entity.EvaluatorFieldMapping{
				{
					EvaluatorVersionID: 101,
					EvaluatorID:        1,
					Version:            "v1",
					FromEvalSet: []*entity.ExptTemplateFieldMapping{
						{FieldName: "input", FromFieldName: "col1"},
					},
					FromTarget: []*entity.ExptTemplateFieldMapping{
						{FieldName: "output", FromFieldName: "col2"},
					},
				},
			},
		},
		TemplateConf: templateConf,
	}

	dto := buildTemplateFieldMappingDTO(template)
	if assert.NotNil(t, dto) && assert.Len(t, dto.EvaluatorFieldMapping, 1) {
		em := dto.EvaluatorFieldMapping[0]
		assert.Equal(t, int64(101), em.GetEvaluatorVersionID())
		// EvaluatorIDVersionItem 中应包含 RunConfig
		item := em.GetEvaluatorIDVersionItem()
		if assert.NotNil(t, item) {
			assert.Equal(t, int64(1), item.GetEvaluatorID())
			assert.Equal(t, "v1", item.GetVersion())
			if assert.NotNil(t, item.RunConfig) && assert.NotNil(t, item.RunConfig.EvaluatorRuntimeParam) {
				assert.Equal(t, `{"a":"b"}`, item.RunConfig.EvaluatorRuntimeParam.GetJSONValue())
			}
		}
	}
}

func TestBuildTemplateFieldMappingDTO_TargetMapping(t *testing.T) {
	template := &entity.ExptTemplate{
		FieldMappingConfig: &entity.ExptFieldMapping{
			TargetFieldMapping: &entity.TargetFieldMapping{
				FromEvalSet: []*entity.ExptTemplateFieldMapping{
					{FieldName: "f1", FromFieldName: "src1", ConstValue: "v1"},
					{FieldName: "f2", FromFieldName: "src2", ConstValue: "v2"},
				},
			},
		},
	}

	dto := buildTemplateFieldMappingDTO(template)
	if assert.NotNil(t, dto) && assert.NotNil(t, dto.TargetFieldMapping) {
		assert.Len(t, dto.TargetFieldMapping.FromEvalSet, 2)
		assert.Equal(t, "f1", gptr.Indirect(dto.TargetFieldMapping.FromEvalSet[0].FieldName))
		assert.Equal(t, "src1", gptr.Indirect(dto.TargetFieldMapping.FromEvalSet[0].FromFieldName))
		assert.Equal(t, "v1", gptr.Indirect(dto.TargetFieldMapping.FromEvalSet[0].ConstValue))
	}
}

func TestConvertTemplateConfToDTO_Full(t *testing.T) {
	conf := &entity.ExptTemplateConfiguration{
		ConnectorConf: entity.Connector{
			TargetConf: &entity.TargetConf{
				IngressConf: &entity.TargetIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "t1", FromField: "src1", Value: "v1"},
						},
					},
					CustomConf: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam, Value: `{"k":"v"}`},
						},
					},
				},
			},
			EvaluatorsConf: &entity.EvaluatorsConf{
				EvaluatorConf: []*entity.EvaluatorConf{
					{
						EvaluatorVersionID: 101,
						EvaluatorID:        1,
						Version:            "v1",
						ScoreWeight:        gptr.Of(0.7),
						RunConf: &entity.EvaluatorRunConfig{
							Env: gptr.Of("prod"),
							EvaluatorRuntimeParam: &entity.RuntimeParam{
								JSONValue: gptr.Of(`{"rk":"rv"}`),
							},
						},
						IngressConf: &entity.EvaluatorIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{FieldName: "ein", FromField: "col1", Value: ""},
								},
							},
							TargetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{FieldName: "eout", FromField: "col2", Value: ""},
								},
							},
						},
					},
				},
			},
		},
	}

	target, evalMappings, rt := convertTemplateConfToDTO(conf)
	if assert.NotNil(t, target) {
		assert.Len(t, target.FromEvalSet, 1)
		assert.Equal(t, "t1", gptr.Indirect(target.FromEvalSet[0].FieldName))
	}
	if assert.NotNil(t, rt) {
		assert.Equal(t, `{"k":"v"}`, gptr.Indirect(rt.JSONValue))
	}
	if assert.Len(t, evalMappings, 1) {
		em := evalMappings[0]
		assert.Equal(t, int64(101), em.GetEvaluatorVersionID())
		item := em.GetEvaluatorIDVersionItem()
		if assert.NotNil(t, item) {
			assert.Equal(t, int64(1), item.GetEvaluatorID())
			assert.Equal(t, "v1", item.GetVersion())
			assert.Equal(t, int64(101), item.GetEvaluatorVersionID())
			if assert.NotNil(t, item.ScoreWeight) {
				assert.Equal(t, 0.7, gptr.Indirect(item.ScoreWeight))
			}
			if assert.NotNil(t, item.RunConfig) && assert.NotNil(t, item.RunConfig.EvaluatorRuntimeParam) {
				assert.Equal(t, `{"rk":"rv"}`, item.RunConfig.EvaluatorRuntimeParam.GetJSONValue())
			}
		}
		assert.Len(t, em.FromEvalSet, 1)
		assert.Len(t, em.FromTarget, 1)
	}
}

func TestAppendEvaluatorIDVersionItemsFromEvaluators(t *testing.T) {
	// 准备 evaluator 及其版本
	pev := &entity.PromptEvaluatorVersion{}
	pev.SetEvaluatorID(1)
	pev.SetVersion("v1")
	pev.SetID(101)
	eval := &entity.Evaluator{
		EvaluatorType:          entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: pev,
	}

	// TripleConfig 中包含相同 evaluator_version_id 的权重
	template := &entity.ExptTemplate{
		Evaluators: []*entity.Evaluator{eval},
		TripleConfig: &entity.ExptTemplateTuple{
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{
				{EvaluatorVersionID: 101, ScoreWeight: 0.9},
			},
		},
	}

	var dst []*evaluatordto.EvaluatorIDVersionItem
	appendEvaluatorIDVersionItemsFromEvaluators(template, &dst)

	if assert.Len(t, dst, 1) {
		it := dst[0]
		assert.Equal(t, int64(1), gptr.Indirect(it.EvaluatorID))
		assert.Equal(t, "v1", gptr.Indirect(it.Version))
		assert.Equal(t, int64(101), gptr.Indirect(it.EvaluatorVersionID))
		// 权重应该从 TripleConfig.EvaluatorIDVersionItems 中匹配并设置
		if assert.NotNil(t, it.ScoreWeight) {
			assert.Equal(t, 0.9, gptr.Indirect(it.ScoreWeight))
		}
	}
}

func TestAppendEvaluatorIDVersionItemsFromVersionRef(t *testing.T) {
	template := &entity.ExptTemplate{
		EvaluatorVersionRef: []*entity.ExptTemplateEvaluatorVersionRef{
			{EvaluatorID: 1, EvaluatorVersionID: 101},
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{
				{EvaluatorVersionID: 101, ScoreWeight: 0.8},
			},
		},
	}

	var dst []*evaluatordto.EvaluatorIDVersionItem
	appendEvaluatorIDVersionItemsFromVersionRef(template, &dst)

	if assert.Len(t, dst, 1) {
		it := dst[0]
		assert.Equal(t, int64(1), gptr.Indirect(it.EvaluatorID))
		assert.Equal(t, int64(101), gptr.Indirect(it.EvaluatorVersionID))
		// 权重同样在 buildEvaluatorIDVersionItemsDTO 中填充，这里只关心基本字段
	}
}

func TestBuildEvaluatorIDVersionItemsDTO_FromEvaluators(t *testing.T) {
	pev := &entity.PromptEvaluatorVersion{}
	pev.SetEvaluatorID(1)
	pev.SetVersion("v1")
	pev.SetID(101)
	eval := &entity.Evaluator{
		EvaluatorType:          entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: pev,
	}

	template := &entity.ExptTemplate{
		Evaluators: []*entity.Evaluator{eval},
		TripleConfig: &entity.ExptTemplateTuple{
			// 不填 EvaluatorIDVersionItems，强制走 Evaluators 分支
			EvaluatorIDVersionItems: nil,
		},
	}

	items := buildEvaluatorIDVersionItemsDTO(template)
	if assert.Len(t, items, 1) {
		it := items[0]
		assert.Equal(t, int64(1), gptr.Indirect(it.EvaluatorID))
		assert.Equal(t, "v1", gptr.Indirect(it.Version))
		assert.Equal(t, int64(101), gptr.Indirect(it.EvaluatorVersionID))
	}
}

func TestBuildEvaluatorIDVersionItemsDTO_FromVersionRef(t *testing.T) {
	template := &entity.ExptTemplate{
		TripleConfig: &entity.ExptTemplateTuple{
			// 这里让 EvaluatorIDVersionItems 为空，从而跳过第一分支
			EvaluatorIDVersionItems: nil,
		},
		EvaluatorVersionRef: []*entity.ExptTemplateEvaluatorVersionRef{
			{EvaluatorID: 2, EvaluatorVersionID: 202},
		},
	}

	items := buildEvaluatorIDVersionItemsDTO(template)
	if assert.Len(t, items, 1) {
		it := items[0]
		assert.Equal(t, int64(2), gptr.Indirect(it.EvaluatorID))
		assert.Equal(t, int64(202), gptr.Indirect(it.EvaluatorVersionID))
	}
}

func TestEvalConfConvert_ConvertEntityToDTO(t *testing.T) {
	raw := `{
    "ConnectorConf":
    {
        "TargetConf":
        {
            "TargetVersionID": 7486074365205872641,
            "IngressConf":
            {
                "EvalSetAdapter":
                {
                    "FieldConfs":
                    [
                        {
                            "FieldName": "role",
                            "FromField": "role",
                            "Value": ""
                        },
                        {
                            "FieldName": "question",
                            "FromField": "input",
                            "Value": ""
                        }
                    ]
                },
                "CustomConf": null
            }
        },
        "EvaluatorsConf":
        {
            "EvaluatorConcurNum": null,
            "EvaluatorConf":
            [
                {
                    "EvaluatorVersionID": 7486074365205823489,
                    "IngressConf":
                    {
                        "EvalSetAdapter":
                        {
                            "FieldConfs":
                            [
                                {
                                    "FieldName": "input",
                                    "FromField": "input",
                                    "Value": ""
                                },
                                {
                                    "FieldName": "reference_output",
                                    "FromField": "reference_output",
                                    "Value": ""
                                }
                            ]
                        },
                        "TargetAdapter":
                        {
                            "FieldConfs":
                            [
                                {
                                    "FieldName": "output",
                                    "FromField": "actual_output",
                                    "Value": ""
                                }
                            ]
                        },
                        "CustomConf": null
                    },
                    "RunConf": {
                        "evaluator_runtime_param": {
                            "json_value": "{\"key\":\"val\"}"
                        }
                    }
                }
            ]
        }
    },
    "ItemConcurNum": null
}`
	conf := &entity.EvaluationConfiguration{}
	err := json.Unmarshal([]byte(raw), &conf)
	assert.Nil(t, err)

	target, evaluators, _, evrcs := NewEvalConfConvert().ConvertEntityToDTO(conf)
	t.Logf("target: %v", json.Jsonify(target))
	t.Logf("evaluators: %v", json.Jsonify(evaluators))

	assert.NotNil(t, target)
	assert.Len(t, evaluators, 1)
	assert.Equal(t, int64(7486074365205823489), evaluators[0].EvaluatorVersionID)
	assert.NotNil(t, evrcs)
	assert.Contains(t, evrcs, int64(7486074365205823489))
	assert.Equal(t, `{"key":"val"}`, *evrcs[7486074365205823489].EvaluatorRuntimeParam.JSONValue)
}

func TestConvertExptTurnResultFilterAccelerator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name    string
		input   *domain_expt.ExperimentFilter
		want    *entity.ExptTurnResultFilterAccelerator
		wantErr bool
	}{
		{
			name: "有效输入",
			input: &domain_expt.ExperimentFilter{
				Filters: &domain_expt.Filters{
					FilterConditions: []*domain_expt.FilterCondition{
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_ItemID,
							},
							Operator:     domain_expt.FilterOperatorType_Equal,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_ItemRunState,
							},
							Operator:     domain_expt.FilterOperatorType_Greater,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_TurnRunState,
							},
							Operator:     domain_expt.FilterOperatorType_GreaterOrEqual,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_EvaluatorScore,
							},
							Operator:     domain_expt.FilterOperatorType_Less,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_ActualOutput,
							},
							Operator:     domain_expt.FilterOperatorType_LessOrEqual,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_Annotation,
							},
							Operator:     domain_expt.FilterOperatorType_Like,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_EvaluatorScoreCorrected,
							},
							Operator:     domain_expt.FilterOperatorType_NotIn,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_EvalSetColumn,
							},
							Operator:     domain_expt.FilterOperatorType_NotLike,
							Value:        "1",
							SourceTarget: nil,
						},
					},
					LogicOp: ptr.Of(domain_expt.FilterLogicOp_And),
				},
				KeywordSearch: &domain_expt.KeywordSearch{
					Keyword: ptr.Of("1"),
					FilterFields: []*domain_expt.FilterField{
						{
							FieldType: domain_expt.FieldType_ActualOutput,
						},
					},
				},
			},
			want: &entity.ExptTurnResultFilterAccelerator{
				ItemIDs: []*entity.FieldFilter{
					{
						Key:    "item_id",
						Op:     "=",
						Values: []any{"1"},
					},
				},
				ItemRunStatus: []*entity.FieldFilter{},
				TurnRunStatus: []*entity.FieldFilter{},
				MapCond: &entity.ExptTurnResultFilterMapCond{
					EvalTargetDataFilters:    []*entity.FieldFilter{},
					EvaluatorScoreFilters:    []*entity.FieldFilter{},
					AnnotationFloatFilters:   []*entity.FieldFilter{},
					AnnotationBoolFilters:    []*entity.FieldFilter{},
					AnnotationStringFilters:  []*entity.FieldFilter{},
					EvalTargetMetricsFilters: []*entity.FieldFilter{},
				},
				ItemSnapshotCond: &entity.ItemSnapshotFilter{
					BoolMapFilters:   []*entity.FieldFilter{},
					StringMapFilters: []*entity.FieldFilter{},
					IntMapFilters:    []*entity.FieldFilter{},
					FloatMapFilters:  []*entity.FieldFilter{},
				},
				KeywordSearch: &entity.KeywordFilter{
					EvalTargetDataFilters: []*entity.FieldFilter{
						{
							Key:    "actual_output",
							Op:     "LIKE",
							Values: []any{"%1%"},
						},
					},
					ItemSnapshotFilter: &entity.ItemSnapshotFilter{
						BoolMapFilters:   []*entity.FieldFilter{},
						StringMapFilters: []*entity.FieldFilter{},
						IntMapFilters:    []*entity.FieldFilter{},
						FloatMapFilters:  []*entity.FieldFilter{},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertExptTurnResultFilterAccelerator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertExptTurnResultFilterAccelerator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got.ItemIDs) != len(tt.want.ItemIDs) {
					t.Errorf("ConvertExptTurnResultFilterAccelerator() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestConvertExptTurnResultFilterAccelerator_EvalTargetMetrics(t *testing.T) {
	tests := []struct {
		name    string
		input   *domain_expt.ExperimentFilter
		want    *entity.ExptTurnResultFilterAccelerator
		wantErr bool
	}{
		{
			name: "TotalLatency filter",
			input: &domain_expt.ExperimentFilter{
				Filters: &domain_expt.Filters{
					LogicOp: ptr.Of(domain_expt.FilterLogicOp_And),
					FilterConditions: []*domain_expt.FilterCondition{
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_TotalLatency,
								FieldKey:  ptr.Of("test_key"),
							},
							Operator: domain_expt.FilterOperatorType_Equal,
							Value:    "100",
						},
					},
				},
			},
			want: &entity.ExptTurnResultFilterAccelerator{
				MapCond: &entity.ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*entity.FieldFilter{
						{
							Key:    "total_latency",
							Op:     "=",
							Values: []any{"100"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "InputTokens filter",
			input: &domain_expt.ExperimentFilter{
				Filters: &domain_expt.Filters{
					LogicOp: ptr.Of(domain_expt.FilterLogicOp_And),
					FilterConditions: []*domain_expt.FilterCondition{
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_InputTokens,
								FieldKey:  ptr.Of("test_key"),
							},
							Operator: domain_expt.FilterOperatorType_Greater,
							Value:    "10",
						},
					},
				},
			},
			want: &entity.ExptTurnResultFilterAccelerator{
				MapCond: &entity.ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*entity.FieldFilter{
						{
							Key:    "input_tokens",
							Op:     ">",
							Values: []any{"10"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "OutputTokens filter",
			input: &domain_expt.ExperimentFilter{
				Filters: &domain_expt.Filters{
					LogicOp: ptr.Of(domain_expt.FilterLogicOp_And),
					FilterConditions: []*domain_expt.FilterCondition{
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_OutputTokens,
								FieldKey:  ptr.Of("test_key"),
							},
							Operator: domain_expt.FilterOperatorType_Less,
							Value:    "20",
						},
					},
				},
			},
			want: &entity.ExptTurnResultFilterAccelerator{
				MapCond: &entity.ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*entity.FieldFilter{
						{
							Key:    "output_tokens",
							Op:     "<",
							Values: []any{"20"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "TotalTokens filter",
			input: &domain_expt.ExperimentFilter{
				Filters: &domain_expt.Filters{
					LogicOp: ptr.Of(domain_expt.FilterLogicOp_And),
					FilterConditions: []*domain_expt.FilterCondition{
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_TotalTokens,
								FieldKey:  ptr.Of("test_key"),
							},
							Operator: domain_expt.FilterOperatorType_In,
							Value:    "30,40,50",
						},
					},
				},
			},
			want: &entity.ExptTurnResultFilterAccelerator{
				MapCond: &entity.ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*entity.FieldFilter{
						{
							Key:    "total_tokens",
							Op:     "IN",
							Values: []any{"30", "40", "50"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple EvalTargetMetrics filters",
			input: &domain_expt.ExperimentFilter{
				Filters: &domain_expt.Filters{
					LogicOp: ptr.Of(domain_expt.FilterLogicOp_And),
					FilterConditions: []*domain_expt.FilterCondition{
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_TotalLatency,
								FieldKey:  ptr.Of("test_key"),
							},
							Operator: domain_expt.FilterOperatorType_Equal,
							Value:    "100",
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_InputTokens,
								FieldKey:  ptr.Of("test_key"),
							},
							Operator: domain_expt.FilterOperatorType_Greater,
							Value:    "10",
						},
					},
				},
			},
			want: &entity.ExptTurnResultFilterAccelerator{
				MapCond: &entity.ExptTurnResultFilterMapCond{
					EvalTargetMetricsFilters: []*entity.FieldFilter{
						{
							Key:    "total_latency",
							Op:     "=",
							Values: []any{"100"},
						},
						{
							Key:    "input_tokens",
							Op:     ">",
							Values: []any{"10"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertExptTurnResultFilterAccelerator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertExptTurnResultFilterAccelerator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.want.MapCond != nil {
					assert.Equal(t, len(tt.want.MapCond.EvalTargetMetricsFilters), len(got.MapCond.EvalTargetMetricsFilters))
					for i, wantFilter := range tt.want.MapCond.EvalTargetMetricsFilters {
						if i < len(got.MapCond.EvalTargetMetricsFilters) {
							gotFilter := got.MapCond.EvalTargetMetricsFilters[i]
							assert.Equal(t, wantFilter.Key, gotFilter.Key)
							assert.Equal(t, wantFilter.Op, gotFilter.Op)
							assert.Equal(t, wantFilter.Values, gotFilter.Values)
						}
					}
				}
			}
		})
	}
}

func TestToTargetFieldMappingDO_RuntimeParam(t *testing.T) {
	tests := []struct {
		name                       string
		request                    *expt.CreateExperimentRequest
		evaluatorVersionRunConfigs map[int64]*evaluatordto.EvaluatorRunConfig
		wantCustomConf             *entity.FieldAdapter
		wantEvaluatorRunConf       map[int64]string
	}{
		{
			name: "正常运行时参数转换",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
							ConstValue:    gptr.Of(""),
						},
					},
				},
				TargetRuntimeParam: &common.RuntimeParam{
					JSONValue: gptr.Of(`{"model_config":{"model_id":"test_model","temperature":0.7}}`),
				},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: &entity.FieldAdapter{
				FieldConfs: []*entity.FieldConf{
					{
						FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
						Value:     `{"model_config":{"model_id":"test_model","temperature":0.7}}`,
					},
				},
			},
		},
		{
			name: "包含评估器运行时参数转换",
			request: &expt.CreateExperimentRequest{
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			evaluatorVersionRunConfigs: map[int64]*evaluatordto.EvaluatorRunConfig{
				456: {
					EvaluatorRuntimeParam: &common.RuntimeParam{
						JSONValue: gptr.Of(`{"key":"val"}`),
					},
				},
			},
			wantEvaluatorRunConf: map[int64]string{
				456: `{"key":"val"}`,
			},
		},
		{
			name: "运行时参数为nil",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				TargetRuntimeParam: nil,
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: nil,
		},
		{
			name: "运行时参数JSONValue为空",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				TargetRuntimeParam: &common.RuntimeParam{
					JSONValue: nil,
				},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: nil,
		},
		{
			name: "mapping为nil但有运行时参数",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: nil,
				TargetRuntimeParam: &common.RuntimeParam{JSONValue: gptr.Of(`{"test":"value"}`)},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: &entity.FieldAdapter{
				FieldConfs: []*entity.FieldConf{
					{
						FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
						Value:     `{"test":"value"}`,
					},
				},
			},
		},
		{
			name: "mapping和运行时参数都为nil",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: nil,
				TargetRuntimeParam: nil,
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: nil,
		},
	}

	converter := NewEvalConfConvert()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToEntity(tt.request, tt.evaluatorVersionRunConfigs)
			assert.NoError(t, err)

			assert.NotNil(t, result)
			assert.NotNil(t, result.ConnectorConf.TargetConf)
			assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf)
			assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf.EvalSetAdapter)

			// 检查EvalSetAdapter的FieldConfs
			if tt.request.TargetFieldMapping == nil {
				assert.Empty(t, result.ConnectorConf.TargetConf.IngressConf.EvalSetAdapter.FieldConfs)
			} else {
				assert.NotEmpty(t, result.ConnectorConf.TargetConf.IngressConf.EvalSetAdapter.FieldConfs)
			}

			// 检查CustomConf
			if tt.wantCustomConf == nil {
				assert.Nil(t, result.ConnectorConf.TargetConf.IngressConf.CustomConf)
			} else {
				assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf.CustomConf)
				assert.Equal(t, len(tt.wantCustomConf.FieldConfs), len(result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs))
				if len(tt.wantCustomConf.FieldConfs) > 0 {
					assert.Equal(t, tt.wantCustomConf.FieldConfs[0].FieldName, result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs[0].FieldName)
					assert.Equal(t, tt.wantCustomConf.FieldConfs[0].Value, result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs[0].Value)
				}
			}

			// 检查Evaluator RunConf
			if len(tt.wantEvaluatorRunConf) > 0 {
				assert.NotNil(t, result.ConnectorConf.EvaluatorsConf)
				for _, ec := range result.ConnectorConf.EvaluatorsConf.EvaluatorConf {
					if wantVal, ok := tt.wantEvaluatorRunConf[ec.EvaluatorVersionID]; ok {
						assert.NotNil(t, ec.RunConf)
						assert.Equal(t, wantVal, *ec.RunConf.EvaluatorRuntimeParam.JSONValue)
					}
				}
			}
		})
	}
}

func TestEvalConfConvert_ConvertEntityToDTO_RuntimeParam(t *testing.T) {
	tests := []struct {
		name             string
		ec               *entity.EvaluationConfiguration
		wantRuntimeParam *common.RuntimeParam
	}{
		{
			name: "包含运行时参数的配置",
			ec: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					TargetConf: &entity.TargetConf{
						TargetVersionID: 123,
						IngressConf: &entity.TargetIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: "input",
										FromField: "question",
									},
								},
							},
							CustomConf: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
										Value:     `{"model_config":{"model_id":"converted_model","temperature":0.5}}`,
									},
								},
							},
						},
					},
				},
			},
			wantRuntimeParam: &common.RuntimeParam{
				JSONValue: gptr.Of(`{"model_config":{"model_id":"converted_model","temperature":0.5}}`),
			},
		},
		{
			name: "无运行时参数的配置",
			ec: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					TargetConf: &entity.TargetConf{
						TargetVersionID: 123,
						IngressConf: &entity.TargetIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: "input",
										FromField: "question",
									},
								},
							},
							CustomConf: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: "other_field",
										Value:     "other_value",
									},
								},
							},
						},
					},
				},
			},
			wantRuntimeParam: &common.RuntimeParam{},
		},
		{
			name: "CustomConf为nil",
			ec: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					TargetConf: &entity.TargetConf{
						TargetVersionID: 123,
						IngressConf: &entity.TargetIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: "input",
										FromField: "question",
									},
								},
							},
							CustomConf: nil,
						},
					},
				},
			},
			wantRuntimeParam: &common.RuntimeParam{},
		},
		{
			name:             "配置为nil",
			ec:               nil,
			wantRuntimeParam: nil,
		},
	}

	converter := NewEvalConfConvert()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, runtimeParam, _ := converter.ConvertEntityToDTO(tt.ec)

			if tt.wantRuntimeParam == nil {
				assert.Nil(t, runtimeParam)
			} else {
				assert.NotNil(t, runtimeParam)
				if tt.wantRuntimeParam.JSONValue == nil {
					assert.Nil(t, runtimeParam.JSONValue)
				} else {
					assert.NotNil(t, runtimeParam.JSONValue)
					assert.Equal(t, gptr.Indirect(tt.wantRuntimeParam.JSONValue), gptr.Indirect(runtimeParam.JSONValue))
				}
			}
		})
	}
}

func TestEvalConfConvert_ConvertToEntity_RuntimeParam(t *testing.T) {
	tests := []struct {
		name                       string
		request                    *expt.CreateExperimentRequest
		evaluatorVersionRunConfigs map[int64]*evaluatordto.EvaluatorRunConfig
		wantCustomConf             *entity.FieldAdapter
		wantErr                    bool
	}{
		{
			name: "包含运行时参数的请求",
			request: &expt.CreateExperimentRequest{
				TargetVersionID: gptr.Of(int64(123)),
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				TargetRuntimeParam: &common.RuntimeParam{
					JSONValue: gptr.Of(`{"model_config":{"model_id":"request_model","max_tokens":200}}`),
				},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
						FromEvalSet: []*domain_expt.FieldMapping{
							{
								FieldName:     gptr.Of("input"),
								FromFieldName: gptr.Of("question"),
							},
						},
					},
				},
			},
			wantCustomConf: &entity.FieldAdapter{
				FieldConfs: []*entity.FieldConf{
					{
						FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
						Value:     `{"model_config":{"model_id":"request_model","max_tokens":200}}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "包含评估器运行时参数的请求",
			request: &expt.CreateExperimentRequest{
				TargetVersionID: gptr.Of(int64(123)),
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			evaluatorVersionRunConfigs: map[int64]*evaluatordto.EvaluatorRunConfig{
				456: {
					EvaluatorRuntimeParam: &common.RuntimeParam{
						JSONValue: gptr.Of(`{"key":"val"}`),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "无运行时参数的请求",
			request: &expt.CreateExperimentRequest{
				TargetVersionID: gptr.Of(int64(123)),
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				TargetRuntimeParam: nil,
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: nil,
			wantErr:        false,
		},
		{
			name: "EvaluatorFieldMapping为nil的请求",
			request: &expt.CreateExperimentRequest{
				TargetVersionID:       gptr.Of(int64(123)),
				EvaluatorFieldMapping: nil,
			},
			wantCustomConf: nil,
			wantErr:        false,
		},
	}

	converter := NewEvalConfConvert()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToEntity(tt.request, tt.evaluatorVersionRunConfigs)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)

			if tt.wantCustomConf == nil {
				if result.ConnectorConf.TargetConf != nil && result.ConnectorConf.TargetConf.IngressConf != nil {
					assert.Nil(t, result.ConnectorConf.TargetConf.IngressConf.CustomConf)
				}
			} else {
				assert.NotNil(t, result.ConnectorConf.TargetConf)
				assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf)
				assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf.CustomConf)
				assert.Equal(t, len(tt.wantCustomConf.FieldConfs), len(result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs))
				if len(tt.wantCustomConf.FieldConfs) > 0 {
					assert.Equal(t, tt.wantCustomConf.FieldConfs[0].FieldName, result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs[0].FieldName)
					assert.Equal(t, tt.wantCustomConf.FieldConfs[0].Value, result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs[0].Value)
				}
			}

			if len(tt.evaluatorVersionRunConfigs) > 0 {
				assert.NotNil(t, result.ConnectorConf.EvaluatorsConf)
				for _, ec := range result.ConnectorConf.EvaluatorsConf.EvaluatorConf {
					if wantConf, ok := tt.evaluatorVersionRunConfigs[ec.EvaluatorVersionID]; ok {
						assert.NotNil(t, ec.RunConf)
						assert.Equal(t, *wantConf.EvaluatorRuntimeParam.JSONValue, *ec.RunConf.EvaluatorRuntimeParam.JSONValue)
					}
				}
			}
		})
	}
}

func TestToExptDTO_RuntimeParam(t *testing.T) {
	tests := []struct {
		name                       string
		experiment                 *entity.Experiment
		wantRuntimeParam           bool
		wantJSONValue              string
		wantEvaluatorIDVersionList bool
	}{
		{
			name: "包含运行时参数的实验",
			experiment: &entity.Experiment{
				ID:       123,
				SourceID: "test_source",
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 456,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: "input",
											FromField: "question",
										},
									},
								},
								CustomConf: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
											Value:     `{"model_config":{"model_id":"dto_test_model"}}`,
										},
									},
								},
							},
						},
					},
				},
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{},
			},
			wantRuntimeParam: true,
			wantJSONValue:    `{"model_config":{"model_id":"dto_test_model"}}`,
		},
		{
			name: "无运行时参数的实验",
			experiment: &entity.Experiment{
				ID:       123,
				SourceID: "test_source",
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 456,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: "input",
											FromField: "question",
										},
									},
								},
								CustomConf: nil,
							},
						},
					},
				},
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{},
			},
			wantRuntimeParam: false,
		},
		{
			name: "包含评估器版本列表的实验",
			experiment: &entity.Experiment{
				ID:       123,
				SourceID: "test_source",
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
					{EvaluatorID: 1, EvaluatorVersionID: 101},
					{EvaluatorID: 2, EvaluatorVersionID: 102},
				},
				Evaluators: []*entity.Evaluator{
					{
						ID:            1,
						EvaluatorType: entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
							ID:          101,
							EvaluatorID: 1,
							Version:     "v1",
						},
					},
					{
						ID:            2,
						EvaluatorType: entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
							ID:          102,
							EvaluatorID: 2,
							Version:     "v2",
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 101,
									IngressConf:        &entity.EvaluatorIngressConf{},
									RunConf: &entity.EvaluatorRunConfig{
										EvaluatorRuntimeParam: &entity.RuntimeParam{
											JSONValue: gptr.Of(`{"key":"val"}`),
										},
									},
								},
							},
						},
					},
				},
			},
			wantEvaluatorIDVersionList: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToExptDTO(tt.experiment)

			assert.NotNil(t, result)
			assert.Equal(t, tt.experiment.ID, gptr.Indirect(result.ID))
			assert.Equal(t, tt.experiment.SourceID, gptr.Indirect(result.SourceID))

			if tt.wantRuntimeParam {
				assert.NotNil(t, result.TargetRuntimeParam)
				assert.NotNil(t, result.TargetRuntimeParam.JSONValue)
				assert.Equal(t, tt.wantJSONValue, gptr.Indirect(result.TargetRuntimeParam.JSONValue))
			} else if tt.name != "包含评估器版本列表的实验" {
				// 当没有运行时参数时，应该返回空的RuntimeParam对象而不是nil
				assert.NotNil(t, result.TargetRuntimeParam)
				assert.Nil(t, result.TargetRuntimeParam.JSONValue)
			}

			if tt.wantEvaluatorIDVersionList {
				assert.Len(t, result.EvaluatorIDVersionList, 2)
				assert.Equal(t, int64(1), *result.EvaluatorIDVersionList[0].EvaluatorID)
				assert.Equal(t, "v1", *result.EvaluatorIDVersionList[0].Version)
				assert.NotNil(t, result.EvaluatorIDVersionList[0].RunConfig)
				assert.Equal(t, `{"key":"val"}`, *result.EvaluatorIDVersionList[0].RunConfig.EvaluatorRuntimeParam.JSONValue)

				assert.Equal(t, int64(2), *result.EvaluatorIDVersionList[1].EvaluatorID)
				assert.Equal(t, "v2", *result.EvaluatorIDVersionList[1].Version)
				assert.Nil(t, result.EvaluatorIDVersionList[1].RunConfig)
			}
		})
	}
}

func TestConvertCreateReq(t *testing.T) {
	tests := []struct {
		name                       string
		cer                        *expt.CreateExperimentRequest
		evaluatorVersionRunConfigs map[int64]*evaluatordto.EvaluatorRunConfig
		want                       *entity.CreateExptParam
		wantErr                    bool
	}{
		{
			name: "normal conversion",
			cer: &expt.CreateExperimentRequest{
				WorkspaceID:         1,
				EvalSetVersionID:    gptr.Of(int64(10)),
				TargetVersionID:     gptr.Of(int64(20)),
				EvaluatorVersionIds: []int64{30, 40},
				Name:                gptr.Of("test-expt"),
				Desc:                gptr.Of("test-desc"),
				EvalSetID:           gptr.Of(int64(100)),
				TargetID:            gptr.Of(int64(200)),
				ExptType:            gptr.Of(domain_expt.ExptType_Offline),
				MaxAliveTime:        gptr.Of(int64(3600)),
				SourceType:          gptr.Of(domain_expt.SourceType_Evaluation),
				SourceID:            gptr.Of("source-id"),
				ItemConcurNum:       gptr.Of(int32(5)),
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("f1"),
							FromFieldName: gptr.Of("from_f1"),
						},
					},
				},
			},
			evaluatorVersionRunConfigs: map[int64]*evaluatordto.EvaluatorRunConfig{
				30: {
					EvaluatorRuntimeParam: &common.RuntimeParam{
						JSONValue: gptr.Of(`{"k":"v"}`),
					},
				},
			},
			want: &entity.CreateExptParam{
				WorkspaceID:         1,
				EvalSetVersionID:    10,
				TargetVersionID:     20,
				EvaluatorVersionIds: []int64{30, 40},
				Name:                "test-expt",
				Desc:                "test-desc",
				EvalSetID:           100,
				TargetID:            gptr.Of(int64(200)),
				ExptType:            entity.ExptType_Offline,
				MaxAliveTime:        3600,
				SourceType:          entity.SourceType_Evaluation,
				SourceID:            "source-id",
			},
			wantErr: false,
		},
		{
			name: "with CreateEvalTargetParam",
			cer: &expt.CreateExperimentRequest{
				WorkspaceID: 1,
				CreateEvalTargetParam: &eval_target.CreateEvalTargetParam{
					SourceTargetID:      gptr.Of("200"),
					SourceTargetVersion: gptr.Of("20"),
					EvalTargetType:      ptr.Of(domain_eval_target.EvalTargetType_CozeBot),
				},
			},
			want: &entity.CreateExptParam{
				WorkspaceID: 1,
				CreateEvalTargetParam: &entity.CreateEvalTargetParam{
					SourceTargetID:      gptr.Of("200"),
					SourceTargetVersion: gptr.Of("20"),
					EvalTargetType:      gptr.Of(entity.EvalTargetTypeCozeBot),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertCreateReq(tt.cer, tt.evaluatorVersionRunConfigs)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)

			// Check basic fields
			assert.Equal(t, tt.want.WorkspaceID, got.WorkspaceID)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.EvalSetVersionID, got.EvalSetVersionID)
			assert.Equal(t, tt.want.TargetVersionID, got.TargetVersionID)
			assert.Equal(t, tt.want.EvaluatorVersionIds, got.EvaluatorVersionIds)

			if tt.want.CreateEvalTargetParam != nil {
				assert.NotNil(t, got.CreateEvalTargetParam)
				assert.Equal(t, tt.want.CreateEvalTargetParam.SourceTargetID, got.CreateEvalTargetParam.SourceTargetID)
				assert.Equal(t, tt.want.CreateEvalTargetParam.EvalTargetType, got.CreateEvalTargetParam.EvalTargetType)
			}

			if got.ExptConf != nil {
				// Verify ExptConf conversion happened (delegated to ConvertToEntity)
				assert.Equal(t, ptr.ConvIntPtr[int32, int](tt.cer.ItemConcurNum), got.ExptConf.ItemConcurNum)
			}
		})
	}
}

func TestBuildTemplateScoreWeightConfigDTO_FromTripleConfig(t *testing.T) {
	t.Run("从TripleConfig.EvaluatorIDVersionItems构建权重配置", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{
					{
						EvaluatorVersionID: 101,
						ScoreWeight:        0.6,
					},
					{
						EvaluatorVersionID: 102,
						ScoreWeight:        0.4,
					},
					{
						EvaluatorVersionID: 0, // 无效，应该跳过
						ScoreWeight:        0.5,
					},
					{
						EvaluatorVersionID: 103,
						ScoreWeight:        0, // 无效，应该跳过
					},
					nil, // nil项，应该跳过
				},
			},
		}
		result := buildTemplateScoreWeightConfigDTO(template)
		assert.NotNil(t, result)
		assert.True(t, *result.EnableWeightedScore)
		assert.Equal(t, 0.6, result.EvaluatorScoreWeights[101])
		assert.Equal(t, 0.4, result.EvaluatorScoreWeights[102])
		assert.NotContains(t, result.EvaluatorScoreWeights, int64(103))
		assert.NotContains(t, result.EvaluatorScoreWeights, int64(0))
	})

	t.Run("TripleConfig为空，返回nil", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: nil,
		}
		result := buildTemplateScoreWeightConfigDTO(template)
		assert.Nil(t, result)
	})

	t.Run("EvaluatorIDVersionItems为空，返回nil", func(t *testing.T) {
		template := &entity.ExptTemplate{
			TripleConfig: &entity.ExptTemplateTuple{
				EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{},
			},
		}
		result := buildTemplateScoreWeightConfigDTO(template)
		assert.Nil(t, result)
	})

	t.Run("TemplateConf已有权重配置，优先使用TemplateConf", func(t *testing.T) {
		scoreWeight := 0.8
		template := &entity.ExptTemplate{
			TemplateConf: &entity.ExptTemplateConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{
						EvaluatorConf: []*entity.EvaluatorConf{
							{
								EvaluatorVersionID: 201,
								ScoreWeight:        &scoreWeight,
							},
						},
					},
				},
			},
			TripleConfig: &entity.ExptTemplateTuple{
				EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{
					{
						EvaluatorVersionID: 101,
						ScoreWeight:        0.6,
					},
				},
			},
		}
		result := buildTemplateScoreWeightConfigDTO(template)
		assert.NotNil(t, result)
		// 应该使用 TemplateConf 中的权重，而不是 TripleConfig 中的
		assert.Equal(t, 0.8, result.EvaluatorScoreWeights[201])
		assert.NotContains(t, result.EvaluatorScoreWeights, int64(101))
	})
}

func TestCreateEvalTargetParamDTO2DOForTemplate(t *testing.T) {
	t.Run("正常转换", func(t *testing.T) {
		param := &eval_target.CreateEvalTargetParam{
			SourceTargetID:      gptr.Of("source_id"),
			SourceTargetVersion: gptr.Of("v1"),
			BotPublishVersion:   gptr.Of("bot_v1"),
			Region:              gptr.Of("region1"),
			Env:                 gptr.Of("prod"),
		}
		param.SetEvalTargetType(gptr.Of(domain_eval_target.EvalTargetType_CozeLoopPrompt))
		param.SetBotInfoType(gptr.Of(domain_eval_target.CozeBotInfoType_DraftBot))
		result := CreateEvalTargetParamDTO2DOForTemplate(param)
		assert.NotNil(t, result)
		assert.NotNil(t, result.SourceTargetID)
		assert.Equal(t, "source_id", *result.SourceTargetID)
		assert.NotNil(t, result.SourceTargetVersion)
		assert.Equal(t, "v1", *result.SourceTargetVersion)
		assert.NotNil(t, result.BotPublishVersion)
		assert.Equal(t, "bot_v1", *result.BotPublishVersion)
		assert.NotNil(t, result.Region)
		assert.Equal(t, "region1", *result.Region)
		assert.NotNil(t, result.Env)
		assert.Equal(t, "prod", *result.Env)
		assert.NotNil(t, result.EvalTargetType)
		assert.Equal(t, entity.EvalTargetTypeLoopPrompt, *result.EvalTargetType)
		assert.NotNil(t, result.BotInfoType)
		assert.Equal(t, entity.CozeBotInfoTypeDraftBot, *result.BotInfoType)
	})

	t.Run("param为nil，返回nil", func(t *testing.T) {
		result := CreateEvalTargetParamDTO2DOForTemplate(nil)
		assert.Nil(t, result)
	})

	t.Run("转换CustomEvalTarget", func(t *testing.T) {
		customTarget := domain_eval_target.NewCustomEvalTarget()
		customTarget.SetID(gptr.Of("100"))
		customTarget.SetName(gptr.Of("custom_target"))
		customTarget.SetAvatarURL(gptr.Of("http://example.com/avatar"))
		customTarget.Ext = map[string]string{"key": "value"}
		param := &eval_target.CreateEvalTargetParam{}
		param.SetCustomEvalTarget(customTarget)
		result := CreateEvalTargetParamDTO2DOForTemplate(param)
		assert.NotNil(t, result)
		assert.NotNil(t, result.CustomEvalTarget)
		assert.NotNil(t, result.CustomEvalTarget.ID)
		assert.Equal(t, "100", *result.CustomEvalTarget.ID)
		assert.NotNil(t, result.CustomEvalTarget.Name)
		assert.Equal(t, "custom_target", *result.CustomEvalTarget.Name)
		assert.NotNil(t, result.CustomEvalTarget.AvatarURL)
		assert.Equal(t, "http://example.com/avatar", *result.CustomEvalTarget.AvatarURL)
		assert.Equal(t, map[string]string{"key": "value"}, result.CustomEvalTarget.Ext)
	})

	t.Run("EvalTargetType为nil，不设置", func(t *testing.T) {
		param := &eval_target.CreateEvalTargetParam{
			SourceTargetID: gptr.Of("source_id"),
		}
		result := CreateEvalTargetParamDTO2DOForTemplate(param)
		assert.NotNil(t, result)
		assert.Nil(t, result.EvalTargetType)
	})

	t.Run("BotInfoType为nil，不设置", func(t *testing.T) {
		param := &eval_target.CreateEvalTargetParam{
			SourceTargetID: gptr.Of("source_id"),
		}
		result := CreateEvalTargetParamDTO2DOForTemplate(param)
		assert.NotNil(t, result)
		assert.Nil(t, result.BotInfoType)
	})
}

func TestEvalConfConvert_ConvertToEntity_SetScoreWeight(t *testing.T) {
	converter := &EvalConfConvert{}
	scoreWeight1 := 0.6
	scoreWeight2 := 0.4

	t.Run("设置ScoreWeight到EvaluatorConf", func(t *testing.T) {
		cer := &expt.CreateExperimentRequest{
			EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
				domain_expt.NewEvaluatorFieldMapping(),
				domain_expt.NewEvaluatorFieldMapping(),
			},
		}
		cer.EvaluatorFieldMapping[0].SetEvaluatorVersionID(101)
		cer.EvaluatorFieldMapping[1].SetEvaluatorVersionID(102)
		cer.EvaluatorScoreWeights = map[int64]float64{
			101: scoreWeight1,
			102: scoreWeight2,
		}

		result, err := converter.ConvertToEntity(cer, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.ConnectorConf.EvaluatorsConf)
		assert.Len(t, result.ConnectorConf.EvaluatorsConf.EvaluatorConf, 2)

		// 验证 ScoreWeight 被正确设置
		for _, conf := range result.ConnectorConf.EvaluatorsConf.EvaluatorConf {
			switch conf.EvaluatorVersionID {
			case 101:
				assert.NotNil(t, conf.ScoreWeight)
				assert.Equal(t, scoreWeight1, *conf.ScoreWeight)
			case 102:
				assert.NotNil(t, conf.ScoreWeight)
				assert.Equal(t, scoreWeight2, *conf.ScoreWeight)
			}
		}
	})

	t.Run("权重为0或负数，不设置ScoreWeight", func(t *testing.T) {
		cer := &expt.CreateExperimentRequest{
			EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
				domain_expt.NewEvaluatorFieldMapping(),
			},
		}
		cer.EvaluatorFieldMapping[0].SetEvaluatorVersionID(101)
		cer.EvaluatorScoreWeights = map[int64]float64{
			101: 0.0, // 权重为0，不应该设置
		}

		result, err := converter.ConvertToEntity(cer, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		if result.ConnectorConf.EvaluatorsConf != nil && len(result.ConnectorConf.EvaluatorsConf.EvaluatorConf) > 0 {
			conf := result.ConnectorConf.EvaluatorsConf.EvaluatorConf[0]
			// 权重为0，不应该设置 ScoreWeight
			assert.Nil(t, conf.ScoreWeight)
		}
	})

	t.Run("EvaluatorConf为nil，跳过", func(t *testing.T) {
		cer := &expt.CreateExperimentRequest{
			EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
				nil, // nil项，应该跳过
			},
			EvaluatorScoreWeights: map[int64]float64{
				101: scoreWeight1,
			},
		}

		result, err := converter.ConvertToEntity(cer, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// nil项应该被跳过，不会导致panic
	})
}

func TestToExptDTO_BuildEvaluatorIDVersionItemsFromVersionRef(t *testing.T) {
	t.Run("从EvaluatorVersionRef构建EvaluatorIDVersionItems", func(t *testing.T) {
		scoreWeight := 0.7
		experiment := &entity.Experiment{
			// 没有 Evaluators，使用 EvaluatorVersionRef
			EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
				{
					EvaluatorID:        1,
					EvaluatorVersionID: 101,
				},
				{
					EvaluatorID:        2,
					EvaluatorVersionID: 102,
				},
				{
					EvaluatorID:        0, // 无效，应该跳过
					EvaluatorVersionID: 103,
				},
				{
					EvaluatorID:        3,
					EvaluatorVersionID: 0, // 无效，应该跳过
				},
			},
			EvalConf: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{
						EvaluatorConf: []*entity.EvaluatorConf{
							{
								EvaluatorVersionID: 101,
								ScoreWeight:        &scoreWeight,
							},
						},
					},
				},
			},
		}

		result := ToExptDTO(experiment)
		assert.NotNil(t, result)
		// 验证权重配置被正确填充到 ScoreWeightConfig
		if result.ScoreWeightConfig != nil {
			assert.Equal(t, scoreWeight, result.ScoreWeightConfig.EvaluatorScoreWeights[101])
		}
		// 验证 EvaluatorIDVersionList 被构建（从 evaluatorVersionIDs 构建）
		assert.NotNil(t, result.EvaluatorIDVersionList)
	})

	t.Run("优先使用Evaluators，不使用EvaluatorVersionRef", func(t *testing.T) {
		experiment := &entity.Experiment{
			Evaluators: []*entity.Evaluator{
				{
					ID: 1,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						EvaluatorID: 1,
						ID:          101,
					},
					EvaluatorType: entity.EvaluatorTypePrompt,
				},
			},
			EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
				{
					EvaluatorID:        2,
					EvaluatorVersionID: 102,
				},
			},
		}

		result := ToExptDTO(experiment)
		assert.NotNil(t, result)
		// 应该优先使用 Evaluators 的信息构建 EvaluatorIDVersionList
		assert.NotNil(t, result.EvaluatorIDVersionList)
	})
}

func TestToExptDTO_FillExptTemplateMeta(t *testing.T) {
	t.Run("填充ExptTemplateMeta", func(t *testing.T) {
		experiment := &entity.Experiment{
			ExptTemplateMeta: &entity.ExptTemplateMeta{
				ID:          100,
				WorkspaceID: 200,
				Name:        "template_name",
				Desc:        "template_desc",
				ExptType:    entity.ExptType_Offline,
			},
		}

		result := ToExptDTO(experiment)
		assert.NotNil(t, result)
		assert.NotNil(t, result.ExptTemplateMeta)
		assert.Equal(t, int64(100), gptr.Indirect(result.ExptTemplateMeta.ID))
		assert.Equal(t, int64(200), gptr.Indirect(result.ExptTemplateMeta.WorkspaceID))
		assert.Equal(t, "template_name", gptr.Indirect(result.ExptTemplateMeta.Name))
		assert.Equal(t, "template_desc", gptr.Indirect(result.ExptTemplateMeta.Desc))
		assert.Equal(t, domain_expt.ExptType_Offline, gptr.Indirect(result.ExptTemplateMeta.ExptType))
	})

	t.Run("ExptTemplateMeta为nil，不填充", func(t *testing.T) {
		experiment := &entity.Experiment{
			ExptTemplateMeta: nil,
		}

		result := ToExptDTO(experiment)
		assert.NotNil(t, result)
		assert.Nil(t, result.ExptTemplateMeta)
	})
}

func TestToTargetFieldMappingDOForTemplate_RuntimeParam(t *testing.T) {
	t.Run("设置RuntimeParam到CustomConf", func(t *testing.T) {
		rtp := &entity.RuntimeParam{
			JSONValue: gptr.Of(`{"key":"value"}`),
		}
		mapping := &domain_expt.TargetFieldMapping{
			FromEvalSet: []*domain_expt.FieldMapping{
				{
					FieldName:     gptr.Of("field1"),
					FromFieldName: gptr.Of("from1"),
				},
			},
		}

		result := toTargetFieldMappingDOForTemplate(mapping, rtp)
		assert.NotNil(t, result)
		assert.NotNil(t, result.CustomConf)
		assert.Len(t, result.CustomConf.FieldConfs, 1)
		assert.Equal(t, consts.FieldAdapterBuiltinFieldNameRuntimeParam, result.CustomConf.FieldConfs[0].FieldName)
		assert.Equal(t, `{"key":"value"}`, result.CustomConf.FieldConfs[0].Value)
	})

	t.Run("RuntimeParam为nil，不设置CustomConf", func(t *testing.T) {
		mapping := &domain_expt.TargetFieldMapping{
			FromEvalSet: []*domain_expt.FieldMapping{
				{
					FieldName:     gptr.Of("field1"),
					FromFieldName: gptr.Of("from1"),
				},
			},
		}

		result := toTargetFieldMappingDOForTemplate(mapping, nil)
		assert.NotNil(t, result)
		assert.Nil(t, result.CustomConf)
	})

	t.Run("RuntimeParam.JSONValue为空，不设置CustomConf", func(t *testing.T) {
		rtp := &entity.RuntimeParam{
			JSONValue: nil,
		}
		mapping := &domain_expt.TargetFieldMapping{}

		result := toTargetFieldMappingDOForTemplate(mapping, rtp)
		assert.NotNil(t, result)
		assert.Nil(t, result.CustomConf)
	})

	t.Run("RuntimeParam.JSONValue为空字符串，不设置CustomConf", func(t *testing.T) {
		rtp := &entity.RuntimeParam{
			JSONValue: gptr.Of(""),
		}
		mapping := &domain_expt.TargetFieldMapping{}

		result := toTargetFieldMappingDOForTemplate(mapping, rtp)
		assert.NotNil(t, result)
		assert.Nil(t, result.CustomConf)
	})
}

func TestToEvaluatorFieldMappingDoForTemplate_Complete(t *testing.T) {
	t.Run("完整转换评估器字段映射", func(t *testing.T) {
		mapping := []*domain_expt.EvaluatorFieldMapping{
			domain_expt.NewEvaluatorFieldMapping(),
		}
		mapping[0].SetEvaluatorVersionID(101)
		mapping[0].SetFromEvalSet([]*domain_expt.FieldMapping{
			{
				FieldName:     gptr.Of("input"),
				FromFieldName: gptr.Of("col1"),
				ConstValue:    gptr.Of(""),
			},
		})
		mapping[0].SetFromTarget([]*domain_expt.FieldMapping{
			{
				FieldName:     gptr.Of("output"),
				FromFieldName: gptr.Of("col2"),
			},
		})
		mapping[0].SetEvaluatorIDVersionItem(&evaluatordto.EvaluatorIDVersionItem{
			EvaluatorID:        gptr.Of(int64(1)),
			Version:            gptr.Of("v1"),
			EvaluatorVersionID: gptr.Of(int64(101)),
		})

		result := toEvaluatorFieldMappingDoForTemplate(mapping, nil)
		assert.NotNil(t, result)
		assert.Len(t, result, 1)
		conf := result[0]
		assert.Equal(t, int64(101), conf.EvaluatorVersionID)
		assert.Equal(t, int64(1), conf.EvaluatorID)
		assert.Equal(t, "v1", conf.Version)
		assert.Len(t, conf.IngressConf.EvalSetAdapter.FieldConfs, 1)
		assert.Len(t, conf.IngressConf.TargetAdapter.FieldConfs, 1)
	})

	t.Run("mapping为nil，返回nil", func(t *testing.T) {
		result := toEvaluatorFieldMappingDoForTemplate(nil, nil)
		assert.Nil(t, result)
	})

	t.Run("mapping包含nil项，跳过", func(t *testing.T) {
		mapping := []*domain_expt.EvaluatorFieldMapping{
			nil,
			domain_expt.NewEvaluatorFieldMapping(),
		}
		mapping[1].SetEvaluatorVersionID(101)

		result := toEvaluatorFieldMappingDoForTemplate(mapping, nil)
		assert.NotNil(t, result)
		assert.Len(t, result, 1)
	})
}
