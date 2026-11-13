// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package central_schedule 提供基于 Redis + Lua 的额度账本实现。
//
// 设计要点：
//   - 所有 key 使用同一 hash tag {cs:<scope_hash>}，保证 Redis Cluster 下
//     Lua 可原子访问同一 slot；
//   - 预占记录是权威账本，MySQL 只做投影；
//   - ReserveBatch 在一个脚本里校验并增加所有维度，任一不足零变更；
//   - generation 双缓冲：重建写影子代，校验旧代未变化后一次原子切换，
//     重建期间正常派发继续使用旧代。
package central_schedule

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	infraRedis "github.com/coze-dev/coze-loop/backend/infra/redis"
	domain "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central_schedule"
)

type Ledger struct {
	cmdable infraRedis.Cmdable
	clock   domain.Clock
}

func NewLedger(cmdable infraRedis.Cmdable) *Ledger {
	return &Ledger{cmdable: cmdable, clock: domain.NewRealClock()}
}

func NewLedgerWithClock(cmdable infraRedis.Cmdable, clock domain.Clock) *Ledger {
	return &Ledger{cmdable: cmdable, clock: clock}
}

func (l *Ledger) now() time.Time {
	if l.clock == nil {
		return time.Now()
	}
	return l.clock.Now()
}

func scopeHash(scope string) string {
	sum := sha1.Sum([]byte(scope))
	return hex.EncodeToString(sum[:])[:16]
}

func (l *Ledger) tag(scope string) string {
	if scope == "" {
		scope = "<missing>"
	}
	return "{cs:" + scopeHash(scope) + "}"
}

func (l *Ledger) metaKey(scope string, runID int64) string {
	return fmt.Sprintf("%s:run:%d:meta", l.tag(scope), runID)
}
func (l *Ledger) usageKey(scope string, runID int64, gen int64) string {
	return fmt.Sprintf("%s:run:%d:usage:%d", l.tag(scope), runID, gen%2)
}
func (l *Ledger) resSetKey(scope string, runID int64) string {
	return fmt.Sprintf("%s:run:%d:res", l.tag(scope), runID)
}
func (l *Ledger) resKey(scope string, runID int64, rid string) string {
	return fmt.Sprintf("%s:run:%d:res:%s", l.tag(scope), runID, rid)
}
func (l *Ledger) claimKey(scope string, runID, itemID int64, attempt int32) string {
	return fmt.Sprintf("%s:run:%d:claim:%d:%d", l.tag(scope), runID, itemID, attempt)
}
func (l *Ledger) activeKey(scope string) string { return l.tag(scope) + ":active" }
func (l *Ledger) leaderKey(scope string) string { return l.tag(scope) + ":leader" }
func (l *Ledger) rebuildLockKey(scope string, runID int64) string {
	return fmt.Sprintf("%s:run:%d:rebuild_lock", l.tag(scope), runID)
}

var initScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then return 0 end
redis.call('HSET', KEYS[1], 'scope', ARGV[1], 'expt_id', ARGV[2], 'run_id', ARGV[3],
  'mode', ARGV[4], 'priority', ARGV[5], 'target', ARGV[6], 'created_at', ARGV[7],
  'generation', '0')
for i = 8, #ARGV, 2 do
  redis.call('HSET', KEYS[1], 'limit:' .. ARGV[i], ARGV[i+1])
  redis.call('HSET', KEYS[2], ARGV[i], 0)
  redis.call('HSET', KEYS[3], ARGV[i], 0)
