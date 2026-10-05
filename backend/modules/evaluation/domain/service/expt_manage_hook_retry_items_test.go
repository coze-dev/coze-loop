// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHookRetryItemsStartPinsSchedule(t *testing.T) {
	m, store, cfg, quota, pub := scheduleRetryFixture(t)
	starter, ok := any(m).(interface {
		StartRetryItemsWithHookSchedule(context.Context, int64, int64, int, []int64, *entity.Session, map[string]string) (bool, int64, bool, error)
	})
	require.True(t, ok, "RetryItems needs a durable start entry, not the legacy event publisher")
	handled, run, retried, err := starter.StartRetryItemsWithHookSchedule(context.Background(), 22, 11, 4, []int64{101}, &entity.Session{UserID: "trusted", AppID: 7}, map[string]string{"route": "original"})
	require.NoError(t, err)
	require.True(t, handled)
	require.False(t, retried)
	require.Equal(t, int64(77), run)
	require.Equal(t, 1, store.creates)
	require.Len(t, pub.sent, 1)
	require.Equal(t, 1, cfg.reads)
	snapshot, err := m.hooks.Codec.DecodeSnapshot(context.Background(), store.run.State.Key, "ppe_original", store.run.Snapshot)
	require.NoError(t, err)
	require.NotNil(t, snapshot.Input().Execution)
	require.NotNil(t, snapshot.Input().Schedule)
	require.Equal(t, "trusted", snapshot.Input().Schedule.Session.UserID)
	require.Equal(t, []int64{101}, store.initial.RunLog.GetItemIDs())
	first := pub.sent[0]
	store.run.ExecutionStarted = true
	store.run.State.Status = entity.ExptStatus_Processing
	store.initial.RunLog.Status = int64(entity.ExptStatus_Processing)
	cfg.yield = false
	require.NoError(t, m.PublishRetryItemsContinuation(context.Background(), store.run.State.Key))
	require.Equal(t, first, pub.sent[1])
	require.Equal(t, 1, cfg.reads)
	require.Equal(t, 1, quota.calls, "continuations must not re-acquire or refresh quota")
	pub.sent = nil
	store.initial.LatestRunID = 99
	require.NoError(t, m.PublishRetryItemsContinuation(context.Background(), store.run.State.Key))
	require.Empty(t, pub.sent)
	store.initial.LatestRunID = run
	closed, err := entity.BeginHookFinalize(&store.run.State, store.run.State.Key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated})
	require.NoError(t, err)
	store.run.State = closed.State
	require.NoError(t, m.PublishRetryItemsContinuation(context.Background(), store.run.State.Key))
	require.Empty(t, pub.sent)
}

type retryItemsScheduleAppendStore struct {
	*scheduleRetryStore
}

func (s *retryItemsScheduleAppendStore) AppendHookRunItems(_ context.Context, in entity.HookAppendRunItemsInput) error {
	if in.Key != s.run.State.Key {
		return entity.ErrHookStoreConflict
	}
	return s.initial.RunLog.AppendItemIDs(in.ItemIDs)
}

func retryItemsQuotaFixture(t *testing.T, started bool) (*ExptMangerImpl, *scheduleRetryStore, *scheduleConfiger, *scheduleQuota, *scheduleEvents, *entity.ExptScheduleEvent) {
	t.Helper()
	m, s, c, q, p := scheduleRetryFixture(t)
	if !started {
		q.quota = &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{999: 1700000000}}
	}
	handled, runID, retried, err := m.StartRetryItemsWithHookSchedule(context.Background(), 22, 11, 4, []int64{101}, &entity.Session{UserID: "trusted", AppID: 7}, map[string]string{"route": "original"})
	require.True(t, handled)
	require.False(t, retried)
	require.Equal(t, int64(77), runID)
	if started {
		require.NoError(t, err)
		s.run.ExecutionStarted = true
		s.run.State.Status = entity.ExptStatus_Processing
		s.initial.RunLog.Status = int64(entity.ExptStatus_Processing)
	} else {
		require.Error(t, err)
		require.Empty(t, p.sent)
	}
	snapshot, err := m.hooks.Codec.DecodeSnapshot(context.Background(), s.run.State.Key, "ppe_original", s.run.Snapshot)
	require.NoError(t, err)
	in := snapshot.Input()
	in.Schedule.CreatedAt = 1700000001
	snapshot, err = entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	s.run.Snapshot, err = m.hooks.Codec.EncodeSnapshot(context.Background(), "key", snapshot)
	require.NoError(t, err)
	event, err := in.Schedule.Event(s.run.State.Key, "ppe_original", entity.EvaluationModeRetryItems, "trusted")
	require.NoError(t, err)
	q.calls, p.sent = 0, nil
	q.quota = &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{999: 1700000000}}
	c.yield = false
	m.hooks.Initialization = &retryItemsScheduleAppendStore{scheduleRetryStore: s}
	locker := lockmocks.NewMockILocker(gomock.NewController(t))
	m.mutex = locker
	locker.EXPECT().BackoffLockWithValue(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), time.Second).Return(false, "77", nil).AnyTimes()
	locker.EXPECT().Exists(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	return m, s, c, q, p, event
}

