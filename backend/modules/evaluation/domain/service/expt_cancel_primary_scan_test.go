// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

// Both schemas live on the task-owned real MySQL instance. The replica keeps
// one committed log while the primary has two, without timing-dependent lag.
func newCancelPrimaryScanFixture(t *testing.T) (*pendingRunFixture, *gorm.DB) {
	t.Helper()
	primaryDSN, replicaDSN := os.Getenv("EXPT_CANCEL_SCAN_PRIMARY_DSN"), os.Getenv("EXPT_CANCEL_SCAN_REPLICA_DSN")
	if primaryDSN == "" || replicaDSN == "" {
		t.Skip("EXPT_CANCEL_SCAN_PRIMARY_DSN/EXPT_CANCEL_SCAN_REPLICA_DSN missing; REAL MYSQL ROUTING NOT EXECUTED")
	}
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	provider, err := db.NewDB(mysql.Open(primaryDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	primary := provider.NewSession(context.Background(), db.WithMaster())
	replica, err := gorm.Open(mysql.Open(replicaDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	for conn, schema := range map[*gorm.DB]string{primary: "expt_cancel_scan_7384603622_primary", replica: "expt_cancel_scan_7384603622_replica"} {
		var identity struct{ Version, Schema string }
		require.NoError(t, conn.Raw("SELECT VERSION() AS version, DATABASE() AS `schema`").Scan(&identity).Error)
		require.Equal(t, schema, identity.Schema)
		t.Logf("REAL_MYSQL_ROUTING version=%s schema=%s", identity.Version, identity.Schema)
	}
	tables := []string{"experiment", "expt_run_log", "expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_turn_evaluator_result_ref", "expt_stats", "expt_item_ref", "eval_target_record", "evaluator_record", "expt_evaluator_ref"}
	t.Cleanup(func() {
		for _, table := range tables {
			require.NoError(t, primary.Exec("DELETE FROM "+table+" WHERE space_id=?", f.space).Error)
		}
		require.NoError(t, replica.Exec("DELETE FROM expt_item_result_run_log WHERE space_id=?", f.space).Error)
		for _, conn := range []*gorm.DB{primary, replica} {
			pool, err := conn.DB()
			require.NoError(t, err)
			require.NoError(t, pool.Close())
		}
	})
	for _, table := range tables {
		require.NoError(t, primary.Exec("INSERT INTO "+table+" SELECT * FROM expt_running_retry_test."+table+" WHERE space_id=?", f.space).Error)
	}
	primaryPool, err := primary.DB()
	require.NoError(t, err)
	replicaPool, err := replica.DB()
	require.NoError(t, err)
	routed, err := db.NewDB(mysql.New(mysql.Config{Conn: primaryPool, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, routed.NewSession(context.Background()).Use(dbresolver.Register(dbresolver.Config{
		Sources:  []gorm.Dialector{mysql.New(mysql.Config{Conn: primaryPool, SkipInitializeWithVersion: true})},
		Replicas: []gorm.Dialector{mysql.New(mysql.Config{Conn: replicaPool, SkipInitializeWithVersion: true})},
	})))
	f.provider, f.conn = routed, primary
	items := exptrepo.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(routed))
	f.port = items.(repo.IExptCurrentResultRepo)
	p.manager.itemResultRepo = items
	p.manager.exptRepo = exptrepo.NewExptRepo(exptmysql.NewExptDAO(routed), exptmysql.NewExptEvaluatorRefDAO(routed), p.manager.idgenerator)
	p.manager.runLogRepo = exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(routed))
	p.manager.runFinishRepo = exptrepo.NewExptRunFinishRepo(routed)
	p.manager.statsRepo = exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(routed))
	p.manager.turnResultRepo = f.turnRepo()
	p.manager.itemRefRepo = exptrepo.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(routed))
	f.token("original-token-one")
	// Use the existing domain serializer so fixture sources match the real contract.
	manual, err := json.Marshal(&entity.ManualRetryContext{RunID: f.run, RequestID: "original-token-two", Mode: entity.EvaluationModeFailRetry, Operator: "test", AcceptedAtMS: 1})
	require.NoError(t, err)
	secondExt, err := json.Marshal(map[string]string{"other": "keep", entity.ManualRetryExtKey: string(manual)})
	require.NoError(t, err)
	require.NoError(t, primary.Create(&model.ExptItemResult{ID: f.base + 20001, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, Status: int32(entity.ItemRunState_Processing), Ext: &secondExt}).Error)
	require.NoError(t, primary.Create(&model.ExptItemResultRunLog{ID: f.base + 21001, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	require.NoError(t, primary.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Processing).Error)
	require.NoError(t, primary.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Processing).Error)
	require.NoError(t, primary.Model(&model.ExptStats{}).Where("space_id=?", f.space).Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 2}).Error)
	require.NoError(t, primary.Create(&model.ExptTurnResult{ID: f.base + 22001, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Processing)}).Error)
	require.NoError(t, primary.Create(&model.ExptTurnResultRunLog{ID: f.base + 23001, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Processing)}).Error)
	require.NoError(t, replica.Exec("INSERT INTO expt_item_result_run_log SELECT * FROM expt_cancel_scan_7384603622_primary.expt_item_result_run_log WHERE space_id=? AND item_id=?", f.space, f.item).Error)
	require.NoError(t, primary.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	return p, replica
}

