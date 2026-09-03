// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	configermocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// buildFailPathExecutor builds a minimally-wired ExptItemEvalCtxExecutor whose
// CompleteItemRun fail branch will be triggered: UpdateItemRunLog succeeds and
// GetErrRetryConf returns non-retry (so evalErrNeedRetry is false).
func buildFailPathExecutor(t *testing.T, ctrl *gomock.Controller, notifier ISandboxAgentNotifier) (*ExptItemEvalCtxExecutor, *repomocks.MockIExptItemResultRepo, *configermocks.MockIConfiger) {
	t.Helper()
	itemResultRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&entity.RetryConf{})
	// CompleteItemRun 写 status 前会读一次当前 run log 做 Terminal 覆盖保护; 此处固定返回非 Terminal，
	// 让这些用例仍走原有的 Fail / Success 写入分支。
	itemResultRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&entity.ExptItemResultRunLog{Status: int32(entity.ItemRunState_Processing)}, nil)
	exec := &ExptItemEvalCtxExecutor{
		ItemResultRepo:       itemResultRepo,
		Configer:             configer,
		sandboxAgentNotifier: notifier,
	}
	return exec, itemResultRepo, configer
}

func sandboxExpt() *entity.Experiment {
	return &entity.Experiment{
		ID:      100,
		SpaceID: 10,
		Name:    "sandbox-expt",
		Target: &entity.EvalTarget{
			EvalTargetVersion: &entity.EvalTargetVersion{
				EvalTargetType: entity.EvalTargetTypeSandboxAgent,
			},
		},
	}
}

// CompleteItemRun fail 分支: 有 notifier 时同步调用 NotifyItemFail。
func TestCompleteItemRun_FailBranch_InvokesNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := servicemocks.NewMockISandboxAgentNotifier(ctrl)
	exec, itemResultRepo, _ := buildFailPathExecutor(t, ctrl, notifier)

	failErr := errors.New("target timeout")
	itemResultRepo.EXPECT().
		UpdateItemRunLog(gomock.Any(), int64(1), int64(2), []int64{3}, gomock.Any(), int64(4)).
		Return(nil)
	notifier.EXPECT().
		NotifyItemFail(gomock.Any(), gomock.Any(), int64(3), failErr).
		Return(nil).
		Times(1)

	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4, RetryTimes: 1},
		Expt:  sandboxExpt(),
	}
	assert.NoError(t, exec.CompleteItemRun(context.Background(), eiec, failErr))
}

// CompleteItemRun 成功分支: 即便 notifier 非 nil, 也不应触发 NotifyItemFail (evalErr==nil)。
func TestCompleteItemRun_SuccessBranch_SkipsNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := servicemocks.NewMockISandboxAgentNotifier(ctrl)
	exec, itemResultRepo, _ := buildFailPathExecutor(t, ctrl, notifier)

	itemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	// notifier.EXPECT() 不注册, 表示"未被调用"。

	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4},
		Expt:  sandboxExpt(),
	}
	assert.NoError(t, exec.CompleteItemRun(context.Background(), eiec, nil))
}

// CompleteItemRun fail 分支: 无 notifier 时不 panic, 主流程仍走通。
func TestCompleteItemRun_FailBranch_NilNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	exec, itemResultRepo, _ := buildFailPathExecutor(t, ctrl, nil)
	itemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4, RetryTimes: 1},
		Expt:  sandboxExpt(),
	}
	assert.NoError(t, exec.CompleteItemRun(context.Background(), eiec, errors.New("boom")))
}

// notifier 返回错误时, CompleteItemRun 仍成功 (不阻塞主流程)。
func TestCompleteItemRun_FailBranch_NotifierErrIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := servicemocks.NewMockISandboxAgentNotifier(ctrl)
	exec, itemResultRepo, _ := buildFailPathExecutor(t, ctrl, notifier)

	itemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	notifier.EXPECT().
		NotifyItemFail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("send timeout"))

	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4, RetryTimes: 1},
		Expt:  sandboxExpt(),
	}
	assert.NoError(t, exec.CompleteItemRun(context.Background(), eiec, errors.New("boom")))
}

