// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	lwtm "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	rm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type legacyAwareOwners struct{ lock *managerStateLock }

func (r legacyAwareOwners) ReadFinalizationOwner(ctx context.Context, key string) (string, error) {
	return r.lock.owner(key), ctx.Err()
}

type legacyAwareLogs struct {
	managerCompatLogs
	t            *testing.T
	beforeUpdate func()
}

func (r legacyAwareLogs) Update(ctx context.Context, expt, run int64, fields map[string]any) error {
	require.NoError(r.t, ctx.Err())
	if r.beforeUpdate != nil {
		r.beforeUpdate()
	}
	l := r.s.logs[run]
	require.Equal(r.t, expt, l.ExptID)
	l.Status = fields["status"].(int64)
	l.StatusMessage = fields["status_message"].([]byte)
	return nil
}

func TestLegacyAwareAdmissionCleanup(t *testing.T) {
	for _, scenario := range []string{"read_failure", "quota_failure", "successor_after_owner_read", "managed", "classification_failure"} {
		t.Run(scenario, func(t *testing.T) {
			managed := scenario == "managed"
			f, s := newManagerCompatFixture(t, managed)
			m := f.manager.(*ExptMangerImpl)
			ctx := context.Background()
			user := &entity.Session{UserID: "user"}
			if !managed {
				handled, err := m.StartRunWithHookSchedule(ctx, 20, 30, 10, 0, user, entity.EvaluationModeSubmit, nil)
				require.NoError(t, err)
				require.False(t, handled)
			}
			require.NoError(t, m.LogRunWithPlanSeed(ctx, 20, 30, entity.EvaluationModeSubmit, 10, "", user))
			initial, err := s.ReadRunInitialization(ctx, entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30})
			require.NoError(t, err)
			require.Equal(t, managed, initial.Managed)
			owner := f.locks.owner(f.key)
			require.Contains(t, owner, "hook_run:30:")
			m.finalization = &ExptManagerFinalizationDependencies{Owners: legacyAwareOwners{f.locks}}
			logs := legacyAwareLogs{managerCompatLogs: managerCompatLogs{s: s}, t: t}
			const successor = "hook_run:30:01234567890123456789012345678901"
			if scenario == "successor_after_owner_read" {
				logs.beforeUpdate = func() { f.locks.replace(f.key, successor) }
			}
			m.runLogRepo = logs
			if scenario == "classification_failure" {
				s.confirmErr = errors.New("state unavailable")
			}
			ctrl := gomock.NewController(t)
			tracker := lwtm.NewMockILatestWriteTracker(ctrl)
			tracker.EXPECT().CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeExperiment, int64(20)).Return(false)
			m.lwt = tracker
			expts := rm.NewMockIExperimentRepo(ctrl)
			m.exptRepo = expts
			failure := errors.New("read failed")
			if scenario == "quota_failure" {
				expts.EXPECT().MGetByID(gomock.Any(), []int64{20}, int64(10)).Return([]*entity.Experiment{{ID: 20, SpaceID: 10, LatestRunID: 30}}, nil)
				m.evaluationSetService.(*sm.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil).AnyTimes()
				m.exptResultService.(*sm.MockExptResultService).EXPECT().MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				m.exptAggrResultService.(*sm.MockExptAggrResultService).EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
				quota := rm.NewMockQuotaRepo(ctrl)
				failure = errors.New("quota rejected")
				quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(10), gomock.Any(), user).Return(failure)
				m.quotaRepo = quota
			} else {
				expts.EXPECT().MGetByID(gomock.Any(), []int64{20}, int64(10)).Return(nil, failure)
			}
			err = m.Run(ctx, 20, 30, 10, 0, user, entity.EvaluationModeSubmit, nil)
			require.ErrorIs(t, err, failure)
			require.Zero(t, f.locks.forceDeletes)
			if managed || scenario == "classification_failure" {
				require.EqualValues(t, entity.ExptStatus_Pending, s.logs[30].Status)
				require.Equal(t, owner, f.locks.owner(f.key))
			} else {
				require.EqualValues(t, entity.ExptStatus_Failed, s.logs[30].Status)
				require.Equal(t, failure.Error(), string(s.logs[30].StatusMessage))
				if scenario == "successor_after_owner_read" {
					require.Equal(t, successor, f.locks.owner(f.key))
				} else {
					require.Empty(t, f.locks.owner(f.key))
				}
			}
		})
	}
}
