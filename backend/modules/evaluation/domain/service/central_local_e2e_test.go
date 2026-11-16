// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	loopredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	centralredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/central_scheduling/redis"
)

func startE2ERedis(t *testing.T) (string, func()) {
	t.Helper()
	port := 6600 + (os.Getpid() % 150)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	dir, err := os.MkdirTemp("", "central-e2e-*")
	require.NoError(t, err)
	cmd := exec.Command("redis-server", "--port", fmt.Sprintf("%d", port), "--save", "", "--appendonly", "no", "--dir", dir)
	require.NoError(t, cmd.Start())
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, derr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if derr == nil {
			_ = c.Close()
			return addr, func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait(); _ = os.RemoveAll(dir) }
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	_ = os.RemoveAll(dir)
	t.Fatal("redis-server did not become ready")
	return "", nil
}

// TestCentralLocalE2E 用真实 Redis 账本跑中心链主路径：活跃集重建、扫描、预占、确认、释放、
// 重启接管、环境隔离，以及中心/legacy 实验并存时不误派发 legacy。
func TestCentralLocalE2E(t *testing.T) {
	addr, stop := startE2ERedis(t)
	defer stop()
	ctx := context.Background()

	opts := &goredis.Options{Addr: addr}
	cmd, err := loopredis.NewClient(opts)
	require.NoError(t, err)
	raw := goredis.NewClient(opts)
	defer func() { _ = raw.Close() }()
	require.NoError(t, raw.FlushDB(ctx).Err())

	cfg := entity.DefaultCentralSchedulingConfig()
	cfg.Enabled = true
	cfg.MaxInflightPerExpt = 4
	cfg.MaxDispatchPerTick = 4
	cfgProvider := &fakeCfgProvider{cfg: cfg, enabled: true, caps: map[entity.ResourceDimension]int64{
		entity.ResourceItemExec: 10, entity.ResourceSandboxExec: 10, entity.ResourceModelCredit: 100,
	}}
	envID := "ppe:central-local"
	env := fakeEnvProvider{env: envID}
	active := newFakeActive()
	ledger := centralredis.NewLedger(cmd)
	exptRepo := newFakeExptRepo()
	dispatcher := &fakeDispatcher{}
	s := NewCentralScheduler(cfgProvider, env, ledger, active, exptRepo, nil, dispatcher)

	centralExpt, centralActive := makeActiveExpt(101, 1001, entity.SchedulingModeCentral, 50, 1, envID)
	concur := 4
	centralExpt.EvalConf.ItemConcurNum = &concur
	legacyExpt, legacyActive := makeActiveExpt(102, 1002, entity.SchedulingModeLegacy, 100, 0, envID)
	exptRepo.expts[101] = centralExpt
	exptRepo.expts[102] = legacyExpt
	exptRepo.queue[101] = []int64{11, 12, 13, 14, 15, 16}
	exptRepo.queue[102] = []int64{21, 22}
	require.NoError(t, active.AddActive(ctx, envID, centralActive, time.Hour))
	require.NoError(t, active.AddActive(ctx, envID, legacyActive, time.Hour))

	require.NoError(t, s.Rebuild(ctx))
	t.Logf("E2E_STEP restart_rebuild active=%d", len(active.items))
	require.NoError(t, s.Tick(ctx))

	require.Len(t, dispatcher.events, 4, "central pool capacity must cap first tick")
	seen := map[int64]bool{}
	for _, ev := range dispatcher.events {
		require.NotEmpty(t, ev.CentralReservationID)
		require.Equal(t, envID, ev.CentralEnv)
		require.NotEqual(t, int64(102), ev.ExptID, "legacy experiment must not be dispatched by central scheduler")
		seen[ev.EvalSetItemID] = true
	}
	require.NotContains(t, seen, int64(21))
	require.NotContains(t, seen, int64(22))
	usage, err := ledger.PoolUsage(ctx, envID)
	require.NoError(t, err)
	require.Equal(t, int64(4), usage[entity.ResourceItemExec])
	t.Logf("E2E_STEP tick_reserve dispatched=%d item_exec_used=%d", len(dispatcher.events), usage[entity.ResourceItemExec])

	resvID := dispatcher.events[0].CentralReservationID
	state, err := ledger.Confirm(ctx, envID, resvID)
	require.NoError(t, err)
	require.Equal(t, entity.ReservationStateConfirmed, state)
	state, err = ledger.Release(ctx, envID, resvID, "item_success")
	require.NoError(t, err)
	require.Equal(t, entity.ReservationStateReleased, state)
	usage, err = ledger.PoolUsage(ctx, envID)
	require.NoError(t, err)
	require.Equal(t, int64(3), usage[entity.ResourceItemExec])
	t.Logf("E2E_STEP confirm_release resv=%s state=%s item_exec_used=%d", resvID, state, usage[entity.ResourceItemExec])

	// 模拟 Pod 重启：新 scheduler 实例从活跃集与账本恢复，不得重复预占已存在的 item。
	dispatcher2 := &fakeDispatcher{}
	s2 := NewCentralScheduler(cfgProvider, env, ledger, active, exptRepo, nil, dispatcher2)
	require.NoError(t, s2.Rebuild(ctx))
	left, err := ledger.ListByRun(ctx, envID, 101, 1001)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(left), 3)
	require.NoError(t, s2.Tick(ctx))
	for _, ev := range dispatcher2.events {
		require.False(t, ev.EvalSetItemID == dispatcher.events[1].EvalSetItemID || ev.EvalSetItemID == dispatcher.events[2].EvalSetItemID || ev.EvalSetItemID == dispatcher.events[3].EvalSetItemID,
			"restart tick must not duplicate an existing reservation")
	}
	t.Logf("E2E_STEP restart_adopt existing_reservations=%d new_dispatches=%d", len(left), len(dispatcher2.events))

	// 环境隔离：另一环境的账本与活跃集不可见。
	otherUsage, err := ledger.PoolUsage(ctx, "ppe:other")
	require.NoError(t, err)
	require.Zero(t, otherUsage[entity.ResourceItemExec])
	listOther, err := ledger.ListAll(ctx, "ppe:other", 10)
	require.NoError(t, err)
	require.Empty(t, listOther)
	t.Logf("E2E_STEP env_isolation other_usage=%d other_reservations=%d", otherUsage[entity.ResourceItemExec], len(listOther))

	// 清理本环境 ledger key，保证测试自回收。
	require.NoError(t, ledger.CleanupEnv(ctx, envID))
	keys, _ := raw.Keys(ctx, "qn:central:*").Result()
	for _, k := range keys {
		require.NoError(t, raw.Del(ctx, k).Err())
	}
	t.Logf("E2E_STEP cleanup remaining_keys=%d", len(keys))
}

