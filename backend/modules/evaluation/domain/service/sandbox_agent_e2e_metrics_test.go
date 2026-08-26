// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configermocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// ============================================================================
// buildSandboxAgentE2ETags
// ============================================================================

func TestBuildSandboxAgentE2ETags_NilInputs(t *testing.T) {
	// etec 为 nil → 返回零值
	got := buildSandboxAgentE2ETags(nil)
	assert.Equal(t, int64(0), got.SpaceID)
	assert.Equal(t, int64(0), got.TurnID)

	// Event 为 nil (但嵌入 ExptItemEvalCtx 非 nil) → 返回零值
	got2 := buildSandboxAgentE2ETags(&entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{},
	})
	assert.Equal(t, int64(0), got2.ExperimentID)

	// Turn / EvalSetItem 为 nil → TurnID / ItemID 为 0, 其余字段仍拼装
	etec := &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{SpaceID: 4, ExptID: 1, ExptRunID: 2},
			Expt:  &entity.Experiment{ID: 1},
		},
	}
	got3 := buildSandboxAgentE2ETags(etec)
	assert.Equal(t, int64(4), got3.SpaceID)
	assert.Equal(t, int64(1), got3.ExperimentID)
	assert.Equal(t, int64(2), got3.ExperimentRunID)
	assert.Equal(t, int64(0), got3.TurnID)
	assert.Equal(t, int64(0), got3.ItemID)
}

func TestBuildSandboxAgentE2ETags_FullPopulated(t *testing.T) {
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 555},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Event:            &entity.ExptItemEvalEvent{SpaceID: 4, ExptID: 100, ExptRunID: 200},
			EvalSetItem:      &entity.EvaluationSetItem{ItemID: 77, ItemKey: "ik-1", EvaluationSetID: 888},
			EvalSetVersionID: 999,
			Expt: &entity.Experiment{
				ID:        100,
				EvalSetID: 888,
				Target: &entity.EvalTarget{
					ID:             33,
					SourceTargetID: "app-xyz",
					EvalTargetVersion: &entity.EvalTargetVersion{
						EvalTargetType: entity.EvalTargetTypeSandboxAgent,
						SandboxAgent:   &entity.SandboxAgent{Name: "agent-a"},
					},
				},
			},
		},
	}
	got := buildSandboxAgentE2ETags(etec)

	assert.Equal(t, int64(4), got.SpaceID)
	assert.Equal(t, int64(100), got.ExperimentID)
	assert.Equal(t, int64(200), got.ExperimentRunID)
	assert.Equal(t, int64(77), got.ItemID)
	assert.Equal(t, int64(555), got.TurnID)
	assert.Equal(t, int64(888), got.DatasetID)
	assert.Equal(t, int64(999), got.DatasetVersion)
	assert.Equal(t, int64(33), got.TargetID)
	assert.Equal(t, "ik-1", got.ItemKey)
	assert.Equal(t, "agent-a", got.AgentName)
	assert.Equal(t, "app-xyz", got.ApplicationID)
	// DatasetKey 未在 Expt 中挂载 EvalSet 因此为空(交由 emit 层占位)
	assert.Equal(t, "", got.DatasetKey)
}

// ============================================================================
// emitSandboxAgentE2EStarted — 全部 gate
// ============================================================================

func newSandboxE2EExecutor(t *testing.T, ctrl *gomock.Controller) (*ExptItemEvalCtxExecutor, *metricsmocks.MockSandboxAgentMetrics) {
	t.Helper()
	m := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
	exec := &ExptItemEvalCtxExecutor{
		sandboxAgentMetrics: m,
	}
	return exec, m
}

func sandboxTurnCtx(retryTimes int, asyncReport bool, asyncEvaluator bool) *entity.ExptTurnEvalCtx {
	return &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{
				SpaceID:                     4,
				ExptID:                      100,
				ExptRunID:                   200,
				RetryTimes:                  retryTimes,
				AsyncReportTrigger:          asyncReport,
				AsyncEvaluatorReportTrigger: asyncEvaluator,
			},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 7},
			Expt: &entity.Experiment{
				ID: 100,
				Target: &entity.EvalTarget{
					EvalTargetVersion: &entity.EvalTargetVersion{
						EvalTargetType: entity.EvalTargetTypeSandboxAgent,
					},
				},
			},
		},
	}
}

