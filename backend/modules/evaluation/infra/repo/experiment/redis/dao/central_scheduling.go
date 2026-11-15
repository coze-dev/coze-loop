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
  'expire_at', ARGV[metaBase + 6],
  'attempt', ARGV[metaBase + 7],
  'state', ARGV[metaBase + 8])
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
local pendingKey = KEYS[4]
local leaseID = ARGV[1]
if redis.call('EXISTS', leaseKey) == 0 then
  redis.call('ZREM', indexKey, leaseID)
  redis.call('ZREM', pendingKey, leaseID)
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
redis.call('ZREM', pendingKey, leaseID)
return 1
`

// 按 (expt, run, item) 释放该 item 的全部 attempt；用于完成/失败/取消/重试前的统一释放。
// 重复调用返回 0，天然幂等。
const centralReleaseItemsScript = `
local usageKey = KEYS[1]
local indexKey = KEYS[2]
local pendingKey = KEYS[3]
local leasePrefix = ARGV[1]
local exptID = ARGV[2]
local runID = ARGV[3]
local itemID = ARGV[4]
local ids = redis.call('ZRANGE', indexKey, 0, -1)
local released = 0
for i = 1, #ids do
  local leaseID = ids[i]
  local leaseKey = leasePrefix .. leaseID
  local fields = redis.call('HGETALL', leaseKey)
  if #fields > 0 then
    local fExpt = ''
    local fRun = ''
    local fItem = ''
    for j = 1, #fields, 2 do
      local f = fields[j]
      if f == 'expt_id' then fExpt = fields[j + 1]
      elseif f == 'expt_run_id' then fRun = fields[j + 1]
      elseif f == 'item_id' then fItem = fields[j + 1]
      end
    end
    if fExpt == exptID and fRun == runID and fItem == itemID then
      for j = 1, #fields, 2 do
        local field = fields[j]
        if string.sub(field, 1, 6) == 'claim:' then
          local kind = string.sub(field, 7)
          local amount = tonumber(fields[j + 1])
          local used = tonumber(redis.call('HGET', usageKey, kind) or '0')
          if used - amount < 0 then amount = used end
          redis.call('HINCRBY', usageKey, kind, -amount)
        end
      end
      redis.call('DEL', leaseKey)
      redis.call('ZREM', indexKey, leaseID)
      redis.call('ZREM', pendingKey, leaseID)
      released = released + 1
    end
  else
    redis.call('ZREM', indexKey, leaseID)
  end
end
return released
`

// 按 experiment 释放（删除实验、实验级取消）。
const centralReleaseExptScript = `
local usageKey = KEYS[1]
local indexKey = KEYS[2]
local pendingKey = KEYS[3]
local leasePrefix = ARGV[1]
local exptID = ARGV[2]
local ids = redis.call('ZRANGE', indexKey, 0, -1)
local released = 0
for i = 1, #ids do
  local leaseID = ids[i]
  local leaseKey = leasePrefix .. leaseID
  local fields = redis.call('HGETALL', leaseKey)
  local match = false
  if #fields > 0 then
    for j = 1, #fields, 2 do
      if fields[j] == 'expt_id' and fields[j + 1] == exptID then match = true break end
    end
  end
  if match then
    for j = 1, #fields, 2 do
      local field = fields[j]
      if string.sub(field, 1, 6) == 'claim:' then
        local kind = string.sub(field, 7)
        local amount = tonumber(fields[j + 1])
        local used = tonumber(redis.call('HGET', usageKey, kind) or '0')
        if used - amount < 0 then amount = used end
        redis.call('HINCRBY', usageKey, kind, -amount)
      end
    end
    redis.call('DEL', leaseKey)
    released = released + 1
  end
  if #fields == 0 or match then
    redis.call('ZREM', indexKey, leaseID)
    redis.call('ZREM', pendingKey, leaseID)
  end
end
return released
`

const centralReleaseRunScript = `
local usageKey = KEYS[1]
local indexKey = KEYS[2]
local pendingKey = KEYS[3]
local leasePrefix = ARGV[1]
local exptID = ARGV[2]
local runID = ARGV[3]
local ids = redis.call('ZRANGE', indexKey, 0, -1)
local released = 0
for i = 1, #ids do
  local leaseID = ids[i]
  local leaseKey = leasePrefix .. leaseID
  local fields = redis.call('HGETALL', leaseKey)
  local match = false
  if #fields > 0 then
    local fExpt = ''
    local fRun = ''
    for j = 1, #fields, 2 do
      if fields[j] == 'expt_id' then fExpt = fields[j + 1]
      elseif fields[j] == 'expt_run_id' then fRun = fields[j + 1]
      end
    end
    match = fExpt == exptID and fRun == runID
  end
  if match then
    for j = 1, #fields, 2 do
      local field = fields[j]
      if string.sub(field, 1, 6) == 'claim:' then
        local kind = string.sub(field, 7)
        local amount = tonumber(fields[j + 1])
        local used = tonumber(redis.call('HGET', usageKey, kind) or '0')
        if used - amount < 0 then amount = used end
        redis.call('HINCRBY', usageKey, kind, -amount)
      end
    end
    redis.call('DEL', leaseKey)
    released = released + 1
  end
  if #fields == 0 or match then
    redis.call('ZREM', indexKey, leaseID)
    redis.call('ZREM', pendingKey, leaseID)
  end
end
return released
`

// 明确派发成功：reserved -> dispatched。已 dispatched 的重复确认是幂等成功；
// 迟到的确认若已进入 pending_confirm，则提升为 dispatched 并从待确认集合移除。
const centralMarkDispatchedScript = `
local leaseKey = KEYS[1]
local pendingKey = KEYS[2]
local leaseID = ARGV[1]
if redis.call('EXISTS', leaseKey) == 0 then
  redis.call('ZREM', pendingKey, leaseID)
  return 0
