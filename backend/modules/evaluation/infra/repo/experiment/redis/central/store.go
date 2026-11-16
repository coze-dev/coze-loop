// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package central implements the Redis-backed central scheduling store.
//
// Every key is namespaced with "qn:central:<env>:<region>" and every mutation
// sets a TTL. Reservation refunds and ledger rebuilds are performed by Lua so
// multi-resource holds are all-or-nothing and release is idempotent.
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

// Structural key TTL. Individual reservations use the configured reservation TTL.
const structuralTTL = 30 * 24 * time.Hour

// reservationKeyGrace keeps the Redis hash alive a little after its logical
// expiry so the scheduler can still refund the hold exactly once. Without it,
// Redis would evict the hash at the same moment the hold becomes reclaimable
// and the reserved capacity would leak until the next ledger rebuild.
const reservationKeyGrace = time.Minute

type Store struct {
	cmd   redis.Cmdable
	clock domain.Clock
}

var _ domain.Store = (*Store)(nil)

func NewStore(cmd redis.Cmdable) *Store {
	return &Store{cmd: cmd, clock: domain.DefaultClock()}
}

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

// EnsureLedger initialises capacity without clobbering usage, adjusting
// available by the capacity delta when the pool size changes.
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
	out := domain.Ledger{
		Scope:     scope,
		Capacity:  map[domain.ResourceKind]int64{},
		Available: map[domain.ResourceKind]int64{},
		Reserved:  map[domain.ResourceKind]int64{},
		UpdatedAt: s.now(),
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
	id := domain.ReservationID(key, itemID)
	ledger, _, resIdx, _, _, _, _, _, keysIdx := s.keys(scope)
	rkey := resKey(scope.Namespace(), id)
	args := []any{
		id,
		key.String(),
		strconv.FormatInt(itemID, 10),
		int64(ttl / time.Second),
		now.Unix(),
		scope.Env,
		scope.Region,
		strconv.FormatInt(scope.SpaceID, 10),
	}
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
		if !ok {
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

func (s *Store) Release(c domain.Ctx, scope domain.Scope, id string) (bool, error) {
	if err := scope.Validate(); err != nil {
		return false, err
	}
	ledger, _, resIdx, _, _, _, _, _, keysIdx := s.keys(scope)
	rkey := resKey(scope.Namespace(), id)
	res, err := s.cmd.Eval(ctxOf(c), releaseScript, []string{rkey, resIdx, keysIdx, ledger}, id, scope.Env, scope.Region).Int64()
	if err != nil {
		return false, fmt.Errorf("central redis: release: %w", err)
	}
	if res == -1 {
		return false, domain.ErrCrossEnvironment
	}
	// The release script HINCRBYs the ledger; never leave a key without TTL, even
	// when the ledger had expired before the release was applied.
	_ = s.cmd.Expire(ctxOf(c), ledger, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), resIdx, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), keysIdx, structuralTTL).Err()
	return res == 1, nil
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
		if r.Key != key {
			continue
		}
		rkey := resKey(scope.Namespace(), r.ID)
		// Renew both the logical expiry (read by the reconciler) and the Redis
		// key TTL (which is kept slightly longer so an expired hold can still be
		// refunded exactly once).
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

func (s *Store) RebuildLedger(c domain.Ctx, scope domain.Scope, capacity map[domain.ResourceKind]int64) (domain.Ledger, error) {
	if err := scope.Validate(); err != nil {
		return domain.Ledger{}, err
	}
	ledger, capKey, resIdx, _, _, _, _, _, _ := s.keys(scope)
	lockKey := scope.Namespace() + ":rebuild:lock"
	locked, err := s.cmd.SetNX(ctxOf(c), lockKey, "1", 10*time.Second).Result()
	if err != nil {
		return domain.Ledger{}, err
	}
	if !locked {
		return s.Ledger(nil, scope)
	}
	defer s.cmd.Del(ctxOf(c), lockKey)
	args := []any{scope.Namespace() + ":res:"}
	for _, kind := range domain.KnownResources {
		args = append(args, string(kind), capacity[kind])
	}
	if _, err := s.cmd.Eval(ctxOf(c), rebuildScript, []string{ledger, capKey, resIdx}, args...).Result(); err != nil {
		return domain.Ledger{}, fmt.Errorf("central redis: rebuild: %w", err)
	}
	_ = s.cmd.Expire(ctxOf(c), ledger, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), capKey, structuralTTL).Err()
	_ = s.cmd.Expire(ctxOf(c), resIdx, structuralTTL).Err()
	return s.Ledger(nil, scope)
}

