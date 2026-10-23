// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestBuildItemConfigFromSetConf(t *testing.T) {
	t.Run("empty_evaluator_confs_returns_empty_item_config", func(t *testing.T) {
		setConf := &entity.EvalSetConfig{
			EvalSetID:        1,
			EvalSetVersionID: 10,
			EvaluatorConfs:   nil,
		}
		cfg := buildItemConfigFromSetConf(setConf, nil)
		assert.NotNil(t, cfg)
		assert.Empty(t, cfg.EvaluatorConfs)
		assert.Nil(t, cfg.EvalTargetConf)
	})

	t.Run("one_evaluator_conf_mapped_correctly", func(t *testing.T) {
		setConf := &entity.EvalSetConfig{
			EvalSetID:        2,
			EvalSetVersionID: 20,
			EvaluatorConfs: []*entity.ExptEvaluatorConf{
				{
					EvaluatorID:        100,
					EvaluatorVersionID: 200,
					Alias:              "judge_A",
				},
			},
		}
		cfg := buildItemConfigFromSetConf(setConf, nil)
		assert.NotNil(t, cfg)
		assert.Len(t, cfg.EvaluatorConfs, 1)
		assert.Equal(t, "judge_A", cfg.EvaluatorConfs[0].Alias)
		assert.Equal(t, int64(200), cfg.EvaluatorConfs[0].EvaluatorVersionID)
		assert.Nil(t, cfg.EvalTargetConf)
	})

	t.Run("target_conf_populated_when_target_confs_present", func(t *testing.T) {
		setConf := &entity.EvalSetConfig{
			EvalSetID:        3,
			EvalSetVersionID: 30,
			TargetConfs: []*entity.ExptTargetConf{
				{
					TargetVersionID: 999,
				},
			},
			EvaluatorConfs: []*entity.ExptEvaluatorConf{
				{
					EvaluatorID:        50,
					EvaluatorVersionID: 500,
					Alias:              "judge_B",
					ScoreWeight:        gptr.Of(0.8),
				},
			},
		}
		cfg := buildItemConfigFromSetConf(setConf, nil)
		assert.NotNil(t, cfg)
		// target conf
		assert.NotNil(t, cfg.EvalTargetConf)
		assert.Equal(t, int64(999), cfg.EvalTargetConf.TargetVersionID)
		// evaluator conf
		assert.Len(t, cfg.EvaluatorConfs, 1)
		assert.Equal(t, "judge_B", cfg.EvaluatorConfs[0].Alias)
		assert.NotNil(t, cfg.EvaluatorConfs[0].ScoreWeight)
		assert.InDelta(t, 0.8, *cfg.EvaluatorConfs[0].ScoreWeight, 1e-9)
	})

	// 以下两个 case 覆盖 runModeConfig (第二参) 非 nil 的分支。此前**全部用例第二参恒传 nil**,
	// 恰好绕开 itemRunConfFromRunModeConfig, 是"空 ItemRunConf 序列化成 {} 吞掉题目级配置"
	// 这个 bug 能潜伏至今的直接原因。
	t.Run("run_conf_nil_when_run_mode_config_has_no_item_level_field", func(t *testing.T) {
		setConf := &entity.EvalSetConfig{
			EvalSetID:        4,
			EvalSetVersionID: 40,
			TargetConfs:      []*entity.ExptTargetConf{{TargetVersionID: 111}},
		}
		// 实验配了 run_mode_config, 但只配了实验级字段 (SUA 行为四项 + max_turns), 没配
		// max_run_minutes —— 这正是"用户想用题目级覆盖 SUA 行为"的典型场景。
		cfg := buildItemConfigFromSetConf(setConf, &entity.RunModeConfig{
			RunMode:                  entity.RunModeSUAMultiTurn,
			SuaMode:                  entity.SuaModeHumanLoop,
			SuaGoal:                  "把订单退掉",
			SuaPersona:               "急躁的用户",
			SuaBehavioralConstraints: "每轮只问一个点",
			SuaPETemplate:            "上轮评估: {{eval_result}}",
			MaxTurns:                 12,
		})
		assert.NotNil(t, cfg)
		assert.NotNil(t, cfg.EvalTargetConf)
		// 必须是 nil, 不能是 &entity.ItemRunConf{}: 空结构体会被写成 ext["builtin_run_conf"]="{}",
		// commercial extractItemRunConf 的 raw=="" 回退就永远走不到, 题目自带 run_conf 被整个吞掉。
		assert.Nil(t, cfg.EvalTargetConf.RunConf,
			"实验级 run_mode_config 无 item 粒度字段时 RunConf 必须为 nil, 否则序列化成 \"{}\" 吞掉题目级 run_conf")
	})

	t.Run("run_conf_carries_max_run_minutes_when_configured", func(t *testing.T) {
		setConf := &entity.EvalSetConfig{
			EvalSetID:        5,
			EvalSetVersionID: 50,
			TargetConfs:      []*entity.ExptTargetConf{{TargetVersionID: 222}},
		}
		cfg := buildItemConfigFromSetConf(setConf, &entity.RunModeConfig{
			RunMode:       entity.RunModeSUAMultiTurn,
			MaxRunMinutes: 30,
			// 实验级字段一并给上: 它们绝不该被搬进题目级 RunConf。
			SuaGoal:  "goal",
			MaxTurns: 9,
		})
		assert.NotNil(t, cfg)
		assert.NotNil(t, cfg.EvalTargetConf)
		assert.NotNil(t, cfg.EvalTargetConf.RunConf)
		assert.Equal(t, 30, cfg.EvalTargetConf.RunConf.MaxRunMinutes)
		// 实验级字段绝不搬进题目级: 否则实验级值伪装成题目级下发, runtime 无法实现题目优先。
		assert.Zero(t, cfg.EvalTargetConf.RunConf.MaxTurns)
		assert.Empty(t, cfg.EvalTargetConf.RunConf.SuaGoal)
	})
}

