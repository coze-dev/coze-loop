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

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type setupCompensationStartReadFailureIdem struct {
	idem.IdempotentService
	key   string
	cause error
}

func (i *setupCompensationStartReadFailureIdem) Exist(ctx context.Context, key string) (bool, error) {
	if key == i.key {
		return false, i.cause
	}
	return i.IdempotentService.Exist(ctx, key)
}

type setupCompensationFixture struct {
	f                    *lateWriteFixture
	m                    *ExptMangerImpl
	server               *miniredis.Miniredis
	event                *entity.ExptScheduleEvent
	quota                *entity.QuotaSpaceExpt
	quotaFault           error
	quotaCommittedFault  bool
	quotaCalls, releases int
	original             model.Experiment
}

func newSetupCompensationFixture(t *testing.T, mode entity.ExptRunMode) *setupCompensationFixture {
	t.Helper()
	f, ctx := newLateWriteFixture(t, "setup-source"), context.Background()
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
	m.lwt.(*lwtmocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), f.expt).Return(false).AnyTimes()
	m.evaluationSetService.(*svcmocks.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil).AnyTimes()
	m.exptResultService.(*svcmocks.MockExptResultService).EXPECT().MGetStats(gomock.Any(), []int64{f.expt}, f.space, gomock.Any()).Return(nil, nil).AnyTimes()
	m.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), f.space, []int64{f.expt}).Return(nil, nil).AnyTimes()
	p := &setupCompensationFixture{f: f, m: m, server: server, quota: &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{}}}
	m.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		p.quotaCalls++
		next, changed, err := update(p.quota.Clone())
		if err != nil {
			return err
		}
		_, had := p.quota.ExptID2RunTime[f.expt]
		_, has := next.ExptID2RunTime[f.expt]
		if had && !has {
			p.releases++
		}
		if p.quotaFault != nil && !has {
			if changed && p.quotaCommittedFault {
				p.quota = next
			}
			return p.quotaFault
		}
		if changed {
			p.quota = next
		}
		return nil
	}).AnyTimes()
	session := &entity.Session{UserID: "setup-compensation"}
	p.event = &entity.ExptScheduleEvent{ExptID: f.expt, ExptRunID: f.run, SpaceID: f.space, ExptType: entity.ExptType_Offline, ExptRunMode: mode, Session: session, CreatedAt: time.Now().Unix()}
	if mode == entity.EvaluationModeRetryItems {
		run, joined, err := m.LogRetryItemsRun(ctx, f.expt, mode, f.space, []int64{f.item}, session)
		require.NoError(t, err)
		require.False(t, joined)
		require.Equal(t, f.run, run)
	} else {
		require.NoError(t, m.LogRun(ctx, f.expt, f.run, mode, f.space, nil, session))
	}
	require.NoError(t, f.conn.Where("id=?", f.expt).Take(&p.original).Error)
	return p
}

func (p *setupCompensationFixture) checkAdmission(t *testing.T, present bool) {
	t.Helper()
	key := p.m.makeExptMutexLockKey(p.f.expt)
	require.Equal(t, present, p.server.Exists(key))
	if present {
		value, err := p.server.Get(key)
		require.NoError(t, err)
		want := "evaluation"
		if p.event.ExptRunMode == entity.EvaluationModeRetryItems {
			want = strconv.FormatInt(p.f.run, 10)
		}
		require.Equal(t, want, value)
	}
}

func (p *setupCompensationFixture) prepare(ctx context.Context) (*entity.Experiment, error) {
	if p.event.ExptRunMode == entity.EvaluationModeRetryItems {
		return p.m.prepareRun(ctx, p.f.expt, p.f.run, p.f.space, p.event.Session, false)
	}
	return p.m.prepareRun(ctx, p.f.expt, p.f.run, p.f.space, p.event.Session)
}

func (p *setupCompensationFixture) checkOriginalExperiment(t *testing.T) {
	t.Helper()
	var after model.Experiment
	require.NoError(t, p.f.conn.Where("id=?", p.f.expt).Take(&after).Error)
	require.Equal(t, p.original, after, "failed setup must preserve the old experiment terminal status, EndAt and counters")
}

type setupCompensationRunUpdateFault struct {
	repo.IExptRunLogRepo
	fault error
}

func (r *setupCompensationRunUpdateFault) Update(ctx context.Context, expt, run, space int64, fields map[string]any) error {
	if r.fault != nil {
		return r.fault
	}
	return r.IExptRunLogRepo.Update(ctx, expt, run, space, fields)
}

