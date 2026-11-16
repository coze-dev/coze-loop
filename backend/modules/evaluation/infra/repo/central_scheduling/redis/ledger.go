// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	loopredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// Ledger 中心调度 Redis 账本。所有 key 使用 qn:central:{env} hash tag，保证同 env 同 slot。
type Ledger struct {
	cmd loopredis.Cmdable
}

func NewLedger(cmd loopredis.Cmdable) *Ledger { return &Ledger{cmd: cmd} }

func normalizeEnv(env string) string {
	env = strings.TrimSpace(env)
	env = strings.ReplaceAll(env, "}", "_")
	env = strings.ReplaceAll(env, "{", "_")
	if env == "" {
		env = "unknown"
	}
	return env
}

func (l *Ledger) Key(env, suffix string) string {
	return "qn:central:{" + normalizeEnv(env) + "}:" + suffix
}

func (l *Ledger) poolKey(env string) string     { return l.Key(env, "pool") }
func (l *Ledger) indexKey(env string) string    { return l.Key(env, "resv:index") }
func (l *Ledger) resvKey(env, id string) string { return l.Key(env, "resv:"+id) }
func (l *Ledger) runKey(env string, exptID, runID int64) string {
	return l.Key(env, fmt.Sprintf("run:%d:%d", exptID, runID))
}

const reserveScript = `
-- KEYS[1]=pool KEYS[2]=resv KEYS[3]=runIndex KEYS[4]=index
-- ARGV: resvID env spaceID exptID runID itemID attempt state owner now ttl dimCount resvPrefix
--       then for each dim: dim need enabled capacity
local resvID = ARGV[1]
local env = ARGV[2]
local spaceID = ARGV[3]
local exptID = ARGV[4]
local runID = ARGV[5]
local itemID = ARGV[6]
local attempt = ARGV[7]
local state = ARGV[8]
local owner = ARGV[9]
local now = ARGV[10]
local ttl = tonumber(ARGV[11])
local dimCount = tonumber(ARGV[12])
local resvPrefix = ARGV[13]
-- 同一 item 已有非终态预占时幂等返回，避免重复扣减容量。
local exists = redis.call('HGET', KEYS[3], itemID)
if exists then
  local st = redis.call('HGET', resvPrefix .. exists, 'state')
  if st and st ~= 'RELEASED' and st ~= 'EXPIRED' then
    return {0, 'EXISTS', exists}
  end
end
local idx = 14
for i = 1, dimCount do
  local dim = ARGV[idx]
  local need = tonumber(ARGV[idx+1])
  local enabled = tonumber(ARGV[idx+2])
  local cap = tonumber(ARGV[idx+3])
  idx = idx + 4
  if enabled == 1 then
    if cap < 0 then return {0, "UNKNOWN", dim} end
    local used = tonumber(redis.call('HGET', KEYS[1], 'used:'..dim) or '0')
    if used + need > cap then return {0, "INSUFFICIENT", dim} end
  end
end
idx = 14
for i = 1, dimCount do
  local dim = ARGV[idx]
  local need = tonumber(ARGV[idx+1])
  local enabled = tonumber(ARGV[idx+2])
  idx = idx + 4
  if enabled == 1 then
    redis.call('HINCRBY', KEYS[1], 'used:'..dim, need)
  end
  redis.call('HSET', KEYS[2], 'enabled:'..dim, enabled, 'need:'..dim, need)
end
redis.call('HSET', KEYS[2], 'id', resvID, 'env', env, 'space_id', spaceID, 'expt_id', exptID, 'run_id', runID,
  'item_id', itemID, 'attempt', attempt, 'state', state, 'owner', owner,
  'created_at', now, 'updated_at', now)
redis.call('HSET', KEYS[3], itemID, resvID)
redis.call('HSET', KEYS[4], resvID, now)
redis.call('EXPIRE', KEYS[1], ttl)
redis.call('EXPIRE', KEYS[2], ttl)
redis.call('EXPIRE', KEYS[3], ttl)
redis.call('EXPIRE', KEYS[4], ttl)
return {1, resvID}
`

