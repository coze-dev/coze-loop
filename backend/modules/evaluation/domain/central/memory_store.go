// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"sync"
	"time"
)

// MemoryStore is an in-memory Store used by unit tests, local shadow evaluation
// and environments without Redis. It mirrors the Redis implementation's
// semantics (atomic multi-resource reserve, idempotent state migration,
// ownership checks, TTL expiry, incremental/full reconcile, leader lease).
type MemoryStore struct {
	mu      sync.Mutex
	clock   Clock
	ledger  map[string]map[ResourceKind]int64 // namespace -> available
	caps    map[string]map[ResourceKind]int64 // namespace -> capacity
	gen     map[string]int64                  // namespace -> ledger generation
	mode    map[string]string                 // namespace -> last reconcile mode
	runs    map[string]memRun                 // namespace/run -> registration
	runMode map[string]memMode                // namespace/space/expt -> adopted mode
	res     map[string]Reservation            // namespace/id -> record
	prio    map[string]memPriority            // namespace/run -> priority
	leader  map[string]memLeader              // namespace -> leader
}

type memRun struct {
	run     ActiveRun
	expires time.Time
}

type memMode struct {
	mode    Mode
	expires time.Time
}

type memPriority struct {
	value   int
	expires time.Time
}

type memLeader struct {
	holder  string
	expires time.Time
}

func NewMemoryStore(clock Clock) *MemoryStore {
	if clock == nil {
		clock = DefaultClock()
	}
	return &MemoryStore{
		clock:   clock,
		ledger:  map[string]map[ResourceKind]int64{},
		caps:    map[string]map[ResourceKind]int64{},
		gen:     map[string]int64{},
		mode:    map[string]string{},
		runs:    map[string]memRun{},
		runMode: map[string]memMode{},
		res:     map[string]Reservation{},
		prio:    map[string]memPriority{},
		leader:  map[string]memLeader{},
	}
}

func (m *MemoryStore) now() time.Time { return m.clock.Now() }

func resStoreKey(scope Scope, id string) string { return id }
func runStoreKey(scope Scope, key RunKey) string {
	return scope.Namespace() + ":" + key.String()
}
func modeStoreKey(scope Scope, key RunKey) string {
	return scope.Namespace() + ":mode:" + key.String()
}

func (m *MemoryStore) EnsureLedger(_ Ctx, scope Scope, capacity map[ResourceKind]int64) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ns := scope.Namespace()
	avail := m.ledger[ns]
	if avail == nil {
		avail = map[ResourceKind]int64{}
		m.ledger[ns] = avail
	}
	oldCap := m.caps[ns]
	if oldCap == nil {
		oldCap = map[ResourceKind]int64{}
		m.caps[ns] = oldCap
	}
	for kind, cap := range capacity {
		if _, ok := oldCap[kind]; !ok {
			avail[kind] = cap
		} else {
			avail[kind] += cap - oldCap[kind]
			if avail[kind] < 0 {
				avail[kind] = 0
			}
		}
		oldCap[kind] = cap
	}
	return nil
}

func (m *MemoryStore) Ledger(_ Ctx, scope Scope) (Ledger, error) {
	if err := scope.Validate(); err != nil {
		return Ledger{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	return m.ledgerLocked(scope), nil
}

func (m *MemoryStore) ledgerLocked(scope Scope) Ledger {
	ns := scope.Namespace()
	out := Ledger{
		Scope:         scope,
		Capacity:      map[ResourceKind]int64{},
		Available:     map[ResourceKind]int64{},
		Reserved:      map[ResourceKind]int64{},
		Generation:    m.gen[ns],
		ReconcileMode: m.mode[ns],
		UpdatedAt:     m.now(),
	}
	for k, v := range m.caps[ns] {
		out.Capacity[k] = v
	}
	for k, v := range m.ledger[ns] {
		out.Available[k] = v
	}
	for _, r := range m.res {
		if r.Scope.Namespace() != ns || !r.State.HoldsQuota() {
			continue
		}
		for k, v := range r.Holds {
			out.Reserved[k] += v
		}
	}
	return out
}

func (m *MemoryStore) RegisterRun(_ Ctx, run ActiveRun, ttl time.Duration) error {
	if err := run.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	m.runs[runStoreKey(run.Scope, run.Key)] = memRun{run: run, expires: m.now().Add(ttl)}
	return nil
}

func (m *MemoryStore) ListRuns(_ Ctx, scope Scope) ([]ActiveRun, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	out := make([]ActiveRun, 0, len(m.runs))
	for _, r := range m.runs {
		if r.run.Scope.Namespace() != scope.Namespace() {
			continue
		}
		if scope.SpaceID != 0 && r.run.Key.SpaceID != scope.SpaceID {
			continue
		}
		out = append(out, r.run)
	}
	SortRuns(out)
	return out, nil
}

func (m *MemoryStore) UnregisterRun(_ Ctx, scope Scope, key RunKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runs, runStoreKey(scope, key))
	return nil
}

func (m *MemoryStore) GetPriority(_ Ctx, scope Scope, key RunKey) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	p, ok := m.prio[runStoreKey(scope, key)]
	if !ok {
		return 0, false, nil
	}
	return p.value, true, nil
}

