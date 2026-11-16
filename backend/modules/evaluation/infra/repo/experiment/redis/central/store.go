// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package central implements the Redis-backed central scheduling store.
//
// Every key is namespaced with "qn:central:<env>:<region>" and every mutation
// sets a TTL. Reservation state migration, refunds, incremental reconcile and
// full rebuild are Lua scripts so multi-resource holds are all-or-nothing,
// idempotent and atomic against concurrent writes.
package central

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	domain "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central"
)

const structuralTTL = 30 * 24 * time.Hour
const reservationKeyGrace = time.Minute
const releasedKeyGrace = time.Minute

type Store struct {
	cmd   redis.Cmdable
	clock domain.Clock
}

var _ domain.Store = (*Store)(nil)

func NewStore(cmd redis.Cmdable) *Store { return &Store{cmd: cmd, clock: domain.DefaultClock()} }

func NewStoreWithClock(cmd redis.Cmdable, clock domain.Clock) *Store {
	if clock == nil {
		clock = domain.DefaultClock()
	}
	return &Store{cmd: cmd, clock: clock}
}

func (s *Store) now() time.Time { return s.clock.Now() }
func (s *Store) reservationLifetime(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return ttl + reservationKeyGrace
}

func (s *Store) keys(scope domain.Scope) (ledger, capacity, resIdx, runs, runIdx, runExp, prio, prioExp, keysIdx string) {
	ns := scope.Namespace()
	return ns + ":ledger", ns + ":cap", ns + ":residx", ns + ":runs", ns + ":runidx",
		ns + ":runexp", ns + ":prio", ns + ":prioexp", ns + ":keys"
}

func resKey(ns, id string) string { return ns + ":res:" + id }

func (s *Store) EnsureLedger(c domain.Ctx, scope domain.Scope, capacity map[domain.ResourceKind]int64) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	ledger, capKey, _, _, _, _, _, _, _ := s.keys(scope)
	args := []any{int64(structuralTTL / time.Second)}
	for _, kind := range domain.KnownResources {
		args = append(args, string(kind), capacity[kind])
	}
	_, err := s.cmd.Eval(ctxOf(c), ensureLedgerScript, []string{ledger, capKey}, args...).Result()
	if err != nil {
		return fmt.Errorf("central redis: ensure ledger: %w", err)
	}
	return nil
}

func (s *Store) Ledger(c domain.Ctx, scope domain.Scope) (domain.Ledger, error) {
	if err := scope.Validate(); err != nil {
		return domain.Ledger{}, err
	}
	ledger, capKey, _, _, _, _, _, _, _ := s.keys(scope)
	caps, err := s.cmd.HGetAll(ctxOf(c), capKey).Result()
	if err != nil && !redis.IsNilError(err) {
		return domain.Ledger{}, err
	}
	avail, err := s.cmd.HGetAll(ctxOf(c), ledger).Result()
	if err != nil && !redis.IsNilError(err) {
		return domain.Ledger{}, err
	}
	ns := scope.Namespace()
	gen, _ := s.cmd.HGet(ctxOf(c), ns+":reconcile", "generation").Int64()
	mode, _ := s.cmd.HGet(ctxOf(c), ns+":reconcile", "mode").Result()
	out := domain.Ledger{
		Scope: scope, Capacity: map[domain.ResourceKind]int64{}, Available: map[domain.ResourceKind]int64{},
		Reserved: map[domain.ResourceKind]int64{}, Generation: gen, ReconcileMode: mode, UpdatedAt: s.now(),
	}
	for k, v := range caps {
		n, _ := strconv.ParseInt(v, 10, 64)
		out.Capacity[domain.ResourceKind(k)] = n
	}
	for k, v := range avail {
		n, _ := strconv.ParseInt(v, 10, 64)
		out.Available[domain.ResourceKind(k)] = n
	}
	reservations, err := s.ListReservations(nil, scope)
	if err != nil {
		return domain.Ledger{}, err
	}
	for kind, v := range domain.SumHolds(reservations) {
		out.Reserved[kind] = v
	}
	return out, nil
}

