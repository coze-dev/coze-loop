// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type fakeCentralRepo struct {
	active    []*entity.CentralActiveExperiment
	states    map[int64][]*entity.CentralItemState
	runStatus map[int64]entity.ExptStatus
	runFound  map[int64]bool
}

func (f *fakeCentralRepo) ListActiveExperiments(ctx context.Context, ownerEnv string, limit int) ([]*entity.CentralActiveExperiment, error) {
	out := make([]*entity.CentralActiveExperiment, 0, len(f.active))
	for _, a := range f.active {
		if a != nil && a.OwnerEnv == ownerEnv {
			out = append(out, a)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeCentralRepo) ListRunItemStates(ctx context.Context, spaceID, exptID, exptRunID int64) ([]*entity.CentralItemState, error) {
	return f.states[exptRunID], nil
}

func (f *fakeCentralRepo) GetRunStatus(ctx context.Context, spaceID, exptID, exptRunID int64) (entity.ExptStatus, bool, error) {
	status, ok := f.runStatus[exptRunID]
	if !ok {
		status = entity.ExptStatus_Processing
	}
	found := true
	if f.runFound != nil {
		found = f.runFound[exptRunID]
	}
	return status, found, nil
}

type fakeLedger struct {
	capacity entity.ResourceVector
	used     entity.ResourceVector
	rsvs     map[string]*entity.Reservation
	rebuilds int
}

func newFakeLedger(capacity entity.ResourceVector) *fakeLedger {
	return &fakeLedger{capacity: capacity, rsvs: map[string]*entity.Reservation{}}
}

func (f *fakeLedger) Reserve(ctx context.Context, ownerEnv string, capacity entity.ResourceVector, requests []*entity.Reservation) (*ReserveResult, error) {
	f.capacity = capacity
	result := &ReserveResult{}
	for _, r := range requests {
		if r == nil {
			continue
		}
		if existing, ok := f.rsvs[r.ID]; ok {
			result.Granted = append(result.Granted, existing)
			continue
		}
		next := f.used.Add(r.Vector)
		if !capacity.Covers(next) {
			result.Denied = append(result.Denied, r)
			break
		}
		f.used = next
		f.rsvs[r.ID] = r
		result.Granted = append(result.Granted, r)
	}
	return result, nil
}

func (f *fakeLedger) Release(ctx context.Context, ownerEnv, reservationID string) (bool, error) {
	r, ok := f.rsvs[reservationID]
	if !ok {
		return false, nil
	}
	f.used = f.used.Sub(r.Vector)
	delete(f.rsvs, reservationID)
	return true, nil
}

func (f *fakeLedger) ReleaseRun(ctx context.Context, ownerEnv string, exptRunID int64) (int, error) {
	count := 0
	for id, r := range f.rsvs {
		if r.ExptRunID != exptRunID {
			continue
		}
		f.used = f.used.Sub(r.Vector)
		delete(f.rsvs, id)
		count++
	}
	return count, nil
}

func (f *fakeLedger) List(ctx context.Context, ownerEnv string) ([]*entity.Reservation, error) {
	out := make([]*entity.Reservation, 0, len(f.rsvs))
	for _, r := range f.rsvs {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeLedger) Snapshot(ctx context.Context, ownerEnv string, capacity entity.ResourceVector) (*LedgerSnapshot, error) {
	f.capacity = capacity
	return &LedgerSnapshot{
		Capacity:         capacity,
		Used:             f.used,
		Available:        capacity.Sub(f.used),
		ReservationCount: len(f.rsvs),
	}, nil
}

func (f *fakeLedger) SetState(ctx context.Context, ownerEnv, reservationID string, state entity.ReservationState) error {
	if r, ok := f.rsvs[reservationID]; ok {
		r.State = state
	}
	return nil
}

func (f *fakeLedger) Rebuild(ctx context.Context, ownerEnv string, capacity entity.ResourceVector, reservations []*entity.Reservation) error {
	f.rebuilds++
	f.capacity = capacity
	f.used = entity.ResourceVector{}
	f.rsvs = map[string]*entity.Reservation{}
	for _, r := range reservations {
		if r == nil {
			continue
		}
		f.rsvs[r.ID] = r
		f.used = f.used.Add(r.Vector)
	}
	return nil
}

type fakeLeader struct{ acquired bool }

func (f *fakeLeader) Acquire(ctx context.Context, ownerEnv string, ttl time.Duration) (LeaderLease, bool, error) {
	return fakeLease{}, f.acquired, nil
}

type fakeLease struct{}

func (fakeLease) Renew(ctx context.Context) (bool, error) { return true, nil }
func (fakeLease) Release(ctx context.Context) error       { return nil }

type fakeConfiger struct{ conf *entity.CentralSchedulerConf }

func (f *fakeConfiger) GetCentralSchedulerConf(ctx context.Context) *entity.CentralSchedulerConf {
	return f.conf
}

type fakeExecutor struct {
	calls    int
	events   []*entity.ExptScheduleEvent
	dispatch map[int64][]*entity.ExptEvalItem // runID -> items to pass through gate
	drop     bool
}

func (f *fakeExecutor) ScheduleOnce(ctx context.Context, event *entity.ExptScheduleEvent, gate DispatchGate) error {
	f.calls++
	f.events = append(f.events, event)
	if f.drop || gate == nil {
		return nil
	}
	if _, err := gate.Filter(ctx, event, f.dispatch[event.ExptRunID]); err != nil {
		return err
	}
	return nil
}

func newTestScheduler(t *testing.T, repo *fakeCentralRepo, ledger *fakeLedger, exec *fakeExecutor, ownerEnv string) *CentralSchedulerImpl {
	t.Helper()
	conf := entity.DefaultCentralSchedulerConf()
	conf.Enabled = true
	conf.TickIntervalSeconds = 5
	conf.DefaultCapacity = ledger.capacity
	conf.DriftItemThreshold = 1000
	conf.DriftEvaluatorThreshold = 1000
	return &CentralSchedulerImpl{
		Repo:     repo,
		Ledger:   ledger,
		Executor: exec,
		Configer: &fakeConfiger{conf: conf},
		Leader:   &fakeLeader{acquired: true},
		OwnerEnv: ownerEnv,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
	}
}

func item(id int64) *entity.ExptEvalItem {
	return &entity.ExptEvalItem{ItemID: id, State: entity.ItemRunState_Queueing}
}

func TestCentralScheduler_Tick_PriorityAndGap(t *testing.T) {
	ctx := context.Background()
	repo := &fakeCentralRepo{
		active: []*entity.CentralActiveExperiment{
			{SpaceID: 1, ExptID: 100, ExptRunID: 1, ExptRunMode: entity.EvaluationModeSubmit, ExptStatus: entity.ExptStatus_Processing, ItemConcurNum: 1, EvaluatorConcurNum: 1, Priority: 10, OwnerEnv: "prod", CreatedBy: "u1", StartAt: time.Unix(0, 0)},
			{SpaceID: 1, ExptID: 200, ExptRunID: 2, ExptRunMode: entity.EvaluationModeSubmit, ExptStatus: entity.ExptStatus_Processing, ItemConcurNum: 1, EvaluatorConcurNum: 1, Priority: 0, OwnerEnv: "prod", CreatedBy: "u2", StartAt: time.Unix(1, 0)},
		},
		states: map[int64][]*entity.CentralItemState{
			1: {{ItemID: 11, Status: entity.ItemRunState_Queueing}},
			2: {{ItemID: 21, Status: entity.ItemRunState_Queueing}},
		},
	}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 10})
	exec := &fakeExecutor{dispatch: map[int64][]*entity.ExptEvalItem{1: {item(11)}, 2: {item(21)}}}
	s := newTestScheduler(t, repo, ledger, exec, "prod")

	require.NoError(t, s.Tick(ctx))
	require.Equal(t, 1, exec.calls, "capacity only allows the highest-priority experiment")
	require.Equal(t, int64(100), exec.events[0].ExptID)
	require.Equal(t, 1, len(ledger.rsvs))
	require.Equal(t, entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, ledger.used)
}

func TestCentralScheduler_Settle_TerminalRelease(t *testing.T) {
	ctx := context.Background()
	repo := &fakeCentralRepo{
		active:    []*entity.CentralActiveExperiment{},
		states:    map[int64][]*entity.CentralItemState{1: {{ItemID: 11, Status: entity.ItemRunState_Success}}},
		runStatus: map[int64]entity.ExptStatus{1: entity.ExptStatus_Processing},
	}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 10, EvaluatorSlots: 10})
	ledger.rsvs["1:11"] = &entity.Reservation{ID: "1:11", ExptRunID: 1, ItemID: 11, Vector: entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, State: entity.ReservationStateDispatched}
	ledger.used = entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}
	exec := &fakeExecutor{}
	s := newTestScheduler(t, repo, ledger, exec, "prod")

	require.NoError(t, s.Tick(ctx))
	require.Equal(t, 0, len(ledger.rsvs))
	require.Equal(t, entity.ResourceVector{}, ledger.used)
}