func (m *MemoryStore) SetPriority(_ Ctx, scope Scope, key RunKey, priority int, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prio[runStoreKey(scope, key)] = memPriority{value: priority, expires: m.now().Add(ttl)}
	return nil
}

func (m *MemoryStore) TryReserve(_ Ctx, scope Scope, key RunKey, itemID int64, holds map[ResourceKind]int64, ttl time.Duration) (ReserveResult, error) {
	if err := scope.Validate(); err != nil {
		return ReserveDenied, err
	}
	if err := key.Validate(); err != nil {
		return ReserveDenied, err
	}
	if itemID <= 0 || len(holds) == 0 {
		return ReserveDenied, ErrInvalidReserve
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	id := ReservationID(scope, key, itemID)
	rk := resStoreKey(scope, id)
	if r, ok := m.res[rk]; ok && r.State.HoldsQuota() {
		if err := ValidateOwnership(scope, r); err != nil {
			return ReserveDenied, err
		}
		if !r.ExpiresAt.IsZero() && !r.ExpiresAt.After(m.now()) {
			m.refundLocked(r)
			delete(m.res, rk)
		} else {
			return ReserveExisting, nil
		}
	}
	for kind, amount := range holds {
		if !kind.Valid() || amount <= 0 {
			return ReserveDenied, ErrInvalidReserve
		}
	}
	ns := scope.Namespace()
	avail := m.ledger[ns]
	if avail == nil {
		avail = map[ResourceKind]int64{}
		m.ledger[ns] = avail
	}
	for kind, amount := range holds {
		if avail[kind] < amount {
			return ReserveDenied, nil
		}
	}
	for kind, amount := range holds {
		avail[kind] -= amount
	}
	now := m.now()
	m.res[rk] = Reservation{
		ID: id, Scope: scope, Key: key, ItemID: itemID,
		Holds: cloneHolds(holds), State: ReservationReserved,
		CreatedAt: now, StateAt: now, ExpiresAt: now.Add(ttl),
	}
	return ReserveGranted, nil
}

func (m *MemoryStore) GetReservation(_ Ctx, scope Scope, id string) (*Reservation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	r, ok := m.res[resStoreKey(scope, id)]
	if !ok {
		return nil, false, nil
	}
	if !r.ExpiresAt.IsZero() && !r.ExpiresAt.After(m.now()) && r.State.HoldsQuota() {
		m.refundLocked(r)
		r.State = ReservationReleased
		r.StateAt = m.now()
		m.res[resStoreKey(scope, id)] = r
	}
	if err := ValidateOwnership(scope, r); err != nil {
		return nil, false, err
	}
	cp := r
	cp.Holds = cloneHolds(r.Holds)
	return &cp, true, nil
}

func (m *MemoryStore) ListReservations(_ Ctx, scope Scope) ([]Reservation, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	out := make([]Reservation, 0, len(m.res))
	for _, r := range m.res {
		if r.Scope.Namespace() != scope.Namespace() || !r.State.HoldsQuota() {
			continue
		}
		if scope.SpaceID != 0 && r.Key.SpaceID != scope.SpaceID {
			continue
		}
		cp := r
		cp.Holds = cloneHolds(r.Holds)
		out = append(out, cp)
	}
	return out, nil
}

func (m *MemoryStore) Release(_ Ctx, scope Scope, id string, reason string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.res[resStoreKey(scope, id)]
	if !ok {
		return false, nil
	}
	if err := ValidateOwnership(scope, r); err != nil {
		return false, err
	}
	if !r.State.HoldsQuota() {
		return false, nil
	}
	m.refundLocked(r)
	r.State = ReservationReleased
	r.StateAt = m.now()
	r.Reason = reason
	m.res[resStoreKey(scope, id)] = r
	return true, nil
}

func (m *MemoryStore) Transition(_ Ctx, scope Scope, id string, from, to ReservationState, reason string, dispatchedAt time.Time) (bool, ReservationState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.res[resStoreKey(scope, id)]
	if !ok {
		return false, "", nil
	}
	if err := ValidateOwnership(scope, r); err != nil {
		return false, r.State, err
	}
	if r.State == to {
		return true, r.State, nil
	}
	if from != "" && r.State != from {
		return false, r.State, nil
	}
	if !CanTransition(r.State, to) {
		return false, r.State, ErrInvalidReserve
	}
	now := m.now()
	r.State = to
	r.StateAt = now
	r.Reason = reason
	if !dispatchedAt.IsZero() {
		r.DispatchedAt = dispatchedAt
	}
	m.res[resStoreKey(scope, id)] = r
	return true, r.State, nil
}

func (m *MemoryStore) Claim(_ Ctx, scope Scope, id string, reason string, dispatchedAt time.Time) (bool, ReservationState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.res[resStoreKey(scope, id)]
	if !ok {
		return false, "", nil
	}
	if err := ValidateOwnership(scope, r); err != nil {
		return false, r.State, err
	}
	if r.State != ReservationReserved {
		return false, r.State, nil
	}
	r.State = ReservationDispatched
	r.StateAt = m.now()
	r.Reason = reason
	if !dispatchedAt.IsZero() {
		r.DispatchedAt = dispatchedAt
	}
	m.res[resStoreKey(scope, id)] = r
	return true, r.State, nil
}

func (m *MemoryStore) RenewReservations(_ Ctx, scope Scope, key RunKey, ttl time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	n := 0
	for rk, r := range m.res {
		if r.Key != key || !r.State.HoldsQuota() {
			continue
		}
		if err := ValidateOwnership(scope, r); err != nil {
			continue
		}
		r.ExpiresAt = now.Add(ttl)
		m.res[rk] = r
		n++
	}
	return n, nil
}

func (m *MemoryStore) IncrementalReconcile(_ Ctx, scope Scope, capacity map[ResourceKind]int64, maxDrift int64) (Ledger, bool, error) {
	if err := scope.Validate(); err != nil {
		return Ledger{}, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	ns := scope.Namespace()
	used := m.usedLocked(ns)
	drift := m.driftLocked(scope, capacity, used)
	avail := m.ledger[ns]
	if avail == nil {
		avail = map[ResourceKind]int64{}
		m.ledger[ns] = avail
	}
	caps := m.caps[ns]
	if caps == nil {
		caps = map[ResourceKind]int64{}
		m.caps[ns] = caps
	}
	for _, kind := range KnownResources {
		expected := capacity[kind] - used[kind]
		if expected < 0 {
			expected = 0
		}
		avail[kind] += expected - avail[kind]
		caps[kind] = capacity[kind]
	}
	m.mode[ns] = "incremental"
	return m.ledgerLocked(scope), drift > maxDrift, nil
}

func (m *MemoryStore) RebuildLedger(_ Ctx, scope Scope, capacity map[ResourceKind]int64) (Ledger, error) {
	if err := scope.Validate(); err != nil {
		return Ledger{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	ns := scope.Namespace()
	used := m.usedLocked(ns)
	avail := map[ResourceKind]int64{}
	caps := map[ResourceKind]int64{}
	for _, kind := range KnownResources {
		caps[kind] = capacity[kind]
		v := capacity[kind] - used[kind]
		if v < 0 {
			v = 0
		}
		avail[kind] = v
	}
	m.ledger[ns] = avail
	m.caps[ns] = caps
	m.gen[ns]++
	m.mode[ns] = "full"
	return m.ledgerLocked(scope), nil
}

func (m *MemoryStore) usedLocked(ns string) map[ResourceKind]int64 {
	used := map[ResourceKind]int64{}
	for _, r := range m.res {
		if r.Scope.Namespace() != ns || !r.State.HoldsQuota() {
			continue
		}
		for k, v := range r.Holds {
			used[k] += v
		}
	}
	return used
}

func (m *MemoryStore) driftLocked(scope Scope, capacity, used map[ResourceKind]int64) int64 {
	avail := m.ledger[scope.Namespace()]
	var drift int64
	for _, kind := range KnownResources {
		expected := capacity[kind] - used[kind]
		if expected < 0 {
			expected = 0
		}
		diff := avail[kind] - expected
		if diff < 0 {
			diff = -diff
		}
		if diff > drift {
			drift = diff
		}
	}
	return drift
}

func (m *MemoryStore) GetRunMode(_ Ctx, scope Scope, key RunKey) (Mode, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	rec, ok := m.runMode[modeStoreKey(scope, key)]
	if !ok || rec.expires.Before(m.now()) {
		return ModeLegacy, false, nil
	}
	return rec.mode, true, nil
}

func (m *MemoryStore) SetRunMode(_ Ctx, scope Scope, key RunKey, mode Mode, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runMode[modeStoreKey(scope, key)] = memMode{mode: mode, expires: m.now().Add(ttl)}
	return nil
}

func (m *MemoryStore) AcquireLeadership(_ Ctx, scope Scope, holder string, ttl time.Duration) (bool, error) {
	if err := scope.Validate(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ns := scope.Namespace()
	now := m.now()
	cur, ok := m.leader[ns]
	if ok && cur.holder != holder && cur.expires.After(now) {
		return false, nil
	}
	m.leader[ns] = memLeader{holder: holder, expires: now.Add(ttl)}
	return true, nil
}

func (m *MemoryStore) RenewLeadership(_ Ctx, scope Scope, holder string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ns := scope.Namespace()
	cur, ok := m.leader[ns]
	if !ok || cur.holder != holder || !cur.expires.After(m.now()) {
		return false, nil
	}
	m.leader[ns] = memLeader{holder: holder, expires: m.now().Add(ttl)}
	return true, nil
}

func (m *MemoryStore) ReleaseLeadership(_ Ctx, scope Scope, holder string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ns := scope.Namespace()
	if cur, ok := m.leader[ns]; ok && cur.holder == holder {
		delete(m.leader, ns)
	}
	return nil
}

func (m *MemoryStore) Cleanup(_ Ctx, scope Scope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ns := scope.Namespace()
	delete(m.ledger, ns)
	delete(m.caps, ns)
	delete(m.gen, ns)
	delete(m.mode, ns)
	delete(m.leader, ns)
	for key, r := range m.runs {
		if r.run.Scope.Namespace() == ns {
			delete(m.runs, key)
		}
	}
	for key := range m.runMode {
		if len(key) >= len(ns) && key[:len(ns)] == ns {
			delete(m.runMode, key)
		}
	}
	for key := range m.prio {
		if len(key) >= len(ns) && key[:len(ns)] == ns {
			delete(m.prio, key)
		}
	}
	for id, r := range m.res {
		if r.Scope.Namespace() == ns {
			delete(m.res, id)
		}
	}
	return nil
}

func (m *MemoryStore) expireLocked() {
	now := m.now()
	for key, r := range m.runs {
		if !r.expires.IsZero() && !r.expires.After(now) {
			delete(m.runs, key)
		}
	}
	for key, p := range m.prio {
		if !p.expires.IsZero() && !p.expires.After(now) {
			delete(m.prio, key)
		}
	}
	for key, rec := range m.runMode {
		if !rec.expires.IsZero() && !rec.expires.After(now) {
			delete(m.runMode, key)
		}
	}
	for key, l := range m.leader {
		if !l.expires.IsZero() && !l.expires.After(now) {
			delete(m.leader, key)
		}
	}
	// Expired reservations are refunded lazily by TryReserve/GetReservation or
	// by reconcile; keeping them makes the refund observable exactly once.
}

func (m *MemoryStore) refundLocked(r Reservation) {
	ns := r.Scope.Namespace()
	if avail := m.ledger[ns]; avail != nil {
		for kind, amount := range r.Holds {
			avail[kind] += amount
		}
	}
}

func cloneHolds(in map[ResourceKind]int64) map[ResourceKind]int64 {
	out := make(map[ResourceKind]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
