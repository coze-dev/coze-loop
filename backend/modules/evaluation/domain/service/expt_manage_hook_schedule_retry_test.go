// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type scheduleRetryStore struct {
	*scheduleStore
	source     *entity.ExptRunLog
	readSource func()
}

func (s *scheduleRetryStore) ReadRunInitialization(ctx context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	if key.RunID == 30 {
		if s.readSource != nil {
			s.readSource()
		}
		return &entity.HookRunInitialization{LatestRunID: 30, RunLog: s.source}, nil
	}
	return s.scheduleStore.ReadRunInitialization(ctx, key)
}

func scheduleRetryFixture(t *testing.T) (*ExptMangerImpl, *scheduleRetryStore, *scheduleConfiger, *scheduleQuota, *scheduleEvents) {
	t.Helper()
	m, base, c, q, p := scheduleManagerFixture(t)
	m.hooks.Configs = scheduleConfigReader{}
	base.initial.LatestRunID = 30
	m.exptRepo.(scheduleExpts).expt.LatestRunID = 30
	m.exptRepo.(scheduleExpts).expt.EvalSetID = 11
	m.exptRepo.(scheduleExpts).expt.EvalSetVersionID = 11
	m.exptRepo.(scheduleExpts).expt.EvalConf = &entity.EvaluationConfiguration{}
	s := &scheduleRetryStore{scheduleStore: base, source: &entity.ExptRunLog{ID: 30, SpaceID: 11, ExptID: 22, ExptRunID: 30, CreatedBy: "old-user", Mode: 1, Status: int64(entity.ExptStatus_Failed)}}
	m.hooks.Initialization = s
	m.hooks.Runs = s
	q.quota = &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{22: 1700000000}}
	return m, s, c, q, p
}

func TestHookScheduleRetryStartPinsSourceAndReplays(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll} {
		t.Run(strconv.Itoa(int(mode)), func(t *testing.T) {
			m, s, c, q, p := scheduleRetryFixture(t)
			failure := errors.New("MQ unavailable")
			p.err = failure
			user := &entity.Session{UserID: "trusted", AppID: 7}
			ext := map[string]string{"route": "original"}
			handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, user, mode, ext)
			require.True(t, handled)
			require.ErrorIs(t, err, failure)
			require.Equal(t, 1, s.creates)
			require.NotNil(t, s.run.SourceRunID)
			require.Equal(t, int64(30), *s.run.SourceRunID)
			require.Equal(t, mode, s.run.Mode)
			snapshot, err := m.hooks.Codec.DecodeSnapshot(context.Background(), s.run.State.Key, "ppe_original", s.run.Snapshot)
			require.NoError(t, err)
			seed := snapshot.Input().Schedule
			hash := s.run.Snapshot.Hash
			first := p.sent[0]
			p.err = nil
			c.yield = false
			handled, err = m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, user, mode, ext)
			require.True(t, handled)
			require.NoError(t, err)
			require.Equal(t, 1, s.creates)
			require.Equal(t, 1, c.reads)
			require.Equal(t, hash, s.run.Snapshot.Hash)
			require.Equal(t, first, p.sent[1])
			require.Equal(t, seed.CreatedAt, q.quota.ExptID2RunTime[22])
			require.Equal(t, int64(30), *s.run.SourceRunID)
		})
	}
}

func TestHookScheduleRetryStartRejectsMissingSourceBeforeLease(t *testing.T) {
	for _, kind := range []string{"zero", "self", "missing", "wrong_identity"} {
		t.Run(kind, func(t *testing.T) {
			m, s, _, q, p := scheduleRetryFixture(t)
			switch kind {
			case "zero":
				s.initial.LatestRunID = 0
				m.exptRepo.(scheduleExpts).expt.LatestRunID = 0
			case "self":
				s.initial.LatestRunID = 33
				m.exptRepo.(scheduleExpts).expt.LatestRunID = 33
			case "missing":
				s.source = nil
			case "wrong_identity":
				s.source.ExptID = 999
			}
			m.mutex = lockmocks.NewMockILocker(gomock.NewController(t))
			handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, entity.EvaluationModeFailRetry, nil)
			require.True(t, handled)
			require.Error(t, err)
			require.NotErrorIs(t, err, entity.ErrHookFinalizationUnsupported)
			if kind == "zero" || kind == "missing" {
				require.ErrorIs(t, err, ErrHookScheduleRetrySourceMissing)
			} else {
				require.ErrorIs(t, err, entity.ErrHookStoreConflict)
			}
			require.Zero(t, s.creates)
			require.Zero(t, q.calls)
			require.Empty(t, p.sent)
		})
	}
}

