// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/google/uuid"
	redisc "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	iredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type pendingRetryTestPort struct {
	repo.IExptManualRetryRepo
	repo.IExptManualRetryPendingRepo
	calls atomic.Int64
}

func (p *pendingRetryTestPort) RequeueTerminalItems(ctx context.Context, r *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryReceipt, error) {
	p.calls.Add(1)
	return p.IExptManualRetryRepo.RequeueTerminalItems(ctx, r)
}

func newPendingRetryTestFixture(t *testing.T) (*currentRetryCombinationFixture, *pendingRetryTestPort, *redisc.Client) {
	t.Helper()
	p := newCurrentRetryCombinationFixture(t, entity.EvaluationModeRetryAll)
	p.completeAndPublish(t, p.f.item, entity.ItemRunState_Success)
	require.Equal(t, "127.0.0.1:26379", os.Getenv("RR_REDIS_ADDR"), "only the authorized local Redis may be used")
	cache, err := iredis.NewClient(&redisc.Options{Addr: os.Getenv("RR_REDIS_ADDR"), ContextTimeoutEnabled: true})
	require.NoError(t, err)
	raw, ok := iredis.Unwrap(cache)
	require.True(t, ok)
	t.Cleanup(func() { _ = raw.Close() })
	actual := exptrepo.NewExptManualRetryRepoWithPending(p.f.provider, p.manager.idgenerator, cache)
	port := &pendingRetryTestPort{IExptManualRetryRepo: actual, IExptManualRetryPendingRepo: actual.(repo.IExptManualRetryPendingRepo)}
	p.manager.manualRetryRepo = port
	p.manager.manualRetryLocker = func() lock.ILocker { return lock.NewRedisLocker(cache) }
	return p, port, raw
}

