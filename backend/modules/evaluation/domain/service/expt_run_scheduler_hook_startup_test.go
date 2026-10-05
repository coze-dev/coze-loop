// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	"github.com/stretchr/testify/require"
)

var errStartupScanReached = errors.New("startup scan reached")

type startupDataset struct {
	EvaluationSetItemService
	item  *entity.EvaluationSetItem
	calls int
}

func (s *startupDataset) ListEvaluationSetItems(context.Context, *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
	s.calls++
	return []*entity.EvaluationSetItem{s.item}, gptr.Of(int64(1)), gptr.Of(int64(1)), nil, nil
}

type startupDetailManager struct {
	IExptManager
	expt *entity.Experiment
}

func (m startupDetailManager) GetDetail(context.Context, int64, int64, *entity.Session, ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	return m.expt, nil
}

// Only stop at the next stage; ExptStart/ScheduleStart are the actual default implementations.
type startupObservedMode struct {
	entity.ExptSchedulerMode
	scans *int
}

func (m startupObservedMode) ScanEvalItems(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) ([]*entity.ExptEvalItem, []*entity.ExptEvalItem, []*entity.ExptEvalItem, error) {
	*m.scans++
	return nil, nil, nil, errStartupScanReached
}

type startupDefaultFactory struct {
	SchedulerModeFactory
	afterConstruct func()
	scans          int
	mode           entity.ExptSchedulerMode
}

func (f *startupDefaultFactory) NewSchedulerMode(mode entity.ExptRunMode) (entity.ExptSchedulerMode, error) {
	m, err := f.SchedulerModeFactory.NewSchedulerMode(mode)
	if err != nil {
		return nil, err
	}
	f.mode = m
	if f.afterConstruct != nil {
		f.afterConstruct()
	}
	return startupObservedMode{m, &f.scans}, nil
}

func startupFixture(t *testing.T, mode entity.ExptRunMode) (*finalizationManagerFixture, entity.HookExecutionManifest, *ExptSchedulerImpl, *startupDefaultFactory, *startupDataset, *entity.ExptScheduleEvent) {
	t.Helper()
	f, m, s := zeroTimeoutFixture(t)
	s.ResultSvc = hookPersistenceFilters{s.ResultSvc}
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("mode", int32(mode)).Error)
	expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: f.key.RunID, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing, TrialRunItemCount: 1, EvalSet: &entity.EvaluationSet{ID: 71, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 71, EvaluationSetID: 71}}}
	item := &entity.EvaluationSetItem{ItemID: m.Frozen.ItemID, ItemVersionID: gptr.Of(m.Frozen.ItemVersionID)}
	for _, mt := range m.Turns {
		item.Turns = append(item.Turns, &entity.Turn{ID: mt.TurnID})
	}
	data := &startupDataset{item: item}
	idem := idemrepo.NewIdempotentService(idemredis.NewIdemDAO(f.redis))
	exptRepo := exptinfra.NewExptRepo(exptmysql.NewExptDAO(f.p), exptmysql.NewExptEvaluatorRefDAO(f.p), activeReferenceIDs{})
	factory := &startupDefaultFactory{SchedulerModeFactory: NewSchedulerModeFactory(f.manager, s.ExptItemResultRepo, s.ExptStatsRepo, s.ExptTurnResultRepo, activeReferenceIDs{}, data, exptRepo, nil, idem, s.Configer, s.Publisher, nil, hookPersistenceFilters{}, nil, nil, nil)}
	s.schedulerModeFactory = factory
	s.Manager = startupDetailManager{IExptManager: f.manager, expt: expt}
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: mode}
	exists, err := idem.Exist(context.Background(), makeStartIdemKey(event))
	require.NoError(t, err)
	require.False(t, exists, "real Redis start key must be absent")
	return f, m, s, factory, data, event
}

