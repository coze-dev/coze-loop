// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func newScanObservationFixture(t *testing.T) *currentResultFixture {
	t.Helper()
	f := newCurrentResultFixture(t, "rq1")
	for _, item := range []int64{f.item, f.item + 1} {
		require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, item).Update("status", int32(entity.ItemRunState_Success)).Error)
		require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	}
	require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 450, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Success)}).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 550, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Success)}).Error)
	return f
}

func observationQueryHasItem(tx *gorm.DB, itemID int64) bool {
	where, ok := tx.Statement.Clauses["WHERE"].Expression.(clause.Where)
	if !ok {
		return false
	}
	for _, condition := range where.Exprs {
		if expr, ok := condition.(clause.Expr); ok {
			for _, value := range expr.Vars {
				if value == itemID {
					return true
				}
			}
		}
	}
	return false
}

// A scanner blocked on B must not retain A's canonical/source locks.
// This uses a real separate SQL transaction holding B, and a real UPDATE to A.
func TestScanObservationMySQLDoesNotAccumulateLocks(t *testing.T) {
	f := newScanObservationFixture(t)
	ctx := context.Background()
	blocked := make(chan struct{})
	release := make(chan struct{})
	blockerDone := make(chan error, 1)
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	go func() {
		blockerDone <- f.provider.Transaction(ctx, func(tx *gorm.DB) error {
			var b model.ExptItemResult
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item+1).Take(&b).Error; err != nil {
				return err
			}
			close(blocked)
			<-release
			return nil
		}, db.WithMaster())
	}()
	select {
	case <-blocked:
	case err := <-blockerDone:
		t.Fatalf("SQL blocker failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("SQL blocker not reached")
	}
	secondItem := make(chan struct{}, 1)
	locks := 0
	require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register("scan_probe_second_item", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result" && observationQueryHasItem(tx, f.item+1) {
			select {
			case secondItem <- struct{}{}:
			default:
			}
		}
	}))
	require.NoError(t, f.conn.Callback().Query().After("gorm:query").Register("scan_count_source_locks", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "FOR UPDATE") {
			locks++
		}
	}))
	defer func() {
		require.NoError(t, f.conn.Callback().Query().Remove("scan_probe_second_item"))
		require.NoError(t, f.conn.Callback().Query().Remove("scan_count_source_locks"))
	}()
	type scanResult struct {
		rows []*entity.ExptItemCurrentRequest
		err  error
	}
	scanned := make(chan scanResult, 1)
	go func() {
		rows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(ctx, f.space, f.expt, f.run, false)
		scanned <- scanResult{rows, err}
	}()
	select {
	case <-secondItem:
	case <-time.After(5 * time.Second):
		unblock()
		t.Fatal("scanner did not reach second Logged item")
	}
	updateCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	updateErr := f.provider.NewSession(updateCtx, db.WithMaster()).Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Update("log_id", "independent-current-write").Error
	cancel()
	unblock()
	require.NoError(t, <-blockerDone)
	result := <-scanned
	require.NoError(t, result.err)
	require.NoError(t, updateErr, "a blocked scan must not hold previously observed Logged item rows until the entire scan completes")
	require.Len(t, result.rows, 3, "all Logged and Queueing items must remain enumerated")
	require.Zero(t, locks, "source observation must not emit locking SELECTs")
	for _, row := range result.rows {
		if row.ResultState == entity.ExptItemResultStateLogged {
			require.True(t, row.PublicationObserved)
			require.NotNil(t, row.Publication)
		}
	}
	t.Logf("REAL_MYSQL_SCAN_OBSERVATION locks=%d logged_items=2 blocked_second_item_did_not_block_first_update", locks)
}

// The test wrapper records the real BeginTx options while forwarding to MySQL.
// The concurrency assertions below also prove the observed data is one snapshot.
type observationProvider struct {
	base    db.Provider
	mu      sync.Mutex
	options []sql.TxOptions
}
type observationConnPool struct {
	*sql.DB
	owner *observationProvider
}

