package experiment

import (
	"context"
	"errors"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func executionCount(t *testing.T, f *executionFixture, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&n).Error)
	return n
}
func executionRead(t *testing.T, f *executionFixture) *entity.HookExecutionInitializationPage {
	t.Helper()
	v, err := f.init.ReadExecutionInitializationPage(context.Background(), f.readInput())
	require.NoError(t, err)
	return v
}
func executionWrite(t *testing.T, f *executionFixture) *entity.HookExecutionInitializationPage {
	t.Helper()
	p := executionRead(t, f)
	out, err := f.init.WriteExecutionInitializationPage(context.Background(), f.writeInput(p.RunVersion))
	require.NoError(t, err)
	return out
}
func executionFlag(t *testing.T, f *executionFixture) bool {
	t.Helper()
	var flag bool
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Pluck("execution_initialized", &flag).Error)
	return flag
}

func TestHookExecutionMySQLPageRollback(t *testing.T) {
	for _, table := range []string{"expt_item_result", "expt_turn_result", "expt_item_result_run_log", "expt_lifecycle_run_item", "expt_lifecycle_run"} {
		t.Run(table, func(t *testing.T) {
			f := newExecutionFixture(t, 2)
			page := executionRead(t, f)
			name := "execution_fail_" + table
			callback := func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					tx.AddError(errors.New("PRIVATE_INJECTED_FAILURE"))
				}
			}
			if strings.HasPrefix(table, "expt_lifecycle") {
				require.NoError(t, f.sql.Callback().Update().After("gorm:update").Register(name, callback))
				defer f.sql.Callback().Update().Remove(name)
			} else {
				require.NoError(t, f.sql.Callback().Create().After("gorm:create").Register(name, callback))
				defer f.sql.Callback().Create().Remove(name)
			}
			out, err := f.init.WriteExecutionInitializationPage(context.Background(), f.writeInput(page.RunVersion))
			require.ErrorIs(t, err, entity.ErrHookExecutionStorage)
			require.Nil(t, out)
			require.NotContains(t, err.Error(), "PRIVATE")
			for _, table := range []string{"expt_item_result", "expt_turn_result", "expt_item_result_run_log"} {
				require.Zero(t, executionCount(t, f, table))
			}
			reread := executionRead(t, f)
			require.Equal(t, page.RunVersion, reread.RunVersion)
			require.Nil(t, reread.Items[0].Manifest)
			require.Nil(t, reread.Items[1].Manifest)
		})
	}
}

func TestHookExecutionMySQLGuardsAndUnknownRows(t *testing.T) {
	for _, kind := range []string{"scope", "latest", "version", "hash", "foreign_result", "soft_deleted", "unknown_same_run", "missing_manifest", "wrong_manifest", "missing_turn", "executed", "extra_turn", "extra_item", "turn_count", "missing_stats"} {
		t.Run(kind, func(t *testing.T) {
			f := newExecutionFixture(t, 1)
			ctx := context.Background()
			page := executionRead(t, f)
			in := f.writeInput(page.RunVersion)
			switch kind {
			case "scope":
				in.ExecutionScope = "elsewhere"
			case "latest":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).Update("latest_run_id", f.key.RunID+1).Error)
			case "version":
				in.ExpectedVersion++
			case "hash":
				in.PlanHash = strings.Repeat("f", 64)
			case "foreign_result", "soft_deleted", "unknown_same_run":
				m := f.manifests[0]
				run := f.key.RunID
				if kind == "foreign_result" {
					run++
				}
				row := model.ExptItemResult{ID: m.ItemResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: run, ItemID: m.Frozen.ItemID}
				require.NoError(t, f.sql.Create(&row).Error)
				if kind == "soft_deleted" {
					require.NoError(t, f.sql.Delete(&row).Error)
				}
			default:
				page = executionWrite(t, f)
				done := f.completeInput(page.RunVersion)
				m := f.manifests[0]
				switch kind {
				case "missing_manifest":
					require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).Update("execution_manifest", nil).Error)
				case "wrong_manifest":
					require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).Update("execution_manifest", []byte(`{"version":99}`)).Error)
				case "missing_turn":
					require.NoError(t, f.sql.Unscoped().Delete(&model.ExptTurnResult{}, "id=? AND space_id=?", m.Turns[0].ResultID, f.space).Error)
				case "executed":
					require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).Update("status", int32(entity.ItemRunState_Processing)).Error)
				case "extra_turn":
					require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, TurnID: 99}).Error)
				case "extra_item":
					require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: 999}).Error)
				case "turn_count":
					done.ExpectedTurnCount--
				case "missing_stats":
					require.NoError(t, f.sql.Unscoped().Delete(&model.ExptStats{}, "space_id=? AND expt_id=?", f.space, f.expt).Error)
				}
				_, err := f.init.CompleteExecutionInitialization(ctx, done)
				require.Error(t, err)
				require.False(t, executionFlag(t, f))
				return
			}
			_, err := f.init.WriteExecutionInitializationPage(ctx, in)
			require.Error(t, err)
			require.False(t, executionFlag(t, f))
		})
	}
}

