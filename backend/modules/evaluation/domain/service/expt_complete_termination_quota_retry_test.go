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

	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	quotadao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
)

// Reuse the existing bounded Redis compare/delete guard and fault injection.
// Recording the full identity also checks that replay keeps the frozen token.
type terminationQuotaRetryGuard struct {
	*cleanupLeaseSlowGuard
	observed []releaseCall
}

func (g *terminationQuotaRetryGuard) Release(ctx context.Context, scope string, run, item int64, token, reason string) error {
	g.observed = append(g.observed, releaseCall{Scope: scope, RunID: run, ItemID: item, RequestID: token, Reason: reason})
	return g.cleanupLeaseSlowGuard.Release(ctx, scope, run, item, token, reason)
}

type terminationQuotaRetryFixture struct {
	f       *filterSyncFixture
	m       *ExptMangerImpl
	guard   *terminationQuotaRetryGuard
	quota   quotadao.IQuotaDAO
	cause   error
	holder  string
	admit   string
	done    string
	reserve string
}

func newTerminationQuotaRetryFixture(t *testing.T, mode entity.ExptRunMode) *terminationQuotaRetryFixture {
	t.Helper()
	f, m := newCancelFilterFixture(t)
	ctx := context.Background()
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("expt_id=? AND expt_run_id=?", f.expt, f.run).Update("mode", mode).Error)
	m.mutex = m.manualRetryLocker().WithHolder("evaluation")
	holder := "evaluation"
	if mode == entity.EvaluationModeRetryItems {
		holder = strconv.FormatInt(f.run, 10)
	}
	admit := m.makeExptMutexLockKey(f.expt)
	require.NoError(t, f.server.Set(admit, holder))
	f.svc.exptTurnResultFilterRepo = &filterStore{}
	raw, ok := infraredis.Unwrap(f.client)
	require.True(t, ok)
	reserve := fmt.Sprintf("termination-quota:%d:%d", f.run, f.item)
	require.NoError(t, f.server.Set(reserve, "rq1"))
	cause := errors.New("central item quota release unavailable")
	guard := &terminationQuotaRetryGuard{cleanupLeaseSlowGuard: &cleanupLeaseSlowGuard{
		raw: raw, keys: map[int64]string{f.item: reserve}, expected: map[int64]string{f.item: "rq1"}, faultOnce: cause,
	}}
	m.centralGuard = guard
	quota := quotadao.NewQuotaDAO(f.client)
	require.NoError(t, quota.SetQuotaSpaceExpt(ctx, f.space, &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{f.expt: time.Now().Unix()}}))
	m.quotaRepo = exptrepo.NewQuotaService(quota, m.manualRetryLocker())
	return &terminationQuotaRetryFixture{f: f, m: m, guard: guard, quota: quota, cause: cause, holder: holder, admit: admit, done: fmt.Sprintf("CompleteExpt:exptexec:onend:%d", f.run), reserve: reserve}
}

func (h *terminationQuotaRetryFixture) terminate(t *testing.T) {
	t.Helper()
	f, m := h.f, h.m
	applied, err := m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	// Log persisted SQL and Redis state even on RED, before the error assertion.
	current, readErr := m.exptRepo.GetByID(context.Background(), f.expt, f.space)
	require.NoError(t, readErr)
	run, readErr := m.runLogRepo.Get(context.Background(), f.expt, f.run)
	require.NoError(t, readErr)
	seen, readErr := m.idem.Exist(context.Background(), h.done)
	require.NoError(t, readErr)
	quota, readErr := h.quota.GetQuotaSpaceExpt(context.Background(), f.space)
	require.NoError(t, readErr)
	_, spaceHeld := quota.ExptID2RunTime[f.expt]
	t.Logf("REAL_MYSQL_TERMINATION_QUOTA_FIRST applied=%t expt_status=%d run_status=%d release_error=%t item_quota_held=%t admission_held=%t space_quota_held=%t done=%t", applied, current.Status, run.Status, err != nil, f.server.Exists(h.reserve), f.server.Exists(h.admit), spaceHeld, seen)
	require.True(t, applied)
	require.Equal(t, entity.ExptStatus_Terminated, current.Status)
	require.Equal(t, int64(entity.ExptStatus_Terminated), run.Status)
	require.NotNil(t, current.EndAt)
	require.ErrorIs(t, err, h.cause)
	require.True(t, errno.NeedMQRetry(err))
	require.False(t, seen, "failed central release must not record Done")
	require.True(t, spaceHeld, "item release failure must precede space admission release")
	value, readErr := f.server.Get(h.admit)
	require.NoError(t, readErr)
	require.Equal(t, h.holder, value)
	value, readErr = f.server.Get(h.reserve)
	require.NoError(t, readErr)
	require.Equal(t, "rq1", value)
}

func (h *terminationQuotaRetryFixture) replay(ctx context.Context) error {
	f := h.f
	scheduler := &ExptSchedulerImpl{Manager: h.m}
	return scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
		f.t.Fatal("terminal replay entered item execution")
		return nil
	})(ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit, Session: &entity.Session{}})
}

func TestCompleteTerminationQuotaRetryRealMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			h := newTerminationQuotaRetryFixture(t, mode)
			f := h.f
			h.guard.before = func(ctx context.Context, item int64) {
				port, err := h.m.terminationRepo()
				require.NoError(t, err)
				source, err := port.GetTerminationItemSource(ctx, f.space, f.expt, f.run, item)
				require.NoError(t, err)
				require.Equal(t, entity.ItemRunState_Terminal, source.Current.Status, "SQL must commit before the injected Release failure")
				require.Equal(t, "rq1", source.Current.RequestID)
			}
			h.terminate(t)
			before := f.snapshot()
			// Fresh context carries no completeData; production SQL recaptures
			// terminal sources instead of scanning only incomplete item rows.
			require.NoError(t, h.replay(context.Background()))
			require.Equal(t, before, f.snapshot(), "replay preserves terminal data, counts and EndAt")
			require.False(t, f.server.Exists(h.reserve))
			require.False(t, f.server.Exists(h.admit))
			quota, err := h.quota.GetQuotaSpaceExpt(context.Background(), f.space)
			require.NoError(t, err)
			require.NotContains(t, quota.ExptID2RunTime, f.expt)
			seen, err := h.m.idem.Exist(context.Background(), h.done)
			require.NoError(t, err)
			require.True(t, seen)
			require.Len(t, h.guard.observed, 2)
			for _, call := range h.guard.observed {
				require.Equal(t, testScope, call.Scope)
				require.Equal(t, f.run, call.RunID)
				require.Equal(t, f.item, call.ItemID)
				require.Equal(t, "rq1", call.RequestID)
			}
			require.NoError(t, f.server.Set(h.admit, "replacement-admission"))
			require.NoError(t, f.server.Set(h.reserve, "rq2"))
			require.NoError(t, h.replay(context.Background()))
			require.Len(t, h.guard.observed, 2, "Done replay must not release another admission or reservation")
			value, err := f.server.Get(h.admit)
			require.NoError(t, err)
			require.Equal(t, "replacement-admission", value)
			value, err = f.server.Get(h.reserve)
			require.NoError(t, err)
			require.Equal(t, "rq2", value)
			t.Logf("REAL_MYSQL_TERMINATION_QUOTA_REPLAY mode=%d original_request=rq1 released=true done=true sql_unchanged=true", mode)
		})
	}
}

func TestCompleteTerminationQuotaFrozenSourcePreservesNewAttempt(t *testing.T) {
	h := newTerminationQuotaRetryFixture(t, entity.EvaluationModeSubmit)
	f := h.f
	original := f.source()
	h.guard.faultOnce = nil
	var afterAdvance map[string]string
	h.guard.before = func(context.Context, int64) {
		// Advance after SQL termination commits, before its frozen source is
		// passed to Redis. Compare/delete must keep the new reservation.
		f.token("rq2")
		require.NoError(t, f.server.Set(h.reserve, "rq2"))
		afterAdvance = f.snapshot()
	}
	_, err := h.m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.Error(t, err, "existing filter source protection must retain the advanced source")
	require.Len(t, h.guard.observed, 1)
	require.Equal(t, "rq1", h.guard.observed[0].RequestID)
	value, err := f.server.Get(h.reserve)
	require.NoError(t, err)
	require.Equal(t, "rq2", value, "old frozen source cannot clear new attempt quota")
	applied, err := h.m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, original, entity.NoAggrCalculate())
	require.NoError(t, err)
	require.False(t, applied, "old ExpectedSource must stop before resource release")
	require.Len(t, h.guard.observed, 1)
	require.Equal(t, afterAdvance, f.snapshot())
	seen, err := h.m.idem.Exist(context.Background(), h.done)
	require.NoError(t, err)
	require.False(t, seen)
	require.True(t, f.server.Exists(h.admit))
	t.Log("REAL_MYSQL_TERMINATION_QUOTA_NEW_ATTEMPT old_request=rq1 new_request=rq2 new_reservation_preserved=true")
}

func TestCompleteTerminationQuotaRetryPreservesConfirmedNewRun(t *testing.T) {
	h := newTerminationQuotaRetryFixture(t, entity.EvaluationModeRetryItems)
	h.terminate(t)
	f := h.f
	replacement := f.run + 100
	runStatus, runMode := int64(entity.ExptStatus_Processing), int32(entity.EvaluationModeRetryItems)
	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: replacement, SpaceID: f.space, ExptID: f.expt, ExptRunID: replacement, Status: &runStatus, Mode: &runMode}).Error)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{"latest_run_id": replacement, "status": entity.ExptStatus_Processing}).Error)
	newAdmission := strconv.FormatInt(replacement, 10)
	require.NoError(t, f.server.Set(h.admit, newAdmission))
	newReserve := fmt.Sprintf("termination-quota:%d:%d", replacement, f.item)
	require.NoError(t, f.server.Set(newReserve, "rq2"))
	require.NoError(t, h.quota.SetQuotaSpaceExpt(context.Background(), f.space, &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{f.expt: 1700000100}}))
	before := f.snapshot()
	require.NoError(t, h.replay(context.Background()))
	require.Equal(t, before, f.snapshot())
	require.Len(t, h.guard.observed, 1, "confirmed new run exits before old cleanup")
	for key, expected := range map[string]string{h.admit: newAdmission, h.reserve: "rq1", newReserve: "rq2"} {
		value, err := f.server.Get(key)
		require.NoError(t, err)
		require.Equal(t, expected, value)
	}
	quota, err := h.quota.GetQuotaSpaceExpt(context.Background(), f.space)
	require.NoError(t, err)
	require.Equal(t, int64(1700000100), quota.ExptID2RunTime[f.expt])
	seen, err := h.m.idem.Exist(context.Background(), h.done)
	require.NoError(t, err)
	require.False(t, seen)
	t.Log("REAL_MYSQL_TERMINATION_QUOTA_NEW_RUN old_replay_skipped=true new_admission_and_quota_preserved=true")
}
