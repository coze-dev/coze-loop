// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type completeReleaseFaultLocker struct {
	lock.ILocker
	fault error
	calls int
}

// Only detail loading is narrowed to primary SQL. Both Complete calls and the
// Append draining body remain production implementations.
type completeOnlineDetailManager struct {
	*ExptMangerImpl
	set *entity.EvaluationSet
}

func (m *completeOnlineDetailManager) GetDetail(ctx context.Context, expt, space int64, _ *entity.Session, _ ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	current, err := m.exptRepo.GetByID(ctx, expt, space)
	if current != nil {
		current.EvalSet = m.set
		current.EvalConf = &entity.EvaluationConfiguration{ItemConcurNum: gptr.Of(1)}
	}
	return current, err
}

func TestCompleteStandardOnlineDrainingFailureKeepsOriginalReleasePair(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	p.publishSuccess(t, false)
	f, ctx := p.f, context.Background()
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"expt_type": entity.ExptType_Online, "status": entity.ExptStatus_Draining}).Error)
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("mode", entity.EvaluationModeAppend).Error)
	ctrl := gomock.NewController(t)
	result := svcmocks.NewMockExptResultService(ctrl)
	failure := errors.New("online CalculateStats unavailable")
	statsCalls := 0
	result.EXPECT().CalculateStats(gomock.Any(), f.expt, f.space, gomock.Any()).DoAndReturn(func(context.Context, int64, int64, *entity.Session) (*entity.ExptCalculateStats, error) {
		statsCalls++
		return nil, failure
	})
	result.EXPECT().CalculateStats(gomock.Any(), f.expt, f.space, gomock.Any()).Return(&entity.ExptCalculateStats{SuccessItemCnt: 1}, nil)
	result.EXPECT().GetIncompleteTurns(gomock.Any(), f.expt, f.space, gomock.Any()).Return(nil, nil)
	p.manager.exptResultService = result
	locker := &completeReleaseFaultLocker{ILocker: p.manager.mutex}
	p.manager.mutex = locker
	manager := &completeOnlineDetailManager{ExptMangerImpl: p.manager, set: p.startSet()}
	event := *p.event
	event.ExptType, event.ExptRunMode = entity.ExptType_Online, entity.EvaluationModeAppend
	appendMode := &ExptAppendExec{manager: manager, idem: p.manager.idem, configer: p.manager.configer, exptItemResultRepo: f.itemRepo(), mutex: p.factory()}
	next, err := appendMode.ExptEnd(ctx, &event, p.expt, 0, 0)
	require.NoError(t, err, "existing Append daemon consumes the second-half completion error")
	require.False(t, next)
	require.Equal(t, 1, statsCalls, "the real CompleteExpt error path must have run")
	key := p.manager.makeExptMutexLockKey(f.expt)
	admitted, err := locker.Exists(ctx, key)
	require.NoError(t, err)
	require.False(t, admitted, "Online CompleteRun must have released admission before the failed second half")
	require.Equal(t, 1, locker.calls)
	run, err := p.manager.runLogRepo.Get(ctx, f.expt, f.run)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Success), run.Status)
	// The Online terminal MQ acknowledges without running either completion body.
	scheduler := &ExptSchedulerImpl{Manager: manager}
	require.NoError(t, scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
		t.Fatal("Online terminal message must keep its existing ACK behavior")
		return nil
	})(ctx, &event))
	locked, _, err := locker.BackoffLockWithValue(ctx, key, "evaluation", time.Hour, time.Millisecond)
	require.NoError(t, err)
	require.True(t, locked)
	require.NoError(t, p.manager.CompleteExpt(ctx, f.expt, &f.run, f.space, p.event.Session, entity.WithCID(fmt.Sprintf("exptexec:onend:%d", f.run))))
	require.Equal(t, 1, locker.calls, "Online CompleteExpt must not release a second admission")
	admitted, err = locker.Exists(ctx, key)
	require.NoError(t, err)
	require.True(t, admitted)
	t.Log("REAL_MYSQL_ONLINE_DRAINING second_half_error_consumed=true complete_run_release_once=true terminal_mq_ack=true")
}

func (p *pendingRunFixture) startSet() *entity.EvaluationSet {
	return &entity.EvaluationSet{ID: p.f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: p.f.base + 81}}
}

type completeStartReadFailureIdem struct {
	idem.IdempotentService
	key   string
	cause error
}

func (i *completeStartReadFailureIdem) Exist(ctx context.Context, key string) (bool, error) {
	if key == i.key {
		return false, i.cause
	}
	return i.IdempotentService.Exist(ctx, key)
}

