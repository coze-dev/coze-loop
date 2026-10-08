// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type startConfig struct {
	component.IConfiger
	yieldReads int
}

func (c *startConfig) GetRetryYieldEnabled(context.Context, int64) bool { c.yieldReads++; return true }
func (c *startConfig) GetExptExecConf(context.Context, int64) *entity.ExptExecConf {
	return &entity.ExptExecConf{SpaceExptConcurLimit: 1, ZombieIntervalSecond: 3600}
}

type startProtector struct{}

func (startProtector) Protect(_ context.Context, _ string, b []byte) ([]byte, error) {
	return append([]byte(nil), b...), nil
}
func (startProtector) Unprotect(_ context.Context, _ string, b []byte) ([]byte, error) {
	return append([]byte(nil), b...), nil
}

type startWake struct{}

func (startWake) PublishWake(context.Context, entity.HookWakeEvent) error { return nil }

type startStore struct {
	repo.IHookRepo
	repo.IHookRunInitializationRepo
	repo.IHookConfigRepo
	codec                hookcomponent.StorageCodec
	initial              entity.HookRunInitialization
	run                  *entity.HookStoredRun
	config               *entity.LifecycleHookConf
	authorized           bool
	configReads, creates int
	readErr              error
	source               *entity.Experiment
	legacyLogs           *startLogs
}

func (s *startStore) GetConfig(_ context.Context, owner hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error) {
	s.configReads++
	if !s.authorized {
		return nil, errors.New("config read before authorization")
	}
	if owner != (hookcomponent.ConfigOwner{WorkspaceID: 7, ObjectID: 42, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "test-scope"}) {
		return nil, entity.ErrHookConfigStorage
	}
	if s.readErr != nil {
		return nil, s.readErr
	}
	return &entity.HookConfigRecord{Revision: "v1", Config: s.config}, nil
}
func (s *startStore) ReadRunInitialization(context.Context, entity.HookRunKey) (*entity.HookRunInitialization, error) {
	copy := s.initial
	return &copy, nil
}
func (s *startStore) CreateRunWithoutHooks(ctx context.Context, log *entity.ExptRunLog, latest int64, revision string) (bool, error) {
	if latest != s.initial.LatestRunID || revision != s.initial.ConfigRevision || s.initial.Managed || s.initial.HooksEnabled {
		return false, entity.ErrHookStoreConflict
	}
	if err := s.legacyLogs.Create(ctx, log); err != nil {
		return false, err
	}
	s.initial.RunLog = log
	s.initial.LatestRunID = log.ExptRunID
	s.source.LatestRunID = log.ExptRunID
	return true, nil
}
func (s *startStore) GetRun(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error) {
	if s.run == nil {
		return nil, entity.ErrHookStoreMissing
	}
	copy := *s.run
	return &copy, nil
}
func (s *startStore) CreateRunWithHooks(ctx context.Context, in entity.HookCreateRunInput) (entity.HookStoreResult, error) {
	snap, err := s.codec.DecodeSnapshot(ctx, in.Key, "test-scope", in.Snapshot)
	if err != nil {
		return entity.HookStoreResult{}, err
	}
	if snap.Input().Schedule == nil {
		return entity.HookStoreResult{}, errors.New("missing original schedule seed")
	}
	s.creates++
	s.initial.Managed = true
	s.initial.RunLog = in.RunLog
	s.initial.LatestRunID = in.Key.RunID
	s.run = &entity.HookStoredRun{State: entity.HookRunState{Key: in.Key, Status: entity.ExptStatus_Pending, Gate: entity.HookGateWaiting, Finalize: entity.HookFinalizeNone, Before: entity.HookOperation{ID: in.Before.OperationID, Status: entity.HookOperationPending}, After: entity.HookOperation{Status: entity.HookOperationDisabled}}, Snapshot: in.Snapshot, Mode: entity.ExptRunMode(in.RunLog.Mode), CreatedBy: in.RunLog.CreatedBy}
	return entity.HookStoreResult{Changed: true, Run: s.run}, nil
}

