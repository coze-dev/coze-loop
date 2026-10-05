// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type scheduleStore struct {
	repo.IHookRepo
	repo.IHookRunInitializationRepo
	run     *entity.HookStoredRun
	initial entity.HookRunInitialization
	creates int
}

func (s *scheduleStore) ReadRunInitialization(context.Context, entity.HookRunKey) (*entity.HookRunInitialization, error) {
	out := s.initial
	return &out, nil
}
func (s *scheduleStore) GetRun(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error) {
	if s.run == nil {
		return nil, entity.ErrHookStoreMissing
	}
	out := *s.run
	return &out, nil
}
func (s *scheduleStore) CreateRunWithHooks(_ context.Context, in entity.HookCreateRunInput) (entity.HookStoreResult, error) {
	s.creates++
	s.initial.Managed = true
	s.initial.RunLog = in.RunLog
	s.initial.LatestRunID = in.Key.RunID
	s.run = &entity.HookStoredRun{State: entity.HookRunState{Key: in.Key, Status: entity.ExptStatus_Pending, Gate: entity.HookGateWaiting, Finalize: entity.HookFinalizeNone,
		Before: entity.HookOperation{ID: in.Before.OperationID, Status: entity.HookOperationPending}, After: entity.HookOperation{Status: entity.HookOperationDisabled}}, Snapshot: in.Snapshot, Mode: entity.ExptRunMode(in.RunLog.Mode), CreatedBy: in.RunLog.CreatedBy, SourceRunID: in.SourceRunID}
	return entity.HookStoreResult{Changed: true, Run: s.run}, nil
}

type scheduleConfiger struct {
	component.IConfiger
	yield bool
	reads int
}

func (c *scheduleConfiger) GetRetryYieldEnabled(context.Context, int64) bool {
	c.reads++
	return c.yield
}
func (c *scheduleConfiger) GetExptExecConf(context.Context, int64) *entity.ExptExecConf {
	return &entity.ExptExecConf{SpaceExptConcurLimit: 1, ZombieIntervalSecond: 3600}
}

type scheduleQuota struct {
	repo.QuotaRepo
	quota  *entity.QuotaSpaceExpt
	before func()
	calls  int
}

func (q *scheduleQuota) CreateOrUpdate(ctx context.Context, space int64, fn func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), session *entity.Session) error {
	q.calls++
	if q.before != nil {
		q.before()
	}
	out, changed, err := fn(q.quota)
	if err == nil && changed {
		q.quota = out
	}
	return err
}

type scheduleEvents struct {
	events.ExptEventPublisher
	sent []*entity.ExptScheduleEvent
	err  error
}

func (p *scheduleEvents) PublishExptScheduleEvent(ctx context.Context, event *entity.ExptScheduleEvent, delay *time.Duration) error {
	p.sent = append(p.sent, event)
	return p.err
}

type scheduleExpts struct {
	repo.IExperimentRepo
	expt *entity.Experiment
}

func (p scheduleExpts) GetByID(context.Context, int64, int64) (*entity.Experiment, error) {
	return p.expt, nil
}

func scheduleManagerFixture(t *testing.T) (*ExptMangerImpl, *scheduleStore, *scheduleConfiger, *scheduleQuota, *scheduleEvents) {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	store := &scheduleStore{initial: entity.HookRunInitialization{HooksEnabled: true, ConfigRevision: "revision"}}
	config := &scheduleConfiger{yield: true}
	quota := &scheduleQuota{}
	publisher := &scheduleEvents{}
	m.configer = config
	m.quotaRepo = quota
	m.publisher = publisher
	m.exptRepo = scheduleExpts{expt: &entity.Experiment{ID: 22, SpaceID: 11, Name: "original", ExptType: entity.ExptType_Offline, NotificationConf: &entity.ExptNotificationConf{}}}
	identity, err := hookinfra.NewIdentityProvider(nil, 0)
	require.NoError(t, err)
	m.hooks = &ExptManagerHookDependencies{Runs: store, Initialization: store, Codec: hookinfra.NewStorageCodec(&managerProtector{}),
		Identity: identity, Runtime: &managerHookRuntime{admission: true}, Wake: &managerHookWake{}, ExecutionScope: "ppe_original", SnapshotKeyID: "key"}
	m.mutex.(*lockmocks.MockILocker).EXPECT().BackoffLockWithValue(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), time.Second).Return(true, "", nil).AnyTimes()
	m.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(77), nil).AnyTimes()
	m.mtr.(*metricmocks.MockExptMetric).EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).AnyTimes()
	return m, store, config, quota, publisher
}