func TestHookRetryItemsQuotaRecovery(t *testing.T) {
	for _, entry := range []string{"continuation", "append_start"} {
		for _, started := range []bool{false, true} {
			state := "pending"
			if started {
				state = "processing"
			}
			t.Run(entry+"/"+state, func(t *testing.T) {
				m, s, c, q, p, original := retryItemsQuotaFixture(t, started)
				before := *s.run
				publish := func(item int64) error {
					if entry == "continuation" {
						return m.PublishRetryItemsContinuation(context.Background(), s.run.State.Key)
					}
					handled, id, reused, err := m.StartRetryItemsWithHookSchedule(context.Background(), 22, 11, 9, []int64{item}, &entity.Session{UserID: "new-caller", AppID: 99}, map[string]string{"route": "changed"})
					require.True(t, handled)
					require.True(t, reused)
					require.Equal(t, int64(77), id)
					return err
				}
				err := publish(102)
				if started {
					require.NoError(t, err)
					require.Zero(t, q.calls, "active continuations must not acquire or refresh a quota slot")
					require.Len(t, p.sent, 1)
					require.Equal(t, original, p.sent[0])
					require.Equal(t, map[int64]int64{999: 1700000000}, q.quota.ExptID2RunTime)
				} else {
					require.Error(t, err, "Pending recovery must consume the startup quota denial")
					status, ok := errorx.FromStatusError(err)
					require.True(t, ok)
					require.Equal(t, int32(errno.ExperimentRunningCountLimitCode), status.Code())
					require.Equal(t, 1, q.calls)
					require.Empty(t, p.sent, "quota denied: no MQ request")
					require.Equal(t, map[int64]int64{999: 1700000000}, q.quota.ExptID2RunTime)
					delete(q.quota.ExptID2RunTime, 999)
					require.NoError(t, publish(103), "the same durable Run must recover after capacity is released")
					require.Len(t, p.sent, 1)
					require.Equal(t, original, p.sent[0])
					require.Equal(t, map[int64]int64{22: 1700000001}, q.quota.ExptID2RunTime)
					require.Equal(t, 2, q.calls)
				}
				require.Equal(t, before, *s.run, "recovery must preserve identity, snapshot, before and original seed")
				require.Equal(t, 1, s.creates)
				require.Equal(t, 1, c.reads, "retry-yield configuration is frozen")
				if entry == "append_start" {
					items := []int64{101, 102}
					if !started {
						items = append(items, 103)
					}
					require.Equal(t, items, s.initial.RunLog.GetItemIDs(), "quota denial does not discard accepted items")
				}
			})
		}
	}
}

func TestHookRetryItemsQuotaRecoveryMQError(t *testing.T) {
	for _, started := range []bool{false, true} {
		m, s, _, q, p, original := retryItemsQuotaFixture(t, started)
		q.quota.ExptID2RunTime = map[int64]int64{22: 1700000001}
		p.err = errors.New("lost MQ receipt")
		require.ErrorIs(t, m.PublishRetryItemsContinuation(context.Background(), s.run.State.Key), p.err)
		p.err = nil
		require.NoError(t, m.PublishRetryItemsContinuation(context.Background(), s.run.State.Key))
		require.Len(t, p.sent, 2)
		require.Equal(t, original, p.sent[0])
		require.Equal(t, original, p.sent[1])
		require.Equal(t, map[int64]int64{22: 1700000001}, q.quota.ExptID2RunTime)
		if started {
			require.Zero(t, q.calls)
		} else {
			require.Equal(t, 2, q.calls)
		}
	}
}

func TestHookRetryItemsInitializedBeforeFirstAdmissionRecovers(t *testing.T) {
	for _, entry := range []string{"continuation", "append_start"} {
		t.Run(entry, func(t *testing.T) {
			m, s, c, q, p, original := retryItemsQuotaFixture(t, true)
			s.run.ExecutionStarted = false
			s.run.PlanReady = true
			digest := entity.NewHookPlanDigest()
			s.run.PlanCount, s.run.PlanHash = digest.Count, digest.Hash
			var err error
			s.run.PlanCursor, err = (entity.HookRetryItemsCursor{Version: 2, Key: s.run.State.Key, Fingerprint: strings.Repeat("b", 64), Phase: "tail", Published: digest}).Encode()
			require.NoError(t, err)
			s.initial.RunLog.ItemIds = []entity.ExptRunLogItems{{ItemIDs: []int64{102}}}
			_, err = entity.DecodeHookRetryItemsCursor(s.run.PlanCursor, s.run.State.Key, s.initial.RunLog.ItemIds, s.run.PlanCount, s.run.PlanHash)
			require.NoError(t, err)
			before := *s.run
			if entry == "continuation" {
				err = m.PublishRetryItemsContinuation(context.Background(), s.run.State.Key)
			} else {
				var handled, reused bool
				var runID int64
				handled, runID, reused, err = m.StartRetryItemsWithHookSchedule(context.Background(), 22, 11, 9, []int64{103}, &entity.Session{UserID: "new-caller", AppID: 99}, map[string]string{"route": "changed"})
				require.True(t, handled)
				require.True(t, reused)
				require.Equal(t, int64(77), runID)
				require.Equal(t, []int64{102, 103}, s.initial.RunLog.GetItemIDs())
			}
			require.NoError(t, err)
			require.Len(t, p.sent, 1, "Processing after initialization must recover even when no item has been admitted")
			require.Equal(t, original, p.sent[0])
			require.Zero(t, q.calls, "do not reacquire or refresh the startup slot")
			require.Equal(t, map[int64]int64{999: 1700000000}, q.quota.ExptID2RunTime)
			require.Equal(t, before, *s.run)
			require.Equal(t, 1, c.reads)
		})
	}
}
