// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentck "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck"
	experimentckmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/gorm_gen/model"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// The public loop must attempt later items after one item fails. A first-item
// early return leaves real prepared INSERT expectations and later reads unmet.
func TestRunningRetryFilterBatch_UpsertContinuesAfterItemFailure(t *testing.T) {
	for _, failure := range []string{"busy", "busy_enumerated", "save", "read", "lease_lost", "none"} {
		t.Run(failure, func(t *testing.T) {
			f := newFilterBatchFixture(t)
			firstErr := errors.New("first item source or insert unavailable")
			if failure == "busy" || failure == "busy_enumerated" {
				f.holdFirst(t)
			}
			if failure == "read" || failure == "lease_lost" {
				f.beforeRead = func(ctx context.Context, item int64) error {
					if item != f.item {
						return nil
					}
					if failure == "read" {
						return firstErr
					}
					f.server.Del(f.key())
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(3 * time.Second):
						t.Fatal("real Redis lease loss did not cancel item context")
						return nil
					}
				}
			}
			sqlMock, _ := f.attachCK(t)
			start := 0
			if failure == "busy" || failure == "busy_enumerated" || failure == "read" || failure == "lease_lost" {
				start = 1
			}
			for i := start; i < 3; i++ {
				var saveErr error
				if i == 0 && failure == "save" {
					saveErr = firstErr
				}
				f.expectInsert(sqlMock, i, saveErr)
			}
			ids := f.unorderedIDs()
			if failure == "busy_enumerated" {
				ids = nil // Scheduler/MQ whole-experiment entry enumerates IDs first.
			}
			err := f.svc.UpsertExptTurnResultFilter(context.Background(), f.space, f.expt, ids)
			if failure == "none" {
				require.NoError(t, err, "successful batch must return nil, not a wrapper around nil")
			} else {
				require.True(t, errno.NeedMQRetry(err))
				if failure == "save" || failure == "read" {
					require.ErrorIs(t, err, firstErr)
				}
			}
			require.Equal(t, f.ids[start:], f.reads, "later current payloads must still be read after an item failure")
			require.Equal(t, f.keys(), f.locks.attempts(), "sort and deduplicate before independent item lock attempts")
			require.NoError(t, sqlMock.ExpectationsWereMet())
			t.Log("REAL_MYSQL_BATCH_SOURCE_OSS_CK_PREPARED_INSERT_SQLMOCK_EXECUTED")
		})
	}
}

func TestRunningRetryFilterBatch_CompareContinuesAfterItemFailure(t *testing.T) {
	for _, failure := range []string{"busy", "read", "actual_read", "none"} {
		t.Run(failure, func(t *testing.T) {
			f := newFilterBatchFixture(t)
			firstErr := errors.New("first item compare source unavailable")
			if failure == "busy" {
				f.holdFirst(t)
			}
			if failure == "read" {
				f.beforeRead = func(_ context.Context, item int64) error {
					if item == f.item {
						return firstErr
					}
					return nil
				}
			}
			sqlMock, ckConn := f.attachCK(t)
			var actualReads []int64
			query := "SELECT etrf\\.\\* FROM .*expt_turn_result_filter AS etrf FINAL WHERE etrf.space_id=\\? AND etrf.expt_id=\\? AND etrf.created_date=\\? AND etrf.item_id IN \\(\\?\\)"
			// The actual reader must remain inside the same item's execution lock.
			require.NoError(t, ckConn.Callback().Row().Before("gorm:row").Register("batch_actual_item_lock", func(tx *gorm.DB) {
				item := f.ids[len(actualReads)]
				if failure == "busy" || failure == "read" {
					item++
				}
				require.True(t, f.server.Exists(f.itemKey(item)))
				actualReads = append(actualReads, item)
			}))
			start := 0
			if failure == "busy" || failure == "read" {
				start = 1
			}
			for i := start; i < 3; i++ {
				q := sqlMock.ExpectQuery(query).WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), "2026-10-20", filterLockedSQLArg{f.server, f.itemKey(f.ids[i]), fmt.Sprint(f.ids[i])})
				if i == 0 && failure == "actual_read" {
					q.WillReturnError(firstErr)
				} else {
					q.WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "item_id"}))
				}
			}
			ctrl := gomock.NewController(t)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			f.svc.publisher = publisher
			var events []int64
			publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptTurnResultFilterEvent, delay *time.Duration) error {
				require.Len(t, event.ItemID, 1)
				require.True(t, f.server.Exists(f.itemKey(event.ItemID[0])))
				require.Equal(t, int32(2), *event.RetryTimes)
				require.Equal(t, entity.UpsertExptTurnResultFilterTypeCheck, *event.FilterType)
				require.Equal(t, 10*time.Second, *delay)
				events = append(events, event.ItemID[0])
				return nil
			}).AnyTimes()
			err := f.svc.CompareExptTurnResultFilters(context.Background(), f.space, f.expt, f.unorderedIDs(), 1)
			if failure == "none" {
				require.NoError(t, err)
			} else {
				require.True(t, errno.NeedMQRetry(err))
				if failure != "busy" {
					require.ErrorIs(t, err, firstErr)
				}
			}
			wantEvents := f.ids[start:]
			if failure == "actual_read" {
				wantEvents = f.ids[1:]
			}
			require.Equal(t, wantEvents, events, "later comparisons must publish missing current projections")
			require.Equal(t, f.ids[start:], actualReads)
			require.Equal(t, f.ids[start:], f.reads)
			require.Equal(t, f.keys(), f.locks.attempts())
			require.NoError(t, sqlMock.ExpectationsWereMet())
		})
	}
}

