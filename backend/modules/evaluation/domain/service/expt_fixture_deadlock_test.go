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

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLateWriteFixtureInsertRetriesOnlyRolledBackDeadlocks(t *testing.T) {
	t.Run("deadlock then success", func(t *testing.T) {
		calls := 0
		err := retryLateWriteFixtureInsert(func() error {
			calls++
			if calls < 3 {
				return &drivermysql.MySQLError{Number: 1213, Message: "deadlock"}
			}
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 3, calls)
	})
	t.Run("bounded deadlocks", func(t *testing.T) {
		calls := 0
		want := &drivermysql.MySQLError{Number: 1213, Message: "deadlock"}
		err := retryLateWriteFixtureInsert(func() error { calls++; return want })
		require.Same(t, want, err)
		require.Equal(t, 4, calls)
	})
	t.Run("non-MySQL error", func(t *testing.T) {
		calls := 0
		want := context.DeadlineExceeded
		err := retryLateWriteFixtureInsert(func() error { calls++; return want })
		require.ErrorIs(t, err, want)
		require.Equal(t, 1, calls)
	})
	for _, number := range []uint16{1062, 1205} {
		t.Run(fmt.Sprint(number), func(t *testing.T) {
			calls := 0
			want := &drivermysql.MySQLError{Number: number, Message: "must not retry"}
			err := retryLateWriteFixtureInsert(func() error { calls++; return want })
			require.Same(t, want, err)
			require.Equal(t, 1, calls)
		})
	}
}

func TestLateWriteFixtureInsertRecoversRealMySQLDeadlock(t *testing.T) {
	f := newLateWriteFixture(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := int64(0); i < 20; i++ {
		require.NoError(t, f.conn.Create(&model.ExptTurnEvaluatorResultRef{ID: f.base + 10000 + i, SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: f.base + 400, EvaluatorVersionID: f.base + 1000 + i, EvaluatorResultID: f.base + 1000 + i}).Error)
	}
	tx := f.conn.WithContext(ctx).Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	// Keep this blocker heavier so InnoDB selects the one-row fixture INSERT as victim.
	for i := int64(0); i < 20; i++ {
		require.NoError(t, tx.Exec("UPDATE expt_turn_evaluator_result_ref SET evaluator_result_id=evaluator_result_id+1 WHERE id=?", f.base+10000+i).Error)
	}
	var blockerID int64
	require.NoError(t, tx.Raw("SELECT CONNECTION_ID()").Scan(&blockerID).Error)
	var lockedIDs []int64
	require.NoError(t, tx.Raw("SELECT id FROM expt_turn_evaluator_result_ref FORCE INDEX (idx_expt_evaluator_result) WHERE space_id=? AND expt_id=? FOR UPDATE", f.space, f.expt).Scan(&lockedIDs).Error)
	inserted := &model.ExptTurnEvaluatorResultRef{ID: f.base + 9000, SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: f.base + 400, EvaluatorVersionID: f.base + 500, EvaluatorResultID: f.base + 500}
	var attempts, deadlocks atomic.Int32
	done := make(chan error, 1)
	requester := make(chan int64, 1)
	go func() {
		done <- f.conn.WithContext(ctx).Connection(func(conn *gorm.DB) error {
			var id int64
			if err := conn.WithContext(ctx).Raw("SELECT CONNECTION_ID()").Scan(&id).Error; err != nil {
				return err
			}
			requester <- id
			return retryLateWriteFixtureInsert(func() error {
				attempts.Add(1)
				err := conn.WithContext(ctx).Create(inserted).Error
				var sqlErr *drivermysql.MySQLError
				if errors.As(err, &sqlErr) && sqlErr.Number == 1213 {
					deadlocks.Add(1)
				}
				return err
			})
		})
	}()
	var requesterID int64
	select {
	case requesterID = <-requester:
	case err := <-done:
		t.Fatalf("fixture connection failed: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.Eventually(t, func() bool {
		var waits int64
		err := f.conn.WithContext(ctx).Raw("SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.threads b ON b.THREAD_ID=w.BLOCKING_THREAD_ID JOIN performance_schema.threads r ON r.THREAD_ID=w.REQUESTING_THREAD_ID WHERE b.PROCESSLIST_ID=? AND r.PROCESSLIST_ID=?", blockerID, requesterID).Scan(&waits).Error
		return err == nil && waits > 0
	}, 3*time.Second, 5*time.Millisecond, "fixture INSERT did not reach the secondary-index lock")
	var pendingIDs []int64
	require.NoError(t, tx.Raw("SELECT id FROM expt_turn_evaluator_result_ref FORCE INDEX (PRIMARY) WHERE id=? FOR UPDATE", inserted.ID).Scan(&pendingIDs).Error)
	require.NoError(t, tx.Commit().Error)
	var insertErr error
	select {
	case insertErr = <-done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	t.Logf("REAL_MYSQL_FIXTURE_INSERT_DEADLOCK deadlocks=%d attempts=%d", deadlocks.Load(), attempts.Load())
	require.Positive(t, deadlocks.Load(), "test must observe a real MySQL1213")
	require.NoError(t, insertErr)
	require.Greater(t, attempts.Load(), int32(1))
	var count int64
	require.NoError(t, f.conn.Model(&model.ExptTurnEvaluatorResultRef{}).Where("id=?", inserted.ID).Count(&count).Error)
	require.Equal(t, int64(1), count, "retry must not duplicate fixture rows")
}
