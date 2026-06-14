// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestExptItemRefConvertor_NilInputs(t *testing.T) {
	conv := NewExptItemRefConvertor()
	assert.Nil(t, conv.DO2PO(nil))
	assert.Nil(t, conv.PO2DO(nil))
}

func TestExptItemRefConvertor_ScalarFields_NoItemConfig(t *testing.T) {
	conv := NewExptItemRefConvertor()
	do := &entity.ExptItemRef{
		ID:               101,
		SpaceID:          3,
		ExptID:           7,
		ItemID:           55,
		ItemVersionID:    0,
		EvalSetID:        9,
		EvalSetVersionID: 90,
		OrderIdx:         12,
	}

	po := conv.DO2PO(do)
	assert.NotNil(t, po)
	assert.Equal(t, do.ID, po.ID)
	assert.Equal(t, do.SpaceID, po.SpaceID)
	assert.Equal(t, do.ExptID, po.ExptID)
	assert.Equal(t, do.ItemID, po.ItemID)
	assert.Equal(t, do.ItemVersionID, po.ItemVersionID)
	assert.Equal(t, do.EvalSetID, po.EvalSetID)
	assert.Equal(t, do.EvalSetVersionID, po.EvalSetVersionID)
	assert.Equal(t, do.OrderIdx, po.OrderIdx)
	// ItemConfig 为 nil 时不写入 PO
	assert.Nil(t, po.ItemConfig)

	got := conv.PO2DO(po)
	assert.NotNil(t, got)
	assert.Equal(t, do.ID, got.ID)
	assert.Equal(t, do.EvalSetVersionID, got.EvalSetVersionID)
	assert.Equal(t, do.OrderIdx, got.OrderIdx)
	assert.Nil(t, got.ItemConfig)
}

func TestExptItemRefConvertor_ItemConfig_RoundTrip(t *testing.T) {
	conv := NewExptItemRefConvertor()
	weight := 0.8
	do := &entity.ExptItemRef{
		ID:               202,
		SpaceID:          3,
		ExptID:           8,
		ItemID:           66,
		ItemVersionID:    0,
		EvalSetID:        10,
		EvalSetVersionID: 100,
		OrderIdx:         5,
		ItemConfig: &entity.ExptItemConfig{
			EvalTargetConf: &entity.ItemTargetConf{TargetVersionID: 999},
			EvaluatorConfs: []*entity.ItemEvaluatorConf{
				{EvaluatorVersionID: 500, Alias: "judge_A", FilterMode: 1, ScoreWeight: &weight},
				{EvaluatorVersionID: 501, Alias: ""},
			},
			TurnIndexes: []int32{0, 2},
			Ext:         map[string]string{"k": "v"},
		},
	}

	po := conv.DO2PO(do)
	assert.NotNil(t, po.ItemConfig)
	assert.NotEmpty(t, *po.ItemConfig)

	got := conv.PO2DO(po)
	assert.NotNil(t, got.ItemConfig)
	// eval_target_conf
	assert.NotNil(t, got.ItemConfig.EvalTargetConf)
	assert.Equal(t, int64(999), got.ItemConfig.EvalTargetConf.TargetVersionID)
	// evaluator_conf 双实例 + alias 消歧 + score_weight 指针
	assert.Len(t, got.ItemConfig.EvaluatorConfs, 2)
	assert.Equal(t, int64(500), got.ItemConfig.EvaluatorConfs[0].EvaluatorVersionID)
	assert.Equal(t, "judge_A", got.ItemConfig.EvaluatorConfs[0].Alias)
	assert.Equal(t, int32(1), got.ItemConfig.EvaluatorConfs[0].FilterMode)
	assert.NotNil(t, got.ItemConfig.EvaluatorConfs[0].ScoreWeight)
	assert.InDelta(t, 0.8, *got.ItemConfig.EvaluatorConfs[0].ScoreWeight, 1e-9)
	assert.Equal(t, "", got.ItemConfig.EvaluatorConfs[1].Alias)
	// turn_indexes + ext
	assert.Equal(t, []int32{0, 2}, got.ItemConfig.TurnIndexes)
	assert.Equal(t, "v", got.ItemConfig.Ext["k"])
}

func TestExptItemRefConvertor_PO2DO_CorruptItemConfig_NoPanicNilConfig(t *testing.T) {
	conv := NewExptItemRefConvertor()
	bad := []byte("{not-json")
	po := &model.ExptItemRef{ID: 303, ItemConfig: &bad}

	got := conv.PO2DO(po)
	assert.NotNil(t, got)
	assert.Equal(t, int64(303), got.ID)
	// 反序列化失败时降级: item_config 置 nil, 不影响其它字段
	assert.Nil(t, got.ItemConfig)
}
