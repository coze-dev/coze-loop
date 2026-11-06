// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func itemLockBehaviorRedis(t *testing.T) infraredis.Cmdable {
	t.Helper()
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	if addr != "" {
		client, err := infraredis.NewClient(&redis.Options{Addr: addr})
		require.NoError(t, err)
		raw, _ := infraredis.Unwrap(client)
		t.Cleanup(func() { _ = raw.Close() })
		t.Logf("REAL_REDIS_EXECUTED addr=%s", addr)
		return client
	}
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	return client
}

// A duplicate delivery that loses the item mutex must not start a second run.
// Without a primary-source reader it also has no proof of completion, so the
// consumer keeps the native broker retry instead of silently ACKing work.
func TestItemLockBusyKeepsSingleRunOnRealLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl) // no publish on the duplicate
	locker := lock.NewRedisLocker(itemLockBehaviorRedis(t))
	svc := &ExptItemEventEvalServiceImpl{
		itemLocker: func() lock.ILocker { return locker },
		publisher:  publisher,
	}
	base := time.Now().UnixNano()
	event := &entity.ExptItemEvalEvent{SpaceID: base, ExptID: base + 1, ExptRunID: base + 2, EvalSetItemID: base + 3}

	var runs atomic.Int32
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	handler := svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
		runs.Add(1)
		entered <- struct{}{}
		<-release
		return nil
	})

	firstDone := make(chan error, 1)
	go func() { firstDone <- handler(context.Background(), event) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first delivery never acquired the item lock")
	}

	// Duplicate delivery while the first execution still holds the lock.
	require.True(t, errno.NeedMQRetry(handler(context.Background(), event)), "busy must stay retryable when completion cannot be proven")
	require.Equal(t, int32(1), runs.Load(), "duplicate must not start a second run")

	close(release)
	require.NoError(t, <-firstDone)
	require.Equal(t, int32(1), runs.Load())
}
