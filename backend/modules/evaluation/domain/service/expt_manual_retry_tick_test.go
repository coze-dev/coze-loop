// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
)

// Existing admission/quota doubles model SQL only. This adapter gives those
// unit tests an original consumer tick while calling the real API and apply path.
type retryPendingUnitPort struct {
	repo.IExptManualRetryRepo
	manager   *ExptMangerImpl
	pending   *repo.ManualRetryPending
	receipt   *entity.ManualRetryReceipt
	resultErr error
}

func (p *retryPendingUnitPort) RegisterPendingRetry(ctx context.Context, pending *repo.ManualRetryPending) error {
	p.pending = pending
	if p.manager.manualRetryLocker == nil {
		p.resultErr = fmt.Errorf("manual retry locker unavailable")
		p.pending = nil
		return nil
	}
	locker := p.manager.manualRetryLocker()
	if locker == nil {
		p.resultErr = fmt.Errorf("manual retry locker unavailable")
		p.pending = nil
		return nil
	}
	request := pending.Request
	locked, tickCtx, cancel, err := locker.LockWithRenew(ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", request.ExptID, request.RunID), 5*time.Second, manualRetryLockMaxHold)
	if err != nil || !locked {
		if err == nil {
			err = errorx.NewByCode(errno.ManualRetryBusyCode)
		}
		p.resultErr = err
		p.pending = nil
		return nil
	}
	defer cancel()
	if err := p.manager.consumePendingCurrentRetry(tickCtx, &entity.ExptScheduleEvent{SpaceID: request.SpaceID, ExptID: request.ExptID, ExptRunID: request.RunID, ExptRunMode: entity.EvaluationModeSubmit, ExptType: entity.ExptType_Offline}); err != nil {
		p.resultErr = err
		p.pending = nil
	}
	return nil
}

func (p *retryPendingUnitPort) GetPendingRetry(context.Context, int64, int64, int64) (*repo.ManualRetryPending, error) {
	return p.pending, nil
}

func (p *retryPendingUnitPort) ResolvePendingRetry(_ context.Context, _ *repo.ManualRetryPending, cancel bool) (*entity.ManualRetryReceipt, error) {
	if p.receipt != nil || p.resultErr != nil {
		return p.receipt, p.resultErr
	}
	if cancel {
		p.pending = nil
		return nil, errorx.NewByCode(errno.ManualRetryBusyCode)
	}
	return nil, nil
}

func (p *retryPendingUnitPort) PreparePendingRetryCommit(context.Context, *repo.ManualRetryPending, *entity.ManualRetryReceipt, []int64) error {
	return nil
}

func (p *retryPendingUnitPort) CheckPendingRetry(context.Context, *repo.ManualRetryPending) error {
	return nil
}

func (p *retryPendingUnitPort) CompletePendingRetry(_ context.Context, pending *repo.ManualRetryPending, receipt *entity.ManualRetryReceipt, cause error) error {
	p.pending = nil
	p.receipt, p.resultErr = receipt, cause
	if receipt != nil {
		receipt.RunID, receipt.RequestID = pending.Request.RunID, pending.Request.RequestID
	}
	return nil
}

func retryThroughOriginalTickForTest(t *testing.T, manager *ExptMangerImpl, ctx context.Context, request *entity.RetryInCurrentRunRequest) (*entity.ManualRetryReceipt, error) {
	t.Helper()
	if manager.manualRetryRepo == nil {
		return manager.RetryInCurrentRun(ctx, request)
	}
	original := manager.manualRetryRepo
	manager.manualRetryRepo = &retryPendingUnitPort{IExptManualRetryRepo: original, manager: manager}
	defer func() { manager.manualRetryRepo = original }()
	return manager.RetryInCurrentRun(ctx, request)
}

func expectAdmissionTerminalCandidate(ctrl *gomock.Controller, manager *ExptMangerImpl, items *repomocks.MockIExptItemResultRepo, expt *entity.Experiment) {
	expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	expt.EvalConf = &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3), ItemConcurNum: gptr.Of(4)}
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	refs := repomocks.NewMockIExptItemRefRepo(ctrl)
	manager.turnResultRepo, manager.itemRefRepo = turns, refs
	manager.manualRetryRepo = &admissionMemoryRepo{}
	source := &entity.ExptItemResult{ID: 101, SpaceID: 9, ExptID: 7, ExptRunID: 6, ItemID: 11, ItemVersionID: 91, Status: entity.ItemRunState_Fail}
	items.EXPECT().BatchGet(gomock.Any(), int64(9), int64(7), []int64{11}).Return([]*entity.ExptItemResult{source}, nil)
	items.EXPECT().MGetItemRunLog(gomock.Any(), int64(7), int64(8), []int64{11}, int64(9)).Return(nil, nil)
	refs.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(9), int64(7), int64(11)).Return(&entity.ExptItemRef{SpaceID: 9, ExptID: 7, ItemID: 11, ItemVersionID: 91}, nil)
	turns.EXPECT().GetItemTurnResults(gomock.Any(), int64(7), int64(11), int64(9)).Return([]*entity.ExptTurnResult{{ID: 301, SpaceID: 9, ExptID: 7, ExptRunID: 6, ItemID: 11, ItemVersionID: 91, TurnID: 21, Status: int32(entity.TurnRunState_Fail)}}, nil)
	turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(7), int64(8), int64(11), int64(9)).Return(nil, nil)
	turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(9), []int64{301}).Return(nil, nil)
}

func retryThroughRealTickForTest(t *testing.T, manager *ExptMangerImpl, ctx context.Context, request *entity.RetryInCurrentRunRequest) (*entity.ManualRetryReceipt, error) {
	t.Helper()
	if ctx.Err() != nil {
		return manager.RetryInCurrentRun(ctx, request)
	}
	port := manager.manualRetryRepo.(repo.IExptManualRetryPendingRepo)
	previous, readErr := port.GetPendingRetry(ctx, request.SpaceID, request.ExptID, request.ExpectedRunID)
	require.NoError(t, readErr)
	apiCtx, stopAPI := context.WithCancel(ctx)
	defer stopAPI()
	type outcome struct {
		receipt *entity.ManualRetryReceipt
		err     error
	}
	done := make(chan outcome, 1)
	go func() { receipt, err := manager.RetryInCurrentRun(apiCtx, request); done <- outcome{receipt, err} }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case result := <-done:
			return result.receipt, result.err
		case <-deadline.C:
			t.Fatal("request did not register or finish")
		default:
		}
		pending, err := port.GetPendingRetry(ctx, request.SpaceID, request.ExptID, request.ExpectedRunID)
		require.NoError(t, err)
		if pending != nil && (previous == nil || previous.Request.RequestID != pending.Request.RequestID) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	locker := manager.manualRetryLocker()
	key := fmt.Sprintf("expt_run_exec_lock:%d:%d", request.ExptID, request.ExpectedRunID)
	locked, tickCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, manualRetryLockMaxHold)
	require.NoError(t, err)
	if !locked {
		stopAPI()
		result := <-done
		return result.receipt, result.err
	}
	err = manager.consumePendingCurrentRetry(tickCtx, &entity.ExptScheduleEvent{SpaceID: request.SpaceID, ExptID: request.ExptID, ExptRunID: request.ExpectedRunID, ExptRunMode: entity.EvaluationModeSubmit, ExptType: entity.ExptType_Offline})
	cancel()
	_, unlockErr := locker.UnlockWithContext(ctx, key)
	require.NoError(t, unlockErr)
	require.NoError(t, err)
	result := <-done
	return result.receipt, result.err
}
