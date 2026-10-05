// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func cleanupExecutorFixture(s *gorm.DB, spaceID, exptID int64) (err error) {
	pool, err := s.DB()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, pool.Close()) }()
	var operationIDs []string
	if err := s.Model(&model.ExptLifecycleHookRun{}).Where("space_id=? AND expt_id=?", spaceID, exptID).Pluck("operation_id", &operationIDs).Error; err != nil {
		return err
	}
	if len(operationIDs) != 0 {
		if err := s.Exec("DELETE FROM expt_lifecycle_hook_attempt WHERE operation_id IN ?", operationIDs).Error; err != nil {
			return err
		}
	}
	for _, table := range []string{"expt_lifecycle_run_item", "expt_lifecycle_hook_run", "expt_lifecycle_run", "expt_run_log"} {
		if err := s.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", spaceID, exptID).Error; err != nil {
			return err
		}
	}
	if err := s.Unscoped().Delete(&model.Experiment{}, "id=? AND space_id=?", exptID, spaceID).Error; err != nil {
		return err
	}
	return nil
}

func newHookCleanupConnection(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires isolated HOOK_MYSQL_TX_DSN")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	s, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := s.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(3)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	return s
}

func TestHookFixtureCleanupIgnoresUnrelatedOperationLock(t *testing.T) {
	for _, state := range []string{"attempt", "operation", "empty"} {
		t.Run(state, func(t *testing.T) {
			empty := state == "empty"
			s := newHookCleanupConnection(t)
			space, expt, otherSpace, otherExpt := executorIDs.Add(1), executorIDs.Add(1), executorIDs.Add(1), executorIDs.Add(1)
			ownID, otherID := executorIDs.Add(1), executorIDs.Add(1)
			ownAttemptID, otherAttemptID := executorIDs.Add(1), executorIDs.Add(1)
			t.Cleanup(func() {
				require.NoError(t, s.Exec("DELETE FROM expt_lifecycle_hook_attempt WHERE id IN (?,?)", ownAttemptID, otherAttemptID).Error)
				require.NoError(t, s.Exec("DELETE FROM expt_lifecycle_hook_run WHERE id IN (?,?)", ownID, otherID).Error)
				require.NoError(t, s.Unscoped().Delete(&model.Experiment{}, "(id=? AND space_id=?) OR (id=? AND space_id=?)", expt, space, otherExpt, otherSpace).Error)
			})
			require.NoError(t, s.Create(&model.Experiment{ID: expt, SpaceID: space, Name: "cleanup-owner", Status: 3}).Error)
			require.NoError(t, s.Create(&model.Experiment{ID: otherExpt, SpaceID: otherSpace, Name: "cleanup-unrelated", Status: 3}).Error)
			seed := func(id, attemptID, spaceID, exptID int64) {
				opID := fmt.Sprintf("cleanup_%d", id)
				require.NoError(t, s.Create(&model.ExptLifecycleHookRun{ID: id, SpaceID: spaceID, ExptID: exptID, ExptRunID: id, Phase: "before", OperationID: opID, IdempotencyKey: opID, Status: "pending", ExecutionScope: "local"}).Error)
				if attemptID != 0 {
					require.NoError(t, s.Create(&model.ExptLifecycleHookAttempt{ID: attemptID, OperationID: opID, Attempt: 1, DeliveryID: opID, LeaseGeneration: 1, StartedAt: time.Now()}).Error)
				}
			}
			if !empty {
				attemptID := ownAttemptID
				if state == "operation" {
					attemptID = 0
				}
				seed(ownID, attemptID, space, expt)
			}
			seed(otherID, otherAttemptID, otherSpace, otherExpt)
			var beforeOp model.ExptLifecycleHookRun
			var beforeAttempt model.ExptLifecycleHookAttempt
			require.NoError(t, s.First(&beforeOp, "id=?", otherID).Error)
			require.NoError(t, s.First(&beforeAttempt, "id=?", otherAttemptID).Error)
			lock := s.Begin()
			require.NoError(t, lock.Error)
			defer func() { require.NoError(t, lock.Rollback().Error) }()
			var locked model.ExptLifecycleHookRun
			require.NoError(t, lock.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id=?", otherID).Error)
			cleanup := newHookCleanupConnection(t)
			pool, err := cleanup.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			require.NoError(t, cleanup.Exec("SET SESSION innodb_lock_wait_timeout=1").Error)
			attemptDeletes := 0
			require.NoError(t, cleanup.Callback().Raw().Before("gorm:raw").Register("count_attempt_delete", func(tx *gorm.DB) {
				if strings.HasPrefix(tx.Statement.SQL.String(), "DELETE FROM expt_lifecycle_hook_attempt") {
					attemptDeletes++
				}
			}))
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			err = cleanupExecutorFixture(cleanup.WithContext(ctx), space, expt)
			require.NoError(t, err, "cleanup must not wait for an unrelated operation row lock")
			if empty {
				require.Zero(t, attemptDeletes)
			} else {
				require.Equal(t, 1, attemptDeletes)
			}
			require.ErrorContains(t, pool.Ping(), "database is closed")
			var afterOp model.ExptLifecycleHookRun
			var afterAttempt model.ExptLifecycleHookAttempt
			require.NoError(t, s.First(&afterOp, "id=?", otherID).Error)
			require.NoError(t, s.First(&afterAttempt, "id=?", otherAttemptID).Error)
			require.Equal(t, beforeOp, afterOp)
			require.Equal(t, beforeAttempt, afterAttempt)
			for _, tc := range []struct {
				table string
				id    int64
			}{
				{model.TableNameExperiment, expt}, {model.TableNameExptLifecycleHookRun, ownID}, {model.TableNameExptLifecycleHookAttempt, ownAttemptID},
			} {
				var count int64
				require.NoError(t, s.Table(tc.table).Where("id=?", tc.id).Count(&count).Error)
				require.Zero(t, count)
			}
			var count int64
			require.NoError(t, s.Model(&model.Experiment{}).Where("id=? AND space_id=?", otherExpt, otherSpace).Count(&count).Error)
			require.Equal(t, int64(1), count)
		})
	}
}

func TestHookFixtureCleanupClosesConnectionOnFailure(t *testing.T) {
	s := newHookCleanupConnection(t)
	pool, err := s.DB()
	require.NoError(t, err)
	injected := errors.New("cleanup query failure")
	require.NoError(t, s.Callback().Query().Before("gorm:query").Register("fail_cleanup_query", func(tx *gorm.DB) { tx.AddError(injected) }))
	require.NoError(t, s.Callback().Raw().Before("gorm:raw").Register("fail_cleanup_raw", func(tx *gorm.DB) { tx.AddError(injected) }))
	err = cleanupExecutorFixture(s, executorIDs.Add(1), executorIDs.Add(1))
	require.ErrorIs(t, err, injected)
	require.ErrorContains(t, pool.Ping(), "database is closed")
}