func (s *Store) RegisterRun(c domain.Ctx, run domain.ActiveRun, ttl time.Duration) error {
	if err := run.Validate(); err != nil {
		return err
	}
	now := s.now()
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	_, _, _, runs, runIdx, runExp, _, _, keysIdx := s.keys(run.Scope)
	keys := []string{runs, runIdx, runExp, keysIdx}
	payload, err := json.Marshal(run)
	if err != nil {
		return err
	}
	exp := now.Add(ttl).Unix()
	if err := s.cmd.HSet(ctxOf(c), runs, run.Key.String(), string(payload)).Err(); err != nil {
		return err
	}
	if err := s.cmd.HSet(ctxOf(c), runExp, run.Key.String(), strconv.FormatInt(exp, 10)).Err(); err != nil {
		return err
	}
	if err := s.cmd.ZAdd(ctxOf(c), runIdx, redisZ(now.Unix(), run.Key.String())).Err(); err != nil {
		return err
	}
	for _, k := range keys {
		_ = s.cmd.Expire(ctxOf(c), k, structuralTTL).Err()
		_ = s.cmd.ZAdd(ctxOf(c), keysIdx, redisZ(now.Unix(), k)).Err()
	}
	return nil
}

func (s *Store) ListRuns(c domain.Ctx, scope domain.Scope) ([]domain.ActiveRun, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	_, _, _, runs, runIdx, runExp, _, _, _ := s.keys(scope)
	members, err := s.cmd.ZRange(ctxOf(c), runIdx, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, err
	}
	now := s.now().Unix()
	out := make([]domain.ActiveRun, 0, len(members))
	for _, key := range members {
		expStr, _ := s.cmd.HGet(ctxOf(c), runExp, key).Result()
		if exp, perr := strconv.ParseInt(expStr, 10, 64); perr == nil && exp < now {
			continue
		}
		payload, gerr := s.cmd.HGet(ctxOf(c), runs, key).Result()
		if gerr != nil {
			continue
		}
		var run domain.ActiveRun
		if jerr := json.Unmarshal([]byte(payload), &run); jerr != nil {
			continue
		}
		if scope.SpaceID != 0 && run.Key.SpaceID != scope.SpaceID {
			continue
		}
		out = append(out, run)
	}
	domain.SortRuns(out)
	return out, nil
}

func (s *Store) UnregisterRun(c domain.Ctx, scope domain.Scope, key domain.RunKey) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	_, _, _, runs, _, runExp, _, _, _ := s.keys(scope)
	if err := s.cmd.HDel(ctxOf(c), runs, key.String()).Err(); err != nil {
		return err
	}
	return s.cmd.HDel(ctxOf(c), runExp, key.String()).Err()
}

func (s *Store) GetPriority(c domain.Ctx, scope domain.Scope, key domain.RunKey) (int, bool, error) {
	if err := scope.Validate(); err != nil {
		return 0, false, err
	}
	_, _, _, _, _, _, prio, prioExp, _ := s.keys(scope)
	expStr, err := s.cmd.HGet(ctxOf(c), prioExp, key.String()).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	exp, perr := strconv.ParseInt(expStr, 10, 64)
	if perr == nil && exp < s.now().Unix() {
		return 0, false, nil
	}
	val, err := s.cmd.HGet(ctxOf(c), prio, key.String()).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	p, perr := strconv.Atoi(val)
	if perr != nil {
		return 0, false, nil
	}
	return p, true, nil
}

func (s *Store) SetPriority(c domain.Ctx, scope domain.Scope, key domain.RunKey, priority int, ttl time.Duration) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := key.Validate(); err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	_, _, _, _, _, _, prio, prioExp, keysIdx := s.keys(scope)
	if err := s.cmd.HSet(ctxOf(c), prio, key.String(), strconv.Itoa(priority)).Err(); err != nil {
		return err
	}
	if err := s.cmd.HSet(ctxOf(c), prioExp, key.String(), strconv.FormatInt(s.now().Add(ttl).Unix(), 10)).Err(); err != nil {
		return err
	}
	_ = s.cmd.Expire(ctxOf(c), prio, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), prioExp, structuralTTL).Err()
	_ = s.cmd.ZAdd(ctxOf(c), keysIdx, redisZ(s.now().Unix(), prio)).Err()
	return nil
}

