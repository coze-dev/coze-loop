// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type fakeCentralConfiger struct {
	component.IConfiger
	conf *entity.CentralSchedulingConf
}

func (f *fakeCentralConfiger) GetCentralSchedulingConf(context.Context) *entity.CentralSchedulingConf {
	return f.conf
}

type fakeActiveRunRepo struct {
	runs        []*entity.CentralActiveRun
	snapshots   map[string]*entity.CentralRunSnapshot
	experiments map[int64]*entity.CentralExperimentScheduling
	queueing    map[string][]int64
	processing  map[string][]int64
	states      map[string]map[int64]entity.ItemRunState
}

func (f *fakeActiveRunRepo) ListRecentRuns(context.Context, time.Time, int) ([]*entity.CentralActiveRun, error) {
	return f.runs, nil
}

func (f *fakeActiveRunRepo) Snapshots(context.Context, []*entity.CentralActiveRun) (map[string]*entity.CentralRunSnapshot, error) {
	out := map[string]*entity.CentralRunSnapshot{}
	for k, v := range f.snapshots {
		out[k] = v
	}
	return out, nil
}

func (f *fakeActiveRunRepo) LoadExperiment(_ context.Context, exptID, _ int64) (*entity.CentralExperimentScheduling, error) {
	if e, ok := f.experiments[exptID]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("experiment %d not found", exptID)
}

func (f *fakeActiveRunRepo) QueueingItemIDs(_ context.Context, run *entity.CentralActiveRun, limit int) ([]int64, error) {
	ids := f.queueing[fakeRunKey(run.ExptID, run.RunID)]
	if limit < len(ids) {
		ids = ids[:limit]
	}
	return ids, nil
}

func (f *fakeActiveRunRepo) ProcessingItemIDs(_ context.Context, run *entity.CentralActiveRun, limit int) ([]int64, error) {
	ids := f.processing[fakeRunKey(run.ExptID, run.RunID)]
	if limit < len(ids) {
		ids = ids[:limit]
	}
	return ids, nil
}

