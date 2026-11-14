// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// CentralKeyPrefix is the single Redis key prefix reserved for the central
// scheduling feature. Every key written by this feature must start with it.
const CentralKeyPrefix = "qn:central:"

// CentralMode is the per-experiment admission mode for central scheduling.
//
// legacy  - untouched: the per-experiment 20s MQ self-loop keeps dispatching.
// shadow  - the central scheduler computes plans and emits observations, but
//
//	never reserves quota and never publishes item events. The legacy
//	loop keeps full ownership. Used to compare plans before cutover.
//
// enforce - the central scheduler reserves quota and dispatches items. The
//
//	legacy loop still runs for lifecycle/zombie/completion bookkeeping
//	but must not submit items for the owned run.
type CentralMode string

const (
	CentralModeLegacy  CentralMode = "legacy"
	CentralModeShadow  CentralMode = "shadow"
	CentralModeEnforce CentralMode = "enforce"

	// CentralDefaultEnv is used when TCC does not configure env_namespace.
	// Production and PPE must always set this explicitly: sharing one Redis
	// instance across environments is the reason this field exists.
	CentralDefaultEnv = "prod"
	// CentralDefaultTickInterval is the central scanner cadence.
	CentralDefaultTickInterval = 5 * time.Second
	// CentralDefaultReservationTTL is a safety net for reservations whose
	// terminal event never reached the centre. Reconciliation normally releases
	// them much earlier.
	CentralDefaultReservationTTL = 30 * time.Minute
	// CentralDefaultOwnershipTTL keeps an experiment pinned to the environment
	// that submitted it. It is renewed on every ownership check.
	CentralDefaultOwnershipTTL = 7 * 24 * time.Hour
	// CentralDefaultScanLimit bounds one DB scan.
	CentralDefaultScanLimit = 2000
	// CentralDefaultMaxItemsPerExperiment bounds a single experiment's grant
	// regardless of user configuration.
	CentralDefaultMaxItemsPerExperiment = 50
	// CentralDefaultItemConcurNum mirrors the PRD default.
	CentralDefaultItemConcurNum = 5
	// CentralDefaultAgingSecond is how much queue wait buys one priority point.
	CentralDefaultAgingSecond = 60
	// CentralDefaultMaxAgingBonus caps the aging bonus so it cannot outrank a
	// hard priority band forever.
	CentralDefaultMaxAgingBonus = 20
	// CentralDefaultFairShareRatio prevents one space from taking the whole
	// pool while other spaces have queued work.
	CentralDefaultFairShareRatio = 0.5
)

// CentralResource dimension names. They are also used as Redis pool key
// suffixes, e.g. qn:central:prod:pool:item.
const (
	CentralResourceItem      = "item"
	CentralResourceTarget    = "target"
	CentralResourceEvaluator = "evaluator"
	CentralResourceSandbox   = "sandbox"
)

