// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TestExptTemplateConverter_DO2PO_CronActivate(t *testing.T) {
	t.Parallel()

	converter := NewExptTemplateConverter()
	createdAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	updatedAt := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC).UnixMilli()

	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          11,
			WorkspaceID: 22,
			Name:        "nightly-template",
			Desc:        "desc",
			ExptType:    entity.ExptType_Online,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:        33,
			EvalSetVersionID: 44,
			TargetID:         55,
			TargetVersionID:  66,
			TargetType:       entity.EvalTargetTypeCozeBot,
		},
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{UserID: gptr.Of("creator")},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of("updater")},
			CreatedAt: gptr.Of(createdAt),
			UpdatedAt: gptr.Of(updatedAt),
		},
		ExptInfo: &entity.ExptInfo{
			CreatedExptCount:    2,
			LatestExptID:        99,
			LatestExptStatus:    entity.ExptStatus_Processing,
			LatestExptStartTime: 123456,
			CronActivate:        true,
		},
	}

	po, err := converter.DO2PO(template)
	assert.NoError(t, err)
	assert.NotNil(t, po)
	assert.True(t, po.CronActivate)
	assert.Equal(t, int32(entity.ExptType_Online), po.ExptType)
	assert.Equal(t, "creator", po.CreatedBy)
	assert.Equal(t, "updater", po.UpdatedBy)
}

