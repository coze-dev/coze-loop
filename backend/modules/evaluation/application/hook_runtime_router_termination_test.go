// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	red "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	infraHook "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
)

type routerTerminationFixture struct {
	t         *testing.T
	p         db.Provider
	sql       *gorm.DB
	runs      repo.IHookRepo
	codec     hook.StorageCodec
	router    *hookRuntimeRouter
	app       *experimentApplication
	auth      *rpcm.MockIAuthProvider
	key       entity.HookRunKey
	before    bool
	publisher *em.MockExptEventPublisher
}

func newRouterTerminationFixture(t *testing.T, mode entity.ExptRunMode, before ...bool) *routerTerminationFixture {
	t.Helper()
	p := db.NewTestDB(t, &model.Experiment{}, &model.ExptRunLog{}, &model.ExptStats{},
		&model.ExptLifecycleRun{}, &model.ExptLifecycleHookRun{}, &model.ExptLifecycleHookAttempt{}, &model.ExptLifecycleRunItem{},
		&model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptEvaluatorRef{}, &model.ExptItemRef{}, &model.ExptTurnEvaluatorResultRef{})
	require.NotNil(t, p)
	pool, err := p.NewSession(context.Background()).DB()
	require.NoError(t, err)
	// The in-memory MySQL engine does not synchronize its multi-connection catalog.
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	c, err := redis.NewClient(&red.Options{Addr: miniredis.RunT(t).Addr()})
	require.NoError(t, err)
	if client, ok := redis.Unwrap(c); ok {
		t.Cleanup(func() { require.NoError(t, client.Close()) })
	}
	f := &routerTerminationFixture{t: t, p: p, sql: p.NewSession(context.Background()), runs: exptrepo.NewHookRunRepo(p), codec: infraHook.NewStorageCodec(wiringLeaseProtector{}), key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}}
	f.before = len(before) > 0 && before[0]
	ctrl := gomock.NewController(t)
	config := cm.NewMockIConfiger(ctrl)
	config.EXPECT().GetMaintainerUserIDs(gomock.Any()).Return(map[string]bool{}).AnyTimes()
	f.auth = rpcm.NewMockIAuthProvider(ctrl)
	metric := mm.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	pub := em.NewMockExptEventPublisher(ctrl)
	f.publisher = pub
	pub.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	pub.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	experiments := exptrepo.NewExptRepo(exptmysql.NewExptDAO(p), exptmysql.NewExptEvaluatorRefDAO(p), nil)
	logs := exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(p))
	items := exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(p))
	ids := struct{ idgen.IIDGenerator }{}
	turns := exptrepo.NewExptTurnResultRepo(ids, exptmysql.NewExptTurnResultDAO(p), exptmysql.NewExptTurnEvaluatorResultRefDAO(p))
	stats := exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(p))
	locker := lock.NewRedisLocker(c)
	quota := exptrepo.NewQuotaService(dao.NewQuotaDAO(c), locker)
	aggr := sm.NewMockExptAggrResultService(ctrl)
	records := sm.NewMockEvaluatorRecordService(ctrl)
	dataset := sm.NewMockEvaluationSetItemService(ctrl)
	result := service.NewExptResultService(items, turns, nil, stats, experiments, metric, nil, ids, nil, nil, nil, nil, nil, nil, records, dataset, pub, nil, nil, nil, service.NewEvaluatorScoreCalculator(nil, nil))
	idem := idemrepo.NewIdempotentService(idemredis.NewIdemDAO(c))
	base := service.NewExptManager(result, experiments, logs, stats, items, nil, turns, config, quota, locker, idem, pub, nil, ids, metric, platestwrite.NewLatestWriteTracker(c), sm.NewMockEvaluationSetVersionService(ctrl), nil, nil, nil, nil, aggr, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if mode == entity.EvaluationModeAppend {
		base, err = service.NewExptManagerWithHooks(base, service.ExptManagerHookDependencies{Initialization: exptrepo.NewHookRunInitializationRepo(p), Runs: f.runs, Configs: exptrepo.NewHookConfigRepo(p, f.codec), Codec: f.codec, Identity: struct{ hook.IdentityProvider }{}, Runtime: struct{ hook.RuntimeConfigProvider }{}, Wake: struct{ hook.WakePublisher }{}, ExecutionScope: "termination-test", SnapshotKeyID: "test-key"})
		require.NoError(t, err)
	}
	manager, err := service.NewExptManagerWithHookFinalization(base, service.ExptManagerFinalizationDependencies{Runs: f.runs, Repository: exptrepo.NewHookFinalizationRepo(p), Owners: exptrepo.NewHookFinalizationOwnerReader(c), ExecutionScope: "termination-test", NewItemLocker: func() lock.ILocker { return lock.NewRedisLocker(c) }})
	require.NoError(t, err)
	modes := service.NewSchedulerModeFactory(manager, items, stats, turns, ids, dataset, experiments, nil, idem, config, pub, records, result, nil, logs, locker)
	scheduler := service.NewExptSchedulerSvc(manager, experiments, items, turns, nil, stats, logs, idem, config, quota, locker, pub, nil, metric, result, ids, dataset, modes, nil, nil, nil, nil, nil)
	consumer := service.NewExptRecordEvalService(manager, config, pub, items, turns, stats, experiments, nil, quota, locker, idem, nil, metric, result, nil, dataset, records, nil, ids, benefit.NoopBenefitServiceImpl{}, exptrepo.NewEvalAsyncRepo(dao.NewEvalAsyncDAO(c, nil)), nil, nil, nil, nil, nil)
	factory, err := NewHookRuntimeExecutionFactory(HookRuntimeExecutionFactoryDependencies{DB: p, Runs: f.runs, Codec: f.codec, Scope: "termination-test", Manager: manager, Scheduler: scheduler, Consumer: consumer, Loader: struct{ hook.PlanPageLoader }{}, IDs: ids})
	require.NoError(t, err)
	f.router = &hookRuntimeRouter{IExptManager: manager, source: exptrepo.NewHookFinalizationRepo(p), runs: f.runs, codec: f.codec, scope: "termination-test", factory: factory, admissionInstalled: true}
	f.app = &experimentApplication{manager: f.router, configer: config, auth: f.auth}
	require.NoError(t, f.sql.Create(&model.Experiment{ID: 20, SpaceID: 10, Name: "termination", ExptType: int32(entity.ExptType_Offline), Status: int32(entity.ExptStatus_Processing), EvalSetSourceType: int32(entity.ExptEvalSetSourceType_MultiSetConfig), CreatedBy: "owner"}).Error)
	require.NoError(t, f.sql.Create(&model.ExptStats{ID: 40, SpaceID: 10, ExptID: 20}).Error)
	if entity.HookBoundRetryMode(mode) {
		require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: 29, SpaceID: 10, ExptID: 20, ExptRunID: 29, Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), Status: gptr.Of(int64(entity.ExptStatus_Failed)), CreatedBy: "owner"}).Error)
	}
	if mode != 0 {
		f.createRun(mode, 30, 0)
	}
	return f
}

