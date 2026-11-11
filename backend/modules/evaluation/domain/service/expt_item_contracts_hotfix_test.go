// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestHotfixResumeSourceUsesExperimentRecordSpace(t *testing.T) {
	ctrl := gomock.NewController(t)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	manager := svcmocks.NewMockIExptManager(ctrl)
	refs := repomocks.NewMockIExptItemRefRepo(ctrl)
	svc := &ExptItemEventEvalServiceImpl{
		exptTurnResultRepo: turns, evaTargetService: target,
		manager: manager, exptItemRefRepo: refs,
	}
	event := &entity.ExptItemEvalEvent{
		SpaceID: 3, ExptID: 5, ExptRunID: 7, EvalSetItemID: 11,
		ResumeSource: &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: 61, InvokeID: "61"},
	}
	current := &entity.ExptItemCurrentRequest{SpaceID: 3, ExptID: 5, RunID: 7, ItemID: 11, ItemVersionID: 13, Mode: entity.EvaluationModeSubmit}
	turn := &entity.ExptTurnResultRunLog{
		SpaceID: 3, ExptID: 5, ExptRunID: 7, ItemID: 11, ItemVersionID: 13,
		TurnID: 0, TargetResultID: 61, Status: entity.TurnRunState_Processing,
	}
	turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(5), int64(7), int64(11), int64(3)).Return([]*entity.ExptTurnResultRunLog{turn}, nil)
	manager.EXPECT().GetDetail(gomock.Any(), int64(5), int64(3), gomock.Any()).Return(&entity.Experiment{
		ID: 5, SpaceID: 3, TargetSpaceID: 100, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
	}, nil).Times(0)
	refs.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(3), int64(5), int64(11)).Return(&entity.ExptItemRef{
		SpaceID: 3, ExptID: 5, ItemID: 11, ItemVersionID: 13,
		ItemConfig: &entity.ExptItemConfig{TargetSourceSpaceID: 200},
	}, nil).Times(0)
	status := entity.EvalTargetRunStatusSuccess
	target.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(61)).Return(&entity.EvalTargetRecord{
		ID: 61, SpaceID: 3, ExperimentRunID: 7, ItemID: 11, ItemVersionID: 13, TurnID: 0, Status: &status,
	}, nil)

	valid, err := svc.validateResumeSource(context.Background(), event, current)
	require.NoError(t, err)
	require.True(t, valid)
}

type hotfixScanRepo struct {
	repo.IExptItemResultRepo
	source    *entity.ExptCurrentItemRunSource
	completed int
}

func (r *hotfixScanRepo) GetCurrentItemRunSource(context.Context, int64, int64, int64, int64) (*entity.ExptCurrentItemRunSource, error) {
	return r.source, nil
}

func (r *hotfixScanRepo) CompleteCurrentItemRun(context.Context, *entity.ExptCurrentItemRunSource, *entity.ExptCurrentItemRunWrite) (bool, error) {
	r.completed++
	return true, nil
}

func (r *hotfixScanRepo) StoreCurrentTurnRunResult(context.Context, *entity.ExptCurrentItemRunSource, *entity.ExptTurnResultRunLog) (bool, error) {
	return false, nil
}

