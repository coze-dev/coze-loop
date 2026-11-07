// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"testing"

	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDeleteQuotaFailurePreservesExperiment(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, scanFailure := range []bool{false, true} {
			t.Run(fmt.Sprintf("batch=%t/scan=%t", batch, scanFailure), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				m := newTestExptManager(ctrl)
				expt := enforceExpt()
				guard := &fakeGuard{}
				if !scanFailure {
					guard.releaseErr = assert.AnError
				}
				m.centralGuard = guard
				expts := m.exptRepo.(*repomocks.MockIExperimentRepo)
				if batch {
					expts.EXPECT().MGetByID(gomock.Any(), []int64{100}, int64(7)).Return([]*entity.Experiment{expt}, nil)
				} else {
					expts.EXPECT().GetByID(gomock.Any(), int64(100), int64(7)).Return(expt, nil)
				}
				var scanErr error
				if scanFailure {
					scanErr = assert.AnError
				}
				m.itemResultRepo.(*repomocks.MockIExptItemResultRepo).EXPECT().ScanItemRunLogs(gomock.Any(), int64(100), int64(200), gomock.Any(), gomock.Any(), gomock.Any(), int64(7)).Return([]*entity.ExptItemResultRunLog{{ItemID: 11}}, int64(1), scanErr)
				var err error
				if batch {
					err = m.MDelete(context.Background(), []int64{100}, 7, &entity.Session{})
				} else {
					err = m.Delete(context.Background(), 100, 7, &entity.Session{})
				}
				require.ErrorIs(t, err, assert.AnError)
			})
		}
	}
}

func TestLegacyRetryQuotaFailureDoesNotCreateRun(t *testing.T) {
	for _, retryItems := range []bool{false, true} {
		name := "log_run"
		if retryItems {
			name = "log_retry_items"
		}
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := newTestExptManager(ctrl)
			m.centralGuard = &fakeGuard{releaseErr: assert.AnError}
			m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(100), int64(7)).Return(enforceExpt(), nil)
			m.itemResultRepo.(*repomocks.MockIExptItemResultRepo).EXPECT().ScanItemRunLogs(gomock.Any(), int64(100), int64(200), gomock.Any(), gomock.Any(), gomock.Any(), int64(7)).Return([]*entity.ExptItemResultRunLog{{ItemID: 11}}, int64(1), nil)
			locker := m.mutex.(*lockmocks.MockILocker)
			locker.EXPECT().UnlockForce(gomock.Any(), "expt_run_mutex_lock:100").Return(true, nil)
			m.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), int64(100), int64(201), int64(7), gomock.Any()).Return(nil)
			var err error
			if retryItems {
				m.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(201), nil)
				locker.EXPECT().BackoffLockWithValue(gomock.Any(), "expt_run_mutex_lock:100", "201", gomock.Any(), gomock.Any()).Return(true, "", nil)
				_, _, err = m.LogRetryItemsRun(context.Background(), 100, entity.EvaluationModeRetryItems, 7, []int64{11}, &entity.Session{})
			} else {
				locker.EXPECT().LockBackoff(gomock.Any(), "expt_run_mutex_lock:100", gomock.Any(), gomock.Any()).Return(true, nil)
				m.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecRun(int64(7), int64(entity.EvaluationModeFailRetry))
				err = m.LogRun(context.Background(), 100, 201, entity.EvaluationModeFailRetry, 7, nil, &entity.Session{})
			}
			require.ErrorIs(t, err, assert.AnError)
		})
	}
}
