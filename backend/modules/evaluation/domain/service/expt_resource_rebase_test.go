// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type rebaseResourceGuard struct {
	*resourceGuard
	confirms        int
	releases        int
	finalizationErr error
}

func (g *rebaseResourceGuard) ConfirmItemRunning(ctx context.Context, expt *entity.Experiment, item *entity.ExptItemResultRunLog) (bool, error) {
	g.confirms++
	return g.resourceGuard.ConfirmItemRunning(ctx, expt, item)
}

func (g *rebaseResourceGuard) Release(context.Context, string, int64, int64, string, string) error {
	g.releases++
	return nil
}

func (g *rebaseResourceGuard) OnItemTerminal(ctx context.Context, item *entity.ExptItemResultRunLog) error {
	_ = g.resourceGuard.OnItemTerminal(ctx, item)
	return g.finalizationErr
}

func TestResourceRebaseCurrentRequestBeforeHandshake(t *testing.T) {
	for _, request := range []string{"stale", "current"} {
		t.Run(request, func(t *testing.T) {
			svc, items, original, event, ctx := resourceFixture()
			svc.exptItemResultRepo = resourceCurrentItemStore{items}
			original.ready = true
			guard := &rebaseResourceGuard{resourceGuard: original}
			svc.centralGuard = guard
			items.requestID, event.RequestID = "current", request
			calls := 0
			next := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
				calls++
				return nil
			})
			require.NoError(t, next(ctx, event))
			if request == "stale" {
				require.Zero(t, calls)
				require.Zero(t, guard.confirms)
				require.Equal(t, int32(entity.ItemRunState_Queueing), items.run.Status)
				require.Equal(t, entity.QuotaReservationStateReserved, items.run.QuotaReservationState)
			} else {
				require.Equal(t, 1, calls)
				require.Equal(t, 1, guard.confirms)
				require.Equal(t, int32(entity.ItemRunState_Processing), items.run.Status)
				require.Equal(t, entity.QuotaReservationStateNone, items.run.QuotaReservationState)
			}
			require.Zero(t, guard.releases)
		})
	}
}

func TestResourceRebaseCurrentRetryKeepsBudgetDespiteYieldFlag(t *testing.T) {
	svc, port, expts, source, event, held := retryBudgetHandler(t)
	svc.centralGuard = &resourceGuard{}
	event.Ext = map[string]string{entity.RetryYieldExtKey: "true"}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{ID: 7, SpaceID: 9, ExptDispatchMode: entity.ExptDispatchModeEnforce})
	port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(source, nil)
	expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(&entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8}, nil)
	svc.configer.(*confmocks.MockIConfiger).EXPECT().GetErrRetryConf(gomock.Any(), int64(9), gomock.Any()).Return(&entity.RetryConf{RetryTimes: 2})
	port.EXPECT().CompleteCurrentItemRun(gomock.Any(), source, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *entity.ExptCurrentItemRunSource, write *entity.ExptCurrentItemRunWrite) (bool, error) {
			require.True(t, *held)
			require.True(t, write.RetryOnly)
			require.False(t, write.Yield)
			require.Equal(t, int32(1), write.RetryTimes)
			return true, nil
		})
	svc.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), nil).DoAndReturn(
		func(_ context.Context, retry *entity.ExptItemEvalEvent, _ *time.Duration, _ func(*entity.ExptItemEvalEvent)) error {
			require.False(t, *held)
			require.Equal(t, source.Current.RequestID, retry.RequestID)
			require.Equal(t, 1, retry.RetryTimes)
			return nil
		})
	require.NoError(t, svc.handleCurrentEventErr(ctx, event, errors.New("retryable target failure"), port, &currentItemExecution{Source: source}))
}

func TestResourceRebaseAdmissionFailureIsBrokerRetry(t *testing.T) {
	svc, items, _, event, ctx := resourceFixture()
	svc.endpoints = svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return errCentralItemAdmission })
	err := svc.Eval(ctx, event)
	require.ErrorIs(t, err, errCentralItemAdmission)
	require.True(t, errno.NeedMQRetry(err))
	require.Equal(t, int32(entity.ItemRunState_Queueing), items.run.Status)
}

func TestResourceRebaseCommittedTerminationStopsBeforeLifecycle(t *testing.T) {
	for _, stopFailed := range []bool{true, false} {
		t.Run(map[bool]string{true: "stop_failed", false: "stopped"}[stopFailed], func(t *testing.T) {
			mgr, items, original, expts, sandbox := cancellationFixture()
			guard := &rebaseResourceGuard{resourceGuard: original}
			mgr.centralGuard = guard
			items.run.Status = int32(entity.ItemRunState_Terminal)
			items.requestID = "current"
			if stopFailed {
				sandbox.stopErr = errors.New("physical stop unavailable")
			}
			source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{
				SpaceID: 3, ExptID: 1, RunID: 2, ItemID: 4, Status: entity.ItemRunState_Processing, RequestID: "current",
			}}
			debt, err := mgr.releaseTerminationResourcesWithDebt(context.Background(), expts.expt, 2, []*entity.ExptCurrentItemRunSource{source}, false, &entity.Session{})
			require.Zero(t, guard.releases, "the old quota ledger must never release the resource protocol's running allocation")
			if stopFailed {
				require.ErrorIs(t, err, sandbox.stopErr)
				require.Len(t, debt, 1)
				require.Empty(t, guard.finalized)
			} else {
				require.NoError(t, err)
				require.Empty(t, debt)
				require.True(t, sandbox.stopped)
				require.Len(t, guard.finalized, 1)
			}
		})
	}
}

func TestResourceRebaseLifecycleFailureRetainsTerminalDebt(t *testing.T) {
	mgr, items, original, expts, sandbox := cancellationFixture()
	guard := &rebaseResourceGuard{resourceGuard: original, finalizationErr: errors.New("center unavailable")}
	mgr.centralGuard = guard
	items.run.Status = int32(entity.ItemRunState_Terminal)
	source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{
		SpaceID: 3, ExptID: 1, RunID: 2, ItemID: 4, Status: entity.ItemRunState_Processing,
	}}
	debt, err := mgr.releaseTerminationResourcesWithDebt(context.Background(), expts.expt, 2, []*entity.ExptCurrentItemRunSource{source}, false, &entity.Session{})
	require.NoError(t, err, "notification remains best-effort after durable termination")
	require.True(t, sandbox.stopped)
	require.Len(t, debt, 1)
	require.Zero(t, guard.releases)
	debt, err = mgr.releaseTerminationResourcesWithDebt(context.Background(), expts.expt, 2, []*entity.ExptCurrentItemRunSource{source}, true, &entity.Session{})
	require.ErrorIs(t, err, guard.finalizationErr, "a finished whole run has no future tick; cleanup must remain retriable")
	require.Len(t, debt, 1)
	require.True(t, errno.NeedMQRetry(err))
	err = mgr.releaseTerminalResourceDebt(context.Background(), expts.expt, &source.Current)
	require.ErrorIs(t, err, guard.finalizationErr, "reconciliation must not clear the source/CAS debt on callback failure")
}

var _ component.ICentralReservationGuard = (*rebaseResourceGuard)(nil)