type startExpts struct {
	repo.IExperimentRepo
	expt    *entity.Experiment
	updates int
}

func (s *startExpts) GetByID(context.Context, int64, int64) (*entity.Experiment, error) {
	return s.expt, nil
}
func (s *startExpts) MGetByID(context.Context, []int64, int64) ([]*entity.Experiment, error) {
	return []*entity.Experiment{s.expt}, nil
}
func (s *startExpts) Update(_ context.Context, x *entity.Experiment) error {
	s.updates++
	s.expt.LatestRunID = x.LatestRunID
	return nil
}

type startLogs struct {
	repo.IExptRunLogRepo
	creates int
	user    string
}

func (s *startLogs) Create(_ context.Context, log *entity.ExptRunLog) error {
	s.creates++
	s.user = log.CreatedBy
	return nil
}

func (s *startLogs) Update(context.Context, int64, int64, map[string]any) error {
	return nil
}

type startQuota struct {
	repo.QuotaRepo
	quota *entity.QuotaSpaceExpt
	calls int
	err   error
}

func (s *startQuota) CreateOrUpdate(_ context.Context, _ int64, f func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
	s.calls++
	if s.err != nil {
		return s.err
	}
	q, changed, err := f(s.quota)
	if changed && err == nil {
		s.quota = q
	}
	return err
}

type startPublisher struct {
	events.ExptEventPublisher
	sent  []*entity.ExptScheduleEvent
	err   error
	delay time.Duration
}

func (p *startPublisher) PublishExptScheduleEvent(_ context.Context, event *entity.ExptScheduleEvent, delay *time.Duration) error {
	p.sent = append(p.sent, event)
	p.delay = gptr.Indirect(delay)
	return p.err
}

func newStartApplication(t *testing.T, installed bool, config *entity.LifecycleHookConf, authErr error) (*experimentApplication, *startStore, *startLogs, *startQuota, *startPublisher) {
	t.Helper()
	ctrl := gomock.NewController(t)
	ids := idmocks.NewMockIIDGenerator(ctrl)
	lock := lockmocks.NewMockILocker(ctrl)
	tracker := lwtmocks.NewMockILatestWriteTracker(ctrl)
	metrics := metricmocks.NewMockExptMetric(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	results := servicemocks.NewMockExptResultService(ctrl)
	aggregates := servicemocks.NewMockExptAggrResultService(ctrl)
	results.EXPECT().MGetStats(gomock.Any(), []int64{42}, int64(7), gomock.Any()).Return(nil, nil).AnyTimes()
	aggregates.EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), int64(7), []int64{42}).Return(nil, nil).AnyTimes()
	ids.EXPECT().GenID(gomock.Any()).Return(int64(71), nil).AnyTimes()
	lock.EXPECT().BackoffLockWithValue(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), time.Second).Return(true, "", nil).AnyTimes()
	lock.EXPECT().LockBackoff(gomock.Any(), gomock.Any(), gomock.Any(), time.Second).Return(true, nil).AnyTimes()
	lock.EXPECT().UnlockWithValue(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	lock.EXPECT().UnlockForce(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	tracker.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	metrics.EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).AnyTimes()
	codec := hookinfra.NewStorageCodec(startProtector{})
	store := &startStore{codec: codec, config: config, initial: entity.HookRunInitialization{HooksEnabled: hookApplicationEnabled(config), ConfigRevision: "v1"}}
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
		require.Equal(t, "42", p.ObjectID)
		require.Equal(t, consts.Run, gptr.Indirect(p.ActionObjects[0].Action))
		store.authorized = authErr == nil
		return authErr
	}).AnyTimes()
	expts := &startExpts{expt: &entity.Experiment{ID: 42, SpaceID: 7, EvalSetID: 11, EvalSetVersionID: 11, Name: "source", ExptType: entity.ExptType_Offline, CreatedBy: "trusted-user", NotificationConf: &entity.ExptNotificationConf{}}}
	store.source = expts.expt
	logs := &startLogs{}
	store.legacyLogs = logs
	quota := &startQuota{}
	publisher := &startPublisher{}
	configer := &startConfig{}
	manager := service.NewExptManager(results, expts, logs, nil, nil, nil, nil, configer, quota, lock, nil, publisher, nil, ids, metrics, tracker, service.NewEvaluationSetVersionServiceImpl(hookCreationDatasetRPC{}), service.NewEvaluationSetServiceImpl(hookCreationDatasetRPC{}), nil, nil, nil, aggregates, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if installed {
		identity, err := hookinfra.NewIdentityProvider(nil, 0)
		require.NoError(t, err)
		manager, err = service.NewExptManagerWithHooks(manager, service.ExptManagerHookDependencies{Initialization: store, Runs: store, Configs: store, Codec: codec, Identity: identity, Runtime: hookApplicationRuntime{enabled: true}, Wake: startWake{}, ExecutionScope: "test-scope", SnapshotKeyID: "operator-key"})
		require.NoError(t, err)
	}
	app := &experimentApplication{manager: manager, idgen: ids, auth: auth, hooks: &ExperimentHookApplicationDependencies{Configs: store, ExecutionScope: "test-scope"}}
	return app, store, logs, quota, publisher
}
func enabledStartConfig() *entity.LifecycleHookConf {
	return &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}
}
func startRequest() *expt.RunExperimentRequest {
	return &expt.RunExperimentRequest{WorkspaceID: gptr.Of(int64(7)), ExptID: gptr.Of(int64(42)), ExptType: gptr.Of(domain.ExptType_Offline), ItemRetryNum: gptr.Of(int32(4)), Session: &common.Session{UserID: gptr.Of(int64(999))}, Ext: map[string]string{"route": "original"}}
}