func TestHookExecutionMySQLConcurrentReplay(t *testing.T) {
	f := newExecutionFixture(t, 1)
	page := executionRead(t, f)
	in := f.writeInput(page.RunVersion)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.init.WriteExecutionInitializationPage(context.Background(), in)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int64(1), executionCount(t, f, "expt_item_result"))
	require.Equal(t, int64(2), executionCount(t, f, "expt_turn_result"))
	fresh := executionRead(t, f)
	require.Equal(t, page.RunVersion+1, fresh.RunVersion)
	bad := in
	bad.Items = append([]entity.HookExecutionManifest(nil), in.Items...)
	bad.Items[0] = bad.Items[0].Clone()
	bad.Items[0].Turns[0].TurnID = 5
	_, err := f.init.WriteExecutionInitializationPage(context.Background(), bad)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	_, err = f.init.CompleteExecutionInitialization(context.Background(), f.completeInput(fresh.RunVersion))
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", in.Items[0].ItemRunLogID).Updates(map[string]any{"status": int32(entity.ItemRunState_Processing), "retry_times": 2}).Error)
	reread := executionRead(t, f)
	require.True(t, reread.Initialized)
	require.Equal(t, in.Items[0], *reread.Items[0].Manifest)
	_, err = f.init.WriteExecutionInitializationPage(context.Background(), in)
	require.NoError(t, err)
	var row model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&row, in.Items[0].ItemRunLogID).Error)
	require.Equal(t, int32(2), row.RetryTimes)
}

func TestHookExecutionMySQLEmptyTrialAndUnsupported(t *testing.T) {
	for _, kind := range []string{"zero", "trial_single1", "after_only", "multi_set", "online", "retry", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			f := newExecutionFixture(t, 0)
			ctx := context.Background()
			switch kind {
			case "trial_single1":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).Update("mode", int32(entity.EvaluationModeTrialRun)).Error)
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).Update("eval_set_source_type", 1).Error)
			case "after_only":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Update("before_enabled", false).Error)
				require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=? AND phase='before'", f.space, f.key.RunID).Delete(&model.ExptLifecycleHookRun{}).Error)
			case "multi_set":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).Update("eval_set_source_type", 2).Error)
			case "online":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).Update("expt_type", 2).Error)
			case "retry":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).Update("mode", int32(entity.EvaluationModeFailRetry)).Error)
			case "legacy":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).Update("lifecycle_hook_version", 0).Error)
			}
			if kind == "zero" || kind == "trial_single1" || kind == "after_only" {
				page := executionRead(t, f)
				done, err := f.init.CompleteExecutionInitialization(ctx, f.completeInput(page.RunVersion))
				require.NoError(t, err)
				require.True(t, done.Initialized)
				return
			}
			_, err := f.init.ReadExecutionInitializationPage(ctx, f.readInput())
			require.Error(t, err)
			called := false
			gate := NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { called = true; return "local", nil })
			decision, err := gate.CanDispatch(ctx, f.key)
			require.NoError(t, err)
			if kind == "multi_set" || kind == "retry" {
				require.Equal(t, entity.HookGateWaiting, decision.Gate, "shared dispatch must not bypass bound initialization")
			} else {
				require.Equal(t, entity.HookGateReady, decision.Gate)
			}
			if kind == "legacy" {
				require.False(t, called)
			}
		})
	}
}