func TestHotfixSandboxSweepUsesExperimentRecordSpace(t *testing.T) {
	ctrl := gomock.NewController(t)
	updated := time.Now()
	source := &entity.ExptCurrentItemRunSource{
		Current: entity.ExptItemCurrentRequest{
			SpaceID: 3, ExptID: 5, RunID: 7, ItemID: 11, ItemVersionID: 13,
			RequestID: "rq1", Mode: entity.EvaluationModeSubmit, Status: entity.ItemRunState_Processing, UpdatedAt: &updated,
		},
		Turns: map[int64]*entity.ExptTurnResultRunLog{
			0: {
				SpaceID: 3, ExptID: 5, ExptRunID: 7, ItemID: 11, ItemVersionID: 13,
				TurnID: 0, TargetResultID: 61, Status: entity.TurnRunState_Processing,
			},
		},
	}
	scan := &hotfixScanRepo{source: source}
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	refs := repomocks.NewMockIExptItemRefRepo(ctrl)
	refs.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), int64(3), int64(5), []int64{11}).Return([]*entity.ExptItemRef{{
		SpaceID: 3, ExptID: 5, ItemID: 11, ItemVersionID: 13,
		ItemConfig: &entity.ExptItemConfig{TargetSourceSpaceID: 200},
	}}, nil).Times(0)
	target.EXPECT().CheckSandboxTerminated(gomock.Any(), int64(3), []int64{61}).Return([]int64{61}, map[int64]string{61: "Failed"})
	target.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), int64(3), []int64{61}, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	scheduler := &ExptSchedulerImpl{
		ExptItemResultRepo: scan, evalTargetService: target, exptItemRefRepo: refs,
		itemLocker: func() lock.ILocker {
			mutex := lockmocks.NewMockILocker(ctrl)
			mutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), 5*time.Second, time.Minute).DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
				child, cancel := context.WithCancel(ctx)
				return true, child, cancel, nil
			})
			mutex.EXPECT().Unlock(gomock.Any()).Return(true, nil)
			return mutex
		},
	}
	event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 5, ExptRunID: 7}
	item := &entity.ExptEvalItem{Source: source, ItemID: 11, ExptID: 5, State: entity.ItemRunState_Processing}
	expt := &entity.Experiment{ID: 5, SpaceID: 3, TargetSpaceID: 100, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig}

	alives, terminated, err := scheduler.sweepCurrentSandboxItems(context.Background(), event, []*entity.ExptEvalItem{item}, expt)
	require.NoError(t, err)
	require.Empty(t, alives)
	require.Len(t, terminated, 1)
	require.Equal(t, 1, scan.completed)
}

// F4: a run-frozen item budget must win over the experiment's current config.
func TestHotfixRunFrozenRetryBudgetBeatsExperimentConfig(t *testing.T) {
	for _, tc := range []struct {
		name        string
		db, retry   int32
		wantRetry   bool
		wantRetries int32
	}{
		{name: "run_budget_one_consumed", db: 1, retry: 1, wantRetry: false, wantRetries: 1},
		{name: "run_budget_one_remaining", db: 0, retry: 0, wantRetry: true, wantRetries: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, port, expts, source, event, held := retryBudgetHandler(t)
			source.Current.RetryTimes = tc.db
			// RunExperiment(item_retry_num=1) freezes MaxRetryTimes=1;
			// the experiment config was later changed to 3 retries.
			event.RetryTimes, event.MaxRetryTimes = int(tc.retry), 1
			port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(source, nil)
			expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) {
				require.True(t, *held)
				return &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}}, nil
			})
			svc.configer.(*confmocks.MockIConfiger).EXPECT().GetErrRetryConf(gomock.Any(), int64(9), gomock.Any()).Return(&entity.RetryConf{RetryTimes: 3})
			var written *entity.ExptCurrentItemRunWrite
			port.EXPECT().CompleteCurrentItemRun(gomock.Any(), source, gomock.Any()).DoAndReturn(func(_ context.Context, _ *entity.ExptCurrentItemRunSource, w *entity.ExptCurrentItemRunWrite) (bool, error) {
				require.True(t, *held)
				written = w
				return true, nil
			})
			svc.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), nil).Return(nil).AnyTimes()
			err := svc.handleCurrentEventErr(context.Background(), event, errors.New("execution failed"), port, &currentItemExecution{Source: source})
			require.NoError(t, err)
			require.NotNil(t, written)
			require.Equal(t, tc.wantRetry, written.RetryOnly)
			require.Equal(t, tc.wantRetries, written.RetryTimes)
			require.False(t, *held)
		})
	}
}