func TestCentralScheduler_Settle_UnknownQueueingRelease(t *testing.T) {
	ctx := context.Background()
	old := time.Unix(1700000000, 0).Add(-time.Minute)
	repo := &fakeCentralRepo{
		states:    map[int64][]*entity.CentralItemState{1: {{ItemID: 11, Status: entity.ItemRunState_Queueing}}},
		runStatus: map[int64]entity.ExptStatus{1: entity.ExptStatus_Processing},
	}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 10, EvaluatorSlots: 10})
	ledger.rsvs["1:11"] = &entity.Reservation{ID: "1:11", ExptRunID: 1, ItemID: 11, Vector: entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, State: entity.ReservationStateDispatching, UpdatedAt: old}
	ledger.used = entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}
	exec := &fakeExecutor{}
	s := newTestScheduler(t, repo, ledger, exec, "prod")

	require.NoError(t, s.Tick(ctx))
	require.Equal(t, 0, len(ledger.rsvs), "queueing beyond settle window must be treated as no-execution evidence")
}

func TestCentralScheduler_Settle_ProcessingKept(t *testing.T) {
	ctx := context.Background()
	repo := &fakeCentralRepo{
		states:    map[int64][]*entity.CentralItemState{1: {{ItemID: 11, Status: entity.ItemRunState_Processing}}},
		runStatus: map[int64]entity.ExptStatus{1: entity.ExptStatus_Processing},
	}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 10, EvaluatorSlots: 10})
	ledger.rsvs["1:11"] = &entity.Reservation{ID: "1:11", ExptRunID: 1, ItemID: 11, Vector: entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, State: entity.ReservationStateReserved}
	ledger.used = entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}
	exec := &fakeExecutor{}
	s := newTestScheduler(t, repo, ledger, exec, "prod")

	require.NoError(t, s.Tick(ctx))
	require.Equal(t, 1, len(ledger.rsvs))
	require.Equal(t, entity.ReservationStateDispatched, ledger.rsvs["1:11"].State)
}