func (f *routerTerminationFixture) createRun(mode entity.ExptRunMode, id, previous int64) {
	f.t.Helper()
	key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: id}
	name := map[entity.ExptRunMode]string{entity.EvaluationModeSubmit: "submit", entity.EvaluationModeFailRetry: "fail_retry", entity.EvaluationModeRetryAll: "retry_all", entity.EvaluationModeRetryItems: "retry_items", entity.EvaluationModeAppend: "append"}[mode]
	typ := "offline"
	if mode == entity.EvaluationModeAppend {
		typ = "online"
		require.NoError(f.t, f.sql.Model(&model.Experiment{}).Where("id=?", 20).UpdateColumns(map[string]any{"expt_type": int32(entity.ExptType_Online), "eval_set_source_type": int32(entity.ExptEvalSetSourceType_SingleSet)}).Error)
	}
	conf := &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.test/after")}}}
	if f.before {
		conf.Before = &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.test/before")}}
	}
	snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "termination-test", CreatedAt: time.Now(),
		Config:    conf,
		Context:   &spi.HookRunContext{WorkspaceID: gptr.Of("10"), ExperimentID: gptr.Of("20"), RunID: gptr.Of(strconv.FormatInt(id, 10)), RunMode: &name, Initiator: &spi.HookInitiator{UserID: gptr.Of("owner"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("termination"), Type: &typ}, EvalSets: []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("10"), ID: gptr.Of("71"), VersionID: gptr.Of("72")}}},
		Execution: &entity.HookExecutionSnapshot{Version: 1, Key: key, ExecutionScope: "termination-test", Mode: mode, SingleSet: mode == entity.EvaluationModeAppend, EvaluatorFallback: &entity.HookExecutionEvaluatorFallback{}, Sets: []entity.HookExecutionSet{{EvalSetID: 71, EvalSetVersionID: 72, ItemConfig: &entity.ExptItemConfig{}}}}})
	require.NoError(f.t, err)
	protected, err := f.codec.EncodeSnapshot(context.Background(), "test-key", snapshot)
	require.NoError(f.t, err)
	in := entity.HookCreateRunInput{Key: key, ExpectedLatestRunID: previous, RunLog: &entity.ExptRunLog{ID: id, ExptRunID: id, ExptID: 20, SpaceID: 10, Mode: int32(mode), Status: int64(entity.ExptStatus_Processing), CreatedBy: "owner"}, Snapshot: protected, After: &entity.HookOperationSeed{ID: id + 100, OperationID: "after-" + strconv.FormatInt(id, 10), IdempotencyKey: "after-" + strconv.FormatInt(id, 10)}}
	if entity.HookBoundRetryMode(mode) {
		in.SourceRunID = gptr.Of(int64(29))
	}
	if f.before {
		in.Before = &entity.HookOperationSeed{ID: id + 200, OperationID: "before-" + strconv.FormatInt(id, 10), IdempotencyKey: "before-" + strconv.FormatInt(id, 10)}
	}
	_, err = f.runs.CreateRunWithHooks(context.Background(), in)
	require.NoError(f.t, err)
	require.NoError(f.t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", 10, id).UpdateColumns(map[string]any{"plan_state": 1, "plan_hash": entity.NewHookPlanDigest().Hash, "execution_initialized": true, "execution_started": true}).Error)
	if f.before {
		require.NoError(f.t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("id=?", id+200).UpdateColumn("status", string(entity.HookOperationSucceeded)).Error)
		require.NoError(f.t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", 10, id).UpdateColumn("gate", 1).Error)
	}
	if mode == entity.EvaluationModeAppend {
		require.NoError(f.t, f.sql.Model(&model.Experiment{}).Where("id=?", 20).UpdateColumn("status", int32(entity.ExptStatus_Processing)).Error)
		require.NoError(f.t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", id).UpdateColumn("status", int64(entity.ExptStatus_Processing)).Error)
	}
}

func (f *routerTerminationFixture) allow() {
	f.auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
		require.Equal(f.t, int64(10), p.SpaceID)
		require.Equal(f.t, "20", p.ObjectID)
		require.Equal(f.t, "owner", gptr.Indirect(p.OwnerID))
		return nil
	})
}