end
redis.call('ZADD', KEYS[5], ARGV[7], ARGV[3])
return 1
`

func (l *Ledger) InitRun(ctx context.Context, account *domain.RunAccount) error {
	if account == nil || account.OwnerScope == "" {
		return domain.ErrOwnerScopeMissing
	}
	args := []any{
		account.OwnerScope, account.ExptID, account.RunID, int32(account.Mode), account.Priority,
		account.TargetConcurrency, l.now().Unix(),
	}
	for k, v := range account.DimensionLimits {
		args = append(args, k, v)
	}
	// 保证 item 维度存在。
	if _, ok := account.DimensionLimits["item"]; !ok {
		args = append(args, "item", account.TargetConcurrency)
	}
	err := l.cmdable.Eval(ctx, initScript, []string{
		l.metaKey(account.OwnerScope, account.RunID),
		l.usageKey(account.OwnerScope, account.RunID, 0),
		l.usageKey(account.OwnerScope, account.RunID, 1),
		l.resSetKey(account.OwnerScope, account.RunID),
		l.activeKey(account.OwnerScope),
	}, args...).Err()
	if err != nil {
		return fmt.Errorf("%w: init run: %v", domain.ErrQuotaUnavailable, err)
	}
	// run -> scope 索引独立 slot，单独写入；重复调用幂等。
	if ierr := l.cmdable.HSet(ctx, l.runIndexKey(account.RunID), "scope", account.OwnerScope).Err(); ierr != nil {
		return fmt.Errorf("%w: init run index: %v", domain.ErrQuotaUnavailable, ierr)
	}
	return nil
}

var reserveBatchScript = `
local n = tonumber(ARGV[1])
local gen = tonumber(redis.call('HGET', KEYS[1], 'generation') or '0')
local ukey = (gen % 2 == 0) and KEYS[2] or KEYS[3]
local pos = 2
local parsed = {}
for i = 1, n do
  local rid = ARGV[pos]
  local dedup = ARGV[pos+1]
  local expt = ARGV[pos+2]
  local run = ARGV[pos+3]
  local item = ARGV[pos+4]
  local attempt = ARGV[pos+5]
  local created = ARGV[pos+6]
  local lease = ARGV[pos+7]
  local dc = tonumber(ARGV[pos+8])
  local dims = {}
  for j = 1, dc do
    local dim = ARGV[pos+9+(j-1)*3]
    local amt = tonumber(ARGV[pos+10+(j-1)*3])
    local lim = tonumber(ARGV[pos+11+(j-1)*3])
    dims[j] = {dim, amt, lim}
  end
  parsed[i] = {rid, dedup, expt, run, item, attempt, created, lease, dims}
  pos = pos + 9 + dc * 3
end

-- 先校验：任一维度累计不足、任一 res 已存在 -> 零变更返回。
local pending = {}
for i = 1, n do
  local r = parsed[i]
  if redis.call('HEXISTS', KEYS[4+i], 'state') == 1 then return {0, 'exists'} end
  for j = 1, #r[9] do
    local d = r[9][j]
    if d[3] >= 0 then
      local key = d[1]
      local used = tonumber(redis.call('HGET', ukey, key) or '0')
      local total = (pending[key] or 0) + d[2]
      if used + total > d[3] then return {0, key} end
      pending[key] = total
    else
      pending[d[1]] = (pending[d[1]] or 0) + d[2]
    end
  end
end

-- 再统一落账。
for i = 1, n do
  local r = parsed[i]
  for j = 1, #r[9] do
    local d = r[9][j]
    redis.call('HINCRBY', ukey, d[1], d[2])
    redis.call('HSET', KEYS[4+i], 'd:' .. d[1], d[2])
  end
  redis.call('HSET', KEYS[4+i],
    'rid', r[1], 'dedup', r[2], 'expt_id', r[3], 'run_id', r[4],
    'item_id', r[5], 'attempt', r[6], 'state', 'reserved',
    'created_at', r[7], 'updated_at', r[7],
    'lease_expire_at', r[8], 'generation', gen, 'version', '1')
  redis.call('SADD', KEYS[4], r[1])
