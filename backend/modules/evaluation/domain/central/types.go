// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package central implements the environment-scoped central scheduler for offline
// evaluation experiments.
//
// The package is deliberately dependency-free (no DB / MQ / RPC imports) so the
// scheduling decisions can be unit-tested and reused by both the OSS backend and
// the commercial backend. Infrastructure specifics are behind the Store /
// RunSource / Driver interfaces.
package central

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Mode controls how the central scheduler participates in experiment dispatch.
//
// Legacy is the zero value on purpose: every existing caller that does not
// configure central scheduling keeps the historical 20s MQ self-loop behaviour.
type Mode int

const (
	// ModeLegacy keeps the per-experiment MQ self-loop and the legacy running-experiment
	// admission untouched.
	ModeLegacy Mode = iota
	// ModeShadow evaluates the central decision (budget / atomic reservation feasibility)
	// but never mutates the ledger and never changes dispatch behaviour.
	ModeShadow
	// ModeEnforce reserves resources before dispatch and suppresses the per-run MQ
	// self-loop for allow-listed runs.
	ModeEnforce
)

func (m Mode) String() string {
	switch m {
	case ModeShadow:
		return "shadow"
	case ModeEnforce:
		return "enforce"
	default:
		return "legacy"
	}
}

// ParseMode accepts the strings used in TCC configuration. Unknown values fall
// back to ModeLegacy so a malformed config can never silently enable enforcement.
func ParseMode(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "shadow":
		return ModeShadow
	case "enforce", "enforced":
		return ModeEnforce
	default:
		return ModeLegacy
	}
}

// Scope identifies the ownership boundary of a scheduling ledger.
//
// Online and PPE share the same MySQL/Redis/TCC instances, so env and region
// are mandatory and become part of every Redis key. A reservation created in
// one scope can never be released from another scope.
type Scope struct {
	Env    string
	Region string
	// SpaceID is optional. Space-level keys use it; 0 means "scan all spaces".
	SpaceID int64
}

var (
	ErrInvalidScope = errors.New("central: scope requires non-empty env and region")
	// ErrCrossEnvironment is returned when a reservation is addressed from a scope
	// that does not own it.
	ErrCrossEnvironment = errors.New("central: reservation ownership mismatch")
	ErrInvalidRunKey    = errors.New("central: invalid run key")
	ErrInvalidReserve   = errors.New("central: invalid reservation request")
)

func (s Scope) Validate() error {
	if strings.TrimSpace(s.Env) == "" || strings.TrimSpace(s.Region) == "" {
		return ErrInvalidScope
	}
	if s.SpaceID < 0 {
		return ErrInvalidScope
	}
	return nil
}

// Namespace returns the mandatory key prefix for this scope.
func (s Scope) Namespace() string {
	return "qn:central:" + sanitizeKeyPart(s.Env) + ":" + sanitizeKeyPart(s.Region)
}

// Key builds a namespaced key. The result always starts with qn:central:.
func (s Scope) Key(parts ...string) string {
	all := make([]string, 0, len(parts)+2)
	all = append(all, s.Namespace())
	all = append(all, parts...)
	for i, p := range all {
		all[i] = sanitizeKeyPart(p)
	}
	return strings.Join(all, ":")
}

func (s Scope) WithSpace(spaceID int64) Scope {
	s.SpaceID = spaceID
	return s
}

func sanitizeKeyPart(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "_"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", "\t", "_", "\n", "_")
	return replacer.Replace(p)
}

// RunKey uniquely identifies one experiment run.
type RunKey struct {
	SpaceID   int64
	ExptID    int64
	ExptRunID int64
}

func (k RunKey) Validate() error {
	if k.SpaceID <= 0 || k.ExptID <= 0 || k.ExptRunID <= 0 {
		return ErrInvalidRunKey
	}
	return nil
}

func (k RunKey) String() string {
	return fmt.Sprintf("%d/%d/%d", k.SpaceID, k.ExptID, k.ExptRunID)
}

