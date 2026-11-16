// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type fakeCfgProvider struct {
	cfg     *entity.CentralSchedulingConfig
	enabled bool
	caps    map[entity.ResourceDimension]int64
}

func (f *fakeCfgProvider) GetCentralSchedulingConfig(context.Context) (*entity.CentralSchedulingConfig, error) {
	cfg := *f.cfg
	cfg.Enabled = f.enabled
	return &cfg, nil
}

func (f *fakeCfgProvider) GetDimensionCapacity(_ context.Context, dim entity.ResourceDimension) (bool, int64, error) {
	return true, f.caps[dim], nil
}

func (f *fakeCfgProvider) IsAuthorized(context.Context, string, int64) bool { return true }

type fakeEnvProvider struct{ env string }

func (f fakeEnvProvider) CurrentEnvID(context.Context) (string, error) { return f.env, nil }

type fakeActiveRepo struct {
	mu    sync.Mutex
	items map[string]*entity.ActiveExperiment
}

func newFakeActive() *fakeActiveRepo {
	return &fakeActiveRepo{items: map[string]*entity.ActiveExperiment{}}
}

func (f *fakeActiveRepo) AddActive(_ context.Context, _ string, a *entity.ActiveExperiment, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[fmt.Sprintf("%d:%d", a.ExptID, a.RunID)] = a
	return nil
}

func (f *fakeActiveRepo) RemoveActive(_ context.Context, _ string, exptID, runID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, v := range f.items {
		if v.ExptID == exptID && v.RunID == runID {
			delete(f.items, k)
		}
	}
	return nil
}

func (f *fakeActiveRepo) ListActive(context.Context, string, int) ([]*entity.ActiveExperiment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*entity.ActiveExperiment, 0, len(f.items))
	for _, v := range f.items {
		out = append(out, v)
	}
	return out, nil
}

func (f *fakeActiveRepo) TryLeader(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeActiveRepo) RenewLeader(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeActiveRepo) ReleaseLeader(context.Context, string, string) error { return nil }
func (f *fakeActiveRepo) TryExptLease(context.Context, string, int64, string, time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeActiveRepo) CleanupEnv(context.Context, string) error { return nil }

type fakeLedger struct {
	resvs   map[string]*entity.Reservation
	used    entity.ResourceVector
	reserve func(*repo.ReserveRequest) error
}

func newFakeLedger() *fakeLedger {
	return &fakeLedger{resvs: map[string]*entity.Reservation{}, used: entity.ResourceVector{}}
}