// CentralSchedulingConf is the TCC contract (key: central_scheduling_conf).
type CentralSchedulingConf struct {
	Enabled bool        `json:"enabled" mapstructure:"enabled"`
	Mode    CentralMode `json:"mode" mapstructure:"mode"`
	// EnvNamespace partitions all Redis state. Online and PPE must use
	// different values even when they share the same Redis instance.
	EnvNamespace string `json:"env_namespace" mapstructure:"env_namespace"`
	// Lane is the PPE lane name; informational and used in ownership values.
	Lane string `json:"lane" mapstructure:"lane"`
	// DefaultAppID is used to rebuild Session for centrally dispatched items.
	// The legacy MQ event's AppID is not persisted in MySQL.
	DefaultAppID int32 `json:"default_app_id" mapstructure:"default_app_id"`

	TickIntervalSecond    int     `json:"tick_interval_second" mapstructure:"tick_interval_second"`
	ReservationTTLSecond  int     `json:"reservation_ttl_second" mapstructure:"reservation_ttl_second"`
	OwnershipTTLSecond    int     `json:"ownership_ttl_second" mapstructure:"ownership_ttl_second"`
	RecentRunWindowSecond int     `json:"recent_run_window_second" mapstructure:"recent_run_window_second"`
	ScanLimit             int     `json:"scan_limit" mapstructure:"scan_limit"`
	MaxItemsPerExperiment int     `json:"max_items_per_experiment" mapstructure:"max_items_per_experiment"`
	DefaultItemConcurNum  int     `json:"default_item_concur_num" mapstructure:"default_item_concur_num"`
	AgingSecond           int     `json:"aging_second" mapstructure:"aging_second"`
	MaxAgingPriorityBonus int     `json:"max_aging_priority_bonus" mapstructure:"max_aging_priority_bonus"`
	RebuildOnDrift        bool    `json:"rebuild_on_drift" mapstructure:"rebuild_on_drift"`
	AllowAllSpaces        bool    `json:"allow_all_spaces" mapstructure:"allow_all_spaces"`
	FairShareRatio        float64 `json:"fair_share_ratio" mapstructure:"fair_share_ratio"`

	// Allowlist / denylist select which experiments the centre may manage.
	// Empty allowlist + AllowAllSpaces=true means all experiments; otherwise
	// only listed spaces/expts enter central mode.
	AllowlistSpaces []int64 `json:"allowlist_spaces" mapstructure:"allowlist_spaces"`
	AllowlistExpts  []int64 `json:"allowlist_expts" mapstructure:"allowlist_expts"`
	DenylistSpaces  []int64 `json:"denylist_spaces" mapstructure:"denylist_spaces"`
	DenylistExpts   []int64 `json:"denylist_expts" mapstructure:"denylist_expts"`

	// PriorityOverrides maps experiment id (as string) to an explicit priority.
	PriorityOverrides map[string]int64 `json:"priority_overrides" mapstructure:"priority_overrides"`

	// Pool holds lane-wide capacities per dimension. Absent dimensions are
	// unconstrained (useful when rolling out one resource at a time).
	Pool map[string]int64 `json:"pool" mapstructure:"pool"`
	// ScopedPool holds per-target / per-evaluator capacities. Keys look like
	// "target:123" or "evaluator_version:456". Only present keys are enforced.
	ScopedPool map[string]int64 `json:"scoped_pool" mapstructure:"scoped_pool"`
}

// Normalize fills defaults and validates enum values. It never panics.
func (c *CentralSchedulingConf) Normalize() *CentralSchedulingConf {
	if c == nil {
		return &CentralSchedulingConf{Mode: CentralModeLegacy}
	}
	out := *c
	if out.Mode == "" {
		out.Mode = CentralModeEnforce
	}
	switch out.Mode {
	case CentralModeLegacy, CentralModeShadow, CentralModeEnforce:
	default:
		out.Mode = CentralModeLegacy
	}
	if strings.TrimSpace(out.EnvNamespace) == "" {
		out.EnvNamespace = CentralDefaultEnv
	}
	if out.TickIntervalSecond <= 0 {
		out.TickIntervalSecond = int(CentralDefaultTickInterval / time.Second)
	}
	if out.ReservationTTLSecond <= 0 {
		out.ReservationTTLSecond = int(CentralDefaultReservationTTL / time.Second)
	}
	if out.OwnershipTTLSecond <= 0 {
		out.OwnershipTTLSecond = int(CentralDefaultOwnershipTTL / time.Second)
	}
	if out.RecentRunWindowSecond <= 0 {
		out.RecentRunWindowSecond = int((10 * time.Minute) / time.Second)
	}
	if out.ScanLimit <= 0 {
		out.ScanLimit = CentralDefaultScanLimit
	}
	if out.MaxItemsPerExperiment <= 0 {
		out.MaxItemsPerExperiment = CentralDefaultMaxItemsPerExperiment
	}
	if out.DefaultItemConcurNum <= 0 {
		out.DefaultItemConcurNum = CentralDefaultItemConcurNum
	}
	if out.AgingSecond <= 0 {
		out.AgingSecond = CentralDefaultAgingSecond
	}
	if out.MaxAgingPriorityBonus < 0 {
		out.MaxAgingPriorityBonus = 0
	}
	if out.MaxAgingPriorityBonus == 0 {
		out.MaxAgingPriorityBonus = CentralDefaultMaxAgingBonus
	}
	if out.FairShareRatio <= 0 || out.FairShareRatio > 1 {
		out.FairShareRatio = CentralDefaultFairShareRatio
	}
	if out.Pool == nil {
		out.Pool = map[string]int64{}
	}
	if out.ScopedPool == nil {
		out.ScopedPool = map[string]int64{}
	}
	if out.PriorityOverrides == nil {
		out.PriorityOverrides = map[string]int64{}
	}
	return &out
}

