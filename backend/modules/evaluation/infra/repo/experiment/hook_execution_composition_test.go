// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	svc "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	store "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Only the dataset boundary is in memory; loader, initializer and storage are real.
type compositionSource struct {
	svc.EvaluationSetItemService
	space    int64
	turns    map[int64][]int64
	requests []int64
}

func (s *compositionSource) get(space, set, id int64) ([]*entity.Turn, error) {
	s.requests = append(s.requests, id)
	ids, found := s.turns[id]
	if !found || space != s.space || set != 71 {
		return nil, fmt.Errorf("fixture source unavailable")
	}
	turns := make([]*entity.Turn, len(ids))
	for i, turnID := range ids {
		turns[i] = &entity.Turn{ID: turnID, EvalSetID: 71}
	}
	return turns, nil
}

func (s *compositionSource) BatchGetEvaluationSetItems(_ context.Context, p *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	if p.VersionID != nil {
		return nil, fmt.Errorf("draft fixture must not receive a version pointer")
	}
	items := make([]*entity.EvaluationSetItem, 0, len(p.ItemIDs))
	for _, id := range p.ItemIDs {
		turns, err := s.get(p.SpaceID, p.EvaluationSetID, id)
		if err != nil {
			return nil, err
		}
		items = append(items, &entity.EvaluationSetItem{SpaceID: p.SpaceID, EvaluationSetID: p.EvaluationSetID, ItemID: id, Turns: turns})
	}
	return items, nil
}

func (s *compositionSource) GetEvaluationSetItemVersion(_ context.Context, space, set, id int64, version *int64, _ *string) (*entity.EvaluationSetItemVersion, error) {
	if version == nil || *version <= 0 {
		return nil, fmt.Errorf("physical item version required")
	}
	turns, err := s.get(space, set, id)
	if err != nil {
		return nil, err
	}
	return &entity.EvaluationSetItemVersion{ItemID: id, ItemVersionID: *version, Turns: turns}, nil
}

type compositionSets struct {
	svc.IEvaluationSetService
	space int64
}

func (s compositionSets) GetEvaluationSet(_ context.Context, space *int64, set int64, _ *bool, _ *entity.SharedResourceOption) (*entity.EvaluationSet, error) {
	if space == nil || *space != s.space || set != 71 {
		return nil, fmt.Errorf("fixture schema scope mismatch")
	}
	return &entity.EvaluationSet{ID: set, SpaceID: s.space, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: s.space, EvaluationSetID: set}}}, nil
}

type compositionVersions struct {
	svc.EvaluationSetVersionService
}

type compositionIDs struct{ requests []int }

func (g *compositionIDs) GenID(ctx context.Context) (int64, error) {
	ids, err := g.GenMultiIDs(ctx, 1)
	return ids[0], err
}
func (g *compositionIDs) GenMultiIDs(_ context.Context, n int) ([]int64, error) {
	g.requests = append(g.requests, n)
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = compositionSequence.Add(1)
	}
	return ids, nil
}

type compositionObservedRepo struct {
	repo.IHookExecutionInitializationRepo
	recording atomic.Bool
	queries   atomic.Int64
	elapsed   time.Duration
}

func (r *compositionObservedRepo) CompleteExecutionInitialization(ctx context.Context, in entity.HookExecutionInitializationCompleteInput) (entity.HookExecutionInitializationCompletion, error) {
	r.queries.Store(0)
	r.recording.Store(true)
	start := time.Now()
	defer func() { r.elapsed = time.Since(start); r.recording.Store(false) }()
	return r.IHookExecutionInitializationRepo.CompleteExecutionInitialization(ctx, in)
}