func (s *Store) Cleanup(c domain.Ctx, scope domain.Scope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	_, _, _, _, _, _, _, _, keysIdx := s.keys(scope)
	// Remove every tracked key, then the structural keys and both indexes. The
	// Lua script only receives keys from this scope's own tracking set.
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
	rec := &domain.Reservation{
		ID:    fields["id"],
		Key:   parseRunKey(fields["run"]),
		Holds: map[domain.ResourceKind]int64{},
		State: domain.ReservationHeld,
	}
	rec.Scope = domain.Scope{Env: fields["env"], Region: fields["region"]}
	if space, perr := strconv.ParseInt(fields["space"], 10, 64); perr == nil {
		rec.Scope.SpaceID = space
	}
	if item, perr := strconv.ParseInt(fields["item"], 10, 64); perr == nil {
		rec.ItemID = item
	}
	if created, perr := strconv.ParseInt(fields["created"], 10, 64); perr == nil {
		rec.CreatedAt = time.Unix(created, 0)
	}
	if exp, perr := strconv.ParseInt(fields["exp"], 10, 64); perr == nil {
		rec.ExpiresAt = time.Unix(exp, 0)
	}
	for k, v := range fields {
		if !strings.HasPrefix(k, "hold:") {
			continue
		}
		n, perr := strconv.ParseInt(v, 10, 64)
		if perr != nil {
			continue
		}
		rec.Holds[domain.ResourceKind(strings.TrimPrefix(k, "hold:"))] = n
	}
	return rec, true, nil
}

func parseRunKey(s string) domain.RunKey {
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return domain.RunKey{}
	}
	space, _ := strconv.ParseInt(parts[0], 10, 64)
	expt, _ := strconv.ParseInt(parts[1], 10, 64)
	run, _ := strconv.ParseInt(parts[2], 10, 64)
	return domain.RunKey{SpaceID: space, ExptID: expt, ExptRunID: run}
}

func redisZ(score int64, member string) goredis.Z {
	return goredis.Z{Score: float64(score), Member: member}
}

// IsNil is re-exported for tests.
func IsNil(err error) bool { return redis.IsNilError(err) }

// ensureLedgerScript: ARGV = ttlSeconds, then (field, capacity) pairs.
const ensureLedgerScript = `
local ledger=KEYS[1]; local cap=KEYS[2]
local ttl=tonumber(ARGV[1])
for i=2,#ARGV,2 do
  local field=ARGV[i]
  local newCap=tonumber(ARGV[i+1])
  local oldCap=tonumber(redis.call('HGET', cap, field) or '0')
  local has=redis.call('HEXISTS', cap, field)
  local av=tonumber(redis.call('HGET', ledger, field) or '0')
  if has==0 then
    av=newCap
  else
    av=av+(newCap-oldCap)
    if av<0 then av=0 end
  end
  redis.call('HSET', ledger, field, av)
  redis.call('HSET', cap, field, newCap)
end
redis.call('EXPIRE', ledger, ttl)
redis.call('EXPIRE', cap, ttl)
return 1
`

