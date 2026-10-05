// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var hookTxSequence = func() *atomic.Int64 { v := new(atomic.Int64); v.Store(time.Now().UnixNano() / 1000); return v }()

type hookTxFixture struct {
	p           db.Provider
	sql         *gorm.DB
	repo        repo.IHookRepo
	space, expt int64
}

func newHookTxFixture(t *testing.T) *hookTxFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires HOOK_MYSQL_TX_DSN real MySQL lock tests")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	s := p.NewSession(context.Background())
	sqlDB, err := s.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(12)
	f := &hookTxFixture{p: p, sql: s, repo: NewHookRunRepo(p), space: hookTxSequence.Add(1), expt: hookTxSequence.Add(1)}
	t.Cleanup(func() {
		require.NoError(t, cleanupHookTxFixture(s, f.space, f.expt))
	})
	require.NoError(t, s.Create(&model.Experiment{ID: f.expt, SpaceID: f.space, Name: fmt.Sprint(f.expt), Status: 3}).Error)
	return f
}

func (f *hookTxFixture) input(before bool, expected int64) entity.HookCreateRunInput {
	run := hookTxSequence.Add(1)
	in := entity.HookCreateRunInput{Key: entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: run},
		RunLog: &entity.ExptRunLog{ID: run, SpaceID: f.space, ExptID: f.expt, ExptRunID: run, CreatedBy: "user", Mode: 3, Status: 3}, ExpectedLatestRunID: expected,
		Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1, 7, 3}, KeyID: "key", Hash: strings.Repeat("a", 64), ExecutionScope: "local"},
		After:    &entity.HookOperationSeed{ID: hookTxSequence.Add(1), OperationID: fmt.Sprintf("hook_after_%d", run), IdempotencyKey: fmt.Sprintf("after_key_%d", run)}}
	if before {
		in.Before = &entity.HookOperationSeed{ID: hookTxSequence.Add(1), OperationID: fmt.Sprintf("hook_before_%d", run), IdempotencyKey: fmt.Sprintf("before_key_%d", run)}
	}
	return in
}

func (f *hookTxFixture) latest(t *testing.T) int64 {
	t.Helper()
	var id int64
	require.NoError(t, f.sql.Raw("SELECT latest_run_id FROM experiment WHERE id=? AND space_id=?", f.expt, f.space).Scan(&id).Error)
	return id
}

func TestHookTxCreateReplayAndOwnership(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	in := f.input(true, 0)
	got, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	require.True(t, got.Changed)
	require.Equal(t, in.Key.RunID, f.latest(t))
	require.Equal(t, entity.HookGateWaiting, got.Run.State.Gate)
	require.False(t, got.Run.PlanReady)
	require.Equal(t, int64(0), got.Run.PlanCount)
	require.Equal(t, int64(0), got.Run.Version)
	require.Len(t, got.Run.Operations, 2)
	for _, op := range got.Run.Operations {
		require.Nil(t, op.ActivatedAt)
		require.Nil(t, op.NextAttemptAt)
	}
	var marker *int32
	require.NoError(t, f.sql.Raw("SELECT lifecycle_hook_version FROM expt_run_log WHERE space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Scan(&marker).Error)
	require.Equal(t, gptr.Of(int32(1)), marker)
	read, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, got.Run, read)
	next := f.input(false, in.Key.RunID)
	_, err = f.repo.CreateRunWithHooks(ctx, next)
	require.NoError(t, err)
	replay := in
	newIDs := f.input(true, 0)
	replay.Before = newIDs.Before
	replay.After = newIDs.After
	replay.Snapshot.Cipher = []byte{9, 8, 7}
	again, err := f.repo.CreateRunWithHooks(ctx, replay)
	require.NoError(t, err)
	require.False(t, again.Changed)
	require.Equal(t, got.Run, again.Run)
	require.Equal(t, next.Key.RunID, f.latest(t))
	replay.Snapshot.Hash = strings.Repeat("b", 64)
	_, err = f.repo.CreateRunWithHooks(ctx, replay)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	wrong := in.Key
	wrong.WorkspaceID++
	_, err = f.repo.GetRun(ctx, wrong)
	require.Error(t, err)
}

func TestHookTxCreateRollback(t *testing.T) {
	for _, table := range []string{"expt_run_log", "expt_lifecycle_run", "expt_lifecycle_hook_run", "experiment"} {
		t.Run(table, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			trigger := fmt.Sprintf("hook_tx_fail_%d", f.expt)
			event, column := "INSERT", "expt_id"
			if table == "experiment" {
				event, column = "UPDATE", "id"
			}
			require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE %s ON %s FOR EACH ROW BEGIN IF NEW.%s=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='hook injected failure'; END IF; END", trigger, event, table, column, f.expt)).Error)
			t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error) })
			result, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.ErrorContains(t, err, "hook injected failure")
			require.Nil(t, result.Run)
			require.Equal(t, int64(0), f.latest(t))
			for _, name := range []string{"expt_run_log", "expt_lifecycle_run", "expt_lifecycle_hook_run"} {
				var count int64
				require.NoError(t, f.sql.Table(name).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
				require.Zero(t, count)
			}
		})
	}
}

func TestHookTxConcurrentCreate(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			f := newHookTxFixture(t)
			a, b := f.input(true, 0), f.input(true, 0)
			if same {
				b = a
			}
			var results [2]entity.HookStoreResult
			var errs [2]error
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i, in := range []entity.HookCreateRunInput{a, b} {
				wg.Add(1)
				go func(i int, in entity.HookCreateRunInput) {
					defer wg.Done()
					<-start
					results[i], errs[i] = f.repo.CreateRunWithHooks(context.Background(), in)
				}(i, in)
			}
			close(start)
			wg.Wait()
			changed := 0
			for i := range results {
				if results[i].Changed {
					changed++
				}
				if same {
					require.NoError(t, errs[i])
				} else if errs[i] != nil {
					require.ErrorIs(t, errs[i], entity.ErrHookStoreConflict)
				}
			}
			require.Equal(t, 1, changed)
			var count int64
			require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
			require.Equal(t, int64(1), count)
		})
	}
}

func TestHookTxCreateRejectsPartialState(t *testing.T) {
	f := newHookTxFixture(t)
	in := f.input(true, 0)
	ctx := context.Background()
	_, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	require.NoError(t, f.sql.Delete(&model.ExptLifecycleHookRun{}, "space_id=? AND expt_id=? AND expt_run_id=? AND phase='before'", f.space, f.expt, in.Key.RunID).Error)
	_, err = f.repo.CreateRunWithHooks(ctx, in)
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
	require.Equal(t, int64(1), count)
}
