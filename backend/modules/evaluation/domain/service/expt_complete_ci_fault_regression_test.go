// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	quotadao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Use production SQL completion and Redis quota/idempotency ports. The ordinary
// admission holder must match the manager; RetryItems uses its numeric run ID.
func newCompleteCIFilterFixture(t *testing.T, mode entity.ExptRunMode) (*filterSyncFixture, *ExptMangerImpl, quotadao.IQuotaDAO) {
	t.Helper()
	f, m := newCancelFilterFixture(t)
	m.mutex = m.manualRetryLocker().WithHolder(legacyStaticHolderValue)
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.run).Update("mode", mode).Error)
	holder := legacyStaticHolderValue
	if mode == entity.EvaluationModeRetryItems {
		holder = strconv.FormatInt(f.run, 10)
	}
	require.NoError(t, f.server.Set(m.makeExptMutexLockKey(f.expt), holder))
	quota := quotadao.NewQuotaDAO(f.client)
	require.NoError(t, quota.SetQuotaSpaceExpt(context.Background(), f.space, &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{f.expt: time.Now().Unix()}}))
	m.quotaRepo = exptrepo.NewQuotaService(quota, m.manualRetryLocker())
	return f, m, quota
}

func completeCIDone(t *testing.T, m *ExptMangerImpl, run int64) bool {
	t.Helper()
	marked, err := m.idem.Exist(context.Background(), fmt.Sprintf("CompleteExpt:exptexec:onend:%d", run))
	require.NoError(t, err)
	return marked
}

type completeCISlowFilter struct {
	*filterStore
	wait           time.Duration
	observedCancel error
}

func (s *completeCISlowFilter) Save(ctx context.Context, rows []*entity.ExptTurnResultFilterEntity) error {
	timer := time.NewTimer(s.wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		s.observedCancel = ctx.Err()
		return ctx.Err()
	case <-timer.C:
	}
	s.observedCancel = ctx.Err()
	return s.filterStore.Save(ctx, rows)
}

func TestCompleteCIFaultR02SlowFilterPersistsDone(t *testing.T) {
	f, m, quota := newCompleteCIFilterFixture(t, entity.EvaluationModeSubmit)
	calls := 0
	sink := &completeCISlowFilter{filterStore: &filterStore{}, wait: 9 * time.Second}
	sink.beforeSave = func(rows []*entity.ExptTurnResultFilterEntity) {
		calls++
		require.True(t, f.server.Exists(f.key()), "filter write retains the actual item lease after nine seconds")
		require.True(t, f.server.Exists(m.makeExptMutexLockKey(f.expt)))
		require.Len(t, rows, 1)
		require.Equal(t, entity.ItemRunState_Terminal, rows[0].Status)
	}
	f.svc.exptTurnResultFilterRepo = sink
	start := time.Now()
	applied, err := m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	marked := completeCIDone(t, m, f.run)
	t.Logf("R02 elapsed=%s filter_cancel=%v filter_rows=%d done=%t admission_held=%t", time.Since(start), sink.observedCancel, len(sink.rows), marked, f.server.Exists(m.makeExptMutexLockKey(f.expt)))
	require.NoError(t, err)
	require.True(t, applied)
	require.GreaterOrEqual(t, time.Since(start), 9*time.Second)
	require.NoError(t, sink.observedCancel, "healthy filter uses its original context")
	require.Len(t, sink.rows, 1)
	require.Equal(t, entity.ItemRunState_Terminal, sink.rows[0].Status)
	require.False(t, f.server.Exists(m.makeExptMutexLockKey(f.expt)))
	require.True(t, marked, "R02: healthy nine-second filter must persist the standard completion CID")
	require.Equal(t, 3*time.Minute, f.server.TTL(fmt.Sprintf("[idempotent]idem_key:CompleteExpt:exptexec:onend:%d", f.run)))
	spaceQuota, err := quota.GetQuotaSpaceExpt(context.Background(), f.space)
	require.NoError(t, err)
	require.NotContains(t, spaceQuota.ExptID2RunTime, f.expt)
	before := f.snapshot()
	// A same-CID replay must protect even a replacement using the same ordinary
	// holder and a newly occupied space slot, without rewriting terminal SQL.
	require.NoError(t, f.server.Set(m.makeExptMutexLockKey(f.expt), legacyStaticHolderValue))
	replacement := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{f.expt: time.Now().Unix() + 100}}
	require.NoError(t, quota.SetQuotaSpaceExpt(context.Background(), f.space, replacement))
	sink.wait = 0
	require.NoError(t, m.CompleteExpt(context.Background(), f.expt, &f.run, f.space, &entity.Session{}, entity.NoAggrCalculate()))
	require.Equal(t, 1, calls, "same CID cannot repeat filter or shared resource cleanup")
	require.True(t, f.server.Exists(m.makeExptMutexLockKey(f.expt)))
	spaceQuota, err = quota.GetQuotaSpaceExpt(context.Background(), f.space)
	require.NoError(t, err)
	require.Equal(t, replacement, spaceQuota)
	require.Equal(t, before, f.snapshot())
}

