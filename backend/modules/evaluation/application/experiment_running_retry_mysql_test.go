// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	dexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/testutil"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type applicationPendingPort struct {
	repo.IExptManualRetryRepo
	repo.IExptManualRetryPendingRepo
	registrations  atomic.Int64
	afterRegister  func()
	registrationMu sync.Mutex
}

func (p *applicationPendingPort) RegisterPendingRetry(ctx context.Context, r *repo.ManualRetryPending) error {
	p.registrationMu.Lock()
	defer p.registrationMu.Unlock()
	p.registrations.Add(1)
	err := p.IExptManualRetryPendingRepo.RegisterPendingRetry(ctx, r)
	if err == nil && p.afterRegister != nil {
		p.afterRegister()
	}
	return err
}

func (p *applicationPendingPort) GetPendingRetry(ctx context.Context, space, expt, run int64) (*repo.ManualRetryPending, error) {
	p.registrationMu.Lock()
	defer p.registrationMu.Unlock()
	return p.IExptManualRetryPendingRepo.GetPendingRetry(ctx, space, expt, run)
}

// The fixture records legacy publication calls separately from SQL admission.
// Existing service tests exercise Run/RetryItems execution; this application
// fixture verifies the shared entry's choice and actual run/pending storage.
type legacyApplicationManager struct {
	service.IExptManager
	runs, retryItems atomic.Int64
}

func (m *legacyApplicationManager) Run(context.Context, int64, int64, int64, int, *entity.Session, entity.ExptRunMode, map[string]string) error {
	m.runs.Add(1)
	return nil
}

func (m *legacyApplicationManager) RetryItems(context.Context, int64, int64, int64, int, []int64, *entity.Session, map[string]string) error {
	m.retryItems.Add(1)
	return nil
}

type runningApplicationFixture struct {
	app                          *experimentApplication
	conn                         *gorm.DB
	cache                        infraredis.Cmdable
	port                         *applicationPendingPort
	manager                      *legacyApplicationManager
	base, space, expt, run, item int64
	consumerConf                 *entity.ExptConsumerConf
}

