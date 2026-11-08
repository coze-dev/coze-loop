// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	exptapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type pendingCancelManager struct {
	*servicemocks.MockIExptManager
	cancelled chan int64
	cancelErr error
	replaced  bool
}

func (m *pendingCancelManager) TerminateCurrentRun(_ context.Context, _, run, _ int64, _ *entity.Session, _ *entity.ExptCurrentItemRunSource, _ ...entity.CompleteExptOptionFn) (bool, error) {
	m.cancelled <- run
	return m.cancelErr == nil && !m.replaced, m.cancelErr
}

func TestKillPendingReentryAndCleanupFailure(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminating, entity.ExptStatus_Terminated} {
		ctrl := gomock.NewController(t)
		cause := errors.New("quota release unavailable")
		manager := &pendingCancelManager{MockIExptManager: servicemocks.NewMockIExptManager(ctrl), cancelled: make(chan int64, 2), cancelErr: cause}
		auth, cfg := rpcmocks.NewMockIAuthProvider(ctrl), componentmocks.NewMockIConfiger(ctrl)
		manager.EXPECT().Get(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(
			&entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 3, ExptType: entity.ExptType_Offline, Status: status}, nil).Times(2)
		cfg.EXPECT().GetMaintainerUserIDs(gomock.Any()).Return(nil).Times(2)
		auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil).Times(2)
		app := &experimentApplication{manager: manager, auth: auth, configer: cfg}
		req := &exptapi.KillExperimentRequest{WorkspaceID: gptr.Of(int64(1)), ExptID: gptr.Of(int64(2))}
		_, err := app.KillExperiment(context.Background(), req)
		require.ErrorIs(t, err, cause)
		manager.cancelErr = nil
		_, err = app.KillExperiment(context.Background(), req)
		require.NoError(t, err)
		require.Len(t, manager.cancelled, 2)
	}
}

func TestKillPendingRejectsUnauthorizedOrReplacedRun(t *testing.T) {
	for _, denied := range []bool{false, true} {
		ctrl := gomock.NewController(t)
		manager := &pendingCancelManager{MockIExptManager: servicemocks.NewMockIExptManager(ctrl), cancelled: make(chan int64, 1), replaced: true}
		auth, cfg := rpcmocks.NewMockIAuthProvider(ctrl), componentmocks.NewMockIConfiger(ctrl)
		manager.EXPECT().Get(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(
			&entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 3, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Pending}, nil)
		cfg.EXPECT().GetMaintainerUserIDs(gomock.Any()).Return(nil)
		var authErr error
		if denied {
			authErr = errors.New("denied")
		}
		auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(authErr)
		app := &experimentApplication{manager: manager, auth: auth, configer: cfg}
		_, err := app.KillExperiment(context.Background(), &exptapi.KillExperimentRequest{WorkspaceID: gptr.Of(int64(1)), ExptID: gptr.Of(int64(2))})
		require.Error(t, err)
		if denied {
			require.Empty(t, manager.cancelled)
		}
	}
}

func TestReviewKillFailedExperimentWithNewPendingRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := &pendingCancelManager{MockIExptManager: servicemocks.NewMockIExptManager(ctrl), cancelled: make(chan int64, 1)}
	auth, cfg := rpcmocks.NewMockIAuthProvider(ctrl), componentmocks.NewMockIConfiger(ctrl)
	manager.EXPECT().Get(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(
		&entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 99, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Failed}, nil)
	manager.EXPECT().GetRunLog(gomock.Any(), int64(2), int64(99), int64(1), gomock.Any()).Return(
		&entity.ExptRunLog{SpaceID: 1, ExptID: 2, ExptRunID: 99, Status: int64(entity.ExptStatus_Pending)}, nil)
	cfg.EXPECT().GetMaintainerUserIDs(gomock.Any()).Return(nil)
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
	app := &experimentApplication{manager: manager, auth: auth, configer: cfg}
	_, err := app.KillExperiment(context.Background(), &exptapi.KillExperimentRequest{WorkspaceID: gptr.Of(int64(1)), ExptID: gptr.Of(int64(2))})
	require.NoError(t, err)
	require.Equal(t, int64(99), <-manager.cancelled)
}

func TestKillPendingUsesOriginalCancellation(t *testing.T) {
	for _, run := range []int64{0, 3} {
		t.Run(fmt.Sprint(run), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := &pendingCancelManager{MockIExptManager: servicemocks.NewMockIExptManager(ctrl), cancelled: make(chan int64, 1)}
			auth, cfg := rpcmocks.NewMockIAuthProvider(ctrl), componentmocks.NewMockIConfiger(ctrl)
			manager.EXPECT().Get(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(
				&entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: run, CreatedBy: "original", ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Pending}, nil)
			cfg.EXPECT().GetMaintainerUserIDs(gomock.Any()).Return(nil)
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			app := &experimentApplication{manager: manager, auth: auth, configer: cfg}
			_, err := app.KillExperiment(session.WithCtxUser(context.Background(), &session.User{ID: "operator"}),
				&exptapi.KillExperimentRequest{WorkspaceID: gptr.Of(int64(1)), ExptID: gptr.Of(int64(2))})
			require.NoError(t, err)
			select {
			case got := <-manager.cancelled:
				require.Equal(t, run, got)
			case <-time.After(3 * time.Second):
				t.Fatal("original cancellation cleanup was not called")
			}
		})
	}
}
