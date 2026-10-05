// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

type managerProtector struct{ protects, unprotects int }

func (p *managerProtector) Protect(_ context.Context, _ string, plain []byte) ([]byte, error) {
	p.protects++
	return append([]byte(nil), plain...), nil
}
func (p *managerProtector) Unprotect(_ context.Context, _ string, cipher []byte) ([]byte, error) {
	p.unprotects++
	return append([]byte(nil), cipher...), nil
}

type managerHookRuntime struct {
	calls     int
	admission bool
	err       error
}

func (p *managerHookRuntime) GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error) {
	p.calls++
	return entity.HookRuntimeConfig{AdmissionEnabled: p.admission}, p.err
}

type managerHookWake struct {
	events []entity.HookWakeEvent
	err    error
}

func (p *managerHookWake) PublishWake(_ context.Context, event entity.HookWakeEvent) error {
	p.events = append(p.events, event)
	return p.err
}

// The spy captures the Manager boundary but delegates every transaction to the real repository.
type managerHookRepo struct {
	repo.IHookRepo
	t        *testing.T
	mock     sqlmock.Sqlmock
	config   []byte
	input    *entity.HookCreateRunInput
	conflict bool
}

func (r *managerHookRepo) CreateRunWithHooks(ctx context.Context, in entity.HookCreateRunInput) (entity.HookStoreResult, error) {
	r.input = &in
	m := r.mock
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(20, 10, in.ExpectedLatestRunID, 2))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.").WillReturnRows(sqlmock.NewRows([]string{"space_id"}))
	raw := r.config
	if r.conflict {
		raw = []byte(`{"concurrent":"update"}`)
	}
	m.ExpectQuery("SELECT .*lifecycle_hook_conf.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(raw, in.ExpectedLatestRunID))
	if r.conflict {
		m.ExpectRollback()
		// Failed creation is followed by a primary confirmation before owner cleanup.
		m.ExpectBegin()
		m.ExpectQuery("SELECT .*lifecycle_hook_conf.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(r.config, in.ExpectedLatestRunID))
		m.ExpectQuery("SELECT .*FROM .expt_run_log.").WillReturnRows(sqlmock.NewRows([]string{"id"}))
		m.ExpectCommit()
		return r.IHookRepo.CreateRunWithHooks(ctx, in)
	}
	for _, table := range []string{"expt_run_log", "expt_lifecycle_hook_run", "expt_lifecycle_run_item"} {
		m.ExpectQuery("SELECT count.*FROM ." + table + ".").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
	now := time.Unix(1700000000, 0)
	m.ExpectQuery("SELECT CURRENT_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
	m.ExpectExec("INSERT INTO .expt_run_log.").WillReturnResult(sqlmock.NewResult(30, 1))
	m.ExpectExec("INSERT INTO .expt_lifecycle_run.").WillReturnResult(sqlmock.NewResult(0, 1))
	ops := sqlmock.NewRows([]string{"id", "operation_id", "idempotency_key", "phase", "status", "execution_scope"})
	for _, stage := range []struct {
		name string
		seed *entity.HookOperationSeed
	}{{"before", in.Before}, {"after", in.After}} {
		if stage.seed == nil {
			continue
		}
		m.ExpectExec("INSERT INTO .expt_lifecycle_hook_run.").WillReturnResult(sqlmock.NewResult(stage.seed.ID, 1))
		ops.AddRow(stage.seed.ID, stage.seed.OperationID, stage.seed.IdempotencyKey, stage.name, "pending", "local")
	}
	mode := entity.ExptRunMode(in.RunLog.Mode)
	if entity.HookBoundRetryMode(mode) && in.SourceRunID != nil && *in.SourceRunID == in.ExpectedLatestRunID && in.ExpectedLatestRunID > 0 {
		m.ExpectExec("^UPDATE `experiment` SET `latest_run_id`=\\?,`status`=\\?,`updated_at`=\\? WHERE \\(id=\\? AND space_id=\\? AND latest_run_id=\\?\\) AND `experiment`\\.`deleted_at` IS NULL$").
			WithArgs(in.Key.RunID, int32(entity.ExptStatus_Pending), now, int64(20), int64(10), in.ExpectedLatestRunID).
			WillReturnResult(sqlmock.NewResult(0, 1))
	} else {
		m.ExpectExec("UPDATE .experiment.*latest_run_id").WithArgs(in.Key.RunID, now, int64(20), int64(10), in.ExpectedLatestRunID).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	gate := 1
	if in.Before != nil {
		gate = 0
	}
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "gate", "before_enabled", "after_enabled", "snapshot_cipher", "snapshot_key_id", "snapshot_hash", "execution_scope"}).AddRow(10, 20, in.Key.RunID, gate, in.Before != nil, in.After != nil, in.Snapshot.Cipher, in.Snapshot.KeyID, in.Snapshot.Hash, "local"))
	m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "status", "mode", "lifecycle_hook_version", "created_by"}).AddRow(in.Key.RunID, 10, 20, in.Key.RunID, 2, in.RunLog.Mode, 1, in.RunLog.CreatedBy))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WillReturnRows(ops)
	m.ExpectCommit()
	out, err := r.IHookRepo.CreateRunWithHooks(ctx, in)
	if err == nil {
		require.False(r.t, out.Run.PlanReady)
		for _, op := range out.Run.Operations {
			require.Nil(r.t, op.ActivatedAt)
			require.Nil(r.t, op.NextAttemptAt)
		}
	}
	return out, err
}