// TestItemRunConfFromRunModeConfig 直测实验级 RunModeConfig → 题目级 ItemRunConf 的翻译。
//
// 核心断言是"非 nil 但无任何可搬字段 ⇒ 返回 nil": entity.ItemRunConf 全字段 omitempty,
// json.Marshal(&entity.ItemRunConf{}) == "{}"; 执行侧 callTarget 的闸门只看 RunConf != nil,
// 空结构体能过闸写进 ext["builtin_run_conf"]; 而 commercial extractItemRunConf 判"未配"用的是
// raw == "", "{}" 非空 ⇒ 「Ext 空则回退读题目自带 run_conf」这条回退永远走不到, 题目级
// sua_* / max_turns / fixed_query_list 全部到不了 runtime, 「题目级优先」语义被静默打掉。
func TestItemRunConfFromRunModeConfig(t *testing.T) {
	t.Run("nil_config_returns_nil", func(t *testing.T) {
		assert.Nil(t, itemRunConfFromRunModeConfig(nil))
	})

	t.Run("no_movable_field_returns_nil_not_empty_struct", func(t *testing.T) {
		// 只有实验级字段 (故意不搬) → 无字段可搬。
		rc := itemRunConfFromRunModeConfig(&entity.RunModeConfig{
			RunMode:                  entity.RunModeSUAMultiTurn,
			SuaMode:                  entity.SuaModeLoop,
			SuaModelID:               123,
			SuaModelName:             "doubao",
			SuaGoal:                  "goal",
			SuaPersona:               "persona",
			SuaBehavioralConstraints: "constraints",
			SuaPETemplate:            "{{eval_result}}",
			MaxTurns:                 8,
		})
		assert.Nil(t, rc, "无 item 粒度字段可搬时必须返回 nil (空结构体会序列化成 \"{}\")")
	})

	t.Run("empty_config_returns_nil", func(t *testing.T) {
		assert.Nil(t, itemRunConfFromRunModeConfig(&entity.RunModeConfig{}))
	})

	t.Run("non_positive_max_run_minutes_returns_nil", func(t *testing.T) {
		assert.Nil(t, itemRunConfFromRunModeConfig(&entity.RunModeConfig{MaxRunMinutes: 0}))
		assert.Nil(t, itemRunConfFromRunModeConfig(&entity.RunModeConfig{MaxRunMinutes: -1}))
	})

	t.Run("max_run_minutes_moved", func(t *testing.T) {
		rc := itemRunConfFromRunModeConfig(&entity.RunModeConfig{MaxRunMinutes: 45})
		assert.NotNil(t, rc)
		assert.Equal(t, 45, rc.MaxRunMinutes)
		// 只搬 max_run_minutes, 其余保持零值。
		assert.Zero(t, rc.MaxTurns)
		assert.Empty(t, rc.SuaPersona)
		assert.Nil(t, rc.FixedQueryList)
	})

	t.Run("empty_struct_marshals_to_curly_braces_which_is_why_nil_matters", func(t *testing.T) {
		// 这一条把"为什么必须返回 nil"的前提锁死: 空 ItemRunConf 的 JSON 是**非空串** "{}",
		// 一旦上游返回空结构体, 下游按 raw == "" 判"未配"的回退就失效。
		b, err := json.Marshal(&entity.ItemRunConf{})
		assert.NoError(t, err)
		assert.Equal(t, "{}", string(b))
	})
}
