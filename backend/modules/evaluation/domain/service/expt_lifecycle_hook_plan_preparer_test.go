// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
)

type preparerStore struct {
	repo.IHookRepo
	runs        map[int64]*entity.HookStoredRun
	rows        map[int64][]entity.HookPlanItem
	logs        map[int64]*entity.ExptRunLog
	expt        *entity.Experiment
	effects     []string
	storageErr  error
	beforeWrite func()
	lostReceipt bool
}

func (s *preparerStore) GetRun(ctx context.Context, key entity.HookRunKey) (*entity.HookStoredRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.storageErr != nil {
		return nil, s.storageErr
	}
	r := s.runs[key.RunID]
	if r == nil {
		return nil, entity.ErrHookStoreMissing
	}
	out := *r
	return &out, nil
}
func (s *preparerStore) guard(ctx context.Context, g entity.HookStoreGuard) error {
	if s.beforeWrite != nil {
		f := s.beforeWrite
		s.beforeWrite = nil
		f()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.storageErr != nil {
		return s.storageErr
	}
	r := s.runs[g.Key.RunID]
	if r == nil || r.State.Key != g.Key || r.Version != g.ExpectedVersion || s.expt.LatestRunID != g.Key.RunID || r.State.Gate == entity.HookGateClosed || r.State.Finalize != entity.HookFinalizeNone || r.PlanReady {
		return entity.ErrHookStoreConflict
	}
	return nil
}
func (s *preparerStore) receipt(r *entity.HookStoredRun) (entity.HookStoreResult, error) {
	out := *r
	if s.lostReceipt {
		s.lostReceipt = false
		return entity.HookStoreResult{}, entity.ErrHookPlanStorage
	}
	return entity.HookStoreResult{Run: &out, Changed: true}, nil
}
func (s *preparerStore) AppendPlanPage(ctx context.Context, in entity.HookPlanPageInput) (entity.HookStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookStoreResult{}, err
	}
	if err := s.guard(ctx, in.HookStoreGuard); err != nil {
		return entity.HookStoreResult{}, err
	}
	r := s.runs[in.Key.RunID]
	if r.PlanCursor != in.Cursor || r.PlanCount != in.StartOrdinal {
		return entity.HookStoreResult{}, entity.ErrHookStoreConflict
	}
	seen := map[int64]bool{}
	for _, i := range s.rows[in.Key.RunID] {
		seen[i.ItemID] = true
	}
	for _, i := range in.Items {
		if seen[i.ItemID] {
			return entity.HookStoreResult{}, entity.ErrHookStoreConflict
		}
		seen[i.ItemID] = true
	}
	s.rows[in.Key.RunID] = append(s.rows[in.Key.RunID], in.Items...)
	r.PlanCount += int64(len(in.Items))
	r.PlanCursor = in.NextCursor
	r.Version++
	s.effects = append(s.effects, "append")
	return s.receipt(r)
}
func (s *preparerStore) AdvancePlanCursor(ctx context.Context, in entity.HookAdvancePlanInput) (entity.HookStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookStoreResult{}, err
	}
	if err := s.guard(ctx, in.HookStoreGuard); err != nil {
		return entity.HookStoreResult{}, err
	}
	r := s.runs[in.Key.RunID]
	if r.PlanCursor != in.Cursor || r.PlanCount != in.ExpectedCount || r.Snapshot.ExecutionScope != in.ExecutionScope {
		return entity.HookStoreResult{}, entity.ErrHookStoreConflict
	}
	r.PlanCursor = in.NextCursor
	r.Version++
	s.effects = append(s.effects, "advance")
	return s.receipt(r)
}
func (s *preparerStore) ReadPlanPage(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookPlanReadPage, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	r, err := s.GetRun(ctx, in.Key)
	if err != nil {
		return nil, err
	}
	if r.Snapshot.ExecutionScope != in.ExecutionScope || in.StartOrdinal > r.PlanCount {
		return nil, entity.ErrHookStoreConflict
	}
	end := min(r.PlanCount, in.StartOrdinal+int64(in.Limit))
	rows := s.rows[in.Key.RunID]
	if int64(len(rows)) != r.PlanCount {
		return nil, entity.ErrHookStoreCorrupt
	}
	return &entity.HookPlanReadPage{Items: append([]entity.HookPlanItem(nil), rows[in.StartOrdinal:end]...), NextOrdinal: end, Count: r.PlanCount, Hash: r.PlanHash, Ready: r.PlanReady, RunVersion: r.Version, HasMore: end < r.PlanCount}, nil
}
func (s *preparerStore) MGetPlanItems(ctx context.Context, in entity.HookPlanLookupInput) (*entity.HookPlanLookupResult, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	r, err := s.GetRun(ctx, in.Key)
	if err != nil {
		return nil, err
	}
	out := &entity.HookPlanLookupResult{RunVersion: r.Version, Count: r.PlanCount, Ready: r.PlanReady}
	for _, id := range in.ItemIDs {
		for _, row := range s.rows[in.Key.RunID] {
			if row.ItemID == id {
				out.Items = append(out.Items, row)
			}
		}
	}
	return out, nil
}
func (s *preparerStore) FinishPlan(ctx context.Context, in entity.HookFinishPlanInput) (entity.HookStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookStoreResult{}, err
	}
	if err := s.guard(ctx, in.HookStoreGuard); err != nil {
		return entity.HookStoreResult{}, err
	}
	r := s.runs[in.Key.RunID]
	if in.Count != r.PlanCount || in.Count != int64(len(s.rows[in.Key.RunID])) {
		return entity.HookStoreResult{}, entity.ErrHookStoreCorrupt
	}
	r.PlanReady = true
	r.PlanHash = in.Hash
	r.State.Before.Activated = true
	r.Version++
	s.effects = append(s.effects, "finish")
	return s.receipt(r)
}
func (s *preparerStore) BeginFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	if err := s.guard(ctx, in.HookStoreGuard); err != nil {
		return entity.HookStoreResult{}, err
	}
	r := s.runs[in.Key.RunID]
	r.State.Intent = in.Intent
	r.State.Finalize = entity.HookFinalizePending
	r.State.Gate = entity.HookGateClosed
	r.Version++
	s.effects = append(s.effects, "finalize")
	return s.receipt(r)
}