func TestRunningRetryFilterBatch_ParentCancelStopsLaterItems(t *testing.T) {
	for _, compare := range []bool{false, true} {
		t.Run(fmt.Sprint(compare), func(t *testing.T) {
			f := newFilterBatchFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.beforeRead = func(ctx context.Context, item int64) error {
				require.Equal(t, f.item, item)
				cancel()
				return ctx.Err()
			}
			var err error
			if compare {
				err = f.svc.CompareExptTurnResultFilters(ctx, f.space, f.expt, f.unorderedIDs(), 1)
			} else {
				err = f.svc.UpsertExptTurnResultFilter(ctx, f.space, f.expt, f.unorderedIDs())
			}
			require.True(t, errno.NeedMQRetry(err))
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, f.keys()[:1], f.locks.attempts(), "cancelled parent may not acquire later item locks")
			require.Empty(t, f.reads, "parent cancellation may not start any later source IO")
		})
	}
}

func TestRunningRetryFilterBatch_FirstErrorPreserved(t *testing.T) {
	for _, compare := range []bool{false, true} {
		t.Run(fmt.Sprint(compare), func(t *testing.T) {
			f := newFilterBatchFixture(t)
			firstErr, lastErr := errors.New("first item read failed"), errors.New("last item read failed")
			f.beforeRead = func(_ context.Context, item int64) error {
				if item == f.item {
					return firstErr
				}
				if item == f.item+2 {
					return lastErr
				}
				return nil
			}
			sqlMock, _ := f.attachCK(t)
			var err error
			if compare {
				sqlMock.ExpectQuery("SELECT etrf\\.\\* FROM .*expt_turn_result_filter AS etrf FINAL").WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), "2026-10-20", filterLockedSQLArg{f.server, f.itemKey(f.item + 1), fmt.Sprint(f.item + 1)}).WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "item_id"}))
				publisher := eventmocks.NewMockExptEventPublisher(gomock.NewController(t))
				f.svc.publisher = publisher
				publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptTurnResultFilterEvent, _ *time.Duration) error {
					require.Equal(t, []int64{f.item + 1}, event.ItemID)
					return nil
				})
				err = f.svc.CompareExptTurnResultFilters(context.Background(), f.space, f.expt, f.unorderedIDs(), 1)
			} else {
				f.expectInsert(sqlMock, 1, nil)
				err = f.svc.UpsertExptTurnResultFilter(context.Background(), f.space, f.expt, f.unorderedIDs())
			}
			require.True(t, errno.NeedMQRetry(err))
			require.ErrorIs(t, err, firstErr)
			require.NotErrorIs(t, err, lastErr)
			require.Equal(t, []int64{f.item + 1}, f.reads)
			require.Equal(t, f.keys(), f.locks.attempts())
			require.NoError(t, sqlMock.ExpectationsWereMet())
		})
	}
}