end
local state = redis.call('HGET', leaseKey, 'state')
if state == 'dispatched' then
  return 1
end
if state == 'reserved' or state == 'pending_confirm' then
  redis.call('HSET', leaseKey, 'state', 'dispatched')
  redis.call('ZREM', pendingKey, leaseID)
  return 1
end
return 0
`

// 派发结果不明：reserved -> pending_confirm，并登记 deadline。
const centralMarkPendingScript = `
local leaseKey = KEYS[1]
local pendingKey = KEYS[2]
local leaseID = ARGV[1]
local deadline = tonumber(ARGV[2])
if redis.call('EXISTS', leaseKey) == 0 then
  return 0
end
local state = redis.call('HGET', leaseKey, 'state')
if state == 'pending_confirm' then
  return 1
end
if state ~= 'reserved' then
  return 0
end
redis.call('HSET', leaseKey, 'state', 'pending_confirm', 'confirm_deadline', deadline)
redis.call('ZADD', pendingKey, deadline, leaseID)
redis.call('EXPIRE', pendingKey, 172800)
return 1
`

// deadline sweeper：到期仍未确认的预占必须补偿释放并留下审计记录，不允许长期悬挂。
const centralSweepPendingScript = `
local pendingKey = KEYS[1]
local usageKey = KEYS[2]
local indexKey = KEYS[3]
local auditKey = KEYS[4]
local leasePrefix = ARGV[1]
local now = tonumber(ARGV[2])
local auditTTL = tonumber(ARGV[3])
local ids = redis.call('ZRANGEBYSCORE', pendingKey, '-inf', now)
local released = 0
for i = 1, #ids do
  local leaseID = ids[i]
  local leaseKey = leasePrefix .. leaseID
  local fields = redis.call('HGETALL', leaseKey)
  if #fields > 0 then
    local state = ''
    for j = 1, #fields, 2 do
      if fields[j] == 'state' then state = fields[j + 1] end
    end
    if state == 'pending_confirm' then
      for j = 1, #fields, 2 do
        local field = fields[j]
        if string.sub(field, 1, 6) == 'claim:' then
          local kind = string.sub(field, 7)
          local amount = tonumber(fields[j + 1])
          local used = tonumber(redis.call('HGET', usageKey, kind) or '0')
          if used - amount < 0 then amount = used end
          redis.call('HINCRBY', usageKey, kind, -amount)
        end
      end
      redis.call('DEL', leaseKey)
      redis.call('ZREM', indexKey, leaseID)
      redis.call('ZADD', auditKey, now, leaseID)
      released = released + 1
    end
  end
  redis.call('ZREM', pendingKey, leaseID)
end
redis.call('EXPIRE', auditKey, auditTTL)
return released
`

// 主实例锁：同 owner 重复获取时应续期而不是失败，否则 ticker 每两个 tick 才能跑一次。
const centralLeaderScript = `
local key = KEYS[1]
local owner = ARGV[1]
local ttlMs = tonumber(ARGV[2])
local current = redis.call('GET', key)
if current == false then
  redis.call('SET', key, owner, 'PX', ttlMs)
  return 1
end
if current == owner then
  redis.call('PEXPIRE', key, ttlMs)
  return 1
end
return 0
`

// 账本重建：在脚本内读取 lease_index 与存活 lease，并原子替换 usage，
// 避免“删除 usage 之后、写回之前”有并发 acquire 造成占用丢失。
// 返回：repaired, 每类资源 after, orphan 数量, orphan lease ids...
const centralRebuildScript = `
local usageKey = KEYS[1]
local indexKey = KEYS[2]
local leasePrefix = ARGV[1]
local ttl = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local nk = tonumber(ARGV[4])
local kinds = {}
local counts = {}
for i = 1, nk do
  local kind = ARGV[4 + i]
  kinds[i] = kind
  counts[kind] = 0