func (f *routerTerminationFixture) read() *entity.HookStoredRun {
	f.t.Helper()
	run, err := f.runs.GetRun(context.Background(), f.key)
	require.NoError(f.t, err)
	return run
}

// Seed a dispatched item with no turn logs yet, the production lazy-turn boundary.
func (f *routerTerminationFixture) materializeItem() {
	t := f.t
	frozen := entity.HookPlanItem{ID: 1000, SourceSpaceID: 10, EvalSetID: 71, EvalSetVersionID: 72, ItemID: 801}
	config, err := json.Marshal(&entity.ExptItemConfig{})
	require.NoError(t, err)
	sum := sha256.Sum256(config)
	manifest := entity.HookExecutionManifest{Version: 1, Key: f.key, Frozen: frozen, ItemResultID: 1001, ItemRunLogID: 1002,
		Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: 1003}}, ItemRef: &entity.HookExecutionItemRef{ID: 1004, ConfigHash: hex.EncodeToString(sum[:])}, TurnLogsInitialized: gptr.Of(false)}
	require.NoError(t, manifest.Validate())
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, f.sql.Create(&model.ExptLifecycleRunItem{ID: 1000, SpaceID: 10, ExptID: 20, ExptRunID: 30, SourceSpaceID: 10, EvalSetID: 71, EvalSetVersionID: 72, ItemID: 801, ExecutionManifest: &raw}).Error)
	require.NoError(t, f.sql.Create(&model.ExptItemRef{ID: 1004, SpaceID: 10, ExptID: 20, ItemID: 801, EvalSetID: 71, EvalSetVersionID: 72, ItemConfig: &config}).Error)
	require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: 1001, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 801, ItemIdx: gptr.Of(int32(0)), Status: int32(entity.ItemRunState_Queueing)}).Error)
	require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: 1002, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 801, Status: int32(entity.ItemRunState_Queueing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: 1003, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 801, TurnIdx: gptr.Of(int32(0)), Status: int32(entity.TurnRunState_Queueing)}).Error)
	digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), []entity.HookPlanItem{frozen})
	require.NoError(t, err)
	fields := map[string]any{"plan_count": 1, "plan_hash": digest.Hash}
	if f.read().Mode == entity.EvaluationModeRetryItems {
		// One published batch and one accepted but unpublished tail stay on this Run.
		batches := []entity.ExptRunLogItems{{ItemIDs: []int64{801}, CreateAt: gptr.Of(int64(1))}, {ItemIDs: []int64{802}, CreateAt: gptr.Of(int64(2))}}
		rawBatches, err := json.Marshal(batches)
		require.NoError(t, err)
		require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", 30).UpdateColumn("item_ids", rawBatches).Error)
		cursor, err := (entity.HookRetryItemsCursor{Version: 2, Key: f.key, Fingerprint: hex.EncodeToString(sum[:]), Phase: "tail", Batch: 1, Published: digest}).Encode()
		require.NoError(t, err)
		fields["plan_cursor"] = cursor
	}
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", 10, 30).UpdateColumns(fields).Error)
}

