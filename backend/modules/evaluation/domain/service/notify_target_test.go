// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	rpcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestResolveNotifyTarget(t *testing.T) {
	ctx := context.Background()

	// feishuConf 构造带 user_id 的 NotificationConf
	feishuConf := func(userID string) *entity.ExptNotificationConf {
		return &entity.ExptNotificationConf{
			FeishuNotification: &entity.FeishuNotificationConf{
				Enable: true,
				UserID: gptr.Of(userID),
			},
		}
	}

	tests := []struct {
		name string
		expt *entity.Experiment
		// mockUserInfo 非 nil 时表示期望调用 MGetUserInfo 并返回该结果
		mockUserID   string
		mockUserInfo []*entity.UserInfo
		mockErr      error
		wantReceive  string
		wantType     string
	}{
		{
			name:        "空 user_id 且空 created_by → 无目标",
			expt:        &entity.Experiment{},
			wantReceive: "",
			wantType:    "",
		},
		{
			name:        "THEMIS_END_UID_INVALID → 无目标",
			expt:        &entity.Experiment{CreatedBy: "THEMIS_END_UID_INVALID"},
			wantReceive: "",
			wantType:    "",
		},
		{
			name:        `"0" → 无目标`,
			expt:        &entity.Experiment{CreatedBy: "0"},
			wantReceive: "",
			wantType:    "",
		},
		{
			name:        "email 格式 → email",
			expt:        &entity.Experiment{NotificationConf: feishuConf("wangtao.everett@bytedance.com")},
			wantReceive: "wangtao.everett@bytedance.com",
			wantType:    "email",
		},
		{
			name:        "ou_ 前缀 → open_id",
			expt:        &entity.Experiment{NotificationConf: feishuConf("ou_abc123")},
			wantReceive: "ou_abc123",
			wantType:    "open_id",
		},
		{
			name:        "on_ 前缀 → union_id",
			expt:        &entity.Experiment{NotificationConf: feishuConf("on_xyz456")},
			wantReceive: "on_xyz456",
			wantType:    "union_id",
		},
		{
			name:         "平台 user id → MGetUserInfo 查 email",
			expt:         &entity.Experiment{NotificationConf: feishuConf("7590093942489318402")},
			mockUserID:   "7590093942489318402",
			mockUserInfo: []*entity.UserInfo{{Email: gptr.Of("wangtao.everett@bytedance.com")}},
			wantReceive:  "wangtao.everett@bytedance.com",
			wantType:     "email",
		},
		{
			name:         "平台 user id 但查不到 email → 无目标",
			expt:         &entity.Experiment{NotificationConf: feishuConf("7590093942489318402")},
			mockUserID:   "7590093942489318402",
			mockUserInfo: []*entity.UserInfo{{Email: gptr.Of("")}},
			wantReceive:  "",
			wantType:     "",
		},
		{
			name:        "user_id 为空时 fallback 到 created_by(email)",
			expt:        &entity.Experiment{CreatedBy: "fallback@bytedance.com"},
			wantReceive: "fallback@bytedance.com",
			wantType:    "email",
		},
		{
			name:        "FeishuNotification.UserID 优先于 created_by",
			expt:        &entity.Experiment{CreatedBy: "created@bytedance.com", NotificationConf: feishuConf("prefer@bytedance.com")},
			wantReceive: "prefer@bytedance.com",
			wantType:    "email",
		},
		{
			name:        "平台 user id MGetUserInfo 报错 → 静默无目标",
			expt:        &entity.Experiment{CreatedBy: "7590093942489318402"},
			mockUserID:  "7590093942489318402",
			mockErr:     assert.AnError,
			wantReceive: "",
			wantType:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			userProvider := rpcMocks.NewMockIUserProvider(ctrl)

			if tt.mockUserID != "" {
				userProvider.EXPECT().MGetUserInfo(ctx, []string{tt.mockUserID}).Return(tt.mockUserInfo, tt.mockErr)
			}

			gotReceive, gotType := resolveNotifyTarget(ctx, userProvider, tt.expt)
			assert.Equal(t, tt.wantReceive, gotReceive)
			assert.Equal(t, tt.wantType, gotType)
		})
	}
}
