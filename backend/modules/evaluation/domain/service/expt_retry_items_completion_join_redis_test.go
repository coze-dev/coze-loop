// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

var realRedisJoinSeq atomic.Int64

// TestLegacyRetryJoinCompletingMutexRealRedis exercises the production Redis
// locker for the exact key LogRetryItemsRun and ExptRetryItemsExec.ExptEnd
// share. It proves:
//   - the join holds the completing key while AppendRunItemIDs runs;
//   - a join that finds the key taken returns ExperimentIsCompletingCode and
//     leaves the holder's key untouched;
//   - the real LockCompletingRun used by ExptEnd blocks the join until released.
func TestLegacyRetryJoinCompletingMutexRealRedis(t *testing.T) {
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	if addr == "" {
		t.Skip("EXPT_RETRY_REAL_REDIS_ADDR missing; REAL REDIS NOT EXECUTED")
	}
	client, err := infraredis.NewClient(&goredis.Options{Addr: addr})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	info, err := raw.Info(context.Background(), "server").Result()
	require.NoError(t, err)
	require.Contains(t, info, "redis_version:")
	version := ""
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "redis_version:") {
			version = strings.TrimPrefix(line, "redis_version:")
			break
		}
	}
	t.Logf("REAL_REDIS_EXECUTED addr=%s version=%s", addr, version)
	t.Cleanup(func() { require.NoError(t, raw.Close()) })

	t.Run("holds_key_across_append_then_releases", func(t *testing.T) {
		h := newRealRedisJoinHarness(t, client, raw)
		var heldDuringAppend bool
		h.repo.onAppend = func() {
			n, err := raw.Exists(context.Background(), h.complete).Result()
			require.NoError(t, err)
			heldDuringAppend = n == 1
		}
		runID, retried, err := h.manager.LogRetryItemsRun(
			context.Background(), h.exptID, entity.EvaluationModeRetryItems, h.spaceID,
			[]int64{h.itemB}, &entity.Session{UserID: "u"})
		require.NoError(t, err)
		require.True(t, retried)
		require.Equal(t, h.runID, runID)
		require.True(t, heldDuringAppend, "join must hold the completing key while appending")

		n, err := raw.Exists(context.Background(), h.complete).Result()
		require.NoError(t, err)
		require.Zero(t, n, "join must release the completing key it acquired")
		holder, err := raw.Get(context.Background(), h.manager.makeExptMutexLockKey(h.exptID)).Result()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprint(h.runID), holder, "join must not touch the live run's experiment lock")
	})

	t.Run("busy_completion_rejects_join_without_stealing_key", func(t *testing.T) {
		h := newRealRedisJoinHarness(t, client, raw)
		require.NoError(t, raw.Set(context.Background(), h.complete, "completer", time.Minute).Err())
		_, retried, err := h.manager.LogRetryItemsRun(
			context.Background(), h.exptID, entity.EvaluationModeRetryItems, h.spaceID,
			[]int64{h.itemB}, &entity.Session{UserID: "u"})
		require.Error(t, err)
		require.False(t, retried)
		statusErr, ok := errorx.FromStatusError(err)
		require.True(t, ok)
		require.Equal(t, int32(errno.ExperimentIsCompletingCode), statusErr.Code())
		require.Zero(t, h.repo.appendCalls)

		holder, err := raw.Get(context.Background(), h.complete).Result()
		require.NoError(t, err)
		require.Equal(t, "completer", holder, "busy join must not release a lock it did not acquire")
	})

	t.Run("real_completion_lock_blocks_join_until_released", func(t *testing.T) {
		h := newRealRedisJoinHarness(t, client, raw)
		require.NoError(t, h.manager.LockCompletingRun(
			context.Background(), h.exptID, h.runID, h.spaceID, &entity.Session{UserID: "completer"}))

		_, _, err := h.manager.LogRetryItemsRun(
			context.Background(), h.exptID, entity.EvaluationModeRetryItems, h.spaceID,
			[]int64{h.itemB}, &entity.Session{UserID: "u"})
		require.Error(t, err)
		statusErr, ok := errorx.FromStatusError(err)
		require.True(t, ok)
		require.Equal(t, int32(errno.ExperimentIsCompletingCode), statusErr.Code())
		require.Zero(t, h.repo.appendCalls)

		require.NoError(t, h.manager.UnlockCompletingRun(
			context.Background(), h.exptID, h.runID, h.spaceID, &entity.Session{UserID: "completer"}))
		runID, retried, err := h.manager.LogRetryItemsRun(
			context.Background(), h.exptID, entity.EvaluationModeRetryItems, h.spaceID,
			[]int64{h.itemB}, &entity.Session{UserID: "u"})
		require.NoError(t, err)
		require.True(t, retried)
		require.Equal(t, h.runID, runID)
		require.Equal(t, 1, h.repo.appendCalls)
	})
}

func newRealRedisJoinHarness(t *testing.T, client infraredis.Cmdable, raw *goredis.Client) *retryJoinRaceHarness {
	t.Helper()
	h := newRetryJoinRaceHarness(t)
	base := int64(7300000000000000000) + realRedisJoinSeq.Add(1)*100
	h.exptID, h.runID = base+1, base+2
	h.complete = makeCompletingLockKeyForTest(h.exptID, h.runID)
	h.event.ExptID, h.event.ExptRunID = h.exptID, h.runID
	h.expt.ID, h.expt.LatestRunID = h.exptID, h.runID
	h.manager.mutex = lock.NewRedisLocker(client)

	lifecycleKey := h.manager.makeExptMutexLockKey(h.exptID)
	require.NoError(t, raw.Set(context.Background(), lifecycleKey, fmt.Sprint(h.runID), time.Minute).Err())
	t.Cleanup(func() {
		_, _ = raw.Del(context.Background(), lifecycleKey, h.complete).Result()
	})
	return h
}