func TestLifecycleHookStartRealManagerPublishesExactlyOnce(t *testing.T) {
	for _, mqFails := range []bool{false, true} {
		t.Run(fmt.Sprint(mqFails), func(t *testing.T) {
			app, store, logs, quota, publisher := newStartApplication(t, true, enabledStartConfig(), nil)
			mqErr := errors.New("MQ unavailable")
			if mqFails {
				publisher.err = mqErr
			}
			req := startRequest()
			out, err := app.RunExperiment(hookCreationContext(), req)
			if mqFails {
				require.Nil(t, out)
				require.ErrorIs(t, err, mqErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(71), out.GetRunID())
			}
			require.Equal(t, 1, store.creates)
			require.Zero(t, logs.creates)
			require.Equal(t, 1, quota.calls)
			require.Len(t, publisher.sent, 1)
			require.Equal(t, 3*time.Second, publisher.delay)
			require.Equal(t, "trusted-user", publisher.sent[0].Session.UserID)
			require.Equal(t, 4, publisher.sent[0].ItemRetryTimes)
			require.Equal(t, "original", publisher.sent[0].Ext["route"])
			require.Equal(t, int64(999), req.Session.GetUserID())
		})
	}
}

func TestLifecycleHookStartMissingRuntimeCannotFallback(t *testing.T) {
	for _, hide := range []bool{false, true} {
		t.Run(fmt.Sprint(hide), func(t *testing.T) {
			app, store, logs, quota, publisher := newStartApplication(t, false, enabledStartConfig(), nil)
			quota.err = errors.New("legacy Run reached")
			if hide {
				app.manager = struct{ service.IExptManager }{app.manager}
			} else {
				_, ok := app.manager.(service.IHookRunScheduleStarter)
				require.True(t, ok, "type assertion alone cannot prove runtime installation")
			}
			out, err := app.RunExperiment(hookCreationContext(), startRequest())
			require.Nil(t, out)
			require.ErrorContains(t, err, "HOOK_SCHEDULE_RUNTIME_UNAVAILABLE")
			require.Equal(t, 1, store.configReads)
			require.Zero(t, logs.creates)
			require.Zero(t, quota.calls)
			require.Empty(t, publisher.sent)
		})
	}
}

