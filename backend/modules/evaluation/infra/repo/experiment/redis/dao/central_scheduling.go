// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	redislib "github.com/redis/go-redis/v9"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// CentralKeyPrefix 是本题允许读写的唯一 Redis key 前缀。
// 线上与 PPE 共用 Redis，因此所有 key 必须在该前缀之下再按 Env/Lane 隔离。
const CentralKeyPrefix = "qn:central:"

// 预占/释放脚本。所有资源检查与写入在同一个 Lua 脚本内完成，
// 保证“多资源全有或全无”，不会出现只占到一半的中间态。
const centralAcquireScript = `
local usageKey = KEYS[1]
local leaseKey = KEYS[2]
local indexKey = KEYS[3]
local ttl = tonumber(ARGV[1])
-- 同一 lease 已存在时视为已预占成功，避免“派发成功但状态更新失败”导致的重试双扣。
if redis.call('EXISTS', leaseKey) == 1 then
  return 1
end
local n = tonumber(ARGV[2])
local claims = {}
for i = 0, n - 1 do
  local kind = ARGV[3 + i * 2]
  local amount = tonumber(ARGV[4 + i * 2])
  local capacity = tonumber(ARGV[3 + n * 2 + i])
  local used = tonumber(redis.call('HGET', usageKey, kind) or '0')
  if used + amount > capacity then
    return 0
  end
  claims[#claims + 1] = {kind = kind, amount = amount}
end
for i = 1, #claims do
  redis.call('HINCRBY', usageKey, claims[i]['kind'], claims[i]['amount'])
end
local metaBase = 3 + n * 3
redis.call('HSET', leaseKey,
  'lease_id', ARGV[metaBase],
  'space_id', ARGV[metaBase + 1],
  'expt_id', ARGV[metaBase + 2],
  'expt_run_id', ARGV[metaBase + 3],
  'item_id', ARGV[metaBase + 4],
  'acquired_at', ARGV[metaBase + 5],
  'expire_at', ARGV[metaBase + 6])
for i = 1, #claims do
  redis.call('HSET', leaseKey, 'claim:' .. claims[i]['kind'], claims[i]['amount'])
end
redis.call('EXPIRE', leaseKey, ttl)
redis.call('ZADD', indexKey, tonumber(ARGV[metaBase + 6]), ARGV[metaBase])
redis.call('EXPIRE', indexKey, ttl + 86400)
redis.call('EXPIRE', usageKey, ttl + 86400)
return 1
`

const centralReleaseScript = `
local usageKey = KEYS[1]
local leaseKey = KEYS[2]
local indexKey = KEYS[3]
local leaseID = ARGV[1]
if redis.call('EXISTS', leaseKey) == 0 then
  redis.call('ZREM', indexKey, leaseID)
  return 0
end
local fields = redis.call('HGETALL', leaseKey)
for i = 1, #fields, 2 do
  local field = fields[i]
  local value = fields[i + 1]
  if string.sub(field, 1, 6) == 'claim:' then
    local kind = string.sub(field, 7)
    local amount = tonumber(value)
    local used = tonumber(redis.call('HGET', usageKey, kind) or '0')
    if used - amount < 0 then
      amount = used
    end
    redis.call('HINCRBY', usageKey, kind, -amount)
  end
end
redis.call('DEL', leaseKey)
redis.call('ZREM', indexKey, leaseID)
return 1
`

const centralUnregisterActiveScript = `
if redis.call('ZREM', KEYS[1], ARGV[1]) > 0 then
  return 1
end
return 0
`

// CentralSchedulingDAO 中心调度 Redis 账本实现。
type CentralSchedulingDAO struct {
	cmdable redis.Cmdable
}

var _ repo.ICentralSchedulingRepo = (*CentralSchedulingDAO)(nil)

func NewCentralSchedulingDAO(cmdable redis.Cmdable) *CentralSchedulingDAO {
	return &CentralSchedulingDAO{cmdable: cmdable}
}

func centralKey(scope entity.CentralEnvScope, suffix string) string {
	s := scope.Sanitize()
	return fmt.Sprintf("%s%s:%s:%s", CentralKeyPrefix, s.Env, s.Lane, suffix)
}

// Keys 暴露 key 构造，便于测试与审计；返回值一定以 qn:central: 开头。
func (d *CentralSchedulingDAO) Keys(scope entity.CentralEnvScope) (usage, leaseIndex, active string, leaseKey func(string) string) {
	usage = centralKey(scope, "usage")
	leaseIndex = centralKey(scope, "lease_index")
	active = centralKey(scope, "active")
	leaseKey = func(leaseID string) string { return centralKey(scope, "lease:"+leaseID) }
	return
}