const adoptScript = `
-- KEYS[1]=pool KEYS[2]=resv KEYS[3]=runIndex KEYS[4]=index
-- ARGV: resvID env spaceID exptID runID itemID attempt now ttl dimCount then dim need enabled
local resvID = ARGV[1]
local env = ARGV[2]
local spaceID = ARGV[3]
local exptID = ARGV[4]
local runID = ARGV[5]
local itemID = ARGV[6]
local attempt = ARGV[7]
local now = ARGV[8]
local ttl = tonumber(ARGV[9])
local dimCount = tonumber(ARGV[10])
local existing = redis.call('HGET', KEYS[2], 'id')
if existing == resvID and redis.call('HGET', KEYS[2], 'state') ~= 'RELEASED' and redis.call('HGET', KEYS[2], 'state') ~= 'EXPIRED' then
  return {1, 'ALREADY'}
end
local idx = 11
for i = 1, dimCount do
  local dim = ARGV[idx]
  local need = tonumber(ARGV[idx+1])
  local enabled = tonumber(ARGV[idx+2])
  idx = idx + 3
  if enabled == 1 then redis.call('HINCRBY', KEYS[1], 'used:'..dim, need) end
  redis.call('HSET', KEYS[2], 'enabled:'..dim, enabled, 'need:'..dim, need)
end
redis.call('HSET', KEYS[2], 'id', resvID, 'env', env, 'space_id', spaceID, 'expt_id', exptID, 'run_id', runID,
  'item_id', itemID, 'attempt', attempt, 'state', 'CONFIRMED', 'owner', 'rebuild',
  'created_at', now, 'updated_at', now, 'confirmed_at', now, 'adopted', '1')
redis.call('HSET', KEYS[3], itemID, resvID)
redis.call('HSET', KEYS[4], resvID, now)
redis.call('EXPIRE', KEYS[1], ttl); redis.call('EXPIRE', KEYS[2], ttl)
redis.call('EXPIRE', KEYS[3], ttl); redis.call('EXPIRE', KEYS[4], ttl)
return {1, 'ADOPTED'}
`

const confirmScript = `
-- KEYS[1]=resv ARGV[1]=now
local st = redis.call('HGET', KEYS[1], 'state')
if not st then return {0, 'UNKNOWN'} end
if st == 'RELEASED' or st == 'EXPIRED' then return {0, st} end
if st == 'CONFIRMED' then return {1, st} end
redis.call('HSET', KEYS[1], 'state', 'CONFIRMED', 'confirmed_at', ARGV[1], 'updated_at', ARGV[1])
return {1, 'CONFIRMED'}
`

const pendingScript = `
-- KEYS[1]=resv ARGV[1]=now
local st = redis.call('HGET', KEYS[1], 'state')
if not st then return {0, 'UNKNOWN'} end
if st == 'RELEASED' or st == 'EXPIRED' then return {0, st} end
if st == 'CONFIRMED' then return {1, st} end
redis.call('HSET', KEYS[1], 'state', 'PENDING_COMPENSATION', 'updated_at', ARGV[1])
return {1, 'PENDING_COMPENSATION'}
`

const releaseScript = `
-- KEYS[1]=pool KEYS[2]=resv KEYS[3]=runIndex KEYS[4]=index
-- ARGV: env targetState reason now dimCount then dims...
local target = ARGV[2]
local reason = ARGV[3]
local now = ARGV[4]
local dimCount = tonumber(ARGV[5])
local idx = 6
local st = redis.call('HGET', KEYS[2], 'state')
if not st then return {0, 'UNKNOWN'} end
if st == 'RELEASED' or st == 'EXPIRED' then return {0, st} end
for i = 1, dimCount do
  local dim = ARGV[idx]; idx = idx + 1
  local enabled = tonumber(redis.call('HGET', KEYS[2], 'enabled:'..dim) or '0')
  local need = tonumber(redis.call('HGET', KEYS[2], 'need:'..dim) or '0')
  if enabled == 1 and need > 0 then
    local used = redis.call('HINCRBY', KEYS[1], 'used:'..dim, -need)
    if used < 0 then redis.call('HSET', KEYS[1], 'used:'..dim, 0) end
  end
end
redis.call('HSET', KEYS[2], 'state', target, 'reason', reason, 'released_at', now, 'updated_at', now)
local itemID = redis.call('HGET', KEYS[2], 'item_id')
local resvID = redis.call('HGET', KEYS[2], 'id')
if itemID and redis.call('HGET', KEYS[3], itemID) == resvID then
  redis.call('HDEL', KEYS[3], itemID)
end
redis.call('HSET', KEYS[4], resvID, now)
return {1, target}
`

