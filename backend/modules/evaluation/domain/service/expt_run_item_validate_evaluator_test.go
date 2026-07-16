// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// evaluatorWithVer 构造一个带 versionID 的 Prompt 评估器 (SingleSet 老实验期望集用)。
// 复用包内已有的 newPromptEvaluator(name, evaluatorID, versionID)。
func evaluatorWithVer(versionID int64) *entity.Evaluator {
	return newPromptEvaluator("", versionID, versionID)
}

// baseEvalConf 构造一个非空的 EvalConf, 让 validateEvaluatorResultsComplete 的前置守卫 (EvalConf/
// ConnectorConf.EvaluatorsConf 非空) 通过, 进入真正的完整性校验分支。
func baseEvalConf() *entity.EvaluationConfiguration {
	return &entity.EvaluationConfiguration{
		ConnectorConf: entity.Connector{
			EvaluatorsConf: &entity.EvaluatorsConf{},
		},
	}
}

func newEvalCtx(expt *entity.Experiment, itemConfig *entity.ExptItemConfig) *entity.ExptTurnEvalCtx {
	return &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt:       expt,
			ItemConfig: itemConfig,
		},
	}
}

func TestValidateEvaluatorResultsComplete_MultiSet_OnlyOwnSetEvaluators(t *testing.T) {
	// 场景: MultiSetConfig 实验, set1 的 item 只绑定 set1 的 evaluator(version=100)。
	// expt.Evaluators 是并集(含 set2 的 version=200), 但校验应只看 ItemConfig.EvaluatorConfs。
	// item 已产出 version=100 的成功 record -> 不应判 missing (不能因为缺 version=200 而 fail)。
	e := &ExptItemEvalCtxExecutor{}
	expt := &entity.Experiment{
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalConf:          baseEvalConf(),
		Evaluators:        []*entity.Evaluator{evaluatorWithVer(100), evaluatorWithVer(200)}, // 并集
	}
	itemConfig := &entity.ExptItemConfig{
		EvaluatorConfs: []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 100}}, // 本集只配 100
	}
	etec := newEvalCtx(expt, itemConfig)
	result := &entity.ExptTurnRunResult{
		EvaluatorResults: []*entity.EvaluatorRecord{
			{ID: 1, EvaluatorVersionID: 100, Status: entity.EvaluatorRunStatusSuccess},
		},
	}

	err := e.validateEvaluatorResultsComplete(etec, result)
	assert.NoError(t, err)
}

func TestValidateEvaluatorResultsComplete_MultiSet_EmptyItemConfig_ExpectsZero(t *testing.T) {
	// 场景: MultiSetConfig 实验, 本评测集未配任何 evaluator (ItemConfig 为 nil / EvaluatorConfs 空)。
	// 期望 0 个评估器 -> 合法, 直接放行, 不因 expt.Evaluators 的并集而误判 missing。
	e := &ExptItemEvalCtxExecutor{}
	expt := &entity.Experiment{
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalConf:          baseEvalConf(),
		Evaluators:        []*entity.Evaluator{evaluatorWithVer(100), evaluatorWithVer(200)},
	}
	result := &entity.ExptTurnRunResult{} // 无任何 evaluator record

	// ItemConfig == nil
	etecNil := newEvalCtx(expt, nil)
	assert.NoError(t, e.validateEvaluatorResultsComplete(etecNil, result))

	// ItemConfig.EvaluatorConfs 空
	etecEmpty := newEvalCtx(expt, &entity.ExptItemConfig{EvaluatorConfs: nil})
	assert.NoError(t, e.validateEvaluatorResultsComplete(etecEmpty, result))
}

func TestValidateEvaluatorResultsComplete_MultiSet_SkippedRecordSatisfies(t *testing.T) {
	// 场景: MultiSetConfig 实验, 本集绑定 version=100, 但 filter 不命中 -> 落 Skipped 占位 record。
	// Skipped 占位应被视为"已满足", 不能判 missing。
	e := &ExptItemEvalCtxExecutor{}
	expt := &entity.Experiment{
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalConf:          baseEvalConf(),
		Evaluators:        []*entity.Evaluator{evaluatorWithVer(100)},
	}
	itemConfig := &entity.ExptItemConfig{
		EvaluatorConfs: []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 100}},
	}
	etec := newEvalCtx(expt, itemConfig)
	result := &entity.ExptTurnRunResult{
		EvaluatorResults: []*entity.EvaluatorRecord{
			{ID: 9, EvaluatorVersionID: 100, Status: entity.EvaluatorRunStatusSkipped},
		},
	}

	assert.NoError(t, e.validateEvaluatorResultsComplete(etec, result))
}