// ResourceKind is a members of the multi-resource quota pool.
type ResourceKind string

const (
	ResourceItemSlot    ResourceKind = "item_slot"
	ResourceTargetSlot  ResourceKind = "target_slot"
	ResourceSandboxSlot ResourceKind = "sandbox_slot"
)

// KnownResources is used to order/iterate resources deterministically.
var KnownResources = []ResourceKind{
	ResourceItemSlot,
	ResourceTargetSlot,
	ResourceSandboxSlot,
}

func (r ResourceKind) Valid() bool {
	switch r {
	case ResourceItemSlot, ResourceTargetSlot, ResourceSandboxSlot:
		return true
	default:
		return false
	}
}

// ActiveRun is the scheduler's view of an active experiment run.
type ActiveRun struct {
	Scope Scope
	Key   RunKey

	// RunMode/ExptType mirror the entity values; kept as int32 so this package
	// stays independent of the entity package.
	RunMode  int32
	ExptType int32

	Priority   int
	EnqueuedAt time.Time
	UpdatedAt  time.Time

	// MaxConcurrency is the run-level item concurrency ceiling.
	MaxConcurrency int64
	// RemainingItems is the number of un-finished queueing items.
	RemainingItems int64
	// PerItem is how much of each resource one in-flight item consumes.
	PerItem map[ResourceKind]int64

	CreatedBy string

	// Budget is an ephemeral value computed by the scheduler for this tick.
	// It is the number of new items this run may dispatch right now.
	Budget int64
}

// Normalize fills zero values with safe defaults.
func (r ActiveRun) Normalize() ActiveRun {
	if r.EnqueuedAt.IsZero() {
		r.EnqueuedAt = r.UpdatedAt
	}
	if r.MaxConcurrency <= 0 {
		r.MaxConcurrency = 1
	}
	if r.RemainingItems < 0 {
		r.RemainingItems = 0
	}
	if len(r.PerItem) == 0 {
		r.PerItem = map[ResourceKind]int64{ResourceItemSlot: 1}
	}
	return r
}

func (r ActiveRun) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if r.Scope.SpaceID != r.Key.SpaceID {
		return ErrInvalidScope
	}
	if err := r.Key.Validate(); err != nil {
		return err
	}
	for kind, amount := range r.PerItem {
		if !kind.Valid() {
			return fmt.Errorf("%w: unknown resource %q", ErrInvalidReserve, kind)
		}
		if amount <= 0 {
			return fmt.Errorf("%w: resource %q amount must be positive", ErrInvalidReserve, kind)
		}
	}
	return nil
}

// TotalPerItem returns the run's per-item demand sum, used for fast capacity
// sanity checks.
func (r ActiveRun) TotalPerItem() int64 {
	var total int64
	for _, amount := range r.PerItem {
		total += amount
	}
	return total
}

// HoldsFor returns the multi-resource holds for n items. It returns an error
// when any resource would overflow; callers should treat that as deny.
func (r ActiveRun) HoldsFor(n int64) (map[ResourceKind]int64, error) {
	if n < 0 {
		return nil, fmt.Errorf("%w: negative item count", ErrInvalidReserve)
	}
	holds := make(map[ResourceKind]int64, len(r.PerItem))
	for kind, per := range r.PerItem {
		if per > 0 && n > (1<<62)/per {
			return nil, fmt.Errorf("%w: resource %q overflow", ErrInvalidReserve, kind)
		}
		holds[kind] = per * n
	}
	return holds, nil
}

// ReservationState is kept for observability and drift analysis.
type ReservationState string

const (
	ReservationHeld     ReservationState = "held"
	ReservationReleased ReservationState = "released"
)