// TTL helpers.
func (c *CentralSchedulingConf) TickInterval() time.Duration {
	return time.Duration(c.TickIntervalSecond) * time.Second
}

func (c *CentralSchedulingConf) ReservationTTL() time.Duration {
	return time.Duration(c.ReservationTTLSecond) * time.Second
}

func (c *CentralSchedulingConf) OwnershipTTL() time.Duration {
	return time.Duration(c.OwnershipTTLSecond) * time.Second
}

func (c *CentralSchedulingConf) RecentRunWindow() time.Duration {
	return time.Duration(c.RecentRunWindowSecond) * time.Second
}

// ModeFor returns the effective mode for one experiment. Denylist always wins;
// an explicit allowlist entry wins over AllowAllSpaces; otherwise the global
// roll-out mode is used only when the experiment is allowlisted.
func (c *CentralSchedulingConf) ModeFor(spaceID, exptID int64) CentralMode {
	if c == nil || !c.Enabled {
		return CentralModeLegacy
	}
	for _, id := range c.DenylistExpts {
		if id == exptID {
			return CentralModeLegacy
		}
	}
	for _, id := range c.DenylistSpaces {
		if id == spaceID {
			return CentralModeLegacy
		}
	}
	explicit := false
	for _, id := range c.AllowlistExpts {
		if id == exptID {
			explicit = true
			break
		}
	}
	if !explicit {
		for _, id := range c.AllowlistSpaces {
			if id == spaceID {
				explicit = true
				break
			}
		}
	}
	if !explicit && !c.AllowAllSpaces {
		return CentralModeLegacy
	}
	if c.Mode == "" {
		return CentralModeEnforce
	}
	return c.Mode
}

// PoolCapacity returns the configured capacity for a dimension. The second
// return is false when the dimension is unconstrained.
func (c *CentralSchedulingConf) PoolCapacity(dimension string) (int64, bool) {
	if c == nil || c.Pool == nil {
		return 0, false
	}
	v, ok := c.Pool[dimension]
	return v, ok
}

// ScopedPoolCapacity returns a scoped capacity. Scoped dimensions are always
// explicit: absent means unconstrained.
func (c *CentralSchedulingConf) ScopedPoolCapacity(dimension string) (int64, bool) {
	if c == nil || c.ScopedPool == nil {
		return 0, false
	}
	v, ok := c.ScopedPool[dimension]
	return v, ok
}

// ExplicitPriority returns a TCC-configured priority override.
func (c *CentralSchedulingConf) ExplicitPriority(exptID int64) (int64, bool) {
	if c == nil || c.PriorityOverrides == nil {
		return 0, false
	}
	v, ok := c.PriorityOverrides[strconv.FormatInt(exptID, 10)]
	return v, ok
}