func (s *Store) TryReserve(c domain.Ctx, scope domain.Scope, key domain.RunKey, itemID int64, holds map[domain.ResourceKind]int64, ttl time.Duration) (domain.ReserveResult, error) {
	if err := scope.Validate(); err != nil {
		return domain.ReserveDenied, err
	}
	if err := key.Validate(); err != nil {
		return domain.ReserveDenied, err
	}
	if itemID <= 0 || len(holds) == 0 {
		return domain.ReserveDenied, domain.ErrInvalidReserve
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := s.now()
	id := domain.ReservationID(scope, key, itemID)
	ledger, _, resIdx, _, _, _, _, _, keysIdx := s.keys(scope)
	rkey := resKey(scope.Namespace(), id)
	args := []any{id, key.String(), strconv.FormatInt(itemID, 10), int64(ttl / time.Second), now.Unix(), scope.Env, scope.Region, strconv.FormatInt(scope.SpaceID, 10)}
	for _, kind := range domain.KnownResources {
		if amount, ok := holds[kind]; ok && amount > 0 {
			args = append(args, string(kind), amount)
		}
	}
	res, err := s.cmd.Eval(ctxOf(c), tryReserveScript, []string{ledger, rkey, resIdx, keysIdx}, args...).Int64()
	if err != nil {
		return domain.ReserveDenied, fmt.Errorf("central redis: reserve: %w", err)
	}
	switch res {
	case 1:
		_ = s.cmd.ZAdd(ctxOf(c), keysIdx, redisZ(now.Unix(), rkey)).Err()
		_ = s.cmd.Expire(ctxOf(c), resIdx, structuralTTL).Err()
		_ = s.cmd.Expire(ctxOf(c), keysIdx, structuralTTL).Err()
		return domain.ReserveGranted, nil
	case 2:
		return domain.ReserveExisting, nil
	case -1:
		return domain.ReserveDenied, domain.ErrCrossEnvironment
	default:
		return domain.ReserveDenied, nil
	}
}

func (s *Store) GetReservation(c domain.Ctx, scope domain.Scope, id string) (*domain.Reservation, bool, error) {
	if err := scope.Validate(); err != nil {
		return nil, false, err
	}
	rkey := resKey(scope.Namespace(), id)
	rec, ok, err := s.readReservation(c, rkey)
	if err != nil || !ok {
		return nil, false, err
	}
	if err := domain.ValidateOwnership(scope, *rec); err != nil {
		return nil, false, err
	}
	return rec, true, nil
}

func (s *Store) ListReservations(c domain.Ctx, scope domain.Scope) ([]domain.Reservation, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	_, _, resIdx, _, _, _, _, _, _ := s.keys(scope)
	members, err := s.cmd.ZRange(ctxOf(c), resIdx, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, err
	}
	out := make([]domain.Reservation, 0, len(members))
	for _, id := range members {
		rec, ok, rerr := s.readReservation(c, resKey(scope.Namespace(), id))
		if rerr != nil {
			return nil, rerr
		}
		if !ok || !rec.State.HoldsQuota() {
			continue
		}
		if rec.Scope.Namespace() != scope.Namespace() {
			continue
		}
		if scope.SpaceID != 0 && rec.Key.SpaceID != scope.SpaceID {
			continue
		}
		out = append(out, *rec)
	}
	return out, nil
}

func (s *Store) Release(c domain.Ctx, scope domain.Scope, id string, reason string) (bool, error) {
	if err := scope.Validate(); err != nil {
		return false, err
	}
	ledger, _, resIdx, _, _, _, _, _, keysIdx := s.keys(scope)
	rkey := resKey(scope.Namespace(), id)
	res, err := s.cmd.Eval(ctxOf(c), releaseScript, []string{rkey, resIdx, keysIdx, ledger}, id, scope.Env, scope.Region, reason, s.now().Unix(), int64(releasedKeyGrace/time.Second)).Int64()
	if err != nil {
		return false, fmt.Errorf("central redis: release: %w", err)
	}
	if res == -1 {
		return false, domain.ErrCrossEnvironment
	}
	_ = s.cmd.Expire(ctxOf(c), ledger, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), resIdx, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), keysIdx, structuralTTL).Err()
	return res == 1, nil
}