func (f *fakeActiveRunRepo) ItemStates(_ context.Context, run *entity.CentralActiveRun, itemIDs []int64) (map[int64]entity.ItemRunState, error) {
	out := map[int64]entity.ItemRunState{}
	source := f.states[fakeRunKey(run.ExptID, run.RunID)]
	for _, id := range itemIDs {
		if s, ok := source[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

type fakeReservationStore struct {
	mu           sync.Mutex
	usage        map[string]int64
	reservations map[string]*entity.CentralReservation
	members      map[string]map[string]bool
	reserveCalls int
	releaseCalls int
	lastReq      *entity.CentralReserveRequest
}

func newFakeReservationStore() *fakeReservationStore {
	return &fakeReservationStore{
		usage:        map[string]int64{},
		reservations: map[string]*entity.CentralReservation{},
		members:      map[string]map[string]bool{},
	}
}

func (f *fakeReservationStore) Reserve(_ context.Context, req *entity.CentralReserveRequest) (*entity.CentralReserveResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserveCalls++
	f.lastReq = req
	if _, ok := f.reservations[req.ReservationID]; ok {
		return &entity.CentralReserveResult{Granted: true, AlreadyHeld: true, ReservationID: req.ReservationID}, nil
	}
	keys := entity.CentralPoolKeys(req.Claims, req.Capacities)
	for _, k := range keys {
		if f.usage[k]+req.Claims[k] > req.Capacities[k] {
			return &entity.CentralReserveResult{Granted: false, ConstrainedBy: k, Remaining: map[string]int64{k: req.Capacities[k] - f.usage[k]}}, nil
		}
	}
	for _, k := range keys {
		f.usage[k] += req.Claims[k]
	}
	f.reservations[req.ReservationID] = &entity.CentralReservation{
		ID: req.ReservationID, Env: req.Env, ExptID: req.ExptID, SpaceID: req.SpaceID,
		RunID: req.RunID, ItemID: req.ItemID, Claims: req.Claims.Clone(), PoolKeys: keys,
		State: entity.CentralReservationHeld, Adopted: req.Adopted,
	}
	runKey := fakeRunKey(req.ExptID, req.RunID)
	if f.members[runKey] == nil {
		f.members[runKey] = map[string]bool{}
	}
	f.members[runKey][req.ReservationID] = true
	return &entity.CentralReserveResult{Granted: true, ReservationID: req.ReservationID}, nil
}

func (f *fakeReservationStore) Release(_ context.Context, _ string, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls++
	resv, ok := f.reservations[id]
	if !ok {
		return false, nil
	}
	for _, k := range resv.PoolKeys {
		f.usage[k] -= resv.Claims[k]
		if f.usage[k] < 0 {
			f.usage[k] = 0
		}
	}
	delete(f.reservations, id)
	for runKey := range f.members {
		delete(f.members[runKey], id)
	}
	return true, nil
}

func (f *fakeReservationStore) List(_ context.Context, _ string, exptID, runID int64) ([]*entity.CentralReservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	runKey := fakeRunKey(exptID, runID)
	out := make([]*entity.CentralReservation, 0, len(f.members[runKey]))
	ids := make([]string, 0, len(f.members[runKey]))
	for id := range f.members[runKey] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if r, ok := f.reservations[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeReservationStore) ReleaseRun(ctx context.Context, env string, exptID, runID int64) (int, error) {
	list, _ := f.List(ctx, env, exptID, runID)
	n := 0
	for _, r := range list {
		if ok, _ := f.Release(ctx, env, r.ID); ok {
			n++
		}
	}
	return n, nil
}

func (f *fakeReservationStore) MarkDispatched(_ context.Context, _ string, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.reservations[id]; ok {
		r.State = entity.CentralReservationDispatched
	}
	return nil
}

func (f *fakeReservationStore) RenewRun(context.Context, string, int64, int64, time.Duration) error {
	return nil
}

func (f *fakeReservationStore) Ledger(context.Context, string, int64, int64) (*entity.CentralRunLedger, error) {
	return &entity.CentralRunLedger{}, nil
}

type fakeOwnershipStore struct {
	mu     sync.Mutex
	owners map[int64]string
}

func (f *fakeOwnershipStore) Claim(_ context.Context, exptID, _ int64, owner string, _ time.Duration) (bool, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cur, ok := f.owners[exptID]; ok && cur != owner {
		return false, cur, nil
	}
	f.owners[exptID] = owner
	return true, owner, nil
}

func (f *fakeOwnershipStore) Owner(_ context.Context, exptID int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.owners[exptID], nil
}

func (f *fakeOwnershipStore) Release(_ context.Context, exptID int64, owner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.owners[exptID] == owner {
		delete(f.owners, exptID)
	}
	return nil
}

type fakeSubmitter struct {
	calls    int
	fail     bool
	order    []string
	onSubmit func(run *entity.CentralActiveRun, itemID int64)
}

func (f *fakeSubmitter) SubmitItems(_ context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) error {
	f.calls += len(items)
	for _, item := range items {
		f.order = append(f.order, fmt.Sprintf("%d/%d", event.ExptID, item.ItemID))
		if f.fail {
			return fmt.Errorf("publish failed")
		}
		if f.onSubmit != nil {
			f.onSubmit(&entity.CentralActiveRun{ExptID: event.ExptID, SpaceID: event.SpaceID, RunID: event.ExptRunID}, item.ItemID)
		}
	}
	return nil
}

func fakeRunKey(exptID, runID int64) string {
	return fmt.Sprintf("%d:%d", exptID, runID)
}

func newTestScheduler(t *testing.T, conf *entity.CentralSchedulingConf, active *fakeActiveRunRepo, resv *fakeReservationStore, own *fakeOwnershipStore, submitter *fakeSubmitter) CentralScheduler {
	t.Helper()
	return NewCentralScheduler(CentralSchedulerDeps{
		Configer:      &fakeCentralConfiger{conf: conf},
		ActiveRunRepo: active,
		Reservations:  resv,
		Ownership:     own,
		Submitter:     submitter,
		Clock:         func() time.Time { return time.Unix(100_000, 0) },
		Logger:        func(_ context.Context, format string, args ...any) { t.Logf(format, args...) },
	})
}

func baseConf() *entity.CentralSchedulingConf {
	return (&entity.CentralSchedulingConf{
		Enabled:        true,
		Mode:           entity.CentralModeEnforce,
		EnvNamespace:   "ppe_test",
		AllowAllSpaces: true,
		DefaultAppID:   7,
		FairShareRatio: 1.0,
		Pool: map[string]int64{
			entity.CentralResourceItem:      3,
			entity.CentralResourceTarget:    3,
			entity.CentralResourceEvaluator: 3,
		},
	}).Normalize()
}

func baseRun(exptID, spaceID, runID int64, exptType entity.ExptType) *entity.CentralActiveRun {
	return &entity.CentralActiveRun{
		ExptID: exptID, SpaceID: spaceID, RunID: runID,
		RunMode: entity.EvaluationModeSubmit, RunStatus: entity.ExptStatus_Processing,
		CreatedBy: "user-1", RunCreatedAt: time.Unix(90_000, 0),
		ExptType: exptType, TriggerType: "manual",
	}
}

func baseExpt(exptID, spaceID int64, concur int) *entity.CentralExperimentScheduling {
	return &entity.CentralExperimentScheduling{
		ExptID: exptID, SpaceID: spaceID, ItemConcurNum: concur,
		TargetID: 10, TargetType: entity.EvalTargetTypeCozeBot,
		EvaluatorVersionIDs: []int64{101},
	}
}

func TestCentralTick_DispatchesByPriorityAndGap(t *testing.T) {
	active := &fakeActiveRunRepo{
		runs: []*entity.CentralActiveRun{
			baseRun(100, 1, 1000, entity.ExptType_Offline),
			baseRun(200, 2, 2000, entity.ExptType_Online),
		},
		snapshots: map[string]*entity.CentralRunSnapshot{
			fakeRunKey(100, 1000): {InFlight: 0, Queueing: 5, OldestQueuedAt: ptrTime(time.Unix(95_000, 0))},
			fakeRunKey(200, 2000): {InFlight: 0, Queueing: 2, OldestQueuedAt: ptrTime(time.Unix(99_000, 0))},
		},
		experiments: map[int64]*entity.CentralExperimentScheduling{
			100: baseExpt(100, 1, 2),
			200: baseExpt(200, 2, 1),
		},
		queueing: map[string][]int64{
			fakeRunKey(100, 1000): {11, 12, 13, 14, 15},
			fakeRunKey(200, 2000): {21, 22},
		},
	}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test", 200: "ppe_test"}}
	submitter := &fakeSubmitter{}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, submitter)

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, report.ReservedGrants)
	assert.Equal(t, 3, report.Dispatched)
	// Online experiment 200 outranks offline 100 despite being scanned second.
	assert.Equal(t, []string{"200/21", "100/11", "100/12"}, submitter.order)
	assert.Equal(t, int64(3), resv.usage[entity.CentralResourceItem])
	assert.Equal(t, int64(3), resv.usage[entity.CentralResourceEvaluator])
}

func TestCentralTick_ShadowDoesNotReserveOrSubmit(t *testing.T) {
	conf := baseConf()
	conf.Mode = entity.CentralModeShadow
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {Queueing: 2}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 2)},
		queueing:    map[string][]int64{fakeRunKey(100, 1000): {11, 12}},
	}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	submitter := &fakeSubmitter{}
	scheduler := newTestScheduler(t, conf, active, resv, own, submitter)

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, report.PlannedGrants)
	assert.Equal(t, 0, report.ReservedGrants)
	assert.Equal(t, 0, submitter.calls)
	assert.Equal(t, 0, resv.reserveCalls)
}

