// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central_schedule_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infraRedis "github.com/coze-dev/coze-loop/backend/infra/redis"
	ledger "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/central_schedule"

	cs "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central_schedule"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

type fakeLeader struct {
	ok  bool
	err error
}

func (f *fakeLeader) Acquire(context.Context, string, string, time.Duration) (bool, error) {
	return f.ok, f.err
}

type fakeAdmission struct{ err error }

func (f *fakeAdmission) Check(context.Context, *cs.RunView) error { return f.err }

type fakeRuns struct{ runs []*cs.RunView }

func (f *fakeRuns) ScanActive(context.Context, string, int) ([]*cs.RunView, error) {
	return f.runs, nil
}

type fakeItems struct {
	queueing   map[int64][]*cs.ItemView
	status     map[int64]cs.ItemStatus
	processing map[int64][]*cs.ItemView
}

func (f *fakeItems) ScanQueueing(_ context.Context, run *cs.RunView, limit int) ([]*cs.ItemView, error) {
	items := f.queueing[run.RunID]
	if limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}

func (f *fakeItems) GetItemStatus(_ context.Context, _runID, itemID int64) (cs.ItemStatus, error) {
	return f.status[itemID], nil
}

func (f *fakeItems) ListProcessing(_ context.Context, run *cs.RunView) ([]*cs.ItemView, error) {
	return f.processing[run.RunID], nil
}

type fakeDispatcher struct {
	calls  []string
	items  []*cs.ItemView
	failID int64
	err    error
}

func (f *fakeDispatcher) Publish(_ context.Context, item *cs.ItemView, reservationID, _ string) error {
	f.calls = append(f.calls, reservationID)
	f.items = append(f.items, item)
	if f.err != nil {
		return f.err
	}
	if f.failID != 0 && item.ItemID == f.failID {
		return errors.New("publish failed")
	}
	return nil
}

type fakeProjection struct{ released []string }

func (f *fakeProjection) MarkProcessing(context.Context, *cs.ItemView, string) error { return nil }
func (f *fakeProjection) MarkReleased(_ context.Context, _runID, _itemID int64, rid string) error {
	f.released = append(f.released, rid)
	return nil
}

func account(runID, target int64) *cs.RunAccount {
	return &cs.RunAccount{
		OwnerScope:        "ppe:test:cn:stone.cozeloop.evaluation",
		ExptID:            100,
		RunID:             runID,
		Mode:              cs.ModeCentral,
		Priority:          1,
		TargetConcurrency: target,
		DimensionLimits:   map[string]int64{"item": target, "evaluator:1": target, "sandbox:t:sandbox": target},
	}
}

func dims() []cs.Dimension {
	return []cs.Dimension{
		{Key: "item", Amount: 1},
		{Key: "evaluator:1", Amount: 1},
	}
}

func newHarness(t *testing.T, runs []*cs.RunView, target int64) (*cs.Scheduler, *ledger.Ledger, *fakeItems, *fakeDispatcher, *fakeClock) {
	cli := infraRedis.NewTestRedis(t)
	clock := &fakeClock{t: time.Unix(1000, 0)}
	l := ledger.NewLedgerWithClock(cli, clock)
	for _, run := range runs {
		require.NoError(t, l.InitRun(context.Background(), account(run.RunID, target)))
	}
	items := &fakeItems{queueing: map[int64][]*cs.ItemView{}, status: map[int64]cs.ItemStatus{}, processing: map[int64][]*cs.ItemView{}}
	disp := &fakeDispatcher{}
	s := &cs.Scheduler{
		Leader:     &fakeLeader{ok: true},
		Runs:       &fakeRuns{runs: runs},
		Admission:  &fakeAdmission{},
		Items:      items,
		Ledger:     l,
		Dispatcher: disp,
		Projection: &fakeProjection{},
		Clock:      clock,
		InstanceID: "test-instance",
		Config: cs.Config{
			OwnerScope:        "ppe:test:cn:stone.cozeloop.evaluation",
			AckTimeout:        time.Second,
			CompensationLimit: 2,
			LeaseTTL:          time.Minute,
			BatchLimit:        100,
			RunLimit:          10,
		},
	}
	return s, l, items, disp, clock
}