func TestCentralScheduler_OwnerEnvEmptyFailClosed(t *testing.T) {
	ctx := context.Background()
	s := newTestScheduler(t, &fakeCentralRepo{}, newFakeLedger(entity.ResourceVector{ItemSlots: 10}), &fakeExecutor{}, "")
	require.Error(t, s.Tick(ctx))
}

func TestCentralScheduler_RebuildFromDB(t *testing.T) {
	ctx := context.Background()
	repo := &fakeCentralRepo{
		active: []*entity.CentralActiveExperiment{
			{SpaceID: 1, ExptID: 100, ExptRunID: 1, ExptRunMode: entity.EvaluationModeSubmit, ExptStatus: entity.ExptStatus_Processing, ItemConcurNum: 2, EvaluatorConcurNum: 1, OwnerEnv: "prod"},
		},
		states: map[int64][]*entity.CentralItemState{
			1: {
				{ItemID: 11, Status: entity.ItemRunState_Processing},
				{ItemID: 12, Status: entity.ItemRunState_Queueing},
			},
		},
	}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 10, EvaluatorSlots: 10})
	s := newTestScheduler(t, repo, ledger, &fakeExecutor{}, "prod")
	require.NoError(t, s.Rebuild(ctx))
	require.Equal(t, 1, ledger.rebuilds)
	require.Equal(t, 1, len(ledger.rsvs), "only Processing items consume quota")
	require.Equal(t, entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, ledger.used)
}

func TestSchedulingConfig_ImmutabilitySemantics(t *testing.T) {
	legacy := entity.BuildSchedulingConfig("", 0, nil)
	assert.Nil(t, legacy, "absent fields must stay nil = legacy")
	central := entity.BuildSchedulingConfig("central", 5, map[string]string{entity.SchedulingOwnerEnvExtKey: "ppe_x"})
	require.NotNil(t, central)
	assert.Equal(t, entity.SchedulingModeCentral, central.Mode)
	assert.Equal(t, "ppe_x", central.OwnerEnv)
	bad := entity.BuildSchedulingConfig("unknown", 1, nil)
	require.NotNil(t, bad)
	assert.Equal(t, entity.SchedulingModeLegacy, bad.EffectiveMode(), "unknown mode must not silently become central")
}