// NewExptItemEvaluation variadic 参数: 传入 notifier 时应保存到 executor。
func TestNewExptItemEvaluation_VariadicNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := servicemocks.NewMockISandboxAgentNotifier(ctrl)
	inst := NewExptItemEvaluation(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, notifier)
	require.NotNil(t, inst)
	exec, ok := inst.(*ExptItemEvalCtxExecutor)
	require.True(t, ok)
	assert.Same(t, notifier, exec.sandboxAgentNotifier)

	// 不传时应为 nil (兼容旧调用点)
	inst2 := NewExptItemEvaluation(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	exec2 := inst2.(*ExptItemEvalCtxExecutor)
	assert.Nil(t, exec2.sandboxAgentNotifier)
}

// storeTurnRunResult 沙箱 agent 加白分支: 错误文案沿用原文, 不走 ConvertErrMsg。
func TestStoreTurnRunResult_SandboxAgentWhitelist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	turnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	configer := configermocks.NewMockIConfiger(ctrl)
	// Configer.GetErrCtrl 不应被调用: 沙箱 agent 分支跳过 ConvertErrMsg。
	// 若被调用, gomock 会因未注册期望而 fail。

	exec := &ExptItemEvalCtxExecutor{
		TurnResultRepo: turnResultRepo,
		Configer:       configer,
	}

	rawMsg := "raw sandbox agent error verbatim"
	turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt: &entity.Experiment{
				ID:      1,
				SpaceID: 2,
				Target: &entity.EvalTarget{
					EvalTargetVersion: &entity.EvalTargetVersion{
						EvalTargetType: entity.EvalTargetTypeSandboxAgent,
					},
				},
			},
			Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
		},
	}
	result := &entity.ExptTurnRunResult{EvalErr: errors.New(rawMsg)}

	turnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			require.Equal(t, entity.TurnRunState_Fail, logs[0].Status)
			// 序列化后的 err_msg 应包含原文, 未经 ConvertErrMsg 归一化。
			require.NotEmpty(t, logs[0].ErrMsg)
			require.Contains(t, logs[0].ErrMsg, rawMsg)
			return nil
		})

	assert.NoError(t, exec.storeTurnRunResult(context.Background(), etec, result))
}

// storeTurnRunResult 非沙箱 agent + 非白名单错误码时, 走 ConvertErrMsg 归一化。
// (regression: 确认沙箱加白只作用于沙箱实验)
func TestStoreTurnRunResult_NonSandboxGoesThroughConvertErrMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	turnResultRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	configer := configermocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl()).Times(1)

	exec := &ExptItemEvalCtxExecutor{
		TurnResultRepo: turnResultRepo,
		Configer:       configer,
	}

	turnResultLog := &entity.ExptTurnResultRunLog{ID: 1, TurnID: 1}
	etec := &entity.ExptTurnEvalCtx{
		Turn: &entity.Turn{ID: 1},
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Expt: &entity.Experiment{
				ID:      1,
				SpaceID: 2,
				Target: &entity.EvalTarget{
					EvalTargetVersion: &entity.EvalTargetVersion{
						EvalTargetType: entity.EvalTargetTypeLoopPrompt, // 非沙箱
					},
				},
			},
			Event:               &entity.ExptItemEvalEvent{ExptRunID: 3},
			EvalSetItem:         &entity.EvaluationSetItem{ItemID: 2},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: turnResultLog}},
		},
	}
	result := &entity.ExptTurnRunResult{EvalErr: errors.New("some plain error")}

	turnResultRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil)
	assert.NoError(t, exec.storeTurnRunResult(context.Background(), etec, result))
}

// 让 NotifyItemFail 触发时收到的 expt/itemID 正确无误。
func TestCompleteItemRun_FailBranch_NotifierReceivesExpectedArgs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := servicemocks.NewMockISandboxAgentNotifier(ctrl)
	exec, itemResultRepo, _ := buildFailPathExecutor(t, ctrl, notifier)

	expt := sandboxExpt()
	failErr := errors.New("failed reason")

	itemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), int64(11), int64(22), []int64{33}, gomock.Any(), int64(44)).Return(nil)
	notifier.EXPECT().
		NotifyItemFail(gomock.Any(), gomock.AssignableToTypeOf(&entity.Experiment{}), int64(33), failErr).
		DoAndReturn(func(_ context.Context, gotExpt *entity.Experiment, gotItemID int64, gotErr error) error {
			assert.Same(t, expt, gotExpt)
			assert.Equal(t, int64(33), gotItemID)
			assert.Same(t, failErr, gotErr)
			return nil
		})

	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{ExptID: 11, ExptRunID: 22, EvalSetItemID: 33, SpaceID: 44, RetryTimes: 1},
		Expt:  expt,
	}
	assert.NoError(t, exec.CompleteItemRun(context.Background(), eiec, failErr))
}