func TestExptTemplateConverter_PO2DO_CronActivate(t *testing.T) {
	t.Parallel()

	converter := NewExptTemplateConverter()
	createdAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	refs := []*model.ExptTemplateEvaluatorRef{{EvaluatorID: 7, EvaluatorVersionID: 8}}

	t.Run("table field fills expt info when json is empty", func(t *testing.T) {
		po := &model.ExptTemplate{
			ID:               1,
			SpaceID:          2,
			Name:             "tpl",
			Description:      "desc",
			EvalSetID:        3,
			EvalSetVersionID: 4,
			TargetID:         5,
			TargetVersionID:  6,
			TargetType:       int64(entity.EvalTargetTypeCozeBot),
			ExptType:         int32(entity.ExptType_Offline),
			CronActivate:     true,
			CreatedBy:        "creator",
			UpdatedBy:        "updater",
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		}

		do, err := converter.PO2DO(po, refs)
		assert.NoError(t, err)
		assert.NotNil(t, do)
		assert.NotNil(t, do.ExptInfo)
		assert.True(t, do.ExptInfo.CronActivate)
		assert.Equal(t, []int64{8}, do.GetEvaluatorVersionIds())
	})

	t.Run("table field overrides expt info json", func(t *testing.T) {
		exptInfoJSON := []byte(`{"created_expt_count":1,"cron_activate":false}`)
		po := &model.ExptTemplate{
			ID:               10,
			SpaceID:          20,
			Name:             "tpl-json",
			Description:      "desc",
			EvalSetID:        30,
			EvalSetVersionID: 40,
			TargetID:         50,
			TargetVersionID:  60,
			TargetType:       int64(entity.EvalTargetTypeCozeBot),
			ExptType:         int32(entity.ExptType_Offline),
			CronActivate:     true,
			ExptInfo:         &exptInfoJSON,
			CreatedBy:        "creator",
			UpdatedBy:        "updater",
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
			DeletedAt:        gorm.DeletedAt{Time: createdAt, Valid: true},
		}

		do, err := converter.PO2DO(po, refs)
		assert.NoError(t, err)
		assert.NotNil(t, do)
		assert.NotNil(t, do.ExptInfo)
		assert.True(t, do.ExptInfo.CronActivate)
		assert.Equal(t, int64(1), do.ExptInfo.CreatedExptCount)
		assert.NotNil(t, do.BaseInfo)
		assert.NotNil(t, do.BaseInfo.DeletedAt)
	})

	t.Run("template conf populates source and score weight flag", func(t *testing.T) {
		weight := 0.5
		templateConf := &entity.ExptTemplateConfiguration{
			ItemConcurNum: gptr.Of(3),
			ConnectorConf: entity.Connector{
				EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{
					EvaluatorID:        7,
					EvaluatorVersionID: 8,
					Version:            "v8",
					ScoreWeight:        &weight,
				}}},
			},
			ExptSource: &entity.ExptSource{SourceType: entity.SourceType(1), SourceID: "pipe-1"},
		}
		templateConfJSON, err := json.Marshal(templateConf)
		assert.NoError(t, err)
		po := &model.ExptTemplate{
			ID:               88,
			SpaceID:          99,
			Name:             "tpl-conf",
			Description:      "desc",
			EvalSetID:        30,
			EvalSetVersionID: 40,
			TargetID:         50,
			TargetVersionID:  60,
			TargetType:       int64(entity.EvalTargetTypeCozeBot),
			ExptType:         int32(entity.ExptType_Offline),
			TemplateConf:     &templateConfJSON,
			CreatedBy:        "creator",
			UpdatedBy:        "updater",
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		}

		do, err := converter.PO2DO(po, refs)
		assert.NoError(t, err)
		if assert.NotNil(t, do) && assert.NotNil(t, do.FieldMappingConfig) && assert.NotNil(t, do.FieldMappingConfig.ItemConcurNum) {
			assert.Equal(t, 3, *do.FieldMappingConfig.ItemConcurNum)
		}
		if assert.NotNil(t, do.TemplateConf) && assert.NotNil(t, do.TemplateConf.ConnectorConf.EvaluatorsConf) {
			assert.True(t, do.TemplateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight)
			assert.Equal(t, &weight, do.TemplateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].ScoreWeight)
		}
		if assert.NotNil(t, do.ExptSource) {
			assert.Equal(t, entity.SourceType(1), do.ExptSource.SourceType)
			assert.Equal(t, "pipe-1", do.ExptSource.SourceID)
		}
	})

	t.Run("empty table field keeps nil expt info", func(t *testing.T) {
		po := &model.ExptTemplate{
			ID:               100,
			SpaceID:          200,
			Name:             "tpl-empty",
			Description:      "desc",
			EvalSetID:        300,
			EvalSetVersionID: 400,
			TargetID:         500,
			TargetVersionID:  600,
			TargetType:       int64(entity.EvalTargetTypeCozeBot),
			ExptType:         int32(entity.ExptType_Offline),
			CronActivate:     false,
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		}

		do, err := converter.PO2DO(po, nil)
		assert.NoError(t, err)
		assert.NotNil(t, do)
		assert.Nil(t, do.ExptInfo)
	})

	t.Run("template conf rebuilds target runtime and evaluator mappings", func(t *testing.T) {
		zero := 0.0
		templateConf := &entity.ExptTemplateConfiguration{
			ItemConcurNum: gptr.Of(6),
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
					EvaluatorID:        11,
					EvaluatorVersionID: 12,
					Version:            "skip",
				}, {
					EvaluatorID:        21,
					EvaluatorVersionID: 22,
					Version:            "v22",
					ScoreWeight:        &zero,
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
				}}},
			},
		}
		templateConfJSON, err := json.Marshal(templateConf)
		assert.NoError(t, err)
		po := &model.ExptTemplate{
			ID:               101,
			SpaceID:          202,
			Name:             "tpl-mapping",
			Description:      "desc",
			EvalSetID:        303,
			EvalSetVersionID: 404,
			TargetID:         505,
			TargetVersionID:  606,
			TargetType:       int64(entity.EvalTargetTypeCozeBot),
			ExptType:         int32(entity.ExptType_Offline),
			TemplateConf:     &templateConfJSON,
			CreatedBy:        "creator",
			UpdatedBy:        "updater",
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		}

		do, err := converter.PO2DO(po, nil)
		assert.NoError(t, err)
		if assert.NotNil(t, do) && assert.NotNil(t, do.FieldMappingConfig) {
			if assert.NotNil(t, do.FieldMappingConfig.TargetRuntimeParam) && assert.NotNil(t, do.FieldMappingConfig.TargetRuntimeParam.JSONValue) {
				assert.Equal(t, `{"debug":true}`, *do.FieldMappingConfig.TargetRuntimeParam.JSONValue)
			}
			if assert.NotNil(t, do.FieldMappingConfig.TargetFieldMapping) && assert.Len(t, do.FieldMappingConfig.TargetFieldMapping.FromEvalSet, 2) {
				assert.Equal(t, "input", do.FieldMappingConfig.TargetFieldMapping.FromEvalSet[0].FieldName)
				assert.Equal(t, "question", do.FieldMappingConfig.TargetFieldMapping.FromEvalSet[0].FromFieldName)
				assert.Equal(t, "const_field", do.FieldMappingConfig.TargetFieldMapping.FromEvalSet[1].FieldName)
				assert.Equal(t, "constant", do.FieldMappingConfig.TargetFieldMapping.FromEvalSet[1].ConstValue)
			}
			if assert.Len(t, do.FieldMappingConfig.EvaluatorFieldMapping, 1) {
				mapping := do.FieldMappingConfig.EvaluatorFieldMapping[0]
				assert.Equal(t, int64(21), mapping.EvaluatorID)
				assert.Equal(t, int64(22), mapping.EvaluatorVersionID)
				assert.Equal(t, "v22", mapping.Version)
				if assert.Len(t, mapping.FromEvalSet, 1) {
					assert.Equal(t, "eval_field", mapping.FromEvalSet[0].FieldName)
					assert.Equal(t, "dataset_field", mapping.FromEvalSet[0].FromFieldName)
				}
				if assert.Len(t, mapping.FromTarget, 1) {
					assert.Equal(t, "target_field", mapping.FromTarget[0].FieldName)
					assert.Equal(t, "const-target", mapping.FromTarget[0].ConstValue)
				}
			}
		}
		if assert.NotNil(t, do) && assert.NotNil(t, do.TemplateConf) && assert.NotNil(t, do.TemplateConf.ConnectorConf.EvaluatorsConf) {
			assert.False(t, do.TemplateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight)
		}
	})
}