func TestHookExecutionMySQLCancelOrdering(t *testing.T) {
	for _, winner := range []string{"cancel_before_write", "cancel_before_complete", "complete_before_cancel"} {
		t.Run(winner, func(t *testing.T) {
			f := newExecutionFixture(t, 1)
			page := executionRead(t, f)
			if winner != "cancel_before_write" {
				page = executionWrite(t, f)
			}
			ctx := context.Background()
			version := page.RunVersion
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			var releaseOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			name := "execution_order"
			require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table != "expt_lifecycle_run" {
					return
				}
				fields, ok := tx.Statement.Dest.(map[string]any)
				if !ok {
					return
				}
				_, isComplete := fields["execution_initialized"]
				_, isCancel := fields["finalize_state"]
				if (winner == "complete_before_cancel" && isComplete) || (winner != "complete_before_cancel" && isCancel) {
					once.Do(func() { close(entered); <-release })
				}
			}))
			defer f.sql.Callback().Update().Remove(name)
			cancel := func(v int64) error {
				_, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: v}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
				return err
			}
			first, second := make(chan error, 1), make(chan error, 1)
			if winner == "complete_before_cancel" {
				go func() { _, err := f.init.CompleteExecutionInitialization(ctx, f.completeInput(version)); first <- err }()
			} else {
				go func() { first <- cancel(version) }()
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("transaction did not enter barrier")
			}
			if winner == "complete_before_cancel" {
				go func() { second <- cancel(version) }()
			} else if winner == "cancel_before_write" {
				go func() { _, err := f.init.WriteExecutionInitializationPage(ctx, f.writeInput(version)); second <- err }()
			} else {
				go func() { _, err := f.init.CompleteExecutionInitialization(ctx, f.completeInput(version)); second <- err }()
			}
			releaseOnce.Do(func() { close(release) })
			require.NoError(t, <-first)
			require.Error(t, <-second)
			require.Equal(t, winner == "complete_before_cancel", executionFlag(t, f))
			if winner == "cancel_before_write" {
				require.Zero(t, executionCount(t, f, "expt_item_result"))
			}
			if winner == "complete_before_cancel" {
				run, err := f.repo.GetRun(ctx, f.key)
				require.NoError(t, err)
				require.NoError(t, cancel(run.Version))
			}
			run, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: f.manifests[0].Frozen.ItemID})
			require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
		})
	}
}

func TestHookExecutionMySQLCompleteRollback(t *testing.T) {
	f := newExecutionFixture(t, 1)
	page := executionWrite(t, f)
	name := "execution_complete_failure"
	require.NoError(t, f.sql.Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_lifecycle_run" {
			if fields, ok := tx.Statement.Dest.(map[string]any); ok {
				if _, ok = fields["execution_initialized"]; ok {
					tx.AddError(errors.New("completion storage failed"))
				}
			}
		}
	}))
	defer f.sql.Callback().Update().Remove(name)
	_, err := f.init.CompleteExecutionInitialization(context.Background(), f.completeInput(page.RunVersion))
	require.ErrorIs(t, err, entity.ErrHookExecutionStorage)
	require.False(t, executionFlag(t, f))
	require.Equal(t, page.RunVersion, executionRead(t, f).RunVersion)
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
	require.Equal(t, int64(entity.ExptStatus_Pending), *log.Status)
	var expt model.Experiment
	require.NoError(t, f.sql.First(&expt, f.expt).Error)
	require.Equal(t, int32(entity.ExptStatus_Pending), expt.Status)
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Zero(t, stats.PendingCnt)
	require.Equal(t, float64(7), stats.CreditCost)
	require.Equal(t, int64(1), executionCount(t, f, "expt_item_result"))
	require.Equal(t, int64(2), executionCount(t, f, "expt_turn_result"))
}

func TestHookExecutionMySQLReadBoundsAndCancellation(t *testing.T) {
	f := newExecutionFixture(t, 1)
	for _, change := range []func(*entity.HookExecutionInitializationReadInput){
		func(in *entity.HookExecutionInitializationReadInput) { in.ExecutionScope = "wrong" },
		func(in *entity.HookExecutionInitializationReadInput) { in.Key.WorkspaceID++ },
		func(in *entity.HookExecutionInitializationReadInput) { in.StartOrdinal = 2 },
		func(in *entity.HookExecutionInitializationReadInput) { in.Limit = 101 },
	} {
		in := f.readInput()
		change(&in)
		page, err := f.init.ReadExecutionInitializationPage(context.Background(), in)
		require.Error(t, err)
		require.Nil(t, page)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	page, err := f.init.ReadExecutionInitializationPage(ctx, f.readInput())
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, page)
	require.False(t, executionFlag(t, f))
	require.Zero(t, executionCount(t, f, "expt_item_result"))
}

func TestHookExecutionMySQLUsesDBTime(t *testing.T) {
	f := newExecutionFixture(t, 1)
	f.sql.Config.NowFunc = func() time.Time { return time.Now().Add(24 * time.Hour) }
	var before time.Time
	require.NoError(t, f.sql.Raw("SELECT CURRENT_TIMESTAMP(3)").Scan(&before).Error)
	executionWrite(t, f)
	var row model.ExptItemResult
	require.NoError(t, f.sql.First(&row, f.manifests[0].ItemResultID).Error)
	require.WithinDuration(t, before, row.CreatedAt, 2*time.Second)
}
