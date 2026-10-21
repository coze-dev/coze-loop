// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSourceStandardMySQLResultedFinishedReadOnly(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Success)).Error)
	before := f.snapshot()
	port, ok := NewExptResultPublishDAO(f.provider).(interface {
		GetStandardEvalOutputSource(context.Context, int64, int64, int64, int64) (*entity.ExptItemResultPublishSource, error)
	})
	require.True(t, ok, "production DAO must provide primary consistent source read")
	var queries []string
	require.NoError(t, f.conn.Callback().Query().After("gorm:query").Register("standard_source_no_record_read", func(tx *gorm.DB) {
		queries = append(queries, tx.Statement.SQL.String())
	}))
	source, err := port.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, 0, f.item)
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, "rq1", source.Source.Current.RequestID)
	require.Equal(t, f.run, source.Source.Current.RunID)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), source.ItemLog.ResultState)
	require.NoError(t, f.conn.Callback().Query().Remove("standard_source_no_record_read"))
	for _, query := range queries {
		require.NotContains(t, strings.ToUpper(query), "FOR UPDATE")
		require.NotContains(t, query, "`eval_target_record`")
		require.NotContains(t, query, "`evaluator_record`")
	}
	require.Equal(t, before, f.snapshot())
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", f.run+1).Error)
	source, err = port.GetStandardEvalOutputSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.Nil(t, source, "old run logs cannot revive moved canonical/latest source")
}

func TestSourceStandardMySQLSnapshotAcrossIsolation(t *testing.T) {
	for _, isolation := range []string{"REPEATABLE-READ", "READ-COMMITTED"} {
		t.Run(isolation, func(t *testing.T) {
			f := newCurrentResultFixture(t, "rq1")
			provider := newObservationProvider(t, isolation)
			conn := provider.NewSession(context.Background(), db.WithMaster())
			initial := make(chan struct{})
			resume := make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(resume) })
			locks := 0
			paused := false
			require.NoError(t, conn.Callback().Query().After("gorm:query").Register("standard_source_initial_snapshot", func(tx *gorm.DB) {
				if strings.Contains(tx.Statement.SQL.String(), "FOR UPDATE") {
					locks++
				}
				if tx.Statement.Table == "experiment" && !paused {
					paused = true
					close(initial)
					<-resume
				}
			}))
			defer func() { require.NoError(t, conn.Callback().Query().Remove("standard_source_initial_snapshot")) }()
			type observation struct {
				source *entity.ExptItemResultPublishSource
				err    error
			}
			done := make(chan observation, 1)
			go func() {
				source, err := NewExptResultPublishDAO(provider).(*ExptResultPublishDAOImpl).GetStandardEvalOutputSource(context.Background(), f.space, f.expt, 0, f.item)
				done <- observation{source, err}
			}()
			select {
			case <-initial:
			case <-time.After(5 * time.Second):
				t.Fatal("initial primary snapshot was not reached")
			}
			raw, _ := json.Marshal(map[string]string{entity.ManualRetryExtKey: string(mustStandardJSON(t, &entity.ManualRetryContext{RunID: f.run, RequestID: "rq2", Mode: entity.EvaluationModeFailRetry, Operator: "test", AcceptedAtMS: 1}))})
			require.NoError(t, f.provider.Transaction(context.Background(), func(tx *gorm.DB) error {
				if err := tx.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("ext", raw).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.ExptTurnResult{}).Where("space_id=? AND turn_id=0", f.space).Update("target_result_id", f.base+99).Error; err != nil {
					return err
				}
				return tx.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Update("target_result_id", f.base+99).Error
			}, db.WithMaster()))
			once.Do(func() { close(resume) })
			observed := <-done
			if isolation == "READ-COMMITTED" {
				require.Error(t, observed.err, "new request cannot enter an RC source observation")
				require.Nil(t, observed.source)
			} else {
				require.NoError(t, observed.err)
				require.NotNil(t, observed.source)
				require.Equal(t, "rq1", observed.source.Source.Current.RequestID)
				require.Equal(t, f.base+41, observed.source.Source.Turns[0].TargetResultID)
			}
			require.Zero(t, locks)
			provider.mu.Lock()
			options := append([]sql.TxOptions(nil), provider.options...)
			provider.mu.Unlock()
			require.Len(t, options, 1)
			require.Equal(t, sql.LevelDefault, options[0].Isolation)
			require.False(t, options[0].ReadOnly)
			t.Logf("REAL_MYSQL_STANDARD_SOURCE_SNAPSHOT session_default=%s default_begin=true semantic_read_only=true locks=%d stable_or_discard=true", isolation, locks)
		})
	}
}

func mustStandardJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

func TestSourceStandardMySQLSendBeforeCommitNotReady(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	locked := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	var once sync.Once
	defer once.Do(func() { close(release) })
	go func() {
		done <- f.provider.Transaction(context.Background(), func(tx *gorm.DB) error {
			if err := tx.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("result_state", int32(entity.ExptItemResultStateResulted)).Error; err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		}, db.WithMaster())
	}()
	<-locked
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	source, err := NewExptResultPublishDAO(f.provider).(*ExptResultPublishDAOImpl).GetStandardEvalOutputSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), source.ItemLog.ResultState, "MQ consumer before DB commit must observe Logged without blocking or stale acknowledgement")
	once.Do(func() { close(release) })
	require.NoError(t, <-done)
	source, err = NewExptResultPublishDAO(f.provider).(*ExptResultPublishDAOImpl).GetStandardEvalOutputSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), source.ItemLog.ResultState)
}
