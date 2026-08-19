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

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// withCardIDs 临时把两个 template ID 设置为非空,测试完毕恢复。
// 由于 sandboxAgent{Progress,ItemFail}CardID 是包级 var, 只能串行运行 (禁用 t.Parallel)。
func withCardIDs(t *testing.T, progress, itemFail string) {
	t.Helper()
	origP := sandboxAgentProgressCardID
	origF := sandboxAgentItemFailCardID
	sandboxAgentProgressCardID = progress
	sandboxAgentItemFailCardID = itemFail
	t.Cleanup(func() {
		sandboxAgentProgressCardID = origP
		sandboxAgentItemFailCardID = origF
	})
}

func newNotifierWithMocks(ctrl *gomock.Controller) (*sandboxAgentNotifier, *rpcmocks.MockINotifyRPCAdapter, *rpcmocks.MockIUserProvider, *repomocks.MockIExptStatsRepo, *lockmocks.MockILocker) {
	notify := rpcmocks.NewMockINotifyRPCAdapter(ctrl)
	user := rpcmocks.NewMockIUserProvider(ctrl)
	stats := repomocks.NewMockIExptStatsRepo(ctrl)
	locker := lockmocks.NewMockILocker(ctrl)
	return &sandboxAgentNotifier{
		notifyRPC:     notify,
		userProvider:  user,
		exptStatsRepo: stats,
		locker:        locker,
	}, notify, user, stats, locker
}

// NotifyProgressIfDue 完整 happy path: 拿到锁 + 解析到 receiveID + stats 拉到 + 发送成功。
func TestNotifyProgressIfDue_HappyPath(t *testing.T) {
	withCardIDs(t, "test_progress_card_id", "test_item_fail_card_id")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, notify, _, stats, locker := newNotifierWithMocks(ctrl)
	expt := &entity.Experiment{
		ID:      42,
		SpaceID: 7,
		Name:    "my-expt",
		Target:  &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{
			Enable: true,
			UserID: strPtr("ou_test_open_id"),
		}},
	}
	locker.EXPECT().Lock(gomock.Any(), "sandbox_agent_progress_notify:42", gomock.Any()).Return(true, nil)
	stats.EXPECT().Get(gomock.Any(), int64(42), int64(7)).Return(&entity.ExptStats{SuccessItemCnt: 3, FailItemCnt: 1}, nil)
	notify.EXPECT().
		SendMessageCard(gomock.Any(), "ou_test_open_id", "open_id", "test_progress_card_id", gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _ string, param map[string]string) error {
			assert.Equal(t, "3", param["success_cnt"])
			assert.Equal(t, "1", param["fail_cnt"])
			assert.Equal(t, "4", param["total_cnt"])
			assert.Equal(t, "my-expt", param["expt_name"])
			return nil
		})
	assert.NoError(t, n.NotifyProgressIfDue(context.Background(), expt))
}

// NotifyProgressIfDue stats 拉取报错时仍继续发送 (总数记 0)。
func TestNotifyProgressIfDue_StatsErrStillSends(t *testing.T) {
	withCardIDs(t, "cid", "cid2")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, notify, _, stats, locker := newNotifierWithMocks(ctrl)
	expt := &entity.Experiment{
		ID: 42, SpaceID: 7, Name: "e",
		Target:           &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{Enable: true, UserID: strPtr("ou_x")}},
	}
	locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))
	notify.EXPECT().SendMessageCard(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _ string, param map[string]string) error {
			assert.Equal(t, "0", param["total_cnt"]) // stats nil -> 0
			return nil
		})
	assert.NoError(t, n.NotifyProgressIfDue(context.Background(), expt))
}

// NotifyProgressIfDue: 无法解析到 receiveID (Enable=true, UserID="THEMIS_END_UID_INVALID", CreatedBy 也空) -> 静默,不发送。
func TestNotifyProgressIfDue_NoTarget(t *testing.T) {
	withCardIDs(t, "cid", "cid2")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, _, _, _, locker := newNotifierWithMocks(ctrl)
	expt := &entity.Experiment{
		ID: 42, SpaceID: 7,
		Target:           &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{Enable: true, UserID: strPtr("")}},
		CreatedBy:        "", // 无兜底 target
	}
	locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	// notifyRPC.SendMessageCard 不应被调用。
	assert.NoError(t, n.NotifyProgressIfDue(context.Background(), expt))
}

// NotifyProgressIfDue: SendMessageCard 报错时 log 后返回 nil。
func TestNotifyProgressIfDue_SendErr(t *testing.T) {
	withCardIDs(t, "cid", "cid2")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, notify, _, stats, locker := newNotifierWithMocks(ctrl)
	expt := &entity.Experiment{
		ID: 42, SpaceID: 7, Name: "e",
		Target:           &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{Enable: true, UserID: strPtr("ou_x")}},
	}
	locker.EXPECT().Lock(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptStats{}, nil)
	notify.EXPECT().SendMessageCard(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("lark timeout"))
	assert.NoError(t, n.NotifyProgressIfDue(context.Background(), expt))
}