func newRunningApplicationFixture(t *testing.T) *runningApplicationFixture {
	t.Helper()
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("EXPT_RETRY_MYSQL_DSN not configured")
	}
	provider, err := db.NewDB(mysqlDriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(context.Background(), db.WithMaster())
	var identity struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version, DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)
	t.Logf("REAL_MYSQL_APPLICATION_EXECUTED version=%s schema=%s", identity.Version, identity.Schema)
	base := int64(870000000000000000) + time.Now().UnixNano()%1000000000000
	space, expt, run, item := base+1, base+2, base+3, base+11
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_run_log", "expt_stats"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", space, expt).Error)
		}
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", space, expt).Error)
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type,created_by,eval_conf,eval_set_id,eval_set_version_id) VALUES(?,?,?,?,?,?,?,?,?)", expt, space, run, entity.ExptStatus_Processing, entity.ExptType_Offline, "original", `{"item_retry_num":3,"item_concur_num":4}`, base+80, base+81).Error)
	raw, err := json.Marshal([]entity.ExptRunLogItems{{ItemIDs: []int64{item}}})
	require.NoError(t, err)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: run, SpaceID: space, ExptID: expt, ExptRunID: run, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), CreatedBy: "original", ItemIds: &raw, FailCnt: 1}).Error)
	require.NoError(t, conn.Create(&model.ExptStats{ID: base + 4, SpaceID: space, ExptID: expt, FailCnt: 1}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: base + 12, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: base + 91, Status: 3, Ext: gptr.Of([]byte(`{"business":"keep"}`))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: base + 13, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: base + 91, Status: 3, ResultState: gptr.Of(int32(1)), RetryTimes: 3}).Error)
	require.NoError(t, conn.Create(&model.ExptItemRef{ID: base + 14, SpaceID: space, ExptID: expt, ItemID: item, ItemVersionID: base + 91, ItemConfig: gptr.Of([]byte(`{}`))}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResult{ID: base + 15, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: base + 91, TurnID: base + 21, Status: 2, TargetResultID: 123, WeightedScore: gptr.Of(0.0)}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: base + 16, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: base + 91, TurnID: base + 21, Status: 2, TargetResultID: 123, EvaluatorResultIds: gptr.Of([]byte(`null`)), Ext: []byte(`{"other":"keep"}`)}).Error)
	ctrl := gomock.NewController(t)
	config := confmocks.NewMockIConfiger(ctrl)
	f := &runningApplicationFixture{consumerConf: &entity.ExptConsumerConf{}}
	require.NoError(t, json.Unmarshal([]byte(`{"retry_complete_enabled":true}`), f.consumerConf))
	config.EXPECT().GetConsumerConf(gomock.Any()).DoAndReturn(func(context.Context) *entity.ExptConsumerConf { return f.consumerConf }).AnyTimes()
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	generator := idmocks.NewMockIIDGenerator(ctrl)
	lwt := lwtmocks.NewMockILatestWriteTracker(ctrl)
	cache := testutil.RetryRedis(t)
	factory := func() lock.ILocker { return lock.NewRedisLocker(cache) }
	mutex := factory().WithHolder("evaluation")
	metric := metricmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).AnyTimes()
	generator.EXPECT().GenID(gomock.Any()).Return(base+10000, nil).AnyTimes()
	config.EXPECT().GetExptExecConf(gomock.Any(), space).Return(&entity.ExptExecConf{ZombieIntervalSecond: 300, ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 4, MaxItemConcurNum: 4}}).AnyTimes()
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	// 入口配置与真实 pending 消费共用既有 configer。
	lwt.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), expt).Return(false).AnyTimes()
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	actual := exptrepo.NewExptManualRetryRepoWithPending(provider, generator, cache)
	port := &applicationPendingPort{IExptManualRetryRepo: actual, IExptManualRetryPendingRepo: actual.(repo.IExptManualRetryPendingRepo)}
	manager := service.NewExptManager(testutil.DisplayResult{},
		exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), generator),
		exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(provider)), exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(provider)), exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(provider)), exptrepo.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(provider)), exptrepo.NewExptTurnResultRepo(generator, exptmysql.NewExptTurnResultDAO(provider), exptmysql.NewExptTurnEvaluatorResultRefDAO(provider)), config, nil, mutex, nil, publisher, nil, generator, metric, lwt, testutil.DisplayVersions{SetID: base + 80}, testutil.DisplaySet{}, nil, nil, nil, testutil.DisplayAggregate{}, nil, nil, nil, nil, nil, nil, nil, component.NewNoopCentralSchedulerScopeProvider(), component.NewNoopCentralAdmissionPolicy(), component.NewNoopCentralReservationGuard(), port, nil, component.NewNoopCurrentRunSchedulerWaker(), factory)
	legacy := &legacyApplicationManager{IExptManager: manager}
	app := &experimentApplication{manager: legacy, auth: auth, idgen: generator, configer: config}
	testutil.StartPendingTicks(t, manager, port, factory, &entity.ExptScheduleEvent{SpaceID: space, ExptID: expt, ExptRunID: run, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit, Session: &entity.Session{UserID: "entry-test"}})
	f.app, f.conn, f.cache, f.port, f.manager = app, conn, cache, port, legacy
	f.base, f.space, f.expt, f.run, f.item = base, space, expt, run, item
	return f
}

