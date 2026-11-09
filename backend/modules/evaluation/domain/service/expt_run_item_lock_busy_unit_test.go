// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestItemLockBusySourceFailureAndACKBoundaries(t *testing.T) {
	finishedCurrent := func() *entity.ExptItemCurrentRequest {
		current := currentUnitSource().Current
		current.Status = entity.ItemRunState_Success
		current.ResultState = entity.ExptItemResultStateLogged
		return &current
	}
	readErr := errors.New("source read failed")
	staleResumeEvent := currentUnitEvent()
	staleResumeEvent.ResumeSource = &entity.ExptResumeSource{Kind: "target", RecordID: 31, TurnID: 21, InvokeID: "31"}
	staleResumeSource := currentUnitSource()
	staleResumeSource.Turns[21] = &entity.ExptTurnResultRunLog{
		SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7,
		TurnID: 21, TargetResultID: 32, Status: entity.TurnRunState_Success,
	}
	invalidResumeEvent := currentUnitEvent()
	invalidResumeEvent.ResumeSource = &entity.ExptResumeSource{Kind: "target", RecordID: 0, TurnID: 21, InvokeID: "0"}
	modeChangedCurrent := currentUnitSource().Current
	modeChangedCurrent.Mode = entity.EvaluationModeSubmit
	modeChangedFrozen := currentUnitSource()
	modeChangedFrozen.Current.Mode = entity.EvaluationModeSubmit
	negativeRetryEvent := currentUnitEvent()
	negativeRetryEvent.RetryTimes = -1

	for _, tc := range []struct {
		name          string
		event         *entity.ExptItemEvalEvent
		current       *entity.ExptItemCurrentRequest
		currentErr    error
		frozen        *entity.ExptCurrentItemRunSource
		frozenErr     error
		expectFrozen  bool
		wantACK       bool
		wantErrSubstr string
	}{
		{name: "current read error stays retryable", event: currentUnitEvent(), currentErr: readErr, wantErrSubstr: "source read failed"},
		{name: "missing current stays retryable", event: currentUnitEvent(), wantErrSubstr: "item execution lock busy"},
		{name: "unstable current stays retryable", event: currentUnitEvent(), current: &entity.ExptItemCurrentRequest{ObservationUnstable: true}, wantErrSubstr: "item execution lock busy"},
		{name: "negative retry count stays retryable", event: negativeRetryEvent, current: &entity.ExptItemCurrentRequest{}, wantErrSubstr: "invalid busy event retry count"},
		{name: "current mode change stays retryable", event: currentUnitEvent(), current: &modeChangedCurrent, wantErrSubstr: "invalid busy event source"},
		{name: "frozen read error stays retryable", event: currentUnitEvent(), current: finishedCurrent(), frozenErr: readErr, expectFrozen: true, wantErrSubstr: "source read failed"},
		{name: "missing frozen source stays retryable", event: currentUnitEvent(), current: finishedCurrent(), expectFrozen: true, wantErrSubstr: "item execution lock busy"},
		{name: "frozen mode change stays retryable", event: currentUnitEvent(), current: finishedCurrent(), frozen: modeChangedFrozen, expectFrozen: true, wantErrSubstr: "busy event source mode changed"},
		{name: "invalid resume stays retryable", event: invalidResumeEvent, current: finishedCurrent(), frozen: currentUnitSource(), expectFrozen: true, wantErrSubstr: "invalid original resume source"},
		{name: "superseded resume can ACK", event: staleResumeEvent, current: finishedCurrent(), frozen: staleResumeSource, expectFrozen: true, wantACK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := &currentUnitRepo{MockIExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl), MockIExptCurrentResultRepo: repomocks.NewMockIExptCurrentResultRepo(ctrl)}
			items.MockIExptItemResultRepo.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(tc.current, tc.currentErr)
			if tc.expectFrozen {
				items.MockIExptCurrentResultRepo.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(tc.frozen, tc.frozenErr)
			}
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: items, exptTurnResultRepo: repomocks.NewMockIExptTurnResultRepo(ctrl)}
			err := svc.handleItemLockBusy(context.Background(), tc.event)
			if tc.wantACK {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErrSubstr)
			require.True(t, errno.NeedMQRetry(err), "unproven busy work must remain broker-retryable")
		})
	}
}
