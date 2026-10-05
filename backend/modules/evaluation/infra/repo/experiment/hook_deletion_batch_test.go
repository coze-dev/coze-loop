// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type batchDeletionDB struct {
	db.Provider
	depth, transactions int
}

func (p *batchDeletionDB) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	p.transactions++
	return p.Provider.Transaction(ctx, func(tx *gorm.DB) error { p.depth++; defer func() { p.depth-- }(); return fn(tx) }, opts...)
}

type batchDeletionRuns struct {
	repo.IHookRepo
	runs map[entity.HookRunKey]*entity.HookStoredRun
}

func (r batchDeletionRuns) GetRun(_ context.Context, key entity.HookRunKey) (*entity.HookStoredRun, error) {
	v := r.runs[key]
	if v == nil {
		return nil, entity.ErrHookStoreMissing
	}
	out := *v
	return &out, nil
}

type batchDeletionProtector struct{}

func (batchDeletionProtector) Protect(_ context.Context, _ string, b []byte) ([]byte, error) {
	return append([]byte(nil), b...), nil
}
func (batchDeletionProtector) Unprotect(_ context.Context, _ string, b []byte) ([]byte, error) {
	return append([]byte(nil), b...), nil
}

type batchDeletionCodec struct {
	hookcomponent.StorageCodec
	t     *testing.T
	p     *batchDeletionDB
	calls int
	err   error
}

func (c *batchDeletionCodec) DecodeSnapshot(ctx context.Context, k entity.HookRunKey, scope string, p entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error) {
	require.Zero(c.t, c.p.depth, "snapshot authentication must be outside SQL locks")
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return c.StorageCodec.DecodeSnapshot(ctx, k, scope, p)
}

type batchDeletionFixture struct {
	t     *testing.T
	p     *batchDeletionDB
	mock  sqlmock.Sqlmock
	runs  batchDeletionRuns
	codec *batchDeletionCodec
}

func newBatchDeletionFixture(t *testing.T) *batchDeletionFixture {
	t.Helper()
	conn, m, err := sqlmock.New()
	require.NoError(t, err)
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	f := &batchDeletionFixture{t: t, p: &batchDeletionDB{Provider: p}, mock: m, runs: batchDeletionRuns{runs: map[entity.HookRunKey]*entity.HookStoredRun{}}}
	f.codec = &batchDeletionCodec{StorageCodec: hookinfra.NewStorageCodec(batchDeletionProtector{}), t: t, p: f.p}
	return f
}
func (f *batchDeletionFixture) run(expt, id int64, mode entity.ExptRunMode, bound, single bool) *entity.HookStoredRun {
	f.t.Helper()
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: expt, RunID: id}
	name := map[entity.ExptRunMode]string{1: "submit", 2: "fail_retry", 4: "retry_all", 6: "trial_run"}[mode]
	in := entity.HookRunSnapshotInput{Key: key, ExecutionScope: "test-scope", CreatedAt: time.Unix(1700000000, 0),
		Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.test/hook")}}, After: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.test/after")}}},
		Context: &spi.HookRunContext{WorkspaceID: gptr.Of("1"), ExperimentID: gptr.Of(strconv.FormatInt(expt, 10)), RunID: gptr.Of(strconv.FormatInt(id, 10)), RunMode: &name,
			Initiator: &spi.HookInitiator{UserID: gptr.Of("actor"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Type: gptr.Of("offline"), Name: gptr.Of("original")},
			EvalSets: []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("1"), ID: gptr.Of("10"), VersionID: gptr.Of("11")}}}}
	if bound {
		in.Execution = &entity.HookExecutionSnapshot{Version: 1, Key: key, ExecutionScope: "test-scope", Mode: mode, SingleSet: single,
			EvaluatorFallback: &entity.HookExecutionEvaluatorFallback{}, Sets: []entity.HookExecutionSet{{EvalSetID: 10, EvalSetVersionID: 11, ItemConfig: &entity.ExptItemConfig{}}}}
	}
	snapshot, err := entity.NewHookRunSnapshot(in)
	require.NoError(f.t, err)
	p, err := f.codec.EncodeSnapshot(context.Background(), "key", snapshot)
	require.NoError(f.t, err)
	run := &entity.HookStoredRun{State: entity.HookRunState{Key: key, Status: entity.ExptStatus_Processing, Gate: entity.HookGateClosed, Finalize: entity.HookFinalizePending, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated},
		Before: entity.HookOperation{ID: "before-" + strconv.FormatInt(id, 10), Status: entity.HookOperationFailed}, After: entity.HookOperation{ID: "after-" + strconv.FormatInt(id, 10), Status: entity.HookOperationPending}},
		Snapshot: p, Mode: mode, CreatedBy: "actor", Version: 1, TerminalAt: gptr.Of(time.Unix(1700000000, 0)), PlanReady: true, PlanHash: entity.NewHookPlanDigest().Hash}
	if entity.HookBoundRetryMode(mode) {
		run.SourceRunID = gptr.Of(int64(5))
	}
	f.runs.runs[key] = run
	return run
}
func batchParentRows(expt int64, source entity.ExptEvalSetSourceType, status ...entity.ExptStatus) *sqlmock.Rows {
	s := entity.ExptStatus_Processing
	if len(status) > 0 {
		s = status[0]
	}
	return sqlmock.NewRows([]string{"id", "space_id", "expt_type", "eval_set_source_type", "status", "latest_run_id"}).AddRow(expt, 1, int32(entity.ExptType_Offline), int32(source), int32(s), int64(0))
}
func batchMarkerRows(ids ...int64) *sqlmock.Rows {
	r := sqlmock.NewRows([]string{"expt_run_id", "lifecycle_hook_version"})
	for _, id := range ids {
		r.AddRow(id, 1)
	}
	return r
}
func (f *batchDeletionFixture) preload(expt int64, source entity.ExptEvalSetSourceType, ids ...int64) {
	f.mock.ExpectQuery("SELECT .*FROM .experiment.").WithArgs(expt, int64(1), 1).WillReturnRows(batchParentRows(expt, source))
	f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WithArgs(int64(1), expt, int64(0), 100).WillReturnRows(batchMarkerRows(ids...))
}

