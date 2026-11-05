// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentck "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

// Exercise both consumer branches with the real execution mutex and result
// service. Reentering the public refresh skips INSERT while returning nil.
func TestRunningRetryFilterConsumer_HeldLockRefresh(t *testing.T) {
	for _, reservation := range []bool{true, false} {
		for _, saveFails := range []bool{false, true} {
			t.Run(fmt.Sprintf("reserved_%t_save_error_%t", reservation, saveFails), func(t *testing.T) {
				testRunningRetryFilterConsumer(t, reservation, saveFails)
			})
		}
	}
}

func testRunningRetryFilterConsumer(t *testing.T, reservation, saveFails bool) {
	t.Helper()
	f := newFilterSyncFixture(t)
	ctx := context.Background()
	event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, EvalSetItemID: f.item, RequestID: "rq2", ExptRunMode: entity.EvaluationModeFailRetry}
	initialState, finalState := entity.ItemRunState_Processing, entity.ItemRunState_Queueing
	initialTurn, finalTurn := entity.TurnRunState_Processing, entity.TurnRunState_Queueing
	reservationState := entity.QuotaReservationStateNone
	if reservation {
		initialState, finalState = entity.ItemRunState_Queueing, entity.ItemRunState_Processing
		initialTurn, finalTurn = entity.TurnRunState_Queueing, entity.TurnRunState_Processing
		reservationState = entity.QuotaReservationStateReserved
	}
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", initialState).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(initialState), "quota_reservation_state": int32(reservationState)}).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", initialTurn).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", initialTurn).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("result_state", entity.ExptItemResultStateDefault).Error)
	// Counts follow the actual canonical old bucket before atomic Start/Requeue.
	pending, processing := int32(1), int32(2)
	if reservation {
		pending, processing = 2, 1
	}
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"pending_cnt": pending, "processing_cnt": processing}).Error)

	// The ID-only work was queued before the source changed. A separate requeue
	// holder publishes the new snapshot before the consumer acquires its lock.
	holder := filterRealTestLocker(f.client)
	ok, err := holder.Lock(ctx, f.key(), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	f.token("rq2")
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"target_result_id": f.base + 91, "item_version_id": f.version + 1}).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", f.version+1).Error)
	// The new DB source contract requires both frozen log versions to match.
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", f.version+1).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"item_version_id": f.version + 1, "target_result_id": f.base + 91}).Error)
	require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"item_version_id": f.version + 1, "eval_set_id": f.base + 92, "eval_set_version_id": f.base + 93}).Error)
	_, err = holder.Unlock(f.key())
	require.NoError(t, err)

	var active atomic.Bool
	var payloadReads atomic.Int32
	callback := "consumer_filter_item_lock"
	require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if !active.Load() {
			return
		}
		require.True(t, f.server.Exists(f.key()), "consumer source query escaped execution lock")
		switch tx.Statement.Table {
		case "expt_turn_result", "expt_item_ref", "expt_turn_evaluator_result_ref":
			payloadReads.Add(1)
		}
	}))
	t.Cleanup(func() { _ = f.conn.Callback().Query().Remove(callback) })

	ctrl := gomock.NewController(t)
	sqlDB, sqlMock, err := sqlmock.New(sqlmock.ValueConverterOption(filterSQLConverter{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	ckConn, err := gorm.Open(clickhouse.New(clickhouse.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	var saves atomic.Int32
	competitor := filterRealTestLocker(f.client)
	require.NoError(t, ckConn.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		saves.Add(1)
		require.True(t, f.server.Exists(f.key()), "OSS CK Save escaped execution lock")
		acquired, lockErr := competitor.Lock(tx.Statement.Context, f.key(), time.Minute)
		require.NoError(t, lockErr)
		if acquired {
			_, _ = competitor.Unlock(f.key())
		}
		require.False(t, acquired, "independent holder acquired item before prepared INSERT")
	}))
	mapping := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl)
	mapping.EXPECT().GetByExptID(gomock.Any(), f.space, f.expt).Return(nil, nil).AnyTimes()
	f.svc.exptTurnResultFilterRepo = experimentrepo.NewExptTurnResultFilterRepo(experimentck.NewExptTurnResultFilterDAO(&filterCKProvider{conn: ckConn}, nil), mapping)
	sqlMock.ExpectBegin()
	insert := sqlMock.ExpectPrepare("INSERT INTO `expt_turn_result_filter`").ExpectExec().WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), filterLockedSQLArg{f.server, f.key(), fmt.Sprint(f.item)}, fmt.Sprint(f.version+1), int64(0), "0", int64(finalState), fmt.Sprintf("{\"actual_output\":\"%d\"}", f.base+91), "{}", "{}", "{}", "{}", "{}", int64(0), fmt.Sprint(f.base+92), float64(0), fmt.Sprint(f.base+93), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg())
	if saveFails {
		insert.WillReturnError(errors.New("consumer CK insert unavailable"))
		sqlMock.ExpectRollback()
	} else {
		insert.WillReturnResult(sqlmock.NewResult(0, 1))
		sqlMock.ExpectCommit()
	}

	// Both services use the same production key and independent real holders.
	consumer := &ExptItemEventEvalServiceImpl{
		itemLocker:         func() lock.ILocker { return filterRealTestLocker(f.client) },
		centralGuard:       &fakeGuard{confirmResult: reservation},
		dispatchRepo:       experimentrepo.NewExptItemDispatchRepo(experimentmysql.NewExptItemDispatchDAO(f.provider)),
		exptItemResultRepo: f.itemRepo(),
		exptTurnResultRepo: f.turnRepo(),
		exptStatsRepo:      experimentrepo.NewExptStatsRepo(experimentmysql.NewExptStatsDAO(f.provider)),
		resultSvc:          f.svc,
	}
	nextCalls := 0
	handler := consumer.HandleEventLock(consumer.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextCalls++
		require.True(t, f.server.Exists(f.key()))
		return nil
	}))
	active.Store(true)
	err = handler(admittedEnforceCtx(event), event)
	active.Store(false)
	require.NoError(t, err, "consumer display refresh errors remain best effort")
	require.Equal(t, int32(1), saves.Load(), "consumer must attempt the actual OSS CK INSERT while holding its item lock")
	require.GreaterOrEqual(t, payloadReads.Load(), int32(3), "canonical turn, ref and evaluator payload must be reread inside consumer lock")
	require.NoError(t, sqlMock.ExpectationsWereMet())
	if reservation {
		require.Equal(t, 1, nextCalls)
	} else {
		require.Zero(t, nextCalls, "absent reservation repairs the projection without executing")
	}
	items, err := f.itemRepo().BatchGet(ctx, f.space, f.expt, []int64{f.item})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, finalState, items[0].Status)
	var stats mysqlmodel.ExptStats
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
	if reservation {
		require.Equal(t, int32(1), stats.PendingCnt)
		require.Equal(t, int32(2), stats.ProcessingCnt)
	} else {
		require.Equal(t, int32(2), stats.PendingCnt)
		require.Equal(t, int32(1), stats.ProcessingCnt)
	}
	var turn mysqlmodel.ExptTurnResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Take(&turn).Error)
	require.Equal(t, int32(finalTurn), turn.Status)
	require.Equal(t, f.base+91, turn.TargetResultID)
	observations, err := consumer.dispatchRepo.MGetDispatchObservations(ctx, f.space, f.expt, f.run, []int64{f.item})
	require.NoError(t, err)
	require.Len(t, observations, 1)
	require.Equal(t, int32(finalState), observations[0].Status)
	require.Equal(t, entity.QuotaReservationStateNone, observations[0].QuotaReservationState)
	require.False(t, f.server.Exists(f.key()), "consumer releases execution lock after refresh")
	ok, err = competitor.Lock(ctx, f.key(), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	_, err = competitor.Unlock(f.key())
	require.NoError(t, err)
	t.Log("REAL_MYSQL_SOURCE_CONSUMER_OSS_CK_PREPARED_INSERT_SQLMOCK_EXECUTED; boundary is SQLmock, not real CK/ByteHouse")
}
