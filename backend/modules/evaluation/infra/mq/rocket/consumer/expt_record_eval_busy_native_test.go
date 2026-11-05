// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

type busyNativeManager struct {
	service.IExptManager
	runs   repo.IExptRunLogRepo
	detail *entity.Experiment
}

func (m *busyNativeManager) GetRunLog(ctx context.Context, expt, run, _ int64, _ *entity.Session) (*entity.ExptRunLog, error) {
	return m.runs.Get(ctx, expt, run)
}

func (m *busyNativeManager) GetDetail(context.Context, int64, int64, *entity.Session, ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	return m.detail, nil
}

// The reservation fixture executes Redis CAS with the original token, including
// empty tokens. No busy outcome can reach either the confirm or release write.
type busyNativeGuard struct {
	component.ICentralReservationGuard
	client    infraredis.Cmdable
	key       string
	scope     string
	run, item int64
}

func (g *busyNativeGuard) ConfirmRunning(ctx context.Context, scope string, run, item int64, request string) (bool, error) {
	if scope != g.scope || run != g.run || item != g.item {
		return false, fmt.Errorf("unexpected reservation identity")
	}
	value, err := g.client.Eval(ctx, "local v=redis.call('GET',KEYS[1]); if v and v==ARGV[1] then return 1 end; return 0", []string{g.key}, request).Int()
	return value == 1, err
}

func (g *busyNativeGuard) Release(ctx context.Context, scope string, run, item int64, request, _ string) error {
	if scope != g.scope || run != g.run || item != g.item {
		return fmt.Errorf("unexpected release identity")
	}
	return g.client.Eval(ctx, "if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end; return 0", []string{g.key}, request).Err()
}

