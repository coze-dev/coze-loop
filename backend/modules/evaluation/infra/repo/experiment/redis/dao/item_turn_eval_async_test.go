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

	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestEvalAsyncDAO_GetStrongAndMarkResumeReady(t *testing.T) {
	t.Parallel()
	cmdable := infraredis.NewTestRedis(t)
	dao := NewEvalAsyncDAO(cmdable)
	ctx := context.Background()

	go func() {
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, dao.SetEvalAsyncCtx(ctx, "evaluator:1", &entity.EvalAsyncCtx{RecordID: 1, ResumeBarrierEnabled: true}))
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
	dao := NewEvalAsyncDAO(infraredis.NewTestRedis(t))
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
	dao := NewEvalAsyncDAO(cmdable)
	ctx := context.Background()
	const largeID int64 = 9007199254740993
	original := &entity.EvalAsyncCtx{
		ResumeBarrierEnabled: true,
		RecordID:             largeID,
		EvaluatorVersionID:   largeID - 1,
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

func TestEvalAsyncDAO_MarkResumeReadyDoesNotArmLegacyContext(t *testing.T) {
	cmdable := infraredis.NewTestRedis(t)
	dao := NewEvalAsyncDAO(cmdable)
	ctx := context.Background()

	require.NoError(t, dao.SetEvalAsyncCtx(ctx, "legacy", &entity.EvalAsyncCtx{RecordID: 1}))
	got, err := dao.MarkEvalAsyncResumeReady(ctx, "legacy")
	require.NoError(t, err)
	assert.False(t, got.ResumeBarrierEnabled)
	assert.False(t, got.ResumeReady)
	assert.True(t, got.CanResumeExperiment())
}
