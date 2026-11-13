// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central_schedule

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infraRedis "github.com/coze-dev/coze-loop/backend/infra/redis"
	domain "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central_schedule"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

func newAccount(runID int64) *domain.RunAccount {
	return &domain.RunAccount{
		OwnerScope:        "ppe:test:cn:stone.cozeloop.evaluation",
		ExptID:            100,
		RunID:             runID,
		Mode:              domain.ModeCentral,
		Priority:          10,
		TargetConcurrency: 4,
		DimensionLimits:   map[string]int64{"item": 4, "evaluator:1": 2, "sandbox:t:sandbox": 2},
	}
}

func newReservation(rid string, runID, itemID int64, dims ...domain.Dimension) *domain.Reservation {
	now := time.Unix(1000, 0)
	return &domain.Reservation{
		ID:            rid,
		DedupKey:      domain.DedupKey(runID, itemID, 1),
		OwnerScope:    "ppe:test:cn:stone.cozeloop.evaluation",
		ExptID:        100,
		RunID:         runID,
		ItemID:        itemID,
		Attempt:       1,
		Dimensions:    dims,
		CreatedAt:     now,
		UpdatedAt:     now,
		LeaseExpireAt: now.Add(time.Hour),
	}
}

func itemDims(evaluator, sandbox int64) []domain.Dimension {
	return []domain.Dimension{
		{Key: "item", Amount: 1},
		{Key: "evaluator:1", Amount: evaluator},
		{Key: "sandbox:t:sandbox", Amount: sandbox},
	}
}

func TestLedgerReserveAllOrNothing(t *testing.T) {
	ctx := context.Background()
	cli := infraRedis.NewTestRedis(t)
	clock := &fakeClock{t: time.Unix(1000, 0)}
	l := NewLedgerWithClock(cli, clock)
	acc := newAccount(1)
	require.NoError(t, l.InitRun(ctx, acc))

	// 两个 item 各需 1 个 evaluator，但限额只有 2：整批可以占。
	r1 := newReservation("r1", 1, 1, itemDims(1, 0)...)
	r2 := newReservation("r2", 1, 2, itemDims(1, 0)...)
	require.NoError(t, l.ReserveBatch(ctx, acc, []*domain.Reservation{r1, r2}))

	// 再加一个 evaluator 维度会超限：整批必须零变更。
	r3 := newReservation("r3", 1, 3, itemDims(1, 0)...)
	r4 := newReservation("r4", 1, 4, itemDims(1, 0)...)
	err := l.ReserveBatch(ctx, acc, []*domain.Reservation{r3, r4})
	var shortfall *domain.ShortfallError
	require.ErrorAs(t, err, &shortfall)
	require.Equal(t, "evaluator:1", shortfall.Dimension)

	usage, err := l.Usage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), usage["item"])
	require.Equal(t, int64(2), usage["evaluator:1"])

	// 整批失败没有留下任何一条 r3/r4。
	_, err = l.Get(ctx, "r3")
	require.ErrorIs(t, err, domain.ErrReservationAbsent)
	_, err = l.Get(ctx, "r4")
	require.ErrorIs(t, err, domain.ErrReservationAbsent)
}