func (p *observationConnPool) BeginTx(ctx context.Context, options *sql.TxOptions) (*sql.Tx, error) {
	option := sql.TxOptions{}
	if options != nil {
		option = *options
	}
	p.owner.mu.Lock()
	p.owner.options = append(p.owner.options, option)
	p.owner.mu.Unlock()
	return p.DB.BeginTx(ctx, options)
}
func (p *observationProvider) NewSession(ctx context.Context, options ...db.Option) *gorm.DB {
	session := p.base.NewSession(ctx, options...)
	if pool, ok := session.Statement.ConnPool.(*sql.DB); ok {
		session.Statement.ConnPool = &observationConnPool{DB: pool, owner: p}
	}
	return session
}
func (p *observationProvider) Transaction(ctx context.Context, callback func(*gorm.DB) error, options ...db.Option) error {
	return p.NewSession(ctx, options...).Transaction(callback)
}

func newObservationProvider(t *testing.T, isolation string) *observationProvider {
	t.Helper()
	cfg, err := drivermysql.ParseDSN(os.Getenv("EXPT_RETRY_MYSQL_DSN"))
	require.NoError(t, err)
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	// The override is connection-local on this test-owned pool, never global.
	cfg.Params["transaction_isolation"] = "'" + isolation + "'"
	base, err := db.NewDB(gormmysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := base.NewSession(context.Background(), db.WithMaster())
	t.Cleanup(func() { pool, err := conn.DB(); require.NoError(t, err); require.NoError(t, pool.Close()) })
	var observed string
	require.NoError(t, conn.Raw("SELECT @@session.transaction_isolation").Scan(&observed).Error)
	require.Equal(t, isolation, observed)
	var engine string
	require.NoError(t, conn.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='expt_item_result'").Scan(&engine).Error)
	require.Equal(t, "InnoDB", engine)
	t.Logf("REAL_MYSQL_ISOLATION session_default=%s engine=%s", observed, engine)
	return &observationProvider{base: base}
}

// A concurrent writer commits a new invocation after the initial run/item scan.
// Even a READ-COMMITTED connection must return every part of the old snapshot.
func TestScanObservationMySQLConsistentAcrossSessionIsolation(t *testing.T) {
	for _, isolation := range []string{"REPEATABLE-READ", "READ-COMMITTED"} {
		t.Run(isolation, func(t *testing.T) {
			f := newScanObservationFixture(t)
			port := NewExptResultPublishDAO(f.provider)
			expected, err := port.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			provider := newObservationProvider(t, isolation)
			conn := provider.NewSession(context.Background(), db.WithMaster())
			joined := make(chan struct{})
			resume := make(chan struct{})
			var once sync.Once
			defer func() { once.Do(func() { close(resume) }) }()
			locks := 0
			var txMetadata struct {
				IsolationLevel string
				AccessMode     string
			}
			var metadataErr error
			require.NoError(t, conn.Callback().Query().After("gorm:query").Register("scan_pause_after_initial_join", func(tx *gorm.DB) {
				query := tx.Statement.SQL.String()
				if strings.Contains(query, "FOR UPDATE") {
					locks++
				}
				if strings.Contains(query, "AS l") && strings.Contains(query, "LEFT JOIN") {
					metadataErr = tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT et.ISOLATION_LEVEL AS isolation_level,et.ACCESS_MODE AS access_mode FROM performance_schema.events_transactions_current AS et JOIN performance_schema.threads AS th ON th.THREAD_ID=et.THREAD_ID WHERE th.PROCESSLIST_ID=CONNECTION_ID() LIMIT 1").Scan(&txMetadata).Error
					close(joined)
					<-resume
				}
			}))
			defer func() { require.NoError(t, conn.Callback().Query().Remove("scan_pause_after_initial_join")) }()
			type scanResult struct {
				rows []*entity.ExptItemCurrentRequest
				err  error
			}
			done := make(chan scanResult, 1)
			go func() {
				rows, err := NewExptItemResultDAO(provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
				done <- scanResult{rows, err}
			}()
			select {
			case <-joined:
			case <-time.After(5 * time.Second):
				t.Fatal("initial consistent scan not reached")
			}
			writeErr := f.provider.Transaction(context.Background(), func(tx *gorm.DB) error {
				if err := tx.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("log_id", "new-invocation-item").Error; err != nil {
					return err
				}
				if err := tx.Model(&model.ExptTurnResult{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Updates(map[string]any{"target_result_id": f.base + 71, "log_id": "new-invocation-turn"}).Error; err != nil {
					return err
				}
				refs, _ := json.Marshal(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 81}}})
				if err := tx.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Updates(map[string]any{"target_result_id": f.base + 71, "evaluator_result_ids": refs, "log_id": "new-invocation-log"}).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=? AND expt_turn_result_id=?", f.space, f.base+400).Update("evaluator_result_id", f.base+81).Error; err != nil {
					return err
				}
				return tx.Model(&model.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_config", []byte(`{"target_source_space_id":99}`)).Error
			}, db.WithMaster())
			once.Do(func() { close(resume) })
			result := <-done
			require.NoError(t, writeErr)
			require.NoError(t, result.err)
			require.Len(t, result.rows, 3)
			var observed *entity.ExptItemResultPublishSource
			for _, row := range result.rows {
				if row.ItemID == f.item {
					observed = row.Publication
				}
			}
			require.Equal(t, expected, observed, "initial request/turn/ref/config observation must not be replaced by a new same-request invocation")
			provider.mu.Lock()
			options := append([]sql.TxOptions(nil), provider.options...)
			provider.mu.Unlock()
			require.Len(t, options, 1)
			require.Equal(t, sql.LevelRepeatableRead, options[0].Isolation, "snapshot isolation must be explicit even on READ-COMMITTED connections")
			require.True(t, options[0].ReadOnly)
			require.NoError(t, metadataErr)
			require.Equal(t, "REPEATABLE READ", txMetadata.IsolationLevel)
			require.Equal(t, "READ ONLY", txMetadata.AccessMode)
			require.Zero(t, locks)
			var unchangedDefault string
			require.NoError(t, provider.NewSession(context.Background(), db.WithMaster()).Raw("SELECT @@session.transaction_isolation").Scan(&unchangedDefault).Error)
			require.Equal(t, isolation, unchangedDefault, "one-transaction isolation must not change connection defaults")
			t.Logf("REAL_MYSQL_SCAN_SNAPSHOT session_default=%s BeginTx=%+v performance_schema=%+v locking_queries=%d", isolation, options[0], txMetadata, locks)
			current, err := port.IsItemResultPublishSourceCurrent(context.Background(), observed)
			require.NoError(t, err)
			require.False(t, current, "notification validation rejects the old frozen source after the writer committed")
			before := f.snapshot()
			applied, err := port.CommitItemResult(context.Background(), &entity.ExptItemResultPublication{Source: observed})
			require.NoError(t, err)
			require.False(t, applied)
			require.Equal(t, before, f.snapshot(), "old scan cannot publish against new references")
		})
	}
}

// Removing the scan locks must not remove the final validation/commit locks.
// Each operation waits on an actual canonical FOR UPDATE held by a writer,
// and makes no writes once the writer commits a different original source.
func TestScanObservationMySQLPublicationStillLocks(t *testing.T) {
	for _, operation := range []string{"validate", "commit"} {
		t.Run(operation, func(t *testing.T) {
			f := newScanObservationFixture(t)
			ctx := context.Background()
			port := NewExptResultPublishDAO(f.provider)
			source, err := port.GetItemResultPublishSource(ctx, f.space, f.expt, f.run, f.item)
			require.NoError(t, err)
			ready := make(chan struct{})
			release := make(chan struct{})
			writerDone := make(chan error, 1)
			var once sync.Once
			defer func() { once.Do(func() { close(release) }) }()
			go func() {
				writerDone <- f.provider.Transaction(ctx, func(tx *gorm.DB) error {
					var row model.ExptItemResult
					if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Take(&row).Error; err != nil {
						return err
					}
					if err := tx.Model(&row).Update("log_id", "changed-under-canonical-lock").Error; err != nil {
						return err
					}
					close(ready)
					<-release
					return nil
				}, db.WithMaster())
			}()
			select {
			case <-ready:
			case err := <-writerDone:
				t.Fatalf("writer failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("writer did not acquire canonical row")
			}
			started := make(chan struct{}, 1)
			require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register("publication_probe_lock", func(tx *gorm.DB) {
				if tx.Statement.Table == "expt_item_result" && observationQueryHasItem(tx, f.item) {
					locking, ok := tx.Statement.Clauses["FOR"].Expression.(clause.Locking)
					if ok && locking.Strength == "UPDATE" {
						select {
						case started <- struct{}{}:
						default:
						}
					}
				}
			}))
			defer func() { require.NoError(t, f.conn.Callback().Query().Remove("publication_probe_lock")) }()
			type result struct {
				applied bool
				err     error
			}
			done := make(chan result, 1)
			go func() {
				var ok bool
				var err error
				if operation == "validate" {
					ok, err = port.IsItemResultPublishSourceCurrent(ctx, source)
				} else {
					ok, err = port.CommitItemResult(ctx, &entity.ExptItemResultPublication{Source: source})
				}
				done <- result{ok, err}
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("publication did not attempt canonical FOR UPDATE")
			}
			select {
			case result := <-done:
				t.Fatalf("%s bypassed writer row lock: %+v", operation, result)
			case <-time.After(40 * time.Millisecond):
			}
			once.Do(func() { close(release) })
			require.NoError(t, <-writerDone)
			resultValue := <-done
			require.NoError(t, resultValue.err)
			require.False(t, resultValue.applied)
			before := f.snapshot()
			applied, err := port.CommitItemResult(ctx, &entity.ExptItemResultPublication{Source: source})
			require.NoError(t, err)
			require.False(t, applied)
			require.Equal(t, before, f.snapshot())
			t.Logf("REAL_MYSQL_PUBLICATION_LOCK operation=%s waited_on_canonical_FOR_UPDATE=true stale_writes=0", operation)
		})
	}
}

func TestScanObservationMySQLProcessingSharesSnapshot(t *testing.T) {
	f := newScanObservationFixture(t)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Processing)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Processing), "result_state": int32(entity.ExptItemResultStateDefault)}).Error)
	expected, err := f.port.GetCurrentItemRunSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	provider := newObservationProvider(t, "READ-COMMITTED")
	conn := provider.NewSession(context.Background(), db.WithMaster())
	scannedInitial := make(chan struct{})
	resume := make(chan struct{})
	var once sync.Once
	defer func() { once.Do(func() { close(resume) }) }()
	locks := 0
	require.NoError(t, conn.Callback().Query().After("gorm:query").Register("processing_snapshot_pause", func(tx *gorm.DB) {
		text := tx.Statement.SQL.String()
		if strings.Contains(text, "FOR UPDATE") {
			locks++
		}
		if strings.Contains(text, "AS l") && strings.Contains(text, "LEFT JOIN") {
			close(scannedInitial)
			<-resume
		}
	}))
	defer func() { require.NoError(t, conn.Callback().Query().Remove("processing_snapshot_pause")) }()
	type result struct {
		rows []*entity.ExptItemCurrentRequest
		err  error
	}
	done := make(chan result, 1)
	go func() {
		rows, err := NewExptItemResultDAO(provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
		done <- result{rows, err}
	}()
	select {
	case <-scannedInitial:
	case <-time.After(5 * time.Second):
		t.Fatal("initial Processing scan not reached")
	}
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Update("target_result_id", f.base+71).Error)
	once.Do(func() { close(resume) })
	observed := <-done
	require.NoError(t, observed.err)
	require.Len(t, observed.rows, 3)
	for _, row := range observed.rows {
		if row.ItemID == f.item {
			require.Equal(t, expected, row.Source)
			require.Equal(t, entity.ItemRunState_Processing, row.Status)
		}
		if row.ItemID == f.item+1 {
			require.True(t, row.PublicationObserved)
			require.NotNil(t, row.Publication)
		}
	}
	require.Zero(t, locks)
	t.Log("REAL_MYSQL_PROCESSING_SNAPSHOT old_source_preserved=true mixed_logged_processing_queueing=3 locking_queries=0")
}