func (f *routerTerminationFixture) waitCommitted() {
	f.t.Helper()
	require.Eventually(f.t, func() bool {
		var life model.ExptLifecycleRun
		err := f.sql.Where("space_id=? AND expt_run_id=?", 10, 30).First(&life).Error
		return err == nil && life.FinalizeState == 2
	}, 5*time.Second, 20*time.Millisecond)
}

// Removing the SetExptTerminating route must fail here, before asynchronous completion.
func TestHookRuntimeRouterKillExperimentTermination(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mode   entity.ExptRunMode
		before bool
	}{
		{"submit", entity.EvaluationModeSubmit, false}, {"retry_all", entity.EvaluationModeRetryAll, false},
		{"failure", entity.EvaluationModeFailRetry, false}, {"target_items", entity.EvaluationModeRetryItems, false},
		{"retry_all_before_succeeded", entity.EvaluationModeRetryAll, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRouterTerminationFixture(t, tc.mode, tc.before)
			f.materializeItem()
			f.allow()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			response, err := f.app.KillExperiment(ctx, &expt.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
			require.NoError(t, err)
			require.NotNil(t, response)
			accepted := f.read()
			require.Equal(t, entity.HookGateClosed, accepted.State.Gate)
			require.Equal(t, entity.ExptStatus_Terminated, accepted.State.Intent.Status)
			_, err = f.app.KillExperiment(ctx, &expt.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
			require.Error(t, err, "the public entry keeps its existing non-running rejection after acceptance")
			require.NoError(t, f.router.SetExptTerminating(ctx, 20, 30, 10, nil))
			require.Equal(t, accepted.Version, f.read().Version)
			f.waitCommitted()
			done := f.read()
			require.Equal(t, entity.ExptStatus_Terminated, done.State.Status)
			require.True(t, done.State.After.Activated)
			afterOperations := 0
			for _, operation := range done.Operations {
				if operation.Phase == entity.HookPhaseAfter {
					afterOperations++
				}
			}
			require.Equal(t, 1, afterOperations)
			if tc.before {
				require.Equal(t, entity.HookOperationSucceeded, done.State.Before.Status)
			}
			require.NoError(t, f.router.SetExptTerminating(ctx, 20, 30, 10, nil))
			require.Equal(t, done, f.read())
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, 1002).Error)
			require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
			require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
			gate, err := exptrepo.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "termination-test", nil }).CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateClosed, gate.Gate)
			if tc.mode == entity.EvaluationModeRetryItems {
				var log model.ExptRunLog
				require.NoError(t, f.sql.First(&log, 30).Error)
				var batches []entity.ExptRunLogItems
				require.NoError(t, json.Unmarshal(gptr.Indirect(log.ItemIds), &batches))
				cursor, err := entity.DecodeHookRetryItemsCursor(done.PlanCursor, f.key, batches, done.PlanCount, done.PlanHash)
				require.NoError(t, err)
				require.Equal(t, "closed", cursor.Phase)
				require.Equal(t, 2, cursor.Terminal.AcceptedBatches)
				require.Equal(t, 1, cursor.Terminal.Batch)
				require.Equal(t, int64(1), done.PlanCount)
			}
		})
	}
}

func TestHookRuntimeRouterKillExperimentKeepsOriginalRun(t *testing.T) {
	f := newRouterTerminationFixture(t, entity.EvaluationModeRetryAll)
	var successor *entity.HookStoredRun
	var projection model.Experiment
	f.auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *rpc.AuthorizationWithoutSPIParam) error {
		f.createRun(entity.EvaluationModeRetryAll, 31, 30)
		var err error
		successor, err = f.runs.GetRun(context.Background(), entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 31})
		require.NoError(t, err)
		require.NoError(t, f.sql.First(&projection, 20).Error)
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := f.app.KillExperiment(ctx, &expt.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
	require.NoError(t, err)
	f.waitCommitted()
	got, err := f.runs.GetRun(ctx, successor.State.Key)
	require.NoError(t, err)
	require.Equal(t, successor, got)
	var after model.Experiment
	require.NoError(t, f.sql.First(&after, 20).Error)
	require.Equal(t, projection, after)
	require.Equal(t, entity.ExptStatus_Terminated, f.read().State.Status)
}