type preparerExperiments struct {
	repo.IExperimentRepo
	s *preparerStore
}

func (r preparerExperiments) GetByID(ctx context.Context, id, space int64) (*entity.Experiment, error) {
	if r.s.storageErr != nil {
		return nil, r.s.storageErr
	}
	return r.s.expt, nil
}

type preparerInitialization struct {
	repo.IHookRunInitializationRepo
	s *preparerStore
}

func (r preparerInitialization) ReadRunInitialization(ctx context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	if r.s.storageErr != nil {
		return nil, r.s.storageErr
	}
	log := r.s.logs[key.RunID]
	if log == nil {
		return nil, entity.ErrHookStoreMissing
	}
	copy := *log
	copy.ItemIds = append([]entity.ExptRunLogItems(nil), log.ItemIds...)
	return &entity.HookRunInitialization{RunLog: &copy, Managed: r.s.runs[key.RunID] != nil, LatestRunID: r.s.expt.LatestRunID}, nil
}

type preparerResults struct {
	items, turns bool
	err          error
	calls        int
}

func (r *preparerResults) HasExperimentResults(ctx context.Context, key entity.HookRunKey, scope string) (bool, error) {
	r.calls++
	if r.err != nil {
		return false, r.err
	}
	if key != (entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}) || scope != "local" {
		return false, errors.New("bad existence query")
	}
	return r.items || r.turns, nil
}

type preparerIDs struct{ next int64 }

func (i *preparerIDs) GenID(context.Context) (int64, error) { i.next++; return i.next, nil }
func (i *preparerIDs) GenMultiIDs(ctx context.Context, n int) ([]int64, error) {
	out := make([]int64, n)
	for j := range out {
		out[j], _ = i.GenID(ctx)
	}
	return out, nil
}

type preparerSelector struct {
	pages    map[string]entity.HookSelectionPage
	err      error
	calls    int
	selectFn func(context.Context, entity.HookSelectionInput) (entity.HookSelectionPage, error)
}

func (s *preparerSelector) SelectPage(ctx context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
	s.calls++
	if s.selectFn != nil {
		return s.selectFn(ctx, in)
	}
	if s.err != nil {
		return entity.HookSelectionPage{}, s.err
	}
	p, ok := s.pages[in.Cursor]
	if !ok {
		return entity.HookSelectionPage{}, fmt.Errorf("unrecognized opaque cursor")
	}
	return p, nil
}