func newCompositionInitializer(t *testing.T, f *compositionFixture, source *compositionSource, ids *compositionIDs) (hook.ExecutionInitializer, *compositionObservedRepo) {
	t.Helper()
	loader, err := svc.NewHookFrozenPlanLoader(svc.HookFrozenPlanLoaderDependencies{Plans: store.NewHookPlanRepo(f.p), Items: source, Versions: &compositionVersions{}, Sets: compositionSets{space: f.space}})
	require.NoError(t, err)
	observed := &compositionObservedRepo{IHookExecutionInitializationRepo: f.init}
	count := func(*gorm.DB) {
		if observed.recording.Load() {
			observed.queries.Add(1)
		}
	}
	name := "execution_composition_sql"
	require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(name, count))
	require.NoError(t, f.sql.Callback().Row().After("gorm:row").Register(name, count))
	require.NoError(t, f.sql.Callback().Raw().After("gorm:raw").Register(name, count))
	require.NoError(t, f.sql.Callback().Update().After("gorm:update").Register(name, count))
	t.Cleanup(func() {
		f.sql.Callback().Query().Remove(name)
		f.sql.Callback().Row().Remove(name)
		f.sql.Callback().Raw().Remove(name)
		f.sql.Callback().Update().Remove(name)
	})
	initializer, err := svc.NewHookFrozenExecutionInitializer(svc.HookFrozenExecutionInitializerDependencies{Repository: observed, Loader: loader, IDs: ids})
	require.NoError(t, err)
	return initializer, observed
}

func compositionAddTail(t *testing.T, f *compositionFixture) entity.HookPlanItem {
	t.Helper()
	item := entity.HookPlanItem{ID: compositionSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: compositionSequence.Add(1)}
	// The existing fixture prepares at most one 100-item plan page; extend only this fixture's sealed input.
	require.NoError(t, f.sql.Create(&model.ExptLifecycleRunItem{ID: item.ID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, Ordinal: 100, SourceSpaceID: f.space, EvalSetID: 71, ItemID: item.ItemID}).Error)
	items := make([]entity.HookPlanItem, 0, 100)
	for _, item := range f.plan {
		items = append(items, item)
	}
	digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), items)
	require.NoError(t, err)
	digest, err = entity.AppendHookPlanDigest(digest, []entity.HookPlanItem{item})
	require.NoError(t, err)
	f.hash = digest.Hash
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Updates(map[string]any{"plan_count": 101, "plan_hash": f.hash, "version": gorm.Expr("version+1")}).Error)
	return item
}

