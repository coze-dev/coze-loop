// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// Pool counters must outlive every reservation that contributes to them. If a
// pool key expired while reservations were still live, a later reader would
// see zero usage and over-admit. This is the one deliberate long TTL; it is
// still a TTL, refreshed on every mutation.
const centralPoolTTL = 30 * 24 * time.Hour

// run metadata outlives reservations so reconciliation can see released
// counters for a while after a run finishes.
const centralRunMetaTTL = 24 * time.Hour

func NewCentralReservationStore(cmdable redis.Cmdable) repo.CentralReservationStore {
	return &centralReservationStore{cmdable: cmdable}
}

type centralReservationStore struct {
	cmdable redis.Cmdable
}

func centralPoolKey(env, dimension string) string {
	return entity.CentralKeyPrefix + env + ":pool:" + dimension
}

func centralReservationKey(env, id string) string {
	return entity.CentralKeyPrefix + env + ":resv:" + id
}

func centralRunReservationsKey(env string, exptID, runID int64) string {
	return entity.CentralKeyPrefix + env + ":run:" + strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10) + ":resv"
}

func centralRunLedgerKey(env string, exptID, runID int64) string {
	return entity.CentralKeyPrefix + env + ":run:" + strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10) + ":ledger"
}

// reserveScript is the heart of the feature: check every enforced dimension
// and mutate only when all of them fit. KEYS[1]=reservation, KEYS[2]=run set,
// KEYS[3]=ledger, KEYS[4..]=pool counters.
//
// Fixed ARGV: 1=resvID 2=env 3=exptID 4=spaceID 5=runID 6=itemID
// 7=claimsJSON 8=adopted 9=ttlSeconds 10=runMetaTTL 11=state 12=createdAt
// 13=comma-separated enforced dimensions.
// Then per pool key: capacity, claim, dimensionName.
//
// Returns {1,"",0} granted, {2,"",0} already held, {0,dim,remaining} denied.
const reserveScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
  return {2, '', 0}
end
local n = #KEYS - 3
for i = 1, n do
  local base = 13 + (i - 1) * 3
  local cap = tonumber(ARGV[base + 1])
  local need = tonumber(ARGV[base + 2])
  local dim = ARGV[base + 3]
  local used = tonumber(redis.call('GET', KEYS[3 + i]) or '0')
  if cap ~= nil and need ~= nil and used + need > cap then
    return {0, dim, cap - used}
  end
end
for i = 1, n do
  local base = 13 + (i - 1) * 3
  local need = tonumber(ARGV[base + 2])
  if need ~= nil and need ~= 0 then
    redis.call('INCRBY', KEYS[3 + i], need)
    redis.call('EXPIRE', KEYS[3 + i], 2592000)
  end
end
redis.call('HSET', KEYS[1],
  'id', ARGV[1], 'env', ARGV[2], 'expt_id', ARGV[3], 'space_id', ARGV[4],
  'run_id', ARGV[5], 'item_id', ARGV[6], 'claims', ARGV[7],
  'adopted', ARGV[8], 'state', ARGV[11], 'pool_keys', ARGV[13],
  'created_at', ARGV[12], 'updated_at', ARGV[12])