func TestHookScheduleStartPersistsBeforePublicationAndReplays(t *testing.T) {
	m, store, config, quota, publisher := scheduleManagerFixture(t)
	m.hooks.Configs = scheduleConfigReader{}
	publisher.err = errors.New("MQ unavailable")
	input := map[string]string{"route": "original", entity.RetryYieldExtKey: "false"}
	user := &entity.Session{UserID: "trusted", AppID: 7}
	handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, user, entity.EvaluationModeSubmit, input)
	require.True(t, handled)
	require.ErrorIs(t, err, publisher.err)
	require.Equal(t, 1, store.creates)
	require.Len(t, publisher.sent, 1)
	snapshot, err := m.hooks.Codec.DecodeSnapshot(context.Background(), store.run.State.Key, "ppe_original", store.run.Snapshot)
	require.NoError(t, err)
	seed := snapshot.Input().Schedule
	require.NotNil(t, seed)
	require.Equal(t, int32(7), seed.Session.AppID)
	require.Equal(t, "true", seed.Ext[entity.RetryYieldExtKey])
	require.Equal(t, "false", input[entity.RetryYieldExtKey])
	first := publisher.sent[0]
	originalHash := store.run.Snapshot.Hash
	config.yield = false
	publisher.err = nil
	handled, err = m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, user, entity.EvaluationModeSubmit, input)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, config.reads)
	require.Equal(t, 1, store.creates)
	require.Len(t, publisher.sent, 2)
	require.Equal(t, first, publisher.sent[1])
	require.Equal(t, originalHash, store.run.Snapshot.Hash)
	require.Equal(t, seed.CreatedAt, quota.quota.ExptID2RunTime[22])
}

type scheduleConfigReader struct{ repo.IHookConfigRepo }

func (scheduleConfigReader) GetConfig(context.Context, hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error) {
	return &entity.HookConfigRecord{Revision: "revision", Config: managerEnabledConfig(true, false)}, nil
}

func TestHookScheduleLegacyStarterDoesNothing(t *testing.T) {
	m, _, _, _, p := scheduleManagerFixture(t)
	m.hooks = nil
	handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, entity.EvaluationModeSubmit, nil)
	require.False(t, handled)
	require.NoError(t, err)
	require.Empty(t, p.sent)
}

func startedScheduleFixture(t *testing.T) (*ExptMangerImpl, *scheduleStore, *scheduleQuota, *scheduleEvents, *entity.ExptScheduleEvent) {
	t.Helper()
	m, s, _, q, p := scheduleManagerFixture(t)
	m.hooks.Configs = scheduleConfigReader{}
	handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted", AppID: 7}, entity.EvaluationModeSubmit, map[string]string{"route": "original"})
	require.True(t, handled)
	require.NoError(t, err)
	require.Len(t, p.sent, 1)
	event := p.sent[0]
	p.sent = nil
	q.calls = 0
	return m, s, q, p, event
}

func TestHookSchedulePublishStopsClosedSupersededOrStartedRun(t *testing.T) {
	for _, change := range []string{"cancel", "successor", "closed", "started", "processing", "terminal"} {
		t.Run(change, func(t *testing.T) {
			m, s, q, p, event := startedScheduleFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			q.before = func() {
				switch change {
				case "cancel":
					cancel()
				case "successor":
					s.initial.LatestRunID = 99
				case "closed":
					s.run.State.Gate = entity.HookGateClosed
				case "started":
					s.run.ExecutionStarted = true
				case "processing":
					s.run.State.Status = entity.ExptStatus_Processing
					s.initial.RunLog.Status = int64(entity.ExptStatus_Processing)
				case "terminal":
					s.initial.RunLog.Status = int64(entity.ExptStatus_Success)
				}
			}
			err := m.PublishHookSchedule(ctx, event)
			if change == "cancel" {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, p.sent)
		})
	}
}

func TestHookSchedulePublishRejectsAlteredOriginalEvent(t *testing.T) {
	for _, change := range []string{"deadline", "retry", "ext", "identity", "app", "run", "mode"} {
		t.Run(change, func(t *testing.T) {
			m, _, q, p, event := startedScheduleFixture(t)
			switch change {
			case "deadline":
				event.CreatedAt++
			case "retry":
				event.ItemRetryTimes++
			case "ext":
				event.Ext[entity.RetryYieldExtKey] = "false"
			case "identity":
				event.Session.UserID = "another"
			case "app":
				event.Session.AppID++
			case "run":
				event.ExptRunID++
			case "mode":
				event.ExptRunMode = entity.EvaluationModeTrialRun
			}
			require.Error(t, m.PublishHookSchedule(context.Background(), event))
			require.Empty(t, p.sent)
			require.Zero(t, q.calls)
		})
	}
}

func TestHookScheduleStartRejectsChangedReplayWithoutRewritingSnapshot(t *testing.T) {
	for _, change := range []string{"retry", "ext", "identity", "app"} {
		t.Run(change, func(t *testing.T) {
			m, s, _, p, _ := startedScheduleFixture(t)
			hash := s.run.Snapshot.Hash
			retry := 4
			ext := map[string]string{"route": "original"}
			user := &entity.Session{UserID: "trusted", AppID: 7}
			switch change {
			case "retry":
				retry++
			case "ext":
				ext["route"] = "changed"
			case "identity":
				user.UserID = "changed"
			case "app":
				user.AppID++
			}
			handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, retry, user, entity.EvaluationModeSubmit, ext)
			require.True(t, handled)
			require.ErrorIs(t, err, entity.ErrHookStoreConflict)
			require.Equal(t, 1, s.creates)
			require.Equal(t, hash, s.run.Snapshot.Hash)
			require.Empty(t, p.sent)
		})
	}
}