end
return {1, 'ok'}
`

// ReserveBatch 一次 Lua 调用占齐整批 item 的所有维度；任一不足返回 ShortfallError。
func (l *Ledger) ReserveBatch(ctx context.Context, account *domain.RunAccount, reservations []*domain.Reservation) error {
	if account == nil || account.OwnerScope == "" {
		return domain.ErrOwnerScopeMissing
	}
	if len(reservations) == 0 {
		return nil
	}
	keys := []string{
		l.metaKey(account.OwnerScope, account.RunID),
		l.usageKey(account.OwnerScope, account.RunID, 0),
		l.usageKey(account.OwnerScope, account.RunID, 1),
		l.resSetKey(account.OwnerScope, account.RunID),
	}
	args := []any{len(reservations)}
	type dimReq struct {
		dim    string
		amount int64
		limit  int64
	}
	reqs := make([]map[string]dimReq, 0, len(reservations))
	for _, res := range reservations {
		if res == nil {
			return fmt.Errorf("central_schedule: nil reservation in batch")
		}
		if res.ID == "" {
			return fmt.Errorf("central_schedule: reservation id is required")
		}
		dims := domain.Dimensions(res.Dimensions)
		keys = append(keys, l.resKey(account.OwnerScope, account.RunID, res.ID))
		args = append(args,
			res.ID, res.DedupKey, res.ExptID, res.RunID, res.ItemID, res.Attempt,
			res.CreatedAt.Unix(), res.LeaseExpireAt.Unix(), len(dims))
		req := make(map[string]dimReq, len(dims))
		for _, d := range dims {
			limit := int64(-1)
			if v, ok := account.DimensionLimits[d.Key]; ok {
				limit = v
			}
			if d.Key == "item" {
				limit = account.TargetConcurrency
			}
			args = append(args, d.Key, d.Amount, limit)
			req[d.Key] = dimReq{dim: d.Key, amount: d.Amount, limit: limit}
		}
		reqs = append(reqs, req)
	}
	cmd := l.cmdable.Eval(ctx, reserveBatchScript, keys, args...)
	if cmd.Err() != nil {
		return fmt.Errorf("%w: reserve: %v", domain.ErrQuotaUnavailable, cmd.Err())
	}
	vals, err := cmd.Slice()
	if err != nil || len(vals) < 2 {
		return fmt.Errorf("%w: reserve result: %v", domain.ErrQuotaUnavailable, err)
	}
	ok, _ := vals[0].(int64)
	if ok != 1 {
		dim, _ := vals[1].(string)
		var requested int64
		var limit int64 = -1
		for _, req := range reqs {
			if d, found := req[dim]; found {
				requested += d.amount
				limit = d.limit
			}
		}
		available := int64(-1)
		if limit >= 0 {
			used, _ := l.cmdable.HGet(ctx, l.usageKey(account.OwnerScope, account.RunID, 0), dim).Int64()
			available = limit - used
		}
		return &domain.ShortfallError{Dimension: dim, Available: available, Requested: requested}
	}
	// 成功：回填状态/代次，并写 rid -> (scope,run) 索引（独立 slot，单独命令）。
	gen, _ := l.cmdable.HGet(ctx, l.metaKey(account.OwnerScope, account.RunID), "generation").Int64()
	for _, res := range reservations {
		res.State = domain.StateReserved
		res.Generation = gen
		if ierr := l.cmdable.HSet(ctx, l.resIndexKey(res.ID),
			"scope", account.OwnerScope, "run_id", strconv.FormatInt(res.RunID, 10)).Err(); ierr != nil {
			return fmt.Errorf("%w: reservation index: %v", domain.ErrQuotaUnavailable, ierr)
		}
	}
	return nil
}

func dimAmount(dims []domain.Dimension, key string) int64 {
	for _, d := range dims {
		if d.Key == key {
			return d.Amount
		}
	}
	return 0
}

var markDispatchingScript = `
local state = redis.call('HGET', KEYS[1], 'state')
if not state then return 0 end
if state == 'reserved' or state == 'uncertain' then
  redis.call('HSET', KEYS[1], 'state', 'dispatching', 'publish_at', ARGV[1], 'updated_at', ARGV[1])
  redis.call('HINCRBY', KEYS[1], 'version', 1)
  return 1
end
if state == 'dispatching' then return 2 end
if state == 'released' or state == 'expired' then return 3 end
return 0
`

func (l *Ledger) MarkDispatching(ctx context.Context, reservation *domain.Reservation) error {
	if reservation == nil {
		return domain.ErrReservationAbsent
	}
	cmd := l.cmdable.Eval(ctx, markDispatchingScript, []string{
		l.resKey(reservation.OwnerScope, reservation.RunID, reservation.ID),
	}, l.now().Unix())
	if cmd.Err() != nil {
		return fmt.Errorf("%w: mark dispatching: %v", domain.ErrQuotaUnavailable, cmd.Err())
	}
	code, _ := cmd.Int64()
	if code == 0 {
		return domain.ErrStateConflict
	}
	return nil
}

var ackScript = `
local state = redis.call('HGET', KEYS[1], 'state')
if not state then return 0 end
if state == 'reserved' or state == 'dispatching' or state == 'uncertain' then
  redis.call('HSET', KEYS[1], 'state', 'confirmed', 'ack_at', ARGV[1], 'updated_at', ARGV[1])
  redis.call('HINCRBY', KEYS[1], 'version', 1)
  return 1
