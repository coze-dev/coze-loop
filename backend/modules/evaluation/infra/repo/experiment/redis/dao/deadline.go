// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
)

const (
	exptDeadlineZSetKey     = "expt:deadline"
	exptDeadlineClaimPrefix = "expt:deadline:claim:"
)

// IExptDeadlineDAO 到点调度 ZSET，member 格式 expt:{spaceID}:{exptID}:{runID}，score=deadlineTs
type IExptDeadlineDAO interface {
	AddDeadline(ctx context.Context, spaceID, exptID, runID int64, deadlineTs int64) error
	ScanDue(ctx context.Context, nowUnix int64, limit int) ([]string, error)
	TryClaim(ctx context.Context, member string, ttl time.Duration) (bool, error)
	Remove(ctx context.Context, member string) error
}

func NewExptDeadlineDAO(cmdable redis.Cmdable) IExptDeadlineDAO {
	return &exptDeadlineDAOImpl{cmdable: cmdable}
}

type exptDeadlineDAOImpl struct {
	cmdable redis.Cmdable
}

func (d *exptDeadlineDAOImpl) member(spaceID, exptID, runID int64) string {
	return fmt.Sprintf("expt:%d:%d:%d", spaceID, exptID, runID)
}

func (d *exptDeadlineDAOImpl) AddDeadline(ctx context.Context, spaceID, exptID, runID int64, deadlineTs int64) error {
	member := d.member(spaceID, exptID, runID)
	_, err := d.cmdable.ZAddNX(ctx, exptDeadlineZSetKey, goredis.Z{Score: float64(deadlineTs), Member: member}).Result()
	return err
}

func (d *exptDeadlineDAOImpl) ScanDue(ctx context.Context, nowUnix int64, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	opt := &goredis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(nowUnix, 10),
		Offset: 0,
		Count:  int64(limit),
	}
	return d.cmdable.ZRangeByScore(ctx, exptDeadlineZSetKey, opt).Result()
}

func (d *exptDeadlineDAOImpl) claimKey(member string) string {
	return exptDeadlineClaimPrefix + member
}

func (d *exptDeadlineDAOImpl) TryClaim(ctx context.Context, member string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return d.cmdable.SetNX(ctx, d.claimKey(member), "1", ttl).Result()
}

func (d *exptDeadlineDAOImpl) Remove(ctx context.Context, member string) error {
	_, err := d.cmdable.ZRem(ctx, exptDeadlineZSetKey, member).Result()
	return err
}
