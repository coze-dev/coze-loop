// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// buildSandboxAgentExpt 构造一个满足沙箱 agent + 通知已启用的实验。
func buildSandboxAgentExpt(enable bool, userID string) *entity.Experiment {
	return &entity.Experiment{
		ID:      100,
		SpaceID: 10,
		Name:    "sandbox-expt",
		Target: &entity.EvalTarget{
			EvalTargetVersion: &entity.EvalTargetVersion{
				EvalTargetType: entity.EvalTargetTypeSandboxAgent,
			},
		},
		NotificationConf: &entity.ExptNotificationConf{
			FeishuNotification: &entity.FeishuNotificationConf{
				Enable: enable,
				UserID: gptr.Of(userID),
			},
		},
	}
}

func newTestSandboxAgentNotifier(ctrl *gomock.Controller) (
	*sandboxAgentNotifier,
	*rpcMocks.MockINotifyRPCAdapter,
	*rpcMocks.MockIUserProvider,
	*repoMocks.MockIExptStatsRepo,
	*lockMocks.MockILocker,
) {
	notify := rpcMocks.NewMockINotifyRPCAdapter(ctrl)
	user := rpcMocks.NewMockIUserProvider(ctrl)
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)
	locker := lockMocks.NewMockILocker(ctrl)
	n := &sandboxAgentNotifier{
		notifyRPC:     notify,
		userProvider:  user,
		exptStatsRepo: stats,
		locker:        locker,
	}
	return n, notify, user, stats, locker
}

func TestNewSandboxAgentNotifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	notify := rpcMocks.NewMockINotifyRPCAdapter(ctrl)
	user := rpcMocks.NewMockIUserProvider(ctrl)
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)
	locker := lockMocks.NewMockILocker(ctrl)
	got := NewSandboxAgentNotifier(notify, user, stats, locker)
	assert.NotNil(t, got)
}

func TestSandboxAgentNotifier_NotifyProgressIfDue_NonSandbox_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, _ := newTestSandboxAgentNotifier(ctrl)
	expt := &entity.Experiment{ID: 1, SpaceID: 1, Target: &entity.EvalTarget{
		EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeLoopPrompt},
	}}
	err := n.NotifyProgressIfDue(context.Background(), expt)
	assert.NoError(t, err) // 非沙箱 agent -> 静默,不调用任何依赖
}

func TestSandboxAgentNotifier_NotifyProgressIfDue_EnableFalse_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, _ := newTestSandboxAgentNotifier(ctrl)
	expt := buildSandboxAgentExpt(false, "ou_abc")
	err := n.NotifyProgressIfDue(context.Background(), expt)
	assert.NoError(t, err)
}

func TestSandboxAgentNotifier_NotifyProgressIfDue_EmptyCardID_Skip(t *testing.T) {
	// consts.SandboxAgentProgressNotifyCardID 默认为 "",此测试保护 card 未配置时应静默。
	// 若将来把 template ID 填进 consts,该测试需要一起改为设置临时空值再复位。
	if consts.SandboxAgentProgressNotifyCardID != "" {
		t.Skip("progress card id configured, skip empty-id branch test")
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, _ := newTestSandboxAgentNotifier(ctrl)
	expt := buildSandboxAgentExpt(true, "ou_abc")
	err := n.NotifyProgressIfDue(context.Background(), expt)
	assert.NoError(t, err)
}

func TestSandboxAgentNotifier_NotifyProgressIfDue_GateHeld_Skip(t *testing.T) {
	// 拿不到 hourly 锁 -> 距上次通知<1h -> 静默。此路径与 card_id 是否配置无关,单独覆盖。
	// 这里通过在锁层直接返回 false 提前退出,避免依赖 card_id。
	if consts.SandboxAgentProgressNotifyCardID == "" {
		t.Skip("progress card id empty, gate branch unreachable")
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, locker := newTestSandboxAgentNotifier(ctrl)
	expt := buildSandboxAgentExpt(true, "ou_abc")
	locker.EXPECT().Lock(gomock.Any(), gomock.Any(), sandboxAgentProgressGateTTL).Return(false, nil)
	err := n.NotifyProgressIfDue(context.Background(), expt)
	assert.NoError(t, err)
}

func TestSandboxAgentNotifier_NotifyProgressIfDue_LockErr_Skip(t *testing.T) {
	if consts.SandboxAgentProgressNotifyCardID == "" {
		t.Skip("progress card id empty, gate branch unreachable")
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, locker := newTestSandboxAgentNotifier(ctrl)
	expt := buildSandboxAgentExpt(true, "ou_abc")
	locker.EXPECT().Lock(gomock.Any(), gomock.Any(), sandboxAgentProgressGateTTL).Return(false, errors.New("redis boom"))
	err := n.NotifyProgressIfDue(context.Background(), expt)
	assert.NoError(t, err) // 锁层报错不阻塞
}

func TestSandboxAgentNotifier_NotifyItemFail_NonSandbox_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, _ := newTestSandboxAgentNotifier(ctrl)
	expt := &entity.Experiment{ID: 1, SpaceID: 1, Target: &entity.EvalTarget{
		EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeLoopPrompt},
	}}
	err := n.NotifyItemFail(context.Background(), expt, 42, errors.New("boom"))
	assert.NoError(t, err)
}

func TestSandboxAgentNotifier_NotifyItemFail_EnableFalse_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, _ := newTestSandboxAgentNotifier(ctrl)
	expt := buildSandboxAgentExpt(false, "ou_abc")
	err := n.NotifyItemFail(context.Background(), expt, 42, errors.New("boom"))
	assert.NoError(t, err)
}

