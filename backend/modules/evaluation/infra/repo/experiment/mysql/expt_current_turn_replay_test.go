// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type turnReplayProvider struct {
	db.Provider
	now time.Time
}

func TestCurrentTurnMySQLZeroRowsRequiresPersistedWrite(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "unapplied", true: "missing"}[missing], func(t *testing.T) {
			f := newCurrentResultFixture(t, "rq1")
			source := f.source()
			result := *source.Turns[0]
			result.LogID = "must-be-persisted"
			update := f.conn.Callback().Update().Get("gorm:update")
			require.NoError(t, f.conn.Callback().Update().Replace("gorm:update", func(tx *gorm.DB) {
				if tx.Statement.Table != "expt_turn_result_run_log" {
					update(tx)
					return
				}
				if missing {
					tx.AddError(tx.Exec("DELETE FROM expt_turn_result_run_log WHERE id=?", result.ID).Error)
				}
				tx.RowsAffected = 0
			}))
			defer func() { require.NoError(t, f.conn.Callback().Update().Replace("gorm:update", update)) }()
			before := f.snapshot()
			applied, err := f.port.StoreCurrentTurnRunResult(context.Background(), source, &result)
			require.Error(t, err)
			require.False(t, applied)
			require.Equal(t, before, f.snapshot(), "failed verification rolls back the transaction")
		})
	}
}

func TestCurrentTurnMySQLCompetingMarkerHasOneWinner(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	pending, err := json.Marshal(map[string]string{entity.ResourceTurnStartExtKey: "pending"})
	require.NoError(t, err)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.source().Turns[0].ID).
		Update("ext", pending).Error)
	source := f.source()
	var wg sync.WaitGroup
	ready := make(chan struct{})
	applied := make([]bool, 2)
	errs := make([]error, 2)
	for i := range applied {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result := *source.Turns[0]
			result.Ext = map[string]string{entity.ResourceTurnStartExtKey: "claimed"}
			<-ready
			applied[i], errs[i] = f.port.StoreCurrentTurnRunResult(context.Background(), source, &result)
		}(i)
	}
	close(ready)
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.NotEqual(t, applied[0], applied[1], "stale pending source must lose before no-op verification")
	before := f.snapshot()
	result := *source.Turns[0]
	result.Ext = map[string]string{entity.ResourceTurnStartExtKey: "claimed"}
	ok, err := f.port.StoreCurrentTurnRunResult(context.Background(), source, &result)
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, before, f.snapshot())
}

func (p turnReplayProvider) NewSession(ctx context.Context, opts ...db.Option) *gorm.DB {
	session := p.Provider.NewSession(ctx, opts...)
	session.Config.NowFunc = func() time.Time { return p.now }
	return session
}
func (p turnReplayProvider) Transaction(ctx context.Context, callback func(*gorm.DB) error, opts ...db.Option) error {
	return p.NewSession(ctx, opts...).Transaction(callback)
}

func TestCurrentTurnMySQLNoopReplayPreservesLockedSource(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	provider := turnReplayProvider{Provider: f.provider, now: time.Now().UTC().Truncate(time.Second)}
	port := NewExptItemResultDAO(provider).(repo.IExptCurrentResultRepo)
	var affected []int64
	require.NoError(t, f.conn.Callback().Update().After("gorm:update").Register("turn_replay_affected", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_turn_result_run_log" {
			affected = append(affected, tx.RowsAffected)
		}
	}))
	defer func() { require.NoError(t, f.conn.Callback().Update().Remove("turn_replay_affected")) }()
	ctx := context.Background()
	source, err := port.GetCurrentItemRunSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	result := *source.Turns[0]
	result.LogID = "stable-noop-replay"
	applied, err := port.StoreCurrentTurnRunResult(ctx, source, &result)
	t.Logf("REAL_MYSQL_TURN_FIRST_WRITE affected=%v applied=%t err=%v", affected, applied, err)
	require.NoError(t, err)
	require.True(t, applied)
	source, err = port.GetCurrentItemRunSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	result = *source.Turns[0]
	before := f.snapshot()
	applied, err = port.StoreCurrentTurnRunResult(ctx, source, &result)
	t.Logf("REAL_MYSQL_TURN_NOOP affected=%v applied=%t err=%v fixed_clock=%s", affected, applied, err, provider.now.Format(time.RFC3339))
	require.NoError(t, err, "a same-source duplicate update must not be mistaken for a missing turn")
	require.True(t, applied)
	require.Equal(t, []int64{1, 0}, affected, "exercise real changed-row semantics without changing driver flags")
	require.Equal(t, before, f.snapshot())
}