func TestCancelPrimaryScanRealMySQLRoutingAndPaging(t *testing.T) {
	p, _ := newCancelPrimaryScanFixture(t)
	f := p.f
	for _, raw := range []bool{false, true} {
		for _, retryOrder := range []bool{false, true} {
			t.Run(fmt.Sprintf("raw_%t/retry_order_%t", raw, retryOrder), func(t *testing.T) {
				filter := &entity.ExptItemRunLogFilter{RawFilter: raw, OrderByRetryTimesFirst: retryOrder}
				if raw {
					filter.RawCond.SQL = "status = ?"
					filter.RawCond.Vars = []any{int32(entity.ItemRunState_Processing)}
				}
				legacy, _, err := f.itemRepo().ScanItemRunLogs(context.Background(), f.expt, f.run, filter, 0, 500, f.space)
				require.NoError(t, err)
				require.Len(t, legacy, 1, "ordinary reads retain replica routing")
				require.Equal(t, f.item, legacy[0].ItemID)
				ctx := contexts.WithCtxWriteDB(context.Background())
				all, _, err := f.itemRepo().ScanItemRunLogs(ctx, f.expt, f.run, filter, 0, 500, f.space)
				require.NoError(t, err)
				require.Len(t, all, 2, "primary enumeration must include the replica-missing committed log")
				require.Equal(t, []int64{f.item, f.item + 1}, []int64{all[0].ItemID, all[1].ItemID})
				if !retryOrder {
					var cursor int64
					var ids []int64
					for page := 0; page < 3; page++ {
						rows, next, err := f.itemRepo().ScanItemRunLogs(ctx, f.expt, f.run, filter, cursor, 1, f.space)
						require.NoError(t, err)
						if page == 2 {
							require.Empty(t, rows)
							require.Zero(t, next)
							break
						}
						require.Len(t, rows, 1)
						require.Greater(t, next, cursor)
						ids = append(ids, rows[0].ItemID)
						cursor = next
					}
					require.Equal(t, []int64{f.item, f.item + 1}, ids)
				}
			})
		}
	}
}