func (d *CentralSchedulingDAO) Acquire(ctx context.Context, scope entity.CentralEnvScope, capacities map[entity.CentralResourceKind]int64, lease *entity.CentralLease, ttl time.Duration) (bool, error) {
	if lease == nil || lease.LeaseID == "" {
		return false, errorx.New("central acquire: lease id is required")
	}
	if len(lease.Claims) == 0 {
		return false, errorx.New("central acquire: claims are required")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	usageKey := centralKey(scope, "usage")
	leaseKey := centralKey(scope, "lease:"+lease.LeaseID)
	indexKey := centralKey(scope, "lease_index")

	args := make([]interface{}, 0, 2+len(lease.Claims)*2+len(lease.Claims)+8)
	args = append(args, int64(ttl/time.Second))
	args = append(args, len(lease.Claims))
	for _, claim := range lease.Claims {
		args = append(args, string(claim.Kind), claim.Amount)
	}
	for _, claim := range lease.Claims {
		cap := capacities[claim.Kind]
		if cap <= 0 {
			return false, errorx.New("central acquire: capacity of %s is not configured", claim.Kind)
		}
		args = append(args, cap)
	}
	if lease.ExpireAt <= 0 {
		lease.ExpireAt = time.Now().Add(ttl).Unix()
	}
	args = append(args,
		lease.LeaseID,
		lease.SpaceID,
		lease.ExptID,
		lease.ExptRunID,
		lease.ItemID,
		lease.AcquiredAt,
		lease.ExpireAt,
	)
	res, err := d.cmdable.Eval(ctx, centralAcquireScript, []string{usageKey, leaseKey, indexKey}, args...).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central acquire: eval failed, lease=%s", lease.LeaseID)
	}
	return toInt64(res) == 1, nil
}

func (d *CentralSchedulingDAO) Release(ctx context.Context, scope entity.CentralEnvScope, leaseID string) (bool, error) {
	if strings.TrimSpace(leaseID) == "" {
		return false, nil
	}
	usageKey := centralKey(scope, "usage")
	leaseKey := centralKey(scope, "lease:"+leaseID)
	indexKey := centralKey(scope, "lease_index")
	res, err := d.cmdable.Eval(ctx, centralReleaseScript, []string{usageKey, leaseKey, indexKey}, leaseID).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central release: eval failed, lease=%s", leaseID)
	}
	return toInt64(res) == 1, nil
}

func (d *CentralSchedulingDAO) Usage(ctx context.Context, scope entity.CentralEnvScope) (map[entity.CentralResourceKind]int64, error) {
	raw, err := d.cmdable.HGetAll(ctx, centralKey(scope, "usage")).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, errorx.Wrapf(err, "central usage: hgetall failed")
	}
	out := make(map[entity.CentralResourceKind]int64, len(raw))
	for kind, value := range raw {
		v, _ := strconv.ParseInt(value, 10, 64)
		out[entity.CentralResourceKind(kind)] = v
	}
	return out, nil
}

func (d *CentralSchedulingDAO) ListLeases(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralLease, error) {
	indexKey := centralKey(scope, "lease_index")
	leaseIDs, err := d.cmdable.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, errorx.Wrapf(err, "central leases: zrange failed")
	}
	leases := make([]*entity.CentralLease, 0, len(leaseIDs))
	for _, leaseID := range leaseIDs {
		leaseKey := centralKey(scope, "lease:"+leaseID)
		fields, err := d.cmdable.HGetAll(ctx, leaseKey).Result()
		if err != nil && !redis.IsNilError(err) {
			return nil, errorx.Wrapf(err, "central leases: hgetall failed, lease=%s", leaseID)
		}
		if len(fields) == 0 {
			continue
		}
		lease := &entity.CentralLease{
			LeaseID:    fields["lease_id"],
			SpaceID:    mustParseInt64(fields["space_id"]),
			ExptID:     mustParseInt64(fields["expt_id"]),
			ExptRunID:  mustParseInt64(fields["expt_run_id"]),
			ItemID:     mustParseInt64(fields["item_id"]),
			AcquiredAt: mustParseInt64(fields["acquired_at"]),
			ExpireAt:   mustParseInt64(fields["expire_at"]),
		}
		const claimFieldPrefix = "claim:"
		for field, raw := range fields {
			if !strings.HasPrefix(field, claimFieldPrefix) {
				continue
			}
			amount, _ := strconv.ParseInt(raw, 10, 64)
			if amount <= 0 {
				continue
			}
			lease.Claims = append(lease.Claims, entity.CentralResourceClaim{
				Kind:   entity.CentralResourceKind(strings.TrimPrefix(field, claimFieldPrefix)),
				Amount: amount,
			})
		}
		leases = append(leases, lease)
	}
	return leases, nil
}

