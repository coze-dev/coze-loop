// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

type terminalRetryPreparer interface {
	prepareTerminalCandidatesAndReusableRefs(context.Context, *entity.Experiment, int64, entity.ExptRunMode, []int64, EvaluatorRecordService) (*entity.ManualRetryPreparation, error)
}

func TestManualRetryPrepareSuccessfulStages(t *testing.T) {
	for _, tc := range []struct {
		name            string
		perSetVersion   *int64
		wrongTopVersion bool
		wrongEvalSource bool
	}{
		{name: "success_without_score"},
		{name: "rpc_error_no_write"},
		{name: "wrong_target_source"},
		{name: "target_source_top41_cfg99", perSetVersion: gptr.Of(int64(99))},
		{name: "target_source_top41_cfg0", perSetVersion: gptr.Of(int64(0))},
		{name: "target_source_wrong_top_record_cfg99", perSetVersion: gptr.Of(int64(99)), wrongTopVersion: true},
		{name: "target_source_wrong_top_record_cfg0", perSetVersion: gptr.Of(int64(0)), wrongTopVersion: true},
		{name: "target_source_wrong_evaluator_cfg0", perSetVersion: gptr.Of(int64(0)), wrongEvalSource: true},
	} {
		kind := tc.name
		dependencyError := kind == "rpc_error_no_write"
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			refs := repomocks.NewMockIExptItemRefRepo(ctrl)
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			eval := svcmocks.NewMockEvaluatorRecordService(ctrl)
			manager := &ExptMangerImpl{itemResultRepo: items, turnResultRepo: turns, itemRefRepo: refs, evalTargetService: target}
			preparer, ok := any(manager).(terminalRetryPreparer)
			require.True(t, ok, "manual terminal preparation must freeze stages before resetting references")
			items.EXPECT().ScanItemResults(gomock.Any(), int64(7), int64(0), int64(500), []int32{3}, int64(9)).DoAndReturn(func(ctx context.Context, _, _, _ int64, _ []int32, _ int64) ([]*entity.ExptItemResult, int64, error) {
				require.True(t, contexts.CtxWriteDB(ctx))
				return []*entity.ExptItemResult{{ID: 101, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, Status: entity.ItemRunState_Fail, Ext: map[string]string{"other": "keep"}}}, 0, nil
			})
			items.EXPECT().MGetItemRunLog(gomock.Any(), int64(7), int64(8), []int64{11}, int64(9)).Return([]*entity.ExptItemResultRunLog{{ID: 201, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, Status: 3, ResultState: 1, RetryTimes: 3}}, nil)
			itemConfig := &entity.ExptItemConfig{TargetSourceSpaceID: 19}
			if tc.perSetVersion != nil {
				itemConfig.EvalTargetConf = &entity.ItemTargetConf{TargetVersionID: *tc.perSetVersion}
			}
			refs.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(9), int64(7), int64(11)).Return(&entity.ExptItemRef{ID: 501, SpaceID: 9, ExptID: 7, ItemID: 11, ItemVersionID: 91, ItemConfig: itemConfig}, nil)
			turns.EXPECT().GetItemTurnResults(gomock.Any(), int64(7), int64(11), int64(9)).Return([]*entity.ExptTurnResult{{ID: 301, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, TurnID: 21, Status: 2, TargetResultID: 31, WeightedScore: gptr.Of(0.8)}, {ID: 302, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, TurnID: 22, Status: 1, WeightedScore: gptr.Of(0.0)}}, nil)
			turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(7), int64(8), int64(11), int64(9)).Return([]*entity.ExptTurnResultRunLog{{ID: 401, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, TurnID: 21, TargetResultID: 31, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: 61, 52: 62}}}, {ID: 402, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, TurnID: 22, Status: entity.TurnRunState_Success}}, nil)
			turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(9), []int64{301, 302}).Return([]*entity.ExptTurnEvaluatorResultRef{{ID: 701, SpaceID: 9, ExptID: 7, ExptTurnResultID: 301, EvaluatorVersionID: 51, EvaluatorResultID: 61}, {ID: 702, SpaceID: 9, ExptID: 7, ExptTurnResultID: 301, EvaluatorVersionID: 52, EvaluatorResultID: 62}}, nil)
			var rpcErr error
			if dependencyError {
				rpcErr = errors.New("record unavailable")
			}
			targetItem := int64(11)
			if kind == "wrong_target_source" {
				targetItem = 99
			}
			recordVersion := int64(41)
			if tc.wrongTopVersion {
				recordVersion = 99
			}
			target.EXPECT().GetRecordByID(gomock.Any(), int64(19), int64(31)).Return(&entity.EvalTargetRecord{ID: 31, SpaceID: 19, TargetVersionID: recordVersion, ExperimentRunID: 8, ItemID: targetItem, TurnID: 21, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, rpcErr)
			if !dependencyError {
				evalItem := int64(11)
				if tc.wrongEvalSource {
					evalItem = 99
				}
				eval.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{{ID: 61, ExperimentID: 7, ExperimentRunID: 8, ItemID: evalItem, TurnID: 21, EvaluatorVersionID: 51, Status: entity.EvaluatorRunStatusSuccess}, {ID: 62, Status: entity.EvaluatorRunStatusFail}}, nil).AnyTimes()
			}
			got, err := preparer.prepareTerminalCandidatesAndReusableRefs(context.Background(), &entity.Experiment{ID: 7, SpaceID: 9, TargetVersionID: 41, ExptType: entity.ExptType_Offline}, 8, entity.EvaluationModeFailRetry, nil, eval)
			if kind == "wrong_target_source" || tc.wrongTopVersion {
				require.ErrorContains(t, err, "target source")
				require.Nil(t, got)
				return
			}
			if dependencyError {
				require.ErrorContains(t, err, "record unavailable")
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Len(t, got.Items, 1)
			item := got.Items[0]
			require.Equal(t, int64(8), item.PreviousRunID)
			require.Equal(t, int32(3), item.CurrentItemLog.RetryTimes)
			require.Equal(t, int64(31), item.Turns[0].TargetResultID)
			if tc.wrongEvalSource {
				require.Nil(t, item.Turns[0].EvaluatorResults)
				require.Empty(t, item.Turns[0].KeepRefIDs)
				return
			}
			require.Equal(t, map[int64]int64{51: 61}, item.Turns[0].EvaluatorResults.EvalVerIDToResID)
			require.Equal(t, []int64{701}, item.Turns[0].KeepRefIDs)
			require.True(t, item.Turns[1].PreserveSuccess)
			require.Equal(t, 0.0, *item.Turns[1].SourceTurn.WeightedScore)
			require.Equal(t, "keep", item.SourceItem.Ext["other"])
		})
	}
}

