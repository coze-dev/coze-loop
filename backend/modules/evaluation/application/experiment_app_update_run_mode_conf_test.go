// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// TestUpdateExptRunConf_MaxRunMinutesAndMaxTurns 覆盖就地编辑「单题最长运行时间 / 最长轮次」的
// 入参校验与透传。两项都只对尚未调度的题次生效，生效点在 domain 的 eval_conf RMW 块上。
func TestUpdateExptRunConf_MaxRunMinutesAndMaxTurns(t *testing.T) {
	const (
		workspaceID = int64(123)
		exptID      = int64(456)
		userID      = "789"
	)

	newApp := func(ctrl *gomock.Controller) (*experimentApplication, *servicemocks.MockIExptManager) {
		mockManager := servicemocks.NewMockIExptManager(ctrl)
		mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
		mockConfiger := componentMocks.NewMockIConfiger(ctrl)
		mockManager.EXPECT().Get(gomock.Any(), exptID, workspaceID, &entity.Session{}).Return(
			&entity.Experiment{ID: exptID, SpaceID: workspaceID, Status: entity.ExptStatus_Processing, CreatedBy: userID}, nil)
		mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
		mockConfiger.EXPECT().GetExptExecConf(gomock.Any(), workspaceID).Return(
			&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{MaxItemConcurNum: 200}}).AnyTimes()
		return &experimentApplication{manager: mockManager, auth: mockAuth, configer: mockConfiger}, mockManager
	}

	t.Run("两项透传到 domain", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app, mockManager := newApp(ctrl)
		mockManager.EXPECT().UpdateRunConf(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, param *entity.UpdateRunConfParam) error {
				assert.Equal(t, 45, gptr.Indirect(param.MaxRunMinutes))
				assert.Equal(t, 12, gptr.Indirect(param.MaxTurns))
				return nil
			})

		_, err := app.UpdateExptRunConf(context.Background(), &exptpb.UpdateExptRunConfRequest{
			ExptID: exptID, WorkspaceID: workspaceID,
			MaxRunMinutes: gptr.Of(int32(45)), MaxTurns: gptr.Of(int32(12)),
		})
		assert.NoError(t, err)
	})

	t.Run("不传两项时 domain 侧为 nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app, mockManager := newApp(ctrl)
		mockManager.EXPECT().UpdateRunConf(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, param *entity.UpdateRunConfParam) error {
				assert.Nil(t, param.MaxRunMinutes)
				assert.Nil(t, param.MaxTurns)
				return nil
			})

		_, err := app.UpdateExptRunConf(context.Background(), &exptpb.UpdateExptRunConfRequest{
			ExptID: exptID, WorkspaceID: workspaceID, ItemRetryNum: gptr.Of(int32(2)),
		})
		assert.NoError(t, err)
	})

	// 0 与负数都拒绝：max_turns 落 0 会让多轮跑法静默退化成只跑 1 轮。
	for _, tt := range []struct {
		name string
		req  *exptpb.UpdateExptRunConfRequest
	}{
		{"最长时间为 0", &exptpb.UpdateExptRunConfRequest{ExptID: exptID, WorkspaceID: workspaceID, MaxRunMinutes: gptr.Of(int32(0))}},
		{"最长时间为负", &exptpb.UpdateExptRunConfRequest{ExptID: exptID, WorkspaceID: workspaceID, MaxRunMinutes: gptr.Of(int32(-1))}},
		{"轮次为 0", &exptpb.UpdateExptRunConfRequest{ExptID: exptID, WorkspaceID: workspaceID, MaxTurns: gptr.Of(int32(0))}},
		{"轮次为负", &exptpb.UpdateExptRunConfRequest{ExptID: exptID, WorkspaceID: workspaceID, MaxTurns: gptr.Of(int32(-2))}},
	} {
		t.Run(tt.name+"→拒绝且不落库", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app, mockManager := newApp(ctrl)
			mockManager.EXPECT().UpdateRunConf(gomock.Any(), gomock.Any()).Times(0)

			_, err := app.UpdateExptRunConf(context.Background(), tt.req)
			require.Error(t, err)
			statusErr, ok := errorx.FromStatusError(err)
			require.True(t, ok)
			assert.Equal(t, int32(errno.ExperimentValidateFailCode), statusErr.Code())
		})
	}
}