// tryReserveScript:
// ARGV[1]=id, [2]=runKey, [3]=itemID, [4]=ttlSec, [5]=now, [6]=env, [7]=region,
// [8]=spaceID, then (resource, amount) pairs from 9.
const tryReserveScript = `
local ledger=KEYS[1]; local res=KEYS[2]; local idx=KEYS[3]; local keysidx=KEYS[4]
local id=ARGV[1]; local ttl=tonumber(ARGV[4]); local now=tonumber(ARGV[5])
local keyttl=ttl+60
if redis.call('EXISTS', res)==1 then
  local exp=tonumber(redis.call('HGET', res, 'exp') or '0')
  if exp > now then
    if redis.call('HGET', res,'env') ~= ARGV[6] or redis.call('HGET', res,'region') ~= ARGV[7] then
      return -1
    end
    return 2
  end
  local h=redis.call('HGETALL', res)
  for i=1,#h,2 do
    local f=h[i]
    if string.sub(f,1,5)=='hold:' then
      redis.call('HINCRBY', ledger, string.sub(f,6), tonumber(h[i+1]))
    end
  end
  redis.call('DEL', res)
  redis.call('ZREM', idx, id)
end
for i=9,#ARGV,2 do
  local field=ARGV[i]
  local amt=tonumber(ARGV[i+1])
  local av=tonumber(redis.call('HGET', ledger, field) or '0')
  if av < amt then return 0 end
end
for i=9,#ARGV,2 do
  local field=ARGV[i]
  local amt=tonumber(ARGV[i+1])
  redis.call('HINCRBY', ledger, field, -amt)
  redis.call('HSET', res, 'hold:'..field, amt)
end
redis.call('HSET', res, 'id', id, 'run', ARGV[2], 'item', ARGV[3], 'env', ARGV[6], 'region', ARGV[7], 'space', ARGV[8], 'created', now, 'exp', now+ttl)
redis.call('EXPIRE', res, keyttl)
redis.call('ZADD', idx, now+ttl, id)
redis.call('ZADD', keysidx, now, res)
return 1
`

// releaseScript: KEYS = res, rescidx, keysidx, ledger; ARGV = id, env, region.
const releaseScript = `
local res=KEYS[1]; local idx=KEYS[2]; local keysidx=KEYS[3]; local ledger=KEYS[4]
local id=ARGV[1]
if redis.call('EXISTS', res)==0 then return 0 end
if redis.call('HGET', res,'env') ~= ARGV[2] or redis.call('HGET', res,'region') ~= ARGV[3] then
  return -1
end
local h=redis.call('HGETALL', res)
for i=1,#h,2 do
  local f=h[i]
  if string.sub(f,1,5)=='hold:' then
    redis.call('HINCRBY', ledger, string.sub(f,6), tonumber(h[i+1]))
  end
end
redis.call('DEL', res)
redis.call('ZREM', idx, id)
redis.call('ZREM', keysidx, res)
return 1
`

// rebuildScript: KEYS = ledger, cap, rescidx; ARGV[1]=res prefix, then (field, capacity) pairs.
const rebuildScript = `
local ledger=KEYS[1]; local cap=KEYS[2]; local idx=KEYS[3]
local prefix=ARGV[1]
local members=redis.call('ZRANGE', idx, 0, -1)
local used={}
for _,m in ipairs(members) do
  local k=prefix..m
  if redis.call('EXISTS', k)==1 then
    local h=redis.call('HGETALL', k)
    for i=1,#h,2 do
      local f=h[i]
      if string.sub(f,1,5)=='hold:' then
        local rk=string.sub(f,6)
        used[rk]=(used[rk] or 0)+tonumber(h[i+1])
      end
    end
  else
    redis.call('ZREM', idx, m)
  end
end
for i=2,#ARGV,2 do
  local field=ARGV[i]
  local capv=tonumber(ARGV[i+1])
  local u=used[field] or 0
  local av=capv-u
  if av<0 then av=0 end
  redis.call('HSET', ledger, field, av)
  redis.call('HSET', cap, field, capv)
end
return 1
`

// cleanupScript: KEYS = keysidx; ARGV[1] = namespace prefix.
const cleanupScript = `
local keysidx=KEYS[1]
local ns=ARGV[1]
local members=redis.call('ZRANGE', keysidx, 0, -1)
for _,k in ipairs(members) do
  if string.sub(k,1,#ns)==ns then
    redis.call('DEL', k)
  end
end
redis.call('DEL', keysidx)
redis.call('DEL', ns..':ledger', ns..':cap', ns..':residx', ns..':runs', ns..':runidx', ns..':runexp', ns..':prio', ns..':prioexp', ns..':rebuild:lock')
return 1
`