func TestHookRuntimeRouterKillExperimentSafetyChecks(t *testing.T) {
	for _, tc := range []string{"workspace", "offline_pending", "finished", "deleted", "invalid_binding"} {
		t.Run(tc, func(t *testing.T) {
			f := newRouterTerminationFixture(t, entity.EvaluationModeRetryAll)
			space := int64(10)
			switch tc {
			case "workspace":
				space = 11
			case "offline_pending":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", 20).UpdateColumn("status", int32(entity.ExptStatus_Pending)).Error)
			case "finished":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", 20).UpdateColumn("status", int32(entity.ExptStatus_Success)).Error)
			case "deleted":
				require.NoError(t, f.sql.Delete(&model.Experiment{}, 20).Error)
			case "invalid_binding":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", 10, 30).UpdateColumn("source_run_id", 30).Error)
				f.allow()
			}
			before := f.read()
			_, err := f.app.KillExperiment(context.Background(), &expt.KillExperimentRequest{WorkspaceID: &space, ExptID: gptr.Of(int64(20))})
			require.Error(t, err)
			require.Equal(t, before, f.read())
		})
	}
}

func TestHookRuntimeRouterKillExperimentPermissionDenied(t *testing.T) {
	f := newRouterTerminationFixture(t, entity.EvaluationModeRetryAll)
	denied := errors.New("permission denied")
	f.auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(denied)
	before := f.read()
	response, err := f.app.KillExperiment(context.Background(), &expt.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
	require.ErrorIs(t, err, denied)
	require.Nil(t, response)
	require.Equal(t, before, f.read())
}

func TestHookRuntimeRouterKillExperimentOnlineProcessing(t *testing.T) {
	f := newRouterTerminationFixture(t, entity.EvaluationModeAppend)
	f.allow()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := f.app.KillExperiment(ctx, &expt.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
	require.NoError(t, err)
	f.waitCommitted()
	require.Equal(t, entity.ExptStatus_Terminated, f.read().State.Status)
	require.True(t, f.read().State.After.Activated)
}

func TestHookRuntimeRouterKillExperimentLegacy(t *testing.T) {
	f := newRouterTerminationFixture(t, 0)
	f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
		require.Equal(t, int64(30), event.ExptRunID)
		require.Equal(t, int64(20), event.ExptID)
		require.Equal(t, int64(10), event.SpaceID)
		return nil
	})
	require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: 30, SpaceID: 10, ExptID: 20, ExptRunID: 30, Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), Status: gptr.Of(int64(entity.ExptStatus_Processing)), CreatedBy: "owner"}).Error)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", 20).UpdateColumns(map[string]any{"latest_run_id": 30, "eval_set_source_type": int32(entity.ExptEvalSetSourceType_SingleSet)}).Error)
	// Legacy must not need a factory, storage codec, or a Hook snapshot.
	f.router.factory, f.router.codec = nil, nil
	f.allow()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := f.app.KillExperiment(ctx, &expt.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		var experiment model.Experiment
		return f.sql.First(&experiment, 20).Error == nil && experiment.Status == int32(entity.ExptStatus_Terminated)
	}, 5*time.Second, 20*time.Millisecond)
	var run model.ExptRunLog
	require.NoError(t, f.sql.First(&run, 30).Error)
	require.Equal(t, int64(entity.ExptStatus_Terminated), gptr.Indirect(run.Status))
	for _, table := range []any{&model.ExptLifecycleRun{}, &model.ExptLifecycleHookRun{}} {
		var count int64
		require.NoError(t, f.sql.Model(table).Count(&count).Error)
		require.Zero(t, count)
	}
}

func TestHookRuntimeRouterTerminationRequiresExplicitRun(t *testing.T) {
	f := newRouterTerminationFixture(t, entity.EvaluationModeRetryAll)
	before := f.read()
	for _, runID := range []int64{0, -1} {
		require.ErrorIs(t, f.router.SetExptTerminating(context.Background(), 20, runID, 10, nil), entity.ErrHookStoreCorrupt)
		require.Equal(t, before, f.read())
	}
}