type hookManagerFixture struct {
	t         *testing.T
	base      *ExptMangerImpl
	manager   IExptManager
	mock      sqlmock.Sqlmock
	deps      ExptManagerHookDependencies
	repo      *managerHookRepo
	protector *managerProtector
	runtime   *managerHookRuntime
	wake      *managerHookWake
	expt      *entity.Experiment
	raw       []byte
	lockOwner string
}

func newHookManagerFixture(t *testing.T, conf *entity.LifecycleHookConf) *hookManagerFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	base := newTestExptManager(ctrl)
	conn, m, err := sqlmock.New()
	require.NoError(t, err)
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.ExpectationsWereMet()); _ = conn.Close() })
	protector := new(managerProtector)
	codec := hookinfra.NewStorageCodec(protector)
	raw, err := codec.EncodeConfig(context.Background(), "key", hookcomponent.ConfigOwner{WorkspaceID: 10, ObjectID: 20, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: "local"}, conf)
	require.NoError(t, err)
	protector.protects = 0
	identity, err := hookinfra.NewIdentityProvider(nil, 0)
	require.NoError(t, err)
	r := &managerHookRepo{IHookRepo: experimentrepo.NewHookRunRepo(p), t: t, mock: m, config: raw}
	// Inspect the actual persistence models as well as SQL. Fixed driver readback
	// rows alone would not catch a missing marker or prematurely activated after.
	require.NoError(t, p.NewSession(context.Background()).Callback().Create().Before("gorm:create").Register("verify_manager_initialization", func(tx *gorm.DB) {
		switch row := tx.Statement.Dest.(type) {
		case *model.ExptRunLog:
			if r.input != nil {
				require.Equal(t, gptr.Of(int32(1)), row.LifecycleHookVersion)
				require.Equal(t, r.input.RunLog.CreatedBy, row.CreatedBy)
			} else {
				require.Nil(t, row.LifecycleHookVersion)
			}
		case *model.ExptLifecycleRun:
			require.NotEmpty(t, row.SnapshotCipher)
			require.Equal(t, "local", row.ExecutionScope)
			require.Equal(t, "key", row.SnapshotKeyID)
			require.Equal(t, r.input.Before != nil, row.BeforeEnabled)
			require.Equal(t, r.input.After != nil, row.AfterEnabled)
			if r.input.Before != nil {
				require.Equal(t, int32(0), row.Gate)
			} else {
				require.Equal(t, int32(1), row.Gate)
			}
			require.Zero(t, row.PlanState)
			require.Zero(t, row.FinalizeState)
			require.False(t, row.ExecutionStarted)
		case *model.ExptLifecycleHookRun:
			require.Equal(t, "pending", row.Status)
			require.Nil(t, row.ActivatedAt)
			require.Nil(t, row.NextAttemptAt)
			require.Zero(t, row.Attempt)
			require.Equal(t, "local", row.ExecutionScope)
		}
	}))
	runtime := &managerHookRuntime{admission: true}
	wake := new(managerHookWake)
	deps := ExptManagerHookDependencies{Initialization: experimentrepo.NewHookRunInitializationRepo(p), Runs: r, Configs: experimentrepo.NewHookConfigRepo(p, codec), Codec: codec, Identity: identity, Runtime: runtime, Wake: wake, ExecutionScope: "local", SnapshotKeyID: "key"}
	manager, err := NewExptManagerWithHooks(base, deps)
	require.NoError(t, err)
	return &hookManagerFixture{t: t, base: base, manager: manager, mock: m, deps: deps, repo: r, protector: protector, runtime: runtime, wake: wake, raw: raw, expt: &entity.Experiment{ID: 20, SpaceID: 10, Name: "真实实验", CreatedBy: "experiment-owner", ExptType: entity.ExptType_Offline}}
}
func (f *hookManagerFixture) expectRead(latest int64, log *sqlmock.Rows) {
	f.mock.ExpectBegin()
	f.mock.ExpectQuery("SELECT .*lifecycle_hook_conf.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(f.raw, latest))
	if log == nil {
		log = sqlmock.NewRows([]string{"id"})
	}
	f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WillReturnRows(log)
	f.mock.ExpectCommit()
}
func (f *hookManagerFixture) expectLock(mode entity.ExptRunMode) {
	f.expectOwnerLock(30, true, "")
	f.base.mtr.(*metricmocks.MockExptMetric).EXPECT().EmitExptExecRun(int64(10), int64(mode))
}

func (f *hookManagerFixture) expectOwnerLock(runID int64, locked bool, existingOwner string) {
	f.base.mutex.(*lockmocks.MockILocker).EXPECT().BackoffLockWithValue(gomock.Any(), f.base.makeExptMutexLockKey(20), gomock.Any(), gomock.Any(), time.Second).
		DoAndReturn(func(_ context.Context, _ string, owner string, _ time.Duration, _ time.Duration) (bool, string, error) {
			parsed, err := managerHookLockRunID(owner)
			require.NoError(f.t, err)
			require.Equal(f.t, runID, parsed)
			f.lockOwner = owner
			return locked, existingOwner, nil
		})
}

func (f *hookManagerFixture) expectOwnerCleanup() {
	f.base.mutex.(*lockmocks.MockILocker).EXPECT().UnlockWithValue(gomock.Any(), f.base.makeExptMutexLockKey(20), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ string, owner string) (bool, error) {
			require.NoError(f.t, ctx.Err())
			require.NotEmpty(f.t, f.lockOwner)
			require.Equal(f.t, f.lockOwner, owner)
			require.NoError(f.t, f.mock.ExpectationsWereMet(), "confirmation must finish before compare-delete")
			return true, nil
		})
}
func (f *hookManagerFixture) expectConfig() {
	f.mock.ExpectQuery("SELECT .*lifecycle_hook_conf.*FROM .experiment.").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(f.raw, f.expt.LatestRunID))
}
func managerEnabledConfig(before, after bool) *entity.LifecycleHookConf {
	stage := func(enabled bool) *entity.HookConfig {
		if !enabled {
			return nil
		}
		return &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://hook.example/run")}, ParametersJSON: gptr.Of(`{"big":9007199254740993,"initiator":{"user_id":"untrusted"}}`)}
	}
	return &entity.LifecycleHookConf{Before: stage(before), After: stage(after)}
}
func TestHookManagerAtomicStages(t *testing.T) {
	for _, tc := range []struct {
		name          string
		before, after bool
	}{{"before", true, false}, {"after", false, true}, {"both", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHookManagerFixture(t, managerEnabledConfig(tc.before, tc.after))
			f.expectLock(entity.EvaluationModeSubmit)
			f.expectRead(0, nil)
			f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
			f.expectConfig()
			id := int64(100)
			f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) { id++; return id, nil }).MinTimes(1)
			err := f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, []int64{71}, &entity.Session{UserID: "run-user"})
			require.NoError(t, err)
			require.NotNil(t, f.repo.input)
			require.Equal(t, tc.before, f.repo.input.Before != nil)
			require.Equal(t, tc.after, f.repo.input.After != nil)
			require.Equal(t, "run-user", f.repo.input.RunLog.CreatedBy)
			require.NotEmpty(t, f.repo.input.ExpectedConfigRevision)
			snapshot, err := f.deps.Codec.DecodeSnapshot(context.Background(), f.repo.input.Key, "local", f.repo.input.Snapshot)
			require.NoError(t, err)
			in := snapshot.Input()
			require.Equal(t, "run-user", in.Context.Initiator.GetUserID())
			require.Equal(t, "真实实验", in.Context.Experiment.GetName())
			require.Equal(t, "offline", in.Context.Experiment.GetType())
			require.Equal(t, "submit", string(in.Context.GetRunMode()))
			require.NotNil(t, in.Context.EvalSets)
			require.Nil(t, in.Context.Target)
			require.False(t, in.CreatedAt.IsZero())
			for _, stage := range []*entity.HookConfig{in.Config.Before, in.Config.After} {
				if stage != nil {
					require.Equal(t, `{"big":9007199254740993,"initiator":{"user_id":"untrusted"}}`, *stage.ParametersJSON)
				}
			}
			for _, seed := range []*entity.HookOperationSeed{f.repo.input.Before, f.repo.input.After} {
				if seed != nil {
					require.Equal(t, "hook_"+strconv.FormatInt(seed.ID, 10), seed.OperationID)
					require.NotEmpty(t, seed.IdempotencyKey)
				}
			}
			require.NotEmpty(t, f.wake.events)
		})
	}
}

