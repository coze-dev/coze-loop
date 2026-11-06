// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
)

// The barrier only schedules real Redis lock calls. It neither grants a lock
// nor changes a repository response. The second caller has already read SQL
// (and, for retry, prepared its plan) when it reaches secondArrived.
type standardCompleteTickBarrier struct {
	key                         string
	attempts, acquired          atomic.Int32
	firstLocked, secondArrived  chan struct{}
	firstProceed, secondProceed chan struct{}
	firstOnce, secondOnce       sync.Once
	delaySecond                 bool
}

func (b *standardCompleteTickBarrier) releaseFirst() {
	b.firstOnce.Do(func() { close(b.firstProceed) })
}
func (b *standardCompleteTickBarrier) releaseSecond() {
	b.secondOnce.Do(func() { close(b.secondProceed) })
}

type standardCompleteScheduledLocker struct {
	lock.ILocker
	barrier *standardCompleteTickBarrier
}

func (l *standardCompleteScheduledLocker) LockWithRenew(ctx context.Context, key string, ttl, hold time.Duration) (bool, context.Context, func(), error) {
	b := l.barrier
	if key != b.key {
		return l.ILocker.LockWithRenew(ctx, key, ttl, hold)
	}
	n := b.attempts.Add(1)
	if n == 2 {
		close(b.secondArrived)
		if b.delaySecond {
			select {
			case <-b.secondProceed:
			case <-ctx.Done():
				return false, ctx, func() {}, ctx.Err()
			}
		}
	}
	locked, child, cancel, err := l.ILocker.LockWithRenew(ctx, key, ttl, hold)
	if err != nil || !locked {
		return locked, child, cancel, err
	}
	b.acquired.Add(1)
	if n == 1 {
		close(b.firstLocked)
		select {
		case <-b.firstProceed:
		case <-child.Done():
			cancel()
			return false, child, func() {}, child.Err()
		}
	}
	return locked, child, cancel, err
}

func waitStandardCompleteSignal(t *testing.T, ctx context.Context, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatal("standard completion barrier timed out: ", ctx.Err())
	}
}

type standardCompleteRetryOutcome struct {
	receipt *entity.ManualRetryReceipt
	err     error
}