func TestHookStartupDefaultIdemMissDoesNotReset(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, action := range []string{"active", "cancel", "new-latest"} {
			t.Run(fmt.Sprintf("%d/%s", mode, action), func(t *testing.T) {
				f, m, s, factory, data, event := startupFixture(t, mode)
				var before hookPersistenceSnapshot
				var exptBefore model.Experiment
				factory.afterConstruct = func() {
					if mode == entity.EvaluationModeSubmit {
						require.IsType(t, &ExptSubmitExec{}, factory.mode)
					} else {
						require.IsType(t, &ExptTrialRunExec{}, factory.mode)
					}
					if action == "cancel" {
						require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "startup race", nil))
					}
					if action == "new-latest" {
						next := finalizationTestIDs.Add(1)
						_, err := f.repo.CreateRunWithHooks(context.Background(), entity.HookCreateRunInput{Key: entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next}, ExpectedLatestRunID: f.key.RunID, RunLog: &entity.ExptRunLog{ID: next, ExptRunID: next, SpaceID: f.space, ExptID: f.expt, Mode: 1, Status: 3, CreatedBy: "user"}, Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", KeyID: "key", ExecutionScope: "local"}, After: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)}})
						require.NoError(t, err)
					}
					before = hookPersistenceRead(t, f, m)
					require.NoError(t, f.sql.First(&exptBefore, f.expt).Error)
				}
				err := s.schedule(context.Background(), event)
				if err != nil {
					require.ErrorIs(t, err, errStartupScanReached)
				}
				require.Equal(t, before, hookPersistenceRead(t, f, m), "legacy startup must not reset committed initialization/counts")
				var exptAfter model.Experiment
				require.NoError(t, f.sql.First(&exptAfter, f.expt).Error)
				require.Equal(t, exptBefore, exptAfter, "startup must not revive cancelled or newer Run status")
				require.Zero(t, data.calls, "frozen initialization must not re-read mutable source")
				if action == "active" {
					require.Equal(t, 1, factory.scans, "legitimate schedule duties must remain")
				}
			})
		}
	}
}

type startupReceiptBarrier struct {
	repo.IHookExecutionInitializationRepo
	after func()
}

func (p startupReceiptBarrier) ReadExecutionInitializationPage(ctx context.Context, in entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
	v, err := p.IHookExecutionInitializationRepo.ReadExecutionInitializationPage(ctx, in)
	if err == nil {
		p.after()
	}
	return v, err
}

func TestHookStartupCancellationAfterCommittedReceipt(t *testing.T) {
	f, m, s, factory, data, event := startupFixture(t, entity.EvaluationModeSubmit)
	var before hookPersistenceSnapshot
	s.hookInitialization = startupReceiptBarrier{IHookExecutionInitializationRepo: s.hookInitialization, after: func() {
		require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "after receipt", nil))
		before = hookPersistenceRead(t, f, m)
	}}
	require.NoError(t, s.schedule(context.Background(), event))
	require.Zero(t, factory.scans)
	require.Zero(t, data.calls)
	require.Equal(t, before, hookPersistenceRead(t, f, m))
}

type startupGateBarrier struct {
	repo.IHookGateRepo
	after func()
}

func (g startupGateBarrier) CanDispatch(ctx context.Context, key entity.HookRunKey) (entity.HookAdmissionDecision, error) {
	v, err := g.IHookGateRepo.CanDispatch(ctx, key)
	if err == nil {
		g.after()
	}
	return v, err
}

func TestHookStartupRequiresCommittedReceipt(t *testing.T) {
	f, m, s, factory, data, event := startupFixture(t, entity.EvaluationModeSubmit)
	s.hookGate = startupGateBarrier{IHookGateRepo: s.hookGate, after: func() {
		require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("execution_initialized", false).Error)
	}}
	before := hookPersistenceRead(t, f, m)
	require.Error(t, s.schedule(context.Background(), event))
	require.Zero(t, factory.scans)
	require.Zero(t, data.calls)
	require.Equal(t, before, hookPersistenceRead(t, f, m))
}

func TestHookStartupLegacyMarkerKeepsDefaultStart(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f, m, s, factory, data, event := startupFixture(t, mode)
			require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("lifecycle_hook_version", 0).Error)
			for _, table := range []any{&model.ExptLifecycleHookRun{}, &model.ExptLifecycleRunItem{}, &model.ExptLifecycleRun{}} {
				require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(table).Error)
			}
			source, err := s.hookScheduler.ReadFinalizationSource(context.Background(), f.key)
			require.NoError(t, err)
			require.False(t, source.Managed)
			require.ErrorIs(t, s.schedule(context.Background(), event), errStartupScanReached)
			require.Equal(t, 1, data.calls)
			require.Equal(t, 1, factory.scans)
			require.Equal(t, int32(1), hookPersistenceRead(t, f, m).Stats.PendingCnt, "legacy startup behavior is unchanged")
		})
	}
}