func TestHookManagerConflictNeverFallsBack(t *testing.T) {
	f := newHookManagerFixture(t, managerEnabledConfig(true, false))
	f.repo.conflict = true
	f.expectLock(entity.EvaluationModeSubmit)
	f.expectRead(0, nil)
	f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
	f.expectConfig()
	f.expectOwnerCleanup()
	f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(100), nil).AnyTimes()
	err := f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "run-user"})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Empty(t, f.wake.events)
	require.NotContains(t, err.Error(), "concurrent")
}

func TestHookManagerWakeFailureKeepsCommittedRun(t *testing.T) {
	f := newHookManagerFixture(t, managerEnabledConfig(true, false))
	f.wake.err = errors.New("MQ unavailable private body")
	f.expectLock(entity.EvaluationModeSubmit)
	f.expectRead(0, nil)
	f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
	f.expectConfig()
	f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(100), nil).AnyTimes()
	require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "run-user"}))
	require.Len(t, f.wake.events, 1)
	require.Equal(t, int64(30), f.wake.events[0].Run.RunID)
	require.True(t, strings.HasPrefix(f.wake.events[0].OperationID, "hook_"))
}

func (f *hookManagerFixture) expectLegacyCreate(conflict bool) {
	m := f.mock
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(20, 10, f.expt.LatestRunID, 2))
	raw := f.raw
	if conflict {
		raw = []byte(`{"enabled-after-read":true}`)
	}
	m.ExpectQuery("SELECT .*lifecycle_hook_conf.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(raw, f.expt.LatestRunID))
	if conflict {
		m.ExpectRollback()
		f.expectRead(f.expt.LatestRunID, nil)
		return
	}
	m.ExpectQuery("SELECT .*FROM .expt_run_log.").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	now := time.Unix(1700000000, 0)
	m.ExpectQuery("SELECT CURRENT_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
	m.ExpectExec("INSERT INTO .expt_run_log.").WillReturnResult(sqlmock.NewResult(30, 1))
	m.ExpectExec("UPDATE .experiment.*latest_run_id").WithArgs(int64(30), now, int64(20), int64(10), f.expt.LatestRunID).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
}

func TestHookManagerDisabledNeverUsesHookProviders(t *testing.T) {
	for _, conf := range []*entity.LifecycleHookConf{nil, {Before: &entity.HookConfig{Enabled: gptr.Of(false)}, After: &entity.HookConfig{Enabled: gptr.Of(false)}}} {
		f := newHookManagerFixture(t, conf)
		f.runtime.err = errors.New("must not be read")
		f.expectLock(entity.EvaluationModeSubmit)
		f.expectRead(0, nil)
		f.expectLegacyCreate(false)
		require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "legacy-user"}))
		require.Nil(t, f.repo.input)
		require.Zero(t, f.protector.protects)
		require.Zero(t, f.protector.unprotects)
		require.Zero(t, f.runtime.calls)
		require.Empty(t, f.wake.events)
	}
}

