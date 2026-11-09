// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func prioritySQL(t *testing.T, f *independentAuditFixture, tables ...string) map[string]any {
	t.Helper()
	rows := map[string]any{}
	for _, table := range tables {
		var values []map[string]any
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Order("id").Find(&values).Error)
		rows[table] = values
	}
	return rows
}

type priorityPreReadGate struct {
	repo.IExptRunLogRepo
	entered, proceed chan struct{}
	once             sync.Once
	observed         atomic.Int64
}

func (g *priorityPreReadGate) Get(ctx context.Context, expt, run int64) (*entity.ExptRunLog, error) {
	row, err := g.IExptRunLogRepo.Get(ctx, expt, run)
	if err == nil && row != nil {
		g.once.Do(func() {
			g.observed.Store(row.Status)
			close(g.entered)
			select {
			case <-g.proceed:
			case <-ctx.Done():
				err = ctx.Err()
			}
		})
	}
	return row, err
}

// The frozen e6 overlay also calls this same barrier with its exact original
// CompleteRun/CompleteExpt bodies and full-row Save, rather than a disposition mock.
func priorityHalfWindow(t *testing.T, mode entity.ExptRunMode, legacy func(*independentAuditFixture) (func() error, func() error), continueOld bool) {
	t.Helper()
	f := newIndependentAuditFixture(t, mode)
	f.seedSources(1, true)
	completeRun, completeExpt := legacy(f)
	gate := &priorityPreReadGate{IExptRunLogRepo: f.manager.runLogRepo, entered: make(chan struct{}), proceed: make(chan struct{})}
	candidate := *f.manager
	candidate.runLogRepo = gate
	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()
	var once sync.Once
	resume := func() { once.Do(func() { close(gate.proceed) }) }
	defer resume()
	done := make(chan error, 1)
	go func() { done <- candidate.CompleteExpt(ctx, f.expt, &f.r1, f.space, &entity.Session{}) }()
	independentAuditWait(t, gate.entered)
	require.Equal(t, int64(entity.ExptStatus_Processing), gate.observed.Load(), "manager must observe active before the old write")
	require.NoError(t, completeRun())
	before := prioritySQL(t, f, "experiment", "expt_run_log", "expt_stats", "expt_item_result", "expt_turn_result")
	admission := f.admission()
	require.True(t, f.slot())
	resume()
	err := independentAuditResult(t, done)
	assert.True(t, errno.NeedMQRetry(err), "BUSINESS_RED HALF must stay Pending after old write between manager pre-read and DAO locks: %v", err)
	assert.Equal(t, before, prioritySQL(t, f, "experiment", "expt_run_log", "expt_stats", "expt_item_result", "expt_turn_result"), "HALF no terminal/end/stats writes")
	assert.Equal(t, admission, f.admission())
	assert.True(t, f.slot())
	cid, readErr := f.raw.Exists(f.ctx, f.cidKey()).Result()
	require.NoError(t, readErr)
	assert.Zero(t, cid)
	f.snapshot("HALF_after_new_DAO_after_real_old_run_write")
	if continueOld {
		require.NoError(t, completeExpt())
		current, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
		require.NoError(t, err)
		require.Equal(t, entity.ExptStatus_Success, current.Status)
		require.NotNil(t, current.EndAt)
		require.False(t, f.slot())
	} else {
		current, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
		require.NoError(t, err)
		assert.Equal(t, entity.ExptStatus_Processing, current.Status)
		assert.Nil(t, current.EndAt)
	}
}

func TestCompletionPriorityHalfWindowSQLWriter(t *testing.T) {
	priorityHalfWindow(t, entity.EvaluationModeSubmit, func(f *independentAuditFixture) (func() error, func() error) {
		return func() error {
			return f.manager.runLogRepo.Update(f.ctx, f.expt, f.r1, f.space, map[string]any{"status": int64(entity.ExptStatus_Success)})
		}, func() error { return nil }
	}, false)
}

func TestCompletionPriorityTerminationHalfSQLNoop(t *testing.T) {
	for _, shape := range [][2]entity.ExptStatus{{entity.ExptStatus_Processing, entity.ExptStatus_Success}, {entity.ExptStatus_Processing, entity.ExptStatus_Failed}, {entity.ExptStatus_Processing, entity.ExptStatus_Terminating}, {entity.ExptStatus_Terminating, entity.ExptStatus_Terminated}} {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			f := newIndependentAuditFixture(t)
			f.seedSources(1, true)
			require.NoError(t, f.manager.exptRepo.Update(f.ctx, &entity.Experiment{ID: f.expt, SpaceID: f.space, Status: shape[0]}))
			require.NoError(t, f.manager.runLogRepo.Update(f.ctx, f.expt, f.r1, f.space, map[string]any{"status": int64(shape[1])}))
			before := prioritySQL(t, f, "experiment", "expt_run_log", "expt_stats", "expt_item_result", "expt_turn_result")
			called := false
			result, err := f.manager.runFinishRepo.(repo.IExptRunTerminationRepo).TerminateCurrentRun(f.ctx, &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.r1, Finalize: true, Status: entity.ExptStatus_Terminated, BeforeCommit: func(context.Context, entity.ExptRunMode) error { called = true; return nil }})
			assert.Error(t, err, "BUSINESS_RED termination half state retains the original pair")
			assert.False(t, called)
			assert.Equal(t, before, prioritySQL(t, f, "experiment", "expt_run_log", "expt_stats", "expt_item_result", "expt_turn_result"))
			if result != nil {
				assert.False(t, result.Applied)
			}
			require.Equal(t, "evaluation", f.admission())
			require.True(t, f.slot())
		})
	}
}