// NotifyItemFail 完整 happy path: enabled + card_id 有 + target 解析成功 -> 发送。
func TestNotifyItemFail_HappyPath(t *testing.T) {
	withCardIDs(t, "cid", "item_fail_cid")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, notify, _, _, _ := newNotifierWithMocks(ctrl)
	expt := &entity.Experiment{
		ID: 42, SpaceID: 7, Name: "e",
		Target:           &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{Enable: true, UserID: strPtr("ou_x")}},
	}
	notify.EXPECT().
		SendMessageCard(gomock.Any(), "ou_x", "open_id", "item_fail_cid", gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _ string, param map[string]string) error {
			assert.Equal(t, "e", param["expt_name"])
			assert.Equal(t, "42", param["expt_id"])
			assert.Equal(t, "999", param["item_id"])
			return nil
		})
	assert.NoError(t, n.NotifyItemFail(context.Background(), expt, 999, errors.New("item failed")))
}

// NotifyItemFail: 无 target -> 静默。
func TestNotifyItemFail_NoTarget(t *testing.T) {
	withCardIDs(t, "cid", "item_fail_cid")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, _, _, _, _ := newNotifierWithMocks(ctrl)
	expt := &entity.Experiment{
		ID: 42, SpaceID: 7,
		Target:           &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{Enable: true, UserID: strPtr("")}},
	}
	assert.NoError(t, n.NotifyItemFail(context.Background(), expt, 1, errors.New("x")))
}

// NotifyItemFail: SendMessageCard 报错时 log 后返回 nil。
func TestNotifyItemFail_SendErr(t *testing.T) {
	withCardIDs(t, "cid", "item_fail_cid")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, notify, _, _, _ := newNotifierWithMocks(ctrl)
	expt := &entity.Experiment{
		ID: 42, SpaceID: 7,
		Target:           &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{Enable: true, UserID: strPtr("ou_x")}},
	}
	notify.EXPECT().SendMessageCard(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("boom"))
	assert.NoError(t, n.NotifyItemFail(context.Background(), expt, 1, errors.New("x")))
}

// enabled 分支: Target nil -> false (即使 NotificationConf.Enable=true)。
func TestEnabled_NilTarget(t *testing.T) {
	n := &sandboxAgentNotifier{notifyRPC: rpcmocks.NewMockINotifyRPCAdapter(gomock.NewController(t))}
	expt := &entity.Experiment{
		ID:               1,
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{Enable: true}},
	}
	assert.False(t, n.enabled(expt, logTagProgress))
}

// enabled: NotificationConf nil -> false.
func TestEnabled_NilNotificationConf(t *testing.T) {
	n := &sandboxAgentNotifier{notifyRPC: rpcmocks.NewMockINotifyRPCAdapter(gomock.NewController(t))}
	expt := &entity.Experiment{
		Target: &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
	}
	assert.False(t, n.enabled(expt, logTagProgress))
}

// enabled: FeishuNotification 为 nil -> false.
func TestEnabled_NilFeishuConf(t *testing.T) {
	n := &sandboxAgentNotifier{notifyRPC: rpcmocks.NewMockINotifyRPCAdapter(gomock.NewController(t))}
	expt := &entity.Experiment{
		Target:           &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{},
	}
	assert.False(t, n.enabled(expt, logTagProgress))
}

// enabled: 满足所有条件 -> true.
func TestEnabled_AllOK(t *testing.T) {
	n := &sandboxAgentNotifier{notifyRPC: rpcmocks.NewMockINotifyRPCAdapter(gomock.NewController(t))}
	expt := &entity.Experiment{
		Target:           &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		NotificationConf: &entity.ExptNotificationConf{FeishuNotification: &entity.FeishuNotificationConf{Enable: true}},
	}
	assert.True(t, n.enabled(expt, logTagProgress))
}

// strPtr 辅助工具, 保持与 entity 中 *string 语义一致。
func strPtr(s string) *string {
	return &s
}

// 保底: withCardIDs 恢复语义正确。
func TestWithCardIDsRestore(t *testing.T) {
	origP := sandboxAgentProgressCardID
	origF := sandboxAgentItemFailCardID
	t.Run("inner", func(t *testing.T) {
		withCardIDs(t, "x", "y")
		require.Equal(t, "x", sandboxAgentProgressCardID)
		require.Equal(t, "y", sandboxAgentItemFailCardID)
	})
	require.Equal(t, origP, sandboxAgentProgressCardID)
	require.Equal(t, origF, sandboxAgentItemFailCardID)
}