func TestHookScheduleRetryStartCannotReplaceActiveLease(t *testing.T) {
	m, s, _, q, p := scheduleRetryFixture(t)
	locker := lockmocks.NewMockILocker(gomock.NewController(t))
	m.mutex = locker
	locker.EXPECT().BackoffLockWithValue(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), time.Second).Return(false, "old-owner", nil)
	_, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, entity.EvaluationModeRetryAll, nil)
	require.Error(t, err)
	require.NotErrorIs(t, err, entity.ErrHookFinalizationUnsupported)
	require.Zero(t, s.creates)
	require.Zero(t, q.calls)
	require.Empty(t, p.sent)
}

func TestHookScheduleRetryStartDoesNotSwitchSourceDuringInitialization(t *testing.T) {
	m, s, _, q, p := scheduleRetryFixture(t)
	s.readSource = func() { s.initial.LatestRunID = 99; m.exptRepo.(scheduleExpts).expt.LatestRunID = 99 }
	locker := lockmocks.NewMockILocker(gomock.NewController(t))
	m.mutex = locker
	locker.EXPECT().BackoffLockWithValue(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), time.Second).Return(true, "", nil)
	locker.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	_, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, entity.EvaluationModeFailRetry, nil)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Zero(t, s.creates)
	require.Zero(t, q.calls)
	require.Empty(t, p.sent)
}

func TestHookScheduleRetryLateReplayDoesNotTouchSuccessorQuota(t *testing.T) {
	m, s, _, q, p := scheduleRetryFixture(t)
	user := &entity.Session{UserID: "trusted"}
	_, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, user, entity.EvaluationModeFailRetry, nil)
	require.NoError(t, err)
	hash := s.run.Snapshot.Hash
	s.initial.LatestRunID = 99
	q.quota.ExptID2RunTime[22] = 1900000000
	p.sent = nil
	_, err = m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, user, entity.EvaluationModeFailRetry, nil)
	require.NoError(t, err)
	require.Empty(t, p.sent)
	require.Equal(t, int64(1900000000), q.quota.ExptID2RunTime[22])
	require.Equal(t, hash, s.run.Snapshot.Hash)
	require.Equal(t, int64(30), *s.run.SourceRunID)
}

func TestHookScheduleRetryStoredSourceIsRequiredForPublish(t *testing.T) {
	m, s, _, q, p := scheduleRetryFixture(t)
	_, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, entity.EvaluationModeRetryAll, nil)
	require.NoError(t, err)
	event := p.sent[0]
	p.sent = nil
	q.calls = 0
	s.run.SourceRunID = nil
	require.ErrorIs(t, m.PublishHookSchedule(context.Background(), event), ErrHookScheduleRetrySourceMissing)
	require.Empty(t, p.sent)
	require.Zero(t, q.calls)
}

func TestHookScheduleRetryStartKeepsUnsupportedBoundaries(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{0, entity.EvaluationModeAppend, entity.EvaluationModeRetryItems, 99} {
		m, s, _, q, p := scheduleRetryFixture(t)
		_, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, mode, nil)
		require.ErrorIs(t, err, entity.ErrHookFinalizationUnsupported)
		require.Zero(t, s.creates)
		require.Zero(t, q.calls)
		require.Empty(t, p.sent)
	}
	m, s, _, q, p := scheduleRetryFixture(t)
	m.exptRepo.(scheduleExpts).expt.ExptType = entity.ExptType_Online
	_, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, entity.EvaluationModeFailRetry, nil)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Zero(t, s.creates)
	require.Zero(t, q.calls)
	require.Empty(t, p.sent)
}