end
if state == 'confirmed' then return 2 end
if state == 'released' or state == 'expired' then return 3 end
return 0
`

func (l *Ledger) Ack(ctx context.Context, reservationID string, at domain.UpdateTime) (domain.AckResult, error) {
	res, err := l.Get(ctx, reservationID)
	if err != nil {
		if err == domain.ErrReservationAbsent {
			return domain.AckMissing, nil
		}
		return domain.AckMissing, err
	}
	cmd := l.cmdable.Eval(ctx, ackScript, []string{
		l.resKey(res.OwnerScope, res.RunID, res.ID),
	}, at.Now.Unix())
	if cmd.Err() != nil {
		return domain.AckMissing, fmt.Errorf("%w: ack: %v", domain.ErrQuotaUnavailable, cmd.Err())
	}
	code, _ := cmd.Int64()
	return domain.AckResult(code), nil
}

var markUncertainScript = `
local state = redis.call('HGET', KEYS[1], 'state')
if not state then return 0 end
if state == 'reserved' or state == 'dispatching' then
  redis.call('HSET', KEYS[1], 'state', 'uncertain', 'updated_at', ARGV[1])
  redis.call('HINCRBY', KEYS[1], 'version', 1)
  return 1
end
if state == 'uncertain' then return 2 end
return 0
`

func (l *Ledger) MarkUncertain(ctx context.Context, reservationID string, at domain.UpdateTime) error {
	res, err := l.Get(ctx, reservationID)
	if err != nil {
		return err
	}
	cmd := l.cmdable.Eval(ctx, markUncertainScript, []string{
		l.resKey(res.OwnerScope, res.RunID, res.ID),
	}, at.Now.Unix())
	if cmd.Err() != nil {
		return fmt.Errorf("%w: mark uncertain: %v", domain.ErrQuotaUnavailable, cmd.Err())
	}
	code, _ := cmd.Int64()
	if code == 0 {
		return domain.ErrStateConflict
	}
	return nil
}

var releaseScript = `
local state = redis.call('HGET', KEYS[3], 'state')
if not state then return 0 end
if state == 'released' or state == 'expired' then return 2 end
local gen = tonumber(redis.call('HGET', KEYS[3], 'generation') or '0')
local ukey = (gen % 2 == 0) and KEYS[1] or KEYS[2]
local fields = redis.call('HKEYS', KEYS[3])
for i = 1, #fields do
  local f = fields[i]
  if string.sub(f, 1, 2) == 'd:' then
    local dim = string.sub(f, 3)
    local amt = tonumber(redis.call('HGET', KEYS[3], f) or '0')
    local newv = redis.call('HINCRBY', ukey, dim, -amt)
    if tonumber(newv) < 0 then redis.call('HSET', ukey, dim, 0) end
  end