func TestHookBatchDeletionPurePreparesEveryRunOutsideLocks(t *testing.T) {
	f := newBatchDeletionFixture(t)
	f.run(20, 31, entity.EvaluationModeFailRetry, true, false)
	f.run(20, 32, entity.EvaluationModeRetryAll, true, false)
	f.preload(20, entity.ExptEvalSetSourceType_MultiSetConfig, 31, 32)
	r, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{20}, 1, "test-scope")
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, 2, f.codec.calls, "every original pending Run must have an authenticated binding")
	require.Zero(t, f.p.transactions, "preparation must not perform deletion")
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestHookBatchDeletionPureRejectsInvalidSnapshotOrLedger(t *testing.T) {
	for _, kind := range []string{"missing", "hash", "scope", "key", "flags", "unknown_marker", "unbound_retry", "unbound_multiset", "unknown_source", "incomplete_bound"} {
		t.Run(kind, func(t *testing.T) {
			f := newBatchDeletionFixture(t)
			r := f.run(20, 31, entity.EvaluationModeFailRetry, true, false)
			source := entity.ExptEvalSetSourceType_MultiSetConfig
			switch kind {
			case "missing":
				delete(f.runs.runs, r.State.Key)
			case "hash":
				r.Snapshot.Hash = string(make([]byte, 64))
			case "scope":
				r.Snapshot.ExecutionScope = "foreign"
			case "key":
				r.State.Key.RunID = 99
			case "flags":
				r.State.After.Status = entity.HookOperationDisabled
			case "unbound_retry":
				r = f.run(20, 31, entity.EvaluationModeFailRetry, false, true)
				source = entity.ExptEvalSetSourceType_SingleSet
			case "unbound_multiset":
				r = f.run(20, 31, entity.EvaluationModeSubmit, false, false)
			case "unknown_source":
				source = entity.ExptEvalSetSourceType(99)
			case "incomplete_bound":
				snapshot, err := f.codec.StorageCodec.DecodeSnapshot(context.Background(), r.State.Key, "test-scope", r.Snapshot)
				require.NoError(t, err)
				in := snapshot.Input()
				in.Execution.EvaluatorFallback = nil
				snapshot, err = entity.NewHookRunSnapshot(in)
				require.NoError(t, err)
				r.Snapshot, err = f.codec.EncodeSnapshot(context.Background(), "key", snapshot)
				require.NoError(t, err)
			}
			if kind == "unknown_marker" {
				f.mock.ExpectQuery("SELECT .*FROM .experiment.").WithArgs(int64(20), int64(1), 1).WillReturnRows(batchParentRows(20, source))
				f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WithArgs(int64(1), int64(20), int64(0), 100).WillReturnRows(sqlmock.NewRows([]string{"expt_run_id", "lifecycle_hook_version"}).AddRow(31, 9))
			} else {
				f.preload(20, source, 31)
			}
			_, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{20}, 1, "test-scope")
			require.Error(t, err)
			require.Zero(t, f.p.transactions)
		})
	}
}