func TestCentralTick_DispatchFailureReleasesReservation(t *testing.T) {
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {Queueing: 1}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
		queueing:    map[string][]int64{fakeRunKey(100, 1000): {11}},
	}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	submitter := &fakeSubmitter{fail: true}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, submitter)

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, report.ReservedGrants)
	assert.Equal(t, 0, report.Dispatched)
	require.Len(t, report.Anomalies, 1)
	assert.Equal(t, entity.CentralAnomalyDispatchFailed, report.Anomalies[0].Kind)
	// The reservation must be compensated so the item is not leaked.
	assert.Equal(t, int64(0), resv.usage[entity.CentralResourceItem])
	assert.Equal(t, 0, len(resv.reservations))
}

func TestCentralTick_ReleasesOrphanReservationWhenItemTerminal(t *testing.T) {
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
		states: map[string]map[int64]entity.ItemRunState{
			fakeRunKey(100, 1000): {11: entity.ItemRunState_Success},
		},
	}
	resv := newFakeReservationStore()
	_, _ = resv.Reserve(context.Background(), &entity.CentralReserveRequest{
		Env: "ppe_test", ExptID: 100, SpaceID: 1, RunID: 1000, ItemID: 11,
		ReservationID: entity.CentralReservationID(100, 1000, 11),
		Claims:        baseExpt(100, 1, 1).ResourceDemand(),
		Capacities:    map[string]int64{entity.CentralResourceItem: 3, entity.CentralResourceTarget: 3, entity.CentralResourceEvaluator: 3},
	})
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, &fakeSubmitter{})

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, report.ReleasedOrphans)
	assert.Equal(t, int64(0), resv.usage[entity.CentralResourceItem])
}