func TestRunningRetryMySQLApplicationAdmission(t *testing.T) {
	f := newRunningApplicationFixture(t)
	app, conn := f.app, f.conn
	space, expt, run, item, base := f.space, f.expt, f.run, f.item, f.base
	var raw []byte
	require.NoError(t, conn.Table("expt_run_log").Select("item_ids").Where("id=?", run).Row().Scan(&raw))
	req := &exptpb.RetryExperimentRequest{WorkspaceID: &space, ExptID: &expt, ExpectedRunID: &run, RetryMode: gptr.Of(dexpt.ExptRetryMode_RetryTargetItems), ItemIds: []int64{item, item}, Ext: map[string]string{"caller": "identity"}}
	response, err := app.RetryExperiment(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, int32(1), response.GetAcceptedCount())
	require.Equal(t, run, response.GetRunID())
	var canonical model.ExptItemResult
	require.NoError(t, conn.First(&canonical, "id=?", base+12).Error)
	require.Zero(t, canonical.Status)
	ext, err := entity.ParseExptItemResultExt(*canonical.Ext)
	require.NoError(t, err)
	require.Equal(t, "keep", ext["business"])
	require.NotContains(t, ext, "caller")
	manual, err := entity.ParseManualRetryContext(ext)
	require.NoError(t, err)
	_, uuidErr := uuid.Parse(manual.RequestID)
	require.NoError(t, uuidErr)
	var log model.ExptItemResultRunLog
	require.NoError(t, conn.First(&log, "id=?", base+13).Error)
	require.Zero(t, *log.ResultState)
	require.Equal(t, int32(3), log.RetryTimes)
	var runLog model.ExptRunLog
	require.NoError(t, conn.First(&runLog, "id=?", run).Error)
	require.JSONEq(t, string(raw), string(*runLog.ItemIds))
	again, err := app.RetryExperiment(context.Background(), req)
	require.NoError(t, err)
	require.Zero(t, again.GetAcceptedCount())
	require.Equal(t, "queueing", again.RejectedItems[0].GetReason())
	require.Equal(t, item, again.RejectedItems[0].GetItemID())
	require.Equal(t, run, again.GetRunID())
	// 开启时重复请求仍由 run / 队列状态决定。
	gate := confmocks.NewMockIConfiger(gomock.NewController(t))
	gate.EXPECT().GetConsumerConf(gomock.Any()).Return(f.consumerConf).AnyTimes()
	gate.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Times(0)
	app.configer = gate
	third, err := app.RetryExperiment(context.Background(), req)
	require.NoError(t, err)
	require.Zero(t, third.GetAcceptedCount())
	require.Equal(t, "queueing", third.RejectedItems[0].GetReason())
	require.NoError(t, conn.Model(&model.Experiment{}).Where("id=?", expt).Update("status", entity.ExptStatus_Success).Error)
	_, err = app.RetryExperiment(context.Background(), req)
	require.ErrorContains(t, err, "run_finished")
	req.Ext = map[string]string{"caller": "changed"}
	_, err = app.RetryExperiment(context.Background(), req)
	require.ErrorContains(t, err, "run_finished")
	require.NoError(t, conn.First(&runLog, "id=?", run).Error)
	require.JSONEq(t, string(raw), string(*runLog.ItemIds))
	t.Log("REAL_MYSQL_APPLICATION_RESULT_VERIFIED same-run reset, budget preserved, duplicate rejected, no persisted receipt")
}

func TestRetryCompletePendingSurvivesDisabledEntry(t *testing.T) {
	f := newRunningApplicationFixture(t)
	conf := f.consumerConf
	gate := confmocks.NewMockIConfiger(gomock.NewController(t))
	gate.EXPECT().GetConsumerConf(gomock.Any()).Return(conf).Times(1)
	f.app.configer = gate
	f.port.afterRegister = func() { conf.RetryCompleteEnabled = false }
	response, err := f.app.RetryExperiment(context.Background(), &exptpb.RetryExperimentRequest{WorkspaceID: &f.space, ExptID: &f.expt, ExpectedRunID: &f.run, RetryMode: gptr.Of(dexpt.ExptRetryMode_RetryTargetItems), ItemIds: []int64{f.item}})
	require.NoError(t, err)
	require.False(t, conf.RetryCompleteEnabled)
	require.Equal(t, int32(1), response.GetAcceptedCount())
	require.Equal(t, f.run, response.GetRunID())
	var row model.ExptItemResult
	require.NoError(t, f.conn.First(&row, "id=?", f.base+12).Error)
	require.Zero(t, row.Status)
	t.Log("TCC_DISABLED_AFTER_PENDING_REGISTER real original tick committed accepted request")
}