func (s *Store) Transition(c domain.Ctx, scope domain.Scope, id string, from, to domain.ReservationState, reason string, dispatchedAt time.Time) (bool, domain.ReservationState, error) {
	if err := scope.Validate(); err != nil {
		return false, "", err
	}
	if !to.Valid() || to == domain.ReservationReleased {
		return false, "", domain.ErrInvalidReserve
	}
	rkey := resKey(scope.Namespace(), id)
	disp := int64(0)
	if !dispatchedAt.IsZero() {
		disp = dispatchedAt.Unix()
	}
	res, err := s.cmd.Eval(ctxOf(c), transitionScript, []string{rkey}, id, string(from), string(to), reason, s.now().Unix(), disp).Int64()
	if err != nil {
		return false, "", fmt.Errorf("central redis: transition: %w", err)
	}
	if res == 0 {
		return false, "", nil
	}
	rec, ok, rerr := s.readReservation(c, rkey)
	if rerr != nil || !ok {
		return res == 1, "", rerr
	}
	return res == 1, rec.State, nil
}

func (s *Store) RenewReservations(c domain.Ctx, scope domain.Scope, key domain.RunKey, ttl time.Duration) (int, error) {
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	reservations, err := s.ListReservations(nil, scope)
	if err != nil {
		return 0, err
	}
	_, _, resIdx, _, _, _, _, _, keysIdx := s.keys(scope)
	now := s.now()
	n := 0
	lifetime := s.reservationLifetime(ttl)
	for _, r := range reservations {
		if r.Key != key || !r.State.HoldsQuota() {
			continue
		}
		rkey := resKey(scope.Namespace(), r.ID)
		if err := s.cmd.HSet(ctxOf(c), rkey, "exp", now.Add(ttl).Unix()).Err(); err != nil {
			continue
		}
		if err := s.cmd.Expire(ctxOf(c), rkey, lifetime).Err(); err != nil {
			continue
		}
		_ = s.cmd.ZAdd(ctxOf(c), resIdx, redisZ(now.Add(ttl).Unix(), r.ID)).Err()
		_ = s.cmd.ZAdd(ctxOf(c), keysIdx, redisZ(now.Unix(), rkey)).Err()
		n++
	}
	_ = s.cmd.Expire(ctxOf(c), resIdx, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), keysIdx, structuralTTL).Err()
	return n, nil
}

func (s *Store) IncrementalReconcile(c domain.Ctx, scope domain.Scope, capacity map[domain.ResourceKind]int64, maxDrift int64) (domain.Ledger, bool, error) {
	if err := scope.Validate(); err != nil {
		return domain.Ledger{}, false, err
	}
	ledger, capKey, resIdx, _, _, _, _, _, _ := s.keys(scope)
	reconcileKey := scope.Namespace() + ":reconcile"
	args := []any{scope.Namespace() + ":res:", s.now().Unix(), maxDrift}
	for _, kind := range domain.KnownResources {
		args = append(args, string(kind), capacity[kind])
	}
	vals, err := s.cmd.Eval(ctxOf(c), incrementalReconcileScript, []string{ledger, capKey, resIdx, reconcileKey}, args...).Int64Slice()
	if err != nil {
		return domain.Ledger{}, false, fmt.Errorf("central redis: incremental reconcile: %w", err)
	}
	needFull := false
	if len(vals) >= 2 {
		needFull = vals[0] > maxDrift
	}
	_ = s.cmd.Expire(ctxOf(c), ledger, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), capKey, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), reconcileKey, structuralTTL).Err()
	l, lerr := s.Ledger(nil, scope)
	return l, needFull, lerr
}

