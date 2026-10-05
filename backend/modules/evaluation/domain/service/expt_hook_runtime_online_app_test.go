// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/stretchr/testify/require"
)

type onlineAppDataset struct{ *boundInitDataset }

func (d onlineAppDataset) BatchCreateEvaluationSetItems(_ context.Context, in *entity.BatchCreateEvaluationSetItemsParam) (map[int64]int64, []*entity.ItemErrorGroup, []*entity.DatasetItemOutput, error) {
	ids := map[int64]int64{}
	for i, item := range in.Items {
		ids[int64(i)] = item.ItemID
	}
	return ids, nil, nil, nil
}
func (d onlineAppDataset) BatchGetEvaluationSetItems(ctx context.Context, in *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	items, err := d.boundInitDataset.BatchGetEvaluationSetItems(ctx, in)
	for _, item := range items {
		item.BaseInfo = &entity.BaseInfo{}
	}
	return items, err
}

type OnlineAppTestDriver struct {
	Start                 func(context.Context) (int64, error)
	Invoke                func(context.Context, int64, int64) error
	Finish                func(context.Context, int64) error
	Kill                  func(context.Context) error
	SetAuthorizationError func(error)
	Scheduler             ExptSchedulerEvent
	Consumer              ExptItemEvalEvent
}

func OnlineAppChainForTest(t *testing.T, build func(RetryItemsAppTestEnvironment, ExptResultService) OnlineAppTestDriver, recoverWithoutWriteKey ...bool) {
	f, old, _, _, _, oldEvent := onlineRuntimeFixture(t, true, true)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	// A completed prior Run permits proving a fresh Run through the actual application.
	require.NoError(t, old.Manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "", oldEvent.Session))
	items := onlineAppDataset{new(boundInitDataset)}
	deps, pub, _ := executionChainDependencies(t, f, nil, repo.HookBoundRuntimeRepositories{}, nil, entity.EvalTargetTypeLoopPrompt)
	pub.ExptEventPublisher = f.manager.publisher
	f.manager.publisher = pub
	cfg := retryItemsAppConfig{new(executionChainConfig)}
	f.manager.configer = cfg
	evaluators := &onlineChainEvaluators{executionChainEvaluators: &executionChainEvaluators{f: f}, t: t}
	consumer := deps.Consumer.(*ExptItemEventEvalServiceImpl)
	consumer.evaluationSetItemService = items
	consumer.evaluatorService = evaluators
	scheduler := deps.Scheduler.(*ExptSchedulerImpl)
	scheduler.ExptTurnResultRepo = retryItemsIndexTurns{scheduler.ExptTurnResultRepo}
	scheduler.ResultSvc.(*ExptResultServiceImpl).ExptTurnResultRepo = scheduler.ExptTurnResultRepo
	scheduler.schedulerModeFactory.(*DefaultSchedulerModeFactory).resultSvc = scheduler.ResultSvc
	environment := RetryItemsAppTestEnvironment{DB: f.p, Redis: f.redis, Codec: f.manager.hooks.Codec, Identity: f.manager.hooks.Identity, Manager: f.manager, Scheduler: scheduler, Consumer: consumer, IDs: f.manager.idgenerator, Config: cfg, Items: items, Versions: &loaderVersions{}, Sets: &loaderSets{value: &entity.EvaluationSet{ID: 71, SpaceID: f.space, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: f.space, EvaluationSetID: 71}}}}, Refs: exptinfra.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.p)), Turns: scheduler.ExptTurnResultRepo, Results: scheduler.ExptItemResultRepo, Experiments: f.manager.exptRepo, SpaceID: f.space, ExptID: f.expt}
	driver := build(environment, hookPersistenceFilters{scheduler.ResultSvc})
	runID, err := driver.Start(ctx)
	require.NoError(t, err)
	require.NotEqual(t, f.key.RunID, runID)
	f.key.RunID = runID
	require.NotEmpty(t, pub.published)
	event := pub.published[len(pub.published)-1]
	require.Equal(t, runID, event.ExptRunID)
	require.Equal(t, "original-user", event.Session.UserID)
	require.NoError(t, driver.Invoke(ctx, runID, 801))
	if len(recoverWithoutWriteKey) > 0 && recoverWithoutWriteKey[0] {
		manager := *f.manager
		manager.hooks = nil
		environment.Manager = &manager
		driver = build(environment, hookPersistenceFilters{scheduler.ResultSvc})
		_, err = driver.Start(ctx)
		require.Error(t, err, "missing write key must still forbid a new Hook Run")
	}
	require.NoError(t, driver.Scheduler.Schedule(ctx, event))
	require.Empty(t, pub.items)
	completeExecutionChainBefore(t, f)
	require.NoError(t, driver.Finish(ctx, runID))
	require.Error(t, driver.Invoke(ctx, runID, 803))
	require.NoError(t, driver.Scheduler.Schedule(ctx, event))
	require.Len(t, pub.items, 1)
	require.NoError(t, driver.Consumer.Eval(ctx, pub.items[0]))
	require.NoError(t, driver.Scheduler.Schedule(ctx, event))
	run := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
	require.True(t, run.State.After.Activated)
	require.Equal(t, "original-user", run.CreatedBy)
	require.Equal(t, 1, evaluators.calls)
}
