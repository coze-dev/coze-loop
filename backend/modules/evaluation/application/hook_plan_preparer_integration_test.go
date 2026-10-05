// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
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
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

const preparerIntegrationScope = "preparer-integration"

// Independently calculated v1 chain for ordinals 0..2, source 11/12/13,
// item IDs 201/202/203 and item versions 301/302/303; database PKs are excluded.
const preparerIntegrationThreeItemHash = "27357bdc31acbc3f422b8c37d36c780215d830f5517b2634822ae51cc02e0113"

type preparerSQLExperiments struct {
	repo.IExperimentRepo
	p   db.Provider
	key entity.HookRunKey
}

func (r preparerSQLExperiments) GetByID(ctx context.Context, id, space int64) (*entity.Experiment, error) {
	if id != r.key.ExperimentID || space != r.key.WorkspaceID || !contexts.CtxWriteDB(ctx) {
		return nil, entity.ErrHookStoreConflict
	}
	var row model.Experiment
	if err := r.p.NewSession(ctx, db.WithMaster()).Where("id=? AND space_id=?", id, space).First(&row).Error; err != nil {
		return nil, err
	}
	return convert.NewExptConverter().PO2DO(&row, nil)
}

type preparerIntegrationIDs struct{ allocated atomic.Int64 }

func (g *preparerIntegrationIDs) GenID(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	g.allocated.Add(1)
	return executorIDs.Add(1), nil
}
func (g *preparerIntegrationIDs) GenMultiIDs(ctx context.Context, n int) ([]int64, error) {
	if n < 0 || n > 100 {
		return nil, errors.New("unexpected allocation size")
	}
	ids := make([]int64, n)
	for i := range ids {
		id, err := g.GenID(ctx)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

type preparerIntegrationSelector struct {
	mu         sync.Mutex
	cursors    []string
	selectPage func(context.Context, entity.HookSelectionInput) (entity.HookSelectionPage, error)
}

func (s *preparerIntegrationSelector) SelectPage(ctx context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
	s.mu.Lock()
	s.cursors = append(s.cursors, in.Cursor)
	s.mu.Unlock()
	page, err := s.selectPage(ctx, in)
	page.Items = append([]entity.HookPlanItem(nil), page.Items...)
	return page, err
}
func (s *preparerIntegrationSelector) calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.cursors...)
}

func preparerIntegrationItem(id int64) entity.HookPlanItem {
	return entity.HookPlanItem{SourceSpaceID: 11, EvalSetID: 12, EvalSetVersionID: 13, ItemID: id, ItemVersionID: id + 100}
}

type preparerIntegrationFixture struct {
	sql  *gorm.DB
	p    db.Provider
	key  entity.HookRunKey
	runs repo.IHookRepo
	deps service.HookPlanPreparerDependencies
	ids  *preparerIntegrationIDs
}

