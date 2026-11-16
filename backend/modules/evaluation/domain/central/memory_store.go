// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"sync"
	"time"
)

// MemoryStore is an in-memory Store used by unit tests, local shadow evaluation
// and environments without Redis. It mirrors the Redis implementation's
// semantics (atomic multi-resource reserve, idempotent release, ownership
// checks, TTL expiry).
type MemoryStore struct {
	mu      sync.Mutex
	clock   Clock
	ledger  map[string]map[ResourceKind]int64 // namespace -> available
	caps    map[string]map[ResourceKind]int64 // namespace -> capacity
	runs    map[string]memRun                 // run key -> registration
	runNS   map[string]string                 // run key -> namespace
	res     map[string]Reservation            // reservation id -> record
	prio    map[string]memPriority            // run key -> priority
	resSeen map[string]time.Time              // reservation id -> expires
}

type memRun struct {
	run     ActiveRun
	expires time.Time
}

type memPriority struct {
	value   int
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
		runs:    map[string]memRun{},
		runNS:   map[string]string{},
		res:     map[string]Reservation{},
		prio:    map[string]memPriority{},
		resSeen: map[string]time.Time{},
	}
}

func (m *MemoryStore) now() time.Time { return m.clock.Now() }

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
	ns := scope.Namespace()
	out := Ledger{
		Scope:     scope,
		Capacity:  map[ResourceKind]int64{},
		Available: map[ResourceKind]int64{},
		Reserved:  map[ResourceKind]int64{},
		UpdatedAt: m.now(),
	}
	for k, v := range m.caps[ns] {
		out.Capacity[k] = v
	}
	for k, v := range m.ledger[ns] {
		out.Available[k] = v
	}
	for _, r := range m.res {
		if r.Scope.Namespace() != ns {
			continue
		}
		for k, v := range r.Holds {
			out.Reserved[k] += v
		}
	}
	return out, nil
}

func (m *MemoryStore) RegisterRun(_ Ctx, run ActiveRun, ttl time.Duration) error {
	if err := run.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	key := run.Key.String()
	m.runs[key] = memRun{run: run, expires: m.now().Add(ttl)}
	m.runNS[key] = run.Scope.Namespace()
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
		if scope.SpaceID != 0 && r.run.Key.SpaceID != scope.SpaceID {
			continue
		}
		if r.run.Scope.Namespace() != scope.Namespace() {
			continue
		}
		out = append(out, r.run)
	}
	SortRuns(out)
	return out, nil
}

func (m *MemoryStore) UnregisterRun(_ Ctx, _ Scope, key RunKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runs, key.String())
	delete(m.runNS, key.String())
	return nil
}

func (m *MemoryStore) GetPriority(_ Ctx, _ Scope, key RunKey) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	p, ok := m.prio[key.String()]
	if !ok {
		return 0, false, nil
	}
	return p.value, true, nil
}

func (m *MemoryStore) SetPriority(_ Ctx, _ Scope, key RunKey, priority int, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prio[key.String()] = memPriority{value: priority, expires: m.now().Add(ttl)}
	return nil
}