func TestEmitE2EStarted_NilReceiverAndMetrics(t *testing.T) {
	// nil receiver 不 panic
	var nilExec *ExptItemEvalCtxExecutor
	nilExec.emitSandboxAgentE2EStarted(context.Background(), sandboxTurnCtx(0, false, false))

	// sandboxAgentMetrics nil → 静默返回
	exec := &ExptItemEvalCtxExecutor{}
	exec.emitSandboxAgentE2EStarted(context.Background(), sandboxTurnCtx(0, false, false))
}

func TestEmitE2EStarted_NilEtecOrNestedNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, _ := newSandboxE2EExecutor(t, ctrl)
	// mock 未注册 EXPECT → 若调用即失败

	// etec == nil
	exec.emitSandboxAgentE2EStarted(context.Background(), nil)
	// Event 为 nil (嵌入 ExptItemEvalCtx 非 nil)
	exec.emitSandboxAgentE2EStarted(context.Background(), &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{},
	})
	// Expt 为 nil (Event 非 nil)
	exec.emitSandboxAgentE2EStarted(context.Background(), &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: &entity.ExptItemEvalEvent{}},
	})
}

func TestEmitE2EStarted_NonSandboxExptGated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, _ := newSandboxE2EExecutor(t, ctrl)

	etec := sandboxTurnCtx(0, false, false)
	etec.Expt.Target.EvalTargetVersion.EvalTargetType = entity.EvalTargetTypeLoopPrompt
	exec.emitSandboxAgentE2EStarted(context.Background(), etec)
}

func TestEmitE2EStarted_RetryTimesGated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, _ := newSandboxE2EExecutor(t, ctrl)

	// RetryTimes>0 → 不 emit
	exec.emitSandboxAgentE2EStarted(context.Background(), sandboxTurnCtx(1, false, false))
}

func TestEmitE2EStarted_AsyncTriggerGated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, _ := newSandboxE2EExecutor(t, ctrl)

	// async 回调重进 → 不 emit (两种 trigger 分别验证)
	exec.emitSandboxAgentE2EStarted(context.Background(), sandboxTurnCtx(0, true, false))
	exec.emitSandboxAgentE2EStarted(context.Background(), sandboxTurnCtx(0, false, true))
}

// happy path: 首次入调度 + 非 async + 沙箱 agent → emit 一次, tag 齐全
func TestEmitE2EStarted_HappyPathTagsCorrect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	etec := sandboxTurnCtx(0, false, false)
	etec.Turn.ID = 555
	etec.Expt.Target.ID = 33
	etec.Expt.Target.SourceTargetID = "app-xyz"
	etec.Expt.Target.EvalTargetVersion.SandboxAgent = &entity.SandboxAgent{Name: "agent-a"}
	etec.EvalSetItem.EvaluationSetID = 888
	etec.EvalSetItem.ItemKey = "ik-x"
	etec.EvalSetVersionID = 999
	etec.Expt.EvalSetID = 888

	var captured eval_metrics.SandboxAgentE2ETags
	m.EXPECT().EmitE2EStarted(gomock.Any()).
		Do(func(tags eval_metrics.SandboxAgentE2ETags) {
			captured = tags
		}).Times(1)
	exec.emitSandboxAgentE2EStarted(context.Background(), etec)

	assert.Equal(t, int64(4), captured.SpaceID)
	assert.Equal(t, int64(100), captured.ExperimentID)
	assert.Equal(t, int64(200), captured.ExperimentRunID)
	assert.Equal(t, int64(7), captured.ItemID)
	assert.Equal(t, int64(555), captured.TurnID)
	assert.Equal(t, int64(888), captured.DatasetID)
	assert.Equal(t, int64(999), captured.DatasetVersion)
	assert.Equal(t, int64(33), captured.TargetID)
	assert.Equal(t, "ik-x", captured.ItemKey)
	assert.Equal(t, "agent-a", captured.AgentName)
	assert.Equal(t, "app-xyz", captured.ApplicationID)
}

// ============================================================================
// emitSandboxAgentE2EFinishedIfTerminal — 全部 gate
// ============================================================================

func TestEmitE2EFinished_NilReceiverAndMetrics(t *testing.T) {
	var nilExec *ExptItemEvalCtxExecutor
	etec := sandboxTurnCtx(0, false, false)
	nilExec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, nil)

	exec := &ExptItemEvalCtxExecutor{}
	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, nil)
}