func TestExptTemplateConverter_DO2PO_DefaultsAndJSON(t *testing.T) {
	t.Parallel()

	converter := NewExptTemplateConverter()

	t.Run("nil nested fields keep defaults", func(t *testing.T) {
		po, err := converter.DO2PO(&entity.ExptTemplate{})
		assert.NoError(t, err)
		if assert.NotNil(t, po) {
			assert.Zero(t, po.ID)
			assert.Zero(t, po.SpaceID)
			assert.Zero(t, po.EvalSetID)
			assert.Zero(t, po.TargetID)
			assert.False(t, po.CronActivate)
			assert.Nil(t, po.TemplateConf)
			assert.Nil(t, po.ExptInfo)
		}
	})

	t.Run("template conf and expt info are marshaled", func(t *testing.T) {
		weight := 0.4
		po, err := converter.DO2PO(&entity.ExptTemplate{
			TemplateConf: &entity.ExptTemplateConfiguration{
				ItemConcurNum: gptr.Of(2),
				ConnectorConf: entity.Connector{
					EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{
						EvaluatorVersionID: 8,
						ScoreWeight:        &weight,
					}}},
				},
				ExptSource: &entity.ExptSource{SourceType: entity.SourceType(1), SourceID: "source-1"},
			},
			ExptInfo: &entity.ExptInfo{CronActivate: true, CreatedExptCount: 3},
		})
		assert.NoError(t, err)
		if assert.NotNil(t, po) {
			assert.True(t, po.CronActivate)
			assert.NotNil(t, po.TemplateConf)
			assert.NotNil(t, po.ExptInfo)
		}
	})
}