func (l *Ledger) Reserve(ctx context.Context, req *repo.ReserveRequest) (*entity.Reservation, error) {
	if req == nil || req.Reservation == nil {
		return nil, errors.New("nil reserve request")
	}
	r := req.Reservation
	ttl := int64(req.TTL / time.Second)
	if ttl <= 0 {
		ttl = int64((72 * time.Hour) / time.Second)
	}
	dims := make([]entity.ResourceDimension, 0, len(entity.AllResourceDimensions))
	for _, dim := range entity.AllResourceDimensions {
		if _, ok := r.Vector[dim]; ok {
			dims = append(dims, dim)
		}
	}
	args := []any{r.ID, r.Env, r.SpaceID, r.ExptID, r.RunID, r.ItemID, r.Attempt, string(entity.ReservationStateReserved), r.Owner, r.CreatedAt, ttl, len(dims), l.Key(r.Env, "resv:")}
	for _, dim := range dims {
		enabled := 0
		if req.Enabled[dim] {
			enabled = 1
		}
		cap := int64(-1)
		if c, ok := req.Capacity[dim]; ok {
			cap = c
		} else if !req.Enabled[dim] {
			cap = 0
		}
		args = append(args, string(dim), r.Vector[dim], enabled, cap)
	}
	res, err := l.cmd.Eval(ctx, reserveScript, l.keys(r.Env, r), args...).Result()
	if err != nil {
		return nil, err
	}
	reply, ok := res.([]any)
	if !ok || len(reply) < 2 {
		return nil, fmt.Errorf("unexpected reserve reply: %v", res)
	}
	if code, _ := reply[0].(int64); code != 1 {
		reason, _ := reply[1].(string)
		if reason == "EXISTS" {
			return nil, repo.ErrReservationExists
		}
		detail := ""
		if len(reply) > 2 {
			detail, _ = reply[2].(string)
		}
		return nil, fmt.Errorf("central reserve failed: %s %s", reason, detail)
	}
	r.State = entity.ReservationStateReserved
	r.UpdatedAt = r.CreatedAt
	return r, nil
}

func (l *Ledger) Adopt(ctx context.Context, r *entity.Reservation, ttl time.Duration) error {
	if r == nil {
		return errors.New("nil reservation")
	}
	sec := int64(ttl / time.Second)
	if sec <= 0 {
		sec = int64((72 * time.Hour) / time.Second)
	}
	dims := make([]entity.ResourceDimension, 0)
	for _, dim := range entity.AllResourceDimensions {
		if _, ok := r.Vector[dim]; ok {
			dims = append(dims, dim)
		}
	}
	args := []any{r.ID, r.Env, r.SpaceID, r.ExptID, r.RunID, r.ItemID, r.Attempt, r.CreatedAt, sec, len(dims)}
	for _, dim := range dims {
		args = append(args, string(dim), r.Vector[dim], 1)
	}
	_, err := l.cmd.Eval(ctx, adoptScript, l.keys(r.Env, r), args...).Result()
	return err
}

func (l *Ledger) keys(env string, r *entity.Reservation) []string {
	return []string{l.poolKey(env), l.resvKey(env, r.ID), l.runKey(env, r.ExptID, r.RunID), l.indexKey(env)}
}

func (l *Ledger) Confirm(ctx context.Context, env, resvID string) (entity.ReservationState, error) {
	res, err := l.cmd.Eval(ctx, confirmScript, []string{l.resvKey(env, resvID)}, entity.NowUnixMS()).Result()
	if err != nil {
		return "", err
	}
	return parseStateReply(res)
}

func (l *Ledger) MarkPendingCompensation(ctx context.Context, env, resvID string) (entity.ReservationState, error) {
	res, err := l.cmd.Eval(ctx, pendingScript, []string{l.resvKey(env, resvID)}, entity.NowUnixMS()).Result()
	if err != nil {
		return "", err
	}
	return parseStateReply(res)
}

func (l *Ledger) Release(ctx context.Context, env, resvID, reason string) (entity.ReservationState, error) {
	return l.release(ctx, env, resvID, entity.ReservationStateReleased, reason)
}

func (l *Ledger) Expire(ctx context.Context, env, resvID, reason string) (entity.ReservationState, error) {
	return l.release(ctx, env, resvID, entity.ReservationStateExpired, reason)
}

func (l *Ledger) release(ctx context.Context, env, resvID string, target entity.ReservationState, reason string) (entity.ReservationState, error) {
	r, err := l.Get(ctx, env, resvID)
	if err != nil {
		return "", err
	}
	if r == nil {
		return "", nil
	}
	dims := make([]any, 0, len(entity.AllResourceDimensions)+1)
	dims = append(dims, env, string(target), reason, entity.NowUnixMS(), len(entity.AllResourceDimensions))
	for _, dim := range entity.AllResourceDimensions {
		dims = append(dims, string(dim))
	}
	res, err := l.cmd.Eval(ctx, releaseScript, l.keys(env, r), dims...).Result()
	if err != nil {
		return "", err
	}
	return parseStateReply(res)
}

func parseStateReply(res any) (entity.ReservationState, error) {
	reply, ok := res.([]any)
	if !ok || len(reply) < 2 {
		return "", fmt.Errorf("unexpected ledger reply: %v", res)
	}
	state, _ := reply[1].(string)
	return entity.ReservationState(state), nil
}

