// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestToExptDTO_MultiSetReadView_GetPath 验证新实验 (MultiSetConfig) 读视图回显 + §2 主集降级投影。
func TestToExptDTO_MultiSetReadView_GetPath(t *testing.T) {
	weightA := 0.6
	experiment := &entity.Experiment{
		ID:                1001,
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalSetID:         20, // 主集标签
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{
					EvalSetID:        10,
					EvalSetVersionID: 110,
					EvaluatorConfs: []*entity.ExptEvaluatorConf{
						{EvaluatorID: 1, EvaluatorVersionID: 101, Alias: "judge_A", ScoreWeight: gptr.Of(0.3)},
					},
				},
				{
					EvalSetID:        20, // == experiment.EvalSetID → 主集
					EvalSetVersionID: 220,
					TargetConfs: []*entity.ExptTargetConf{
						{TargetID: 5, TargetVersionID: 55, FieldMapping: []*entity.FieldConf{{FieldName: "in", FromField: "q"}}},
					},
					EvaluatorConfs: []*entity.ExptEvaluatorConf{
						// 同 version 两实例: 默认实例 (alias=='') 应被选中投影
						{EvaluatorID: 2, EvaluatorVersionID: 201, Alias: "judge_B", ScoreWeight: gptr.Of(0.9)},
						{EvaluatorID: 2, EvaluatorVersionID: 201, Alias: "", ScoreWeight: &weightA,
							FromEvalSet: []*entity.FieldConf{{FieldName: "input", FromField: "question"}}},
					},
				},
			},
		},
		// enrichment 已填充的读视图字段
		TotalItemCount: 42,
		EvalSetDetails: []*entity.ExptEvalSetDetail{
			{EvalSetID: 10, EvalSetVersionID: 110, IsPrimary: false, ItemCount: 12},
			{EvalSetID: 20, EvalSetVersionID: 220, IsPrimary: true, ItemCount: 30,
				EvalSet: &entity.EvaluationSet{ID: 20, Name: "primary-set"}},
		},
	}

	res := ToExptDTO(experiment)
	assert.NotNil(t, res)

	// §3 读视图回显
	assert.Equal(t, int64(42), res.GetTotalItemCount())
	assert.Len(t, res.EvalSetDetails, 2)
	assert.Equal(t, int64(10), res.EvalSetDetails[0].GetEvalSetID())
	assert.Equal(t, int32(12), res.EvalSetDetails[0].GetItemCount())
	assert.False(t, res.EvalSetDetails[0].GetIsPrimary())
	assert.True(t, res.EvalSetDetails[1].GetIsPrimary())
	assert.Equal(t, int32(30), res.EvalSetDetails[1].GetItemCount())
	assert.NotNil(t, res.EvalSetDetails[1].EvalSet) // Get 路径填充详情

	// §2 主集降级投影: eval_set_id/version 取主集 (set 20 / ver 220)
	assert.Equal(t, int64(20), res.GetEvalSetID())
	assert.Equal(t, int64(220), res.GetEvalSetVersionID())

	// target_field_mapping / target 取主集 target_confs[0]
	assert.NotNil(t, res.TargetFieldMapping)
	assert.Len(t, res.TargetFieldMapping.FromEvalSet, 1)

	// evaluator_field_mapping: 主集 version 201 取默认实例 (alias=='', 带 from_eval_set)
	var got201 bool
	for _, em := range res.EvaluatorFieldMapping {
		if em.GetEvaluatorVersionID() == 201 {
			got201 = true
			assert.Len(t, em.FromEvalSet, 1)
		}
	}
	assert.True(t, got201, "evaluator_field_mapping 应投影主集 version 201 的默认实例")

	// score_weight_config: 主集 version 201 取默认实例权重 0.6
	assert.NotNil(t, res.ScoreWeightConfig)
	assert.Equal(t, weightA, res.ScoreWeightConfig.EvaluatorScoreWeights[201])
}

// TestToExptDTO_MultiSetReadView_ListPath List 路径 (eval_set_details 只 id/count, 无详情) 仍正常回显。
func TestToExptDTO_MultiSetReadView_ListPath(t *testing.T) {
	experiment := &entity.Experiment{
		ID:                1002,
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalSetID:         10,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{EvalSetID: 10, EvalSetVersionID: 110},
			},
		},
		TotalItemCount: 0, // 首跑前
		EvalSetDetails: []*entity.ExptEvalSetDetail{
			{EvalSetID: 10, EvalSetVersionID: 110, IsPrimary: true, ItemCount: 0}, // List 不填 EvalSet
		},
	}

	res := ToExptDTO(experiment)
	assert.NotNil(t, res)
	assert.Equal(t, int64(0), res.GetTotalItemCount())
	assert.Len(t, res.EvalSetDetails, 1)
	assert.Nil(t, res.EvalSetDetails[0].EvalSet) // List 路径不含详情
	assert.Equal(t, int64(10), res.GetEvalSetID())
	assert.Equal(t, int64(110), res.GetEvalSetVersionID())
}

// TestToExptDTO_SingleSet_NoReadView 老实验 (SingleSet) 不受影响: 不回显新字段, 不做降级投影。
func TestToExptDTO_SingleSet_NoReadView(t *testing.T) {
	experiment := &entity.Experiment{
		ID:                1003,
		EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet,
		EvalSetID:         7,
		EvalSetVersionID:  77,
		EvalConf:          &entity.EvaluationConfiguration{},
	}

	res := ToExptDTO(experiment)
	assert.NotNil(t, res)
	assert.Nil(t, res.TotalItemCount)
	assert.Empty(t, res.EvalSetDetails)
	// 老字段照旧 (来自 flat 列)
	assert.Equal(t, int64(7), res.GetEvalSetID())
	assert.Equal(t, int64(77), res.GetEvalSetVersionID())
}