func TestRunningRetryFilterBatch_PreCancelledDoesNotEnumerate(t *testing.T) {
	for _, compare := range []bool{false, true} {
		t.Run(fmt.Sprint(compare), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			enumerations, factories := 0, 0
			items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(1), int64(3), gomock.Any(), false).DoAndReturn(func(context.Context, int64, int64, entity.Page, bool) ([]*entity.ExptItemResult, int64, error) {
				enumerations++
				return nil, 0, nil
			}).AnyTimes()
			svc := ExptResultServiceImpl{ExptItemResultRepo: items, resultPublishLocker: func() lock.ILocker { factories++; return nil }}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var err error
			if compare {
				err = svc.CompareExptTurnResultFilters(ctx, 3, 1, nil, 1)
			} else {
				err = svc.UpsertExptTurnResultFilter(ctx, 3, 1, nil)
			}
			require.True(t, errno.NeedMQRetry(err))
			require.ErrorIs(t, err, context.Canceled)
			require.Zero(t, enumerations)
			require.Zero(t, factories)
		})
	}
}

func TestRunningRetryFilterBatch_CompareMaxRetryKeepsMetrics(t *testing.T) {
	f := newFilterBatchFixture(t)
	sqlMock, _ := f.attachCK(t)
	for _, item := range f.ids {
		sqlMock.ExpectQuery("SELECT etrf\\.\\* FROM .*expt_turn_result_filter AS etrf FINAL").WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), "2026-10-20", filterLockedSQLArg{f.server, f.itemKey(item), fmt.Sprint(item)}).WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "item_id"}))
	}
	ctrl := gomock.NewController(t)
	f.svc.publisher = eventmocks.NewMockExptEventPublisher(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	f.svc.Metric = metric
	metric.EXPECT().EmitExptTurnResultFilterQueryLatency(f.space, gomock.Any(), false).Times(3)
	metric.EXPECT().EmitExptTurnResultFilterCheck(f.space, false, false, true, true).Times(3)
	require.NoError(t, f.svc.CompareExptTurnResultFilters(context.Background(), f.space, f.expt, f.unorderedIDs(), 3))
	require.Equal(t, f.ids, f.reads)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

type filterBatchFixture struct {
	*filterSyncFixture
	ids        []int64
	locks      *filterBatchRedis
	reads      []int64
	beforeRead func(context.Context, int64) error
}

func newFilterBatchFixture(t *testing.T) *filterBatchFixture {
	t.Helper()
	f := &filterBatchFixture{filterSyncFixture: newFilterSyncFixture(t)}
	f.ids = []int64{f.item, f.item + 1, f.item + 2}
	for i, item := range f.ids {
		index := int64(i)
		require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "item_version_id": f.version + index + 1}).Error)
		require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, item).Updates(map[string]any{"item_version_id": f.version + index + 1, "eval_set_id": f.base + 92 + index, "eval_set_version_id": f.base + 93 + index}).Error)
		if i == 0 {
			require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResult{}).Where("space_id=? AND item_id=?", f.space, item).Updates(map[string]any{"item_version_id": f.version + 1, "target_result_id": f.base + 91}).Error)
		} else {
			zero := 0.0
			require.NoError(t, f.conn.Create(&mysqlmodel.ExptTurnResult{ID: f.base + 400 + index, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: f.version + index + 1, TurnID: 0, Status: int32(entity.TurnRunState_Success), TargetResultID: f.base + 91 + index, WeightedScore: &zero}).Error)
		}
	}
	f.locks = &filterBatchRedis{Cmdable: f.client}
	f.svc.resultPublishLocker = func() lock.ILocker { return filterRealTestLocker(f.locks) }
	f.svc.ExptTurnResultRepo = &filterBatchTurnRepo{IExptTurnResultRepo: f.turnRepo(), before: func(ctx context.Context, item int64) error {
		require.True(t, f.server.Exists(f.itemKey(item)), "canonical payload must be rebuilt in its item lock")
		// The other later item remains independently available during this read.
		for _, other := range f.ids[1:] {
			if other == item {
				continue
			}
			probe := filterRealTestLocker(f.client)
			ok, err := probe.Lock(ctx, f.itemKey(other), time.Minute)
			require.NoError(t, err)
			require.True(t, ok, "batch must not hold all item locks together")
			_, err = probe.Unlock(f.itemKey(other))
			require.NoError(t, err)
		}
		if f.beforeRead != nil {
			if err := f.beforeRead(ctx, item); err != nil {
				return err
			}
		}
		f.reads = append(f.reads, item)
		return nil
	}}
	return f
}