func newPreparerIntegrationFixture(t *testing.T, mode entity.ExptRunMode, items []int64) *preparerIntegrationFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires isolated HOOK_MYSQL_TX_DSN; preparation-only checks must leave it empty")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	require.True(t, cfg.ParseTime)
	require.Equal(t, "UTC", cfg.Loc.String())
	require.Equal(t, "'+00:00'", cfg.Params["time_zone"])
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	s := p.NewSession(context.Background())
	pool, err := s.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	key := entity.HookRunKey{WorkspaceID: executorIDs.Add(1), ExperimentID: executorIDs.Add(1), RunID: executorIDs.Add(1)}
	t.Cleanup(func() { require.NoError(t, cleanupExecutorFixture(s, key.WorkspaceID, key.ExperimentID)) })
	expt := &entity.Experiment{ID: key.ExperimentID, SpaceID: key.WorkspaceID, CreatedBy: "preparer-user", Name: "preparer-fixture", ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing, EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet, EvalSetID: 12, EvalSetVersionID: 13, EvalSetSpaceID: 11}
	po, err := convert.NewExptConverter().DO2PO(expt)
	require.NoError(t, err)
	require.NoError(t, s.Create(po).Error)
	expts := preparerSQLExperiments{p: p, key: key}
	expt, err = expts.GetByID(contexts.WithCtxWriteDB(context.Background()), key.ExperimentID, key.WorkspaceID)
	require.NoError(t, err)
	fingerprint, err := entity.HookSelectionConfigFingerprint(expt)
	require.NoError(t, err)
	block, err := aes.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	codec := hookinfra.NewStorageCodec(executorProtector{aead})
	modeName := map[entity.ExptRunMode]string{entity.EvaluationModeSubmit: "submit", entity.EvaluationModeFailRetry: "fail_retry", entity.EvaluationModeRetryItems: "retry_items"}[mode]
	require.NotEmpty(t, modeName)
	conf := &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}
	configRepo := experiment.NewHookConfigRepo(p, codec)
	configOwner := hookcomponent.ConfigOwner{WorkspaceID: key.WorkspaceID, ObjectID: key.ExperimentID, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: preparerIntegrationScope}
	_, err = configRepo.UpdateConfig(context.Background(), configOwner, entity.HookConfigUpdateInput{KeyID: "preparer-key", Config: &entity.LifecycleHookConf{Before: conf, After: conf}})
	require.NoError(t, err)
	config, err := configRepo.GetConfig(context.Background(), configOwner)
	require.NoError(t, err)
	snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: preparerIntegrationScope, CreatedAt: time.Now(), Config: &entity.LifecycleHookConf{Before: conf, After: conf}, Selection: &entity.HookSelectionSeed{Version: 1, ConfigFingerprint: fingerprint}, Context: &spi.HookRunContext{WorkspaceID: gptr.Of(strconv.FormatInt(key.WorkspaceID, 10)), ExperimentID: gptr.Of(strconv.FormatInt(key.ExperimentID, 10)), RunID: gptr.Of(strconv.FormatInt(key.RunID, 10)), RunMode: &modeName, Initiator: &spi.HookInitiator{UserID: gptr.Of("preparer-user"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of(expt.Name), Type: gptr.Of("offline")}, EvalSets: []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("11"), ID: gptr.Of("12"), VersionID: gptr.Of("13")}}}})
	require.NoError(t, err)
	protected, err := codec.EncodeSnapshot(context.Background(), "preparer-key", snapshot)
	require.NoError(t, err)
	log := &entity.ExptRunLog{ID: key.RunID, ExptRunID: key.RunID, ExptID: key.ExperimentID, SpaceID: key.WorkspaceID, CreatedBy: "preparer-user", Mode: int32(mode), Status: int64(entity.ExptStatus_Processing)}
	if len(items) > 0 {
		log.ItemIds = []entity.ExptRunLogItems{{ItemIDs: append([]int64(nil), items...), CreateAt: gptr.Of(int64(123))}}
	}
	seed := func(phase string) *entity.HookOperationSeed {
		id := executorIDs.Add(1)
		return &entity.HookOperationSeed{ID: id, OperationID: fmt.Sprintf("prepare_%s_%d", phase, id), IdempotencyKey: fmt.Sprintf("prepare_key_%d", id)}
	}
	runs := experiment.NewHookRunRepo(p)
	created, err := runs.CreateRunWithHooks(context.Background(), entity.HookCreateRunInput{Key: key, RunLog: log, Snapshot: protected, ExpectedConfigRevision: config.Revision, Before: seed("before"), After: seed("after")})
	require.NoError(t, err)
	require.False(t, created.Run.PlanReady)
	require.Zero(t, created.Run.PlanCount)
	ids := new(preparerIntegrationIDs)
	return &preparerIntegrationFixture{sql: s, p: p, key: key, runs: runs, ids: ids, deps: service.HookPlanPreparerDependencies{Runs: runs, Plans: experiment.NewHookPlanRepo(p), Codec: codec, Experiments: expts, Initialization: experiment.NewHookRunInitializationRepo(p), ResultReader: experiment.NewHookPlanResultReader(p), IDs: ids}}
}

