// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Only Begin options are simulated; all data queries use the actual MySQL driver.
// This cannot certify production NDB acceptance.
func TestNDBObservationMySQLDefaultBegin(t *testing.T) {
	t.Run("scan", func(t *testing.T) {
		f := newCurrentResultFixture(t, "")
		provider := newObservationProvider(t, "READ-COMMITTED")
		provider.rejectExplicit = true
		require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Queueing)).Error)
		require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Queueing)).Error)
		before := f.snapshot()
		rows, err := NewExptItemResultDAO(provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
		require.NoError(t, err, "default-off legacy queue must not fail at NDB Begin")
		require.Len(t, rows, 3)
		for _, row := range rows {
			require.Equal(t, entity.ItemRunState_Queueing, row.Status)
			require.Empty(t, row.RequestID)
		}
		require.Equal(t, before, f.snapshot(), "observers must leave all eleven tables unchanged")
	})
}

type readonlyObservationProbe struct {
	writes, locks, recordLocks int
	queries                    []string
}

func observeReadonlySQL(t *testing.T, conn *gorm.DB) *readonlyObservationProbe {
	t.Helper()
	p := &readonlyObservationProbe{}
	write := func(*gorm.DB) { p.writes++ }
	require.NoError(t, conn.Callback().Create().Before("gorm:create").Register("observation_create", write))
	require.NoError(t, conn.Callback().Update().Before("gorm:update").Register("observation_update", write))
	require.NoError(t, conn.Callback().Delete().Before("gorm:delete").Register("observation_delete", write))
	require.NoError(t, conn.Callback().Raw().Before("gorm:raw").Register("observation_raw", func(tx *gorm.DB) {
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(tx.Statement.SQL.String())), "SELECT") {
			p.writes++
		}
	}))
	require.NoError(t, conn.Callback().Query().After("gorm:query").Register("observation_query", func(tx *gorm.DB) {
		query := strings.ToUpper(tx.Statement.SQL.String())
		p.queries = append(p.queries, query)
		if strings.Contains(query, "FOR UPDATE") || strings.Contains(query, "FOR SHARE") || strings.Contains(query, "LOCK IN SHARE") {
			p.locks++
		}
		var count int
		require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT COUNT(*) FROM performance_schema.data_locks WHERE LOCK_TYPE='RECORD' AND THREAD_ID=(SELECT THREAD_ID FROM performance_schema.threads WHERE PROCESSLIST_ID=CONNECTION_ID())").Scan(&count).Error)
		if count > p.recordLocks {
			p.recordLocks = count
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, conn.Callback().Create().Remove("observation_create"))
		require.NoError(t, conn.Callback().Update().Remove("observation_update"))
		require.NoError(t, conn.Callback().Delete().Remove("observation_delete"))
		require.NoError(t, conn.Callback().Raw().Remove("observation_raw"))
		require.NoError(t, conn.Callback().Query().Remove("observation_query"))
	})
	return p
}

func (p *readonlyObservationProbe) assert(t *testing.T, f *currentResultFixture, expected map[string]string) {
	t.Helper()
	require.Zero(t, p.writes, "observer emitted DML/session statements")
	require.Zero(t, p.locks, "observer used locking SELECT")
	require.Zero(t, p.recordLocks, "actual observer transaction acquired record locks")
	require.NotEmpty(t, p.queries)
	for _, query := range p.queries {
		require.True(t, strings.HasPrefix(query, "SELECT"), query)
		require.NotContains(t, query, "EVAL_TARGET_RECORD")
		require.NotContains(t, query, "EVALUATOR_RECORD")
	}
	require.Equal(t, expected, f.snapshot(), "observer changed nine source/accounting tables or two record tables")
	t.Logf("REAL_MYSQL_READONLY queries=%d writes=%d locking_selects=%d record_locks=%d snapshots=11 records_unchanged=true", len(p.queries), p.writes, p.locks, p.recordLocks)
}