// Reservation is one atomic multi-resource hold.
type Reservation struct {
	ID        string
	Scope     Scope
	Key       RunKey
	ItemID    int64
	Holds     map[ResourceKind]int64
	State     ReservationState
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (r Reservation) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrInvalidReserve
	}
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := r.Key.Validate(); err != nil {
		return err
	}
	for kind, amount := range r.Holds {
		if !kind.Valid() || amount <= 0 {
			return ErrInvalidReserve
		}
	}
	return nil
}

// ReservationID is deterministic so retries and releases are idempotent.
func ReservationID(key RunKey, itemID int64) string {
	return key.String() + ":item:" + strconv.FormatInt(itemID, 10)
}

// ReserveResult is the outcome of an atomic reservation attempt.
type ReserveResult int

const (
	ReserveDenied ReserveResult = iota
	ReserveGranted
	// ReserveExisting means the deterministic reservation already exists (retry
	// or duplicate dispatch); the caller may treat the item as already granted.
	ReserveExisting
)

// Ledger is a point-in-time view of the scope's resource pool.
type Ledger struct {
	Scope     Scope
	Capacity  map[ResourceKind]int64
	Available map[ResourceKind]int64
	Reserved  map[ResourceKind]int64
	UpdatedAt time.Time
}

// Limits is the pool configuration for one scope.
type Limits struct {
	Capacity map[ResourceKind]int64
	// ReservationTTL bounds an in-flight item hold.
	ReservationTTL time.Duration
	// DispatchGrace is how long a reservation may exist before the item is observed
	// in Processing; after that reconcile treats it as an uncertain dispatch.
	DispatchGrace time.Duration
	// MaxDrift is the tolerated absolute drift before a ledger rebuild is triggered.
	MaxDrift int64
}

func (l Limits) withDefaults() Limits {
	if l.ReservationTTL <= 0 {
		l.ReservationTTL = 5 * time.Minute
	}
	if l.DispatchGrace <= 0 {
		l.DispatchGrace = time.Minute
	}
	if l.MaxDrift <= 0 {
		l.MaxDrift = 0
	}
	if len(l.Capacity) == 0 {
		l.Capacity = map[ResourceKind]int64{
			ResourceItemSlot:    200,
			ResourceTargetSlot:  200,
			ResourceSandboxSlot: 50,
		}
	}
	return l
}

func (l Limits) capacityOf(kind ResourceKind) int64 {
	if l.Capacity == nil {
		return 0
	}
	return l.Capacity[kind]
}

func (l Limits) Validate() error {
	for kind, cap := range l.Capacity {
		if !kind.Valid() {
			return fmt.Errorf("%w: unknown capacity resource %q", ErrInvalidReserve, kind)
		}
		if cap < 0 {
			return fmt.Errorf("%w: negative capacity for %q", ErrInvalidReserve, kind)
		}
	}
	return nil
}