func preparePendingRetryTestRequest(t *testing.T, p *currentRetryCombinationFixture, port *pendingRetryTestPort) *repo.ManualRetryPending {
	t.Helper()
	req := p.request(entity.EvaluationModeRetryItems)
	plan, err := p.manager.prepareTerminalCandidatesAndReusableRefs(context.Background(), p.expt, p.f.run, req.Mode, req.ItemIDs, nil)
	require.NoError(t, err)
	require.Len(t, plan.Items, 1)
	request := &entity.RequeueTerminalItemsRequest{SpaceID: p.f.space, ExptID: p.f.expt, RunID: p.f.run, RequestID: uuid.NewString(), Operator: "pending-contract-test", Mode: req.Mode, SnapshotAtMS: plan.SnapshotAtMS, Items: plan.Items, RejectedItems: plan.RejectedItems}
	pending := &repo.ManualRetryPending{Request: request, ExpiresAtMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(t, port.RegisterPendingRetry(context.Background(), pending))
	return pending
}

func TestPendingRetryCanceledCachedRequestCannotCommit(t *testing.T) {
	p, port, _ := newPendingRetryTestFixture(t)
	pending := preparePendingRetryTestRequest(t, p, port)
	cached, err := port.GetPendingRetry(context.Background(), p.f.space, p.f.expt, p.f.run)
	require.NoError(t, err)
	before := p.f.snapshot()
	_, err = port.ResolvePendingRetry(context.Background(), pending, true)
	status, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	require.Equal(t, int32(errno.ManualRetryBusyCode), status.Code())
	cached.Request.BeforeRequeue = func(ctx context.Context) error { return port.CheckPendingRetry(ctx, cached) }
	receipt, err := port.RequeueTerminalItems(context.Background(), cached.Request)
	require.Nil(t, receipt)
	require.Error(t, err)
	require.Equal(t, before, p.f.snapshot(), "cancel wins before any item/turn/stat mutation")
}

type pendingReceiptFailureHook struct {
	key  string
	fail atomic.Bool
}

func (h *pendingReceiptFailureHook) DialHook(next redisc.DialHook) redisc.DialHook { return next }
func (h *pendingReceiptFailureHook) ProcessPipelineHook(next redisc.ProcessPipelineHook) redisc.ProcessPipelineHook {
	return next
}
func (h *pendingReceiptFailureHook) ProcessHook(next redisc.ProcessHook) redisc.ProcessHook {
	return func(ctx context.Context, cmd redisc.Cmder) error {
		if cmd.Name() == "set" && len(cmd.Args()) > 1 && fmt.Sprint(cmd.Args()[1]) == h.key && h.fail.CompareAndSwap(true, false) {
			return errors.New("controlled receipt Redis timeout")
		}
		return next(ctx, cmd)
	}
}

func TestPendingRetryCommittedTokenRecoversAfterRunFinishedAndOldSave(t *testing.T) {
	p, port, raw := newPendingRetryTestFixture(t)
	pending := preparePendingRetryTestRequest(t, p, port)
	hook := &pendingReceiptFailureHook{key: fmt.Sprintf("[experiment]pending_manual_retry:%d:%d:%d:%s", p.f.space, p.f.expt, p.f.run, pending.Request.RequestID)}
	hook.fail.Store(true)
	raw.AddHook(hook)
	locker := p.manager.manualRetryLocker()
	key := fmt.Sprintf("expt_run_exec_lock:%d:%d", p.f.expt, p.f.run)
	locked, ctx, cancel, err := locker.LockWithRenew(context.Background(), key, 5*time.Second, manualRetryLockMaxHold)
	require.NoError(t, err)
	require.True(t, locked)
	require.NoError(t, p.manager.consumePendingCurrentRetry(ctx, p.event))
	cancel()
	_, err = locker.UnlockWithContext(context.Background(), key)
	require.NoError(t, err)
	require.False(t, hook.fail.Load(), "the actual postcommit receipt write must fail once")
	require.Equal(t, int64(1), port.calls.Load())
	current, err := p.f.itemRepo().GetCurrentRequest(context.Background(), p.f.space, p.f.expt, p.f.run, p.f.item)
	require.NoError(t, err)
	require.Equal(t, pending.Request.RequestID, current.RequestID)
	require.Equal(t, entity.ItemRunState_Queueing, current.Status)
	p.completeAndPublish(t, p.f.item, entity.ItemRunState_Success)
	p.completeAndPublish(t, p.items[1], entity.ItemRunState_Success)
	// e6's DAO calls GORM Save without the current immutable-field projection.
	// Execute that persistence operation with an old-shaped item_ids payload.
	run, err := exptmysql.NewExptRunLogDAO(p.f.provider).Get(context.Background(), p.f.expt, p.f.run)
	require.NoError(t, err)
	require.Contains(t, string(gptr.Indirect(run.ItemIds)), "future")
	oldItems, err := json.Marshal([]struct {
		ItemIDs  []int64
		CreateAt *int64
	}{{ItemIDs: p.items, CreateAt: gptr.Of(int64(1700000000))}})
	require.NoError(t, err)
	run.ItemIds, run.Status = &oldItems, gptr.Of(int64(entity.ExptStatus_Success))
	require.NoError(t, p.f.provider.NewSession(context.Background(), db.WithMaster()).WithContext(context.Background()).Save(run).Error)
	saved, err := exptmysql.NewExptRunLogDAO(p.f.provider).Get(context.Background(), p.f.expt, p.f.run, db.WithMaster())
	require.NoError(t, err)
	require.JSONEq(t, string(oldItems), string(gptr.Indirect(saved.ItemIds)))
	require.NotContains(t, string(gptr.Indirect(saved.ItemIds)), "future", "the database metadata must actually be overwritten")
	require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", p.f.space, p.f.expt).Update("status", entity.ExptStatus_Success).Error)
	source, err := p.f.port.GetCurrentItemRunSource(context.Background(), p.f.space, p.f.expt, p.f.run, p.f.item)
	require.NoError(t, err)
	require.Nil(t, source, "the active-only reader cannot establish the committed result")
	receipt, err := port.ResolvePendingRetry(context.Background(), pending, true)
	require.NoError(t, err)
	require.Equal(t, int32(1), receipt.AcceptedCount)
	require.Equal(t, pending.Request.RequestID, receipt.RequestID)
	require.NoError(t, p.manager.consumePendingCurrentRetry(context.Background(), p.event))
	require.Equal(t, int64(1), port.calls.Load(), "a committed request is never requeued a second time")
}

type pendingReadFailureHook struct {
	prefix string
	fail   atomic.Bool
}

func (h *pendingReadFailureHook) DialHook(next redisc.DialHook) redisc.DialHook { return next }
func (h *pendingReadFailureHook) ProcessPipelineHook(next redisc.ProcessPipelineHook) redisc.ProcessPipelineHook {
	return next
}
func (h *pendingReadFailureHook) ProcessHook(next redisc.ProcessHook) redisc.ProcessHook {
	return func(ctx context.Context, cmd redisc.Cmder) error {
		if cmd.Name() == "get" && len(cmd.Args()) > 1 && strings.HasPrefix(fmt.Sprint(cmd.Args()[1]), h.prefix) && h.fail.CompareAndSwap(true, false) {
			return errors.New("controlled pending Redis timeout")
		}
		return next(ctx, cmd)
	}
}

func TestPendingRetryRedisReadFailureRequiresMQRetryWithoutPending(t *testing.T) {
	p, _, raw := newPendingRetryTestFixture(t)
	hook := &pendingReadFailureHook{prefix: fmt.Sprintf("[experiment]pending_manual_retry:%d:%d:%d", p.f.space, p.f.expt, p.f.run)}
	hook.fail.Store(true)
	raw.AddHook(hook)
	err := p.manager.consumePendingCurrentRetry(context.Background(), p.event)
	require.True(t, errno.NeedMQRetry(err))
	current, getErr := p.f.experimentRepo().GetByID(context.Background(), p.f.expt, p.f.space)
	require.NoError(t, getErr)
	require.Equal(t, entity.ExptStatus_Processing, current.Status)
}

func TestPendingRetryDefault30SecondDeadlineReturnsBusyAndCancels(t *testing.T) {
	p, port, _ := newPendingRetryTestFixture(t)
	before := p.f.snapshot()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	started := time.Now()
	receipt, err := p.manager.RetryInCurrentRun(ctx, p.request(entity.EvaluationModeRetryItems))
	elapsed := time.Since(started)
	require.Nil(t, receipt)
	status, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	require.Equal(t, int32(errno.ManualRetryBusyCode), status.Code())
	require.Less(t, elapsed, 30*time.Second, "waiting for an O-to-N cycle must not consume the default client deadline")
	pending, getErr := port.GetPendingRetry(context.Background(), p.f.space, p.f.expt, p.f.run)
	require.NoError(t, getErr)
	require.Nil(t, pending)
	require.Equal(t, before, p.f.snapshot())
	require.Equal(t, int64(0), port.calls.Load())
	t.Logf("DEFAULT_30S request_ms=%d busy=true pending_removed=true item_turn_stats_unchanged=true", elapsed.Milliseconds())
}

type pendingDeleteFailureHook struct {
	key  string
	fail atomic.Bool
}

func (h *pendingDeleteFailureHook) DialHook(next redisc.DialHook) redisc.DialHook { return next }
func (h *pendingDeleteFailureHook) ProcessPipelineHook(next redisc.ProcessPipelineHook) redisc.ProcessPipelineHook {
	return next
}
func (h *pendingDeleteFailureHook) ProcessHook(next redisc.ProcessHook) redisc.ProcessHook {
	return func(ctx context.Context, cmd redisc.Cmder) error {
		if cmd.Name() == "eval" && len(cmd.Args()) > 3 && fmt.Sprint(cmd.Args()[3]) == h.key && strings.Contains(fmt.Sprint(cmd.Args()[1]), "Request.RequestID") && h.fail.CompareAndSwap(true, false) {
			return errors.New("controlled pending conditional-delete timeout")
		}
		return next(ctx, cmd)
	}
}

func TestPendingRetryReceiptHitClearsFailedDeleteAndPreservesNewOwner(t *testing.T) {
	p, port, raw := newPendingRetryTestFixture(t)
	pending := preparePendingRetryTestRequest(t, p, port)
	hook := &pendingDeleteFailureHook{key: fmt.Sprintf("[experiment]pending_manual_retry:%d:%d:%d", p.f.space, p.f.expt, p.f.run)}
	hook.fail.Store(true)
	raw.AddHook(hook)
	require.NoError(t, p.manager.consumePendingCurrentRetry(context.Background(), p.event))
	require.False(t, hook.fail.Load(), "the actual first conditional delete must fail")
	receipt, err := port.ResolvePendingRetry(context.Background(), pending, false)
	require.NoError(t, err)
	require.Equal(t, int32(1), receipt.AcceptedCount)
	current, err := port.GetPendingRetry(context.Background(), p.f.space, p.f.expt, p.f.run)
	require.NoError(t, err)
	require.Nil(t, current, "a result hit must reconcile the old pending slot")
	p.completeAndPublish(t, p.f.item, entity.ItemRunState_Success)
	second := preparePendingRetryTestRequest(t, p, port)
	require.NotEqual(t, pending.Request.RequestID, second.Request.RequestID)
	oldReceipt, err := port.ResolvePendingRetry(context.Background(), pending, false)
	require.NoError(t, err)
	require.Equal(t, pending.Request.RequestID, oldReceipt.RequestID)
	current, err = port.GetPendingRetry(context.Background(), p.f.space, p.f.expt, p.f.run)
	require.NoError(t, err)
	require.Equal(t, second.Request.RequestID, current.Request.RequestID, "reconciling an old receipt must not delete a new request")
	_, err = port.ResolvePendingRetry(context.Background(), second, true)
	require.Error(t, err)
}

func TestPendingRetryExpiredAbandonedRequestDoesNotBlockNewRegistration(t *testing.T) {
	p, port, raw := newPendingRetryTestFixture(t)
	abandoned := preparePendingRetryTestRequest(t, p, port)
	cached, err := port.GetPendingRetry(context.Background(), p.f.space, p.f.expt, p.f.run)
	require.NoError(t, err)
	// No Resolve/cancel follows the first registration, as when its API exits.
	// Expire its absolute deadline while retaining the actual one-hour Redis key.
	abandoned.ExpiresAtMS = time.Now().Add(-time.Second).UnixMilli()
	encoded, err := json.Marshal(abandoned)
	require.NoError(t, err)
	key := fmt.Sprintf("[experiment]pending_manual_retry:%d:%d:%d", p.f.space, p.f.expt, p.f.run)
	require.NoError(t, raw.Set(context.Background(), key, encoded, time.Hour).Err())
	require.Greater(t, raw.TTL(context.Background(), key).Val(), 30*time.Minute)
	before := p.f.snapshot()
	request := *abandoned.Request
	request.RequestID = uuid.NewString()
	second := &repo.ManualRetryPending{Request: &request, ExpiresAtMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(t, port.RegisterPendingRetry(context.Background(), second), "an expired abandoned owner cannot require N delivery or one-hour TTL")
	require.Equal(t, before, p.f.snapshot(), "registration reclamation must not requeue any business row")
	cached.Request.BeforeRequeue = func(ctx context.Context) error { return port.CheckPendingRetry(ctx, cached) }
	_, err = port.RequeueTerminalItems(context.Background(), cached.Request)
	require.Error(t, err)
	require.Equal(t, before, p.f.snapshot(), "the previously cached request cannot commit after reclamation")
	thirdRequest := request
	thirdRequest.RequestID = uuid.NewString()
	err = port.RegisterPendingRetry(context.Background(), &repo.ManualRetryPending{Request: &thirdRequest, ExpiresAtMS: second.ExpiresAtMS})
	status, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	require.Equal(t, int32(errno.ManualRetryBusyCode), status.Code(), "an unexpired owner remains exclusive")
	_, err = port.ResolvePendingRetry(context.Background(), abandoned, true)
	require.Error(t, err)
	current, err := port.GetPendingRetry(context.Background(), p.f.space, p.f.expt, p.f.run)
	require.NoError(t, err)
	require.Equal(t, second.Request.RequestID, current.Request.RequestID, "late reclamation cannot delete a newer owner")
	_, err = port.ResolvePendingRetry(context.Background(), second, true)
	require.Error(t, err)
}

func TestPendingRetryFinishWinsBeforeConsumptionWithoutQueueing(t *testing.T) {
	p, port, _ := newPendingRetryTestFixture(t)
	pending := preparePendingRetryTestRequest(t, p, port)
	cached, err := port.GetPendingRetry(context.Background(), p.f.space, p.f.expt, p.f.run)
	require.NoError(t, err)
	p.completeAndPublish(t, p.items[1], entity.ItemRunState_Success)
	finished, err := p.manager.runFinishRepo.FinishCurrentRun(context.Background(), &entity.ExptRunFinishRequest{SpaceID: p.f.space, ExptID: p.f.expt, RunID: p.f.run})
	require.NoError(t, err)
	require.Equal(t, entity.ExptRunFinishFinished, finished.Disposition)
	before := p.f.snapshot()
	_, err = port.ResolvePendingRetry(context.Background(), pending, false)
	status, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	require.Equal(t, int32(errno.ManualRetryRunFinishedCode), status.Code())
	cached.Request.BeforeRequeue = func(ctx context.Context) error { return port.CheckPendingRetry(ctx, cached) }
	_, err = port.RequeueTerminalItems(context.Background(), cached.Request)
	require.Error(t, err)
	require.Equal(t, before, p.f.snapshot())
	current, err := p.f.experimentRepo().GetByID(context.Background(), p.f.expt, p.f.space)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Success, current.Status)
	var queueing int64
	require.NoError(t, p.f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND status=?", p.f.space, p.f.expt, entity.ItemRunState_Queueing).Count(&queueing).Error)
	require.Zero(t, queueing)
}

func TestPendingRetryCancelWaitsForCommitAndReturnsActualReceipt(t *testing.T) {
	p, port, _ := newPendingRetryTestFixture(t)
	pending := preparePendingRetryTestRequest(t, p, port)
	entered, release := make(chan struct{}), make(chan struct{})
	pending.Request.BeforeRequeue = func(ctx context.Context) error { return port.CheckPendingRetry(ctx, pending) }
	pending.Request.BeforeCommit = func(ctx context.Context, receipt *entity.ManualRetryReceipt, ids []int64) error {
		if err := port.PreparePendingRetryCommit(ctx, pending, receipt, ids); err != nil {
			return err
		}
		close(entered)
		<-release
		return nil
	}
	type reply struct {
		receipt *entity.ManualRetryReceipt
		err     error
	}
	commitDone, cancelDone := make(chan reply, 1), make(chan reply, 1)
	go func() {
		receipt, err := port.RequeueTerminalItems(context.Background(), pending.Request)
		commitDone <- reply{receipt, err}
	}()
	<-entered
	go func() {
		receipt, err := port.ResolvePendingRetry(context.Background(), pending, true)
		cancelDone <- reply{receipt, err}
	}()
	select {
	case result := <-cancelDone:
		close(release)
		t.Fatalf("cancel bypassed the committing run row lock: %v", result.err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	committed, canceled := <-commitDone, <-cancelDone
	require.NoError(t, committed.err)
	require.NoError(t, canceled.err)
	require.Equal(t, int32(1), committed.receipt.AcceptedCount)
	require.Equal(t, pending.Request.RequestID, canceled.receipt.RequestID)
	require.Equal(t, int32(1), canceled.receipt.AcceptedCount)
	current, err := p.f.itemRepo().GetCurrentRequest(context.Background(), p.f.space, p.f.expt, p.f.run, p.f.item)
	require.NoError(t, err)
	require.Equal(t, pending.Request.RequestID, current.RequestID)
	require.Equal(t, entity.ItemRunState_Queueing, current.Status)
}