func TestValidateEvaluatorResultsComplete_MultiSet_AliasDoubleKey(t *testing.T) {
	// 场景: 同一 versionID 两个 alias 实例, 命中判定必须按 (versionID, alias) 双键。
	// 只产出了 alias="a" 的 record, alias="b" 缺失 -> 应判 missing。
	e := &ExptItemEvalCtxExecutor{}
	expt := &entity.Experiment{
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalConf:          baseEvalConf(),
		Evaluators:        []*entity.Evaluator{evaluatorWithVer(100)},
	}
	itemConfig := &entity.ExptItemConfig{
		EvaluatorConfs: []*entity.ItemEvaluatorConf{
			{EvaluatorVersionID: 100, Alias: "a"},
			{EvaluatorVersionID: 100, Alias: "b"},
		},
	}
	etec := newEvalCtx(expt, itemConfig)

	// 只有 alias="a" 命中 -> missing 应包含 alias="b"
	resultPartial := &entity.ExptTurnRunResult{
		EvaluatorResults: []*entity.EvaluatorRecord{
			{ID: 1, EvaluatorVersionID: 100, Alias: "a", Status: entity.EvaluatorRunStatusSuccess},
		},
	}
	assert.Error(t, e.validateEvaluatorResultsComplete(etec, resultPartial))

	// 两个 alias 都命中 -> 通过
	resultFull := &entity.ExptTurnRunResult{
		EvaluatorResults: []*entity.EvaluatorRecord{
			{ID: 1, EvaluatorVersionID: 100, Alias: "a", Status: entity.EvaluatorRunStatusSuccess},
			{ID: 2, EvaluatorVersionID: 100, Alias: "b", Status: entity.EvaluatorRunStatusSuccess},
		},
	}
	assert.NoError(t, e.validateEvaluatorResultsComplete(etec, resultFull))
}

func TestValidateEvaluatorResultsComplete_SingleSet_UsesExptEvaluators(t *testing.T) {
	// 回归: SingleSet 老实验 (ItemConfig 恒 nil) 期望集仍走 expt.Evaluators, 按 versionID 单键匹配。
	e := &ExptItemEvalCtxExecutor{}
	expt := &entity.Experiment{
		EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet,
		EvalConf:          baseEvalConf(),
		Evaluators:        []*entity.Evaluator{evaluatorWithVer(100), evaluatorWithVer(200)},
	}
	etec := newEvalCtx(expt, nil)

	// 缺 version=200 -> missing
	resultPartial := &entity.ExptTurnRunResult{
		EvaluatorResults: []*entity.EvaluatorRecord{
			{ID: 1, EvaluatorVersionID: 100, Status: entity.EvaluatorRunStatusSuccess},
		},
	}
	assert.Error(t, e.validateEvaluatorResultsComplete(etec, resultPartial))

	// 两个都在 -> 通过
	resultFull := &entity.ExptTurnRunResult{
		EvaluatorResults: []*entity.EvaluatorRecord{
			{ID: 1, EvaluatorVersionID: 100, Status: entity.EvaluatorRunStatusSuccess},
			{ID: 2, EvaluatorVersionID: 200, Status: entity.EvaluatorRunStatusSuccess},
		},
	}
	assert.NoError(t, e.validateEvaluatorResultsComplete(etec, resultFull))
}

func TestValidateEvaluatorResultsComplete_NilGuards(t *testing.T) {
	e := &ExptItemEvalCtxExecutor{}
	// nil etec / nil expt / nil result / asyncAbort -> 一律放行
	assert.NoError(t, e.validateEvaluatorResultsComplete(nil, &entity.ExptTurnRunResult{}))
	assert.NoError(t, e.validateEvaluatorResultsComplete(newEvalCtx(nil, nil), &entity.ExptTurnRunResult{}))
	assert.NoError(t, e.validateEvaluatorResultsComplete(newEvalCtx(&entity.Experiment{EvalConf: baseEvalConf()}, nil), nil))
	assert.NoError(t, e.validateEvaluatorResultsComplete(
		newEvalCtx(&entity.Experiment{EvalConf: baseEvalConf()}, nil),
		&entity.ExptTurnRunResult{AsyncAbort: true},
	))
}