func TestHookManagerNoHookQuotaReadFailureDoesNotBlock(t *testing.T) {
	f := newHookManagerFixture(t, nil)
	f.expt.LatestRunID = 29
	f.base.centralGuard = new(fakeGuard)
	var err error
	f.manager, err = NewExptManagerWithHooks(f.base, f.deps)
	require.NoError(t, err)
	f.expectLock(entity.EvaluationModeRetryAll)
	f.expectRead(29, nil)
	f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(nil, errors.New("quota metadata temporarily unavailable"))
	f.expectLegacyCreate(false)
	require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeRetryAll, 10, nil, &entity.Session{UserID: "legacy-user"}), "no-Hook retries retain best-effort quota cleanup")
	require.Zero(t, f.protector.protects)
	require.Zero(t, f.protector.unprotects)
}

func TestHookManagerConcurrentEnableCannotBypass(t *testing.T) {
	f := newHookManagerFixture(t, nil)
	f.expectLock(entity.EvaluationModeSubmit)
	f.expectRead(0, nil)
	f.expectLegacyCreate(true)
	f.expectOwnerCleanup()
	err := f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "run-user"})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Zero(t, f.protector.protects)
	require.Zero(t, f.protector.unprotects)
	require.Empty(t, f.wake.events)
}

type managerUsers struct {
	rpc.IUserProvider
	calls int
	err   error
	users []*entity.UserInfo
}

