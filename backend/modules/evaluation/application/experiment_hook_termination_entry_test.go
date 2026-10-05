// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestHookTerminationEntryApplicationAcceptanceFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := sm.NewMockIExptManager(ctrl)
	configer := cm.NewMockIConfiger(ctrl)
	failure := errors.New("termination acceptance unavailable")
	manager.EXPECT().Get(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(&entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 3, Status: entity.ExptStatus_Processing, CreatedBy: "user"}, nil)
	configer.EXPECT().GetMaintainerUserIDs(gomock.Any()).Return(map[string]bool{"user": true})
	manager.EXPECT().SetExptTerminating(gomock.Any(), int64(2), int64(3), int64(1), gomock.Any()).Return(failure)
	// No completion expectation: acceptance failure must return before launching asynchronous work.
	app := &experimentApplication{manager: manager, configer: configer}
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "user"})
	response, err := app.KillExperiment(ctx, &exptpb.KillExperimentRequest{WorkspaceID: gptr.Of(int64(1)), ExptID: gptr.Of(int64(2))})
	require.ErrorIs(t, err, failure)
	require.Nil(t, response)
}