func TestHookBatchDeletionPurePreloadFollowsRawMarkerTail(t *testing.T) {
	f := newBatchDeletionFixture(t)
	f.run(20, 101, entity.EvaluationModeTrialRun, false, true)
	f.mock.ExpectQuery("SELECT .*FROM .experiment.").WithArgs(int64(20), int64(1), 1).WillReturnRows(batchParentRows(20, entity.ExptEvalSetSourceType_SingleSet))
	page := sqlmock.NewRows([]string{"expt_run_id", "lifecycle_hook_version"})
	for i := 1; i <= 100; i++ {
		page.AddRow(i, 0)
	}
	f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WithArgs(int64(1), int64(20), int64(0), 100).WillReturnRows(page)
	f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WithArgs(int64(1), int64(20), int64(100), 100).WillReturnRows(batchMarkerRows(101))
	_, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{20}, 1, "test-scope")
	require.NoError(t, err)
	require.Equal(t, 1, f.codec.calls)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestHookBatchDeletionPurePreparationRejectsCanceledAndMissingDependencies(t *testing.T) {
	f := newBatchDeletionFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := PrepareHookDeletion(ctx, f.p, f.runs, f.codec, []int64{20}, 1, "test-scope")
	require.ErrorIs(t, err, context.Canceled)
	_, err = PrepareHookDeletion(context.Background(), nil, f.runs, f.codec, []int64{20}, 1, "test-scope")
	require.Error(t, err)
	_, err = PrepareHookDeletion(context.Background(), f.p, f.runs, (*batchDeletionCodec)(nil), []int64{20}, 1, "test-scope")
	require.Error(t, err)
	require.Zero(t, f.p.transactions)
}