func TestTickPriorityOrderAndNoOverfill(t *testing.T) {
	ctx := context.Background()
	runs := []*cs.RunView{
		{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, Priority: 5, TargetConcurrency: 1, DimensionLimits: map[string]int64{"item": 1, "evaluator:1": 1}, CreatedAt: time.Unix(100, 0)},
		{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 2, RunID: 2, Mode: cs.ModeCentral, Priority: 10, TargetConcurrency: 1, DimensionLimits: map[string]int64{"item": 1, "evaluator:1": 1}, CreatedAt: time.Unix(200, 0)},
		{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 3, RunID: 3, Mode: cs.ModeLegacy, Priority: 99, TargetConcurrency: 1, DimensionLimits: map[string]int64{"item": 1}, CreatedAt: time.Unix(50, 0)},
	}
	s, _, items, disp, _ := newHarness(t, runs, 1)
	items.queueing[1] = []*cs.ItemView{{RunID: 1, ItemID: 11, Dimensions: dims()}, {RunID: 1, ItemID: 12, Dimensions: dims()}}
	items.queueing[2] = []*cs.ItemView{{RunID: 2, ItemID: 21, Dimensions: dims()}}
	items.queueing[3] = []*cs.ItemView{{RunID: 3, ItemID: 31, Dimensions: dims()}}

	stats, err := s.Tick(ctx)
	require.NoError(t, err)
	// run 2 priority 10 先于 run 1 priority 5；legacy run 3 被排除。
	require.Len(t, disp.items, 2)
	require.Equal(t, int64(2), disp.items[0].RunID)
	require.Equal(t, int64(1), disp.items[1].RunID)
	require.Equal(t, 3, stats.ScannedRuns)
	require.Equal(t, 2, stats.Dispatched)
	require.Equal(t, 0, stats.Errors)
	// 每个 run target=1，第二 tick 不应再补。
	stats, err = s.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, stats.Dispatched)
}

func TestTickLeaderFalseNoDispatch(t *testing.T) {
	ctx := context.Background()
	run := &cs.RunView{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, TargetConcurrency: 1, DimensionLimits: map[string]int64{"item": 1}, CreatedAt: time.Unix(100, 0)}
	s, _, items, disp, _ := newHarness(t, []*cs.RunView{run}, 1)
	s.Leader = &fakeLeader{ok: false}
	items.queueing[1] = []*cs.ItemView{{RunID: 1, ItemID: 11, Dimensions: dims()}}
	stats, err := s.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, stats.Dispatched)
	require.Empty(t, disp.calls)
}

func TestTickPublishFailureReleasesQuota(t *testing.T) {
	ctx := context.Background()
	run := &cs.RunView{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, TargetConcurrency: 1, DimensionLimits: map[string]int64{"item": 1, "evaluator": 1}, CreatedAt: time.Unix(100, 0)}
	s, l, items, disp, _ := newHarness(t, []*cs.RunView{run}, 1)
	items.queueing[1] = []*cs.ItemView{{RunID: 1, ItemID: 11, Dimensions: dims()}}
	disp.err = errors.New("mq down")
	stats, err := s.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Errors)
	usage, err := l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(0), usage["item"])
	require.Equal(t, int64(0), usage["evaluator:1"])
}

