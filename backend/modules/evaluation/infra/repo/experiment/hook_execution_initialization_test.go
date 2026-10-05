package experiment

import (
	"context"
	"fmt"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"testing"
)

type executionFixture struct {
	*hookTxFixture
	init      repo.IHookExecutionInitializationRepo
	key       entity.HookRunKey
	manifests []entity.HookExecutionManifest
	hash      string
}

func newExecutionFixture(t *testing.T, count int) *executionFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_EXECUTION_DSN")
	if dsn == "" {
		t.Skip("requires isolated HOOK_MYSQL_EXECUTION_DSN")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "hook_7378265404_execution", cfg.DBName)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	s := p.NewSession(context.Background())
	pool, err := s.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(12)
	f := &executionFixture{hookTxFixture: &hookTxFixture{p: p, sql: s, repo: NewHookRunRepo(p), space: hookTxSequence.Add(1), expt: hookTxSequence.Add(1)}, init: NewHookExecutionInitializationRepo(p)}
	t.Cleanup(func() {
		for _, table := range []string{"expt_item_result", "expt_turn_result", "expt_item_result_run_log", "expt_turn_result_run_log", "expt_stats"} {
			require.NoError(t, s.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
		}
		require.NoError(t, cleanupHookTxFixture(s, f.space, f.expt))
	})
	require.NoError(t, s.Create(&model.Experiment{ID: f.expt, SpaceID: f.space, Name: fmt.Sprint(f.expt), Status: int32(entity.ExptStatus_Pending), ExptType: 1, EvalSetID: 71}).Error)
	require.NoError(t, s.Create(&model.ExptStats{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, CreditCost: 7}).Error)
	in := f.input(true, 0)
	in.RunLog.Mode = int32(entity.EvaluationModeSubmit)
	in.RunLog.Status = int64(entity.ExptStatus_Pending)
	f.key = in.Key
	created, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	items := make([]entity.HookPlanItem, count)
	for i := range items {
		items[i] = entity.HookPlanItem{ID: hookTxSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: hookTxSequence.Add(1)}
		if i%2 == 1 {
			items[i].ItemVersionID = 42
		}
	}
	version := created.Run.Version
	if count > 0 {
		appended, err := f.repo.AppendPlanPage(context.Background(), entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: version}, Items: items, NextCursor: "done"})
		require.NoError(t, err)
		version = appended.Run.Version
	}
	digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), items)
	require.NoError(t, err)
	f.hash = digest.Hash
	_, err = f.repo.FinishPlan(context.Background(), entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: version}, Count: int64(count), Hash: f.hash})
	require.NoError(t, err)
	require.NoError(t, s.Model(&model.ExptLifecycleHookRun{}).Where("space_id=? AND expt_run_id=? AND phase='before'", f.space, f.key.RunID).Updates(map[string]any{"status": "succeeded", "attempt": 1}).Error)
	require.NoError(t, s.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Update("gate", 1).Error)
	for i, item := range items {
		f.manifests = append(f.manifests, entity.HookExecutionManifest{Version: 1, Key: f.key, Ordinal: int64(i), Frozen: item, ItemResultID: hookTxSequence.Add(1), ItemRunLogID: hookTxSequence.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, TurnIdx: 0, ResultID: hookTxSequence.Add(1)}, {TurnID: 10, TurnIdx: 1, ResultID: hookTxSequence.Add(1)}}, TurnLogsInitialized: gptr.Of(false)})
	}
	return f
}

func (f *executionFixture) readInput() entity.HookExecutionInitializationReadInput {
	return entity.HookExecutionInitializationReadInput{Key: f.key, ExecutionScope: "local", Limit: 100}
}
func (f *executionFixture) writeInput(v int64) entity.HookExecutionInitializationWriteInput {
	return entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: v}, ExecutionScope: "local", PlanHash: f.hash, Items: f.manifests}
}
func (f *executionFixture) completeInput(v int64) entity.HookExecutionInitializationCompleteInput {
	return entity.HookExecutionInitializationCompleteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: v}, ExecutionScope: "local", PlanHash: f.hash, ExpectedItemCount: int64(len(f.manifests)), ExpectedTurnCount: int64(len(f.manifests) * 2)}
}

func TestHookExecutionMySQLRoundTrip(t *testing.T) {
	f := newExecutionFixture(t, 2)
	ctx := context.Background()
	page, err := f.init.ReadExecutionInitializationPage(ctx, f.readInput())
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.Nil(t, page.Items[0].Manifest)
	written, err := f.init.WriteExecutionInitializationPage(ctx, f.writeInput(page.RunVersion))
	require.NoError(t, err)
	require.Equal(t, f.manifests[0], *written.Items[0].Manifest)
	require.Zero(t, written.Items[0].Manifest.Frozen.ItemVersionID)
	require.Equal(t, int64(42), written.Items[1].Manifest.Frozen.ItemVersionID)
	replay, err := f.init.WriteExecutionInitializationPage(ctx, f.writeInput(page.RunVersion))
	require.NoError(t, err)
	require.Equal(t, written, replay)
	done, err := f.init.CompleteExecutionInitialization(ctx, f.completeInput(written.RunVersion))
	require.NoError(t, err)
	require.True(t, done.Initialized)
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(2), stats.PendingCnt)
	require.Equal(t, float64(7), stats.CreditCost)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("id=?", stats.ID).Update("pending_cnt", 1).Error)
	done2, err := f.init.CompleteExecutionInitialization(ctx, f.completeInput(written.RunVersion))
	require.NoError(t, err)
	require.False(t, done2.Changed)
	require.NoError(t, f.sql.First(&stats, stats.ID).Error)
	require.Equal(t, int32(1), stats.PendingCnt)
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
	require.Equal(t, int64(entity.ExptStatus_Processing), gptr.Indirect(log.Status))
}