redis.call('EXPIRE', KEYS[1], ARGV[9])
redis.call('HSET', KEYS[2], ARGV[1], ARGV[6])
redis.call('EXPIRE', KEYS[2], ARGV[10])
redis.call('HINCRBY', KEYS[3], 'held', 1)
redis.call('HINCRBY', KEYS[3], 'version', 1)
redis.call('EXPIRE', KEYS[3], ARGV[10])
return {1, '', 0}
`

func (s *centralReservationStore) Reserve(ctx context.Context, req *entity.CentralReserveRequest) (*entity.CentralReserveResult, error) {
	if req == nil || req.Claims == nil {
		return nil, errorx.New("central reserve: nil request")
	}
	if req.TTL <= 0 {
		req.TTL = entity.CentralDefaultReservationTTL
	}
	keys := entity.CentralPoolKeys(req.Claims, req.Capacities)
	redisKeys := make([]string, 0, 3+len(keys))
	resvKey := centralReservationKey(req.Env, req.ReservationID)
	runKey := centralRunReservationsKey(req.Env, req.ExptID, req.RunID)
	ledgerKey := centralRunLedgerKey(req.Env, req.ExptID, req.RunID)
	redisKeys = append(redisKeys, resvKey, runKey, ledgerKey)
	for _, k := range keys {
		redisKeys = append(redisKeys, centralPoolKey(req.Env, k))
	}

	claimsJSON, err := json.Marshal(req.Claims)
	if err != nil {
		return nil, errorx.Wrapf(err, "central reserve: marshal claims")
	}
	adopted := "0"
	if req.Adopted {
		adopted = "1"
	}
	args := []interface{}{
		req.ReservationID, req.Env, req.ExptID, req.SpaceID, req.RunID, req.ItemID,
		string(claimsJSON), adopted, int64(req.TTL / time.Second), int64(centralRunMetaTTL / time.Second),
		string(entity.CentralReservationHeld), time.Now().Unix(), joinKeys(keys),
	}
	for _, k := range keys {
		args = append(args, req.Capacities[k], req.Claims[k], k)
	}

	got, err := s.cmdable.Eval(ctx, reserveScript, redisKeys, args...).Result()
	if err != nil {
		return nil, errorx.Wrapf(err, "central reserve: redis eval")
	}
	values, ok := got.([]interface{})
	if !ok || len(values) != 3 {
		return nil, errorx.New("central reserve: unexpected redis reply %T", got)
	}
	code, _ := values[0].(int64)
	res := &entity.CentralReserveResult{ReservationID: req.ReservationID}
	switch code {
	case 1:
		res.Granted = true
	case 2:
		res.Granted = true
		res.AlreadyHeld = true
	default:
		res.Granted = false
		res.ConstrainedBy = toString(values[1])
		if rem, ok := values[2].(int64); ok {
			res.Remaining = map[string]int64{res.ConstrainedBy: rem}
		}
	}
	return res, nil
}

// releaseScript decrements only when the reservation exists, clamps at zero,
// and is therefore safe to call twice.
const releaseScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then
  return 0
end
local n = #KEYS - 3
for i = 1, n do
  local base = 3 + (i - 1) * 2
  local need = tonumber(ARGV[base + 1])
  if need ~= nil and need ~= 0 then
    local used = tonumber(redis.call('GET', KEYS[3 + i]) or '0')
    local next = used - need
    if next < 0 then next = 0 end
    redis.call('SET', KEYS[3 + i], next, 'EX', 2592000)
  end
end
redis.call('DEL', KEYS[1])
redis.call('HDEL', KEYS[2], ARGV[3])
redis.call('HINCRBY', KEYS[3], 'released', 1)
local held = tonumber(redis.call('HGET', KEYS[3], 'held') or '0')
if held > 0 then
  redis.call('HINCRBY', KEYS[3], 'held', -1)
end
redis.call('HINCRBY', KEYS[3], 'version', 1)
redis.call('EXPIRE', KEYS[3], ARGV[2])
return 1
`

func (s *centralReservationStore) Release(ctx context.Context, env, reservationID string) (bool, error) {
	if reservationID == "" {
		return false, nil
	}
	resv, err := s.getReservation(ctx, env, reservationID)
	if err != nil {
		return false, err
	}
	if resv == nil {
		return false, nil
	}
	keys := resv.PoolKeys
	if len(keys) == 0 {
		// Legacy reservations written before pool_keys existed: fall back to
		// every claim dimension, accepting a clamped no-op for unconstrained
		// dimensions.
		keys = entity.CentralPoolKeys(resv.Claims, mapStringInt64ToCaps(resv.Claims))
	}
	redisKeys := []string{
		centralReservationKey(env, reservationID),
		centralRunReservationsKey(env, resv.ExptID, resv.RunID),
		centralRunLedgerKey(env, resv.ExptID, resv.RunID),
	}
	for _, k := range keys {
		redisKeys = append(redisKeys, centralPoolKey(env, k))
	}
	args := []interface{}{reservationID, int64(centralRunMetaTTL / time.Second), reservationID}
	for _, k := range keys {
		args = append(args, resv.Claims[k], k)
	}
	got, err := s.cmdable.Eval(ctx, releaseScript, redisKeys, args...).Int64()
	if err != nil {
		return false, errorx.Wrapf(err, "central release: redis eval")
	}
	return got == 1, nil
}

// mapStringInt64ToCaps makes every claim dimension "enforced" so the release
// script visits exactly the counters that were incremented at reserve time.
func mapStringInt64ToCaps(claims entity.CentralResourceClaims) map[string]int64 {
	out := make(map[string]int64, len(claims))
	for k := range claims {
		out[k] = 0
	}
	return out
}

func (s *centralReservationStore) getReservation(ctx context.Context, env, id string) (*entity.CentralReservation, error) {
	key := centralReservationKey(env, id)
	fields, err := s.cmdable.HGetAll(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil
		}
		return nil, errorx.Wrapf(err, "central reservation: hgetall %s", key)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	return reservationFromFields(fields)
}

func (s *centralReservationStore) List(ctx context.Context, env string, exptID, runID int64) ([]*entity.CentralReservation, error) {
	runKey := centralRunReservationsKey(env, exptID, runID)
	members, err := s.cmdable.HGetAll(ctx, runKey).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil
		}
		return nil, errorx.Wrapf(err, "central reservation: list %s", runKey)
	}
	out := make([]*entity.CentralReservation, 0, len(members))
	for id := range members {
		resv, rerr := s.getReservation(ctx, env, id)
		if rerr != nil {
			return nil, rerr
		}
		if resv == nil {
			// Reservation expired before the run index; skip. The reconcile
			// path will treat the item as unreserved and re-adopt it.
			continue
		}
		out = append(out, resv)
	}
	return out, nil
}

