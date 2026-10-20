// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var lateWriteFixtureSeq atomic.Int64

type lateWriteFixture struct {
	t                                     *testing.T
	provider                              db.Provider
	conn                                  *gorm.DB
	port                                  repo.IExptCurrentResultRepo
	base, space, expt, run, item, version int64
}

func newLateWriteFixture(t *testing.T, token string) *lateWriteFixture {
	t.Helper()
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("EXPT_RETRY_MYSQL_DSN missing; REAL MYSQL NOT EXECUTED")
	}
	provider, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(context.Background(), db.WithMaster())
	var identity struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)
	t.Logf("REAL_MYSQL_EXECUTED version=%s schema=%s", identity.Version, identity.Schema)
	base := int64(7384603622000000000) + time.Now().UnixNano()%1000000000000 + lateWriteFixtureSeq.Add(1)*10000
	f := &lateWriteFixture{t: t, provider: provider, conn: conn, base: base, space: base + 1, expt: base + 2, run: base + 3, item: base + 11, version: base + 31}
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_run_log", "expt_stats"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
		}
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", f.space, f.expt).Error)
		for _, table := range []string{"eval_target_record", "evaluator_record"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=?", f.space).Error)
		}
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, conn.Exec("INSERT INTO experiment (id,space_id,latest_run_id,status,expt_type) VALUES (?,?,?,?,?)", f.expt, f.space, f.run, entity.ExptStatus_Processing, entity.ExptType_Offline).Error)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: f.run, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), ItemIds: gptr.Of([]byte(`[{"ItemIDs":[],"CreateAt":1700000000}]`))}).Error)
	require.NoError(t, conn.Create(&model.ExptStats{ID: base + 4, SpaceID: f.space, ExptID: f.expt, ProcessingCnt: 2, PendingCnt: 1}).Error)
	for index, status := range []entity.ItemRunState{entity.ItemRunState_Processing, entity.ItemRunState_Processing, entity.ItemRunState_Queueing} {
		item := f.item + int64(index)
		require.NoError(t, conn.Create(&model.ExptItemResult{ID: base + 100 + int64(index), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: f.version, Status: int32(status), Ext: gptr.Of([]byte(`{"other":"keep"}`))}).Error)
		require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: base + 200 + int64(index), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: f.version, Status: int32(status), RetryTimes: 1, ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
		require.NoError(t, conn.Create(&model.ExptItemRef{ID: base + 300 + int64(index), SpaceID: f.space, ExptID: f.expt, ItemID: item, ItemVersionID: f.version}).Error)
	}
	f.token(token)
	refs, _ := json.Marshal(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: base + 51, Alias: "alias", RecordID: base + 61}}, Inline: []*entity.InlineEvalResult{{InlineKey: "inline", RecordID: base + 62}}})
	for index, turn := range []int64{0, base + 22} {
		require.NoError(t, conn.Create(&model.ExptTurnResult{ID: base + 400 + int64(index), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: turn, Status: int32(entity.TurnRunState_Processing), TargetResultID: base + 41, WeightedScore: gptr.Of(0.0)}).Error)
		require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: base + 500 + int64(index), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: turn, Status: int32(entity.TurnRunState_Processing), TargetResultID: base + 41, EvaluatorResultIds: &refs, Ext: []byte(`{"other":"keep"}`)}).Error)
	}
	require.NoError(t, conn.Create(&model.ExptTurnEvaluatorResultRef{ID: base + 600, SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: base + 400, EvaluatorVersionID: base + 51, Alias_: "alias", EvaluatorResultID: base + 61}).Error)
	require.NoError(t, conn.Exec("INSERT INTO eval_target_record (id,space_id,target_id,target_version_id,experiment_run_id,item_id,item_version_id,turn_id,log_id,trace_id,status) VALUES (?,?,?,?,?,?,?,?,?,?,?)", base+41, f.space, base+42, base+43, f.run, f.item, f.version, 0, "target", "trace", 1).Error)
	require.NoError(t, conn.Exec("INSERT INTO evaluator_record (id,space_id,evaluator_version_id,experiment_id,experiment_run_id,item_id,item_version_id,turn_id,trace_id,status,target_record_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)", base+61, f.space, base+51, f.expt, f.run, f.item, f.version, 0, "trace", 1, 0).Error)
	port, ok := any(experimentrepo.NewExptItemResultRepo(experimentmysql.NewExptItemResultDAO(provider))).(repo.IExptCurrentResultRepo)
	require.True(t, ok, "source conditional execution transaction missing")
	f.port = port
	return f
}
func (f *lateWriteFixture) token(token string) {
	f.t.Helper()
	ext := map[string]string{"other": "keep"}
	if token != "" {
		raw, err := json.Marshal(&entity.ManualRetryContext{RunID: f.run, RequestID: token, Mode: entity.EvaluationModeFailRetry, Operator: "test", AcceptedAtMS: 1})
		require.NoError(f.t, err)
		ext[entity.ManualRetryExtKey] = string(raw)
	}
	raw, err := json.Marshal(ext)
	require.NoError(f.t, err)
	require.NoError(f.t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Update("ext", raw).Error)
}
func (f *lateWriteFixture) source() *entity.ExptCurrentItemRunSource {
	f.t.Helper()
	s, err := f.port.GetCurrentItemRunSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(f.t, err)
	require.NotNil(f.t, s)
	return s
}
func (f *lateWriteFixture) snapshot() map[string]string {
	f.t.Helper()
	out := map[string]string{}
	for _, table := range []string{"experiment", "expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_turn_evaluator_result_ref", "expt_stats", "expt_run_log", "expt_item_ref", "eval_target_record", "evaluator_record"} {
		var rows []map[string]any
		require.NoError(f.t, f.conn.Table(table).Where("space_id=?", f.space).Order("id").Find(&rows).Error)
		raw, err := json.Marshal(rows)
		require.NoError(f.t, err)
		out[table] = string(raw)
	}
	return out
}
func (f *lateWriteFixture) result(source *entity.ExptCurrentItemRunSource) *entity.ExptTurnResultRunLog {
	r := *source.Turns[0]
	r.Status = entity.TurnRunState_Success
	r.TargetResultID = f.base + 71
	r.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 81}}}
	return &r
}