func TestCentralScheduler_Settle_FailureRelease(t *testing.T) {
	ctx := context.Background()
	repo := &fakeCentralRepo{
		states:    map[int64][]*entity.CentralItemState{1: {{ItemID: 11, Status: entity.ItemRunState_Fail}}},
		runStatus: map[int64]entity.ExptStatus{1: entity.ExptStatus_Processing},
	}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 10, EvaluatorSlots: 10})
	ledger.rsvs["1:11"] = &entity.Reservation{ID: "1:11", ExptRunID: 1, ItemID: 11, Vector: entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, State: entity.ReservationStateDispatched}
	ledger.used = entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}
	s := newTestScheduler(t, repo, ledger, &fakeExecutor{}, "prod")
	require.NoError(t, s.Tick(ctx))
	assert.Equal(t, 0, len(ledger.rsvs))
	assert.Equal(t, entity.ResourceVector{}, ledger.used)
}

func TestCentralScheduler_Settle_CancelledRunRelease(t *testing.T) {
	ctx := context.Background()
	repo := &fakeCentralRepo{
		runStatus: map[int64]entity.ExptStatus{1: entity.ExptStatus_Terminated},
	}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 10, EvaluatorSlots: 10})
	ledger.rsvs["1:11"] = &entity.Reservation{ID: "1:11", ExptRunID: 1, ItemID: 11, Vector: entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, State: entity.ReservationStateDispatched}
	ledger.used = entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}
	s := newTestScheduler(t, repo, ledger, &fakeExecutor{}, "prod")
	require.NoError(t, s.Tick(ctx))
	assert.Equal(t, 0, len(ledger.rsvs))
	assert.Equal(t, entity.ResourceVector{}, ledger.used)
}

func TestCentralScheduler_Settle_DeletedRunRelease(t *testing.T) {
	ctx := context.Background()
	repo := &fakeCentralRepo{runFound: map[int64]bool{1: false}}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 10, EvaluatorSlots: 10})
	ledger.rsvs["1:11"] = &entity.Reservation{ID: "1:11", ExptRunID: 1, ItemID: 11, Vector: entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, State: entity.ReservationStateDispatched}
	ledger.used = entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}
	s := newTestScheduler(t, repo, ledger, &fakeExecutor{}, "prod")
	require.NoError(t, s.Tick(ctx))
	assert.Equal(t, 0, len(ledger.rsvs))
	assert.Equal(t, entity.ResourceVector{}, ledger.used)
}

func TestCentralScheduler_RetryDoesNotDoubleCharge(t *testing.T) {
	ctx := context.Background()
	repo := &fakeCentralRepo{
		active: []*entity.CentralActiveExperiment{
			{SpaceID: 1, ExptID: 100, ExptRunID: 1, ExptRunMode: entity.EvaluationModeSubmit, ExptStatus: entity.ExptStatus_Processing, ItemConcurNum: 1, EvaluatorConcurNum: 1, OwnerEnv: "prod", RetryItemIDs: []int64{11}},
		},
		states:    map[int64][]*entity.CentralItemState{1: {{ItemID: 11, Status: entity.ItemRunState_Queueing}}},
		runStatus: map[int64]entity.ExptStatus{1: entity.ExptStatus_Processing},
	}
	ledger := newFakeLedger(entity.ResourceVector{ItemSlots: 10, EvaluatorSlots: 10})
	ledger.rsvs["1:11"] = &entity.Reservation{ID: "1:11", ExptRunID: 1, ItemID: 11, Vector: entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, State: entity.ReservationStateDispatching}
	ledger.used = entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}
	exec := &fakeExecutor{dispatch: map[int64][]*entity.ExptEvalItem{1: {item(11)}}}
	s := newTestScheduler(t, repo, ledger, exec, "prod")

	require.NoError(t, s.Tick(ctx))
	require.Equal(t, 1, len(ledger.rsvs))
	assert.Equal(t, entity.ResourceVector{ItemSlots: 1, EvaluatorSlots: 1}, ledger.used, "重试复用同一 reservation，不得重复扣额度")
	require.Equal(t, 1, exec.calls)
}
