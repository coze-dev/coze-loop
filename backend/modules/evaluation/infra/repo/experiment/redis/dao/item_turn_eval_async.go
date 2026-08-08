// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/convert"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	redisv9 "github.com/redis/go-redis/v9"
)

type IEvalAsyncDAO interface {
	SetEvalAsyncCtx(ctx context.Context, invokeID string, actx *entity.EvalAsyncCtx) error
	GetEvalAsyncCtx(ctx context.Context, invokeID string) (*entity.EvalAsyncCtx, error)
	GetEvalAsyncCtxStrong(ctx context.Context, invokeID string) (*entity.EvalAsyncCtx, error)
	MarkEvalAsyncResumeReady(ctx context.Context, invokeID string) (*entity.EvalAsyncCtx, error)
}

func NewEvalAsyncDAO(cmdable redis.Cmdable) IEvalAsyncDAO {
	const table = "experiment"
	return &evalAsyncDAOImpl{cmdable: cmdable, table: table}
}

type evalAsyncDAOImpl struct {
	cmdable redis.Cmdable
	table   string
}

func (e *evalAsyncDAOImpl) makeExptItemTurnEvalAsyncCtxKey(invokeID string) string {
	return fmt.Sprintf("[%s]item_turn_eval_async_ctx:%s", e.table, invokeID)
}

func (e *evalAsyncDAOImpl) SetEvalAsyncCtx(ctx context.Context, invokeID string, actx *entity.EvalAsyncCtx) error {
	bytes, err := convert.NewExptItemTurnEvalAsyncCtx().FromDO(actx)
	if err != nil {
		return err
	}
	key := e.makeExptItemTurnEvalAsyncCtxKey(invokeID)
	if err := e.cmdable.Set(ctx, key, bytes, time.Hour*12).Err(); err != nil {
		return errorx.Wrapf(err, "redis set key: %v", key)
	}
	return nil
}

func (e *evalAsyncDAOImpl) GetEvalAsyncCtx(ctx context.Context, invokeID string) (*entity.EvalAsyncCtx, error) {
	return e.getEvalAsyncCtx(ctx, invokeID)
}

func (e *evalAsyncDAOImpl) GetEvalAsyncCtxStrong(ctx context.Context, invokeID string) (*entity.EvalAsyncCtx, error) {
	var lastErr error
	delays := []time.Duration{0, 50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}
	for _, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		actx, err := e.getEvalAsyncCtxFromPrimary(ctx, invokeID)
		if err == nil || !redis.IsNilError(err) {
			return actx, err
		}
		lastErr = err
	}
	if redis.IsNilError(lastErr) {
		return nil, errorx.New("eval async context not found after bounded retry, invoke_id: %s", invokeID)
	}
	return nil, lastErr
}

const markResumeReadyScript = `
local current = redis.call('GET', KEYS[1])
if not current then return -1 end
if string.find(current, '"resume_ready":true', 1, true) then return 0 end
local updated, count = string.gsub(current, '"resume_ready":false', '"resume_ready":true', 1)
if count == 0 then
  updated, count = string.gsub(current, '^{', '{"resume_ready":true,', 1)
end
if count == 0 then return -2 end
redis.call('SET', KEYS[1], updated, 'EX', ARGV[1])
return 1
`

func (e *evalAsyncDAOImpl) MarkEvalAsyncResumeReady(ctx context.Context, invokeID string) (*entity.EvalAsyncCtx, error) {
	actx, err := e.GetEvalAsyncCtxStrong(ctx, invokeID)
	if err != nil {
		return nil, err
	}
	if actx == nil || !actx.ResumeBarrierEnabled || actx.ResumeReady {
		return actx, nil
	}
	key := e.makeExptItemTurnEvalAsyncCtxKey(invokeID)
	// Keep the raw JSON intact instead of decoding with Redis cjson (Lua numbers cannot safely represent i64 IDs).
	// resume_ready is a root-level field emitted by Go's JSON encoder, so an anchored string replacement/insertion is safe.

	updated, evalErr := e.cmdable.Eval(ctx, markResumeReadyScript, []string{key}, int64((12*time.Hour)/time.Second)).Int64()
	if evalErr != nil {
		return nil, errorx.Wrapf(evalErr, "redis mark resume ready fail, key: %v", key)
	}
	if updated < 0 {
		return nil, errorx.New("mark eval async resume ready failed, invoke_id: %s, code: %d", invokeID, updated)
	}
	actx, err = e.GetEvalAsyncCtxStrong(ctx, invokeID)
	if err != nil {
		return nil, err
	}
	return actx, nil
}

// Redis-v6 routes EVAL/EVALSHA by the script key and only routes commands whose
// command metadata is ReadOnly to replicas. EVAL is therefore master-routed even
// though this script only performs GET. This gives callback/arm reads the latest
// context without changing the shared client's read priority for unrelated traffic.
const getEvalAsyncCtxFromPrimaryScript = `return redis.call('GET', KEYS[1])`

func (e *evalAsyncDAOImpl) getEvalAsyncCtxFromPrimary(ctx context.Context, invokeID string) (*entity.EvalAsyncCtx, error) {
	key := e.makeExptItemTurnEvalAsyncCtxKey(invokeID)
	got, err := e.cmdable.Eval(ctx, getEvalAsyncCtxFromPrimaryScript, []string{key}).Text()
	if err != nil {
		if redis.IsNilError(err) || err == redisv9.Nil {
			return nil, redisv9.Nil
		}
		return nil, errorx.Wrapf(err, "redis primary get fail, key: %v", key)
	}
	return convert.NewExptItemTurnEvalAsyncCtx().ToDO(conv.UnsafeStringToBytes(got))
}

func (e *evalAsyncDAOImpl) getEvalAsyncCtx(ctx context.Context, invokeID string) (*entity.EvalAsyncCtx, error) {
	key := e.makeExptItemTurnEvalAsyncCtxKey(invokeID)
	got, err := e.cmdable.Get(ctx, key).Result()
	if err != nil {
		// Preserve redis.Nil so the strong-read path can apply bounded retry.
		if redis.IsNilError(err) {
			return nil, err
		}
		return nil, errorx.Wrapf(err, "redis get fail, key: %v", key)
	}
	return convert.NewExptItemTurnEvalAsyncCtx().ToDO(conv.UnsafeStringToBytes(got))
}