func TestHookExecutionCompositionResumeWithoutCommittedSource(t *testing.T) {
	for _, mode := range []string{"deleted", "changed"} {
		t.Run(mode, func(t *testing.T) {
			f := newCompositionFixture(t, 100)
			tail := compositionAddTail(t, f)
			source := &compositionSource{space: f.space, turns: map[int64][]int64{}}
			for _, item := range f.plan {
				source.turns[item.ItemID] = []int64{0, 10}
			}
			ids := &compositionIDs{}
			initializer, observed := newCompositionInitializer(t, f, source, ids)
			ctx := context.Background()
			partial, err := initializer.InitializeExecution(ctx, f.key, "local")
			require.ErrorIs(t, err, entity.ErrHookFrozenItemUnavailable)
			require.False(t, partial.Initialized)
			require.False(t, compositionFlag(t, f))
			require.Equal(t, int64(100), compositionCount(t, f, "expt_item_result"))
			require.Equal(t, int64(100), compositionCount(t, f, "expt_item_result_run_log"))
			require.Equal(t, int64(200), compositionCount(t, f, "expt_turn_result"))
			require.Equal(t, []int{400}, ids.requests)
			var partialStats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&partialStats).Error)
			require.Zero(t, partialStats.PendingCnt, "partial pages do not publish initial stats")
			var partialRun model.ExptRunLog
			require.NoError(t, f.sql.First(&partialRun, f.key.RunID).Error)
			require.Equal(t, int64(entity.ExptStatus_Pending), gptr.Indirect(partialRun.Status))
			committed := compositionRead(t, f)
			require.Len(t, committed.Items, 100)
			for _, item := range committed.Items {
				require.NotNil(t, item.Manifest)
				require.Equal(t, []int64{0, 10}, []int64{item.Manifest.Turns[0].TurnID, item.Manifest.Turns[1].TurnID})
				if mode == "deleted" {
					delete(source.turns, item.Frozen.ItemID)
				} else {
					source.turns[item.Frozen.ItemID] = []int64{0, 99, 999}
				}
			}
			execGate := store.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil })
			gate, err := execGate.CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateWaiting, gate.Gate)
			_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: committed.RunVersion}, ItemID: committed.Items[0].Frozen.ItemID})
			require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
			source.turns[tail.ItemID] = []int64{0, 20, 30}
			source.requests = nil
			done, err := initializer.InitializeExecution(ctx, f.key, "local")
			require.NoError(t, err)
			require.True(t, done.Initialized)
			require.True(t, compositionFlag(t, f))
			require.Equal(t, []int64{tail.ItemID}, source.requests, "only the missing ordinal segment may read source")
			require.Equal(t, []int{400, 5}, ids.requests)
			again := compositionRead(t, f)
			require.Equal(t, committed.Items, again.Items, "committed manifests, IDs and turn tuples must survive source changes")
			require.Equal(t, int64(101), compositionCount(t, f, "expt_item_result"))
			require.Equal(t, int64(101), compositionCount(t, f, "expt_item_result_run_log"))
			require.Equal(t, int64(203), compositionCount(t, f, "expt_turn_result"))
			require.Zero(t, compositionCount(t, f, "expt_turn_result_run_log"))
			var stats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
			require.Equal(t, int32(101), stats.PendingCnt)
			var run model.ExptRunLog
			require.NoError(t, f.sql.First(&run, f.key.RunID).Error)
			require.Equal(t, int64(entity.ExptStatus_Processing), gptr.Indirect(run.Status))
			var expt model.Experiment
			require.NoError(t, f.sql.First(&expt, f.expt).Error)
			require.Equal(t, int32(entity.ExptStatus_Processing), expt.Status)
			tailPage, err := f.init.ReadExecutionInitializationPage(ctx, entity.HookExecutionInitializationReadInput{Key: f.key, ExecutionScope: "local", StartOrdinal: 100, Limit: 1})
			require.NoError(t, err)
			require.Len(t, tailPage.Items, 1)
			tailTurns := tailPage.Items[0].Manifest.Turns
			require.Equal(t, []int64{0, 20, 30}, []int64{tailTurns[0].TurnID, tailTurns[1].TurnID, tailTurns[2].TurnID})
			gate, err = execGate.CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateReady, gate.Gate)
			admitted, err := f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: done.RunVersion}, ItemID: committed.Items[0].Frozen.ItemID})
			require.NoError(t, err)
			require.True(t, admitted.Admitted)
			source.requests = nil
			_, err = initializer.InitializeExecution(ctx, f.key, "local")
			require.NoError(t, err)
			require.Empty(t, source.requests)
			require.Equal(t, []int{400, 5}, ids.requests)
			t.Logf("completion items=101 turns=203 sql=%d api_elapsed=%s", observed.queries.Load(), observed.elapsed)
		})
	}
}

func TestHookExecutionCompositionScaleMeasure(t *testing.T) {
	for _, count := range []int{1, 10} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := newCompositionFixture(t, count)
			source := &compositionSource{space: f.space, turns: map[int64][]int64{}}
			for _, item := range f.plan {
				source.turns[item.ItemID] = []int64{0, 10}
			}
			initializer, observed := newCompositionInitializer(t, f, source, &compositionIDs{})
			done, err := initializer.InitializeExecution(context.Background(), f.key, "local")
			require.NoError(t, err)
			require.True(t, done.Initialized)
			require.Equal(t, int64(count*2), compositionCount(t, f, "expt_turn_result"))
			t.Logf("completion items=%d turns=%d sql=%d api_elapsed=%s", count, count*2, observed.queries.Load(), observed.elapsed)
		})
	}
}

var compositionSequence = func() *atomic.Int64 { n := new(atomic.Int64); n.Store(time.Now().UnixNano()); return n }()

type compositionFixture struct {
	p           db.Provider
	sql         *gorm.DB
	repo        repo.IHookRepo
	init        repo.IHookExecutionInitializationRepo
	key         entity.HookRunKey
	space, expt int64
	plan        []entity.HookPlanItem
	hash        string
}

