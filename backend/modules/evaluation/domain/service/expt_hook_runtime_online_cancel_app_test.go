// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func OnlineAppCancelChainForTest(t *testing.T, build func(RetryItemsAppTestEnvironment, ExptResultService) OnlineAppTestDriver, scenario string) {
	t.Helper()
	f, old, _, _, _, oldEvent := onlineRuntimeFixture(t, true, true)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	require.NoError(t, old.Manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "", oldEvent.Session))
	items := onlineAppDataset{new(boundInitDataset)}
	deps, pub, _ := executionChainDependencies(t, f, nil, repo.HookBoundRuntimeRepositories{}, nil, entity.EvalTargetTypeLoopPrompt)
	pub.ExptEventPublisher = f.manager.publisher
	f.manager.publisher = pub
	cfg := retryItemsAppConfig{new(executionChainConfig)}
	f.manager.configer = cfg
	evaluators := &onlineChainEvaluators{executionChainEvaluators: &executionChainEvaluators{f: f}, t: t}
	consumer := deps.Consumer.(*ExptItemEventEvalServiceImpl)
	consumer.evaluationSetItemService, consumer.evaluatorService = items, evaluators
	scheduler := deps.Scheduler.(*ExptSchedulerImpl)
	scheduler.ExptTurnResultRepo = retryItemsIndexTurns{scheduler.ExptTurnResultRepo}
	scheduler.ResultSvc.(*ExptResultServiceImpl).ExptTurnResultRepo = scheduler.ExptTurnResultRepo
	scheduler.schedulerModeFactory.(*DefaultSchedulerModeFactory).resultSvc = scheduler.ResultSvc
	environment := RetryItemsAppTestEnvironment{DB: f.p, Redis: f.redis, Codec: f.manager.hooks.Codec, Identity: f.manager.hooks.Identity, Manager: f.manager, Scheduler: scheduler, Consumer: consumer, IDs: f.manager.idgenerator, Config: cfg, Items: items, Versions: &loaderVersions{}, Sets: &loaderSets{value: &entity.EvaluationSet{ID: 71, SpaceID: f.space, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: f.space, EvaluationSetID: 71}}}}, Refs: exptinfra.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.p)), Turns: scheduler.ExptTurnResultRepo, Results: scheduler.ExptItemResultRepo, Experiments: f.manager.exptRepo, SpaceID: f.space, ExptID: f.expt}
	driver := build(environment, hookPersistenceFilters{scheduler.ResultSvc})
	if scenario == "no_hook_draft" {
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).UpdateColumns(map[string]any{"latest_run_id": 0, "status": int32(entity.ExptStatus_Pending), "lifecycle_hook_conf": nil}).Error)
	} else {
		runID, err := driver.Start(ctx)
		require.NoError(t, err)
		f.key.RunID = runID
		require.NoError(t, driver.Invoke(ctx, runID, 801))
		require.NoError(t, driver.Scheduler.Schedule(ctx, pub.published[len(pub.published)-1]))
		require.Empty(t, pub.items, "before waiting must not dispatch accepted items")
		run := finalizationRead(t, f)
		require.Equal(t, entity.HookGateWaiting, run.State.Gate)
		require.Equal(t, entity.ExptStatus_Pending, run.State.Status)
		require.Equal(t, "original-user", run.CreatedBy)
	}
	t.Logf("isolated cancel fixture space=%d experiment=%d run=%d", f.space, f.expt, f.key.RunID)
	denied := errors.New("cancel permission denied")
	switch scenario {
	case "wrong_run":
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).UpdateColumn("latest_run_id", f.key.RunID+100000).Error)
	case "corrupt_binding":
		require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("snapshot_cipher", []byte("corrupt")).Error)
	case "wrong_scope":
		require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("execution_scope", "other").Error)
	case "missing_marker":
		require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("space_id=? AND id=?", f.space, f.key.RunID).UpdateColumn("lifecycle_hook_version", 0).Error)
	case "offline_pending":
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).UpdateColumn("expt_type", int32(entity.ExptType_Offline)).Error)
	case "permission_denied":
		driver.SetAuthorizationError(denied)
	}
	before := readOnlineCancelState(t, f)
	err := driver.Kill(ctx)
	if scenario != "accepted" {
		require.Error(t, err)
		if scenario == "permission_denied" {
			require.ErrorIs(t, err, denied)
		}
		require.Equal(t, before, readOnlineCancelState(t, f), "rejected cancellation must not write state or launch legacy termination")
		require.Empty(t, pub.items)
		return
	}
	require.NoError(t, err, "actual App.KillExperiment must accept the verified Online waiting Run")
	run := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
	require.Equal(t, entity.ExptStatus_Terminated, run.State.Status)
	require.Equal(t, entity.HookGateClosed, run.State.Gate)
	require.True(t, run.State.After.Activated)
	require.Equal(t, "original-user", run.CreatedBy)
	require.Equal(t, before.Lifecycle.SnapshotHash, run.Snapshot.Hash)
	require.Error(t, driver.Invoke(ctx, f.key.RunID, 803), "cancellation must reject new admission")
	require.Empty(t, pub.items)
	require.Zero(t, evaluators.calls)
}

type onlineCancelState struct {
	Experiment model.Experiment
	Run        model.ExptRunLog
	Lifecycle  model.ExptLifecycleRun
	Operations []*model.ExptLifecycleHookRun
}

func readOnlineCancelState(t *testing.T, f *finalizationManagerFixture) onlineCancelState {
	t.Helper()
	var state onlineCancelState
	require.NoError(t, f.sql.Where("id=? AND space_id=?", f.expt, f.space).First(&state.Experiment).Error)
	require.NoError(t, f.sql.Where("id=? AND space_id=?", f.key.RunID, f.space).First(&state.Run).Error)
	require.NoError(t, f.sql.Where("expt_run_id=? AND space_id=?", f.key.RunID, f.space).First(&state.Lifecycle).Error)
	require.NoError(t, f.sql.Where("expt_run_id=? AND space_id=?", f.key.RunID, f.space).Order("id").Find(&state.Operations).Error)
	return state
}