func TestHookScheduleTrialCapturesSelectionBeforeSnapshot(t *testing.T) {
	m, s, _, _, p := scheduleManagerFixture(t)
	m.hooks.Configs = scheduleConfigReader{}
	m.exptRepo.(scheduleExpts).expt.TrialRunItemCount = 2
	handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, entity.EvaluationModeTrialRun, map[string]string{"__item_ids": "[101,102]"})
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, []int64{101, 102}, s.initial.RunLog.GetItemIDs())
	require.Equal(t, entity.EvaluationModeTrialRun, p.sent[0].ExptRunMode)
	snapshot, err := m.hooks.Codec.DecodeSnapshot(context.Background(), s.run.State.Key, "ppe_original", s.run.Snapshot)
	require.NoError(t, err)
	require.True(t, snapshot.Input().Selection.HasExplicitItemIDs)
	require.Equal(t, "[101,102]", snapshot.Input().Schedule.Ext["__item_ids"])
}

type scheduleNoNotificationRead struct {
	repo.IExperimentRepo
	t *testing.T
}

func (p scheduleNoNotificationRead) GetByID(context.Context, int64, int64) (*entity.Experiment, error) {
	p.t.Fatal("closed replay must not emit start notifications")
	return nil, nil
}

func TestHookScheduleStartClosedReplayHasNoNotification(t *testing.T) {
	m, s, q, p, _ := startedScheduleFixture(t)
	m.exptRepo = scheduleNoNotificationRead{t: t}
	q.before = func() { s.run.State.Gate = entity.HookGateClosed }
	handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted", AppID: 7}, entity.EvaluationModeSubmit, map[string]string{"route": "original"})
	require.True(t, handled)
	require.NoError(t, err)
	require.Empty(t, p.sent)
}

func TestHookSchedulePublishPreservesOldDeadlineAndQuotaTimestamp(t *testing.T) {
	m, s, q, p, _ := startedScheduleFixture(t)
	snapshot, err := m.hooks.Codec.DecodeSnapshot(context.Background(), s.run.State.Key, "ppe_original", s.run.Snapshot)
	require.NoError(t, err)
	in := snapshot.Input()
	in.Schedule.CreatedAt = 1700000001
	snapshot, err = entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	s.run.Snapshot, err = m.hooks.Codec.EncodeSnapshot(context.Background(), "key", snapshot)
	require.NoError(t, err)
	q.quota = &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{22: 1700000001}}
	event, err := in.Schedule.Event(s.run.State.Key, "ppe_original", entity.EvaluationModeSubmit, "trusted")
	require.NoError(t, err)
	require.NoError(t, m.PublishHookSchedule(context.Background(), event))
	require.Len(t, p.sent, 1)
	require.Equal(t, int64(1700000001), p.sent[0].CreatedAt)
	require.Equal(t, int64(1700000001), q.quota.ExptID2RunTime[22])
}

func TestHookSchedulePublishQuotaDeniedNeverPublishes(t *testing.T) {
	m, _, q, p, event := startedScheduleFixture(t)
	q.quota = &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{999: 1700000001}}
	require.Error(t, m.PublishHookSchedule(context.Background(), event))
	require.Empty(t, p.sent)
	require.Equal(t, map[int64]int64{999: 1700000001}, q.quota.ExptID2RunTime)
}

func TestHookScheduleNotificationModeParity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mode      entity.ExptRunMode
		wantCards int
	}{
		{"submit", entity.EvaluationModeSubmit, 1},
		{"trial", entity.EvaluationModeTrialRun, 1},
		{"fail_retry", entity.EvaluationModeFailRetry, 0},
		{"retry_all", entity.EvaluationModeRetryAll, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, store, _, quota, publisher := scheduleRetryFixture(t)
			expt := m.exptRepo.(scheduleExpts).expt
			expt.NotificationConf = nil
			expt.CreatedBy = "notify@example.test"
			expt.Status = entity.ExptStatus_Pending
			// Mirror the committed Latest projection before the notification read.
			quota.before = func() { expt.LatestRunID = store.initial.LatestRunID }
			cards := 0
			m.notifyRPCAdapter.(*rpcmocks.MockINotifyRPCAdapter).EXPECT().
				SendMessageCard(gomock.Any(), "notify@example.test", "email", gomock.Any(), gomock.Any()).
				DoAndReturn(func(context.Context, string, string, string, map[string]string) error {
					cards++
					return nil
				}).AnyTimes()
			handled, err := m.StartRunWithHookSchedule(context.Background(), 22, 33, 11, 4, &entity.Session{UserID: "trusted"}, tc.mode, nil)
			require.True(t, handled)
			require.NoError(t, err)
			require.Len(t, publisher.sent, 1)
			require.Equal(t, tc.mode, publisher.sent[0].ExptRunMode)
			require.Equal(t, tc.wantCards, cards)
		})
	}
}