// Same owned database and production Run/plan setup as newExecutionFixture;
// external-package placement is necessary because service wire.go imports this repo.
func newCompositionFixture(t *testing.T, count int) *compositionFixture {
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
	pool.SetMaxOpenConns(8)
	f := &compositionFixture{p: p, sql: s, repo: store.NewHookRunRepo(p), init: store.NewHookExecutionInitializationRepo(p), space: compositionSequence.Add(1), expt: compositionSequence.Add(1)}
	t.Cleanup(func() {
		defer pool.Close()
		for _, table := range []string{"expt_item_result", "expt_turn_result", "expt_item_result_run_log", "expt_turn_result_run_log", "expt_stats", "expt_lifecycle_run_item", "expt_lifecycle_hook_run", "expt_lifecycle_run", "expt_run_log"} {
			require.NoError(t, s.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
		}
		require.NoError(t, s.Unscoped().Delete(&model.Experiment{}, "space_id=? AND id=?", f.space, f.expt).Error)
	})
	require.NoError(t, s.Create(&model.Experiment{ID: f.expt, SpaceID: f.space, Name: fmt.Sprint(f.expt), Status: int32(entity.ExptStatus_Pending), ExptType: 1, EvalSetID: 71}).Error)
	require.NoError(t, s.Create(&model.ExptStats{ID: compositionSequence.Add(1), SpaceID: f.space, ExptID: f.expt, CreditCost: 7}).Error)
	run := compositionSequence.Add(1)
	f.key = entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: run}
	input := entity.HookCreateRunInput{Key: f.key, RunLog: &entity.ExptRunLog{ID: run, SpaceID: f.space, ExptID: f.expt, ExptRunID: run, CreatedBy: "fixture-user", Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Pending)},
		Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1, 7, 3}, KeyID: "fixture-key", Hash: strings.Repeat("a", 64), ExecutionScope: "local"},
		Before:   &entity.HookOperationSeed{ID: compositionSequence.Add(1), OperationID: fmt.Sprintf("composition-before-%d", run), IdempotencyKey: fmt.Sprintf("before-key-%d", run)},
		After:    &entity.HookOperationSeed{ID: compositionSequence.Add(1), OperationID: fmt.Sprintf("composition-after-%d", run), IdempotencyKey: fmt.Sprintf("after-key-%d", run)}}
	created, err := f.repo.CreateRunWithHooks(context.Background(), input)
	require.NoError(t, err)
	for i := 0; i < count; i++ {
		item := entity.HookPlanItem{ID: compositionSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: compositionSequence.Add(1)}
		if i%2 == 1 {
			item.ItemVersionID = 42
		}
		f.plan = append(f.plan, item)
	}
	version := created.Run.Version
	if count > 0 {
		written, err := f.repo.AppendPlanPage(context.Background(), entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: version}, Items: f.plan, NextCursor: "done"})
		require.NoError(t, err)
		version = written.Run.Version
	}
	digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), f.plan)
	require.NoError(t, err)
	f.hash = digest.Hash
	_, err = f.repo.FinishPlan(context.Background(), entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: version}, Count: int64(count), Hash: f.hash})
	require.NoError(t, err)
	require.NoError(t, s.Model(&model.ExptLifecycleHookRun{}).Where("space_id=? AND expt_run_id=? AND phase='before'", f.space, run).Updates(map[string]any{"status": "succeeded", "attempt": 1}).Error)
	require.NoError(t, s.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, run).Update("gate", 1).Error)
	return f
}
func compositionRead(t *testing.T, f *compositionFixture) *entity.HookExecutionInitializationPage {
	t.Helper()
	p, err := f.init.ReadExecutionInitializationPage(context.Background(), entity.HookExecutionInitializationReadInput{Key: f.key, ExecutionScope: "local", Limit: 100})
	require.NoError(t, err)
	return p
}
func compositionCount(t *testing.T, f *compositionFixture, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&n).Error)
	return n
}
func compositionFlag(t *testing.T, f *compositionFixture) bool {
	t.Helper()
	var value bool
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Pluck("execution_initialized", &value).Error)
	return value
}