func (m *MemoryStore) TryReserve(_ Ctx, scope Scope, key RunKey, itemID int64, holds map[ResourceKind]int64, ttl time.Duration) (ReserveResult, error) {
	if err := scope.Validate(); err != nil {
		return ReserveDenied, err
	}
	if err := key.Validate(); err != nil {
		return ReserveDenied, err
	}
	if itemID <= 0 {
		return ReserveDenied, ErrInvalidReserve
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	id := ReservationID(key, itemID)
	if r, ok := m.res[id]; ok && r.State != ReservationReleased {
		if r.Scope.Env != scope.Env || r.Scope.Region != scope.Region {
			return ReserveDenied, ErrCrossEnvironment
		}
		if !r.ExpiresAt.IsZero() && !r.ExpiresAt.After(m.now()) {
			m.refundLocked(r)
			delete(m.res, id)
		} else {
			return ReserveExisting, nil
		}
	}
	ns := scope.Namespace()
	avail := m.ledger[ns]
	if avail == nil {
		avail = map[ResourceKind]int64{}
		m.ledger[ns] = avail
	}
	for kind, amount := range holds {
		if !kind.Valid() || amount <= 0 {
			return ReserveDenied, ErrInvalidReserve
		}
	}
	// all-or-nothing check
	for kind, amount := range holds {
		if avail[kind] < amount {
			return ReserveDenied, nil
		}
	}
	for kind, amount := range holds {
		avail[kind] -= amount
	}
	now := m.now()
	m.res[id] = Reservation{
		ID:        id,
		Scope:     scope,
		Key:       key,
		ItemID:    itemID,
		Holds:     cloneHolds(holds),
		State:     ReservationHeld,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	return ReserveGranted, nil
}

func (m *MemoryStore) GetReservation(_ Ctx, scope Scope, id string) (*Reservation, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	r, ok := m.res[id]
	if !ok {
		return nil, false, nil
	}
	if !r.ExpiresAt.IsZero() && !r.ExpiresAt.After(m.now()) {
		m.refundLocked(r)
		delete(m.res, id)
		return nil, false, nil
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
		if r.State == ReservationReleased {
			continue
		}
		if r.Scope.Namespace() != scope.Namespace() {
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

func (m *MemoryStore) Release(_ Ctx, scope Scope, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.res[id]
	if !ok {
		return false, nil
	}
	if err := ValidateOwnership(scope, r); err != nil {
		return false, err
	}
	// Refund before deleting so a repeated release is a no-op.
	ns := r.Scope.Namespace()
	if avail := m.ledger[ns]; avail != nil {
		for kind, amount := range r.Holds {
			avail[kind] += amount
		}
	}
	delete(m.res, id)
	return true, nil
}

func (m *MemoryStore) RenewReservations(_ Ctx, scope Scope, key RunKey, ttl time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	n := 0
	for id, r := range m.res {
		if r.Key != key || r.State == ReservationReleased {
			continue
		}
		if err := ValidateOwnership(scope, r); err != nil {
			continue
		}
		r.ExpiresAt = now.Add(ttl)
		m.res[id] = r
		n++
	}
	return n, nil
}

func (m *MemoryStore) RebuildLedger(_ Ctx, scope Scope, capacity map[ResourceKind]int64) (Ledger, error) {
	if err := scope.Validate(); err != nil {
		return Ledger{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	ns := scope.Namespace()
	used := map[ResourceKind]int64{}
	for _, r := range m.res {
		if r.Scope.Namespace() != ns || r.State == ReservationReleased {
			continue
		}
		for k, v := range r.Holds {
			used[k] += v
		}
	}
	avail := map[ResourceKind]int64{}
	caps := map[ResourceKind]int64{}
	for kind, cap := range capacity {
		caps[kind] = cap
		v := cap - used[kind]
		if v < 0 {
			v = 0
		}
		avail[kind] = v
	}
	m.ledger[ns] = avail
	m.caps[ns] = caps
	out := Ledger{
		Scope:     scope,
		Capacity:  caps,
		Available: avail,
		Reserved:  used,
		UpdatedAt: m.now(),
	}
	return out, nil
}

func (m *MemoryStore) Cleanup(_ Ctx, scope Scope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ns := scope.Namespace()
	delete(m.ledger, ns)
	delete(m.caps, ns)
	for key, n := range m.runNS {
		if n == ns {
			delete(m.runs, key)
			delete(m.runNS, key)
		}
	}
	for id, r := range m.res {
		if r.Scope.Namespace() == ns {
			delete(m.res, id)
		}
	}
	for key := range m.prio {
		// Priority keys are global by run key; remove only runs owned by this ns.
		if _, ok := m.runNS[key]; !ok {
			// Cannot distinguish; keep. Redis cleanup removes the whole prefix.
			continue
		}
	}
	return nil
}

func (m *MemoryStore) expireLocked() {
	now := m.now()
	for id, expires := range m.resSeen {
		if !expires.After(now) {
			delete(m.resSeen, id)
		}
	}
	// Expired reservations are kept until the reconciler releases them so the
	// release is observable and refunded exactly once. They are ignored by
	// TryReserve/GetReservation and refunded lazily there.
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