func TestItemBusyNativeConsumerKeepsPendingAndConvergesDuplicates(t *testing.T) {
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("EXPT_RETRY_MYSQL_DSN missing; REAL MYSQL NOT EXECUTED")
	}
	ctx := context.Background()
	provider, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(ctx, db.WithMaster())
	var database struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&database).Error)
	require.Equal(t, "expt_running_retry_test", database.Schema)
	t.Logf("REAL_MYSQL_EXECUTED version=%s schema=%s", database.Version, database.Schema)
	base := int64(820000000000000000) + time.Now().UnixNano()%1000000000000
	space, expt, run, item, version := base+1, base+2, base+3, base+4, base+5
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_run_log", "expt_stats"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", space, expt).Error)
		}
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", space, expt).Error)
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, conn.Create(&model.Experiment{ID: expt, SpaceID: space, LatestRunID: run, ExptType: int32(entity.ExptType_Offline), Status: int32(entity.ExptStatus_Processing), ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "busy-native"}).Error)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: run, SpaceID: space, ExptID: expt, ExptRunID: run, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit))}).Error)
	require.NoError(t, conn.Create(&model.ExptStats{ID: base + 6, SpaceID: space, ExptID: expt, ProcessingCnt: 1}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: base + 7, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: version, Status: int32(entity.ItemRunState_Processing)}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: base + 8, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: version, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	for i := int64(0); i < 2; i++ {
		require.NoError(t, conn.Create(&model.ExptTurnResult{ID: base + 10 + i, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: version, TurnID: i, Status: int32(entity.TurnRunState_Processing)}).Error)
		require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: base + 20 + i, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: 0, TurnID: i, Status: int32(entity.TurnRunState_Processing)}).Error)
	}
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	guard := &busyNativeGuard{client: client, key: "reservation:busy-native", scope: "busy-native", run: run, item: item}
	require.NoError(t, server.Set(guard.key, ""))
	ctrl := gomock.NewController(t)
	config := configmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{}).AnyTimes()
	config.EXPECT().GetExptExecConf(gomock.Any(), space).Return(&entity.ExptExecConf{}).AnyTimes()
	config.EXPECT().GetErrRetryConf(gomock.Any(), space, gomock.Any()).Return(&entity.RetryConf{}).AnyTimes()
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), gomock.Any()).AnyTimes()
	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(unblock)
	reads := 0
	dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
		reads++

		close(entered)
		<-resume
		return []*entity.EvaluationSetItem{{ItemID: item, ItemVersionID: gptr.Of(version), Turns: []*entity.Turn{{ID: 0}, {ID: 1}}}}, nil
	}).Times(1)
	detail := &entity.Experiment{ID: expt, SpaceID: space, LatestRunID: run, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing, EvalSet: &entity.EvaluationSet{EvaluationSetVersion: &entity.EvaluationSetVersion{ID: base + 30, EvaluationSetID: base + 31}}, EvalConf: &entity.EvaluationConfiguration{ConnectorConf: &entity.Connector{EvaluatorsConf: &entity.EvaluatorsConf{}}}}
	runs := exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(provider))
	items := exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(provider))
	turns := exptrepo.NewExptTurnResultRepo(nil, exptmysql.NewExptTurnResultDAO(provider), exptmysql.NewExptTurnEvaluatorResultRefDAO(provider))
	manager := &busyNativeManager{runs: runs, detail: detail}
	svc := service.NewExptRecordEvalService(manager, config, nil, items, turns, exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(provider)), exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), nil), nil, nil, lock.NewRedisLocker(client), idem, nil, metric, nil, nil, dataset, nil, nil, nil, nil, nil, nil, nil, guard, nil, nil)
	consumer := NewExptRecordEvalConsumer(svc)
	deliver := func(message *mq.MessageExt) error { return consumer.HandleMessage(ctx, message) }
	event := &entity.ExptItemEvalEvent{SpaceID: space, ExptID: expt, ExptRunID: run, EvalSetItemID: item, ExptRunMode: entity.EvaluationModeSubmit, CreateAt: time.Now().Unix(), Session: &entity.Session{UserID: "busy-native"}}
	encode := func(event *entity.ExptItemEvalEvent) *mq.MessageExt {
		body, err := json.Marshal(event)
		require.NoError(t, err)
		return &mq.MessageExt{Message: mq.Message{Body: body}}
	}
	message := encode(event)
	lockKey := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", expt, item)
	for _, owner := range []string{"filter-holder", "termination-holder", "publication-holder"} {
		require.NoError(t, server.Set(lockKey, owner))
		for i := 0; i < 2; i++ {
			require.True(t, errno.NeedMQRetry(deliver(message)), "non-execution owner cannot take over a pending delivery")
		}
		stale := event.Clone()
		stale.RequestID = "obsolete"
		require.NoError(t, deliver(encode(stale)), "native source proves the obsolete delivery has no remaining work")
		require.Zero(t, reads)
		require.True(t, server.Exists(guard.key))
		server.Del(lockKey)
	}
	result := make(chan error, 1)
	go func() { result <- deliver(message) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the retained delivery did not reach the real evaluation chain")
	}
	require.True(t, errno.NeedMQRetry(deliver(message)), "a concurrent pending duplicate is retained while the worker executes")
	unblock()
	require.NoError(t, <-result)
	var log model.ExptItemResultRunLog
	require.NoError(t, conn.Where("space_id=? AND item_id=?", space, item).Take(&log).Error)
	require.Equal(t, int32(entity.ItemRunState_Success), log.Status)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(log.ResultState))
	var stored []model.ExptTurnResultRunLog
	require.NoError(t, conn.Where("space_id=? AND item_id=?", space, item).Order("turn_id").Find(&stored).Error)
	require.Len(t, stored, 2)
	for _, turn := range stored {
		require.Equal(t, int32(entity.TurnRunState_Success), turn.Status)
		require.Zero(t, turn.ItemVersionID, "ordinary legacy0 storage remains compatible")
	}
	require.False(t, server.Exists(guard.key), "real terminal completion releases only the original empty-token reservation")
	require.NoError(t, server.Set(lockKey, "publication-holder"))
	require.NoError(t, server.Set(guard.key, "rq2"))
	for i := 0; i < 3; i++ {
		require.NoError(t, deliver(message), "validated completed duplicate converges even under publication contention")
	}
	quota, err := server.Get(guard.key)
	require.NoError(t, err)
	require.Equal(t, "rq2", quota, "completed old delivery cannot release another request's reservation")
	finish, err := exptrepo.NewExptRunFinishRepo(provider).FinishCurrentRun(ctx, &entity.ExptRunFinishRequest{SpaceID: space, ExptID: expt, RunID: run})
	require.NoError(t, err)
	require.Equal(t, entity.ExptRunFinishPending, finish.Disposition, "Logged work must publish before the run can finish")
	t.Log("REAL_MYSQL_REAL_CONSUMER_EVALTURNS_REDIS_CAS_EXECUTED; broker replay is explicitly driven in-process")
}