end
redis.call('HSET', KEYS[3], 'state', 'released', 'released_at', ARGV[2], 'updated_at', ARGV[2])
redis.call('HINCRBY', KEYS[3], 'version', 1)
redis.call('SREM', KEYS[4], ARGV[1])
return 1
`

func (l *Ledger) Release(ctx context.Context, reservationID string, at domain.UpdateTime) (domain.ReleaseResult, error) {
	res, err := l.Get(ctx, reservationID)
	if err != nil {
		if err == domain.ErrReservationAbsent {
			return domain.ReleaseMissing, nil
		}
		return domain.ReleaseMissing, err
	}
	cmd := l.cmdable.Eval(ctx, releaseScript, []string{
		l.usageKey(res.OwnerScope, res.RunID, 0),
		l.usageKey(res.OwnerScope, res.RunID, 1),
		l.resKey(res.OwnerScope, res.RunID, res.ID),
		l.resSetKey(res.OwnerScope, res.RunID),
	}, reservationID, at.Now.Unix())
	if cmd.Err() != nil {
		return domain.ReleaseMissing, fmt.Errorf("%w: release: %v", domain.ErrQuotaUnavailable, cmd.Err())
	}
	code, _ := cmd.Int64()
	return domain.ReleaseResult(code), nil
}

func (l *Ledger) Get(ctx context.Context, reservationID string) (*domain.Reservation, error) {
	// reservationID 不含 owner/run 定位信息，按约定使用 scope/run 前缀从索引反查。
	// 为支持按 rid 直接读取，额外维护全局 rid -> (scope,run) 索引。
	loc, err := l.cmdable.HGetAll(ctx, l.resIndexKey(reservationID)).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: get reservation index: %v", domain.ErrQuotaUnavailable, err)
	}
	if len(loc) == 0 {
		return nil, domain.ErrReservationAbsent
	}
	scope := loc["scope"]
	runID, _ := strconv.ParseInt(loc["run_id"], 10, 64)
	raw, err := l.cmdable.HGetAll(ctx, l.resKey(scope, runID, reservationID)).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: get reservation: %v", domain.ErrQuotaUnavailable, err)
	}
	if len(raw) == 0 {
		return nil, domain.ErrReservationAbsent
	}
	return parseReservation(scope, runID, raw), nil
}

func (l *Ledger) resIndexKey(reservationID string) string {
	// 索引 key 必须与 ledger 同 slot；用 rid 的 hash 作为 tag 会分裂 slot。
	// 这里把索引放在 scope 级 key 中：调用方必须能提供 scope。为兼容 rid-only
	// 接口，使用固定 tag {cs:index:<rid_hash>}，仅索引读写，不参与额度原子脚本。
	return "{cs:index:" + scopeHash(reservationID) + "}:res_index"
}

var setIndexScript = `
redis.call('HSET', KEYS[1], 'scope', ARGV[2], 'run_id', ARGV[3])
return 1
`

func parseReservation(scope string, runID int64, raw map[string]string) *domain.Reservation {
	res := &domain.Reservation{
		ID:         raw["rid"],
		DedupKey:   raw["dedup"],
		OwnerScope: scope,
		RunID:      runID,
		State:      domain.ReservationState(raw["state"]),
		Generation: parseInt(raw["generation"]),
		Version:    parseInt(raw["version"]),
	}
	res.ExptID = parseInt(raw["expt_id"])
	res.ItemID = parseInt(raw["item_id"])
	res.Attempt = int32(parseInt(raw["attempt"]))
	if t := parseInt(raw["created_at"]); t > 0 {
		res.CreatedAt = time.Unix(t, 0)
	}
	if t := parseInt(raw["updated_at"]); t > 0 {
		res.UpdatedAt = time.Unix(t, 0)
	}
	if t := parseInt(raw["publish_at"]); t > 0 {
		v := time.Unix(t, 0)
		res.PublishAt = &v
	}
	if t := parseInt(raw["ack_at"]); t > 0 {
		v := time.Unix(t, 0)
		res.AckAt = &v
	}
	if t := parseInt(raw["lease_expire_at"]); t > 0 {
		res.LeaseExpireAt = time.Unix(t, 0)
	}
	for k, v := range raw {
		if strings.HasPrefix(k, "d:") {
			res.Dimensions = append(res.Dimensions, domain.Dimension{Key: strings.TrimPrefix(k, "d:"), Amount: parseInt(v)})
		}
	}
	sort.Slice(res.Dimensions, func(i, j int) bool { return res.Dimensions[i].Key < res.Dimensions[j].Key })
	return res
}

func parseInt(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func (l *Ledger) Usage(ctx context.Context, runID int64) (map[string]int64, error) {
	// runID 不足以定位 scope；调用方通过 RunAccount 初始化的 scope 由 meta 记录。
	// 这里扫描 active 索引找到 scope。实际高频路径由 Scheduler 传入 scope 更优；
	// 当前接口保留 runID-only，使用 active 索引兜底。
	scope, err := l.scopeForRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	gen, err := l.cmdable.HGet(ctx, l.metaKey(scope, runID), "generation").Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("%w: usage generation: %v", domain.ErrQuotaUnavailable, err)
	}
	raw, herr := l.cmdable.HGetAll(ctx, l.usageKey(scope, runID, gen)).Result()
	if herr != nil {
		return nil, fmt.Errorf("%w: usage: %v", domain.ErrQuotaUnavailable, herr)
	}
	out := make(map[string]int64, len(raw))
	for k, v := range raw {
		out[k] = parseInt(v)
	}
	return out, nil
}

func (l *Ledger) ListRunReservations(ctx context.Context, runID int64, state domain.ReservationState) ([]*domain.Reservation, error) {
	scope, err := l.scopeForRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	ids, err := l.cmdable.Eval(ctx, "return redis.call('SMEMBERS', KEYS[1])", []string{l.resSetKey(scope, runID)}).StringSlice()
	if err != nil {
		return nil, fmt.Errorf("%w: list reservations: %v", domain.ErrQuotaUnavailable, err)
	}
	out := make([]*domain.Reservation, 0, len(ids))
	for _, rid := range ids {
		raw, herr := l.cmdable.HGetAll(ctx, l.resKey(scope, runID, rid)).Result()
		if herr != nil || len(raw) == 0 {
			continue
		}
		res := parseReservation(scope, runID, raw)
		if state != "" && res.State != state {
			continue
		}
		out = append(out, res)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (l *Ledger) Claim(ctx context.Context, runID, itemID int64, attempt int32, reservationID string, ttlSec int64) (bool, error) {
	scope, err := l.scopeForRun(ctx, runID)
	if err != nil {
		return false, err
	}
	if ttlSec <= 0 {
		ttlSec = 120
	}
	ok, err := l.cmdable.SetNX(ctx, l.claimKey(scope, runID, itemID, attempt), reservationID, time.Duration(ttlSec)*time.Second).Result()
	if err != nil {
		return false, fmt.Errorf("%w: claim: %v", domain.ErrQuotaUnavailable, err)
	}
	return ok, nil
}

// ResolveReservation 用 claim key 反查派发凭证，执行侧确认只用这一个入口。
func (l *Ledger) ResolveReservation(ctx context.Context, runID, itemID int64, attempt int32) (string, error) {
	scope, err := l.scopeForRun(ctx, runID)
	if err != nil {
		if errors.Is(err, domain.ErrNotInitialized) {
			return "", domain.ErrReservationAbsent
		}
		return "", err
	}
	rid, err := l.cmdable.Get(ctx, l.claimKey(scope, runID, itemID, attempt)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", domain.ErrReservationAbsent
		}
		return "", fmt.Errorf("%w: resolve reservation: %v", domain.ErrQuotaUnavailable, err)
	}
	return rid, nil
}

func (l *Ledger) Rebuild(ctx context.Context, account *domain.RunAccount, expected map[string]int64, live []*domain.Reservation) (domain.RebuildResult, error) {
	if account == nil || account.OwnerScope == "" {
		return domain.RebuildResult{}, domain.ErrOwnerScopeMissing
	}
	scope, runID := account.OwnerScope, account.RunID
	meta := l.metaKey(scope, runID)
	// 只允许一个重建者；锁非常短，失败退化为下一轮。
	ok, err := l.cmdable.SetNX(ctx, l.rebuildLockKey(scope, runID), "1", 30*time.Second).Result()
	if err != nil || !ok {
		return domain.RebuildResult{}, nil
	}
	defer l.cmdable.Del(ctx, l.rebuildLockKey(scope, runID))

	oldGen, err := l.cmdable.HGet(ctx, meta, "generation").Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return domain.RebuildResult{}, fmt.Errorf("%w: rebuild generation: %v", domain.ErrQuotaUnavailable, err)
	}
	newGen := oldGen + 1
	oldUsage := l.usageKey(scope, runID, oldGen)
	newUsage := l.usageKey(scope, runID, newGen)

	// 最多重试 3 次：若最终切换时旧代发生变化（有新预占插入），重新从零构建。
	for attempt := 0; attempt < 3; attempt++ {
		// 重新读取 live，确保包含本轮新预占。
		current, lerr := l.ListRunReservations(ctx, runID, "")
		if lerr != nil {
			return domain.RebuildResult{}, lerr
		}
		expectedNow := make(map[string]int64, len(expected))
		for k, v := range expected {
			expectedNow[k] = v
		}
		resKeys := make([]string, 0, len(current))
		for _, res := range current {
			if res == nil || res.State.Terminal() {
				continue
			}
			resKeys = append(resKeys, l.resKey(scope, runID, res.ID))
			for _, d := range res.Dimensions {
				expectedNow[d.Key] += d.Amount
			}
		}
		// expected 是调用方基于 live 计算的；这里改用 current 的维度作为权威，
		// 避免“调用方 live 过期”导致双算。expected 仅用于 DB Processing 的补充。
		_ = expected

		if err := l.cmdable.Del(ctx, newUsage).Err(); err != nil {
			return domain.RebuildResult{}, fmt.Errorf("%w: rebuild del: %v", domain.ErrQuotaUnavailable, err)
		}
		if len(expectedNow) > 0 {
			args := make([]any, 0, len(expectedNow)*2)
			for k, v := range expectedNow {
				args = append(args, k, v)
			}
			if err := l.cmdable.HSet(ctx, newUsage, args...).Err(); err != nil {
				return domain.RebuildResult{}, fmt.Errorf("%w: rebuild hset: %v", domain.ErrQuotaUnavailable, err)
			}
		}

		// 记录旧代当前值，切换脚本会校验它们未变化。
		oldVals := make(map[string]int64, len(expectedNow))
		for k := range expectedNow {
			v, _ := l.cmdable.HGet(ctx, oldUsage, k).Int64()
			oldVals[k] = v
		}
		swapArgs := []any{newGen, len(expectedNow)}
		for k, v := range oldVals {
			swapArgs = append(swapArgs, k, v)
		}
		cmd := l.cmdable.Eval(ctx, rebuildSwapScript, append([]string{meta, oldUsage, newUsage}, resKeys...), swapArgs...)
		if cmd.Err() != nil {
			return domain.RebuildResult{}, fmt.Errorf("%w: rebuild swap: %v", domain.ErrQuotaUnavailable, cmd.Err())
		}
		code, _ := cmd.Int64()
		if code == 1 {
			_ = l.cmdable.Del(ctx, oldUsage).Err()
			return domain.RebuildResult{Swapped: true, Generation: newGen}, nil
		}
		// old gen 变化：重试。
	}
	return domain.RebuildResult{}, nil
}

const rebuildSwapScript = `
local newGen = ARGV[1]
local n = tonumber(ARGV[2])
for i = 0, n - 1 do
  local dim = ARGV[3 + i * 2]
  local old = ARGV[4 + i * 2]
  local cur = redis.call('HGET', KEYS[2], dim) or '0'
  if cur ~= old then return 0 end