func (f *preparerIntegrationFixture) run(t *testing.T) *entity.HookStoredRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run, err := f.runs.GetRun(ctx, f.key)
	require.NoError(t, err)
	return run
}
func (f *preparerIntegrationFixture) scope(q *gorm.DB) *gorm.DB {
	return q.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.key.WorkspaceID, f.key.ExperimentID, f.key.RunID)
}
func (f *preparerIntegrationFixture) phase(t *testing.T) string {
	t.Helper()
	var c struct {
		Phase string `json:"phase"`
	}
	require.NoError(t, json.Unmarshal([]byte(f.run(t).PlanCursor), &c))
	return c.Phase
}
func (f *preparerIntegrationFixture) step(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	p, err := service.NewHookPlanPreparer(f.deps)
	if err != nil {
		return err
	}
	return p.PreparePlan(ctx, hookcomponent.WorkerRunInput{ExecutionScope: preparerIntegrationScope, Candidate: entity.HookRunCandidate{Key: f.key}})
}
func (f *preparerIntegrationFixture) assertActivation(t *testing.T, ready bool) {
	t.Helper()
	var ops []model.ExptLifecycleHookRun
	require.NoError(t, f.scope(f.sql).Order("phase ASC").Find(&ops).Error)
	require.Len(t, ops, 2)
	for _, op := range ops {
		require.Zero(t, op.Attempt)
		require.Equal(t, "pending", op.Status)
		if op.Phase == "before" && ready {
			require.NotNil(t, op.ActivatedAt)
			require.NotNil(t, op.NextAttemptAt)
		} else {
			require.Nil(t, op.ActivatedAt)
			require.Nil(t, op.NextAttemptAt)
		}
	}
}
func (f *preparerIntegrationFixture) assertRows(t *testing.T, want ...int64) {
	t.Helper()
	var rows []model.ExptLifecycleRunItem
	require.NoError(t, f.scope(f.sql).Order("ordinal ASC, id ASC").Find(&rows).Error)
	require.Len(t, rows, len(want))
	for i, row := range rows {
		require.Positive(t, row.ID)
		require.Equal(t, int64(i), row.Ordinal)
		require.Equal(t, want[i], row.ItemID)
		require.Equal(t, int64(11), row.SourceSpaceID)
		require.Equal(t, int64(12), row.EvalSetID)
		require.Equal(t, int64(13), row.EvalSetVersionID)
		require.Equal(t, want[i]+100, row.ItemVersionID)
	}
	require.Equal(t, int64(len(want)), f.run(t).PlanCount)
}
func (f *preparerIntegrationFixture) finish(t *testing.T) {
	t.Helper()
	for i := 0; i < 12; i++ {
		if f.run(t).PlanReady {
			f.assertActivation(t, true)
			return
		}
		f.assertActivation(t, false)
		require.NoError(t, f.step(context.Background()))
	}
	t.Fatal("preparer did not reach ready within bounded steps")
}

func TestHookPlanPreparerMySQLPagesEmptyDedupAndRestart(t *testing.T) {
	f := newPreparerIntegrationFixture(t, entity.EvaluationModeFailRetry, nil)
	pages := map[string]entity.HookSelectionPage{"": {Items: []entity.HookPlanItem{preparerIntegrationItem(201), preparerIntegrationItem(202)}, NextCursor: "p1"}, "p1": {NextCursor: "gap"}, "gap": {Items: []entity.HookPlanItem{preparerIntegrationItem(202), preparerIntegrationItem(203)}, NextCursor: "done", Done: true}}
	newSource := func() *preparerIntegrationSelector {
		return &preparerIntegrationSelector{selectPage: func(_ context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
			if in.Key != f.key || in.Mode != entity.EvaluationModeFailRetry {
				return entity.HookSelectionPage{}, errors.New("wrong selection identity")
			}
			page, ok := pages[in.Cursor]
			if !ok {
				return entity.HookSelectionPage{}, errors.New("unexpected source cursor")
			}
			return page, nil
		}}
	}
	first := newSource()
	f.deps.Selector = first
	require.NoError(t, f.step(context.Background()))
	f.assertRows(t, 201, 202)
	f.assertActivation(t, false)
	require.Equal(t, []string{""}, first.calls())
	restarted := newSource()
	f.deps.Selector = restarted
	before := f.run(t)
	require.NoError(t, f.step(context.Background()))
	after := f.run(t)
	require.Greater(t, after.Version, before.Version)
	require.NotEqual(t, before.PlanCursor, after.PlanCursor)
	require.Equal(t, before.PlanCount, after.PlanCount)
	f.assertRows(t, 201, 202)
	f.assertActivation(t, false)
	require.NoError(t, f.step(context.Background()))
	f.assertRows(t, 201, 202, 203)
	f.assertActivation(t, false)
	require.Equal(t, []string{"p1", "gap"}, restarted.calls())
	require.Equal(t, "verify", f.phase(t))
	f.finish(t)
	f.assertRows(t, 201, 202, 203)
	require.Equal(t, preparerIntegrationThreeItemHash, f.run(t).PlanHash)
	require.Equal(t, int64(3), f.ids.allocated.Load())
}