func TestCentralTick_FinishedRunReleasesAllAndOwnership(t *testing.T) {
	terminated := baseRun(100, 1, 1000, entity.ExptType_Offline)
	terminated.RunStatus = entity.ExptStatus_Terminated
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{terminated},
		snapshots:   map[string]*entity.CentralRunSnapshot{},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
	}
	resv := newFakeReservationStore()
	_, _ = resv.Reserve(context.Background(), &entity.CentralReserveRequest{
		Env: "ppe_test", ExptID: 100, SpaceID: 1, RunID: 1000, ItemID: 11,
		ReservationID: entity.CentralReservationID(100, 1000, 11),
		Claims:        baseExpt(100, 1, 1).ResourceDemand(),
		Capacities:    map[string]int64{entity.CentralResourceItem: 3, entity.CentralResourceTarget: 3, entity.CentralResourceEvaluator: 3},
	})
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, &fakeSubmitter{})

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, report.FinishedRuns)
	assert.Equal(t, 1, report.ReleasedOrphans)
	assert.Equal(t, 0, len(resv.reservations))
	assert.Equal(t, "", own.owners[100])
}

func TestCentralTick_SkipsForeignEnvironmentRun(t *testing.T) {
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {Queueing: 1}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
		queueing:    map[string][]int64{fakeRunKey(100, 1000): {11}},
	}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "prod"}}
	submitter := &fakeSubmitter{}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, submitter)

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, report.SkippedRuns)
	assert.Equal(t, 0, submitter.calls)
	assert.Equal(t, 0, resv.reserveCalls)
}

func TestCentralTick_AdoptsUnreservedProcessingItem(t *testing.T) {
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {InFlight: 1}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
		processing:  map[string][]int64{fakeRunKey(100, 1000): {11}},
		states:      map[string]map[int64]entity.ItemRunState{fakeRunKey(100, 1000): {11: entity.ItemRunState_Processing}},
	}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, &fakeSubmitter{})

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Drifts, 1)
	assert.Equal(t, 1, report.Drifts[0].DBProcessing)
	assert.Equal(t, 1, report.Drifts[0].RedisHeld)
	assert.Equal(t, 0, len(report.Drifts[0].UnreservedProcessing))
	assert.Equal(t, int64(1), resv.usage[entity.CentralResourceItem])
}

func TestCentralTick_ReportsDebtWhenAdoptionLacksCapacity(t *testing.T) {
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {InFlight: 1}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
		processing:  map[string][]int64{fakeRunKey(100, 1000): {11}},
		states:      map[string]map[int64]entity.ItemRunState{fakeRunKey(100, 1000): {11: entity.ItemRunState_Processing}},
	}
	resv := newFakeReservationStore()
	// A previous reservation consumed the entire item pool.
	_, _ = resv.Reserve(context.Background(), &entity.CentralReserveRequest{
		Env: "ppe_test", ExptID: 999, SpaceID: 9, RunID: 9999, ItemID: 1,
		ReservationID: entity.CentralReservationID(999, 9999, 1),
		Claims:        entity.CentralResourceClaims{entity.CentralResourceItem: 3},
		Capacities:    map[string]int64{entity.CentralResourceItem: 3},
	})
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, &fakeSubmitter{})

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Drifts, 1)
	assert.Equal(t, []int64{11}, report.Drifts[0].UnreservedProcessing)
	found := false
	for _, a := range report.Anomalies {
		if a.Kind == entity.CentralAnomalyReservationLost && a.ItemID == 11 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestCentralTick_PendingRunIsLeftToLegacyBootstrap(t *testing.T) {
	pending := baseRun(100, 1, 1000, entity.ExptType_Offline)
	pending.RunStatus = entity.ExptStatus_Pending
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{pending},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
		queueing:    map[string][]int64{fakeRunKey(100, 1000): {11}},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {Queueing: 1}},
	}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	submitter := &fakeSubmitter{}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, submitter)

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, report.PlannedGrants)
	assert.Equal(t, 0, submitter.calls)
}

