// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
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
		cfg := buildItemConfigFromSetConf(setConf)
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
		cfg := buildItemConfigFromSetConf(setConf)
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
		cfg := buildItemConfigFromSetConf(setConf)
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
}