func (u *managerUsers) MGetUserInfo(ctx context.Context, ids []string) ([]*entity.UserInfo, error) {
	u.calls++
	return u.users, u.err
}

func TestHookManagerOptionalProfileAndSnapshotSources(t *testing.T) {
	for _, profileErr := range []bool{false, true} {
		f := newHookManagerFixture(t, managerEnabledConfig(true, true))
		users := &managerUsers{users: []*entity.UserInfo{{UserID: gptr.Of("run-user"), Email: gptr.Of("same@example.test"), Name: gptr.Of("同一发起人")}, {UserID: gptr.Of("other"), Email: gptr.Of("wrong@example.test")}}}
		if profileErr {
			users.err = errors.New("profile service offline")
		}
		identity, err := hookinfra.NewIdentityProvider(users, 0)
		require.NoError(t, err)
		deps := f.deps
		deps.Identity = identity
		f.manager, err = NewExptManagerWithHooks(f.base, deps)
		require.NoError(t, err)
		f.expt.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 71, EvalSetVersionID: 72, SourceSpaceID: 11}, {EvalSetID: 81}}}
		f.expt.TargetID = 91
		f.expt.TargetVersionID = 92
		f.expt.TargetType = entity.EvalTargetTypeCustomRPCServer
		f.expectLock(entity.EvaluationModeTrialRun)
		f.expectRead(0, nil)
		f.expectConfig()
		f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
		f.base.evaluationSetVersionService.(*svcmocks.MockEvaluationSetVersionService).EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(11), int64(72), gptr.Of(true), nil).Return(&entity.EvaluationSetVersion{ID: 72, SpaceID: 11, EvaluationSetID: 71, Version: "v1.3"}, nil, nil)
		next := int64(100)
		f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) { next++; return next, nil }).Times(2)
		require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeTrialRun, 10, []int64{200}, &entity.Session{UserID: "run-user"}))
		snapshot, err := deps.Codec.DecodeSnapshot(context.Background(), f.repo.input.Key, "local", f.repo.input.Snapshot)
		require.NoError(t, err)
		c := snapshot.Input().Context
		require.Equal(t, "trial_run", string(c.GetRunMode()))
		require.Equal(t, "run-user", c.Initiator.GetUserID())
		require.Equal(t, 1, users.calls)
		if profileErr {
			require.Nil(t, c.Initiator.Email)
			require.Nil(t, c.Initiator.Name)
		} else {
			require.Equal(t, "same@example.test", c.Initiator.GetEmail())
			require.Equal(t, "同一发起人", c.Initiator.GetName())
		}
		require.Len(t, c.EvalSets, 2)
		require.Equal(t, "11", c.EvalSets[0].GetWorkspaceID())
		require.Equal(t, "71", c.EvalSets[0].GetID())
		require.Equal(t, "72", c.EvalSets[0].GetVersionID())
		require.Equal(t, "v1.3", c.EvalSets[0].GetVersion())
		require.Equal(t, "10", c.EvalSets[1].GetWorkspaceID())
		require.Nil(t, c.EvalSets[1].VersionID)
		require.Nil(t, c.EvalSets[1].Version)
		require.Equal(t, "91", c.Target.GetID())
		require.Equal(t, "92", c.Target.GetVersionID())
		require.Equal(t, "CustomRPCServer", c.Target.GetType())
		require.Equal(t, []int64{200}, f.repo.input.RunLog.GetItemIDs())
	}
}

func TestHookManagerEnabledValidationFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, user string
		nameValue  string
		huge       bool
		admission  bool
	}{
		{"invalid user", "0", "valid", false, true},
		{"512 byte name", "user", strings.Repeat("中", 171), false, true},
		{"full snapshot body", "user", "valid", true, true},
		{"admission paused", "user", "valid", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHookManagerFixture(t, managerEnabledConfig(true, false))
			f.runtime.admission = tc.admission
			f.expt.Name = tc.nameValue
			if tc.huge {
				f.expt.EvalConf = &entity.EvaluationConfiguration{}
				for i := 0; i < 2000; i++ {
					f.expt.EvalConf.EvalSetConfigs = append(f.expt.EvalConf.EvalSetConfigs, &entity.EvalSetConfig{EvalSetID: int64(1000 + i)})
				}
			}
			f.expectLock(entity.EvaluationModeSubmit)
			f.expectRead(0, nil)
			f.expectConfig()
			f.expectOwnerCleanup()
			f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
			err := f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: tc.user})
			require.Error(t, err)
			require.Nil(t, f.repo.input)
			require.Empty(t, f.wake.events)
		})
	}
}

func TestHookManagerOnlineEmptyCollections(t *testing.T) {
	f := newHookManagerFixture(t, managerEnabledConfig(false, true))
	f.expt.ExptType = entity.ExptType_Online
	f.expectLock(entity.EvaluationModeAppend)
	f.expectRead(0, nil)
	f.expectConfig()
	f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
	f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(100), nil)
	require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeAppend, 10, nil, &entity.Session{UserID: "run-user"}))
	snapshot, err := f.deps.Codec.DecodeSnapshot(context.Background(), f.repo.input.Key, "local", f.repo.input.Snapshot)
	require.NoError(t, err)
	require.Equal(t, "online", snapshot.Input().Context.Experiment.GetType())
	require.NotNil(t, snapshot.Input().Context.EvalSets)
	require.Empty(t, snapshot.Input().Context.EvalSets)
}

func TestHookManagerRetryNewRunPinsOldQuotaKey(t *testing.T) {
	for _, retryItems := range []bool{false, true} {
		f := newHookManagerFixture(t, managerEnabledConfig(true, false))
		f.expt.EvalConf = &entity.EvaluationConfiguration{}
		f.expt.EvalSetID = 71
		f.expt.EvalSetVersionID = 71
		guard := new(fakeGuard)
		f.base.centralGuard = guard
		var err error
		f.manager, err = NewExptManagerWithHooks(f.base, f.deps)
		require.NoError(t, err)
		f.expt.LatestRunID = 29
		f.expt.ExptDispatchMode = entity.ExptDispatchModeEnforce
		f.expt.SchedulerScope = "quota-scope"
		f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
		f.base.itemResultRepo.(*repomocks.MockIExptItemResultRepo).EXPECT().ScanItemRunLogs(gomock.Any(), int64(20), int64(29), gomock.Any(), gomock.Any(), gomock.Any(), int64(10)).Return([]*entity.ExptItemResultRunLog{{ItemID: 91}}, int64(1), nil)
		f.expectRead(29, nil)
		f.expectConfig()
		mode := entity.EvaluationModeRetryAll
		if retryItems {
			mode = entity.EvaluationModeRetryItems
			f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(30), nil)
			f.expectOwnerLock(30, true, "")
			f.base.mtr.(*metricmocks.MockExptMetric).EXPECT().EmitExptExecRun(int64(10), int64(mode))
		} else {
			f.expectLock(mode)
		}
		f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(100), nil)
		if retryItems {
			run, retried, err := f.manager.LogRetryItemsRun(context.Background(), 20, mode, 10, []int64{91}, &entity.Session{UserID: "run-user"})
			require.NoError(t, err)
			require.Equal(t, int64(30), run)
			require.False(t, retried)
		} else {
			require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, mode, 10, []int64{91}, &entity.Session{UserID: "run-user"}))
		}
		require.Equal(t, gptr.Of(int64(29)), f.repo.input.SourceRunID)
		require.Equal(t, int64(29), f.repo.input.ExpectedLatestRunID)
		require.Equal(t, []releaseCall{{Scope: "quota-scope", RunID: 29, ItemID: 91, Reason: "superseded by retry run=30"}}, guard.releases())
	}
}