func TestCancelPrimaryScanTerminatesReplicaMissingItem(t *testing.T) {
	p, replica := newCancelPrimaryScanFixture(t)
	f := p.f
	guard := &fakeGuard{}
	p.manager.centralGuard = guard
	applied, err := p.manager.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, p.event.Session, nil, entity.NoAggrCalculate())
	require.NoError(t, err)
	require.True(t, applied)
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
		var rows []model.ExptItemResultRunLog
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Order("item_id").Find(&rows).Error)
		require.Len(t, rows, 2)
		for _, row := range rows {
			require.Equal(t, f.run, row.ExptRunID)
			require.Equal(t, int32(entity.ItemRunState_Terminal), row.Status, "every primary item must terminate")
		}
	}
	var experiment model.Experiment
	require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&experiment).Error)
	require.Equal(t, f.run, experiment.LatestRunID)
	require.Equal(t, int32(entity.ExptStatus_Terminated), experiment.Status)
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&stats).Error)
	require.Zero(t, stats.ProcessingCnt)
	require.Equal(t, int32(2), stats.TerminatedCnt)
	require.Len(t, guard.releases(), 2, "both original reservations must be released")
	for i, release := range guard.releases() {
		require.Equal(t, f.run, release.RunID)
		require.Equal(t, f.item+int64(i), release.ItemID)
		require.Equal(t, []string{"original-token-one", "original-token-two"}[i], release.RequestID)
	}
	var stale []model.ExptItemResultRunLog
	require.NoError(t, replica.Where("space_id=?", f.space).Find(&stale).Error)
	require.Len(t, stale, 1)
	require.Equal(t, int32(entity.ItemRunState_Processing), stale[0].Status)
	t.Log("REAL_MYSQL_CANCEL primary_items=2 replica_items=1 terminal_items=2 original_token_releases=2 latest_run_preserved=true")
}

func TestCancelPrimaryScanNewRunReleasesBothOriginalTokens(t *testing.T) {
	p, _ := newCancelPrimaryScanFixture(t)
	f := p.f
	next := f.run + 100
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.ExptStatus_Terminated), "end_at": time.Now()}).Error)
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Terminated)).Error)
	guard := &fakeGuard{}
	p.manager.centralGuard = guard
	_, err := p.manager.mutex.UnlockWithValue(context.Background(), p.manager.makeExptMutexLockKey(f.expt), strconv.FormatInt(f.run, 10))
	require.NoError(t, err)
	metric := metricsmocks.NewMockExptMetric(gomock.NewController(t))
	metric.EXPECT().EmitExptExecRun(f.space, int64(entity.EvaluationModeRetryAll))
	p.manager.mtr = metric
	require.NoError(t, p.manager.LogRun(context.Background(), f.expt, next, entity.EvaluationModeRetryAll, f.space, []int64{f.item, f.item + 1}, p.event.Session))
	require.Len(t, guard.releases(), 2, "new run must release the replica-missing old original reservation")
	for i, release := range guard.releases() {
		require.Equal(t, f.run, release.RunID)
		require.Equal(t, f.item+int64(i), release.ItemID)
		require.Equal(t, []string{"original-token-one", "original-token-two"}[i], release.RequestID)
	}
	resetGuard := componentmocks.NewMockICentralReservationGuard(gomock.NewController(t))
	for i, token := range []string{"original-token-one", "original-token-two"} {
		resetGuard.EXPECT().ReleaseTerminalReservation(gomock.Any(), testScope, f.run, f.item+int64(i), token).Return(component.TerminalReservationAbsent, nil)
	}
	p.manager.centralGuard = resetGuard
	require.NoError(t, p.manager.ResetItemsForNewRun(context.Background(), f.space, f.expt, next, []int64{f.item, f.item + 1}, map[string]any{"status": int32(entity.ItemRunState_Queueing), "expt_run_id": next}))
	var rows []model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id=?", f.space).Order("item_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, next, row.ExptRunID)
		ext, err := entity.ParseExptItemResultExt(gptr.Indirect(row.Ext))
		require.NoError(t, err)
		require.Equal(t, map[string]string{"other": "keep"}, ext)
	}
	t.Log("REAL_MYSQL_NEW_RUN primary_items=2 replica_items=1 old_original_token_releases=2 manual_retry_only_cleared=true")
}