func (d *CentralSchedulingDAO) RebuildUsage(ctx context.Context, scope entity.CentralEnvScope, capacities map[entity.CentralResourceKind]int64, ttl time.Duration) (before, after map[entity.CentralResourceKind]int64, repaired int, err error) {
	before, err = d.Usage(ctx, scope)
	if err != nil {
		return nil, nil, 0, err
	}
	leases, err := d.ListLeases(ctx, scope)
	if err != nil {
		return nil, nil, 0, err
	}
	after = make(map[entity.CentralResourceKind]int64, len(capacities))
	for kind := range capacities {
		after[kind] = 0
	}
	for _, lease := range leases {
		if lease.ExpireAt > 0 && lease.ExpireAt < time.Now().Unix() {
			// 过期 lease 的残留条目由 Redis TTL 自行清理；重建时不再计入。
			continue
		}
		for _, claim := range lease.Claims {
			after[claim.Kind] += claim.Amount
		}
	}
	if sameUsage(before, after) {
		return before, after, 0, nil
	}

	usageKey := centralKey(scope, "usage")
	if err := d.cmdable.Del(ctx, usageKey).Err(); err != nil {
		return before, after, 0, errorx.Wrapf(err, "central rebuild: del usage failed")
	}
	if len(after) > 0 {
		values := make([]interface{}, 0, len(after)*2)
		for kind, amount := range after {
			values = append(values, string(kind), amount)
		}
		if err := d.cmdable.HSet(ctx, usageKey, values...).Err(); err != nil {
			return before, after, 0, errorx.Wrapf(err, "central rebuild: hset usage failed")
		}
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if err := d.cmdable.Expire(ctx, usageKey, ttl).Err(); err != nil {
		return before, after, 0, errorx.Wrapf(err, "central rebuild: expire usage failed")
	}
	return before, after, len(leases), nil
}

func (d *CentralSchedulingDAO) RegisterActive(ctx context.Context, scope entity.CentralEnvScope, run *entity.CentralActiveRun, ttl time.Duration) error {
	if run == nil || run.ExptID <= 0 || run.ExptRunID <= 0 {
		return errorx.New("central active: invalid run")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	activeKey := centralKey(scope, "active")
	member := entity.MakeCentralActiveMember(run.SpaceID, run.ExptID, run.ExptRunID, run.Priority, run.SessionUserID)
	if err := d.cmdable.ZAdd(ctx, activeKey, redislib.Z{Score: float64(run.Priority), Member: member}).Err(); err != nil {
		return errorx.Wrapf(err, "central active: zadd failed")
	}
	if err := d.cmdable.Expire(ctx, activeKey, ttl).Err(); err != nil {
		return errorx.Wrapf(err, "central active: expire failed")
	}
	return nil
}

func (d *CentralSchedulingDAO) UnregisterActive(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) error {
	activeKey := centralKey(scope, "active")
	// 成员里包含 spaceID；注销时按 exptID/runID 匹配，兼容 spaceID 未知的调用方。
	members, err := d.cmdable.ZRange(ctx, activeKey, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return errorx.Wrapf(err, "central active: zrange failed")
	}
	for _, m := range members {
		_, gotExptID, gotRunID, _, _, ok := entity.ParseCentralActiveMember(m)
		if !ok || gotExptID != exptID || gotRunID != runID {
			continue
		}
		if _, err := d.cmdable.Eval(ctx, centralUnregisterActiveScript, []string{activeKey}, m).Result(); err != nil {
			return errorx.Wrapf(err, "central active: zrem failed, member=%s", m)
		}
	}
	return nil
}

func (d *CentralSchedulingDAO) ListActive(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralActiveRun, error) {
	activeKey := centralKey(scope, "active")
	members, err := d.cmdable.ZRange(ctx, activeKey, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, errorx.Wrapf(err, "central active: zrange failed")
	}
	runs := make([]*entity.CentralActiveRun, 0, len(members))
	for _, member := range members {
		spaceID, exptID, runID, priority, userID, ok := entity.ParseCentralActiveMember(member)
		if !ok {
			continue
		}
		runs = append(runs, &entity.CentralActiveRun{SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, Priority: priority, SessionUserID: userID})
	}
	return runs, nil
}

func (d *CentralSchedulingDAO) ReleaseRun(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) (int, error) {
	leases, err := d.ListLeases(ctx, scope)
	if err != nil {
		return 0, err
	}
	released := 0
	for _, lease := range leases {
		if lease == nil || lease.ExptID != exptID || lease.ExptRunID != runID {
			continue
		}
		ok, relErr := d.Release(ctx, scope, lease.LeaseID)
		if relErr != nil {
			return released, relErr
		}
		if ok {
			released++
		}
	}
	return released, nil
}

func (d *CentralSchedulingDAO) AcquireLeader(ctx context.Context, scope entity.CentralEnvScope, owner string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 20 * time.Second
	}
	ok, err := d.cmdable.SetNX(ctx, centralKey(scope, "leader"), owner, ttl).Result()
	if err != nil && !redis.IsNilError(err) {
		return false, errorx.Wrapf(err, "central leader: setnx failed")
	}
	return ok, nil
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func mustParseInt64(raw string) int64 {
	v, _ := strconv.ParseInt(raw, 10, 64)
	return v
}

func sameUsage(a, b map[entity.CentralResourceKind]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for kind, amount := range a {
		if b[kind] != amount {
			return false
		}
	}
	return true
}