func TestCentralTick_LegacyModeDoesNothing(t *testing.T) {
	conf := baseConf()
	conf.Mode = entity.CentralModeLegacy
	active := &fakeActiveRunRepo{runs: []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)}}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	submitter := &fakeSubmitter{}
	scheduler := newTestScheduler(t, conf, active, resv, own, submitter)

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, report.ScannedRuns)
	assert.Equal(t, 0, submitter.calls)
}

func ptrTime(t time.Time) *time.Time { return &t }

// Compile-time assertions that the fakes satisfy the production interfaces.
var (
	_ repo.CentralActiveRunRepo    = (*fakeActiveRunRepo)(nil)
	_ repo.CentralReservationStore = (*fakeReservationStore)(nil)
	_ repo.CentralOwnershipStore   = (*fakeOwnershipStore)(nil)
	_ CentralItemSubmitter         = (*fakeSubmitter)(nil)
)

type fakeLocker struct {
	lock.ILocker
	holders map[string]string
}

func (f *fakeLocker) BackoffLockWithValue(_ context.Context, key, val string, _ time.Duration, _ time.Duration) (bool, string, error) {
	if cur, ok := f.holders[key]; ok {
		return false, cur, nil
	}
	f.holders[key] = val
	return true, val, nil
}

func TestCentralOwnershipGate_EnvIsolationAndLegacyFallback(t *testing.T) {
	conf := baseConf()
	conf.EnvNamespace = "ppe_test"
	locker := &fakeLocker{holders: map[string]string{}}
	impl := &ExptSchedulerImpl{
		Configer: &fakeCentralConfiger{conf: conf},
		Mutex:    locker,
	}
	ctx := context.Background()

	state, err := impl.centralOwnership(ctx, &entity.ExptScheduleEvent{
		SpaceID: 1, ExptID: 100, ExptRunID: 1000, Env: "ppe_test",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, centralOwnershipOwned, state)

	// A second environment consuming the same event must not touch it.
	state, err = impl.centralOwnership(ctx, &entity.ExptScheduleEvent{
		SpaceID: 1, ExptID: 101, ExptRunID: 1001, Env: "prod",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, centralOwnershipForeign, state)

	// Runs created before rollout carry no env and stay on the legacy loop.
	state, err = impl.centralOwnership(ctx, &entity.ExptScheduleEvent{
		SpaceID: 1, ExptID: 102, ExptRunID: 1002,
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, centralOwnershipNotApplicable, state)

	// Shadow claims ownership for observation but never suppresses legacy.
	shadow := baseConf()
	shadow.Mode = entity.CentralModeShadow
	impl.Configer = &fakeCentralConfiger{conf: shadow}
	state, err = impl.centralOwnership(ctx, &entity.ExptScheduleEvent{
		SpaceID: 1, ExptID: 104, ExptRunID: 1004, Env: "ppe_test",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, centralOwnershipShadow, state)

	// Legacy mode never claims.
	legacy := baseConf()
	legacy.Mode = entity.CentralModeLegacy
	impl.Configer = &fakeCentralConfiger{conf: legacy}
	state, err = impl.centralOwnership(ctx, &entity.ExptScheduleEvent{
		SpaceID: 1, ExptID: 103, ExptRunID: 1003, Env: "ppe_test",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, centralOwnershipNotApplicable, state)

	// Experiment-level IDL policy can downgrade an allowlisted run to legacy.
	impl.Configer = &fakeCentralConfiger{conf: baseConf()}
	expt := &entity.Experiment{EvalConf: &entity.EvaluationConfiguration{Ext: map[string]string{
		entity.CentralExtModeKey: string(entity.CentralModeLegacy),
	}}}
	state, err = impl.centralOwnership(ctx, &entity.ExptScheduleEvent{
		SpaceID: 1, ExptID: 105, ExptRunID: 1005, Env: "ppe_test",
	}, expt)
	require.NoError(t, err)
	assert.Equal(t, centralOwnershipNotApplicable, state)
}

func TestCentralTick_SecondTickDoesNotDoubleDispatch(t *testing.T) {
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {Queueing: 2}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
		queueing:    map[string][]int64{fakeRunKey(100, 1000): {11, 12}},
		processing:  map[string][]int64{},
		states:      map[string]map[int64]entity.ItemRunState{fakeRunKey(100, 1000): {}},
	}
	submitter := &fakeSubmitter{}
	submitter.onSubmit = func(run *entity.CentralActiveRun, itemID int64) {
		key := fakeRunKey(run.ExptID, run.RunID)
		active.snapshots[key].InFlight++
		active.processing[key] = append(active.processing[key], itemID)
		active.states[key][itemID] = entity.ItemRunState_Processing
	}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, submitter)

	_, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, submitter.calls)

	// MySQL now shows the item processing. A second tick must reconcile the
	// existing reservation and leave it alone rather than publishing again.
	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, report.Dispatched)
	assert.Equal(t, 1, submitter.calls)
	assert.Equal(t, 1, len(resv.reservations))
}

func TestCentralTick_RebuildPathOnUnresolvableDrift(t *testing.T) {
	conf := baseConf()
	conf.RebuildOnDrift = true
	conf.Pool = map[string]int64{entity.CentralResourceItem: 0, entity.CentralResourceTarget: 0, entity.CentralResourceEvaluator: 0}
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{baseRun(100, 1, 1000, entity.ExptType_Offline)},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {InFlight: 1}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 1)},
		processing:  map[string][]int64{fakeRunKey(100, 1000): {11}},
		states:      map[string]map[int64]entity.ItemRunState{fakeRunKey(100, 1000): {11: entity.ItemRunState_Processing}},
	}
	resv := newFakeReservationStore()
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	scheduler := newTestScheduler(t, conf, active, resv, own, &fakeSubmitter{})

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	require.Len(t, report.Drifts, 1)
	assert.True(t, report.Drifts[0].Rebuilt)
	assert.Equal(t, []int64{11}, report.Drifts[0].UnreservedProcessing)
}

