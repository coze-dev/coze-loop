// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func busyRetryRedis(t *testing.T) infraredis.Cmdable {
	t.Helper()
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	require.NotEmpty(t, addr, "real Redis must be configured; busy-retry counterexample cannot skip")
	client, err := infraredis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	info, err := raw.Info(context.Background(), "server").Result()
	require.NoError(t, err)
	require.Contains(t, info, "redis_version:")
	t.Cleanup(func() { _ = raw.Close() })
	t.Logf("REAL_REDIS_EXECUTED addr=%s", addr)
	return client
}

// busyProjectionRepo keeps the production submitCurrentItem ordering: Publish
// happens before the second item-key acquisition, and the projection CAS runs
// while that second acquisition is still held.
type busyProjectionRepo struct {
	repo.IExptItemResultRepo
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (r *busyProjectionRepo) MarkProcessingIfCurrent(ctx context.Context, spaceID, exptID, runID, itemID int64, requestID string) (bool, error) {
	r.once.Do(func() { close(r.entered) })
	select {
	case <-r.release:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	return r.IExptItemResultRepo.MarkProcessingIfCurrent(ctx, spaceID, exptID, runID, itemID, requestID)
}

type busyCapturePublisher struct {
	events.ExptEventPublisher
	published chan *entity.ExptItemEvalEvent
}

func (p *busyCapturePublisher) BatchPublishExptRecordEvalEvent(_ context.Context, events []*entity.ExptItemEvalEvent, _ *time.Duration) error {
	p.published <- events[0]
	return nil
}

// TestSubmitCurrentItemSecondLockBusyKeepsDispatchRetryable reproduces the
// counterexample: the first Queueing delivery loses the item lock to the
// producer's own second projection holder. Losing that lock is not proof that
// the dispatch was executed, so the consumer must return the native broker
// retry and the same message must execute after the producer releases.
func TestSubmitCurrentItemSecondLockBusyKeepsDispatchRetryable(t *testing.T) {
	f := newLateWriteFixture(t, "")
	client := busyRetryRedis(t)
	factory := func() lock.ILocker { return lock.NewRedisLocker(client) }

	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{}).AnyTimes()
	publisher := &busyCapturePublisher{published: make(chan *entity.ExptItemEvalEvent, 1)}
	repo := &busyProjectionRepo{
		IExptItemResultRepo: f.itemRepo(),
		entered:             make(chan struct{}),
		release:             make(chan struct{}),
	}
	scheduler := &ExptSchedulerImpl{ExptItemResultRepo: repo, Publisher: publisher, Configer: config, itemLocker: factory}

	queueingItem := f.item + 2
	schedule := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ExptRunMode: entity.EvaluationModeSubmit}
	candidate := &entity.ExptEvalItem{ItemID: queueingItem, State: entity.ItemRunState_Queueing}
	producerDone := make(chan error, 1)
	go func() {
		producerDone <- scheduler.submitCurrentItem(context.Background(), schedule, candidate, factory())
	}()

	var delivered *entity.ExptItemEvalEvent
	select {
	case delivered = <-publisher.published:
	case <-time.After(5 * time.Second):
		t.Fatal("producer never published the Queueing event")
	}
	select {
	case <-repo.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("producer never acquired the second projection lock")
	}

	manager := svcmocks.NewMockIExptManager(ctrl)
	manager.EXPECT().GetRunLog(gomock.Any(), f.expt, f.run, f.space, gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)}, nil).AnyTimes()
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	experiments.EXPECT().GetByID(gomock.Any(), f.expt, f.space).Return(&entity.Experiment{ID: f.expt, SpaceID: f.space, ExptDispatchMode: entity.ExptDispatchModeLegacy}, nil).AnyTimes()
	consumer := &ExptItemEventEvalServiceImpl{
		exptItemResultRepo: f.itemRepo(),
		itemLocker:         factory,
		manager:            manager,
		experimentRepo:     experiments,
	}
	var executed atomic.Bool
	handler := RecordEvalChain(consumer.HandleEventErr, consumer.HandleEventCheck, consumer.HandleCentralAdmission,
		consumer.HandleEventLock, consumer.HandleCurrentSource, consumer.HandleCentralReservation)(func(context.Context, *entity.ExptItemEvalEvent) error {
		executed.Store(true)
		return nil
	})

	err := handler(context.Background(), delivered)
	require.True(t, errno.NeedMQRetry(err), "a pending dispatch that lost the item lock must stay retryable, got %v", err)
	require.False(t, executed.Load(), "the item must not execute while the producer holds the projection lock")

	close(repo.release)
	require.NoError(t, <-producerDone)

	require.NoError(t, handler(context.Background(), delivered), "the retried message must execute after the producer releases")
	require.True(t, executed.Load(), "retryable dispatch must eventually run the item")
}

// TestFilterLockBusyKeepsResumeCallbackRetryable reproduces the second
// counterexample: an independent withFilterItemLock refresh holds the
// Processing item's lock when the current invocation's resume callback arrives.
// The callback must be retried, not ACKed before HandleCurrentSource can prove
// the source is complete or superseded.
func TestFilterLockBusyKeepsResumeCallbackRetryable(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	client := busyRetryRedis(t)
	factory := func() lock.ILocker { return lock.NewRedisLocker(client) }

	filter := &ExptResultServiceImpl{resultPublishLocker: factory}
	filterHolding := make(chan struct{})
	filterRelease := make(chan struct{})
	filterDone := make(chan error, 1)
	go func() {
		filterDone <- filter.withFilterItemLock(context.Background(), f.expt, f.item, func(context.Context) error {
			close(filterHolding)
			<-filterRelease
			return nil
		})
	}()
	select {
	case <-filterHolding:
	case <-time.After(5 * time.Second):
		t.Fatal("filter never acquired the item lock")
	}

	ctrl := gomock.NewController(t)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	target.EXPECT().GetRecordByID(gomock.Any(), f.space, f.base+41).Return(&entity.EvalTargetRecord{
		ID: f.base + 41, SpaceID: f.space, ExperimentRunID: f.run, ItemID: f.item,
		ItemVersionID: f.version, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusSuccess),
	}, nil).AnyTimes()
	consumer := &ExptItemEventEvalServiceImpl{
		exptItemResultRepo: f.itemRepo(),
		exptTurnResultRepo: f.turnRepo(),
		evaTargetService:   target,
		itemLocker:         factory,
	}
	event := f.event()
	event.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: f.base + 41, InvokeID: strconv.FormatInt(f.base+41, 10)}
	var executed atomic.Bool
	handler := RecordEvalChain(consumer.HandleEventErr, consumer.HandleEventLock,
		consumer.HandleCurrentSource)(func(context.Context, *entity.ExptItemEvalEvent) error {
		executed.Store(true)
		return nil
	})

	err := handler(context.Background(), event)
	require.True(t, errno.NeedMQRetry(err), "a current resume callback that lost the item lock must stay retryable, got %v", err)
	require.False(t, executed.Load(), "the callback must not be ACKed before the source checks run")

	close(filterRelease)
	require.NoError(t, <-filterDone)

	require.NoError(t, handler(context.Background(), event), "the retried resume callback must execute after the filter releases")
	require.True(t, executed.Load(), "retryable resume callback must eventually continue the invocation")
}