func (s *centralReservationStore) ReleaseRun(ctx context.Context, env string, exptID, runID int64) (int, error) {
	reservations, err := s.List(ctx, env, exptID, runID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, resv := range reservations {
		released, rerr := s.Release(ctx, env, resv.ID)
		if rerr != nil {
			return n, rerr
		}
		if released {
			n++
		}
	}
	// The run index can still contain expired members; delete it so a finished
	// run leaves no enumerable state behind.
	if err := s.cmdable.Del(ctx, centralRunReservationsKey(env, exptID, runID)).Err(); err != nil {
		return n, errorx.Wrapf(err, "central reservation: delete run index")
	}
	return n, nil
}

func (s *centralReservationStore) MarkDispatched(ctx context.Context, env, reservationID string) error {
	resv, err := s.getReservation(ctx, env, reservationID)
	if err != nil {
		return err
	}
	if resv == nil {
		return nil
	}
	key := centralReservationKey(env, reservationID)
	if resv.State == entity.CentralReservationDispatched {
		return nil
	}
	if err := s.cmdable.HSet(ctx, key, "state", string(entity.CentralReservationDispatched), "updated_at", time.Now().Unix()).Err(); err != nil {
		return errorx.Wrapf(err, "central reservation: mark dispatched %s", key)
	}
	ledgerKey := centralRunLedgerKey(env, resv.ExptID, resv.RunID)
	if err := s.cmdable.HIncrBy(ctx, ledgerKey, "dispatched", 1).Err(); err != nil {
		return errorx.Wrapf(err, "central reservation: dispatched counter")
	}
	_ = s.cmdable.Expire(ctx, ledgerKey, centralRunMetaTTL).Err()
	return nil
}

func (s *centralReservationStore) RenewRun(ctx context.Context, env string, exptID, runID int64, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = entity.CentralDefaultReservationTTL
	}
	reservations, err := s.List(ctx, env, exptID, runID)
	if err != nil {
		return err
	}
	for _, resv := range reservations {
		key := centralReservationKey(env, resv.ID)
		if err := s.cmdable.Expire(ctx, key, ttl).Err(); err != nil {
			return errorx.Wrapf(err, "central reservation: renew %s", key)
		}
		_ = s.cmdable.HSet(ctx, key, "updated_at", time.Now().Unix()).Err()
	}
	runKey := centralRunReservationsKey(env, exptID, runID)
	if err := s.cmdable.Expire(ctx, runKey, ttl).Err(); err != nil {
		return errorx.Wrapf(err, "central reservation: renew run index")
	}
	ledgerKey := centralRunLedgerKey(env, exptID, runID)
	if err := s.cmdable.Expire(ctx, ledgerKey, ttl).Err(); err != nil {
		return errorx.Wrapf(err, "central reservation: renew ledger")
	}
	return nil
}

func (s *centralReservationStore) Ledger(ctx context.Context, env string, exptID, runID int64) (*entity.CentralRunLedger, error) {
	key := centralRunLedgerKey(env, exptID, runID)
	fields, err := s.cmdable.HGetAll(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return &entity.CentralRunLedger{}, nil
		}
		return nil, errorx.Wrapf(err, "central reservation: ledger %s", key)
	}
	return ledgerFromFields(fields), nil
}

func reservationFromFields(fields map[string]string) (*entity.CentralReservation, error) {
	resv := &entity.CentralReservation{
		ID:    fields["id"],
		Env:   fields["env"],
		State: entity.CentralReservationState(fields["state"]),
	}
	resv.ExptID, _ = strconv.ParseInt(fields["expt_id"], 10, 64)
	resv.SpaceID, _ = strconv.ParseInt(fields["space_id"], 10, 64)
	resv.RunID, _ = strconv.ParseInt(fields["run_id"], 10, 64)
	resv.ItemID, _ = strconv.ParseInt(fields["item_id"], 10, 64)
	resv.Adopted = fields["adopted"] == "1"
	resv.CreatedAtUnix, _ = strconv.ParseInt(fields["created_at"], 10, 64)
	resv.UpdatedAtUnix, _ = strconv.ParseInt(fields["updated_at"], 10, 64)
	if raw := fields["claims"]; raw != "" {
		var claims entity.CentralResourceClaims
		if err := json.Unmarshal([]byte(raw), &claims); err != nil {
			return nil, errorx.Wrapf(err, "central reservation: decode claims")
		}
		resv.Claims = claims
	}
	if raw := fields["pool_keys"]; raw != "" {
		resv.PoolKeys = splitKeys(raw)
	}
	return resv, nil
}

func joinKeys(keys []string) string {
	return strings.Join(keys, ",")
}

func splitKeys(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ledgerFromFields(fields map[string]string) *entity.CentralRunLedger {
	out := &entity.CentralRunLedger{}
	out.Held = atoi(fields["held"])
	out.Dispatched = atoi(fields["dispatched"])
	out.Released = atoi(fields["released"])
	out.Debt = atoi(fields["debt"])
	out.Version, _ = strconv.ParseInt(fields["version"], 10, 64)
	return out
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(v)
	}
}