func TestCentralTick_DrainingKeepsInFlightReservationButStopsRefill(t *testing.T) {
	draining := baseRun(100, 1, 1000, entity.ExptType_Online)
	draining.RunStatus = entity.ExptStatus_Draining
	active := &fakeActiveRunRepo{
		runs:        []*entity.CentralActiveRun{draining},
		snapshots:   map[string]*entity.CentralRunSnapshot{fakeRunKey(100, 1000): {InFlight: 1, Queueing: 3}},
		experiments: map[int64]*entity.CentralExperimentScheduling{100: baseExpt(100, 1, 2)},
		queueing:    map[string][]int64{fakeRunKey(100, 1000): {11, 12, 13}},
		processing:  map[string][]int64{fakeRunKey(100, 1000): {10}},
		states:      map[string]map[int64]entity.ItemRunState{fakeRunKey(100, 1000): {10: entity.ItemRunState_Processing}},
	}
	resv := newFakeReservationStore()
	_, err := resv.Reserve(context.Background(), &entity.CentralReserveRequest{
		Env: "ppe_test", ExptID: 100, SpaceID: 1, RunID: 1000, ItemID: 10,
		ReservationID: entity.CentralReservationID(100, 1000, 10),
		Claims:        baseExpt(100, 1, 2).ResourceDemand(),
		Capacities:    map[string]int64{entity.CentralResourceItem: 3, entity.CentralResourceTarget: 3, entity.CentralResourceEvaluator: 3},
	})
	require.NoError(t, err)
	own := &fakeOwnershipStore{owners: map[int64]string{100: "ppe_test"}}
	submitter := &fakeSubmitter{}
	scheduler := newTestScheduler(t, baseConf(), active, resv, own, submitter)

	report, err := scheduler.Tick(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, report.Dispatched)
	assert.Equal(t, 0, submitter.calls)
	assert.Equal(t, 1, len(resv.reservations))
	assert.Equal(t, 0, report.FinishedRuns)
}