type priorityQuotaFault struct {
	repo.QuotaRepo
	f      *independentAuditFixture
	cause  error
	cancel context.CancelFunc
	calls  int
	duties *[2]int
}

func (q *priorityQuotaFault) CreateOrUpdate(ctx context.Context, space int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), session *entity.Session) error {
	q.calls++
	current, err := q.f.manager.exptRepo.GetByID(q.f.ctx, q.f.expt, q.f.space)
	if err != nil {
		return err
	}
	require.True(q.f.t, entity.IsExptFinished(current.Status))
	require.NotNil(q.f.t, current.EndAt)
	q.f.log("NOTIFY_quota_boundary", map[string]any{"calls": q.calls, "duties": *q.duties, "ctx_error": fmt.Sprint(ctx.Err())})
	if q.calls == 1 && q.cause != nil {
		return q.cause
	}
	err = q.QuotaRepo.CreateOrUpdate(ctx, space, update, session)
	if q.calls == 1 && q.cancel != nil {
		q.cancel()
	}
	return err
}

type priorityAdmissionCounter struct {
	lock.ILocker
	calls atomic.Int32
}

func (l *priorityAdmissionCounter) UnlockWithContext(ctx context.Context, key string) (bool, error) {
	l.calls.Add(1)
	return l.ILocker.UnlockWithContext(ctx, key)
}

func (l *priorityAdmissionCounter) UnlockWithValue(ctx context.Context, key, value string) (bool, error) {
	l.calls.Add(1)
	return l.ILocker.UnlockWithValue(ctx, key, value)
}

func priorityNotificationCase(t *testing.T, mode entity.ExptRunMode, fault string, replay func(*independentAuditFixture) error) {
	t.Helper()
	f := newIndependentAuditFixture(t, mode)
	f.seedSources(1, true)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).Updates(map[string]any{"source_type": int32(entity.SourceType_Workflow), "expt_template_id": f.base + 700}).Error)
	duties := [2]int{}
	ctrl := gomock.NewController(t)
	pipeline := rpcmocks.NewMockIPipelineListAdapter(ctrl)
	pipeline.EXPECT().PipelineNodeFinishCallback(gomock.Any(), f.expt, f.space).DoAndReturn(func(context.Context, int64, int64) error {
		duties[0]++
		return errors.New("best effort workflow failure")
	}).AnyTimes()
	template := svcmocks.NewMockIExptTemplateManager(ctrl)
	template.EXPECT().UpdateExptInfo(gomock.Any(), f.base+700, f.space, f.expt, entity.ExptStatus_Success, int64(0), nil).DoAndReturn(func(context.Context, int64, int64, int64, entity.ExptStatus, int64, *int64) error {
		duties[1]++
		return errors.New("best effort template failure")
	}).AnyTimes()
	f.manager.pipelineListAdapter = pipeline
	f.manager.templateManager = template
	counter := &priorityAdmissionCounter{ILocker: f.manager.mutex}
	f.manager.mutex = counter
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	q := &priorityQuotaFault{QuotaRepo: f.quota, f: f, duties: &duties}
	cause := errors.New("exact first quota IO failure")
	if fault == "quota_error" {
		q.cause = cause
	} else {
		q.cancel = cancel
		cause = context.Canceled
	}
	f.manager.quotaRepo = q
	err := f.manager.CompleteExpt(ctx, f.expt, &f.r1, f.space, &entity.Session{})
	require.ErrorIs(t, err, cause)
	assert.Equal(t, [2]int{1, 1}, duties, "BUSINESS_RED NOTIFY first call must attempt workflow/template before fallible quota")
	require.Equal(t, int32(1), counter.calls.Load())
	require.Equal(t, "<absent>", f.admission())
	require.NoError(t, replay(f))
	assert.Equal(t, [2]int{1, 1}, duties, "terminal replay cannot be the first notification attempt")
	require.Equal(t, int32(1), counter.calls.Load(), "terminal replay cannot delete admission again")
	// Publish a genuine successor, then replay R1 again. Shared state belongs to R2.
	require.NoError(t, f.manager.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{}))
	require.NoError(t, NewQuotaService(f.quota, f.manager.configer).AllowExptRun(f.ctx, f.expt, f.space, &entity.Session{}))
	before := prioritySQL(t, f, "experiment", "expt_run_log", "expt_stats")
	require.NoError(t, replay(f))
	require.Equal(t, int32(1), counter.calls.Load())
	require.Equal(t, before, prioritySQL(t, f, "experiment", "expt_run_log", "expt_stats"))
	require.Equal(t, "evaluation", f.admission())
	require.True(t, f.slot())
}

