// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func centralOwnershipKey(exptID int64) string {
	return entity.CentralKeyPrefix + "owner:expt:" + strconv.FormatInt(exptID, 10)
}

// claimOwnershipScript never steals a foreign owner. It renews when the same
// owner asks again, which makes claim idempotent across every tick.
const claimOwnershipScript = `
local cur = redis.call('GET', KEYS[1])
if not cur then
  redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
  return {1, ARGV[1]}
end
if cur == ARGV[1] then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
  return {1, cur}
end
return {0, cur}
`

func NewCentralOwnershipStore(cmdable redis.Cmdable) repo.CentralOwnershipStore {
	return &centralOwnershipStore{cmdable: cmdable}
}

type centralOwnershipStore struct {
	cmdable redis.Cmdable
}

func (s *centralOwnershipStore) Claim(ctx context.Context, exptID, runID int64, owner string, ttl time.Duration) (bool, string, error) {
	if ttl <= 0 {
		ttl = entity.CentralDefaultOwnershipTTL
	}
	got, err := s.cmdable.Eval(ctx, claimOwnershipScript,
		[]string{centralOwnershipKey(exptID)}, owner, int64(ttl/time.Millisecond)).Result()
	if err != nil {
		return false, "", errorx.Wrapf(err, "central ownership: claim")
	}
	values, ok := got.([]interface{})
	if !ok || len(values) != 2 {
		return false, "", errorx.New("central ownership: unexpected reply %T", got)
	}
	code, _ := values[0].(int64)
	return code == 1, toString(values[1]), nil
}

func (s *centralOwnershipStore) Owner(ctx context.Context, exptID int64) (string, error) {
	key := centralOwnershipKey(exptID)
	owner, err := s.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return "", nil
		}
		return "", errorx.Wrapf(err, "central ownership: get %s", key)
	}
	// Reading ownership is a passive heartbeat: keep a long run pinned.
	_ = s.cmdable.Expire(ctx, key, entity.CentralDefaultOwnershipTTL).Err()
	return owner, nil
}

const releaseOwnershipScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`

func (s *centralOwnershipStore) Release(ctx context.Context, exptID int64, owner string) error {
	if owner == "" {
		return nil
	}
	if _, err := s.cmdable.Eval(ctx, releaseOwnershipScript, []string{centralOwnershipKey(exptID)}, owner).Int64(); err != nil {
		return errorx.Wrapf(err, "central ownership: release")
	}
	return nil
}

// NewRedisCentralLease provides single-scanner exclusion via SET NX PX.
func NewRedisCentralLease(cmdable redis.Cmdable) *RedisCentralLease {
	return &RedisCentralLease{cmdable: cmdable}
}

type RedisCentralLease struct {
	cmdable redis.Cmdable
	holder  string
}

func (l *RedisCentralLease) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = entity.CentralDefaultTickInterval
	}
	holder := fmt.Sprintf("%s:%d", "central", time.Now().UnixNano())
	ok, err := l.cmdable.SetNX(ctx, key, holder, ttl).Result()
	if err != nil {
		return false, errorx.Wrapf(err, "central lease: acquire")
	}
	if ok {
		l.holder = holder
	}
	return ok, nil
}

const releaseLeaseScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`

func (l *RedisCentralLease) Release(ctx context.Context, key string) error {
	if l.holder == "" {
		return nil
	}
	if _, err := l.cmdable.Eval(ctx, releaseLeaseScript, []string{key}, l.holder).Int64(); err != nil {
		logs.CtxWarn(ctx, "[CentralScheduling] release lease %s failed: %v", key, err)
	}
	l.holder = ""
	return nil
}