func TestAckDuplicateAndLate(t *testing.T) {
	ctx := context.Background()
	run := &cs.RunView{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, TargetConcurrency: 1, DimensionLimits: map[string]int64{"item": 1, "evaluator": 1}, CreatedAt: time.Unix(100, 0)}
	s, l, items, _, clock := newHarness(t, []*cs.RunView{run}, 1)
	items.queueing[1] = []*cs.ItemView{{RunID: 1, ItemID: 11, Dimensions: dims()}}
	_, err := s.Tick(ctx)
	require.NoError(t, err)
	rid := ""
	live, err := l.ListRunReservations(ctx, 1, "")
	require.NoError(t, err)
	require.Len(t, live, 1)
	rid = live[0].ID

	ok, err := s.AckItem(ctx, rid, items.queueing[1][0])
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = s.AckItem(ctx, rid, items.queueing[1][0])
	require.NoError(t, err)
	require.False(t, ok)

	_, err = l.Release(ctx, rid, cs.UpdateTime{Now: clock.Now()})
	require.NoError(t, err)
	ok, err = s.AckItem(ctx, rid, items.queueing[1][0])
	require.NoError(t, err)
	require.False(t, ok)
}

func TestReconcilerTerminalReleaseAndOrphan(t *testing.T) {
	ctx := context.Background()
	run := &cs.RunView{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, TargetConcurrency: 4, DimensionLimits: map[string]int64{"item": 4, "evaluator:1": 4}, CreatedAt: time.Unix(100, 0)}
	s, l, items, _, clock := newHarness(t, []*cs.RunView{run}, 4)
	items.queueing[1] = []*cs.ItemView{{RunID: 1, ItemID: 11, Dimensions: dims()}, {RunID: 1, ItemID: 12, Dimensions: dims()}}
	_, err := s.Tick(ctx)
	require.NoError(t, err)

	// item 11 DB 终态 -> 增量释放。
	items.status[11] = cs.ItemSuccess
	// item 12 DB 仍 Queueing；把时钟推过 lease 到期 -> 孤儿释放。
	items.status[12] = cs.ItemQueueing
	clock.t = clock.t.Add(2 * time.Hour)
	rec := &cs.Reconciler{Runs: s.Runs, Items: items, Ledger: l, Projection: s.Projection, Clock: clock, Config: s.Config}
	stats, err := rec.Incremental(ctx, run, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.Released, 1)
	usage, err := l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(0), usage["item"])
}

func TestReconcilerFullRebuildNoDoubleCount(t *testing.T) {
	ctx := context.Background()
	run := &cs.RunView{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, TargetConcurrency: 4, DimensionLimits: map[string]int64{"item": 4, "evaluator:1": 4}, CreatedAt: time.Unix(100, 0)}
	s, l, items, _, _ := newHarness(t, []*cs.RunView{run}, 4)
	items.queueing[1] = []*cs.ItemView{{RunID: 1, ItemID: 11, Dimensions: dims()}, {RunID: 1, ItemID: 12, Dimensions: dims()}}
	_, err := s.Tick(ctx)
	require.NoError(t, err)
	usageBefore, err := l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), usageBefore["item"])

	rec := &cs.Reconciler{Runs: s.Runs, Items: items, Ledger: l, Projection: s.Projection, Clock: s.Clock, Config: s.Config}
	stats, err := rec.FullRebuild(ctx, s.Config.OwnerScope)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Rebuilt)
	usageAfter, err := l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, usageBefore["item"], usageAfter["item"], "rebuild must not double count")
	require.Equal(t, usageBefore["evaluator:1"], usageAfter["evaluator:1"])
}