func TestManualRetryPrepareManualSelection(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	refs := repomocks.NewMockIExptItemRefRepo(ctrl)
	preparer, ok := any(&ExptMangerImpl{itemResultRepo: items, turnResultRepo: turns, itemRefRepo: refs}).(terminalRetryPreparer)
	require.True(t, ok)
	items.EXPECT().BatchGet(gomock.Any(), int64(9), int64(7), []int64{11, 12, 13}).Return([]*entity.ExptItemResult{{ID: 101, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, Status: entity.ItemRunState_Success}, {ItemID: 12, Status: entity.ItemRunState_Processing}, {ItemID: 13, Status: entity.ItemRunState_Queueing}}, nil)
	items.EXPECT().MGetItemRunLog(gomock.Any(), int64(7), int64(8), []int64{11}, int64(9)).Return([]*entity.ExptItemResultRunLog{{ID: 201, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, Status: 2, ResultState: 1}}, nil)
	refs.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(9), int64(7), int64(11)).Return(&entity.ExptItemRef{SpaceID: 9, ExptID: 7, ItemID: 11, ItemVersionID: 91}, nil)
	turns.EXPECT().GetItemTurnResults(gomock.Any(), int64(7), int64(11), int64(9)).Return([]*entity.ExptTurnResult{{ID: 301, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, TurnID: 21, Status: 1, TargetResultID: 31}}, nil)
	turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(7), int64(8), int64(11), int64(9)).Return([]*entity.ExptTurnResultRunLog{{ID: 401, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, ItemVersionID: 91, TurnID: 21}}, nil)
	turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(9), []int64{301}).Return(nil, nil)
	got, err := preparer.prepareTerminalCandidatesAndReusableRefs(context.Background(), &entity.Experiment{ID: 7, SpaceID: 9}, 8, entity.EvaluationModeRetryItems, []int64{13, 11, 12, 11}, nil)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, []*entity.ManualRetryRejectedItem{{ItemID: 12, Reason: "processing"}, {ItemID: 13, Reason: "queueing"}}, got.RejectedItems)
	require.Zero(t, got.Items[0].Turns[0].TargetResultID)
	require.False(t, got.Items[0].Turns[0].PreserveSuccess)
}