func TestCompleteStandardCurrentRetrySharesTickBothOrders(t *testing.T) {
	for _, endpoint := range []string{"CompleteRun", "CompleteExpt"} {
		for _, order := range []string{"retry_first", "complete_first"} {
			t.Run(endpoint+"/"+order, func(t *testing.T) {
				p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
				p.publishSuccess(t, false)
				f := p.f
				ctx, stop := context.WithTimeout(context.Background(), 25*time.Second)
				defer stop()
				barrier := &standardCompleteTickBarrier{key: fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run), firstLocked: make(chan struct{}), secondArrived: make(chan struct{}), firstProceed: make(chan struct{}), secondProceed: make(chan struct{}), delaySecond: order == "complete_first"}
				defer barrier.releaseFirst()
				defer barrier.releaseSecond()
				p.manager.manualRetryLocker = func() lock.ILocker {
					return &standardCompleteScheduledLocker{ILocker: p.factory(), barrier: barrier}
				}
				complete := func() error {
					if endpoint == "CompleteRun" {
						return p.manager.CompleteRun(ctx, f.expt, f.run, f.space, p.event.Session)
					}
					return p.manager.CompleteExpt(ctx, f.expt, &f.run, f.space, p.event.Session)
				}
				req := &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{f.item}, Session: p.event.Session}
				retryDone, completeDone := make(chan standardCompleteRetryOutcome, 1), make(chan error, 1)
				startRetry := func() {
					go func() {
						receipt, err := p.manager.RetryInCurrentRun(ctx, req)
						retryDone <- standardCompleteRetryOutcome{receipt, err}
					}()
				}
				startComplete := func() { go func() { completeDone <- complete() }() }
				awaitComplete := func() error {
					select {
					case err := <-completeDone:
						return err
					case <-ctx.Done():
						t.Fatal("standard Complete did not return: ", ctx.Err())
						return ctx.Err()
					}
				}
				awaitRetry := func() standardCompleteRetryOutcome {
					select {
					case result := <-retryDone:
						return result
					case <-ctx.Done():
						t.Fatal("current retry did not return: ", ctx.Err())
						return standardCompleteRetryOutcome{err: ctx.Err()}
					}
				}
				if order == "retry_first" {
					startRetry()
				} else {
					startComplete()
				}
				waitStandardCompleteSignal(t, ctx, barrier.firstLocked)
				locked, err := p.manager.mutex.Exists(ctx, barrier.key)
				require.NoError(t, err)
				require.True(t, locked, "first standard caller must really hold the shared Redis tick")
				before := f.snapshot()
				if order == "retry_first" {
					startComplete()
				} else {
					startRetry()
				}
				waitStandardCompleteSignal(t, ctx, barrier.secondArrived)
				if order == "retry_first" {
					err := awaitComplete()
					require.True(t, errno.NeedMQRetry(err), "standard completion must reject the actually held retry tick")
					require.Equal(t, before, f.snapshot())
					barrier.releaseFirst()
					retried := awaitRetry()
					require.NoError(t, retried.err)
					require.NotNil(t, retried.receipt)
					require.Equal(t, int32(1), retried.receipt.AcceptedCount)
					requeued := f.snapshot()
					require.True(t, errno.NeedMQRetry(complete()), "canonical Queueing must prevent natural Success after requeue")
					require.Equal(t, requeued, f.snapshot())
					run, err := p.manager.runLogRepo.Get(ctx, f.expt, f.run)
					require.NoError(t, err)
					require.Equal(t, int64(entity.ExptStatus_Pending), run.Status)
					item, err := p.manager.itemResultRepo.GetCurrentRequest(ctx, f.space, f.expt, f.run, f.item)
					require.NoError(t, err)
					require.Equal(t, entity.ItemRunState_Queueing, item.Status)
					require.NotEmpty(t, item.RequestID)
					require.Equal(t, entity.EvaluationModeRetryItems, item.Mode)
					require.Zero(t, p.resources.Load())
					require.Zero(t, p.notifications.Load())
					admission, err := p.manager.mutex.Exists(ctx, p.manager.makeExptMutexLockKey(f.expt))
					require.NoError(t, err)
					require.True(t, admission)
				} else {
					// Retry's old plan is ready, but its actual LockWithRenew has
					// not run yet. Let standard completion commit under its tick.
					require.Equal(t, before, f.snapshot())
					barrier.releaseFirst()
					require.NoError(t, awaitComplete())
					finished := f.snapshot()
					run, err := p.manager.runLogRepo.Get(ctx, f.expt, f.run)
					require.NoError(t, err)
					require.Equal(t, int64(entity.ExptStatus_Success), run.Status)
					barrier.releaseSecond()
					retried := awaitRetry()
					require.Nil(t, retried.receipt)
					require.ErrorContains(t, retried.err, "run_finished")
					require.Equal(t, int32(2), barrier.acquired.Load(), "retry must really acquire the released tick before checking its stale plan")
					require.Equal(t, finished, f.snapshot(), "rejected retry must not write Queueing, refs, stats or a new attempt")
					require.Equal(t, int32(1), p.resources.Load())
					require.Equal(t, int32(1), p.notifications.Load())
				}
				current, err := p.manager.exptRepo.GetByID(ctx, f.expt, f.space)
				require.NoError(t, err)
				require.Equal(t, f.run, current.LatestRunID)
				var invalid int64
				require.NoError(t, f.conn.Raw("SELECT COUNT(*) FROM expt_run_log r JOIN expt_item_result_run_log i ON i.space_id=r.space_id AND i.expt_id=r.expt_id AND i.expt_run_id=r.expt_run_id WHERE r.space_id=? AND r.status=? AND i.status=?", f.space, entity.ExptStatus_Success, entity.ItemRunState_Queueing).Scan(&invalid).Error)
				require.Zero(t, invalid, "standard entries cannot leave Success run + Queueing item")
				t.Logf("REAL_MYSQL_STANDARD_TICK_ORDER endpoint=%s order=%s acquired=%d latest_run_unchanged=true success_plus_queueing=0", endpoint, order, barrier.acquired.Load())
			})
		}
	}
}