func batchLifeRows(r *entity.HookStoredRun) *sqlmock.Rows {
	final := 0
	switch r.State.Finalize {
	case entity.HookFinalizePending:
		final = 1
	case entity.HookFinalizeCommitted:
		final = 2
	}
	gate := 0
	switch r.State.Gate {
	case entity.HookGateReady:
		gate = 1
	case entity.HookGateClosed:
		gate = 2
	}
	return sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "gate", "before_enabled", "after_enabled", "snapshot_cipher", "snapshot_key_id", "snapshot_hash", "execution_scope", "version", "plan_state", "plan_count", "plan_hash", "execution_initialized", "execution_started", "finalize_state", "terminal_status", "terminal_at", "source_run_id"}).AddRow(
		r.State.Key.WorkspaceID, r.State.Key.ExperimentID, r.State.Key.RunID, gate, true, true, r.Snapshot.Cipher, r.Snapshot.KeyID, r.Snapshot.Hash, r.Snapshot.ExecutionScope, r.Version, 1, 0, r.PlanHash, true, r.ExecutionStarted, final, int32(r.State.Intent.Status), r.TerminalAt, r.SourceRunID)
}
func batchLogRows(r *entity.HookStoredRun) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "mode", "status", "created_by", "lifecycle_hook_version"}).AddRow(r.State.Key.RunID, r.State.Key.WorkspaceID, r.State.Key.ExperimentID, r.State.Key.RunID, int32(r.Mode), int64(r.State.Status), r.CreatedBy, 1)
}
func batchOpRows(r *entity.HookStoredRun) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "operation_id", "idempotency_key", "phase", "status", "execution_scope", "version", "activated_at"})
	for i, op := range []entity.HookOperation{r.State.After, r.State.Before} {
		phase := "after"
		if i == 1 {
			phase = "before"
		}
		var active *time.Time
		if op.Activated {
			active = gptr.Of(time.Unix(1700000000, 0))
		}
		rows.AddRow(r.State.Key.RunID*10+int64(i), op.ID, "idem-"+op.ID, phase, string(op.Status), r.Snapshot.ExecutionScope, 1, active)
	}
	return rows
}
func (f *batchDeletionFixture) load(r *entity.HookStoredRun, bound bool) {
	k := r.State.Key
	f.mock.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WithArgs(k.WorkspaceID, k.ExperimentID, k.RunID, 1).WillReturnRows(batchLifeRows(r))
	f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WithArgs(k.WorkspaceID, k.ExperimentID, k.RunID, 1).WillReturnRows(batchLogRows(r))
	f.mock.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WithArgs(k.WorkspaceID, k.ExperimentID, k.RunID).WillReturnRows(batchOpRows(r))
	if bound {
		f.mock.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.").WithArgs(k.WorkspaceID, k.ExperimentID, k.RunID, 1).WillReturnRows(batchLifeRows(r))
		f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WithArgs(k.WorkspaceID, k.ExperimentID, k.RunID, 1).WillReturnRows(batchLogRows(r))
	}
}
func (f *batchDeletionFixture) lockParent(expt int64, source entity.ExptEvalSetSourceType) {
	f.mock.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WithArgs(expt, int64(1), 1).WillReturnRows(batchParentRows(expt, source))
}
func (f *batchDeletionFixture) markers(expt int64, ids ...int64) {
	f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WithArgs(int64(1), expt, int64(0), 100).WillReturnRows(batchMarkerRows(ids...))
}
func (f *batchDeletionFixture) softDelete(expt int64) {
	f.mock.ExpectExec("UPDATE .experiment. SET .deleted_at.").WithArgs(sqlmock.AnyArg(), expt, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestHookBatchDeletionPureMultipleBindingsOneTransaction(t *testing.T) {
	f := newBatchDeletionFixture(t)
	a := f.run(20, 31, 2, true, false)
	b := f.run(20, 32, 4, true, false)
	c := f.run(21, 33, 6, false, true)
	f.preload(20, entity.ExptEvalSetSourceType_MultiSetConfig, 31, 32)
	f.preload(21, entity.ExptEvalSetSourceType_SingleSet, 33)
	r, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{21, 20, 20}, 1, "test-scope")
	require.NoError(t, err)
	f.mock.ExpectBegin()
	f.lockParent(20, entity.ExptEvalSetSourceType_MultiSetConfig)
	f.lockParent(21, entity.ExptEvalSetSourceType_SingleSet)
	f.markers(20, 31, 32)
	f.load(a, true)
	f.load(b, true)
	f.softDelete(20)
	f.markers(21, 33)
	f.load(c, false)
	f.softDelete(21)
	f.mock.ExpectCommit()
	deleted, err := r.DeleteExperiments(context.Background(), []int64{20, 21}, 1, "test-scope")
	require.NoError(t, err)
	require.Len(t, deleted, 2)
	require.Equal(t, 1, f.p.transactions)
	require.Equal(t, 3, f.codec.calls)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestHookBatchDeletionPureHashMutationRollsBackWholeBatch(t *testing.T) {
	f := newBatchDeletionFixture(t)
	r0 := f.run(21, 31, 2, true, true)
	f.preload(20, entity.ExptEvalSetSourceType_SingleSet)
	f.preload(21, entity.ExptEvalSetSourceType_SingleSet, 31)
	r, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{20, 21}, 1, "test-scope")
	require.NoError(t, err)
	r0.Snapshot.Hash = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	f.mock.ExpectBegin()
	f.lockParent(20, entity.ExptEvalSetSourceType_SingleSet)
	f.lockParent(21, entity.ExptEvalSetSourceType_SingleSet)
	f.markers(20)
	f.softDelete(20)
	f.markers(21, 31)
	f.load(r0, false)
	f.mock.ExpectRollback()
	deleted, err := r.DeleteExperiments(context.Background(), []int64{20, 21}, 1, "test-scope")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Empty(t, deleted)
	require.Equal(t, 1, f.p.transactions)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestHookBatchDeletionPureNewRunAfterPreloadConflicts(t *testing.T) {
	f := newBatchDeletionFixture(t)
	a := f.run(20, 31, 2, true, false)
	f.preload(20, entity.ExptEvalSetSourceType_MultiSetConfig, 31)
	r, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{20}, 1, "test-scope")
	require.NoError(t, err)
	f.mock.ExpectBegin()
	f.lockParent(20, entity.ExptEvalSetSourceType_MultiSetConfig)
	f.markers(20, 31, 32)
	f.load(a, true)
	f.mock.ExpectRollback()
	deleted, err := r.DeleteExperiments(context.Background(), []int64{20}, 1, "test-scope")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Empty(t, deleted)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestHookBatchDeletionPureWriteFailureRollsBackWholeBatch(t *testing.T) {
	f := newBatchDeletionFixture(t)
	a := f.run(20, 31, 2, true, false)
	b := f.run(21, 32, 4, true, false)
	f.preload(20, entity.ExptEvalSetSourceType_MultiSetConfig, 31)
	f.preload(21, entity.ExptEvalSetSourceType_MultiSetConfig, 32)
	r, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{20, 21}, 1, "test-scope")
	require.NoError(t, err)
	f.mock.ExpectBegin()
	f.lockParent(20, entity.ExptEvalSetSourceType_MultiSetConfig)
	f.lockParent(21, entity.ExptEvalSetSourceType_MultiSetConfig)
	f.markers(20, 31)
	f.load(a, true)
	f.softDelete(20)
	f.markers(21, 32)
	f.load(b, true)
	failure := errors.New("second parent softdelete failed")
	f.mock.ExpectExec("UPDATE .experiment. SET .deleted_at.").WillReturnError(failure)
	f.mock.ExpectRollback()
	deleted, err := r.DeleteExperiments(context.Background(), []int64{20, 21}, 1, "test-scope")
	require.ErrorIs(t, err, failure)
	require.Empty(t, deleted)
	require.Equal(t, 1, f.p.transactions)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestHookBatchDeletionPurePreservesCommittedSuccessAndRepeatedDelete(t *testing.T) {
	f := newBatchDeletionFixture(t)
	a := f.run(20, 31, 1, false, true)
	a.State.Status = entity.ExptStatus_Success
	a.State.Finalize = entity.HookFinalizeCommitted
	a.State.Intent.Status = entity.ExptStatus_Success
	a.State.Before.Status = entity.HookOperationSucceeded
	a.State.After.Activated = true
	f.codec.err = errors.New("completed Run no longer needs an execution binding")
	f.preload(20, entity.ExptEvalSetSourceType_SingleSet, 31)
	r, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{20}, 1, "test-scope")
	require.NoError(t, err)
	f.mock.ExpectBegin()
	f.mock.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WithArgs(int64(20), int64(1), 1).WillReturnRows(batchParentRows(20, entity.ExptEvalSetSourceType_SingleSet, entity.ExptStatus_Success))
	f.markers(20, 31)
	f.load(a, false)
	f.softDelete(20)
	f.mock.ExpectCommit()
	deleted, err := r.DeleteExperiments(context.Background(), []int64{20}, 1, "test-scope")
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	require.Equal(t, entity.ExptStatus_Success, deleted[0].Status)
	f.mock.ExpectBegin()
	f.mock.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WithArgs(int64(20), int64(1), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "deleted_at"}).AddRow(20, 1, time.Unix(1700000000, 0)))
	f.mock.ExpectCommit()
	again, err := r.DeleteExperiments(context.Background(), []int64{20}, 1, "test-scope")
	require.NoError(t, err)
	require.Empty(t, again)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestHookBatchDeletionPureLegacyBatchAndMissingIDs(t *testing.T) {
	f := newBatchDeletionFixture(t)
	f.mock.ExpectQuery("SELECT .*FROM .experiment.").WithArgs(int64(20), int64(1), 1).WillReturnRows(batchParentRows(20, entity.ExptEvalSetSourceType_SingleSet))
	f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WithArgs(int64(1), int64(20), int64(0), 100).WillReturnRows(sqlmock.NewRows([]string{"expt_run_id", "lifecycle_hook_version"}).AddRow(5, 0))
	f.mock.ExpectQuery("SELECT .*FROM .experiment.").WithArgs(int64(21), int64(1), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	r, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{21, 20, 20}, 1, "test-scope")
	require.NoError(t, err)
	require.Zero(t, f.codec.calls)
	f.mock.ExpectBegin()
	f.lockParent(20, entity.ExptEvalSetSourceType_SingleSet)
	f.mock.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WithArgs(int64(21), int64(1), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	f.mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WithArgs(int64(1), int64(20), int64(0), 100).WillReturnRows(sqlmock.NewRows([]string{"expt_run_id", "lifecycle_hook_version"}).AddRow(5, 0))
	f.softDelete(20)
	f.mock.ExpectCommit()
	deleted, err := r.DeleteExperiments(context.Background(), []int64{20, 21}, 1, "test-scope")
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

func TestHookBatchDeletionPurePreparedRequestRejectsDifferentScopeAndCancellation(t *testing.T) {
	f := newBatchDeletionFixture(t)
	f.preload(20, entity.ExptEvalSetSourceType_SingleSet)
	r, err := PrepareHookDeletion(context.Background(), f.p, f.runs, f.codec, []int64{20}, 1, "test-scope")
	require.NoError(t, err)
	_, err = r.DeleteExperiments(context.Background(), []int64{21}, 1, "test-scope")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	_, err = r.DeleteExperiments(context.Background(), []int64{20}, 2, "test-scope")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	_, err = r.DeleteExperiments(context.Background(), []int64{20}, 1, "foreign")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = r.DeleteExperiments(ctx, []int64{20}, 1, "test-scope")
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, f.p.transactions, "cancellation before deletion must not begin a transaction")
}