func managerStoredLog(marker any) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "created_by", "mode", "status", "item_ids", "lifecycle_hook_version", "created_at"}).AddRow(30, 10, 20, 30, "original-user", 5, 2, []byte(`[{"ItemIDs":[71],"CreateAt":123}]`), marker, time.Unix(123, 0))
}
func (f *hookManagerFixture) expectStoredRun(status, gate int, scope string, read bool) {
	m := f.mock
	if read {
		m.ExpectBegin()
	}
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(20, 10, 30, 2))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "gate", "version", "before_enabled", "after_enabled", "snapshot_cipher", "snapshot_key_id", "snapshot_hash", "execution_scope"}).AddRow(10, 20, 30, gate, 7, true, true, []byte("immutable snapshot, never decode"), "frozen-key", strings.Repeat("a", 64), scope))
	m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "status", "mode", "lifecycle_hook_version", "created_by", "item_ids"}).AddRow(30, 10, 20, 30, status, 5, 1, "original-user", []byte(`[{"ItemIDs":[71],"CreateAt":123}]`)))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "operation_id", "idempotency_key", "phase", "status", "execution_scope"}).AddRow(100, "hook_original_before", "original-before-key", "before", "pending", scope).AddRow(101, "hook_original_after", "original-after-key", "after", "pending", scope))
	if read {
		m.ExpectCommit()
	}
}

func TestHookManagerSameRunReplayKeepsSnapshotAndOperations(t *testing.T) {
	f := newHookManagerFixture(t, managerEnabledConfig(true, true))
	f.expectRead(30, managerStoredLog(1))
	f.expectStoredRun(2, 0, "local", true)
	require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "original-user"}))
	require.Nil(t, f.repo.input)
	require.Zero(t, f.protector.protects)
	require.Zero(t, f.protector.unprotects)
	require.Zero(t, f.runtime.calls)
	require.Len(t, f.wake.events, 2)
	require.Equal(t, "hook_original_before", f.wake.events[0].OperationID)
	require.Equal(t, "hook_original_after", f.wake.events[1].OperationID)
}

func TestHookManagerSameRunRejectsChangedAuthor(t *testing.T) {
	f := newHookManagerFixture(t, managerEnabledConfig(true, true))
	f.expectRead(30, managerStoredLog(1))
	require.ErrorIs(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "new-user"}), entity.ErrHookStoreConflict)
	require.Nil(t, f.repo.input)
	require.Empty(t, f.wake.events)
}

func TestHookManagerRetryItemsReusesFrozenRun(t *testing.T) {
	for _, tc := range []struct {
		name         string
		status, gate int
		scope        string
		items        []int64
		wantErr      bool
	}{
		{"append", 2, 0, "local", []int64{72}, false},
		{"duplicate retry", 2, 0, "local", []int64{71}, true},
		{"closed", 13, 2, "local", []int64{72}, true},
		{"wrong scope", 2, 0, "other", []int64{72}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHookManagerFixture(t, managerEnabledConfig(true, true))
			f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(31), nil)
			existingOwner, err := newManagerHookLockOwner(30)
			require.NoError(t, err)
			f.expectOwnerLock(31, false, existingOwner)
			f.base.mutex.(*lockmocks.MockILocker).EXPECT().Exists(gomock.Any(), gomock.Any()).Return(false, nil)
			f.expectRead(30, nil)
			f.expectRead(30, managerStoredLog(1))
			f.expectStoredRun(tc.status, tc.gate, tc.scope, true)
			f.mock.ExpectBegin()
			f.expectStoredRun(tc.status, tc.gate, tc.scope, false)
			if tc.wantErr {
				f.mock.ExpectRollback()
			} else {
				now := time.Unix(1700000000, 0)
				f.mock.ExpectQuery("SELECT CURRENT_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
				f.mock.ExpectExec("UPDATE .expt_run_log.*SET .item_ids.=.*updated_at.*lifecycle_hook_version=1 AND status=").WillReturnResult(sqlmock.NewResult(0, 1))
				f.mock.ExpectExec("UPDATE .expt_lifecycle_run.*version").WillReturnResult(sqlmock.NewResult(0, 1))
				f.mock.ExpectCommit()
			}
			run, retried, err := f.manager.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, tc.items, &entity.Session{UserID: "different-current-user"})
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(30), run)
				require.True(t, retried)
			}
			require.Nil(t, f.repo.input)
			require.Zero(t, f.protector.protects)
			require.Zero(t, f.protector.unprotects)
			require.Zero(t, f.runtime.calls)
			require.Empty(t, f.wake.events)
		})
	}
}