end
local ids = redis.call('ZRANGE', indexKey, 0, -1)
local repaired = 0
local orphans = {}
for i = 1, #ids do
  local leaseID = ids[i]
  local fields = redis.call('HGETALL', leasePrefix .. leaseID)
  if #fields > 0 then
    local expireAt = 0
    for j = 1, #fields, 2 do
      if fields[j] == 'expire_at' then
        expireAt = tonumber(fields[j + 1]) or 0
      end
    end
    if expireAt == 0 or expireAt >= now then
      repaired = repaired + 1
      for j = 1, #fields, 2 do
        local field = fields[j]
        if string.sub(field, 1, 6) == 'claim:' then
          local kind = string.sub(field, 7)
          local amount = tonumber(fields[j + 1]) or 0
          if counts[kind] == nil then
            counts[kind] = 0
          end
          counts[kind] = counts[kind] + amount
        end
      end
    else
      redis.call('ZREM', indexKey, leaseID)
    end
  else
    orphans[#orphans + 1] = leaseID
    redis.call('ZREM', indexKey, leaseID)
  end
end
redis.call('DEL', usageKey)
for kind, amount in pairs(counts) do
  redis.call('HSET', usageKey, kind, amount)
end
redis.call('EXPIRE', usageKey, ttl)
local out = {repaired}
for i = 1, nk do
  out[#out + 1] = kinds[i]
  out[#out + 1] = counts[kinds[i]] or 0
end
out[#out + 1] = #orphans
for i = 1, #orphans do
  out[#out + 1] = orphans[i]
end
return out
`

// 冻结 run 状态：首次登记写入 mode/priority/seq/target_concurrency；之后只读不覆盖。
const centralEnsureRunScript = `
local runKey = KEYS[1]
local seqKey = KEYS[2]
local spaceID = ARGV[1]
local exptID = ARGV[2]
local runID = ARGV[3]
local mode = ARGV[4]
local priority = ARGV[5]
local target = ARGV[6]
local now = ARGV[7]
local ttl = tonumber(ARGV[8])
if redis.call('EXISTS', runKey) == 1 then
  local seq = redis.call('HGET', runKey, 'seq') or '0'
  return {0,
    redis.call('HGET', runKey, 'mode') or mode,
    redis.call('HGET', runKey, 'priority') or priority,
    seq,
    redis.call('HGET', runKey, 'target_concurrency') or target,
    redis.call('HGET', runKey, 'status') or 'pending',
    redis.call('HGET', runKey, 'registered_at') or now,
    redis.call('HGET', runKey, 'frozen_at') or now,
    redis.call('HGET', runKey, 'space_id') or spaceID,
    exptID,
    runID,
    redis.call('HGET', runKey, 'switch_ticket_id') or '',
    redis.call('HGET', runKey, 'switch_operator') or '',
    redis.call('HGET', runKey, 'switch_reason') or '',
    redis.call('HGET', runKey, 'switched_at') or '0'}
end
local seq = redis.call('INCR', seqKey)
redis.call('EXPIRE', seqKey, ttl + 86400)
redis.call('HSET', runKey,
  'space_id', spaceID,
  'expt_id', exptID,
  'expt_run_id', runID,
  'mode', mode,
  'priority', priority,
  'seq', seq,
  'target_concurrency', target,
  'status', 'pending',
  'registered_at', now,
  'frozen_at', now)
redis.call('EXPIRE', runKey, ttl)
return {1, mode, priority, tostring(seq), target, 'pending', now, now, spaceID, exptID, runID, '', '', '', '0'}
`

// 显式授权的一次性模式切换：一个实验只允许一次，记录工单/操作人/原因。
const centralSwitchModeScript = `
local runKey = KEYS[1]
local switchKey = KEYS[2]
local newMode = ARGV[1]
local ticket = ARGV[2]
local operator = ARGV[3]
local reason = ARGV[4]
local now = ARGV[5]
local ttl = tonumber(ARGV[6])
if redis.call('EXISTS', switchKey) == 1 then
  return 0
end
if redis.call('EXISTS', runKey) == 0 then
  return 0
end
redis.call('HSET', switchKey,
  'ticket_id', ticket,
  'operator', operator,
  'reason', reason,
  'to_mode', newMode,
  'switched_at', now)
redis.call('EXPIRE', switchKey, ttl)
redis.call('HSET', runKey,
  'mode', newMode,
  'switch_ticket_id', ticket,
  'switch_operator', operator,
  'switch_reason', reason,
  'switched_at', now)
return 1
`

// pending 初始化 -> active 维护：两个集合互斥迁移，不会同时存在。
const centralActivateRunScript = `
local pendingKey = KEYS[1]
local activeKey = KEYS[2]
local exptID = ARGV[1]
local runID = ARGV[2]
local member = ARGV[3]
local score = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])
-- 按 expt/run 匹配移除 pending 成员，不依赖 member 里的 userID 是否完全一致。
local needle = ':' .. exptID .. ':' .. runID .. ':'
local members = redis.call('ZRANGE', pendingKey, 0, -1)
for i = 1, #members do
  if string.find(members[i], needle, 1, true) ~= nil then
    redis.call('ZREM', pendingKey, members[i])
  end
end
redis.call('ZADD', activeKey, score, member)
redis.call('EXPIRE', activeKey, ttl)
return 1
`

const centralUnregisterActiveScript = `
if redis.call('ZREM', KEYS[1], ARGV[1]) > 0 then
  return 1
end
return 0
`

// 每次预占生成 run 内单调递增的 attempt，保证“每次预占唯一身份”。
const centralNextAttemptScript = `
local attemptKey = KEYS[1]
local itemID = ARGV[1]
local ttl = tonumber(ARGV[2])
local attempt = redis.call('HINCRBY', attemptKey, itemID, 1)
redis.call('EXPIRE', attemptKey, ttl)
return attempt
`

// CentralKeys 是一次 env/lane 作用域内的全部 key，便于审计与测试。
type CentralKeys struct {
	Usage            string
	LeaseIndex       string
	Active           string
	Pending          string
	PendingConfirm   string
	Seq              string
	Leader           string
	Attempt          string
	RunState         string
	Switch           string
	Unconfirmed      string
	DegradeHealthy   string
	DegradeRecovered string
	Lease            func(string) string
}

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
func (d *CentralSchedulingDAO) Keys(scope entity.CentralEnvScope) CentralKeys {
	return CentralKeys{
		Usage:            centralKey(scope, "usage"),
		LeaseIndex:       centralKey(scope, "lease_index"),
		Active:           centralKey(scope, "active"),
		Pending:          centralKey(scope, "pending"),
		PendingConfirm:   centralKey(scope, "pending_confirm"),
		Seq:              centralKey(scope, "seq"),
		Leader:           centralKey(scope, "leader"),
		Attempt:          centralKey(scope, "attempt"),
		RunState:         centralKey(scope, "run"),
		Switch:           centralKey(scope, "switch"),
		Unconfirmed:      centralKey(scope, "unconfirmed"),
		DegradeHealthy:   centralKey(scope, "degrade_healthy_ticks"),
		DegradeRecovered: centralKey(scope, "degrade_recovered"),
		Lease:            func(leaseID string) string { return centralKey(scope, "lease:"+leaseID) },
	}
}

func invalidScopeErr(op string) error {
	return errorx.New("central %s: env/lane scope is invalid, refuse to read or write other owners' data", op)
}

func (d *CentralSchedulingDAO) Acquire(ctx context.Context, scope entity.CentralEnvScope, capacities map[entity.CentralResourceKind]int64, lease *entity.CentralLease, ttl time.Duration) (bool, error) {
	if !scope.IsValid() {
		return false, invalidScopeErr("acquire")
	}
	if lease == nil || lease.LeaseID == "" {
		return false, errorx.New("central acquire: lease id is required")
	}
	if len(lease.Claims) == 0 {
		return false, errorx.New("central acquire: claims are required")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	keys := d.Keys(scope)

	args := make([]interface{}, 0, 2+len(lease.Claims)*3+9)
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
	state := lease.State
	if state == "" {
		state = entity.CentralLeaseStateReserved
	}
	args = append(args,
		lease.LeaseID,
		lease.SpaceID,
		lease.ExptID,
		lease.ExptRunID,
		lease.ItemID,
		lease.AcquiredAt,
		lease.ExpireAt,
		lease.Attempt,
		string(state),
	)
	res, err := d.cmdable.Eval(ctx, centralAcquireScript, []string{keys.Usage, keys.Lease(lease.LeaseID), keys.LeaseIndex}, args...).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central acquire: eval failed, lease=%s", lease.LeaseID)
	}
	if toInt64(res) != 1 {
		return false, nil
	}
	lease.State = state
	return true, nil
}

func (d *CentralSchedulingDAO) Release(ctx context.Context, scope entity.CentralEnvScope, leaseID string) (bool, error) {
	if strings.TrimSpace(leaseID) == "" {
		return false, nil
	}
	if !scope.IsValid() {
		return false, invalidScopeErr("release")
	}
	keys := d.Keys(scope)
	res, err := d.cmdable.Eval(ctx, centralReleaseScript,
		[]string{keys.Usage, keys.Lease(leaseID), keys.LeaseIndex, keys.PendingConfirm}, leaseID).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central release: eval failed, lease=%s", leaseID)
	}
	return toInt64(res) == 1, nil
}

func (d *CentralSchedulingDAO) ReleaseItems(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64, itemIDs []int64) (int, error) {
	if !scope.IsValid() {
		return 0, invalidScopeErr("release items")
	}
	keys := d.Keys(scope)
	released := 0
	for _, itemID := range itemIDs {
		res, err := d.cmdable.Eval(ctx, centralReleaseItemsScript,
			[]string{keys.Usage, keys.LeaseIndex, keys.PendingConfirm},
			centralKey(scope, "lease:"), strconv.FormatInt(exptID, 10), strconv.FormatInt(runID, 10), strconv.FormatInt(itemID, 10)).Result()
		if err != nil {
			return released, errorx.Wrapf(err, "central release items: eval failed, item=%d", itemID)
		}
		released += int(toInt64(res))
	}
	return released, nil
}

func (d *CentralSchedulingDAO) ReleaseRun(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) (int, error) {
	if !scope.IsValid() {
		return 0, invalidScopeErr("release run")
	}
	keys := d.Keys(scope)
	res, err := d.cmdable.Eval(ctx, centralReleaseRunScript,
		[]string{keys.Usage, keys.LeaseIndex, keys.PendingConfirm},
		centralKey(scope, "lease:"), strconv.FormatInt(exptID, 10), strconv.FormatInt(runID, 10)).Result()
	if err != nil {
		return 0, errorx.Wrapf(err, "central release run: eval failed, expt=%d run=%d", exptID, runID)
	}
	return int(toInt64(res)), nil
}

func (d *CentralSchedulingDAO) ReleaseExpt(ctx context.Context, scope entity.CentralEnvScope, exptID int64) (int, error) {
	if !scope.IsValid() {
		return 0, invalidScopeErr("release expt")
	}
	keys := d.Keys(scope)
	res, err := d.cmdable.Eval(ctx, centralReleaseExptScript,
		[]string{keys.Usage, keys.LeaseIndex, keys.PendingConfirm},
		centralKey(scope, "lease:"), strconv.FormatInt(exptID, 10)).Result()
	if err != nil {
		return 0, errorx.Wrapf(err, "central release expt: eval failed, expt=%d", exptID)
	}
	return int(toInt64(res)), nil
}

func (d *CentralSchedulingDAO) MarkDispatched(ctx context.Context, scope entity.CentralEnvScope, leaseID string) (bool, error) {
	if !scope.IsValid() {
		return false, invalidScopeErr("mark dispatched")
	}
	keys := d.Keys(scope)
	res, err := d.cmdable.Eval(ctx, centralMarkDispatchedScript,
		[]string{keys.Lease(leaseID), keys.PendingConfirm}, leaseID).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central mark dispatched: eval failed, lease=%s", leaseID)
	}
	return toInt64(res) == 1, nil
}

func (d *CentralSchedulingDAO) MarkPendingConfirm(ctx context.Context, scope entity.CentralEnvScope, leaseID string, deadline int64) (bool, error) {
	if !scope.IsValid() {
		return false, invalidScopeErr("mark pending confirm")
	}
	keys := d.Keys(scope)
	res, err := d.cmdable.Eval(ctx, centralMarkPendingScript,
		[]string{keys.Lease(leaseID), keys.PendingConfirm}, leaseID, deadline).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central mark pending confirm: eval failed, lease=%s", leaseID)
	}
	return toInt64(res) == 1, nil
}

func (d *CentralSchedulingDAO) SweepPendingConfirms(ctx context.Context, scope entity.CentralEnvScope, now int64, auditTTL time.Duration) (int, error) {
	if !scope.IsValid() {
		return 0, invalidScopeErr("sweep pending confirm")
	}
	if auditTTL <= 0 {
		auditTTL = 7 * 24 * time.Hour
	}
	keys := d.Keys(scope)
	res, err := d.cmdable.Eval(ctx, centralSweepPendingScript,
		[]string{keys.PendingConfirm, keys.Usage, keys.LeaseIndex, keys.Unconfirmed},
		centralKey(scope, "lease:"), now, int64(auditTTL/time.Second)).Result()
	if err != nil {
		return 0, errorx.Wrapf(err, "central sweep pending confirm: eval failed")
	}
	return int(toInt64(res)), nil
}

func (d *CentralSchedulingDAO) Usage(ctx context.Context, scope entity.CentralEnvScope) (map[entity.CentralResourceKind]int64, error) {
	if !scope.IsValid() {
		return nil, invalidScopeErr("usage")
	}
	raw, err := d.cmdable.HGetAll(ctx, d.Keys(scope).Usage).Result()
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
	if !scope.IsValid() {
		return nil, invalidScopeErr("list leases")
	}
	keys := d.Keys(scope)
	leaseIDs, err := d.cmdable.ZRange(ctx, keys.LeaseIndex, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, errorx.Wrapf(err, "central leases: zrange failed")
	}
	leases := make([]*entity.CentralLease, 0, len(leaseIDs))
	for _, leaseID := range leaseIDs {
		fields, err := d.cmdable.HGetAll(ctx, keys.Lease(leaseID)).Result()
		if err != nil && !redis.IsNilError(err) {
			return nil, errorx.Wrapf(err, "central leases: hgetall failed, lease=%s", leaseID)
		}
		if len(fields) == 0 {
			continue
		}
		leases = append(leases, leaseFromFields(fields))
	}
	return leases, nil
}

func leaseFromFields(fields map[string]string) *entity.CentralLease {
	lease := &entity.CentralLease{
		LeaseID:    fields["lease_id"],
		SpaceID:    mustParseInt64(fields["space_id"]),
		ExptID:     mustParseInt64(fields["expt_id"]),
		ExptRunID:  mustParseInt64(fields["expt_run_id"]),
		ItemID:     mustParseInt64(fields["item_id"]),
		Attempt:    mustParseInt64(fields["attempt"]),
		State:      entity.CentralLeaseState(fields["state"]),
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
	return lease
}

// InspectLedger 做每 tick 的增量校验：只读，不改账本。
func (d *CentralSchedulingDAO) InspectLedger(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralLedgerDrift, error) {
	if !scope.IsValid() {
		return nil, invalidScopeErr("inspect ledger")
	}
	keys := d.Keys(scope)
	ids, err := d.cmdable.ZRange(ctx, keys.LeaseIndex, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, errorx.Wrapf(err, "central inspect: zrange failed")
	}
	used, err := d.Usage(ctx, scope)
	if err != nil {
		return nil, err
	}
	leases := make([]*entity.CentralLease, 0, len(ids))
	drifts := make([]*entity.CentralLedgerDrift, 0)
	for _, leaseID := range ids {
		fields, hErr := d.cmdable.HGetAll(ctx, keys.Lease(leaseID)).Result()
		if hErr != nil && !redis.IsNilError(hErr) {
			return nil, errorx.Wrapf(hErr, "central inspect: hgetall failed, lease=%s", leaseID)
		}
		if len(fields) == 0 {
			drifts = append(drifts, &entity.CentralLedgerDrift{LeaseID: leaseID, Reason: "lease_hash_missing"})
			continue
		}
		leases = append(leases, leaseFromFields(fields))
	}
	expected := make(map[entity.CentralResourceKind]int64, len(entity.DefaultCentralResourceKinds))
	for _, lease := range leases {
		for _, claim := range lease.Claims {
			expected[claim.Kind] += claim.Amount
		}
	}
	for kind, usedValue := range used {
		if usedValue != 0 && expected[kind] == 0 {
			drifts = append(drifts, &entity.CentralLedgerDrift{Kind: kind, Usage: usedValue, Reason: "usage_without_lease"})
		}
	}
	for kind, want := range expected {
		if used[kind] == want {
			continue
		}
		// 漂移归因到具体 lease：逐笔列出该资源的所有存活预占，便于定位。
		for _, lease := range leases {
			amount := lease.TotalClaim(kind)
			if amount == 0 {
				continue
			}
			drifts = append(drifts, &entity.CentralLedgerDrift{
				LeaseID: lease.LeaseID,
				Kind:    kind,
				Amount:  amount,
				Usage:   used[kind],
				Reason:  "usage_drift",
			})
		}
	}
	return drifts, nil
}

func (d *CentralSchedulingDAO) RebuildUsage(ctx context.Context, scope entity.CentralEnvScope, capacities map[entity.CentralResourceKind]int64, ttl time.Duration) (before, after map[entity.CentralResourceKind]int64, repaired int, drift []*entity.CentralLedgerDrift, err error) {
	before, err = d.Usage(ctx, scope)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	after = make(map[entity.CentralResourceKind]int64, len(capacities))
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	kinds := make([]entity.CentralResourceKind, 0, len(capacities))
	for kind := range capacities {
		kinds = append(kinds, kind)
	}
	args := make([]interface{}, 0, 4+len(kinds))
	args = append(args, centralKey(scope, "lease:"), int64(ttl/time.Second), time.Now().Unix(), len(kinds))
	for _, kind := range kinds {
		args = append(args, string(kind))
	}
	keys := d.Keys(scope)
	res, err := d.cmdable.Eval(ctx, centralRebuildScript, []string{keys.Usage, keys.LeaseIndex}, args...).Result()
	if err != nil {
		return before, after, 0, nil, errorx.Wrapf(err, "central rebuild: eval failed")
	}
	values, ok := res.([]interface{})
	if !ok || len(values) < 1+len(kinds)*2+1 {
		return before, after, 0, nil, errorx.New("central rebuild: unexpected script result")
	}
	repaired = int(toInt64(values[0]))
	for i := 1; i+1 < 1+len(kinds)*2; i += 2 {
		kind, _ := values[i].(string)
		after[entity.CentralResourceKind(kind)] = toInt64(values[i+1])
	}
	orphanCountIdx := 1 + len(kinds)*2
	orphanCount := int(toInt64(values[orphanCountIdx]))
	for i := 0; i < orphanCount && orphanCountIdx+1+i < len(values); i++ {
		leaseID, _ := values[orphanCountIdx+1+i].(string)
		drift = append(drift, &entity.CentralLedgerDrift{LeaseID: leaseID, Reason: "lease_hash_missing"})
	}
	// 使用量不一致时按 lease 归因，让漂移能定位到具体一笔。
	for kind, beforeValue := range before {
		if afterValue, ok := after[kind]; ok && beforeValue == afterValue {
			continue
		}
		leases, listErr := d.ListLeases(ctx, scope)
		if listErr != nil {
			break
		}
		for _, lease := range leases {
			amount := lease.TotalClaim(kind)
			if amount == 0 {
				continue
			}
			drift = append(drift, &entity.CentralLedgerDrift{
				LeaseID: lease.LeaseID,
				Kind:    kind,
				Amount:  amount,
				Usage:   beforeValue,
				Reason:  "usage_rebuilt",
			})
		}
		break
	}
	return before, after, repaired, drift, nil
}

// EnsureRunState 首次登记时冻结 mode/priority/seq/target_concurrency；已存在则只读返回。
func (d *CentralSchedulingDAO) EnsureRunState(ctx context.Context, scope entity.CentralEnvScope, state *entity.CentralRunState, ttl time.Duration) (*entity.CentralRunState, bool, error) {
	if !scope.IsValid() {
		return nil, false, invalidScopeErr("ensure run state")
	}
	if state == nil || state.ExptID <= 0 || state.ExptRunID <= 0 {
		return nil, false, errorx.New("central run state: invalid run")
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	keys := d.Keys(scope)
	runKey := keys.RunState + ":" + fmt.Sprintf("%d:%d", state.ExptID, state.ExptRunID)
	now := time.Now().Unix()
	if state.RegisteredAt <= 0 {
		state.RegisteredAt = now
	}
	args := []interface{}{
		state.SpaceID,
		state.ExptID,
		state.ExptRunID,
		string(state.Mode),
		state.Priority,
		state.TargetConcurrency,
		state.RegisteredAt,
		int64(ttl / time.Second),
	}
	res, err := d.cmdable.Eval(ctx, centralEnsureRunScript, []string{runKey, keys.Seq}, args...).Result()
	if err != nil {
		return nil, false, errorx.Wrapf(err, "central run state: ensure failed")
	}
	values, ok := res.([]interface{})
	if !ok || len(values) != 15 {
		return nil, false, errorx.New("central run state: unexpected script result")
	}
	created := toInt64(values[0]) == 1
	out := &entity.CentralRunState{
		Mode:              entity.CentralSchedulerMode(strValue(values[1])),
		Priority:          int(toInt64(values[2])),
		Seq:               toInt64(values[3]),
		TargetConcurrency: int(toInt64(values[4])),
		Status:            entity.CentralRunStatus(strValue(values[5])),
		RegisteredAt:      toInt64(values[6]),
		FrozenAt:          toInt64(values[7]),
		SpaceID:           toInt64(values[8]),
		ExptID:            toInt64(values[9]),
		ExptRunID:         toInt64(values[10]),
		SwitchTicketID:    strValue(values[11]),
		SwitchOperator:    strValue(values[12]),
		SwitchReason:      strValue(values[13]),
		SwitchedAt:        toInt64(values[14]),
	}
	return out, created, nil
}

func (d *CentralSchedulingDAO) GetRunState(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) (*entity.CentralRunState, bool, error) {
	if !scope.IsValid() {
		return nil, false, invalidScopeErr("get run state")
	}
	runKey := d.Keys(scope).RunState + ":" + fmt.Sprintf("%d:%d", exptID, runID)
	fields, err := d.cmdable.HGetAll(ctx, runKey).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, false, errorx.Wrapf(err, "central run state: hgetall failed")
	}
	if len(fields) == 0 {
		return nil, false, nil
	}
	state := &entity.CentralRunState{
		SpaceID:           mustParseInt64(fields["space_id"]),
		ExptID:            mustParseInt64(fields["expt_id"]),
		ExptRunID:         mustParseInt64(fields["expt_run_id"]),
		Mode:              entity.CentralSchedulerMode(fields["mode"]),
		Priority:          int(mustParseInt64(fields["priority"])),
		Seq:               mustParseInt64(fields["seq"]),
		TargetConcurrency: int(mustParseInt64(fields["target_concurrency"])),
		Status:            entity.CentralRunStatus(fields["status"]),
		FrozenAt:          mustParseInt64(fields["frozen_at"]),
		RegisteredAt:      mustParseInt64(fields["registered_at"]),
		SwitchTicketID:    fields["switch_ticket_id"],
		SwitchOperator:    fields["switch_operator"],
		SwitchReason:      fields["switch_reason"],
		SwitchedAt:        mustParseInt64(fields["switched_at"]),
	}
	return state, true, nil
}

// SwitchRunMode 消费审批后的一次性切换授权；同一实验第二次调用返回 false。
func (d *CentralSchedulingDAO) SwitchRunMode(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64, toMode entity.CentralSchedulerMode, auth *entity.CentralModeSwitchAuthorization) (bool, error) {
	if !scope.IsValid() {
		return false, invalidScopeErr("switch run mode")
	}
	if auth == nil || strings.TrimSpace(auth.TicketID) == "" {
		return false, errorx.New("central switch mode: approved ticket is required")
	}
	keys := d.Keys(scope)
	runKey := keys.RunState + ":" + fmt.Sprintf("%d:%d", exptID, runID)
	switchKey := keys.Switch + ":" + strconv.FormatInt(exptID, 10)
	res, err := d.cmdable.Eval(ctx, centralSwitchModeScript, []string{runKey, switchKey},
		string(toMode), auth.TicketID, auth.Operator, auth.Reason, time.Now().Unix(),
		int64((30*24*time.Hour)/time.Second)).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central switch mode: eval failed")
	}
	return toInt64(res) == 1, nil
}

func (d *CentralSchedulingDAO) RegisterPending(ctx context.Context, scope entity.CentralEnvScope, run *entity.CentralActiveRun, ttl time.Duration) error {
	if !scope.IsValid() {
		return invalidScopeErr("register pending")
	}
	return d.register(ctx, d.Keys(scope).Pending, run, ttl)
}

func (d *CentralSchedulingDAO) RegisterActive(ctx context.Context, scope entity.CentralEnvScope, run *entity.CentralActiveRun, ttl time.Duration) error {
	if !scope.IsValid() {
		return invalidScopeErr("register active")
	}
	return d.register(ctx, d.Keys(scope).Active, run, ttl)
}

func (d *CentralSchedulingDAO) register(ctx context.Context, key string, run *entity.CentralActiveRun, ttl time.Duration) error {
	if run == nil || run.ExptID <= 0 || run.ExptRunID <= 0 {
		return errorx.New("central run registry: invalid run")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	member := entity.MakeCentralActiveMember(run.SpaceID, run.ExptID, run.ExptRunID, run.Priority, run.SessionUserID)
	if err := d.cmdable.ZAdd(ctx, key, redislib.Z{Score: entity.CentralPriorityScore(run.Priority, run.Seq), Member: member}).Err(); err != nil {
		return errorx.Wrapf(err, "central registry: zadd failed")
	}
	if err := d.cmdable.Expire(ctx, key, ttl).Err(); err != nil {
		return errorx.Wrapf(err, "central registry: expire failed")
	}
	return nil
}

// ActivateRun 把运行从 pending 初始化迁移到 active 维护；两个 key 不同，互不覆盖。
func (d *CentralSchedulingDAO) ActivateRun(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64, sessionUserID string, ttl time.Duration) error {
	if !scope.IsValid() {
		return invalidScopeErr("activate run")
	}
	state, ok, err := d.GetRunState(ctx, scope, exptID, runID)
	if err != nil {
		return err
	}
	if !ok {
		return errorx.New("central activate: run state not found, expt=%d run=%d", exptID, runID)
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	keys := d.Keys(scope)
	member := entity.MakeCentralActiveMember(state.SpaceID, exptID, runID, state.Priority, sessionUserID)
	res, err := d.cmdable.Eval(ctx, centralActivateRunScript, []string{keys.Pending, keys.Active},
		strconv.FormatInt(exptID, 10), strconv.FormatInt(runID, 10), member,
		entity.CentralPriorityScore(state.Priority, state.Seq), int64(ttl/time.Second)).Result()
	if err != nil {
		return errorx.Wrapf(err, "central activate: eval failed")
	}
	if toInt64(res) != 1 {
		return errorx.New("central activate: unexpected script result")
	}
	return nil
}

func (d *CentralSchedulingDAO) UnregisterActive(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) error {
	if !scope.IsValid() {
		return invalidScopeErr("unregister active")
	}
	keys := d.Keys(scope)
	for _, key := range []string{keys.Active, keys.Pending} {
		members, err := d.cmdable.ZRange(ctx, key, 0, -1).Result()
		if err != nil && !redis.IsNilError(err) {
			return errorx.Wrapf(err, "central registry: zrange failed")
		}
		for _, m := range members {
			_, gotExptID, gotRunID, _, _, ok := entity.ParseCentralActiveMember(m)
			if !ok || gotExptID != exptID || gotRunID != runID {
				continue
			}
			if _, err := d.cmdable.Eval(ctx, centralUnregisterActiveScript, []string{key}, m).Result(); err != nil {
				return errorx.Wrapf(err, "central registry: zrem failed, member=%s", m)
			}
		}
	}
	return nil
}

func (d *CentralSchedulingDAO) ListActive(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralActiveRun, error) {
	if !scope.IsValid() {
		return nil, invalidScopeErr("list active")
	}
	return d.list(ctx, d.Keys(scope).Active)
}

func (d *CentralSchedulingDAO) ListPending(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralActiveRun, error) {
	if !scope.IsValid() {
		return nil, invalidScopeErr("list pending")
	}
	return d.list(ctx, d.Keys(scope).Pending)
}

func (d *CentralSchedulingDAO) list(ctx context.Context, key string) ([]*entity.CentralActiveRun, error) {
	members, err := d.cmdable.ZRange(ctx, key, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, errorx.Wrapf(err, "central registry: zrange failed")
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

func (d *CentralSchedulingDAO) CleanupPending(ctx context.Context, scope entity.CentralEnvScope, before int64) (int, error) {
	if !scope.IsValid() {
		return 0, invalidScopeErr("cleanup pending")
	}
	// pending zset 的 score 编码了优先级与提交序号，不能直接按时间清理；
	// 调用方按 run state 的 registered_at 判定超时后，用 RemovePending 精确移除。
	return 0, nil
}

// RemovePending 只移除 pending 集合中的成员，不影响 active 维护集合。
func (d *CentralSchedulingDAO) RemovePending(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) error {
	if !scope.IsValid() {
		return invalidScopeErr("remove pending")
	}
	members, err := d.cmdable.ZRange(ctx, d.Keys(scope).Pending, 0, -1).Result()
	if err != nil && !redis.IsNilError(err) {
		return errorx.Wrapf(err, "central pending: zrange failed")
	}
	for _, m := range members {
		_, gotExptID, gotRunID, _, _, ok := entity.ParseCentralActiveMember(m)
		if !ok || gotExptID != exptID || gotRunID != runID {
			continue
		}
		if _, err := d.cmdable.Eval(ctx, centralUnregisterActiveScript, []string{d.Keys(scope).Pending}, m).Result(); err != nil {
			return errorx.Wrapf(err, "central pending: zrem failed, member=%s", m)
		}
	}
	return nil
}

// RescoreRun 只更新排序 score（用于 aging 抬升），不改成员里冻结的 base priority。
func (d *CentralSchedulingDAO) RescoreRun(ctx context.Context, scope entity.CentralEnvScope, run *entity.CentralActiveRun, effectivePriority int, pending bool) error {
	if !scope.IsValid() {
		return invalidScopeErr("rescore run")
	}
	if run == nil {
		return errorx.New("central rescore: invalid run")
	}
	key := d.Keys(scope).Active
	if pending {
		key = d.Keys(scope).Pending
	}
	member := entity.MakeCentralActiveMember(run.SpaceID, run.ExptID, run.ExptRunID, run.Priority, run.SessionUserID)
	if err := d.cmdable.ZAdd(ctx, key, redislib.Z{Score: entity.CentralPriorityScore(effectivePriority, run.Seq), Member: member}).Err(); err != nil {
		return errorx.Wrapf(err, "central rescore: zadd failed")
	}
	return nil
}

func (d *CentralSchedulingDAO) NextAttempt(ctx context.Context, scope entity.CentralEnvScope, exptID, runID, itemID int64, ttl time.Duration) (int64, error) {
	if !scope.IsValid() {
		return 0, invalidScopeErr("next attempt")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	key := d.Keys(scope).Attempt + ":" + fmt.Sprintf("%d:%d", exptID, runID)
	res, err := d.cmdable.Eval(ctx, centralNextAttemptScript, []string{key},
		strconv.FormatInt(itemID, 10), int64(ttl/time.Second)).Result()
	if err != nil {
		return 0, errorx.Wrapf(err, "central attempt: eval failed")
	}
	return toInt64(res), nil
}

// GetLeaseState 返回单笔预占的当前唯一状态；不存在返回 false。
func (d *CentralSchedulingDAO) GetLeaseState(ctx context.Context, scope entity.CentralEnvScope, leaseID string) (entity.CentralLeaseState, bool, error) {
	if !scope.IsValid() {
		return "", false, invalidScopeErr("lease state")
	}
	raw, err := d.cmdable.HGet(ctx, d.Keys(scope).Lease(leaseID), "state").Result()
	if err != nil {
		if redis.IsNilError(err) {
			return "", false, nil
		}
		return "", false, errorx.Wrapf(err, "central lease state: hget failed, lease=%s", leaseID)
	}
	return entity.CentralLeaseState(raw), true, nil
}

func (d *CentralSchedulingDAO) CountRunLeases(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) (int, error) {
	leases, err := d.ListLeases(ctx, scope)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, lease := range leases {
		if lease != nil && lease.ExptID == exptID && lease.ExptRunID == runID {
			count++
		}
	}
	return count, nil
}

// RecordHealthyDegradedTick 记录一个连续正常 tick；用于连续 N tick 后自动解除降级。
func (d *CentralSchedulingDAO) RecordHealthyDegradedTick(ctx context.Context, scope entity.CentralEnvScope) (int, error) {
	if !scope.IsValid() {
		return 0, invalidScopeErr("degrade healthy tick")
	}
	key := d.Keys(scope).DegradeHealthy
	value, err := d.cmdable.Incr(ctx, key).Result()
	if err != nil {
		return 0, errorx.Wrapf(err, "central degrade: incr healthy tick failed")
	}
	if err := d.cmdable.Expire(ctx, key, time.Hour).Err(); err != nil {
		return int(value), errorx.Wrapf(err, "central degrade: expire healthy tick failed")
	}
	return int(value), nil
}

// ResetHealthyDegradedTicks 在出现异常 tick 时清零连续正常计数。
func (d *CentralSchedulingDAO) ResetHealthyDegradedTicks(ctx context.Context, scope entity.CentralEnvScope) error {
	if !scope.IsValid() {
		return invalidScopeErr("reset degrade ticks")
	}
	if err := d.cmdable.Del(ctx, d.Keys(scope).DegradeHealthy).Err(); err != nil && !redis.IsNilError(err) {
		return errorx.Wrapf(err, "central degrade: reset healthy tick failed")
	}
	return nil
}

// MarkDegradedRecovered 标记降级已恢复，并刷新短 TTL；无正常 tick 时自动失效。
func (d *CentralSchedulingDAO) MarkDegradedRecovered(ctx context.Context, scope entity.CentralEnvScope, ttl time.Duration) error {
	if !scope.IsValid() {
		return invalidScopeErr("mark degrade recovered")
	}
	if ttl <= 0 {
		ttl = 3 * time.Minute
	}
	if err := d.cmdable.Set(ctx, d.Keys(scope).DegradeRecovered, "1", ttl).Err(); err != nil {
		return errorx.Wrapf(err, "central degrade: mark recovered failed")
	}
	return nil
}

// IsDegradedRecovered 返回连续正常 tick 达标后的恢复标记。
func (d *CentralSchedulingDAO) IsDegradedRecovered(ctx context.Context, scope entity.CentralEnvScope) (bool, error) {
	if !scope.IsValid() {
		return false, invalidScopeErr("degrade recovered")
	}
	n, err := d.cmdable.Exists(ctx, d.Keys(scope).DegradeRecovered).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central degrade: exists failed")
	}
	return n > 0, nil
}

func (d *CentralSchedulingDAO) AcquireLeader(ctx context.Context, scope entity.CentralEnvScope, owner string, ttl time.Duration) (bool, error) {
	if !scope.IsValid() {
		return false, invalidScopeErr("leader")
	}
	if ttl <= 0 {
		ttl = 20 * time.Second
	}
	res, err := d.cmdable.Eval(ctx, centralLeaderScript, []string{d.Keys(scope).Leader}, owner, int64(ttl/time.Millisecond)).Result()
	if err != nil && !redis.IsNilError(err) {
		return false, errorx.Wrapf(err, "central leader: eval failed")
	}
	return toInt64(res) == 1, nil
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

func strValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func mustParseInt64(raw string) int64 {
	v, _ := strconv.ParseInt(raw, 10, 64)
	return v
}