type preparerAppendDuringVerify struct {
	repo.IHookPlanRepo
	afterRead func(context.Context, *entity.HookPlanReadPage) error
	once      sync.Once
}

func (p *preparerAppendDuringVerify) ReadPlanPage(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookPlanReadPage, error) {
	page, err := p.IHookPlanRepo.ReadPlanPage(ctx, in)
	if err != nil {
		return nil, err
	}
	p.once.Do(func() { err = p.afterRead(ctx, page) })
	return page, err
}

func TestHookPlanPreparerMySQLAppendTailDuringVerify(t *testing.T) {
	f := newPreparerIntegrationFixture(t, entity.EvaluationModeRetryItems, []int64{201})
	f.deps.Selector = &preparerIntegrationSelector{selectPage: func(_ context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
		if in.Key != f.key || in.Mode != entity.EvaluationModeRetryItems || in.Cursor != "" {
			return entity.HookSelectionPage{}, errors.New("wrong retry batch")
		}
		items := make([]entity.HookPlanItem, 0, len(in.ItemIDs))
		for _, id := range in.ItemIDs {
			items = append(items, preparerIntegrationItem(id))
		}
		return entity.HookSelectionPage{Items: items, NextCursor: "batch-done", Done: true}, nil
	}}
	require.NoError(t, f.step(context.Background()))
	require.Equal(t, "verify", f.phase(t))
	f.assertRows(t, 201)
	f.assertActivation(t, false)
	f.deps.Plans = &preparerAppendDuringVerify{IHookPlanRepo: f.deps.Plans, afterRead: func(ctx context.Context, page *entity.HookPlanReadPage) error {
		return experiment.NewHookRunInitializationRepo(f.p).AppendHookRunItems(ctx, entity.HookAppendRunItemsInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.RunVersion}, ExecutionScope: preparerIntegrationScope, ItemIDs: []int64{202, 203}})
	}}
	require.ErrorIs(t, f.step(context.Background()), entity.ErrHookStoreConflict, "stale verification must not publish ready after an append")
	f.assertRows(t, 201)
	f.assertActivation(t, false)
	initial, err := f.deps.Initialization.ReadRunInitialization(contexts.WithCtxWriteDB(context.Background()), f.key)
	require.NoError(t, err)
	require.NotNil(t, initial)
	require.NotNil(t, initial.RunLog)
	log := initial.RunLog
	require.Equal(t, []int64{201, 202, 203}, log.GetItemIDs())
	require.Len(t, log.ItemIds, 2)
	f.finish(t)
	f.assertRows(t, 201, 202, 203)
	require.Equal(t, preparerIntegrationThreeItemHash, f.run(t).PlanHash)
	require.Equal(t, int64(3), f.ids.allocated.Load())
}