// CentralActiveRun is one row returned by the global active-run scan. It is
// intentionally a projection: the scheduler must not load full experiment
// details for every space on every tick.
type CentralActiveRun struct {
	ExptID    int64
	SpaceID   int64
	RunID     int64
	RunMode   ExptRunMode
	RunStatus ExptStatus
	CreatedBy string
	// RunCreatedAt is expt_run_log.created_at; expt_created_at is used for
	// short-lived experiments only in observability.
	RunCreatedAt  time.Time
	ExptCreatedAt time.Time

	ExptType    ExptType
	SourceType  SourceType
	TriggerType string

	// OwnerEnv is the environment namespace that submitted the run. Empty means
	// the run predates central rollout and must stay on the legacy loop.
	OwnerEnv string
}

// CentralExperimentScheduling is the projection the planner needs. It is read
// from experiment + expt_evaluator_ref, never from a mutable in-memory model.
type CentralExperimentScheduling struct {
	ExptID      int64
	SpaceID     int64
	ExptType    ExptType
	SourceType  SourceType
	TriggerType string
	TargetID    int64
	TargetType  EvalTargetType
	ItemConcurNum int
	EvaluatorVersionIDs []int64
	// Ext carries the experiment-level overrides accepted by the IDL
	// CentralSchedulingPolicy (mode/priority/max_item_concur_num). The
	// generated IDL code maps them into eval_conf.Ext; the core reads them
	// here so the contract is usable before the gen repo is regenerated.
	Ext map[string]string
}

const (
	CentralExtModeKey       = "central_scheduling_mode"
	CentralExtPriorityKey   = "central_scheduling_priority"
	CentralExtMaxConcurKey  = "central_scheduling_max_item_concur_num"
)

// CentralModeFromExt lets an experiment override the TCC roll-out mode.
// Unknown/missing values fall back to the configured mode.
func CentralModeFromExt(ext map[string]string, fallback CentralMode) CentralMode {
	if ext == nil {
		return fallback
	}
	switch CentralMode(ext[CentralExtModeKey]) {
	case CentralModeLegacy, CentralModeShadow, CentralModeEnforce:
		return CentralMode(ext[CentralExtModeKey])
	default:
		return fallback
	}
}