func TestLedgerAckReleaseIdempotentAndLate(t *testing.T) {
	ctx := context.Background()
	cli := infraRedis.NewTestRedis(t)
	clock := &fakeClock{t: time.Unix(1000, 0)}
	l := NewLedgerWithClock(cli, clock)
	acc := newAccount(2)
	require.NoError(t, l.InitRun(ctx, acc))
	r := newReservation("r1", 2, 1, itemDims(1, 1)...)
	require.NoError(t, l.ReserveBatch(ctx, acc, []*domain.Reservation{r}))

	ack, err := l.Ack(ctx, "r1", domain.UpdateTime{Now: clock.Now()})
	require.NoError(t, err)
	require.Equal(t, domain.AckConfirmed, ack)

	ack, err = l.Ack(ctx, "r1", domain.UpdateTime{Now: clock.Now()})
	require.NoError(t, err)
	require.Equal(t, domain.AckDuplicate, ack)

	rel, err := l.Release(ctx, "r1", domain.UpdateTime{Now: clock.Now()})
	require.NoError(t, err)
	require.Equal(t, domain.ReleaseDone, rel)

	rel, err = l.Release(ctx, "r1", domain.UpdateTime{Now: clock.Now()})
	require.NoError(t, err)
	require.Equal(t, domain.ReleaseAlready, rel)

	ack, err = l.Ack(ctx, "r1", domain.UpdateTime{Now: clock.Now()})
	require.NoError(t, err)
	require.Equal(t, domain.AckLate, ack)

	usage, err := l.Usage(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, int64(0), usage["item"])
	require.Equal(t, int64(0), usage["evaluator:1"])
}

func TestLedgerRebuildGenerationNoDoubleCount(t *testing.T) {
	ctx := context.Background()
	cli := infraRedis.NewTestRedis(t)
	clock := &fakeClock{t: time.Unix(1000, 0)}
	l := NewLedgerWithClock(cli, clock)
	acc := newAccount(3)
	require.NoError(t, l.InitRun(ctx, acc))

	r1 := newReservation("r1", 3, 1, itemDims(1, 0)...)
	r2 := newReservation("r2", 3, 2, itemDims(1, 0)...)
	require.NoError(t, l.ReserveBatch(ctx, acc, []*domain.Reservation{r1, r2}))

	live, err := l.ListRunReservations(ctx, 3, "")
	require.NoError(t, err)
	require.Len(t, live, 2)

	res, err := l.Rebuild(ctx, acc, nil, live)
	require.NoError(t, err)
	require.True(t, res.Swapped)
	require.Equal(t, int64(1), res.Generation)

	usage, err := l.Usage(ctx, 3)
	require.NoError(t, err)
	// 重建后仍是 2，不因“旧代 + 新代”双算成 4。
	require.Equal(t, int64(2), usage["item"])
	require.Equal(t, int64(2), usage["evaluator:1"])

	// 重建后释放一条旧代预占，再新增预占走新代。
	rel, err := l.Release(ctx, "r2", domain.UpdateTime{Now: clock.Now()})
	require.NoError(t, err)
	require.Equal(t, domain.ReleaseDone, rel)
	r3 := newReservation("r3", 3, 3, itemDims(1, 0)...)
	require.NoError(t, l.ReserveBatch(ctx, acc, []*domain.Reservation{r3}))
	usage, err = l.Usage(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, int64(2), usage["item"])

	// 旧代预占释放只减新代一次；不会出现负值或双扣。
	rel, err = l.Release(ctx, "r1", domain.UpdateTime{Now: clock.Now()})
	require.NoError(t, err)
	require.Equal(t, domain.ReleaseDone, rel)
	usage, err = l.Usage(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, int64(1), usage["item"])
	require.Equal(t, int64(1), usage["evaluator:1"])
}

func TestLedgerClaimSingleWinner(t *testing.T) {
	ctx := context.Background()
	cli := infraRedis.NewTestRedis(t)
	clock := &fakeClock{t: time.Unix(1000, 0)}
	l := NewLedgerWithClock(cli, clock)
	require.NoError(t, l.InitRun(ctx, newAccount(4)))

	ok, err := l.Claim(ctx, 4, 1, 1, "r1", 60)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = l.Claim(ctx, 4, 1, 1, "r2", 60)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestLedgerQuotaSourceUnavailable(t *testing.T) {
	ctx := context.Background()
	cli := infraRedis.NewTestRedis(t)
	l := NewLedgerWithClock(cli, &fakeClock{t: time.Unix(1000, 0)})
	// 未 InitRun：run index 缺失，Usage 返回 NotInitialized，Scheduler 会 fail-closed。
	_, err := l.Usage(ctx, 99)
	require.ErrorIs(t, err, domain.ErrNotInitialized)
}