func (f *filterBatchFixture) itemKey(item int64) string {
	return fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, item)
}

func (f *filterBatchFixture) keys() []string {
	return []string{f.itemKey(f.item), f.itemKey(f.item + 1), f.itemKey(f.item + 2)}
}

func (f *filterBatchFixture) unorderedIDs() []int64 {
	return []int64{f.item + 2, f.item, f.item + 1, f.item + 2, f.item}
}

func (f *filterBatchFixture) holdFirst(t *testing.T) {
	t.Helper()
	holder := filterRealTestLocker(f.client)
	ok, err := holder.Lock(context.Background(), f.key(), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	t.Cleanup(func() { _, _ = holder.Unlock(f.key()) })
}

func (f *filterBatchFixture) attachCK(t *testing.T) (sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.ValueConverterOption(filterSQLConverter{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := gorm.Open(clickhouse.New(clickhouse.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, conn.Callback().Create().Before("gorm:create").Register("batch_insert_item_lock", func(tx *gorm.DB) {
		rows, ok := tx.Statement.Dest.([]*experimentckmodel.ExptTurnResultFilter)
		require.True(t, ok)
		require.Len(t, rows, 1)
		require.True(t, f.server.Exists(fmt.Sprintf("expt_item_eval_run_lock:%d:%s", f.expt, rows[0].ItemID)))
	}))
	mapping := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(gomock.NewController(t))
	mapping.EXPECT().GetByExptID(gomock.Any(), f.space, f.expt).Return(nil, nil).AnyTimes()
	f.svc.exptTurnResultFilterRepo = experimentrepo.NewExptTurnResultFilterRepo(experimentck.NewExptTurnResultFilterDAO(&filterCKProvider{conn: conn}, nil), mapping)
	return mock, conn
}

func (f *filterBatchFixture) expectInsert(mock sqlmock.Sqlmock, i int, saveErr error) {
	index, item := int64(i), f.ids[i]
	mock.ExpectBegin()
	exec := mock.ExpectPrepare("INSERT INTO `expt_turn_result_filter`").ExpectExec().WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), filterLockedSQLArg{f.server, f.itemKey(item), fmt.Sprint(item)}, fmt.Sprint(f.version+index+1), int64(0), "0", int64(entity.ItemRunState_Success), fmt.Sprintf("{\"actual_output\":\"%d\"}", f.base+91+index), "{}", "{}", "{}", "{}", "{}", int64(0), fmt.Sprint(f.base+92+index), float64(0), fmt.Sprint(f.base+93+index), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg())
	if saveErr != nil {
		exec.WillReturnError(saveErr)
		mock.ExpectRollback()
	} else {
		exec.WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
	}
}

type filterBatchTurnRepo struct {
	repo.IExptTurnResultRepo
	before func(context.Context, int64) error
}

func (r *filterBatchTurnRepo) ListTurnResultByItemIDs(ctx context.Context, space, expt int64, ids []int64, page entity.Page, deleted bool) ([]*entity.ExptTurnResult, int64, error) {
	if err := r.before(ctx, ids[0]); err != nil {
		return nil, 0, err
	}
	return r.IExptTurnResultRepo.ListTurnResultByItemIDs(ctx, space, expt, ids, page, deleted)
}

type filterBatchRedis struct {
	infraredis.Cmdable
	mu   sync.Mutex
	keys []string
}

func (r *filterBatchRedis) SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd {
	r.mu.Lock()
	r.keys = append(r.keys, key)
	r.mu.Unlock()
	return r.Cmdable.SetNX(ctx, key, value, expiration)
}

func (r *filterBatchRedis) attempts() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.keys...)
}