// CentralPriorityFromExt returns an experiment-level priority override.
func CentralPriorityFromExt(ext map[string]string) (int64, bool) {
	if ext == nil {
		return 0, false
	}
	raw, ok := ext[CentralExtPriorityKey]
	if !ok || raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// CentralMaxConcurFromExt returns an experiment-level dispatch cap.
func CentralMaxConcurFromExt(ext map[string]string) (int, bool) {
	if ext == nil {
		return 0, false
	}
	raw, ok := ext[CentralExtMaxConcurKey]
	if !ok || raw == "" {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// ResourceDemand builds the per-item resource vector from the projection.
func (e *CentralExperimentScheduling) ResourceDemand() CentralResourceClaims {
	claims := CentralResourceClaims{
		CentralResourceItem:   1,
		CentralResourceTarget: 1,
	}
	if e == nil {
		return claims
	}
	if n := int64(len(e.EvaluatorVersionIDs)); n > 0 {
		claims[CentralResourceEvaluator] = n
	}
	if e.TargetType == EvalTargetTypeSandboxAgent {
		claims[CentralResourceSandbox] = 1
	}
	if e.TargetID > 0 {
		claims["target:"+strconv.FormatInt(e.TargetID, 10)] = 1
	}
	for _, id := range e.EvaluatorVersionIDs {
		if id > 0 {
			claims["evaluator_version:"+strconv.FormatInt(id, 10)] = 1
		}
	}
	return claims
}

// CentralRunSnapshot aggregates item status counts for one run.
type CentralRunSnapshot struct {
	InFlight       int
	Queueing       int
	Terminal       int
	OldestQueuedAt *time.Time
}

// CentralResourceClaims is a resource-vector. Keys are dimension names from
// CentralResource* constants plus optional scoped keys.
type CentralResourceClaims map[string]int64

// Add merges another vector.
func (c CentralResourceClaims) Add(other CentralResourceClaims) CentralResourceClaims {
	if c == nil {
		c = CentralResourceClaims{}
	}
	for k, v := range other {
		c[k] += v
	}
	return c
}

// Clone returns a defensive copy.
func (c CentralResourceClaims) Clone() CentralResourceClaims {
	out := make(CentralResourceClaims, len(c))
	for k, v := range c {
		out[k] = v
	}
	return out
}

// SortedKeys is used for deterministic logs and tests.
func (c CentralResourceClaims) SortedKeys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// CentralReserveRequest is the atomic multi-resource reservation request.
type CentralReserveRequest struct {
	Env           string
	ExptID        int64
	SpaceID       int64
	RunID         int64
	ItemID        int64
	ReservationID string
	Claims        CentralResourceClaims
	// Capacities only contains dimensions that are enforced. The store must
	// check every entry before mutating any counter.
	Capacities map[string]int64
	TTL        time.Duration
	Adopted    bool
}

// CentralReserveResult reports the atomic outcome.
type CentralReserveResult struct {
	Granted       bool
	AlreadyHeld   bool
	ReservationID string
	// ConstrainedBy is the first dimension that lacked capacity, if any.
	ConstrainedBy string
	Remaining     map[string]int64
}

// CentralReservationState is the persisted reservation view.
type CentralReservationState string

const (
	CentralReservationHeld       CentralReservationState = "held"
	CentralReservationDispatched CentralReservationState = "dispatched"
)

// CentralReservation is one held resource vector for one item.
type CentralReservation struct {
	ID      string
	Env     string
	ExptID  int64
	SpaceID int64
	RunID   int64
	ItemID  int64
	Claims  CentralResourceClaims
	// PoolKeys records the dimensions that were actually enforced (and thus
	// incremented) at reserve time. Release must only decrement these, never
	// unconstrained claim dimensions.
	PoolKeys      []string
	State         CentralReservationState
	Adopted       bool
	CreatedAtUnix int64
	UpdatedAtUnix int64
}

// CentralRunLedger is the Redis-side double-entry view for one run. The
// reconcile path compares this against MySQL processing rows.
type CentralRunLedger struct {
	Held       int
	Dispatched int
	Released   int
	Debt       int
	Version    int64
}

// CentralDrift describes a ledger-vs-DB discrepancy.
type CentralDrift struct {
	ExptID  int64
	SpaceID int64
	RunID   int64
	// DBProcessing and RedisHeld are compared directly.
	DBProcessing int
	RedisHeld    int
	// OrphanReservations have a Redis reservation but no DB processing row.
	OrphanReservations []string
	// UnreservedProcessing are DB processing rows without a reservation.
	UnreservedProcessing []int64
	// Rebuilt is true when drift triggered a full rebuild.
	Rebuilt bool
}

// CentralRunGrant is one planned item dispatch.
type CentralRunGrant struct {
	Run      *CentralActiveRun
	ItemID   int64
	Claims   CentralResourceClaims
	Priority int64
	Desired  int
	InFlight int
	Queueing int
}

// CentralTickReport is returned by every tick and is also the observability
// payload. It intentionally contains no secrets and no raw event bodies.
type CentralTickReport struct {
	StartedAt       time.Time
	FinishedAt      time.Time
	Env             string
	Mode            CentralMode
	ScannedRuns     int
	OwnedRuns       int
	SkippedRuns     int
	FinishedRuns    int
	PlannedGrants   int
	ReservedGrants  int
	Dispatched      int
	ReleasedOrphans int
	Drifts          []CentralDrift
	Anomalies       []CentralAnomaly
}

// CentralAnomaly is a dispatch / reconcile anomaly worth alerting on.
type CentralAnomaly struct {
	ExptID  int64
	RunID   int64
	ItemID  int64
	Kind    string
	Message string
	At      time.Time
}

const (
	CentralAnomalyReserveFailed   = "reserve_failed"
	CentralAnomalyDispatchFailed  = "dispatch_failed"
	CentralAnomalyReleaseFailed   = "release_failed"
	CentralAnomalyLedgerDrift     = "ledger_drift"
	CentralAnomalyReservationLost = "reservation_lost"
)

// CentralPriorityBase returns the hard priority band for a run. Higher wins.
// The bands are deliberately coarse; aging and explicit overrides refine them.
func CentralPriorityBase(run *CentralActiveRun, expt *CentralExperimentScheduling) int64 {
	if run == nil {
		return 0
	}
	// Explicit TCC override is applied by the caller.
	switch run.ExptType {
	case ExptType_Online:
		return 100
	case ExptType_Offline:
		// Retry modes must resume quickly to avoid leaving a run half done.
		switch run.RunMode {
		case EvaluationModeFailRetry, EvaluationModeRetryAll, EvaluationModeRetryItems:
			return 70
		case EvaluationModeAppend:
			return 90
		case EvaluationModeTrialRun:
			return 30
		}
		if run.TriggerType == "schedule" {
			return 60
		}
		if expt != nil && expt.SourceType == SourceType_IntelligentGen {
			return 55
		}
		return 50
	}
	return 40
}

// CentralAgedPriority adds a bounded aging bonus.
func CentralAgedPriority(base int64, queuedSince time.Time, now time.Time, cfg *CentralSchedulingConf) int64 {
	if base <= 0 || queuedSince.IsZero() || cfg == nil {
		return base
	}
	wait := now.Sub(queuedSince)
	if wait <= 0 {
		return base
	}
	step := time.Duration(cfg.AgingSecond) * time.Second
	if step <= 0 {
		return base
	}
	bonus := int64(wait / step)
	if max := int64(cfg.MaxAgingPriorityBonus); bonus > max {
		bonus = max
	}
	return base + bonus
}

// CentralResourceDemand builds the per-item resource vector for an experiment.
// Every central dispatch consumes one item slot and one target slot; an
// evaluator slot per evaluator version; a sandbox slot only for SandboxAgent.
func CentralResourceDemand(expt *Experiment) CentralResourceClaims {
	claims := CentralResourceClaims{
		CentralResourceItem:   1,
		CentralResourceTarget: 1,
	}
	if expt == nil {
		return claims
	}
	evaluatorN := int64(len(expt.EvaluatorVersionRef))
	if evaluatorN == 0 {
		// Older callers populate Evaluators instead of refs.
		evaluatorN = int64(len(expt.Evaluators))
	}
	if evaluatorN > 0 {
		claims[CentralResourceEvaluator] = evaluatorN
	}
	if expt.TargetType == EvalTargetTypeSandboxAgent {
		claims[CentralResourceSandbox] = 1
	}
	// Scoped dimensions are optional. They are added only when the experiment
	// exposes a stable id; the store ignores scoped keys without capacity.
	if expt.TargetID > 0 {
		claims["target:"+strconv.FormatInt(expt.TargetID, 10)] = 1
	}
	for _, ref := range expt.EvaluatorVersionRef {
		if ref == nil || ref.EvaluatorVersionID <= 0 {
			continue
		}
		claims["evaluator_version:"+strconv.FormatInt(ref.EvaluatorVersionID, 10)] = 1
	}
	return claims
}

// CentralPoolKeys returns the enforced pool keys for a claim vector. Only
// dimensions that have a configured capacity are returned, in sorted order.
func CentralPoolKeys(claims CentralResourceClaims, capacities map[string]int64) []string {
	if len(claims) == 0 || len(capacities) == 0 {
		return nil
	}
	keys := make([]string, 0, len(claims))
	for k := range claims {
		if _, ok := capacities[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// CentralReservationID is deterministic so a retried dispatch cannot create a
// second reservation for the same item.
func CentralReservationID(exptID, runID, itemID int64) string {
	return "resv:" + strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10) + ":" + strconv.FormatInt(itemID, 10)
}