func TestCompletionPriorityNotificationFaultAndNewReplay(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems} {
		for _, fault := range []string{"quota_error", "quota_cancel"} {
			t.Run(fmt.Sprintf("%d/%s", mode, fault), func(t *testing.T) {
				priorityNotificationCase(t, mode, fault, func(f *independentAuditFixture) error {
					return f.manager.CompleteExpt(f.ctx, f.expt, &f.r1, f.space, &entity.Session{})
				})
			})
		}
	}
}

type priorityLatestFailure struct {
	repo.IExperimentRepo
	f         *independentAuditFixture
	cause     error
	successor int64
	healthy   bool
}

func (r *priorityLatestFailure) Update(ctx context.Context, x *entity.Experiment) error {
	if x.LatestRunID == r.f.r2 {
		owner, ok := ctx.Value(completeTickOwnerKey{}).(context.Context)
		r.healthy = ok && owner.Err() == nil
		row, err := r.f.manager.runLogRepo.Get(r.f.ctx, r.f.expt, r.f.r2)
		if err != nil {
			return err
		}
		require.Equal(r.f.t, int64(entity.ExptStatus_Pending), row.Status, "actual Create must succeed before Latest fails")
		require.True(r.f.t, r.healthy, "original lease stays healthy")
		if r.successor != 0 {
			require.NoError(r.f.t, r.f.manager.runLogRepo.Create(r.f.ctx, &entity.ExptRunLog{ID: r.successor, SpaceID: r.f.space, ExptID: r.f.expt, ExptRunID: r.successor, Mode: int32(entity.EvaluationModeRetryAll), Status: int64(entity.ExptStatus_Pending)}))
			require.NoError(r.f.t, r.IExperimentRepo.Update(r.f.ctx, &entity.Experiment{ID: r.f.expt, SpaceID: r.f.space, LatestRunID: r.successor}))
			require.NoError(r.f.t, r.f.raw.Set(r.f.ctx, r.f.keys[0], "evaluation", time.Hour).Err())
			require.NoError(r.f.t, NewQuotaService(r.f.quota, r.f.manager.configer).AllowExptRun(r.f.ctx, r.f.expt, r.f.space, &entity.Session{}))
		}
		return r.cause
	}
	return r.IExperimentRepo.Update(ctx, x)
}

func TestCompletionPriorityOwnedR2LatestFailure(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		for _, successor := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/successor_%t", mode, successor), func(t *testing.T) {
				f := newIndependentAuditFixture(t)
				f.finishR1WithoutCID()
				cause := errors.New("exact Latest update failure after successful Create")
				fail := &priorityLatestFailure{IExperimentRepo: f.manager.exptRepo, f: f, cause: cause}
				if successor {
					fail.successor = f.r2 + 1
				}
				counter := &priorityAdmissionCounter{ILocker: f.manager.mutex}
				f.manager.mutex = counter
				f.manager.exptRepo = fail
				before := prioritySQL(t, f, "experiment", "expt_stats", "expt_item_result", "expt_turn_result")
				var err error
				if mode == entity.EvaluationModeRetryItems {
					ids := idmocks.NewMockIIDGenerator(gomock.NewController(t))
					ids.EXPECT().GenID(gomock.Any()).Return(f.r2, nil)
					f.manager.idgenerator = ids
					_, _, err = f.manager.LogRetryItemsRun(f.ctx, f.expt, mode, f.space, nil, &entity.Session{})
				} else {
					err = f.manager.LogRun(f.ctx, f.expt, f.r2, mode, f.space, nil, &entity.Session{})
				}
				require.ErrorIs(t, err, cause)
				require.True(t, fail.healthy)
				row, err := f.manager.runLogRepo.Get(f.ctx, f.expt, f.r2)
				require.NoError(t, err)
				assert.Equal(t, int64(entity.ExptStatus_Failed), row.Status, "BUSINESS_RED owned R2 created before Latest failure must close only its own row")
				assert.Equal(t, cause.Error(), string(row.StatusMessage))
				if !successor {
					require.Equal(t, before, prioritySQL(t, f, "experiment", "expt_stats", "expt_item_result", "expt_turn_result"))
					require.Equal(t, "<absent>", f.admission())
					require.False(t, f.slot())
					require.Equal(t, int32(1), counter.calls.Load())
				} else {
					current, err := fail.GetByID(f.ctx, f.expt, f.space)
					require.NoError(t, err)
					require.Equal(t, fail.successor, current.LatestRunID)
					r3, err := f.manager.runLogRepo.Get(f.ctx, f.expt, fail.successor)
					require.NoError(t, err)
					require.Equal(t, int64(entity.ExptStatus_Pending), r3.Status)
					require.Equal(t, "evaluation", f.admission())
					require.True(t, f.slot())
					require.Zero(t, counter.calls.Load())
					after := prioritySQL(t, f, "expt_stats", "expt_item_result", "expt_turn_result")
					for key, value := range after {
						require.Equal(t, before[key], value)
					}
				}
				f.snapshot("R2_Create_success_Latest_failure_healthy_original_lease")
			})
		}
	}
}
