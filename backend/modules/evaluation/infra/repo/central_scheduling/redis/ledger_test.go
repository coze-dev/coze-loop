// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	loopredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

var testRedisAddr string

func TestMain(m *testing.M) {
	addr, stop, err := startRedis()
	if err != nil {
		fmt.Fprintf(os.Stderr, "start redis: %v\n", err)
		os.Exit(1)
	}
	testRedisAddr = addr
	code := m.Run()
	stop()
	os.Exit(code)
}

func startRedis() (string, func(), error) {
	port := 6400 + (os.Getpid() % 200)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	dir, err := os.MkdirTemp("", "central-redis-*")
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command("redis-server", "--port", fmt.Sprintf("%d", port), "--save", "", "--appendonly", "no", "--dir", dir)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, derr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if derr == nil {
			_ = conn.Close()
			stop := func() {
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
				os.RemoveAll(dir)
			}
			return addr, stop, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	os.RemoveAll(dir)
	return "", nil, errors.New("redis-server did not become ready")
}

func newTestCmd(t *testing.T) (loopredis.Cmdable, *goredis.Client) {
	t.Helper()
	opts := &goredis.Options{Addr: testRedisAddr}
	cmd, err := loopredis.NewClient(opts)
	require.NoError(t, err)
	raw := goredis.NewClient(opts)
	t.Cleanup(func() { _ = raw.Close() })
	return cmd, raw
}

func reserveReq(t *testing.T, id string, env string, itemID int64, vector entity.ResourceVector, enabled map[entity.ResourceDimension]bool, cap map[entity.ResourceDimension]int64) *repo.ReserveRequest {
	t.Helper()
	return &repo.ReserveRequest{
		Reservation: &entity.Reservation{
			ID: id, Env: env, SpaceID: 11, ExptID: 22, RunID: 33, ItemID: itemID,
			Attempt: 44, Vector: vector, Owner: "ut", CreatedAt: entity.NowUnixMS(),
		},
		Enabled: enabled, Capacity: cap, TTL: time.Hour,
	}
}

func TestReserveIsAllOrNothingAndIdempotent(t *testing.T) {
	ctx := context.Background()
	cmd, _ := newTestCmd(t)
	ledger := NewLedger(cmd)
	env := "ut:env-a"
	all := map[entity.ResourceDimension]bool{entity.ResourceItemExec: true, entity.ResourceSandboxExec: true}
	caps := map[entity.ResourceDimension]int64{entity.ResourceItemExec: 10, entity.ResourceSandboxExec: 1}

	// 容量不足的第二个向量必须整体失败，item_exec 不能被部分占用。
	_, err := ledger.Reserve(ctx, reserveReq(t, "r1", env, 100, entity.ResourceVector{entity.ResourceItemExec: 1, entity.ResourceSandboxExec: 2}, all, caps))
	require.Error(t, err)
	usage, err := ledger.PoolUsage(ctx, env)
	require.NoError(t, err)
	assert.Zero(t, usage.Get(entity.ResourceItemExec))
	assert.Zero(t, usage.Get(entity.ResourceSandboxExec))

	// 成功预占后重复预占同一 item 必须幂等拒绝，且不能二次扣减。
	_, err = ledger.Reserve(ctx, reserveReq(t, "r2", env, 101, entity.ResourceVector{entity.ResourceItemExec: 1, entity.ResourceSandboxExec: 1}, all, caps))
	require.NoError(t, err)
	_, err = ledger.Reserve(ctx, reserveReq(t, "r3", env, 101, entity.ResourceVector{entity.ResourceItemExec: 1}, all, caps))
	require.ErrorIs(t, err, repo.ErrReservationExists)
	usage, err = ledger.PoolUsage(ctx, env)
	require.NoError(t, err)
	assert.Equal(t, int64(1), usage.Get(entity.ResourceItemExec))
	assert.Equal(t, int64(1), usage.Get(entity.ResourceSandboxExec))

	// 释放幂等：第二次释放不重复返还容量。
	state, err := ledger.Release(ctx, env, "r2", "ut_release")
	require.NoError(t, err)
	assert.Equal(t, entity.ReservationStateReleased, state)
	state, err = ledger.Release(ctx, env, "r2", "ut_release_again")
	require.NoError(t, err)
	assert.Equal(t, entity.ReservationStateReleased, state)
	usage, err = ledger.PoolUsage(ctx, env)
	require.NoError(t, err)
	assert.Zero(t, usage.Get(entity.ResourceItemExec))
	assert.Zero(t, usage.Get(entity.ResourceSandboxExec))
}

func TestConfirmThenReleaseAndEnvIsolation(t *testing.T) {
	ctx := context.Background()
	cmd, _ := newTestCmd(t)
	ledger := NewLedger(cmd)
	all := map[entity.ResourceDimension]bool{entity.ResourceItemExec: true}
	caps := map[entity.ResourceDimension]int64{entity.ResourceItemExec: 10}

	_, err := ledger.Reserve(ctx, reserveReq(t, "r1", "env-a", 1, entity.ResourceVector{entity.ResourceItemExec: 1}, all, caps))
	require.NoError(t, err)
	st, err := ledger.Confirm(ctx, "env-a", "r1")
	require.NoError(t, err)
	assert.Equal(t, entity.ReservationStateConfirmed, st)
	st, err = ledger.Release(ctx, "env-a", "r1", "done")
	require.NoError(t, err)
	assert.Equal(t, entity.ReservationStateReleased, st)
	// 已释放记录不允许再次 confirm。
	st, err = ledger.Confirm(ctx, "env-a", "r1")
	require.NoError(t, err)
	assert.Equal(t, entity.ReservationStateReleased, st)

	// 跨 env 不可见、互不影响。
	_, err = ledger.Reserve(ctx, reserveReq(t, "r-env-b", "env-b", 1, entity.ResourceVector{entity.ResourceItemExec: 1}, all, caps))
	require.NoError(t, err)
	got, err := ledger.Get(ctx, "env-a", "r-env-b")
	require.NoError(t, err)
	assert.Nil(t, got)
	usageA, err := ledger.PoolUsage(ctx, "env-a")
	require.NoError(t, err)
	assert.Zero(t, usageA.Get(entity.ResourceItemExec))
	usageB, err := ledger.PoolUsage(ctx, "env-b")
	require.NoError(t, err)
	assert.Equal(t, int64(1), usageB.Get(entity.ResourceItemExec))
}

func TestKeysUseCentralPrefixAndTTL(t *testing.T) {
	ctx := context.Background()
	cmd, raw := newTestCmd(t)
	ledger := NewLedger(cmd)
	// 清空测试库，仅用于本测试的本地实例。
	require.NoError(t, raw.FlushDB(ctx).Err())

	_, err := ledger.Reserve(ctx, reserveReq(t, "rttl", "ut:env", 7, entity.ResourceVector{entity.ResourceItemExec: 1},
		map[entity.ResourceDimension]bool{entity.ResourceItemExec: true},
		map[entity.ResourceDimension]int64{entity.ResourceItemExec: 10}))
	require.NoError(t, err)

	keys, err := raw.Keys(ctx, "qn:central:*").Result()
	require.NoError(t, err)
	require.NotEmpty(t, keys)
	for _, k := range keys {
		assert.Equal(t, "qn:central:", k[:len("qn:central:")], "unexpected key prefix: %s", k)
		ttl := raw.TTL(ctx, k).Val()
		assert.Greater(t, ttl, time.Duration(0), "key %s should have TTL", k)
	}
}