func TestNDBObservationMySQLFullSourceChanges(t *testing.T) {
	changes := []string{"latest", "new_run", "raw_manual", "same_request_new_record", "registered", "inline", "physical_ref", "canonical_terminal", "runlog_processing", "run_terminal", "turn_status", "item_config", "raw_ext", "retry_cycle"}
	for _, isolation := range []string{"READ-COMMITTED", "REPEATABLE-READ"} {
		for _, change := range changes {
			t.Run(isolation+"/scan/"+change, func(t *testing.T) {
				f := newScanObservationFixture(t)
				if change == "raw_manual" {
					f.token("")
				}
				if change == "retry_cycle" {
					require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Processing)).Error)
					require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Processing), "result_state": int32(entity.ExptItemResultStateDefault)}).Error)
				}
				expectedRows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
				require.NoError(t, err)
				provider := newObservationProvider(t, isolation)
				conn := provider.NewSession(context.Background())
				probe := observeReadonlySQL(t, conn)
				expectedSnapshot := f.snapshot()
				var once sync.Once
				// End of the complete first collection: all old source values
				// already exist, before the inner observer queries start.
				require.NoError(t, conn.Callback().Query().After("observation_query").Register("observation_change", func(tx *gorm.DB) {
					if tx.Statement.Table != "expt_turn_evaluator_result_ref" {
						return
					}
					once.Do(func() {
						switch change {
						case "latest":
							require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", f.run+1).Error)
						case "new_run":
							require.NoError(t, f.provider.Transaction(context.Background(), func(writer *gorm.DB) error {
								if err := writer.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", f.run+1).Error; err != nil {
									return err
								}
								return writer.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("expt_run_id", f.run+1).Error
							}))
						case "raw_manual":
							f.token("rq2")
						case "same_request_new_record":
							require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Update("target_result_id", f.base+99).Error)
						case "registered", "inline":
							refs := &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 61}}, Inline: []*entity.InlineEvalResult{{InlineKey: "inline", RecordID: f.base + 62}}}
							if change == "registered" {
								refs.Registered[0].RecordID++
							} else {
								refs.Inline[0].RecordID++
							}
							raw, err := json.Marshal(refs)
							require.NoError(t, err)
							require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Update("evaluator_result_ids", raw).Error)
						case "physical_ref":
							require.NoError(t, f.conn.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=?", f.space).Update("evaluator_result_id", f.base+99).Error)
						case "canonical_terminal":
							require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Terminal)).Error)
						case "runlog_processing":
							require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Processing)).Error)
						case "run_terminal":
							require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
						case "turn_status":
							require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Update("status", int32(entity.TurnRunState_Terminal)).Error)
						case "item_config":
							require.NoError(t, f.conn.Model(&model.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_config", []byte(`{"target_source_space_id":99}`)).Error)
						case "raw_ext":
							require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("ext", []byte(`{"other":"changed"}`)).Error)
						case "retry_cycle":
							// Retry state returns to Processing, but monotonic retry
							// count remains evidence of the new observation generation.
							require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("retry_times", 2).Error)
						}
						expectedSnapshot = f.snapshot()
					})
				}))
				defer func() { require.NoError(t, conn.Callback().Query().Remove("observation_change")) }()
				rows, err := NewExptItemResultDAO(provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
				if isolation == "READ-COMMITTED" {
					require.Error(t, err)
					require.Nil(t, rows)
				} else {
					require.NoError(t, err)
					require.Equal(t, expectedRows, rows)
				}
				probe.assert(t, f, expectedSnapshot)
			})
		}
	}
}

// After its first value was observed, a config mutation must invalidate the
// complete RC observation rather than return any partially collected source.
func TestNDBObservationMySQLChangedSourceDiscardsAll(t *testing.T) {
	t.Run("scan", func(t *testing.T) {
		f := newScanObservationFixture(t)
		provider := newObservationProvider(t, "READ-COMMITTED")
		conn := provider.NewSession(context.Background())
		var once sync.Once
		require.NoError(t, conn.Callback().Query().After("gorm:query").Register("change_after_first_config", func(tx *gorm.DB) {
			if tx.Statement.Table == "expt_item_ref" {
				once.Do(func() {
					require.NoError(t, f.conn.Model(&model.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_config", []byte(`{"target_source_space_id":99}`)).Error)
				})
			}
		}))
		defer func() { require.NoError(t, conn.Callback().Query().Remove("change_after_first_config")) }()
		rows, err := NewExptItemResultDAO(provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
		require.Error(t, err, "RC change must fail closed without partial scan rows")
		require.Nil(t, rows)
	})
}

func TestNDBObservationMySQLStableFullSource(t *testing.T) {
	for _, isolation := range []string{"READ-COMMITTED", "REPEATABLE-READ"} {
		for _, kind := range []string{"logged", "resulted_finished", "raw_ref0_version0"} {
			t.Run(isolation+"/"+kind, func(t *testing.T) {
				f := newScanObservationFixture(t)
				if kind == "resulted_finished" {
					require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("result_state", int32(entity.ExptItemResultStateResulted)).Error)
					require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
					for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
						require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.TurnRunState_Success)).Error)
					}
				}
				if kind == "raw_ref0_version0" {
					f.token("")
					for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_item_ref", "eval_target_record", "evaluator_record"} {
						require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", 0).Error)
					}
					require.NoError(t, f.conn.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=?", f.space).Update("source_type", 0).Error)
				}
				expectedRows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, true)
				require.NoError(t, err)
				before := f.snapshot()
				provider := newObservationProvider(t, isolation)
				provider.rejectExplicit = true
				probe := observeReadonlySQL(t, provider.NewSession(context.Background()))
				rows, err := NewExptItemResultDAO(provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, true)
				require.NoError(t, err)
				require.Equal(t, expectedRows, rows)
				if kind == "raw_ref0_version0" {
					require.Len(t, rows, 3)
					require.NotNil(t, rows[0].Publication)
					require.Empty(t, rows[0].Publication.Source.Current.RequestID)
					require.Zero(t, rows[0].Publication.Source.Current.ItemVersionID)
					require.Zero(t, rows[0].Publication.Source.CurrentRefs[0].SourceType)
				}
				probe.assert(t, f, before)
			})
		}
	}
}

func TestNDBObservationMySQLOldLatestRunRejected(t *testing.T) {
	for _, isolation := range []string{"READ-COMMITTED", "REPEATABLE-READ"} {
		t.Run(isolation, func(t *testing.T) {
			f := newCurrentResultFixture(t, "")
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", f.run+1).Error)
			provider := newObservationProvider(t, isolation)
			before := f.snapshot()
			probe := observeReadonlySQL(t, provider.NewSession(context.Background()))
			rows, err := NewExptItemResultDAO(provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
			require.NoError(t, err)
			require.Empty(t, rows, "old queue/run logs must not dispatch after the authoritative latest run moved")
			probe.assert(t, f, before)
		})
	}
}