func (l *Ledger) Get(ctx context.Context, env, resvID string) (*entity.Reservation, error) {
	raw, err := l.cmd.HGetAll(ctx, l.resvKey(env, resvID)).Result()
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	return reservationFromHash(raw)
}

func reservationFromHash(m map[string]string) (*entity.Reservation, error) {
	r := &entity.Reservation{ID: m["id"], Env: m["env"], State: entity.ReservationState(m["state"])}
	r.SpaceID, _ = strconv.ParseInt(m["space_id"], 10, 64)
	r.ExptID, _ = strconv.ParseInt(m["expt_id"], 10, 64)
	r.RunID, _ = strconv.ParseInt(m["run_id"], 10, 64)
	r.ItemID, _ = strconv.ParseInt(m["item_id"], 10, 64)
	r.Attempt, _ = strconv.ParseInt(m["attempt"], 10, 64)
	r.CreatedAt, _ = strconv.ParseInt(m["created_at"], 10, 64)
	r.UpdatedAt, _ = strconv.ParseInt(m["updated_at"], 10, 64)
	r.ConfirmedAt, _ = strconv.ParseInt(m["confirmed_at"], 10, 64)
	r.ReleasedAt, _ = strconv.ParseInt(m["released_at"], 10, 64)
	r.Owner = m["owner"]
	r.Reason = m["reason"]
	r.Adopted = m["adopted"] == "1"
	r.Vector = entity.ResourceVector{}
	for _, dim := range entity.AllResourceDimensions {
		if v, ok := m["need:"+string(dim)]; ok {
			n, _ := strconv.ParseInt(v, 10, 64)
			r.Vector[dim] = n
		}
	}
	return r, nil
}

func (l *Ledger) ListByRun(ctx context.Context, env string, exptID, runID int64) ([]*entity.Reservation, error) {
	m, err := l.cmd.HGetAll(ctx, l.runKey(env, exptID, runID)).Result()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	out := make([]*entity.Reservation, 0, len(m))
	for _, resvID := range m {
		r, gerr := l.Get(ctx, env, resvID)
		if gerr != nil {
			return nil, gerr
		}
		if r != nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (l *Ledger) ListAll(ctx context.Context, env string, limit int) ([]*entity.Reservation, error) {
	m, err := l.cmd.HGetAll(ctx, l.indexKey(env)).Result()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	out := make([]*entity.Reservation, 0, len(ids))
	for _, id := range ids {
		r, gerr := l.Get(ctx, env, id)
		if gerr != nil {
			return nil, gerr
		}
		if r != nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (l *Ledger) RunInflight(ctx context.Context, env string, exptID, runID int64) (int, error) {
	n, err := l.cmd.HLen(ctx, l.runKey(env, exptID, runID)).Result()
	return int(n), err
}

func (l *Ledger) PoolUsage(ctx context.Context, env string) (entity.ResourceVector, error) {
	m, err := l.cmd.HGetAll(ctx, l.poolKey(env)).Result()
	if err != nil {
		return nil, err
	}
	out := entity.ResourceVector{}
	for _, dim := range entity.AllResourceDimensions {
		n, _ := strconv.ParseInt(m["used:"+string(dim)], 10, 64)
		out[dim] = n
	}
	return out, nil
}

func (l *Ledger) RebuildPool(ctx context.Context, env string, used entity.ResourceVector, ttl time.Duration) error {
	key := l.poolKey(env)
	args := []any{}
	for _, dim := range entity.AllResourceDimensions {
		args = append(args, "used:"+string(dim), used.Get(dim))
	}
	if len(args) > 0 {
		if err := l.cmd.HSet(ctx, key, args...).Err(); err != nil {
			return err
		}
	}
	return l.cmd.Expire(ctx, key, ttl).Err()
}

func (l *Ledger) CleanupEnv(ctx context.Context, env string) error {
	// 只清理本次 env 命名空间；绝不 FLUSH，也不碰其他前缀。
	ids, err := l.cmd.HKeys(ctx, l.indexKey(env)).Result()
	if err != nil {
		return err
	}
	keys := []string{l.poolKey(env), l.indexKey(env), l.Key(env, "active"), l.Key(env, "leader")}
	for _, id := range ids {
		r, gerr := l.Get(ctx, env, id)
		if gerr == nil && r != nil {
			keys = append(keys, l.resvKey(env, id), l.runKey(env, r.ExptID, r.RunID))
		}
	}
	if len(keys) > 0 {
		return l.cmd.Del(ctx, keys...).Err()
	}
	return nil
}
