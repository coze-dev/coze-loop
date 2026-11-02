// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

type terminateAwareManager struct {
	*svcmocks.MockIExptManager
	terminated bool
}

func (m *terminateAwareManager) TerminateCurrentRun(context.Context, int64, int64, int64, *entity.Session, *entity.ExptCurrentItemRunSource, ...entity.CompleteExptOptionFn) (bool, error) {
	m.terminated = true
	return true, nil
}

// 重试 run 建在已终态实验上、ExptStart 又失败时，终止链路会拒绝受理（requires active current run），
// 必须落回 CompleteRun/CompleteExpt 收口，否则 run 永远停在 Pending。
func TestHandleEventErrFallsBackForFinishedExperiment(t *testing.T) {
	for _, kind := range []string{"finished", "processing"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := &terminateAwareManager{MockIExptManager: svcmocks.NewMockIExptManager(ctrl)}
			exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
			impl := &ExptSchedulerImpl{Manager: manager, ExptRepo: exptRepo}
			event := &entity.ExptScheduleEvent{
				ExptID: 7, ExptRunID: 8, SpaceID: 9,
				ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeRetryAll,
				Session: &entity.Session{},
			}
			status := entity.ExptStatus_Processing
			if kind == "finished" {
				status = entity.ExptStatus_Success
			}
			exptRepo.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(&entity.Experiment{ID: 7, SpaceID: 9, Status: status}, nil)
			if kind == "finished" {
				manager.EXPECT().CompleteRun(gomock.Any(), int64(7), int64(8), int64(9), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				manager.EXPECT().CompleteExpt(gomock.Any(), int64(7), gomock.Any(), int64(9), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}

			handler := impl.HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error {
				return errors.New("list evaluation set items failed")
			})
			require.NoError(t, handler(context.Background(), event))
			require.Equal(t, kind == "processing", manager.terminated)
		})
	}
}