func TestRetryCompleteTCCEntryRealStorageMatrix(t *testing.T) {
	for _, configKind := range []string{"nil", "missing", "false", "true"} {
		for _, entry := range []string{"shared_rpc", "openapi"} {
			for _, scenario := range []string{"evaluation", "join", "terminal_items", "failure", "retry_all"} {
				for _, identity := range []string{"nil", "zero", "current", "wrong", "negative"} {
					t.Run(configKind+"/"+entry+"/"+scenario+"/"+identity, func(t *testing.T) {
						f := newRunningApplicationFixture(t)
						enabled := configKind == "true"
						f.consumerConf = nil
						if configKind != "nil" {
							f.consumerConf = &entity.ExptConsumerConf{}
							if configKind != "missing" {
								require.NoError(t, json.Unmarshal([]byte(`{"retry_complete_enabled":`+configKind+`}`), f.consumerConf))
							}
						}
						ctx := context.Background()
						var initialRunLog model.ExptRunLog
						require.NoError(t, f.conn.First(&initialRunLog, "id=?", f.run).Error)
						req := &exptpb.RetryExperimentRequest{WorkspaceID: &f.space, ExptID: &f.expt, RetryMode: gptr.Of(dexpt.ExptRetryMode_RetryTargetItems), ItemIds: []int64{f.item}}
						switch identity {
						case "zero":
							req.ExpectedRunID = gptr.Of(int64(0))
						case "current":
							req.ExpectedRunID = &f.run
						case "wrong":
							req.ExpectedRunID = gptr.Of(f.run + 1)
						case "negative":
							req.ExpectedRunID = gptr.Of(int64(-1))
						}
						mutexKey := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
						switch scenario {
						case "join":
							require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("mode", entity.EvaluationModeRetryItems).Error)
							req.ItemIds = []int64{f.item + 99}
							if !enabled {
								require.NoError(t, f.cache.Set(ctx, mutexKey, strconv.FormatInt(f.run, 10), time.Minute).Err())
							}
						case "terminal_items":
							require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("status", entity.ExptStatus_Success).Error)
							require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("status", entity.ExptStatus_Success).Error)
						case "failure":
							req.RetryMode = gptr.Of(dexpt.ExptRetryMode_RetryFailure)
							req.ItemIds = nil
							if identity == "nil" {
								req.RetryMode = nil
							}
						case "retry_all":
							req.RetryMode = gptr.Of(dexpt.ExptRetryMode_RetryAll)
						}
						if (!enabled && (scenario == "evaluation" || scenario == "failure")) || scenario == "retry_all" {
							require.NoError(t, f.cache.Set(ctx, mutexKey, "evaluation", time.Minute).Err())
						}
						var response *exptpb.RetryExperimentResponse
						var err error
						if entry == "shared_rpc" {
							response, err = f.app.RetryExperiment(ctx, req)
						} else {
							api := &EvalOpenAPIApplication{auth: f.app.auth, metric: &fakeOpenAPIMetric{}, experimentApp: f.app}
							mode := "retry_target_items"
							if scenario == "failure" {
								mode = "retry_failure"
							}
							if scenario == "retry_all" {
								mode = "retry_all"
							}
							result, callErr := api.RetryExperimentOApi(ctx, &openapi.RetryExperimentOApiRequest{WorkspaceID: &f.space, ExperimentID: &f.expt, RetryMode: &mode, ExpectedRunID: req.ExpectedRunID, ItemIds: req.ItemIds})
							err = callErr
							if result != nil {
								response = &exptpb.RetryExperimentResponse{RunID: result.Data.RunID, AcceptedCount: result.Data.AcceptedCount}
							}
						}
						newPath := enabled && scenario != "retry_all" && (identity != "nil" || scenario != "terminal_items")
						var wantCode int32
						if newPath {
							switch {
							case identity == "negative":
								wantCode = int32(errno.CommonInvalidParamCode)
							case identity == "wrong":
								wantCode = int32(errno.ManualRetryRunConflictCode)
							case scenario == "terminal_items":
								wantCode = int32(errno.ManualRetryRunFinishedCode)
							}
						} else if enabled && scenario == "retry_all" && identity != "nil" {
							wantCode = int32(errno.CommonInvalidParamCode)
						} else if scenario == "evaluation" || scenario == "failure" || scenario == "retry_all" {
							wantCode = int32(errno.ExperimentRunningExistedCode)
						}
						if wantCode != 0 {
							status, ok := errorx.FromStatusError(err)
							require.True(t, ok, "business error required: %v", err)
							require.Equal(t, wantCode, status.Code())
							require.Nil(t, response)
						} else {
							require.NoError(t, err)
							require.NotNil(t, response)
							if newPath {
								require.Equal(t, f.run, response.GetRunID())
								if scenario == "join" {
									require.Zero(t, response.GetAcceptedCount())
								} else {
									require.Equal(t, int32(1), response.GetAcceptedCount())
								}
							} else if scenario == "join" {
								require.Equal(t, f.run, response.GetRunID())
								require.Nil(t, response.AcceptedCount)
								var row model.ExptRunLog
								require.NoError(t, f.conn.First(&row, "id=?", f.run).Error)
								var chunks []entity.ExptRunLogItems
								require.NoError(t, json.Unmarshal(gptr.Indirect(row.ItemIds), &chunks))
								var ids []int64
								for _, chunk := range chunks {
									ids = append(ids, chunk.ItemIDs...)
								}
								require.Equal(t, []int64{f.item, f.item + 99}, ids, "legacy join appends the item absent from the old list")
								require.Zero(t, f.manager.retryItems.Load())
							} else {
								require.Equal(t, f.base+10000, response.GetRunID())
								require.Nil(t, response.AcceptedCount)
								var row model.ExptRunLog
								require.NoError(t, f.conn.First(&row, "id=?", response.GetRunID()).Error)
								require.Equal(t, int32(entity.EvaluationModeRetryItems), gptr.Indirect(row.Mode))
							}
						}
						accepted := newPath && wantCode == 0 && scenario != "join"
						var item model.ExptItemResult
						require.NoError(t, f.conn.First(&item, "id=?", f.base+12).Error)
						if accepted {
							require.Equal(t, int64(1), f.port.registrations.Load())
							require.Zero(t, item.Status)
							var runLog model.ExptRunLog
							require.NoError(t, f.conn.First(&runLog, "id=?", f.run).Error)
							require.JSONEq(t, string(gptr.Indirect(initialRunLog.ItemIds)), string(gptr.Indirect(runLog.ItemIds)))
						} else {
							require.Zero(t, f.port.registrations.Load())
							require.Equal(t, int32(entity.ItemRunState_Fail), item.Status)
							ext, parseErr := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
							require.NoError(t, parseErr)
							manual, parseErr := entity.ParseManualRetryContext(ext)
							require.NoError(t, parseErr)
							require.Nil(t, manual)
							rawRedis, ok := infraredis.Unwrap(f.cache)
							require.True(t, ok)
							keys, keyErr := rawRedis.Keys(ctx, fmt.Sprintf(`\[experiment\]pending_manual_retry:%d:%d:*`, f.space, f.expt)).Result()
							require.NoError(t, keyErr)
							require.Empty(t, keys)
						}
						t.Logf("TCC_ENTRY_REAL_STORAGE config=%s entry=%s scenario=%s expected=%s registrations=%d", configKind, entry, scenario, identity, f.port.registrations.Load())
					})
				}
			}
		}
	}
}