func (f *lateWriteFixture) itemRepo() repo.IExptItemResultRepo {
	return f.port.(repo.IExptItemResultRepo)
}
func (f *lateWriteFixture) turnRepo() repo.IExptTurnResultRepo {
	return experimentrepo.NewExptTurnResultRepo(nil, experimentmysql.NewExptTurnResultDAO(f.provider), experimentmysql.NewExptTurnEvaluatorResultRefDAO(f.provider))
}
func (f *lateWriteFixture) event() *entity.ExptItemEvalEvent {
	return &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, EvalSetItemID: f.item, ExptRunMode: entity.EvaluationModeFailRetry, RequestID: "rq1", MaxRetryTimes: 3}
}
func lateWriteLocker(t *testing.T, ctrl *gomock.Controller) ManualRetryLockerFactory {
	t.Helper()
	return func() lock.ILocker {
		mutex := lockmocks.NewMockILocker(ctrl)
		mutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), 5*time.Second, gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
			child, cancel := context.WithCancel(ctx)
			return true, child, cancel, nil
		}).AnyTimes()
		mutex.EXPECT().Unlock(gomock.Any()).Return(true, nil).AnyTimes()
		return mutex
	}
}

func TestLateWriteMySQLStartThenAdmission(t *testing.T) {
	for _, path := range []string{"store", "complete", "yield", "error", "zombie", "sandbox", "cancel_store", "cancel_complete", "source_read_error"} {
		t.Run(path, func(t *testing.T) {
			f := newLateWriteFixture(t, "rq1")
			source := f.source()
			event := f.event()
			event.RetryTimes = 1
			ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: source})
			ctrl := gomock.NewController(t)
			config := componentmocks.NewMockIConfiger(ctrl)
			executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), TurnResultRepo: f.turnRepo(), Configer: config}
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), configer: config, itemLocker: lateWriteLocker(t, ctrl)}
			if path == "source_read_error" {
				before := f.snapshot()
				fault := errors.New("source DB unavailable")
				hook := "late_write_primary_read_failure"
				require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register(hook, func(tx *gorm.DB) { _ = tx.AddError(fault) }))
				err := svc.HandleEventErr(func(ctx context.Context, _ *entity.ExptItemEvalEvent) error {
					executionSource(ctx).Source = source
					return fault
				})(context.Background(), event)
				require.NoError(t, f.conn.Callback().Query().Remove(hook))
				require.True(t, errno.NeedMQRetry(err))
				require.Equal(t, before, f.snapshot())
				return
			}
			f.token("rq2")
			before := f.snapshot()
			if path == "cancel_store" || path == "cancel_complete" {
				child, cancel := context.WithCancel(ctx)
				cancel()
				ctx = child
			}
			switch path {
			case "store", "cancel_store":
				etec := &entity.ExptTurnEvalCtx{Turn: &entity.Turn{ID: 0}, ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: event, Expt: &entity.Experiment{ID: f.expt, SpaceID: f.space}, EvalSetItem: &entity.EvaluationSetItem{ItemID: f.item}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: source.Turns}}}
				err := executor.storeTurnRunResult(ctx, etec, &entity.ExptTurnRunResult{})
				if path == "store" {
					require.ErrorIs(t, err, errExptNonCurrentSource)
				} else {
					require.True(t, errno.NeedMQRetry(err))
				}
			case "complete", "cancel_complete":
				err := executor.CompleteItemRun(ctx, &entity.ExptItemEvalCtx{Event: event}, nil)
				if path == "complete" {
					require.ErrorIs(t, err, errExptNonCurrentSource)
				} else {
					require.True(t, errno.NeedMQRetry(err))
				}
			case "yield":
				svc.yieldItemRunForRetry(ctx, event, errors.New("old error"))
			case "error":
				require.NoError(t, svc.HandleEventErr(func(ctx context.Context, _ *entity.ExptItemEvalEvent) error {
					executionSource(ctx).Source = source
					return errors.New("old error")
				})(context.Background(), event))
			case "zombie", "sandbox":
				config.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ZombieSecond: 1}}}).AnyTimes()
				scheduler := &ExptSchedulerImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), itemLocker: lateWriteLocker(t, ctrl), Configer: config}
				old := time.Now().Add(-time.Hour)
				items := []*entity.ExptEvalItem{{Source: source, ItemID: f.item, ExptID: f.expt, State: entity.ItemRunState_Processing, UpdatedAt: &old, RequestID: "rq1"}}
				if path == "zombie" {
					_, completed, err := scheduler.handleZombies(ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}, items, nil)
					require.NoError(t, err)
					require.Empty(t, completed)
				} else {
					_, completed, err := scheduler.sweepCurrentSandboxItems(ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}, items, &entity.Experiment{})
					require.NoError(t, err)
					require.Empty(t, completed)
				}
			}
			require.Equal(t, before, f.snapshot(), "old processing source cannot mutate any of eleven real tables")
		})
	}
}