func TestEmitE2EFinished_NilInputs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, _ := newSandboxE2EExecutor(t, ctrl)

	// etec nil / event nil / Expt nil → 静默返回
	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), nil, &entity.ExptItemEvalEvent{}, nil, nil)
	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(),
		&entity.ExptTurnEvalCtx{ExptItemEvalCtx: &entity.ExptItemEvalCtx{}}, nil, nil, nil)
	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(),
		&entity.ExptTurnEvalCtx{ExptItemEvalCtx: &entity.ExptItemEvalCtx{}}, &entity.ExptItemEvalEvent{}, nil, nil)
}

func TestEmitE2EFinished_NonSandboxGated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, _ := newSandboxE2EExecutor(t, ctrl)

	etec := sandboxTurnCtx(0, false, false)
	etec.Expt.Target.EvalTargetVersion.EvalTargetType = entity.EvalTargetTypeLoopPrompt
	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, nil)
}

// 失败但仍可重试 → 不 emit (中间失败轮次)
func TestEmitE2EFinished_ErrorNeedRetryGated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, _ := newSandboxE2EExecutor(t, ctrl)

	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 3}).AnyTimes()
	exec.Configer = configer

	// RetryTimes=0 < max=3 → needRetry=true → 不 emit
	etec := sandboxTurnCtx(0, false, false)
	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, errors.New("boom"))
}

// 失败且已达最大重试次数 → emit (终态失败)
func TestEmitE2EFinished_ErrorTerminal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 1}).AnyTimes()
	exec.Configer = configer

	// RetryTimes=2 >= max=1 → 不再重试
	etec := sandboxTurnCtx(2, false, false)
	etec.Event.CreateAt = time.Now().Add(-2 * time.Second).Unix()

	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, err error, _ int32, startTime time.Time) {
			require.NotNil(t, err)
			require.False(t, startTime.IsZero(), "startTime should be non-zero when CreateAt>0")
			// startTime 应在最近的合理范围而不是 1970
			require.True(t, time.Since(startTime) < 24*time.Hour)
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, errors.New("boom"))
}

// 成功终态: evalErr==nil → 直接 emit
func TestEmitE2EFinished_SuccessTerminal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	etec := sandboxTurnCtx(0, false, false)
	etec.Event.CreateAt = time.Now().Add(-1 * time.Second).Unix()

	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, err error, errCode int32, startTime time.Time) {
			require.Nil(t, err)
			require.Equal(t, int32(0), errCode, "success path should carry errCode=0")
			require.False(t, startTime.IsZero())
			// 用 time.Unix(sec, 0) 正确反序列化: startTime 在 1970 之后的合理范围内
			require.True(t, time.Since(startTime) < 24*time.Hour, "should not be ~55 years ago")
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, nil)
}

// CreateAt=0 → startTime 保持零值传给 emit 层
func TestEmitE2EFinished_ZeroCreateAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	etec := sandboxTurnCtx(0, false, false)
	etec.Event.CreateAt = 0

	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, _ error, _ int32, startTime time.Time) {
			require.True(t, startTime.IsZero())
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, nil)
}

// 负 CreateAt → startTime 保持零值 (CreateAt>0 gate 不满足)
func TestEmitE2EFinished_NegativeCreateAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	etec := sandboxTurnCtx(0, false, false)
	etec.Event.CreateAt = -100

	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Nil(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, _ error, _ int32, startTime time.Time) {
			require.True(t, startTime.IsZero())
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, nil)
}

// 失败终态 + StatusError: errCode 应从 err 中抽出并透传给 EmitE2EFinished
func TestEmitE2EFinished_StatusErrorCodeExtracted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 1}).AnyTimes()
	exec.Configer = configer

	etec := sandboxTurnCtx(2, false, false) // RetryTimes >= max → 终态
	etec.Event.CreateAt = time.Now().Add(-1 * time.Second).Unix()

	statusErr := errorx.NewByCode(errno.CommonInternalErrorCode)
	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, err error, errCode int32, _ time.Time) {
			require.NotNil(t, err)
			require.Equal(t, int32(errno.CommonInternalErrorCode), errCode,
				"errCode should be extracted from StatusError")
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, statusErr)
}

// 失败终态 + 非 StatusError (plain errors.New): errCode 走 0 (由 emit 层转占位符 `-`)
func TestEmitE2EFinished_PlainErrorZeroCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 1}).AnyTimes()
	exec.Configer = configer

	etec := sandboxTurnCtx(2, false, false)
	etec.Event.CreateAt = time.Now().Add(-500 * time.Millisecond).Unix()

	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, err error, errCode int32, _ time.Time) {
			require.NotNil(t, err)
			require.Equal(t, int32(0), errCode,
				"plain error (not StatusError) should carry errCode=0")
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, nil, errors.New("plain-boom"))
}

