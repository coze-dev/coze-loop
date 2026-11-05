// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestCentralReservationAtomicDAOHasNoSecondWrites(t *testing.T) {
	ctrl := gomock.NewController(t)
	dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatch.EXPECT().StartReservedItem(gomock.Any(), int64(3), int64(1), int64(2), int64(4), "rq1").Return(repo.StartReservedItemResult{CurrentRequest: true, Started: true}, nil)
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	svc := &ExptItemEventEvalServiceImpl{
		centralGuard: &fakeGuard{confirmResult: true}, dispatchRepo: dispatch,
		exptItemResultRepo: repoMocks.NewMockIExptItemResultRepo(ctrl), exptStatsRepo: repoMocks.NewMockIExptStatsRepo(ctrl),
		exptTurnResultRepo: repoMocks.NewMockIExptTurnResultRepo(ctrl),
	}
	svc.exptItemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(4)).Return(&entity.ExptItemCurrentRequest{RunID: 2, ItemID: 4, RequestID: "rq1", Status: entity.ItemRunState_Processing}, nil)
	called := false
	err := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq1"})
	require.NoError(t, err)
	require.True(t, called)
}

func TestCentralReservationNonCurrentStops(t *testing.T) {
	ctrl := gomock.NewController(t)
	dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatch.EXPECT().StartReservedItem(gomock.Any(), int64(3), int64(1), int64(2), int64(4), "rq1").Return(repo.StartReservedItemResult{}, nil)
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	svc := &ExptItemEventEvalServiceImpl{centralGuard: &fakeGuard{confirmResult: true}, dispatchRepo: dispatch}
	called := false
	require.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq1"}))
	require.False(t, called, "noncurrent/terminal DAO response must stop invocation")
}

// The held-lock filter refresh follows a committed transition; duplicates
// recover execution without repeating the refresh or canonical/turn writes.
func TestCentralReservationCommittedFilter(t *testing.T) {
	for _, started := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("started_%t/filter_error_%t", started, fail), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
				dispatch.EXPECT().StartReservedItem(gomock.Any(), int64(3), int64(1), int64(2), int64(4), "rq1").Return(repo.StartReservedItemResult{CurrentRequest: true, Started: started}, nil)
				dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				result := svcMocks.NewMockExptResultService(ctrl)
				publisher := eventMocks.NewMockExptEventPublisher(ctrl)
				if started && fail {
					publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), &entity.ExptTurnResultFilterEvent{SpaceID: 3, ExperimentID: 1, ItemID: []int64{4}, FilterType: gptr.Of(entity.UpsertExptTurnResultFilterTypeAuto)}, nil).Return(nil)
					publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), &entity.ExptTurnResultFilterEvent{SpaceID: 3, ExperimentID: 1, ItemID: []int64{4}, FilterType: gptr.Of(entity.UpsertExptTurnResultFilterTypeCheck), RetryTimes: gptr.Of(int32(0))}, gptr.Of(10*time.Second)).Return(nil)
				}
				if started {
					var err error
					if fail {
						err = errors.New("filter unavailable")
					}
					result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{4}).Return(err)
				}
				svc := &ExptItemEventEvalServiceImpl{
					centralGuard: &fakeGuard{confirmResult: true}, dispatchRepo: dispatch, resultSvc: result, publisher: publisher,
					exptItemResultRepo: repoMocks.NewMockIExptItemResultRepo(ctrl), exptStatsRepo: repoMocks.NewMockIExptStatsRepo(ctrl), exptTurnResultRepo: repoMocks.NewMockIExptTurnResultRepo(ctrl),
				}
				svc.exptItemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(4)).Return(&entity.ExptItemCurrentRequest{RunID: 2, ItemID: 4, RequestID: "rq1", Status: entity.ItemRunState_Processing}, nil)
				called := false
				require.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq1"}))
				require.True(t, called)
			})
		}
	}
}