func TestLateWriteMySQLManualBudget(t *testing.T) {
	for _, spent := range []int32{1, 3} {
		for _, yield := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d_of_3_yield_%v", spent, yield), func(t *testing.T) {
				f := newLateWriteFixture(t, "rq1")
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("retry_times", spent).Error)
				ctrl := gomock.NewController(t)
				config := componentmocks.NewMockIConfiger(ctrl)
				publisher := eventmocks.NewMockExptEventPublisher(ctrl)
				config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{RetryTimes: 3}).AnyTimes()
				expectedPublish := int(3 - spent)
				if yield {
					expectedPublish = 0
				}
				published := 0
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptItemEvalEvent, _ *time.Duration, _ func(*entity.ExptItemEvalEvent)) error {
					published++
					require.Equal(t, int(spent)+published, event.RetryTimes)
					require.Equal(t, "rq1", event.RequestID)
					return nil
				}).Times(expectedPublish)
				svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), configer: config, publisher: publisher, itemLocker: lateWriteLocker(t, ctrl)}
				event := f.event()
				event.RetryTimes = 0
				if yield {
					event.Ext = map[string]string{entity.RetryYieldExtKey: "true"}
				}
				opportunities := 0
				for {
					source := f.source()
					if entity.IsItemRunFinished(source.Current.Status) {
						break
					}
					if source.Current.Status == entity.ItemRunState_Queueing {
						advanced, err := f.itemRepo().MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, f.item, "rq1")
						require.NoError(t, err)
						require.True(t, advanced)
					}
					opportunities++
					handler := svc.HandleEventErr(svc.HandleCurrentSource(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
						return errors.New("ordinary retryable provider error")
					}))
					require.NoError(t, handler(context.Background(), event))
					var log model.ExptItemResultRunLog
					require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&log).Error)
					require.GreaterOrEqual(t, log.RetryTimes, spent)
					require.LessOrEqual(t, log.RetryTimes, int32(3))
					event.RetryTimes = int(log.RetryTimes)
					require.LessOrEqual(t, opportunities, 3)
				}
				require.Equal(t, 1+int(3-spent), opportunities)
				require.Equal(t, expectedPublish, published)
				source := f.source()
				require.Equal(t, int32(3), source.Current.RetryTimes)
				require.Equal(t, entity.ItemRunState_Fail, source.Current.Status)
				var b, c model.ExptItemResult
				require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item+1).Take(&b).Error)
				require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item+2).Take(&c).Error)
				require.Equal(t, int32(entity.ItemRunState_Processing), b.Status)
				require.Equal(t, int32(entity.ItemRunState_Queueing), c.Status)
			})
		}
	}
}

func TestLateWriteMySQLStoreThenCompleteEmptyRefs(t *testing.T) {
	f := newLateWriteFixture(t, "")
	source := f.source()
	event := f.event()
	event.RequestID = ""
	event.ExptRunMode = entity.EvaluationModeSubmit
	ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: source})
	executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), TurnResultRepo: f.turnRepo()}
	for _, id := range []int64{0, f.base + 22} {
		etec := &entity.ExptTurnEvalCtx{Turn: &entity.Turn{ID: id}, ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: event, Expt: &entity.Experiment{ID: f.expt, SpaceID: f.space}, EvalSetItem: &entity.EvaluationSetItem{ItemID: f.item}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: source.Turns}}}
		require.NoError(t, executor.storeTurnRunResult(ctx, etec, &entity.ExptTurnRunResult{}))
	}
	require.NoError(t, executor.CompleteItemRun(ctx, &entity.ExptItemEvalCtx{Event: event}, nil))
	var log model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&log).Error)
	require.Equal(t, int32(entity.ItemRunState_Success), log.Status)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(log.ResultState))
}
