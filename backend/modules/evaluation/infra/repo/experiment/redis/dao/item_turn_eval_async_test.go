// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"testing"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestEvalAsyncDAO_GetStrongAndMarkResumeReady(t *testing.T) {
	t.Parallel()
	cmdable := infraredis.NewTestRedis(t)
	dao := NewEvalAsyncDAO(cmdable, nil)
	ctx := context.Background()

	go func() {
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, dao.SetEvalAsyncCtx(ctx, "evaluator:1", &entity.EvalAsyncCtx{RecordID: 1}))
	}()

	actx, err := dao.GetEvalAsyncCtxStrong(ctx, "evaluator:1")
	require.NoError(t, err)
	require.NotNil(t, actx)
	assert.False(t, actx.ResumeReady)

	actx, err = dao.MarkEvalAsyncResumeReady(ctx, "evaluator:1")
	require.NoError(t, err)
	assert.True(t, actx.ResumeReady)

	stored, err := dao.GetEvalAsyncCtx(ctx, "evaluator:1")
	require.NoError(t, err)
	assert.True(t, stored.ResumeReady)
}

func TestEvalAsyncDAO_GetStrongStopsAfterBoundedRetry(t *testing.T) {
	t.Parallel()
	dao := NewEvalAsyncDAO(infraredis.NewTestRedis(t), nil)
	start := time.Now()
	actx, err := dao.GetEvalAsyncCtxStrong(context.Background(), "missing")
	assert.Nil(t, actx)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, redisv9.Nil)
	assert.Contains(t, err.Error(), "eval async context not found")
	assert.GreaterOrEqual(t, time.Since(start), 350*time.Millisecond)
	assert.Less(t, time.Since(start), time.Second)
}

func TestEvalAsyncDAO_MarkResumeReadyPreservesLargeIDsAndExistingPayload(t *testing.T) {
	t.Parallel()
	cmdable := infraredis.NewTestRedis(t)
	dao := NewEvalAsyncDAO(cmdable, nil)
	ctx := context.Background()
	const largeID int64 = 9007199254740993
	original := &entity.EvalAsyncCtx{
		RecordID:           largeID,
		EvaluatorVersionID: largeID - 1,
		Event: &entity.ExptItemEvalEvent{
			ExptID:        largeID - 2,
			ExptRunID:     largeID - 3,
			EvalSetItemID: largeID - 4,
		},
	}
	require.NoError(t, dao.SetEvalAsyncCtx(ctx, "evaluator:large", original))

	got, err := dao.MarkEvalAsyncResumeReady(ctx, "evaluator:large")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.ResumeReady)
	assert.Equal(t, largeID, got.RecordID)
	assert.Equal(t, largeID-1, got.EvaluatorVersionID)
	require.NotNil(t, got.Event)
	assert.Equal(t, largeID-2, got.Event.ExptID)
	assert.Equal(t, largeID-3, got.Event.ExptRunID)
	assert.Equal(t, largeID-4, got.Event.EvalSetItemID)
}

func TestMarkResumeReadyScriptAvoidsVersionSensitiveTTLCommands(t *testing.T) {
	t.Parallel()
	assert.NotContains(t, markResumeReadyScript, "KEEPTTL")
	assert.NotContains(t, markResumeReadyScript, "PTTL")
	assert.Contains(t, markResumeReadyScript, "'EX', ARGV[1]")
}

// TTL 必须真的按空间配置写进 redis：光有 entity 层取值逻辑不够，
// 这里断言 SetEvalAsyncCtx 与 MarkEvalAsyncResumeReady 两条写路径都用了配置值
// （后者是 SET ... EX，会重置 TTL，历史上是第二处 12h 硬编码）。
func TestEvalAsyncDAO_TTLFollowsConfiger(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const spaceID int64 = 7590110994886651906
	wantTTL := 24*time.Hour + 30*time.Minute

	configer := mocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetEvalAsyncCtxTTL(gomock.Any(), spaceID).Return(wantTTL).AnyTimes()

	cmdable := infraredis.NewTestRedis(t)
	dao := NewEvalAsyncDAO(cmdable, configer)
	ctx := context.Background()

	actx := &entity.EvalAsyncCtx{RecordID: 1, Event: &entity.ExptItemEvalEvent{SpaceID: spaceID}}
	require.NoError(t, dao.SetEvalAsyncCtx(ctx, "target:ttl", actx))

	ttl := ttlOfKey(t, cmdable, "[experiment]item_turn_eval_async_ctx:target:ttl")
	assert.Greater(t, ttl, wantTTL-time.Minute)
	assert.LessOrEqual(t, ttl, wantTTL)

	// resume ready 重写后 TTL 不应被打回 12h
	_, err := dao.MarkEvalAsyncResumeReady(ctx, "target:ttl")
	require.NoError(t, err)

	ttl = ttlOfKey(t, cmdable, "[experiment]item_turn_eval_async_ctx:target:ttl")
	assert.Greater(t, ttl, wantTTL-time.Minute)
}

// 调试链路 (actx.Event == nil) 没有空间上下文，按 spaceID=0 读全局默认。
func TestEvalAsyncDAO_TTLWithoutSpaceContext(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	configer := mocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetEvalAsyncCtxTTL(gomock.Any(), int64(0)).Return(12 * time.Hour).AnyTimes()

	cmdable := infraredis.NewTestRedis(t)
	dao := NewEvalAsyncDAO(cmdable, configer)
	ctx := context.Background()

	require.NoError(t, dao.SetEvalAsyncCtx(ctx, "debug:ttl", &entity.EvalAsyncCtx{RecordID: 2}))

	ttl := ttlOfKey(t, cmdable, "[experiment]item_turn_eval_async_ctx:debug:ttl")
	assert.Greater(t, ttl, 12*time.Hour-time.Minute)
	assert.LessOrEqual(t, ttl, 12*time.Hour)
}

// configer 不可用时兜底回 12h，与改造前硬编码行为一致。
func TestEvalAsyncDAO_TTLFallbackWithoutConfiger(t *testing.T) {
	t.Parallel()
	cmdable := infraredis.NewTestRedis(t)
	dao := NewEvalAsyncDAO(cmdable, nil)
	ctx := context.Background()

	require.NoError(t, dao.SetEvalAsyncCtx(ctx, "fallback:ttl", &entity.EvalAsyncCtx{RecordID: 3}))

	ttl := ttlOfKey(t, cmdable, "[experiment]item_turn_eval_async_ctx:fallback:ttl")
	assert.Greater(t, ttl, 12*time.Hour-time.Minute)
	assert.LessOrEqual(t, ttl, 12*time.Hour)
}

// ttlOfKey 读 key 的剩余 TTL。Cmdable 没暴露 TTL 命令，从底层 client 取。
func ttlOfKey(t *testing.T, cmdable infraredis.Cmdable, key string) time.Duration {
	t.Helper()
	raw, ok := infraredis.Unwrap(cmdable)
	require.True(t, ok, "unwrap redis client")
	ttl, err := raw.TTL(context.Background(), key).Result()
	require.NoError(t, err)
	return ttl
}