func TestHookManagerLegacyRetryReuseStillSaves(t *testing.T) {
	f := newHookManagerFixture(t, nil)
	f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(31), nil)
	f.expectOwnerLock(31, false, "30")
	f.base.mutex.(*lockmocks.MockILocker).EXPECT().Exists(gomock.Any(), gomock.Any()).Return(false, nil)
	f.expectRead(30, nil)
	f.expectRead(30, managerStoredLog(nil))
	old := &entity.ExptRunLog{ID: 30, ExptID: 20, ExptRunID: 30, SpaceID: 10, CreatedBy: "original-user", Mode: 5}
	f.base.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), int64(20), int64(30)).Return(old, nil)
	f.base.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got *entity.ExptRunLog) error {
		require.Equal(t, "original-user", got.CreatedBy)
		require.Equal(t, []int64{72}, got.GetItemIDs())
		return nil
	})
	run, retried, err := f.manager.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{72}, &entity.Session{UserID: "different-current-user"})
	require.NoError(t, err)
	require.Equal(t, int64(30), run)
	require.True(t, retried)
	require.Nil(t, f.repo.input)
	require.Zero(t, f.protector.unprotects)
}

func TestHookManagerLoadsSnapshotFromPrimary(t *testing.T) {
	f := newHookManagerFixture(t, managerEnabledConfig(true, false))
	f.expectLock(entity.EvaluationModeSubmit)
	f.expectRead(0, nil)
	f.expectConfig()
	f.expt.EvalSetID = 71
	f.expt.EvalSetVersionID = 72
	f.expt.EvalSetSpaceID = 11
	f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).DoAndReturn(func(ctx context.Context, _ int64, _ int64) (*entity.Experiment, error) {
		require.True(t, contexts.CtxWriteDB(ctx), "replica state must not be frozen as the new Run context")
		return f.expt, nil
	})
	f.base.evaluationSetVersionService.(*svcmocks.MockEvaluationSetVersionService).EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(11), int64(72), gptr.Of(true), nil).DoAndReturn(func(ctx context.Context, _ int64, _ int64, _ *bool, _ *entity.SharedResourceOption) (*entity.EvaluationSetVersion, *entity.EvaluationSet, error) {
		require.True(t, contexts.CtxWriteDB(ctx))
		return &entity.EvaluationSetVersion{ID: 72, SpaceID: 11, EvaluationSetID: 71, Version: "v1"}, nil, nil
	})
	f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(100), nil)
	require.NoError(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "run-user"}))
}

func TestHookManagerRejectsInvalidStoredReferences(t *testing.T) {
	for _, change := range []func(*entity.Experiment){
		func(e *entity.Experiment) { e.TargetID = 91; e.TargetType = 999 },
		func(e *entity.Experiment) { e.TargetID = -1 },
		func(e *entity.Experiment) { e.TargetVersionID = 92 },
		func(e *entity.Experiment) { e.EvalSetID = -1 },
		func(e *entity.Experiment) { e.EvalSetVersionID = 72 },
	} {
		f := newHookManagerFixture(t, managerEnabledConfig(true, false))
		change(f.expt)
		f.expectLock(entity.EvaluationModeSubmit)
		f.expectRead(0, nil)
		f.expectConfig()
		f.expectOwnerCleanup()
		f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
		// These expectations permit the pre-fix implementation to complete, exposing
		// the wrong success at the public Manager boundary rather than a mock failure.
		f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(100), nil).AnyTimes()
		err := f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "run-user"})
		require.Error(t, err, "invalid stored references must not disappear from a successful frozen context")
	}
}

func TestHookManagerConstructorRejectsMissingDependencies(t *testing.T) {
	f := newHookManagerFixture(t, nil)
	for _, change := range []func(*ExptManagerHookDependencies){
		func(d *ExptManagerHookDependencies) { d.Initialization = nil }, func(d *ExptManagerHookDependencies) { d.Runs = nil }, func(d *ExptManagerHookDependencies) { d.Configs = nil }, func(d *ExptManagerHookDependencies) { d.Codec = nil }, func(d *ExptManagerHookDependencies) { d.Identity = nil }, func(d *ExptManagerHookDependencies) { d.Runtime = nil }, func(d *ExptManagerHookDependencies) { d.Wake = nil }, func(d *ExptManagerHookDependencies) { d.SnapshotKeyID = "" }, func(d *ExptManagerHookDependencies) { d.ExecutionScope = "" }, func(d *ExptManagerHookDependencies) { var wake *managerHookWake; d.Wake = wake },
	} {
		deps := f.deps
		change(&deps)
		manager, err := NewExptManagerWithHooks(f.base, deps)
		require.Error(t, err)
		require.Nil(t, manager)
	}
	require.Nil(t, f.base.hooks, "constructing the Hook-aware Manager must not silently mutate the legacy instance")
}