type preparerFixture struct {
	s        *preparerStore
	selector *preparerSelector
	results  *preparerResults
	deps     HookPlanPreparerDependencies
	in       hook.WorkerRunInput
}

func newPreparerFixture(t *testing.T, mode entity.ExptRunMode) *preparerFixture {
	t.Helper()
	key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
	expt := &entity.Experiment{ID: 20, SpaceID: 10, LatestRunID: 30, Status: entity.ExptStatus_Pending, ExptType: entity.ExptType_Offline, EvalSetID: 71, EvalSetVersionID: 72, EvalSetSpaceID: 11, TrialRunItemCount: 2}
	fingerprint, err := entity.HookSelectionConfigFingerprint(expt)
	require.NoError(t, err)
	modes := []string{"", "submit", "fail_retry", "append", "retry_all", "retry_items", "trial_run"}
	snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "local", CreatedAt: time.Unix(1000, 0), Config: managerEnabledConfig(true, false), Selection: &entity.HookSelectionSeed{Version: 1, TrialRunItemCount: 2, ConfigFingerprint: fingerprint}, Context: &spi.HookRunContext{WorkspaceID: gptr.Of("10"), ExperimentID: gptr.Of("20"), RunID: gptr.Of("30"), RunMode: gptr.Of(modes[mode]), Initiator: &spi.HookInitiator{UserID: gptr.Of("user"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("plan"), Type: gptr.Of("offline")}, EvalSets: []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("11"), ID: gptr.Of("71"), VersionID: gptr.Of("72")}}}})
	require.NoError(t, err)
	codec := hookinfra.NewStorageCodec(new(managerProtector))
	protected, err := codec.EncodeSnapshot(context.Background(), "key", snapshot)
	require.NoError(t, err)
	run := &entity.HookStoredRun{State: entity.HookRunState{Key: key, Status: entity.ExptStatus_Pending, Gate: entity.HookGateWaiting, Finalize: entity.HookFinalizeNone, Before: entity.HookOperation{ID: "before", Status: entity.HookOperationPending}, After: entity.HookOperation{Status: entity.HookOperationDisabled}}, Snapshot: protected, CreatedBy: "user", Mode: mode}
	s := &preparerStore{runs: map[int64]*entity.HookStoredRun{30: run}, rows: map[int64][]entity.HookPlanItem{}, logs: map[int64]*entity.ExptRunLog{30: {ID: 30, SpaceID: 10, ExptID: 20, ExptRunID: 30, CreatedBy: "user", Mode: int32(mode), Status: int64(entity.ExptStatus_Pending)}}, expt: expt}
	selector := &preparerSelector{pages: map[string]entity.HookSelectionPage{"": {NextCursor: "terminal", Done: true}}}
	results := &preparerResults{}
	return &preparerFixture{s: s, selector: selector, results: results, in: hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: key}}, deps: HookPlanPreparerDependencies{Runs: s, Plans: s, Selector: selector, Codec: codec, Experiments: preparerExperiments{s: s}, Initialization: preparerInitialization{s: s}, ResultReader: results, IDs: &preparerIDs{next: 1000}}}
}
func (f *preparerFixture) step(t *testing.T) error {
	t.Helper()
	p, err := NewHookPlanPreparer(f.deps)
	require.NoError(t, err)
	return p.PreparePlan(context.Background(), f.in)
}
func planTestRow(id int64) entity.HookPlanItem {
	return entity.HookPlanItem{SourceSpaceID: 11, EvalSetID: 71, EvalSetVersionID: 72, ItemID: id, ItemVersionID: 2}
}

func TestHookPlanPreparerBoundedResume(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeSubmit)
	f.selector.pages = map[string]entity.HookSelectionPage{"": {Items: []entity.HookPlanItem{planTestRow(1)}, NextCursor: "opaque-next"}, "opaque-next": {Items: []entity.HookPlanItem{planTestRow(2)}, NextCursor: "opaque-terminal", Done: true}}
	require.NoError(t, f.step(t))
	require.Len(t, f.s.rows[30], 1)
	require.False(t, f.s.runs[30].PlanReady)
	require.NoError(t, f.step(t))
	require.Len(t, f.s.rows[30], 2)
	require.False(t, f.s.runs[30].PlanReady)
	require.NoError(t, f.step(t))
	require.True(t, f.s.runs[30].PlanReady)
	require.True(t, f.s.runs[30].State.Before.Activated)
	require.Equal(t, 2, f.selector.calls)
}