func (f *fakeLedger) Reserve(_ context.Context, req *repo.ReserveRequest) (*entity.Reservation, error) {
	if f.reserve != nil {
		if err := f.reserve(req); err != nil {
			return nil, err
		}
	}
	r := req.Reservation
	r.State = entity.ReservationStateReserved
	f.resvs[r.ID] = r
	for dim, n := range r.Vector {
		f.used[dim] += n
	}
	return r, nil
}
func (f *fakeLedger) Adopt(context.Context, *entity.Reservation, time.Duration) error { return nil }
func (f *fakeLedger) Confirm(_ context.Context, _ string, id string) (entity.ReservationState, error) {
	if r := f.resvs[id]; r != nil && !r.State.IsTerminal() {
		r.State = entity.ReservationStateConfirmed
	}
	return f.resvs[id].State, nil
}
func (f *fakeLedger) MarkPendingCompensation(_ context.Context, _ string, id string) (entity.ReservationState, error) {
	if r := f.resvs[id]; r != nil && !r.State.IsTerminal() && r.State != entity.ReservationStateConfirmed {
		r.State = entity.ReservationStatePendingCompensation
	}
	return f.resvs[id].State, nil
}
func (f *fakeLedger) Release(_ context.Context, _ string, id, _ string) (entity.ReservationState, error) {
	r := f.resvs[id]
	if r == nil {
		return "", nil
	}
	if r.State.IsTerminal() {
		return r.State, nil
	}
	for dim, n := range r.Vector {
		f.used[dim] -= n
	}
	r.State = entity.ReservationStateReleased
	return r.State, nil
}
func (f *fakeLedger) Expire(ctx context.Context, env, id, reason string) (entity.ReservationState, error) {
	return f.Release(ctx, env, id, reason)
}
func (f *fakeLedger) Get(_ context.Context, _ string, id string) (*entity.Reservation, error) {
	return f.resvs[id], nil
}
func (f *fakeLedger) ListByRun(_ context.Context, _ string, exptID, runID int64) ([]*entity.Reservation, error) {
	out := make([]*entity.Reservation, 0)
	for _, r := range f.resvs {
		if r.ExptID == exptID && r.RunID == runID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeLedger) ListAll(context.Context, string, int) ([]*entity.Reservation, error) {
	out := make([]*entity.Reservation, 0, len(f.resvs))
	for _, r := range f.resvs {
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeLedger) RunInflight(context.Context, string, int64, int64) (int, error) { return 0, nil }
func (f *fakeLedger) PoolUsage(context.Context, string) (entity.ResourceVector, error) {
	return f.used, nil
}
func (f *fakeLedger) RebuildPool(context.Context, string, entity.ResourceVector, time.Duration) error {
	return nil
}
func (f *fakeLedger) CleanupEnv(context.Context, string) error { return nil }

type fakeExptRepo struct {
	expts  map[int64]*entity.Experiment
	queue  map[int64][]int64
	states map[int64]entity.ItemRunState
	marked map[int64]entity.ItemRunState
	mu     sync.Mutex
}

func newFakeExptRepo() *fakeExptRepo {
	return &fakeExptRepo{
		expts:  map[int64]*entity.Experiment{},
		queue:  map[int64][]int64{},
		states: map[int64]entity.ItemRunState{},
		marked: map[int64]entity.ItemRunState{},
	}
}

func (f *fakeExptRepo) GetByID(_ context.Context, id, _ int64) (*entity.Experiment, error) {
	return f.expts[id], nil
}
func (f *fakeExptRepo) MGetBasicByID(_ context.Context, ids []int64) ([]*entity.Experiment, error) {
	out := make([]*entity.Experiment, 0, len(ids))
	for _, id := range ids {
		if e := f.expts[id]; e != nil {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeExptRepo) ListCentralActive(context.Context, string, int) ([]*entity.Experiment, error) {
	out := make([]*entity.Experiment, 0)
	for _, e := range f.expts {
		out = append(out, e)
	}
	return out, nil
}
func (f *fakeExptRepo) ScanQueueingItems(_ context.Context, exptID, _, _ int64, limit int) ([]int64, error) {
	q := f.queue[exptID]
	if limit > 0 && len(q) > limit {
		q = q[:limit]
	}
	return q, nil
}
func (f *fakeExptRepo) ScanProcessingItems(context.Context, int64, int64, int64, int) ([]int64, error) {
	return nil, nil
}
func (f *fakeExptRepo) MarkItemsProcessing(_ context.Context, _ int64, _ int64, _ int64, ids []int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		f.marked[id] = entity.ItemRunState_Processing
	}
	return nil
}
func (f *fakeExptRepo) MarkItemsQueueing(_ context.Context, _ int64, _ int64, _ int64, ids []int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		f.marked[id] = entity.ItemRunState_Queueing
	}
	return nil
}
func (f *fakeExptRepo) GetItemStates(context.Context, int64, int64, int64, []int64) (map[int64]entity.ItemRunState, error) {
	return f.states, nil
}
func (f *fakeExptRepo) EnsureRunItems(context.Context, *entity.Experiment) error { return nil }

type fakeDispatcher struct {
	events []*entity.ExptItemEvalEvent
	err    error
}

func (f *fakeDispatcher) Dispatch(_ context.Context, e *entity.ExptItemEvalEvent) error {
	f.events = append(f.events, e)
	return f.err
}

func makeActiveExpt(id, runID int64, mode entity.SchedulingMode, priority int32, wait int64, env string) (*entity.Experiment, *entity.ActiveExperiment) {
	conf := &entity.EvaluationConfiguration{Ext: map[string]string{
		entity.ExtKeySchedulingMode:     string(mode),
		entity.ExtKeySchedulingPriority: fmt.Sprintf("%d", priority),
		entity.ExtKeySchedulingEnv:      env,
	}}
	expt := &entity.Experiment{
		ID: id, SpaceID: 9, LatestRunID: runID, Status: entity.ExptStatus_Processing,
		EvalConf: conf,
	}
	return expt, &entity.ActiveExperiment{
		ExptID: id, RunID: runID, SpaceID: 9, Attempt: 7, Priority: priority,
		WaitSinceMS: wait, Mode: mode, Env: env,
	}
}

func TestTickPriorityOrderAndEnvIsolation(t *testing.T) {
	ctx := context.Background()
	cfg := entity.DefaultCentralSchedulingConfig()
	cfgProvider := &fakeCfgProvider{cfg: cfg, enabled: true, caps: map[entity.ResourceDimension]int64{
		entity.ResourceItemExec: 100, entity.ResourceSandboxExec: 100, entity.ResourceModelCredit: 10000,
	}}
	env := fakeEnvProvider{env: "ppe:lane-a"}
	active := newFakeActive()
	ledger := newFakeLedger()
	exptRepo := newFakeExptRepo()
	dispatcher := &fakeDispatcher{}
	s := &CentralScheduler{cfg: cfgProvider, env: env, ledger: ledger, active: active, exptRepo: exptRepo, dispatcher: dispatcher, owner: "test"}

	lowExpt, lowActive := makeActiveExpt(1, 100, entity.SchedulingModeCentral, 10, 1, "ppe:lane-a")
	highExpt, highActive := makeActiveExpt(2, 200, entity.SchedulingModeCentral, 90, 2, "ppe:lane-a")
	otherEnvExpt, otherEnvActive := makeActiveExpt(3, 300, entity.SchedulingModeCentral, 100, 0, "ppe:lane-b")
	legacyExpt, legacyActive := makeActiveExpt(4, 400, entity.SchedulingModeLegacy, 100, 0, "ppe:lane-a")
	exptRepo.expts[1], exptRepo.expts[2], exptRepo.expts[3], exptRepo.expts[4] = lowExpt, highExpt, otherEnvExpt, legacyExpt
	exptRepo.queue[1] = []int64{11}
	exptRepo.queue[2] = []int64{21}
	exptRepo.queue[3] = []int64{31}
	exptRepo.queue[4] = []int64{41}
	require.NoError(t, active.AddActive(ctx, env.env, lowActive, time.Hour))
	require.NoError(t, active.AddActive(ctx, env.env, highActive, time.Hour))
	require.NoError(t, active.AddActive(ctx, env.env, otherEnvActive, time.Hour))
	require.NoError(t, active.AddActive(ctx, env.env, legacyActive, time.Hour))

	require.NoError(t, s.Tick(ctx))
	require.Len(t, dispatcher.events, 2)
	assert.Equal(t, int64(2), dispatcher.events[0].ExptID, "higher priority must dispatch first")
	assert.Equal(t, int64(21), dispatcher.events[0].EvalSetItemID)
	assert.Equal(t, int64(1), dispatcher.events[1].ExptID)
	assert.Equal(t, "ppe:lane-a", dispatcher.events[0].CentralEnv)

	// 跨 env 与 legacy 不应留在活跃集。
	_, err := active.ListActive(ctx, env.env, 100)
	require.NoError(t, err)
	var ids []int64
	for _, a := range mustActive(t, active, env.env) {
		ids = append(ids, a.ExptID)
	}
	assert.NotContains(t, ids, int64(3))
	assert.NotContains(t, ids, int64(4))
}

func mustActive(t *testing.T, a *fakeActiveRepo, env string) []*entity.ActiveExperiment {
	t.Helper()
	list, err := a.ListActive(context.Background(), env, 100)
	require.NoError(t, err)
	return list
}

func TestTickSkipsInflightAndHandlesDispatchErrors(t *testing.T) {
	ctx := context.Background()
	cfg := entity.DefaultCentralSchedulingConfig()
	cfgProvider := &fakeCfgProvider{cfg: cfg, enabled: true, caps: map[entity.ResourceDimension]int64{
		entity.ResourceItemExec: 100, entity.ResourceSandboxExec: 100, entity.ResourceModelCredit: 10000,
	}}
	env := fakeEnvProvider{env: "ppe:lane-a"}
	active := newFakeActive()
	ledger := newFakeLedger()
	exptRepo := newFakeExptRepo()
	dispatcher := &fakeDispatcher{}
	s := &CentralScheduler{cfg: cfgProvider, env: env, ledger: ledger, active: active, exptRepo: exptRepo, dispatcher: dispatcher, owner: "test"}

	expt, a := makeActiveExpt(10, 1000, entity.SchedulingModeCentral, 0, 1, env.env)
	expt.EvalConf.ItemConcurNum = intPtr(10)
	exptRepo.expts[10] = expt
	exptRepo.queue[10] = []int64{1, 2, 3}
	require.NoError(t, active.AddActive(ctx, env.env, a, time.Hour))

	// 已预占 item 1：不应重复派发，只派发 2/3。
	ledger.resvs["existing"] = &entity.Reservation{
		ID: "existing", Env: env.env, ExptID: 10, RunID: 1000, ItemID: 1,
		State: entity.ReservationStateReserved, Vector: entity.ResourceVector{entity.ResourceItemExec: 1},
	}
	require.NoError(t, s.Tick(ctx))
	require.Len(t, dispatcher.events, 2)
	assert.Equal(t, int64(2), dispatcher.events[0].EvalSetItemID)
	assert.Equal(t, int64(3), dispatcher.events[1].EvalSetItemID)

	// 结果不明：保持 PendingCompensation，DB 不回退。
	dispatcher.err = errors.New("timeout")
	dispatcher.events = nil
	exptRepo.queue[10] = []int64{4}
	require.NoError(t, s.Tick(ctx))
	require.Len(t, dispatcher.events, 1)
	for _, r := range ledger.resvs {
		if r.ItemID == 4 {
			assert.Equal(t, entity.ReservationStatePendingCompensation, r.State)
		}
	}
	assert.Equal(t, entity.ItemRunState_Processing, exptRepo.marked[4])

	// 确定失败：立即释放并回退 Queueing。
	dispatcher.err = repo.ErrDefiniteDispatchFailure
	dispatcher.events = nil
	exptRepo.queue[10] = []int64{5}
	require.NoError(t, s.Tick(ctx))
	require.Len(t, dispatcher.events, 1)
	for _, r := range ledger.resvs {
		if r.ItemID == 5 {
			assert.Equal(t, entity.ReservationStateReleased, r.State)
		}
	}
	assert.Equal(t, entity.ItemRunState_Queueing, exptRepo.marked[5])
}

func intPtr(v int) *int { return &v }