func (s *Store) RebuildLedger(c domain.Ctx, scope domain.Scope, capacity map[domain.ResourceKind]int64) (domain.Ledger, error) {
	if err := scope.Validate(); err != nil {
		return domain.Ledger{}, err
	}
	ledger, capKey, resIdx, _, _, _, _, _, _ := s.keys(scope)
	reconcileKey := scope.Namespace() + ":reconcile"
	args := []any{scope.Namespace() + ":res:", s.now().Unix()}
	for _, kind := range domain.KnownResources {
		args = append(args, string(kind), capacity[kind])
	}
	if _, err := s.cmd.Eval(ctxOf(c), rebuildScript, []string{ledger, capKey, resIdx, reconcileKey}, args...).Result(); err != nil {
		return domain.Ledger{}, fmt.Errorf("central redis: rebuild: %w", err)
	}
	_ = s.cmd.Expire(ctxOf(c), ledger, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), capKey, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), resIdx, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), reconcileKey, structuralTTL).Err()
	return s.Ledger(nil, scope)
}

func (s *Store) GetRunMode(c domain.Ctx, scope domain.Scope, key domain.RunKey) (domain.Mode, bool, error) {
	if err := scope.Validate(); err != nil {
		return domain.ModeLegacy, false, err
	}
	val, err := s.cmd.HGet(ctxOf(c), scope.Namespace()+":runmode", key.String()).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return domain.ModeLegacy, false, nil
		}
		return domain.ModeLegacy, false, err
	}
	parts := strings.SplitN(val, "|", 2)
	mode := domain.ParseMode(parts[0])
	if len(parts) == 2 {
		if exp, perr := strconv.ParseInt(parts[1], 10, 64); perr == nil && exp < s.now().Unix() {
			return domain.ModeLegacy, false, nil
		}
	}
	return mode, true, nil
}

func (s *Store) SetRunMode(c domain.Ctx, scope domain.Scope, key domain.RunKey, mode domain.Mode, ttl time.Duration) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	keyName := scope.Namespace() + ":runmode"
	val := mode.String() + "|" + strconv.FormatInt(s.now().Add(ttl).Unix(), 10)
	if err := s.cmd.HSet(ctxOf(c), keyName, key.String(), val).Err(); err != nil {
		return err
	}
	_ = s.cmd.Expire(ctxOf(c), keyName, structuralTTL).Err()
	_, _, _, _, _, _, _, _, keysIdx := s.keys(scope)
	_ = s.cmd.ZAdd(ctxOf(c), keysIdx, redisZ(s.now().Unix(), keyName)).Err()
	return nil
}