func TestSetupCompensationReservedQuota(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		for _, fault := range []string{"none", "quota", "quota_committed_error", "run_update", "replacement"} {
			t.Run(fmt.Sprintf("%d/%s", mode, fault), func(t *testing.T) {
				p := newSetupCompensationFixture(t, mode)
				f, m, ctx := p.f, p.m, context.Background()
				prepared, err := p.prepare(ctx)
				require.NoError(t, err)
				require.Contains(t, p.quota.ExptID2RunTime, f.expt, "real prepareRun must reserve a space slot")
				p.checkAdmission(t, true)
				failure := errors.New("setup compensation dependency unavailable")
				if fault == "quota" || fault == "quota_committed_error" || fault == "replacement" {
					p.quotaFault, p.quotaCommittedFault = failure, fault == "quota_committed_error"
				}
				runs := &setupCompensationRunUpdateFault{IExptRunLogRepo: m.runLogRepo}
				if fault == "run_update" {
					runs.fault = failure
					m.runLogRepo = runs
				}
				startIdem := &setupCompensationStartReadFailureIdem{IdempotentService: m.idem, key: makeStartIdemKey(p.event), cause: errors.New("evaluation set item 999 does not exist")}
				startCalls := 0
				scheduler := &ExptSchedulerImpl{Manager: m, ExptRepo: m.exptRepo, Configer: m.configer}
				handler := scheduler.HandleEventCheck(scheduler.HandleEventErr(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
					startCalls++
					if mode == entity.EvaluationModeRetryItems {
						return (&ExptRetryItemsExec{idem: startIdem}).ExptStart(ctx, event, prepared)
					}
					return (&ExptRetryAllExec{idem: startIdem}).ExptStart(ctx, event, prepared)
				}))
				err = handler(ctx, p.event)
				if fault != "none" {
					require.Error(t, err, "a failed compensation must not ACK its MQ event")
					if fault != "run_update" {
						require.True(t, errno.NeedMQRetry(err), "quota errors must retain the existing MQ retry error classification")
					}
					logged, readErr := m.runLogRepo.Get(ctx, f.expt, f.run)
					require.NoError(t, readErr)
					require.Equal(t, int64(entity.ExptStatus_Pending), logged.Status, "terminal run ACK must not cut off compensation replay")
					p.checkAdmission(t, fault != "run_update")
					p.checkOriginalExperiment(t)
					if fault == "quota" || fault == "replacement" {
						require.Contains(t, p.quota.ExptID2RunTime, f.expt)
					} else {
						require.NotContains(t, p.quota.ExptID2RunTime, f.expt)
					}
					if fault == "replacement" {
						require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: f.run + 1, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run + 1, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(mode))}).Error)
						require.NoError(t, m.exptRepo.Update(ctx, &entity.Experiment{ID: f.expt, LatestRunID: f.run + 1}))
						replacement := "evaluation"
						if mode == entity.EvaluationModeRetryItems {
							replacement = strconv.FormatInt(f.run+1, 10)
						}
						require.NoError(t, p.server.Set(m.makeExptMutexLockKey(f.expt), replacement))
						p.quota.ExptID2RunTime[f.expt] = time.Now().Unix() + 100
						before, calls := f.snapshot(), p.quotaCalls
						require.NoError(t, handler(ctx, p.event))
						requireHistoricalRunSnapshot(t, before, f.snapshot(), f.run, entity.ExptStatus_Failed, [5]int64{0, 2, 0, 0, 0}, "evaluation set item 999 does not exist")
						require.Equal(t, 2, startCalls, "historical replay still reaches the original setup failure")
						require.Equal(t, calls, p.quotaCalls, "A cannot clear B's space quota")
						require.Contains(t, p.quota.ExptID2RunTime, f.expt)
						value, getErr := p.server.Get(m.makeExptMutexLockKey(f.expt))
						require.NoError(t, getErr)
						require.Equal(t, replacement, value, "A cannot clear B's evaluation or numeric admission")
						return
					}
					p.quotaFault, runs.fault = nil, nil
					err = handler(ctx, p.event)
					require.Equal(t, 2, startCalls, "Pending must reach the actual ExptStart failure again")
				}
				require.NoError(t, err)
				require.NotContains(t, p.quota.ExptID2RunTime, f.expt)
				p.checkAdmission(t, false)
				logged, err := m.runLogRepo.Get(ctx, f.expt, f.run)
				require.NoError(t, err)
				require.Equal(t, int64(entity.ExptStatus_Failed), logged.Status)
				p.checkOriginalExperiment(t)
				t.Logf("REAL_MYSQL_SETUP_COMPENSATION mode=%d fault=%s quota_releases=%d start_calls=%d original_experiment_unchanged=true", mode, fault, p.releases, startCalls)
			})
		}
	}
}

func TestSetupCompensationPreMQQuotaDenied(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			p := newSetupCompensationFixture(t, mode)
			p.quota.ExptID2RunTime[p.f.expt+99] = time.Now().Unix()
			_, err := p.prepare(context.Background())
			require.Error(t, err)
			require.Equal(t, 1, p.quotaCalls, "pre-MQ denial must not invoke ReleaseExptRun")
			require.Equal(t, 0, p.releases)
			require.Contains(t, p.quota.ExptID2RunTime, p.f.expt+99)
			p.checkAdmission(t, false)
			p.checkOriginalExperiment(t)
			logged, err := p.m.runLogRepo.Get(context.Background(), p.f.expt, p.f.run)
			require.NoError(t, err)
			require.Equal(t, int64(entity.ExptStatus_Failed), logged.Status)
		})
	}
}

type setupCompensationDetailReadFault struct {
	repo.IExperimentRepo
	fault error
}

func (r *setupCompensationDetailReadFault) MGetByID(context.Context, []int64, int64) ([]*entity.Experiment, error) {
	return nil, r.fault
}

func TestSetupCompensationPreMQReadFailure(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			p := newSetupCompensationFixture(t, mode)
			failure := errors.New("pre-MQ experiment detail unavailable")
			p.m.exptRepo = &setupCompensationDetailReadFault{IExperimentRepo: p.m.exptRepo, fault: failure}
			_, err := p.prepare(context.Background())
			require.ErrorIs(t, err, failure)
			require.Zero(t, p.quotaCalls, "detail failure happens before quota admission and must not release a space slot")
			p.checkAdmission(t, false)
			p.checkOriginalExperiment(t)
			logged, err := p.m.runLogRepo.Get(context.Background(), p.f.expt, p.f.run)
			require.NoError(t, err)
			require.Equal(t, int64(entity.ExptStatus_Failed), logged.Status)
		})
	}
}