func TestHookPlanPreparerMySQLCancelDuringSelection(t *testing.T) {
	for _, empty := range []bool{true, false} {
		t.Run(fmt.Sprint(empty), func(t *testing.T) {
			f := newPreparerIntegrationFixture(t, entity.EvaluationModeSubmit, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			var wg sync.WaitGroup
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer func() { cancel(); unblock(); wg.Wait() }()
			f.deps.Selector = &preparerIntegrationSelector{selectPage: func(ctx context.Context, _ entity.HookSelectionInput) (entity.HookSelectionPage, error) {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
					return entity.HookSelectionPage{}, ctx.Err()
				}
				page := entity.HookSelectionPage{NextCursor: "next"}
				if !empty {
					page.Items = []entity.HookPlanItem{preparerIntegrationItem(201)}
				}
				return page, nil
			}}
			var prepareErr error
			wg.Add(1)
			go func() { defer wg.Done(); prepareErr = f.step(ctx) }()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("selector was not reached")
			}
			run := f.run(t)
			cancelled, err := f.runs.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
			require.NoError(t, err)
			unblock()
			wg.Wait()
			require.ErrorIs(t, prepareErr, entity.ErrHookStoreConflict)
			final := f.run(t)
			require.Equal(t, cancelled.Run, final)
			require.False(t, final.PlanReady)
			require.Equal(t, entity.HookGateClosed, final.State.Gate)
			f.assertRows(t)
			var before model.ExptLifecycleHookRun
			require.NoError(t, f.scope(f.sql).Where("phase=?", "before").First(&before).Error)
			require.Nil(t, before.ActivatedAt)
			require.Zero(t, before.Attempt)
			require.Equal(t, "failed", before.Status)
		})
	}
}

func (f *preparerIntegrationFixture) retryFailedBeforeSource(t *testing.T) entity.HookRunKey {
	t.Helper()
	ctx := context.Background()
	sourceKey := f.key
	source := f.run(t)
	require.True(t, source.PlanReady)
	require.False(t, source.ExecutionStarted)
	intent := entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "HOOK_BEFORE_FAILED"}
	begun, err := f.runs.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: sourceKey, ExpectedVersion: source.Version}, Intent: intent})
	require.NoError(t, err)
	finalized, err := f.runs.CommitFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: sourceKey, ExpectedVersion: begun.Run.Version}, Intent: intent})
	require.NoError(t, err)
	require.True(t, finalized.Run.PlanReady)
	require.False(t, finalized.Run.ExecutionStarted)
	require.Equal(t, entity.HookGateClosed, finalized.Run.State.Gate)
	require.Equal(t, intent, finalized.Run.State.Intent)
	snapshot, err := f.deps.Codec.DecodeSnapshot(ctx, sourceKey, preparerIntegrationScope, source.Snapshot)
	require.NoError(t, err)
	in := snapshot.Input()
	targetKey := sourceKey
	targetKey.RunID = executorIDs.Add(1)
	in.Key = targetKey
	in.Context.RunID = gptr.Of(strconv.FormatInt(targetKey.RunID, 10))
	in.Context.RunMode = gptr.Of("fail_retry")
	in.CreatedAt = time.Now()
	retrySnapshot, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	protected, err := f.deps.Codec.EncodeSnapshot(ctx, "preparer-key", retrySnapshot)
	require.NoError(t, err)
	owner := hookcomponent.ConfigOwner{WorkspaceID: targetKey.WorkspaceID, ObjectID: targetKey.ExperimentID, Kind: hookcomponent.ConfigOwnerExperiment, ExecutionScope: preparerIntegrationScope}
	config, err := experiment.NewHookConfigRepo(f.p, f.deps.Codec).GetConfig(ctx, owner)
	require.NoError(t, err)
	seed := func(phase string) *entity.HookOperationSeed {
		id := executorIDs.Add(1)
		return &entity.HookOperationSeed{ID: id, OperationID: fmt.Sprintf("retry_%s_%d", phase, id), IdempotencyKey: fmt.Sprintf("retry_key_%d", id)}
	}
	_, err = f.runs.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: targetKey, RunLog: &entity.ExptRunLog{ID: targetKey.RunID, ExptRunID: targetKey.RunID, ExptID: targetKey.ExperimentID, SpaceID: targetKey.WorkspaceID, CreatedBy: source.CreatedBy, Mode: int32(entity.EvaluationModeFailRetry), Status: int64(entity.ExptStatus_Processing)}, Snapshot: protected, SourceRunID: gptr.Of(sourceKey.RunID), ExpectedLatestRunID: sourceKey.RunID, ExpectedConfigRevision: config.Revision, Before: seed("before"), After: seed("after")})
	require.NoError(t, err)
	// Model the normal retry startup projection, without invoking execution/RPC.
	updated := f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=? AND latest_run_id=?", targetKey.ExperimentID, targetKey.WorkspaceID, targetKey.RunID).UpdateColumn("status", int32(entity.ExptStatus_Processing))
	require.NoError(t, updated.Error)
	require.Equal(t, int64(1), updated.RowsAffected)
	f.key = targetKey
	f.deps.Experiments = preparerSQLExperiments{p: f.p, key: targetKey}
	return sourceKey
}