// 执行侧按 run/item/attempt 反查确认；重复确认/释放后迟到确认都必须跳过。
func TestAckAndReleaseByItem(t *testing.T) {
	ctx := context.Background()
	run := &cs.RunView{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, TargetConcurrency: 2, DimensionLimits: map[string]int64{"item": 2, "evaluator:1": 2}, CreatedAt: time.Unix(100, 0)}
	s, l, items, _, _ := newHarness(t, []*cs.RunView{run}, 2)
	items.queueing[1] = []*cs.ItemView{
		{RunID: 1, ItemID: 11, Dimensions: dims()},
		{RunID: 1, ItemID: 12, Dimensions: dims()},
	}
	_, err := s.Tick(ctx)
	require.NoError(t, err)
	usage, err := l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), usage["item"])

	// 未派发的 item 没有 claim，视为缺失，不执行。
	ok, err := s.AckItemByItem(ctx, &cs.ItemView{RunID: 1, ItemID: 99, Attempt: 0})
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = s.AckItemByItem(ctx, &cs.ItemView{RunID: 1, ItemID: 11, Attempt: 0})
	require.NoError(t, err)
	require.True(t, ok)

	// 重复确认：跳过，不重复扣。
	ok, err = s.AckItemByItem(ctx, &cs.ItemView{RunID: 1, ItemID: 11, Attempt: 0})
	require.NoError(t, err)
	require.False(t, ok)

	// 单条释放后再确认：迟到确认，跳过。
	require.NoError(t, s.ReleaseItem(ctx, 1, 11, 0))
	ok, err = s.AckItemByItem(ctx, &cs.ItemView{RunID: 1, ItemID: 11, Attempt: 0})
	require.NoError(t, err)
	require.False(t, ok)

	usage, err = l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), usage["item"], "release only deducts the released item")

	// run 级释放归还剩余，重复调用幂等。
	released, err := s.ReleaseRun(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, released)
	released, err = s.ReleaseRun(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 0, released)
	usage, err = l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(0), usage["item"])
}

// DB 已无 Queueing/Processing 且额度归零时触发 RunDone；配置关闭时 tick 不扫描。
func TestRunDoneAndDisabled(t *testing.T) {
	ctx := context.Background()
	run := &cs.RunView{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, TargetConcurrency: 1, DimensionLimits: map[string]int64{"item": 1}, CreatedAt: time.Unix(100, 0)}
	s, _, _, _, _ := newHarness(t, []*cs.RunView{run}, 1)
	done := 0
	s.RunDone = func(context.Context, *cs.RunView) error { done++; return nil }
	stats, err := s.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, stats.Dispatched)
	require.Equal(t, 1, done, "empty run should be ended once per tick")

	s.ConfigProvider = func(context.Context) cs.Config { return cs.Config{Disabled: true} }
	done = 0
	stats, err = s.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, cs.TickStats{}, stats)
	require.Equal(t, 0, done)
}

// 周期全量重建在派发之后跑：本轮仍能正常派发，且重建不双算、不中断下一轮。
func TestPeriodicRebuildAfterDispatch(t *testing.T) {
	ctx := context.Background()
	run := &cs.RunView{OwnerScope: "ppe:test:cn:stone.cozeloop.evaluation", ExptID: 1, RunID: 1, Mode: cs.ModeCentral, TargetConcurrency: 2, DimensionLimits: map[string]int64{"item": 2, "evaluator:1": 2}, CreatedAt: time.Unix(100, 0)}
	s, l, items, _, _ := newHarness(t, []*cs.RunView{run}, 2)
	s.Reconciler = &cs.Reconciler{Runs: s.Runs, Items: items, Ledger: l, Projection: s.Projection, Clock: s.Clock}
	items.queueing[1] = []*cs.ItemView{
		{RunID: 1, ItemID: 11, Dimensions: dims()},
		{RunID: 1, ItemID: 12, Dimensions: dims()},
	}
	stats, err := s.Tick(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, stats.Dispatched, "dispatch must complete before rebuild")

	usage, err := l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), usage["item"], "rebuild must not double count")

	// 下一轮仍可派发：释放一条后缺口重新出现。
	s.RunDone = nil
	require.NoError(t, s.ReleaseItem(ctx, 1, 11, 0))
	items.queueing[1] = []*cs.ItemView{{RunID: 1, ItemID: 13, Dimensions: dims()}}
	stats, err = s.Tick(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.Dispatched, 1, "dispatch continues across rebuild")
}