func (s *Store) AcquireLeadership(c domain.Ctx, scope domain.Scope, holder string, ttl time.Duration) (bool, error) {
	if err := scope.Validate(); err != nil {
		return false, err
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	keyName := scope.Namespace() + ":leader"
	res, err := s.cmd.Eval(ctxOf(c), acquireLeaderScript, []string{keyName}, holder, int64(ttl/time.Millisecond)).Int64()
	if err != nil {
		return false, err
	}
	if res == 1 {
		_, _, _, _, _, _, _, _, keysIdx := s.keys(scope)
		_ = s.cmd.ZAdd(ctxOf(c), keysIdx, redisZ(s.now().Unix(), keyName)).Err()
		_ = s.cmd.Expire(ctxOf(c), keysIdx, structuralTTL).Err()
	}
	return res == 1, nil
}

func (s *Store) RenewLeadership(c domain.Ctx, scope domain.Scope, holder string, ttl time.Duration) (bool, error) {
	keyName := scope.Namespace() + ":leader"
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	res, err := s.cmd.Eval(ctxOf(c), renewLeaderScript, []string{keyName}, holder, int64(ttl/time.Millisecond)).Int64()
	return res == 1, err
}

func (s *Store) ReleaseLeadership(c domain.Ctx, scope domain.Scope, holder string) error {
	keyName := scope.Namespace() + ":leader"
	_, err := s.cmd.Eval(ctxOf(c), releaseLeaderScript, []string{keyName}, holder).Result()
	return err
}

func (s *Store) Cleanup(c domain.Ctx, scope domain.Scope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	_, _, _, _, _, _, _, _, keysIdx := s.keys(scope)
	_, err := s.cmd.Eval(ctxOf(c), cleanupScript, []string{keysIdx}, scope.Namespace()).Result()
	if err != nil {
		return fmt.Errorf("central redis: cleanup: %w", err)
	}
	return nil
}

func ctxOf(c domain.Ctx) context.Context {
	if c == nil {
		return context.Background()
	}
	if cc, ok := c.(context.Context); ok {
		return cc
	}
	return context.Background()
}

func (s *Store) readReservation(c domain.Ctx, rkey string) (*domain.Reservation, bool, error) {
	exists, err := s.cmd.Exists(ctxOf(c), rkey).Result()
	if err != nil || exists == 0 {
		return nil, false, err
	}
	fields, err := s.cmd.HGetAll(ctxOf(c), rkey).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(fields) == 0 {
		return nil, false, nil
	}
	rec := &domain.Reservation{ID: fields["id"], Holds: map[domain.ResourceKind]int64{}, State: domain.ReservationState(fields["state"])}
	if rec.State == "" {
		rec.State = domain.ReservationReserved
	}
	rec.Scope.Env = fields["env"]
	rec.Scope.Region = fields["region"]
	rec.Scope.SpaceID, _ = strconv.ParseInt(fields["space"], 10, 64)
	parts := strings.Split(fields["run"], "/")
	if len(parts) == 3 {
		rec.Key.SpaceID, _ = strconv.ParseInt(parts[0], 10, 64)
		rec.Key.ExptID, _ = strconv.ParseInt(parts[1], 10, 64)
		rec.Key.ExptRunID, _ = strconv.ParseInt(parts[2], 10, 64)
	}
	rec.ItemID, _ = strconv.ParseInt(fields["item"], 10, 64)
	for field, val := range fields {
		if strings.HasPrefix(field, "hold:") {
			n, _ := strconv.ParseInt(val, 10, 64)
			rec.Holds[domain.ResourceKind(strings.TrimPrefix(field, "hold:"))] = n
		}
	}
	if v, ok := fields["created"]; ok {
		if n, perr := strconv.ParseInt(v, 10, 64); perr == nil {
			rec.CreatedAt = time.Unix(n, 0)
		}
	}
	if v, ok := fields["dispatch_at"]; ok {
		if n, perr := strconv.ParseInt(v, 10, 64); perr == nil && n > 0 {
			rec.DispatchedAt = time.Unix(n, 0)
		}
	}
	if v, ok := fields["state_at"]; ok {
		if n, perr := strconv.ParseInt(v, 10, 64); perr == nil {
			rec.StateAt = time.Unix(n, 0)
		}
	}
	rec.Reason = fields["reason"]
	if v, ok := fields["exp"]; ok {
		if n, perr := strconv.ParseInt(v, 10, 64); perr == nil {
			rec.ExpiresAt = time.Unix(n, 0)
		}
	}
	return rec, true, nil
}

func redisZ(score int64, member string) goredis.Z {
	return goredis.Z{Score: float64(score), Member: member}
}

// ---- Lua scripts ----

const ensureLedgerScript = `
local ledger=KEYS[1]; local cap=KEYS[2]
local ttl=tonumber(ARGV[1])
for i=2,#ARGV,2 do
  local field=ARGV[i]; local capv=tonumber(ARGV[i+1])
  local old=tonumber(redis.call('HGET', cap, field) or '-1')
  if old < 0 then
    redis.call('HSET', ledger, field, capv)
  else
    local av=tonumber(redis.call('HGET', ledger, field) or '0') + (capv-old)
    if av < 0 then av=0 end
    redis.call('HSET', ledger, field, av)
  end
  redis.call('HSET', cap, field, capv)
end
redis.call('EXPIRE', ledger, ttl)
redis.call('EXPIRE', cap, ttl)
return 1
`

const tryReserveScript = `
local ledger=KEYS[1]; local res=KEYS[2]; local idx=KEYS[3]; local keysidx=KEYS[4]
local id=ARGV[1]; local ttl=tonumber(ARGV[4]); local now=tonumber(ARGV[5])
local keyttl=ttl+60
if redis.call('EXISTS', res)==1 then
  local state=redis.call('HGET', res,'state') or 'reserved'
  local exp=tonumber(redis.call('HGET', res,'exp') or '0')
  if state ~= 'released' and exp > now then
    if redis.call('HGET', res,'env') ~= ARGV[6] or redis.call('HGET', res,'region') ~= ARGV[7] then
      return -1
    end
    return 2
  end
  if state ~= 'released' then
    local h=redis.call('HGETALL', res)
    for i=1,#h,2 do
      local f=h[i]
      if string.sub(f,1,5)=='hold:' then
        redis.call('HINCRBY', ledger, string.sub(f,6), tonumber(h[i+1]))
      end
    end
  end
  redis.call('DEL', res)
  redis.call('ZREM', idx, id)
end
for i=9,#ARGV,2 do
  local field=ARGV[i]; local amt=tonumber(ARGV[i+1])
  local av=tonumber(redis.call('HGET', ledger, field) or '0')
  if av < amt then return 0 end
end
for i=9,#ARGV,2 do
  local field=ARGV[i]; local amt=tonumber(ARGV[i+1])
  redis.call('HINCRBY', ledger, field, -amt)
  redis.call('HSET', res, 'hold:'..field, amt)
end
redis.call('HSET', res, 'id', id, 'run', ARGV[2], 'item', ARGV[3], 'env', ARGV[6], 'region', ARGV[7], 'space', ARGV[8], 'created', now, 'state', 'reserved', 'state_at', now, 'exp', now+ttl)
redis.call('EXPIRE', res, keyttl)
redis.call('ZADD', idx, now+ttl, id)
redis.call('ZADD', keysidx, now, res)
return 1
`

const releaseScript = `
local res=KEYS[1]; local idx=KEYS[2]; local keysidx=KEYS[3]; local ledger=KEYS[4]
local id=ARGV[1]; local now=tonumber(ARGV[5])
if redis.call('EXISTS', res)==0 then return 0 end
if redis.call('HGET', res,'env') ~= ARGV[2] or redis.call('HGET', res,'region') ~= ARGV[3] then
  return -1
end
local state=redis.call('HGET', res,'state') or 'reserved'
if state == 'released' then return 0 end
local h=redis.call('HGETALL', res)
for i=1,#h,2 do
  local f=h[i]
  if string.sub(f,1,5)=='hold:' then
    redis.call('HINCRBY', ledger, string.sub(f,6), tonumber(h[i+1]))
  end
end
redis.call('HSET', res, 'state', 'released', 'state_at', now, 'reason', ARGV[4])
redis.call('EXPIRE', res, tonumber(ARGV[6]))
redis.call('ZREM', idx, id)
redis.call('ZADD', keysidx, now, res)
return 1
`

const transitionScript = `
local res=KEYS[1]; local id=ARGV[1]; local from=ARGV[2]; local to=ARGV[3]
if redis.call('EXISTS', res)==0 then return 0 end
local cur=redis.call('HGET', res,'state') or 'reserved'
if cur == to then
  if ARGV[6] ~= '0' then redis.call('HSET', res, 'dispatch_at', tonumber(ARGV[6])) end
  return 1
end
if from ~= '' and cur ~= from then return 0 end
local ok=false
if (cur=='reserved' and to=='dispatched') or (cur=='reserved' and to=='running')
 or (cur=='reserved' and to=='completed') or (cur=='reserved' and to=='failed')
 or (cur=='reserved' and to=='canceled') or (cur=='reserved' and to=='pending_release')
 or (cur=='dispatched' and to=='running') or (cur=='dispatched' and to=='completed')
 or (cur=='dispatched' and to=='failed') or (cur=='dispatched' and to=='canceled')
 or (cur=='dispatched' and to=='pending_release')
 or (cur=='running' and to=='completed') or (cur=='running' and to=='failed')
 or (cur=='running' and to=='canceled') or (cur=='running' and to=='pending_release')
 or (cur=='completed' and to=='pending_release') or (cur=='failed' and to=='pending_release')
 or (cur=='canceled' and to=='pending_release') then ok=true end
if not ok then return 0 end
redis.call('HSET', res, 'state', to, 'state_at', tonumber(ARGV[5]), 'reason', ARGV[4])
if ARGV[6] ~= '0' then redis.call('HSET', res, 'dispatch_at', tonumber(ARGV[6])) end
return 1
`

const incrementalReconcileScript = `
local ledger=KEYS[1]; local cap=KEYS[2]; local idx=KEYS[3]; local rec=KEYS[4]
local prefix=ARGV[1]; local now=ARGV[2]; local maxDrift=tonumber(ARGV[3])
local members=redis.call('ZRANGE', idx, 0, -1)
local used={}
for _,m in ipairs(members) do
  local k=prefix..m
  if redis.call('EXISTS', k)==1 then
    local state=redis.call('HGET', k,'state') or 'reserved'
    if state ~= 'released' then
      local h=redis.call('HGETALL', k)
      for i=1,#h,2 do
        local f=h[i]
        if string.sub(f,1,5)=='hold:' then
          local rk=string.sub(f,6)
          used[rk]=(used[rk] or 0)+tonumber(h[i+1])
        end
      end
    end
  else
    redis.call('ZREM', idx, m)
  end
end
local drift=0
for i=4,#ARGV,2 do
  local field=ARGV[i]; local capv=tonumber(ARGV[i+1])
  local expected=capv-(used[field] or 0)
  if expected<0 then expected=0 end
  local av=tonumber(redis.call('HGET', ledger, field) or '0')
  local diff=av-expected
  if diff<0 then diff=-diff end
  if diff>drift then drift=diff end
  redis.call('HSET', ledger, field, expected)
  redis.call('HSET', cap, field, capv)
end
local gen=tonumber(redis.call('HGET', rec,'generation') or '0')
redis.call('HSET', rec, 'mode', 'incremental', 'last_at', now, 'generation', gen)
return {drift, gen}
`

const rebuildScript = `
local ledger=KEYS[1]; local cap=KEYS[2]; local idx=KEYS[3]; local rec=KEYS[4]
local prefix=ARGV[1]
local members=redis.call('ZRANGE', idx, 0, -1)
local used={}
for _,m in ipairs(members) do
  local k=prefix..m
  if redis.call('EXISTS', k)==1 then
    local state=redis.call('HGET', k,'state') or 'reserved'
    if state ~= 'released' then
      local h=redis.call('HGETALL', k)
      for i=1,#h,2 do
        local f=h[i]
        if string.sub(f,1,5)=='hold:' then
          local rk=string.sub(f,6)
          used[rk]=(used[rk] or 0)+tonumber(h[i+1])
        end
      end
    end
  else
    redis.call('ZREM', idx, m)
  end
end
for i=3,#ARGV,2 do
  local field=ARGV[i]; local capv=tonumber(ARGV[i+1])
  local av=capv-(used[field] or 0)
  if av<0 then av=0 end
  redis.call('HSET', ledger, field, av)
  redis.call('HSET', cap, field, capv)
end
local gen=redis.call('HINCRBY', rec, 'generation', 1)
redis.call('HSET', rec, 'mode', 'full', 'last_at', ARGV[2])
return gen
`

const cleanupScript = `
local keysidx=KEYS[1]; local ns=ARGV[1]
local members=redis.call('ZRANGE', keysidx, 0, -1)
for _,k in ipairs(members) do
  if string.sub(k,1,#ns)==ns then redis.call('DEL', k) end
end
redis.call('DEL', keysidx)
redis.call('DEL', ns..':ledger', ns..':cap', ns..':residx', ns..':runs', ns..':runidx', ns..':runexp', ns..':prio', ns..':prioexp')
redis.call('DEL', ns..':reconcile', ns..':runmode', ns..':leader')
return 1
`

const acquireLeaderScript = `
local key=KEYS[1]; local holder=ARGV[1]; local ms=tonumber(ARGV[2])
if redis.call('SET', key, holder, 'NX', 'PX', ms) then return 1 end
if redis.call('GET', key) == holder then redis.call('PEXPIRE', key, ms); return 1 end
return 0
`

const renewLeaderScript = `
local key=KEYS[1]; local holder=ARGV[1]; local ms=tonumber(ARGV[2])
if redis.call('GET', key) == holder then redis.call('PEXPIRE', key, ms); return 1 end
return 0
`

const releaseLeaderScript = `
local key=KEYS[1]; local holder=ARGV[1]
if redis.call('GET', key) == holder then redis.call('DEL', key); return 1 end
return 0
`