func TestCompleteCIFaultR02CancelledResourceBudgetPersistsDone(t *testing.T) {
	f, m, quota := newCompleteCIFilterFixture(t, entity.EvaluationModeSubmit)
	store := &filterStore{}
	f.svc.exptTurnResultFilterRepo = store
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	target := svcmocks.NewMockIEvalTargetService(gomock.NewController(t))
	target.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), f.space, gomock.Any(), int32(errno.AsyncEvalTargetTerminatedCode), gomock.Any(), false).
		DoAndReturn(func(resourceCtx context.Context, _ int64, _ []int64, _ int32, _ string, _ bool, _ ...bool) error {
			_, bounded := resourceCtx.Deadline()
			require.True(t, bounded)
			<-resourceCtx.Done()
			cancel()
			return resourceCtx.Err()
		})
	m.evalTargetService = target
	_, err := m.TerminateCurrentRun(ctx, f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoCompleteItemTurn(), entity.NoAggrCalculate())
	marked := completeCIDone(t, m, f.run)
	t.Logf("R02 cancelled_resource_budget done=%t admission_held=%t", marked, f.server.Exists(m.makeExptMutexLockKey(f.expt)))
	require.ErrorIs(t, err, context.Canceled)
	require.True(t, errno.NeedMQRetry(err))
	require.Empty(t, store.rows)
	require.False(t, f.server.Exists(m.makeExptMutexLockKey(f.expt)))
	spaceQuota, readErr := quota.GetQuotaSpaceExpt(context.Background(), f.space)
	require.NoError(t, readErr)
	require.NotContains(t, spaceQuota.ExptID2RunTime, f.expt)
	require.True(t, marked, "R02: cancellation branch must also give the actual Set a fresh bounded context")
}

func TestCompleteCIFaultR07CancelledFinalizeRemainsReplayable(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems} {
		for _, noComplete := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode_%d/no_complete_%t", mode, noComplete), func(t *testing.T) {
				f, m, quota := newCompleteCIFilterFixture(t, mode)
				store := &filterStore{}
				calls := 0
				store.beforeSave = func([]*entity.ExptTurnResultFilterEntity) { calls++ }
				f.svc.exptTurnResultFilterRepo = store
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				finish := m.runFinishRepo
				port, err := m.terminationRepo()
				require.NoError(t, err)
				m.runFinishRepo = &cancelFilterCommitHook{IExptRunFinishRepo: finish, port: port, after: cancel}
				opts := []entity.CompleteExptOptionFn{entity.NoAggrCalculate()}
				if noComplete {
					opts = append(opts, entity.NoCompleteItemTurn())
				}
				applied, err := m.TerminateCurrentRun(ctx, f.expt, f.run, f.space, &entity.Session{}, nil, opts...)
				marked := completeCIDone(t, m, f.run)
				spaceQuota, readErr := quota.GetQuotaSpaceExpt(context.Background(), f.space)
				require.NoError(t, readErr)
				t.Logf("R07 no_complete=%t applied=%t filter_calls=%d done=%t admission_held=%t quota_slots=%d", noComplete, applied, calls, marked, f.server.Exists(m.makeExptMutexLockKey(f.expt)), len(spaceQuota.ExptID2RunTime))
				require.True(t, applied)
				require.ErrorIs(t, err, context.Canceled)
				require.True(t, errno.NeedMQRetry(err))
				require.Zero(t, calls)
				require.Empty(t, store.rows)
				current, readErr := m.exptRepo.GetByID(context.Background(), f.expt, f.space)
				require.NoError(t, readErr)
				require.Equal(t, entity.ExptStatus_Terminated, current.Status)
				require.NotNil(t, current.EndAt)
				before := f.snapshot()
				if noComplete {
					var turn model.ExptTurnResult
					require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&turn).Error)
					require.Equal(t, int32(entity.TurnRunState_Processing), turn.Status)
					require.True(t, marked)
					require.False(t, f.server.Exists(m.makeExptMutexLockKey(f.expt)))
					require.NotContains(t, spaceQuota.ExptID2RunTime, f.expt)
				} else {
					require.False(t, marked, "R07: required filter has not run, so completion cannot record Done")
					holder, getErr := f.server.Get(m.makeExptMutexLockKey(f.expt))
					require.NoError(t, getErr)
					want := legacyStaticHolderValue
					if mode == entity.EvaluationModeRetryItems {
						want = strconv.FormatInt(f.run, 10)
					}
					require.Equal(t, want, holder, "filter retry retains the original admission")
					require.Contains(t, spaceQuota.ExptID2RunTime, f.expt)
					// Exercise a standard fresh-context replay through the scheduler,
					// so sources must be recovered from real SQL after Finalize.
					m.runFinishRepo = finish
					scheduler := &ExptSchedulerImpl{Manager: m}
					handler := scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
						t.Fatal("finished replay entered execution")
						return nil
					})
					require.NoError(t, handler(context.Background(), &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ExptType: entity.ExptType_Offline, ExptRunMode: mode, Session: &entity.Session{}}))
					require.Positive(t, calls)
					require.Len(t, store.rows, 1)
					require.Equal(t, entity.ItemRunState_Terminal, store.rows[0].Status)
					require.True(t, completeCIDone(t, m, f.run))
					require.False(t, f.server.Exists(m.makeExptMutexLockKey(f.expt)))
					spaceQuota, readErr = quota.GetQuotaSpaceExpt(context.Background(), f.space)
					require.NoError(t, readErr)
					require.NotContains(t, spaceQuota.ExptID2RunTime, f.expt)
				}
				require.Equal(t, before, f.snapshot(), "filter recovery preserves SQL stats, run identity and EndAt")
				callsBefore := calls
				require.NoError(t, m.CompleteExpt(context.Background(), f.expt, &f.run, f.space, &entity.Session{}, opts...))
				require.Equal(t, callsBefore, calls)
				require.Equal(t, before, f.snapshot())
			})
		}
	}
}