// 失败终态 + evalErr 是 errno.NewTargetResultErr (ErrImpl 而非 StatusError, 真实 code 被丢弃):
// 应从 turnRunRes.TargetResult.EvalTargetRunError.Code 兜底抽出打点码.
// 这是修复"e2e_finished 打点 error_code 恒 0"的核心场景.
func TestEmitE2EFinished_TargetRunErrorFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 1}).AnyTimes()
	exec.Configer = configer

	etec := sandboxTurnCtx(2, false, false)
	etec.Event.CreateAt = time.Now().Add(-1 * time.Second).Unix()

	trr := &entity.ExptTurnRunResult{
		TargetResult: &entity.EvalTargetRecord{
			EvalTargetOutputData: &entity.EvalTargetOutputData{
				EvalTargetRunError: &entity.EvalTargetRunError{Code: 5073, Message: "sandbox timeout"},
			},
		},
	}
	// storeTurnRunResult 会把 EvalTargetRunError 包成 NewTargetResultErr (ErrImpl code=11, 丢真实码)
	wrappedErr := errno.NewTargetResultErr("sandbox timeout")

	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, err error, errCode int32, _ time.Time) {
			require.NotNil(t, err)
			require.Equal(t, int32(5073), errCode,
				"errCode should be recovered from TargetResult.EvalTargetRunError.Code when evalErr is not a StatusError")
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, trr, wrappedErr)
}

// 同上, 但错误来自 evaluator: 应从 turnRunRes.EvaluatorResults[*].EvaluatorRunError.Code 兜底抽出.
func TestEmitE2EFinished_EvaluatorRunErrorFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 1}).AnyTimes()
	exec.Configer = configer

	etec := sandboxTurnCtx(2, false, false)
	etec.Event.CreateAt = time.Now().Add(-1 * time.Second).Unix()

	trr := &entity.ExptTurnRunResult{
		EvaluatorResults: []*entity.EvaluatorRecord{
			{
				EvaluatorOutputData: &entity.EvaluatorOutputData{
					EvaluatorRunError: &entity.EvaluatorRunError{Code: 6001, Message: "evaluator fail"},
				},
			},
		},
	}
	wrappedErr := errno.NewEvaluatorResultErr("evaluator fail")

	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, err error, errCode int32, _ time.Time) {
			require.NotNil(t, err)
			require.Equal(t, int32(6001), errCode,
				"errCode should be recovered from EvaluatorResults[*].EvaluatorRunError.Code")
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, trr, wrappedErr)
}

// evalErr 是 StatusError 时优先走 FromStatusError, 即便 turnRunRes 里也带 target run error,
// 也用 StatusError 的 code (StatusError 上下文更精确).
func TestEmitE2EFinished_StatusErrorPreferredOverTurnRunRes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec, m := newSandboxE2EExecutor(t, ctrl)

	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 1}).AnyTimes()
	exec.Configer = configer

	etec := sandboxTurnCtx(2, false, false)
	etec.Event.CreateAt = time.Now().Add(-1 * time.Second).Unix()

	trr := &entity.ExptTurnRunResult{
		TargetResult: &entity.EvalTargetRecord{
			EvalTargetOutputData: &entity.EvalTargetOutputData{
				EvalTargetRunError: &entity.EvalTargetRunError{Code: 5073},
			},
		},
	}
	statusErr := errorx.NewByCode(errno.CommonInternalErrorCode)

	m.EXPECT().EmitE2EFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ eval_metrics.SandboxAgentE2ETags, _ error, errCode int32, _ time.Time) {
			require.Equal(t, int32(errno.CommonInternalErrorCode), errCode,
				"StatusError code should take priority over turnRunRes fallback")
		}).Times(1)

	exec.emitSandboxAgentE2EFinishedIfTerminal(context.Background(), etec, etec.Event, trr, statusErr)
}

// ============================================================================
// NewExptItemEvaluation: 沙箱 agent metrics 注入
// ============================================================================

func TestNewExptItemEvaluation_SandboxAgentMetricsInjected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
	inst := NewExptItemEvaluation(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, m)
	require.NotNil(t, inst)
	exec, ok := inst.(*ExptItemEvalCtxExecutor)
	require.True(t, ok)
	require.Same(t, m, exec.sandboxAgentMetrics)

	// 不传 metrics 时应为 nil, 兼容旧调用点
	inst2 := NewExptItemEvaluation(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	exec2 := inst2.(*ExptItemEvalCtxExecutor)
	require.Nil(t, exec2.sandboxAgentMetrics)
}