func TestLifecycleHookStartLegacyConfigurationKeepsOldPath(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprint(disabled), func(t *testing.T) {
			var conf *entity.LifecycleHookConf
			if disabled {
				conf = &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}
			}
			app, store, logs, quota, publisher := newStartApplication(t, false, conf, nil)
			stop := errors.New("legacy Run quota storage unavailable")
			quota.err = stop
			out, err := app.RunExperiment(hookCreationContext(), startRequest())
			require.Nil(t, out)
			require.ErrorIs(t, err, stop)
			require.Equal(t, 1, logs.creates)
			require.Equal(t, "999", logs.user)
			require.Equal(t, 1, quota.calls)
			require.Zero(t, store.creates)
			require.Empty(t, publisher.sent)
		})
	}
}

func TestLifecycleHookStartReadRequiresAuthorization(t *testing.T) {
	denied := errors.New("denied")
	app, store, logs, quota, publisher := newStartApplication(t, false, enabledStartConfig(), denied)
	quota.err = errors.New("legacy Run reached")
	out, err := app.RunExperiment(hookCreationContext(), startRequest())
	require.Nil(t, out)
	require.ErrorIs(t, err, denied)
	require.Zero(t, store.configReads)
	require.Zero(t, logs.creates)
	require.Empty(t, publisher.sent)
}

func TestLifecycleHookStartUnhandledAndReadErrorsDoNotFallback(t *testing.T) {
	for _, mode := range []string{"unhandled", "read_error", "missing_config_dependency"} {
		t.Run(mode, func(t *testing.T) {
			app, store, logs, quota, publisher := newStartApplication(t, true, enabledStartConfig(), nil)
			want := entity.ErrHookConfigStorage
			if mode == "unhandled" {
				store.initial.HooksEnabled = false
			} else if mode == "read_error" {
				store.readErr = want
			} else {
				app.hooks.Configs = nil
			}
			out, err := app.RunExperiment(hookCreationContext(), startRequest())
			require.Nil(t, out)
			if mode == "unhandled" {
				require.ErrorContains(t, err, "HOOK_SCHEDULE_RUNTIME_UNAVAILABLE")
			} else {
				require.ErrorIs(t, err, want)
			}
			require.Zero(t, store.creates)
			require.Zero(t, logs.creates)
			require.Zero(t, quota.calls)
			require.Empty(t, publisher.sent)
		})
	}
}

func TestLifecycleHookStartLegacyApplicationWithoutHookServices(t *testing.T) {
	app, store, logs, quota, publisher := newStartApplication(t, false, nil, nil)
	app.hooks = nil
	stop := errors.New("legacy quota storage unavailable")
	quota.err = stop
	_, err := app.RunExperiment(hookCreationContext(), startRequest())
	require.ErrorIs(t, err, stop)
	require.Equal(t, 1, logs.creates)
	require.Equal(t, 1, quota.calls)
	require.Zero(t, store.configReads)
	require.Empty(t, publisher.sent)
}

func TestLifecycleHookStartTrialSeedUsesOriginalInput(t *testing.T) {
	app, store, logs, _, publisher := newStartApplication(t, true, enabledStartConfig(), nil)
	store.source.TrialRunItemCount = 2
	req := startRequest()
	req.TrialRunItemCount = gptr.Of(int64(2))
	req.Ext["__item_ids"] = "[101,102]"
	out, err := app.RunExperiment(hookCreationContext(), req)
	require.NoError(t, err)
	require.Equal(t, int64(71), out.GetRunID())
	require.Zero(t, logs.creates)
	require.Len(t, publisher.sent, 1)
	snapshot, err := store.codec.DecodeSnapshot(context.Background(), store.run.State.Key, "test-scope", store.run.Snapshot)
	require.NoError(t, err)
	require.Equal(t, entity.EvaluationModeTrialRun, publisher.sent[0].ExptRunMode)
	require.Equal(t, []int64{101, 102}, store.initial.RunLog.GetItemIDs())
	require.True(t, snapshot.Input().Selection.HasExplicitItemIDs)
	require.Equal(t, "[101,102]", snapshot.Input().Schedule.Ext["__item_ids"])
	require.Equal(t, "true", publisher.sent[0].Ext[entity.RetryYieldExtKey])
	require.NotContains(t, req.Ext, entity.RetryYieldExtKey)
}