func TestHookPlanPreparerMySQLR07UsesRealBaseResultReader(t *testing.T) {
	for _, baseResult := range []string{"empty", "successful item", "successful turn"} {
		t.Run(baseResult, func(t *testing.T) {
			f := newPreparerIntegrationFixture(t, entity.EvaluationModeSubmit, nil)
			f.deps.Selector = &preparerIntegrationSelector{selectPage: func(_ context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
				if in.Key != f.key || in.Mode != entity.EvaluationModeSubmit || in.Cursor != "" {
					return entity.HookSelectionPage{}, errors.New("unexpected source preparation")
				}
				return entity.HookSelectionPage{Items: []entity.HookPlanItem{preparerIntegrationItem(201), preparerIntegrationItem(202), preparerIntegrationItem(203)}, NextCursor: "source-done", Done: true}, nil
			}}
			f.finish(t)
			f.assertRows(t, 201, 202, 203)
			require.Equal(t, preparerIntegrationThreeItemHash, f.run(t).PlanHash)
			sourceKey := f.retryFailedBeforeSource(t)
			sourceBefore, err := f.runs.GetRun(context.Background(), sourceKey)
			require.NoError(t, err)
			if baseResult != "empty" {
				id := executorIDs.Add(1)
				var row any
				if baseResult == "successful item" {
					row = &model.ExptItemResult{ID: id, SpaceID: sourceKey.WorkspaceID, ExptID: sourceKey.ExperimentID, ExptRunID: sourceKey.RunID, ItemID: 501, ItemVersionID: 601, Status: int32(entity.ItemRunState_Success), LogID: "r07-success-item"}
				} else {
					row = &model.ExptTurnResult{ID: id, SpaceID: sourceKey.WorkspaceID, ExptID: sourceKey.ExperimentID, ExptRunID: sourceKey.RunID, ItemID: 501, ItemVersionID: 601, TurnID: 701, Status: int32(entity.TurnRunState_Success), LogID: "r07-success-turn"}
				}
				// Base results are not part of cleanupExecutorFixture; remove this exact row first.
				t.Cleanup(func() {
					require.NoError(t, f.sql.Unscoped().Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=?", id, sourceKey.WorkspaceID, sourceKey.ExperimentID, sourceKey.RunID).Delete(row).Error)
				})
				require.NoError(t, f.sql.Create(row).Error)
			}
			hasBase, err := f.deps.ResultReader.HasExperimentResults(context.Background(), f.key, preparerIntegrationScope)
			require.NoError(t, err)
			require.Equal(t, baseResult != "empty", hasBase, "successful records on an older Run must still count")
			normal := &preparerIntegrationSelector{selectPage: func(_ context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
				if baseResult == "empty" || in.Key != f.key || in.Mode != entity.EvaluationModeFailRetry || in.Cursor != "" {
					return entity.HookSelectionPage{}, errors.New("unexpected normal retry selection")
				}
				return entity.HookSelectionPage{NextCursor: "no-failed-items", Done: true}, nil
			}}
			f.deps.Selector = normal
			f.finish(t)
			if baseResult == "empty" {
				f.assertRows(t, 201, 202, 203)
				require.Equal(t, preparerIntegrationThreeItemHash, f.run(t).PlanHash)
				require.Empty(t, normal.calls(), "R07 copy must use the frozen plan rather than reselect")
			} else {
				f.assertRows(t)
				require.Equal(t, "a37be24d461361c965b7d31c585c253628f77a928963934bd2c416b83b743867", f.run(t).PlanHash)
				require.Equal(t, []string{""}, normal.calls(), "existing successful base data must choose normal retry, not full source copy")
			}
			sourceAfter, err := f.runs.GetRun(context.Background(), sourceKey)
			require.NoError(t, err)
			require.Equal(t, sourceBefore, sourceAfter)
		})
	}
}