end
for i = 4, #KEYS do
  redis.call('HSET', KEYS[i], 'generation', newGen)
end
redis.call('HSET', KEYS[1], 'generation', newGen)
return 1
`

// scopeForRun 通过 active 索引反查 scope。InitRun 会把 run 写入 scope 的 active zset；
// 若索引丢失，返回 ErrNotInitialized（由上层触发重建）。
func (l *Ledger) scopeForRun(ctx context.Context, runID int64) (string, error) {
	// rid-only 接口需要一个 run->scope 索引；InitRun 写入。
	raw, err := l.cmdable.HGetAll(ctx, l.runIndexKey(runID)).Result()
	if err != nil {
		return "", fmt.Errorf("%w: run index: %v", domain.ErrQuotaUnavailable, err)
	}
	if len(raw) == 0 || raw["scope"] == "" {
		return "", domain.ErrNotInitialized
	}
	return raw["scope"], nil
}

func (l *Ledger) runIndexKey(runID int64) string {
	return fmt.Sprintf("{cs:index:run:%d}:run_index", runID)
}

var setRunIndexScript = `
redis.call('HSET', KEYS[1], 'scope', ARGV[1])
return 1
`

// Leader 使用 scope 级 SET NX + TTL。
func (l *Ledger) Acquire(ctx context.Context, ownerScope string, instanceID string, ttl time.Duration) (bool, error) {
	if ownerScope == "" {
		return false, domain.ErrOwnerScopeMissing
	}
	if ttl <= 0 {
		ttl = 90 * time.Second
	}
	ok, err := l.cmdable.SetNX(ctx, l.leaderKey(ownerScope), instanceID, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("%w: leader: %v", domain.ErrQuotaUnavailable, err)
	}
	return ok, nil
}