func TestExptTemplateConverter_PO2DO_JSONErrorsAndRefs(t *testing.T) {
	t.Parallel()

	converter := NewExptTemplateConverter()
	createdAt := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	t.Run("invalid template conf json returns error", func(t *testing.T) {
		badJSON := []byte(`{"bad":`)
		_, err := converter.PO2DO(&model.ExptTemplate{
			ID:           1,
			SpaceID:      2,
			Name:         "tpl",
			Description:  "desc",
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			TemplateConf: &badJSON,
		}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ExptTemplateConfiguration json unmarshal fail")
	})

	t.Run("invalid expt info json returns error", func(t *testing.T) {
		templateConfJSON, err := json.Marshal(&entity.ExptTemplateConfiguration{})
		assert.NoError(t, err)
		badJSON := []byte(`{"bad":`)
		_, err = converter.PO2DO(&model.ExptTemplate{
			ID:           1,
			SpaceID:      2,
			Name:         "tpl",
			Description:  "desc",
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			TemplateConf: &templateConfJSON,
			ExptInfo:     &badJSON,
		}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ExptInfo json unmarshal fail")
	})

	t.Run("refs populate version ref ids and base info fields", func(t *testing.T) {
		templateConfJSON, err := json.Marshal(&entity.ExptTemplateConfiguration{})
		assert.NoError(t, err)
		refs := []*model.ExptTemplateEvaluatorRef{{EvaluatorID: 7, EvaluatorVersionID: 8}, {EvaluatorID: 9, EvaluatorVersionID: 10}}
		do, err := converter.PO2DO(&model.ExptTemplate{
			ID:           1,
			SpaceID:      2,
			Name:         "tpl",
			Description:  "desc",
			CreatedBy:    "creator",
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			TemplateConf: &templateConfJSON,
		}, refs)
		assert.NoError(t, err)
		if assert.NotNil(t, do) && assert.NotNil(t, do.TripleConfig) {
			assert.Equal(t, []int64{8, 10}, do.TripleConfig.EvaluatorVersionIds)
			if assert.Len(t, do.TripleConfig.EvaluatorIDVersionItems, 2) {
				assert.Equal(t, int64(7), do.TripleConfig.EvaluatorIDVersionItems[0].EvaluatorID)
				assert.Equal(t, int64(8), do.TripleConfig.EvaluatorIDVersionItems[0].EvaluatorVersionID)
			}
			assert.Len(t, do.EvaluatorVersionRef, 2)
			assert.Nil(t, do.BaseInfo.UpdatedBy)
		}
	})
}

func TestExptTemplateEvaluatorRefConverter_DO2PO(t *testing.T) {
	t.Parallel()

	converter := NewExptTemplateEvaluatorRefConverter()
	assert.Empty(t, converter.DO2PO(nil))

	got := converter.DO2PO([]*entity.ExptTemplateEvaluatorRef{{
		ID:                 1,
		SpaceID:            2,
		ExptTemplateID:     3,
		EvaluatorID:        4,
		EvaluatorVersionID: 5,
	}, {
		ID:                 6,
		SpaceID:            7,
		ExptTemplateID:     8,
		EvaluatorID:        9,
		EvaluatorVersionID: 10,
	}})
	if assert.Len(t, got, 2) {
		assert.Equal(t, int64(1), got[0].ID)
		assert.Equal(t, int64(2), got[0].SpaceID)
		assert.Equal(t, int64(3), got[0].ExptTemplateID)
		assert.Equal(t, int64(4), got[0].EvaluatorID)
		assert.Equal(t, int64(5), got[0].EvaluatorVersionID)
		assert.Equal(t, int64(6), got[1].ID)
		assert.Equal(t, int64(7), got[1].SpaceID)
		assert.Equal(t, int64(8), got[1].ExptTemplateID)
		assert.Equal(t, int64(9), got[1].EvaluatorID)
		assert.Equal(t, int64(10), got[1].EvaluatorVersionID)
	}
}