func TestCompleteStandardFailedRetryStartReleasesReservedSpaceQuota(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		for _, fault := range []string{"none", "quota", "replacement"} {
			t.Run(fmt.Sprintf("%d/%s", mode, fault), func(t *testing.T) {
				f, ctx := newLateWriteFixture(t, "start-source"), context.Background()
				ctrl := gomock.NewController(t)
				m := newTestExptManager(ctrl)
				factory, server := finishRealLockerFactory(t)
				m.mutex, m.manualRetryLocker = factory().WithHolder("evaluation"), factory
				m.exptRepo, m.itemResultRepo, m.turnResultRepo = f.experimentRepo(), f.itemRepo(), f.turnRepo()
				m.runLogRepo = exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(f.provider))
				m.runFinishRepo = exptrepo.NewExptRunFinishRepo(f.provider)
				m.idem = &cleanupTestIdem{}
				ended := time.Now().Add(-time.Minute).Truncate(time.Second)
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{"status": entity.ExptStatus_Success, "latest_run_id": f.run - 1, "end_at": ended}).Error)
				require.NoError(t, f.conn.Exec("DELETE FROM expt_run_log WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
				cfg := configmocks.NewMockIConfiger(ctrl)
				cfg.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{SpaceExptConcurLimit: 1, ZombieIntervalSecond: 3600}).AnyTimes()
				m.configer = cfg
				m.idgenerator.(*idgenmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(f.run, nil).AnyTimes()
				m.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecRun(f.space, int64(mode))
				m.lwt.(*lwtmocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), f.expt).Return(false)
				m.evaluationSetService.(*svcmocks.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil)
				m.exptResultService.(*svcmocks.MockExptResultService).EXPECT().MGetStats(gomock.Any(), []int64{f.expt}, f.space, gomock.Any()).Return(nil, nil)
				m.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), f.space, []int64{f.expt}).Return(nil, nil)
				quota := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{}}
				quotaFailure := errors.New("startup space quota release unavailable")
				failRelease, releases := fault != "none", 0
				m.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
					next, changed, err := update(quota.Clone())
					if err != nil {
						return err
					}
					if _, had := quota.ExptID2RunTime[f.expt]; had {
						if _, has := next.ExptID2RunTime[f.expt]; !has {
							releases++
							if failRelease {
								return quotaFailure
							}
						}
					}
					if changed {
						quota = next
					}
					return nil
				}).AnyTimes()
				session := &entity.Session{UserID: "failed-start"}
				if mode == entity.EvaluationModeRetryItems {
					run, joined, err := m.LogRetryItemsRun(ctx, f.expt, mode, f.space, []int64{f.item}, session)
					require.NoError(t, err)
					require.False(t, joined)
					require.Equal(t, f.run, run)
				} else {
					require.NoError(t, m.LogRun(ctx, f.expt, f.run, mode, f.space, nil, session))
				}
				prepared, err := m.prepareRun(ctx, f.expt, f.run, f.space, session, mode != entity.EvaluationModeRetryItems)
				require.NoError(t, err)
				require.Contains(t, quota.ExptID2RunTime, f.expt, "actual prepareRun must have reserved its slot")
				var original model.Experiment
				require.NoError(t, f.conn.Where("id=?", f.expt).Take(&original).Error)
				event := &entity.ExptScheduleEvent{ExptID: f.expt, ExptRunID: f.run, SpaceID: f.space, ExptType: entity.ExptType_Offline, ExptRunMode: mode, Session: session}
				cause := errors.New("evaluation set item 999 does not exist")
				startIdem := &completeStartReadFailureIdem{IdempotentService: m.idem, key: makeStartIdemKey(event), cause: cause}
				scheduler := &ExptSchedulerImpl{Manager: m, ExptRepo: m.exptRepo}
				handler := scheduler.HandleEventErr(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
					if mode == entity.EvaluationModeRetryItems {
						return (&ExptRetryItemsExec{idem: startIdem}).ExptStart(ctx, event, prepared)
					}
					return (&ExptRetryAllExec{idem: startIdem}).ExptStart(ctx, event, prepared)
				})
				err = handler(ctx, event)
				if failRelease {
					require.Error(t, err, "a failed quota release must remain observable to the scheduler")
					require.Contains(t, quota.ExptID2RunTime, f.expt)
					logged, err := m.runLogRepo.Get(ctx, f.expt, f.run)
					require.NoError(t, err)
					require.Equal(t, int64(entity.ExptStatus_Pending), logged.Status)
					require.True(t, server.Exists(m.makeExptMutexLockKey(f.expt)))
					if fault == "replacement" {
						require.NoError(t, m.exptRepo.Update(ctx, &entity.Experiment{ID: f.expt, LatestRunID: f.run + 1, Status: entity.ExptStatus_Processing}))
						require.NoError(t, server.Set(m.makeExptMutexLockKey(f.expt), "replacement-admission"))
						quota.ExptID2RunTime[f.expt] = time.Now().Unix() + 100
						before, releaseCalls := f.snapshot(), releases
						require.NoError(t, handler(ctx, event))
						require.Equal(t, before, f.snapshot())
						require.Equal(t, releaseCalls, releases)
						require.Contains(t, quota.ExptID2RunTime, f.expt)
						value, err := server.Get(m.makeExptMutexLockKey(f.expt))
						require.NoError(t, err)
						require.Equal(t, "replacement-admission", value)
						return
					}
					failRelease = false
					err = handler(ctx, event)
				}
				require.NoError(t, err)
				require.NotContains(t, quota.ExptID2RunTime, f.expt)
				require.False(t, server.Exists(m.makeExptMutexLockKey(f.expt)))
				logged, err := m.runLogRepo.Get(ctx, f.expt, f.run)
				require.NoError(t, err)
				require.Equal(t, int64(entity.ExptStatus_Failed), logged.Status)
				var after model.Experiment
				require.NoError(t, f.conn.Where("id=?", f.expt).Take(&after).Error)
				require.Equal(t, original, after, "setup failure preserves the old experiment Success and EndAt")
				t.Logf("REAL_MYSQL_PREPARERUN_START_FAILURE mode=%d fault=%s quota_release_attempts=%d original_experiment_unchanged=true", mode, fault, releases)
			})
		}
	}
}