// Clock is injectable for deterministic tests.
type Clock interface {
	Now() time.Time
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

// DefaultClock returns a real clock.
func DefaultClock() Clock { return wallClock{} }

// Store persists scheduling state. Implementations must:
//   - namespace every key with Scope.Namespace() (qn:central:...),
//   - set a TTL on every key they create,
//   - make TryReserve all-or-nothing across the supplied resource holds,
//   - make Release idempotent,
//   - reject Release/Get calls whose scope does not own the reservation.
type Store interface {
	// EnsureLedger initialises (or adjusts) available capacity without clobbering
	// existing usage.
	EnsureLedger(ctx Ctx, scope Scope, capacity map[ResourceKind]int64) error
	Ledger(ctx Ctx, scope Scope) (Ledger, error)

	// RegisterRun upserts a run with a TTL so crashed schedulers leave no permanent
	// registrations.
	RegisterRun(ctx Ctx, run ActiveRun, ttl time.Duration) error
	ListRuns(ctx Ctx, scope Scope) ([]ActiveRun, error)
	UnregisterRun(ctx Ctx, scope Scope, key RunKey) error

	GetPriority(ctx Ctx, scope Scope, key RunKey) (priority int, found bool, err error)
	SetPriority(ctx Ctx, scope Scope, key RunKey, priority int, ttl time.Duration) error

	TryReserve(ctx Ctx, scope Scope, key RunKey, itemID int64, holds map[ResourceKind]int64, ttl time.Duration) (ReserveResult, error)
	GetReservation(ctx Ctx, scope Scope, id string) (*Reservation, bool, error)
	ListReservations(ctx Ctx, scope Scope) ([]Reservation, error)
	Release(ctx Ctx, scope Scope, id string) (bool, error)
	RenewReservations(ctx Ctx, scope Scope, key RunKey, ttl time.Duration) (int, error)

	// RebuildLedger resets available counters from the authoritative reservation
	// set: available = capacity - sum(held). It is safe to call concurrently; the
	// store must use a short-lived lock and only one rebuild writes.
	RebuildLedger(ctx Ctx, scope Scope, capacity map[ResourceKind]int64) (Ledger, error)

	// Cleanup removes all qn:central: keys owned by this scope. It must never
	// touch keys outside the scope prefix. Called when a run/verification finishes.
	Cleanup(ctx Ctx, scope Scope) error
}

// Ctx is an alias so this package does not need a context import in its public
// surface while still accepting context.Context values.
type Ctx = interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

// RunSource discovers active runs. Implementations live in infra/application
// because they need DB access.
type RunSource interface {
	ListActive(ctx Ctx, scope Scope) ([]ActiveRun, error)
}

// Driver invokes the existing per-experiment scheduling chain for one run.
// The OSS implementation calls ExptSchedulerEvent.Schedule.
type Driver interface {
	Drive(ctx Ctx, run ActiveRun) error
}

// SortRuns orders runs by (priority asc, enqueued asc, run key) so dispatch is
// deterministic across instances.
func SortRuns(runs []ActiveRun) {
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].Priority != runs[j].Priority {
			return runs[i].Priority < runs[j].Priority
		}
		ti, tj := runs[i].EnqueuedAt, runs[j].EnqueuedAt
		if ti.IsZero() != tj.IsZero() {
			return tj.IsZero() // zero time sorts last
		}
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return runs[i].Key.String() < runs[j].Key.String()
	})
}

// Plan returns the number of items a run may dispatch given its currently held
// item count and the run-level ceiling. Global capacity is enforced later by
// the atomic reservation; Plan only computes the run-local deficit.
func Plan(run ActiveRun, heldItems int64) int64 {
	run = run.Normalize()
	if heldItems < 0 {
		heldItems = 0
	}
	deficit := run.MaxConcurrency - heldItems
	if deficit > run.RemainingItems {
		deficit = run.RemainingItems
	}
	if deficit < 0 {
		return 0
	}
	return deficit
}

// CountHeldByRun returns how many item reservations are held for a run.
func CountHeldByRun(reservations []Reservation, key RunKey) int64 {
	var n int64
	for _, r := range reservations {
		if r.Key == key && r.State != ReservationReleased {
			n++
		}
	}
	return n
}

// SumHolds aggregates resource holds across reservations.
func SumHolds(reservations []Reservation) map[ResourceKind]int64 {
	out := make(map[ResourceKind]int64, len(KnownResources))
	for _, r := range reservations {
		if r.State == ReservationReleased {
			continue
		}
		for kind, amount := range r.Holds {
			out[kind] += amount
		}
	}
	return out
}

// ValidateOwnership ensures a reservation belongs to the supplied scope before
// any mutation. Env and region must match; space must match when both are set.
func ValidateOwnership(scope Scope, r Reservation) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if scope.Env != r.Scope.Env || scope.Region != r.Scope.Region {
		return ErrCrossEnvironment
	}
	if scope.SpaceID != 0 && r.Scope.SpaceID != 0 && scope.SpaceID != r.Scope.SpaceID {
		return ErrCrossEnvironment
	}
	return nil
}