func TestRetryEntryRealStorageStageSelection(t *testing.T) {
	for _, entry := range []string{"shared_rpc", "openapi"} {
		for _, kind := range []string{"items_evaluation_holder", "failure_evaluation_holder", "default_evaluation_holder", "items_join", "items_new_run", "failure_new_run", "retry_all_new_run", "explicit_current", "explicit_zero", "explicit_wrong_run", "explicit_terminal", "explicit_negative", "explicit_retry_all"} {
			t.Run(entry+"/"+kind, func(t *testing.T) {
				f := newRunningApplicationFixture(t)
				if !strings.HasPrefix(kind, "explicit_") {
					f.consumerConf = &entity.ExptConsumerConf{}
				}
				ctx := context.Background()
				req := &exptpb.RetryExperimentRequest{WorkspaceID: &f.space, ExptID: &f.expt, RetryMode: gptr.Of(dexpt.ExptRetryMode_RetryTargetItems), ItemIds: []int64{f.item}}
				mutexKey := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
				switch kind {
				case "items_evaluation_holder", "failure_evaluation_holder", "default_evaluation_holder":
					require.NoError(t, f.cache.Set(ctx, mutexKey, "evaluation", time.Minute).Err())
					if kind == "failure_evaluation_holder" {
						req.RetryMode = gptr.Of(dexpt.ExptRetryMode_RetryFailure)
					}
					if kind == "default_evaluation_holder" {
						req.RetryMode = nil
					}
				case "items_join":
					require.NoError(t, f.cache.Set(ctx, mutexKey, strconv.FormatInt(f.run, 10), time.Minute).Err())
					require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("mode", entity.EvaluationModeRetryItems).Error)
					req.ItemIds = []int64{f.item + 99}
				case "items_new_run", "failure_new_run", "retry_all_new_run":
					require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("status", entity.ExptStatus_Success).Error)
					require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("status", entity.ExptStatus_Success).Error)
					if kind == "failure_new_run" {
						req.RetryMode = gptr.Of(dexpt.ExptRetryMode_RetryFailure)
					}
					if kind == "retry_all_new_run" {
						req.RetryMode = gptr.Of(dexpt.ExptRetryMode_RetryAll)
					}
				default:
					req.ExpectedRunID = &f.run
					switch kind {
					case "explicit_zero":
						req.ExpectedRunID = gptr.Of(int64(0))
					case "explicit_wrong_run":
						req.ExpectedRunID = gptr.Of(f.run + 1)
					case "explicit_terminal":
						require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("status", entity.ExptStatus_Success).Error)
					case "explicit_negative":
						req.ExpectedRunID = gptr.Of(int64(-1))
					case "explicit_retry_all":
						req.RetryMode = gptr.Of(dexpt.ExptRetryMode_RetryAll)
					}
				}
				var response *exptpb.RetryExperimentResponse
				var err error
				if entry == "shared_rpc" {
					response, err = f.app.RetryExperiment(ctx, req)
				} else {
					api := &EvalOpenAPIApplication{auth: f.app.auth, metric: &fakeOpenAPIMetric{}, experimentApp: f.app}
					request := &openapi.RetryExperimentOApiRequest{WorkspaceID: &f.space, ExperimentID: &f.expt, ExpectedRunID: req.ExpectedRunID, ItemIds: req.ItemIds}
					mode := "retry_target_items"
					if req.GetRetryMode() == dexpt.ExptRetryMode_RetryFailure || req.RetryMode == nil {
						mode = "retry_failure"
					}
					if req.GetRetryMode() == dexpt.ExptRetryMode_RetryAll {
						mode = "retry_all"
					}
					request.RetryMode = &mode
					result, callErr := api.RetryExperimentOApi(ctx, request)
					err = callErr
					if result != nil {
						response = &exptpb.RetryExperimentResponse{RunID: result.Data.RunID, AcceptedCount: result.Data.AcceptedCount}
					}
				}
				switch kind {
				case "items_evaluation_holder", "failure_evaluation_holder", "default_evaluation_holder":
					status, ok := errorx.FromStatusError(err)
					require.True(t, ok)
					require.Equal(t, int32(errno.ExperimentRunningExistedCode), status.Code())
					require.Nil(t, response)
					value, readErr := f.cache.Get(ctx, mutexKey).Result()
					require.NoError(t, readErr)
					require.Equal(t, "evaluation", value)
				case "items_join":
					require.NoError(t, err)
					require.Equal(t, f.run, response.GetRunID())
					require.Nil(t, response.AcceptedCount)
					var row model.ExptRunLog
					require.NoError(t, f.conn.First(&row, "id=?", f.run).Error)
					var chunks []entity.ExptRunLogItems
					decodeErr := json.Unmarshal(gptr.Indirect(row.ItemIds), &chunks)
					require.NoError(t, decodeErr)
					var ids []int64
					for _, chunk := range chunks {
						ids = append(ids, chunk.ItemIDs...)
					}
					require.Contains(t, ids, f.item+99, "legacy join must append the requested item to the real run")
					require.Zero(t, f.manager.retryItems.Load(), "joining the live RetryItems run must not reinitialize it")
				case "items_new_run", "failure_new_run", "retry_all_new_run":
					require.NoError(t, err)
					require.Equal(t, f.base+10000, response.GetRunID())
					require.Nil(t, response.AcceptedCount)
					var row model.ExptRunLog
					require.NoError(t, f.conn.First(&row, "id=?", response.GetRunID()).Error)
					expected := entity.EvaluationModeRetryItems
					if kind == "failure_new_run" {
						expected = entity.EvaluationModeFailRetry
					}
					if kind == "retry_all_new_run" {
						expected = entity.EvaluationModeRetryAll
					}
					require.Equal(t, int32(expected), gptr.Indirect(row.Mode))
					if kind == "items_new_run" {
						require.Equal(t, int64(1), f.manager.retryItems.Load())
					} else {
						require.Equal(t, int64(1), f.manager.runs.Load())
					}
				case "explicit_current", "explicit_zero":
					require.NoError(t, err)
					require.Equal(t, f.run, response.GetRunID())
					require.Equal(t, int32(1), response.GetAcceptedCount())
					require.Equal(t, int64(1), f.port.registrations.Load())
				default:
					status, ok := errorx.FromStatusError(err)
					require.True(t, ok)
					code := errno.CommonInvalidParamCode
					if kind == "explicit_wrong_run" {
						code = errno.ManualRetryRunConflictCode
					}
					if kind == "explicit_terminal" {
						code = errno.ManualRetryRunFinishedCode
					}
					require.Equal(t, int32(code), status.Code())
					require.Nil(t, response)
				}
				var item model.ExptItemResult
				require.NoError(t, f.conn.First(&item, "id=?", f.base+12).Error)
				ext, decodeErr := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
				require.NoError(t, decodeErr)
				if kind == "explicit_current" || kind == "explicit_zero" {
					manual, parseErr := entity.ParseManualRetryContext(ext)
					require.NoError(t, parseErr)
					require.NotNil(t, manual)
					require.NotEmpty(t, manual.RequestID)
					require.Equal(t, f.run, manual.RunID)
				} else {
					require.Zero(t, f.port.registrations.Load(), "legacy/rejected requests must never register pending")
					manual, parseErr := entity.ParseManualRetryContext(ext)
					require.NoError(t, parseErr)
					require.Nil(t, manual)
					require.Equal(t, int32(entity.ItemRunState_Fail), item.Status, "entry choice must preserve the existing canonical item")
					rawRedis, ok := infraredis.Unwrap(f.cache)
					require.True(t, ok)
					keys, keyErr := rawRedis.Keys(ctx, fmt.Sprintf(`\[experiment\]pending_manual_retry:%d:%d:*`, f.space, f.expt)).Result()
					require.NoError(t, keyErr)
					require.Empty(t, keys, "neither intent nor result key may be written")
				}
				t.Logf("EXPLICIT_ENTRY_REAL_STORAGE entry=%s case=%s registrations=%d run=%d err=%v", entry, kind, f.port.registrations.Load(), response.GetRunID(), err)
			})
		}
	}
}