func (l *completeReleaseFaultLocker) Unlock(key string) (bool, error) {
	l.calls++
	if l.fault != nil {
		err := l.fault
		l.fault = nil
		return false, err
	}
	return l.ILocker.Unlock(key)
}

type completeReleaseFaultRunRepo struct {
	repo.IExptRunLogRepo
	fault error
}

func (r *completeReleaseFaultRunRepo) Get(ctx context.Context, expt, run int64) (*entity.ExptRunLog, error) {
	if r.fault != nil {
		err := r.fault
		r.fault = nil
		return nil, err
	}
	return r.IExptRunLogRepo.Get(ctx, expt, run)
}

func TestCompleteStandardAdmissionFailureRemainsReplayable(t *testing.T) {
	for _, fault := range []string{"run_read", "unlock"} {
		t.Run(fault, func(t *testing.T) {
			p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
			p.publishSuccess(t, false)
			f, ctx := p.f, context.Background()
			failure := errors.New("final admission release unavailable")
			locker := &completeReleaseFaultLocker{ILocker: p.manager.mutex}
			p.manager.mutex = locker
			if fault == "unlock" {
				locker.fault = failure
			} else {
				p.manager.runLogRepo = &completeReleaseFaultRunRepo{IExptRunLogRepo: p.manager.runLogRepo, fault: failure}
			}
			key := p.manager.makeExptMutexLockKey(f.expt)
			cid := fmt.Sprintf("CompleteExpt:exptexec:onend:%d", f.run)
			require.ErrorIs(t, p.manager.CompleteExpt(ctx, f.expt, &f.run, f.space, p.event.Session), failure)
			before := f.snapshot()
			run, err := p.manager.runLogRepo.Get(ctx, f.expt, f.run)
			require.NoError(t, err)
			require.Equal(t, int64(entity.ExptStatus_Success), run.Status)
			admitted, err := p.manager.mutex.Exists(ctx, key)
			require.NoError(t, err)
			require.True(t, admitted)
			seen, err := p.manager.idem.Exist(ctx, cid)
			require.NoError(t, err)
			require.False(t, seen, "failed final release must not be hidden by a completed CID")
			require.NoError(t, p.manager.CompleteExpt(ctx, f.expt, &f.run, f.space, p.event.Session))
			require.Equal(t, before, f.snapshot(), "release replay cannot rewrite atomic completion data")
			admitted, err = p.manager.mutex.Exists(ctx, key)
			require.NoError(t, err)
			require.False(t, admitted)
			calls, resources := locker.calls, p.resources.Load()
			require.NoError(t, p.manager.CompleteExpt(ctx, f.expt, &f.run, f.space, p.event.Session))
			require.Equal(t, calls, locker.calls, "known successful release must not be attempted again")
			require.Equal(t, resources, p.resources.Load())
			t.Logf("REAL_MYSQL_FINAL_RELEASE_RECOVERY fault=%s terminal_data_unchanged=true duplicate_unlock=false", fault)
		})
	}
}