func TestSandboxAgentNotifier_NotifyItemFail_EmptyCardID_Skip(t *testing.T) {
	if consts.SandboxAgentItemFailNotifyCardID != "" {
		t.Skip("item fail card id configured, skip empty-id branch test")
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, _ := newTestSandboxAgentNotifier(ctrl)
	expt := buildSandboxAgentExpt(true, "ou_abc")
	err := n.NotifyItemFail(context.Background(), expt, 42, errors.New("boom"))
	assert.NoError(t, err)
}

func TestSandboxAgentNotifier_ResetHourlyGate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, locker := newTestSandboxAgentNotifier(ctrl)
	locker.EXPECT().UnlockForce(gomock.Any(), "sandbox_agent_progress_notify:100").Return(true, nil)
	assert.NoError(t, n.ResetHourlyGate(context.Background(), 10, 100))
}

func TestSandboxAgentNotifier_ResetHourlyGate_ErrPropagated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	n, _, _, _, locker := newTestSandboxAgentNotifier(ctrl)
	locker.EXPECT().UnlockForce(gomock.Any(), gomock.Any()).Return(false, errors.New("redis down"))
	assert.Error(t, n.ResetHourlyGate(context.Background(), 10, 100))
}

func TestBuildSandboxAgentProgressParam_WithStats(t *testing.T) {
	expt := buildSandboxAgentExpt(true, "ou_abc")
	stats := &entity.ExptStats{
		SuccessItemCnt:    3,
		FailItemCnt:       2,
		ProcessingItemCnt: 1,
		PendingItemCnt:    5,
		TerminatedItemCnt: 0,
	}
	got := buildSandboxAgentProgressParam(expt, stats)
	assert.Equal(t, "sandbox-expt", got[consts.SandboxAgentProgressKeyExptName])
	assert.Equal(t, "100", got[consts.SandboxAgentProgressKeyExptID])
	assert.Equal(t, "10", got[consts.SandboxAgentProgressKeySpaceID])
	assert.Equal(t, "3", got[consts.SandboxAgentProgressKeySuccessCnt])
	assert.Equal(t, "2", got[consts.SandboxAgentProgressKeyFailCnt])
	assert.Equal(t, "1", got[consts.SandboxAgentProgressKeyProcessingCnt])
	assert.Equal(t, "5", got[consts.SandboxAgentProgressKeyPendingCnt])
	assert.Equal(t, "0", got[consts.SandboxAgentProgressKeyTerminatedCnt])
	assert.Equal(t, "11", got[consts.SandboxAgentProgressKeyTotalCnt])
}

func TestBuildSandboxAgentProgressParam_NilStats(t *testing.T) {
	// Stats 为 nil 时 total 应为 0, 而不是崩溃。
	expt := buildSandboxAgentExpt(true, "ou_abc")
	got := buildSandboxAgentProgressParam(expt, nil)
	assert.Equal(t, "0", got[consts.SandboxAgentProgressKeyTotalCnt])
	assert.Equal(t, "0", got[consts.SandboxAgentProgressKeySuccessCnt])
}

func TestBuildSandboxAgentItemFailParam_TruncatesLongMsg(t *testing.T) {
	expt := buildSandboxAgentExpt(true, "ou_abc")
	long := make([]byte, sandboxAgentItemFailErrMsgMaxLen+100)
	for i := range long {
		long[i] = 'x'
	}
	got := buildSandboxAgentItemFailParam(expt, 42, errors.New(string(long)))
	msg := got[consts.SandboxAgentItemFailKeyErrMsg]
	assert.Equal(t, sandboxAgentItemFailErrMsgMaxLen+3, len(msg))
	assert.Equal(t, "...", msg[len(msg)-3:])
	assert.Equal(t, "42", got[consts.SandboxAgentItemFailKeyItemID])
	assert.Equal(t, "sandbox-expt", got[consts.SandboxAgentItemFailKeyExptName])
}

func TestSandboxAgentProgressGateKey(t *testing.T) {
	assert.Equal(t, "sandbox_agent_progress_notify:12345", sandboxAgentProgressGateKey(12345))
}

// 保底冒烟: 通过 nil notifyRPC 触发 enabled=false 分支。
func TestSandboxAgentNotifier_NilRPC_AllSkip(t *testing.T) {
	n := &sandboxAgentNotifier{}
	expt := buildSandboxAgentExpt(true, "ou_abc")
	assert.NoError(t, n.NotifyProgressIfDue(context.Background(), expt))
	assert.NoError(t, n.NotifyItemFail(context.Background(), expt, 1, errors.New("x")))
}

// 保护 hourly TTL 常量,防止有人误改小值绕过闸门。
func TestSandboxAgentProgressGateTTL(t *testing.T) {
	assert.Equal(t, time.Hour, sandboxAgentProgressGateTTL)
}
