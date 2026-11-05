// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFilterApplicationRejectsInvalidRequest(t *testing.T) {
	for _, req := range []*expt.UpsertExptTurnResultFilterRequest{
		nil,
		{},
		{WorkspaceID: gptr.Of(int64(1)), ExperimentID: gptr.Of(int64(2)), FilterType: gptr.Of("unknown")},
		{WorkspaceID: gptr.Of(int64(1)), ExperimentID: gptr.Of(int64(2)), ItemIds: []int64{-1}},
		{WorkspaceID: gptr.Of(int64(1)), ExperimentID: gptr.Of(int64(2)), RetryTimes: gptr.Of(int32(-1))},
	} {
		app := &experimentApplication{resultSvc: servicemocks.NewMockExptResultService(gomock.NewController(t))}
		_, err := app.UpsertExptTurnResultFilter(context.Background(), req)
		require.Error(t, err)
	}
}

func TestFilterApplicationPreservesRetryError(t *testing.T) {
	svc := servicemocks.NewMockExptResultService(gomock.NewController(t))
	want := errno.NewMQRetryErr("target unavailable")
	svc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(1), int64(2), []int64{3}).Return(want)
	app := &experimentApplication{resultSvc: svc}
	_, err := app.UpsertExptTurnResultFilter(context.Background(), &expt.UpsertExptTurnResultFilterRequest{WorkspaceID: gptr.Of(int64(1)), ExperimentID: gptr.Of(int64(2)), ItemIds: []int64{3}})
	require.Same(t, want, err)
	require.True(t, errno.NeedMQRetry(err))
}

type filterCheckService struct {
	service.ExptResultService
	refreshErr error
	round      int32
	checks     int
}

func (s *filterCheckService) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return s.refreshErr
}
func (s *filterCheckService) ScheduleExptFilterCheck(_ context.Context, _ int64, _ int64, _ []int64, round int32) error {
	s.round = round
	s.checks++
	return nil
}

func TestFilterApplicationAutoChecksOnlySuccessfulRefresh(t *testing.T) {
	for _, fail := range []bool{false, true} {
		svc := &filterCheckService{}
		if fail {
			svc.refreshErr = errno.NewMQRetryErr("target unavailable")
		}
		app := &experimentApplication{resultSvc: svc}
		_, err := app.UpsertExptTurnResultFilter(context.Background(), &expt.UpsertExptTurnResultFilterRequest{WorkspaceID: gptr.Of(int64(1)), ExperimentID: gptr.Of(int64(2)), ItemIds: []int64{3}, RetryTimes: gptr.Of(int32(2))})
		if fail {
			require.Same(t, svc.refreshErr, err)
			require.Zero(t, svc.checks)
		} else {
			require.NoError(t, err)
			require.Equal(t, 1, svc.checks)
			require.Equal(t, int32(2), svc.round)
		}
	}
}