func TestCompleteCIFaultR12SchedulerPreservesPendingFailureCause(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			p := newSetupCompensationFixture(t, mode)
			ctx, f, m := context.Background(), p.f, p.m
			_, err := p.prepare(ctx)
			require.NoError(t, err)
			p.checkAdmission(t, true)
			require.Contains(t, p.quota.ExptID2RunTime, f.expt)
			cause := errors.New("evaluation set item 999 does not exist: source version 123\n原始失败原因保留")
			scheduler := &ExptSchedulerImpl{Manager: m, ExptRepo: m.exptRepo}
			require.NoError(t, scheduler.HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error { return cause })(ctx, p.event))
			logged, err := m.runLogRepo.Get(ctx, f.expt, f.run)
			require.NoError(t, err)
			t.Logf("R12 run_status=%d original_cause_preserved=%t quota_releases=%d", logged.Status, string(logged.StatusMessage) == cause.Error(), p.releases)
			require.Equal(t, int64(entity.ExptStatus_Failed), logged.Status)
			require.Equal(t, cause.Error(), string(logged.StatusMessage), "R12: existing scheduler options must reach pending compensation intact")
			p.checkOriginalExperiment(t)
			p.checkAdmission(t, false)
			require.NotContains(t, p.quota.ExptID2RunTime, f.expt)
			require.Equal(t, 1, p.releases)
			require.NoError(t, NewQuotaService(m.quotaRepo, m.configer).AllowExptRun(ctx, f.expt+100, f.space, p.event.Session), "released space limit=1 permits the next experiment")
			// A new RetryItems admission uses a fresh numeric ID after both modes.
			newRun := f.run + 100
			nextIDs := idgenmocks.NewMockIIDGenerator(gomock.NewController(t))
			nextIDs.EXPECT().GenID(gomock.Any()).Return(newRun, nil)
			m.idgenerator = nextIDs
			m.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecRun(f.space, int64(entity.EvaluationModeRetryItems))
			admitted, joined, err := m.LogRetryItemsRun(ctx, f.expt, entity.EvaluationModeRetryItems, f.space, []int64{f.item}, p.event.Session)
			require.NoError(t, err)
			require.False(t, joined)
			require.Equal(t, newRun, admitted)
			holder, err := p.server.Get(m.makeExptMutexLockKey(f.expt))
			require.NoError(t, err)
			require.Equal(t, strconv.FormatInt(newRun, 10), holder)
		})
	}
}

// Verify the optional message does not remove the existing generic cause for
// callers that omit it. Reuse real admission and the same compensation path.
func TestCompleteCIFaultR12PendingFailureCauseFallback(t *testing.T) {
	p := newSetupCompensationFixture(t, entity.EvaluationModeRetryAll)
	_, err := p.prepare(context.Background())
	require.NoError(t, err)
	require.NoError(t, p.m.CompleteRun(context.Background(), p.f.expt, p.f.run, p.f.space, p.event.Session))
	logged, err := p.m.runLogRepo.Get(context.Background(), p.f.expt, p.f.run)
	require.NoError(t, err)
	require.Equal(t, "run setup did not start", string(logged.StatusMessage))
	p.checkOriginalExperiment(t)
	p.checkAdmission(t, false)
}
