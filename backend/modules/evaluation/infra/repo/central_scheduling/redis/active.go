// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	loopredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type ActiveRepo struct {
	cmd loopredis.Cmdable
}

func NewActiveRepo(cmd loopredis.Cmdable) *ActiveRepo { return &ActiveRepo{cmd: cmd} }

func (a *ActiveRepo) activeKey(env string) string {
	return "qn:central:{" + normalizeEnv(env) + "}:active"
}
func (a *ActiveRepo) metaKey(env string) string {
	return "qn:central:{" + normalizeEnv(env) + "}:active:meta"
}
func (a *ActiveRepo) leaderKey(env string) string {
	return "qn:central:{" + normalizeEnv(env) + "}:leader"
}
func (a *ActiveRepo) exptLeaseKey(env string, exptID int64) string {
	return fmt.Sprintf("qn:central:{%s}:exptlease:%d", normalizeEnv(env), exptID)
}

func member(exptID, runID int64) string { return fmt.Sprintf("%d:%d", exptID, runID) }

func metaValue(exp *entity.ActiveExperiment) string {
	return fmt.Sprintf("%d:%d", int64(exp.RunMode), exp.Attempt)
}

func parseMetaValue(v string) (entity.ExptRunMode, int64) {
	parts := strings.SplitN(v, ":", 2)
	mode, _ := strconv.ParseInt(parts[0], 10, 64)
	attempt := int64(0)
	if len(parts) == 2 {
		attempt, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	return entity.ExptRunMode(mode), attempt
}

func parseMember(s string) (int64, int64, bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	e, err1 := strconv.ParseInt(parts[0], 10, 64)
	r, err2 := strconv.ParseInt(parts[1], 10, 64)
	return e, r, err1 == nil && err2 == nil
}

func (a *ActiveRepo) AddActive(ctx context.Context, env string, exp *entity.ActiveExperiment, ttl time.Duration) error {
	key := a.activeKey(env)
	score := float64(exp.WaitSinceMS)
	if score <= 0 {
		score = float64(entity.NowUnixMS())
	}
	m := member(exp.ExptID, exp.RunID)
	if err := a.cmd.ZAdd(ctx, key, redis.Z{Score: score, Member: m}).Err(); err != nil {
		return err
	}
	if err := a.cmd.HSet(ctx, a.metaKey(env), m, metaValue(exp)).Err(); err != nil {
		return err
	}
	if err := a.cmd.Expire(ctx, a.metaKey(env), ttl).Err(); err != nil {
		return err
	}
	return a.cmd.Expire(ctx, key, ttl).Err()
}

func (a *ActiveRepo) RemoveActive(ctx context.Context, env string, exptID, runID int64) error {
	m := member(exptID, runID)
	_ = a.cmd.HDel(ctx, a.metaKey(env), m).Err()
	return a.cmd.ZRem(ctx, a.activeKey(env), m).Err()
}

func (a *ActiveRepo) ListActive(ctx context.Context, env string, limit int) ([]*entity.ActiveExperiment, error) {
	if limit <= 0 {
		limit = 10000
	}
	members, err := a.cmd.ZRange(ctx, a.activeKey(env), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*entity.ActiveExperiment, 0, len(members))
	for _, m := range members {
		e, r, ok := parseMember(m)
		if !ok {
			continue
		}
		raw, _ := a.cmd.HGet(ctx, a.metaKey(env), m).Result()
		mode, attempt := parseMetaValue(raw)
		out = append(out, &entity.ActiveExperiment{ExptID: e, RunID: r, RunMode: mode, Attempt: attempt})
	}
	return out, nil
}

const renewScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]))
  return 1
end
return 0
`

func (a *ActiveRepo) TryLeader(ctx context.Context, env, owner string, ttl time.Duration) (bool, error) {
	return a.cmd.SetNX(ctx, a.leaderKey(env), owner, ttl).Result()
}

func (a *ActiveRepo) RenewLeader(ctx context.Context, env, owner string, ttl time.Duration) (bool, error) {
	res, err := a.cmd.Eval(ctx, renewScript, []string{a.leaderKey(env)}, owner, int64(ttl/time.Second)).Result()
	if err != nil {
		return false, err
	}
	n, _ := res.(int64)
	return n == 1, nil
}

func (a *ActiveRepo) ReleaseLeader(ctx context.Context, env, owner string) error {
	// 条件删除，避免误删新 leader。
	if v, err := a.cmd.Get(ctx, a.leaderKey(env)).Result(); err == nil && v == owner {
		return a.cmd.Del(ctx, a.leaderKey(env)).Err()
	}
	return nil
}

func (a *ActiveRepo) TryExptLease(ctx context.Context, env string, exptID int64, owner string, ttl time.Duration) (bool, error) {
	return a.cmd.SetNX(ctx, a.exptLeaseKey(env, exptID), owner, ttl).Result()
}

func (a *ActiveRepo) CleanupEnv(ctx context.Context, env string) error {
	return a.cmd.Del(ctx, a.activeKey(env), a.metaKey(env), a.leaderKey(env)).Err()
}
