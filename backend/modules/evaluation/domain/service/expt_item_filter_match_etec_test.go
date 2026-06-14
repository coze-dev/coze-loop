// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// 工厂: 构造一个含 ItemConfig 的 etec, 内嵌 turn 含给定 field
func newEtecWithConfig(itemConfig *entity.ExptItemConfig, turn *entity.Turn) *entity.ExptTurnEvalCtx {
	return &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			ItemConfig: itemConfig,
		},
		Turn: turn,
	}
}

func TestShouldRunEvaluatorByItemConfig_NilItemConfig_Run(t *testing.T) {
	etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: &entity.ExptItemEvalCtx{}}
	assert.True(t, shouldRunEvaluatorByItemConfig(context.Background(), etec, 100),
		"ItemConfig 为 nil(老路径) → 放行")
}

func TestShouldRunEvaluatorByItemConfig_NoMatchingConf_Run(t *testing.T) {
	itemConfig := &entity.ExptItemConfig{
		EvaluatorConfs: []*entity.ItemEvaluatorConf{
			{EvaluatorVersionID: 999, Alias: "other"},
		},
	}
	etec := newEtecWithConfig(itemConfig, nil)
	assert.True(t, shouldRunEvaluatorByItemConfig(context.Background(), etec, 100),
		"ItemConfig 未声明该 versionID(数据不一致) → 默认放行")
}

func TestShouldRunEvaluatorByItemConfig_FilterModeNone_Run(t *testing.T) {
	itemConfig := &entity.ExptItemConfig{
		EvaluatorConfs: []*entity.ItemEvaluatorConf{
			{EvaluatorVersionID: 100, FilterMode: 0, Filter: nil},
		},
	}
	etec := newEtecWithConfig(itemConfig, nil)
	assert.True(t, shouldRunEvaluatorByItemConfig(context.Background(), etec, 100),
		"FilterMode=None → 放行")
}

func TestShouldRunEvaluatorByItemConfig_IncludeMiss_Skip(t *testing.T) {
	itemConfig := &entity.ExptItemConfig{
		EvaluatorConfs: []*entity.ItemEvaluatorConf{
			{
				EvaluatorVersionID: 100,
				Alias:              "judge_A",
				FilterMode:         1, // Include
				Filter: &entity.ExptItemFilter{
					FilterFields: []*entity.ExptItemFilterField{
						{FieldName: "category", QueryType: "equal", Values: []string{"A"}},
					},
				},
			},
		},
	}
	turn := &entity.Turn{
		FieldDataList: []*entity.FieldData{
			{Name: "category", Content: &entity.Content{Text: gptr.Of("B")}}, // 不命中
		},
	}
	etec := newEtecWithConfig(itemConfig, turn)
	assert.False(t, shouldRunEvaluatorByItemConfig(context.Background(), etec, 100),
		"Include + 不命中 → 跳过该 evaluator")
}

func TestShouldRunEvaluatorByItemConfig_IncludeHit_Run(t *testing.T) {
	itemConfig := &entity.ExptItemConfig{
		EvaluatorConfs: []*entity.ItemEvaluatorConf{
			{
				EvaluatorVersionID: 100,
				Alias:              "judge_A",
				FilterMode:         1,
				Filter: &entity.ExptItemFilter{
					FilterFields: []*entity.ExptItemFilterField{
						{FieldName: "category", QueryType: "equal", Values: []string{"A"}},
					},
				},
			},
		},
	}
	turn := &entity.Turn{
		FieldDataList: []*entity.FieldData{
			{Name: "category", Content: &entity.Content{Text: gptr.Of("A")}},
		},
	}
	etec := newEtecWithConfig(itemConfig, turn)
	assert.True(t, shouldRunEvaluatorByItemConfig(context.Background(), etec, 100),
		"Include + 命中 → 跑")
}

func TestShouldRunEvaluatorByItemConfig_MultipleConfs_FirstWins(t *testing.T) {
	// 同 versionID 有多个 alias 实例时,当前实现只取第一个 conf 做判定(tech debt)
	itemConfig := &entity.ExptItemConfig{
		EvaluatorConfs: []*entity.ItemEvaluatorConf{
			{
				EvaluatorVersionID: 100,
				Alias:              "first",
				FilterMode:         1,
				Filter: &entity.ExptItemFilter{
					FilterFields: []*entity.ExptItemFilterField{
						{FieldName: "cat", QueryType: "equal", Values: []string{"A"}},
					},
				},
			},
			{
				EvaluatorVersionID: 100,
				Alias:              "second",
				FilterMode:         0, // None — 永远跑
			},
		},
	}
	turn := &entity.Turn{
		FieldDataList: []*entity.FieldData{
			{Name: "cat", Content: &entity.Content{Text: gptr.Of("X")}}, // 第一个 conf 不命中
		},
	}
	etec := newEtecWithConfig(itemConfig, turn)
	// 当前 MVP: 第一个 conf 判定为不命中 → 跳过(忽略第二个 conf 的"永远跑")
	// 等 alias 多实例独立判定后,这条断言应该变为 true(第二个 conf 应该跑)
	assert.False(t, shouldRunEvaluatorByItemConfig(context.Background(), etec, 100),
		"MVP: 同 versionID 取首个 conf 做判定 — tech debt 待 alias 多实例独立处理")
}
