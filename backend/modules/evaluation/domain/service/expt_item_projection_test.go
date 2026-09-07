// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

type itemProjectionTestRepo struct {
	repo.IExptTurnResultRepo
	turn         *entity.ExptTurnResult
	refs         []*entity.ExptTurnEvaluatorResultRef
	currentRun   int64
	legacyWrites int
}

func (r *itemProjectionTestRepo) GetItemTurnResults(context.Context, int64, int64, int64) ([]*entity.ExptTurnResult, error) {
	copy := *r.turn
	return []*entity.ExptTurnResult{&copy}, nil
}

func (r *itemProjectionTestRepo) SaveTurnResults(_ context.Context, rows []*entity.ExptTurnResult) error {
	r.turn = rows[0]
	r.legacyWrites++
	return nil
}

func (r *itemProjectionTestRepo) CreateTurnEvaluatorRefs(_ context.Context, refs []*entity.ExptTurnEvaluatorResultRef) error {
	for _, ref := range refs {
		found := false
		for i, existing := range r.refs {
			if existing.EvaluatorVersionID == ref.EvaluatorVersionID && existing.Alias == ref.Alias && existing.InlineKey == ref.InlineKey {
				r.refs[i] = ref
				found = true
			}
		}
		if !found {
			r.refs = append(r.refs, ref)
		}
	}
	return nil
}

func (r *itemProjectionTestRepo) ApplyItemRunResults(_ context.Context, _, runID, _, _ int64, rows []*entity.ExptTurnResult, refs []*entity.ExptTurnEvaluatorResultRef) (bool, error) {
	if runID != r.currentRun {
		return false, nil
	}
	r.turn = rows[0]
	r.refs = refs
	return true, nil
}

func (r *itemProjectionTestRepo) BatchGetTurnEvaluatorResultRef(context.Context, int64, []int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	return r.refs, nil
}

func (r *itemProjectionTestRepo) GetTurnEvaluatorResultRefByExptID(context.Context, int64, int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	return r.refs, nil
}

func TestRetryFailure_ItemProjectionDropsPreviousGenerationFromReaders(t *testing.T) {
	for _, tt := range []struct {
		name      string
		targetID  int64
		runRefs   *entity.EvaluatorResults
		wantIDs   []int64
		wantScore *float64
	}{
		{name: "R3 pre-record failure", targetID: 202},
		{name: "same target across runs", targetID: 201, runRefs: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 301}}, wantIDs: []int64{301}, wantScore: gptr.Of(0.25)},
		{name: "no target", targetID: 0, runRefs: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 301}}, wantIDs: []int64{301}, wantScore: gptr.Of(0.25)},
		{name: "only current successful score remains", targetID: 202, runRefs: &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 402, RecordID: 302}}}, wantIDs: []int64{302}, wantScore: gptr.Of(0.5)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ctx := context.Background()
			itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
			underlying := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			stats := repoMocks.NewMockIExptStatsRepo(ctrl)
			idgen := idgenmocks.NewMockIIDGenerator(ctrl)
			recordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
			projection := &itemProjectionTestRepo{
				IExptTurnResultRepo: underlying, currentRun: 103,
				turn: &entity.ExptTurnResult{ID: 21, SpaceID: 3, ExptID: 1, ExptRunID: 102, ItemID: 10, TurnID: 20, TargetResultID: 202, WeightedScore: gptr.Of(0.25)},
				refs: []*entity.ExptTurnEvaluatorResultRef{{ID: 31, SpaceID: 3, ExptID: 1, ExptTurnResultID: 21, EvaluatorVersionID: 401, EvaluatorResultID: 301}},
			}
			itemRepo.EXPECT().GetItemRunLog(gomock.Any(), int64(1), int64(103), int64(10), int64(3)).Return(&entity.ExptItemResultRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, Status: int32(entity.ItemRunState_Fail), ResultState: int32(entity.ExptItemResultStateLogged)}, nil)
			underlying.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(1), int64(103), int64(10), int64(3)).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, TurnID: 20, TargetResultID: tt.targetID, EvaluatorResultIds: tt.runRefs, Status: entity.TurnRunState_Fail}}, nil)
			itemRepo.EXPECT().GetItemTurnResults(gomock.Any(), int64(3), int64(1), int64(10)).DoAndReturn(func(context.Context, int64, int64, int64) ([]*entity.ExptTurnResult, error) {
				return projection.GetItemTurnResults(ctx, 1, 10, 3)
			})
			itemRepo.EXPECT().BatchGet(gomock.Any(), int64(3), int64(1), []int64{10}).Return([]*entity.ExptItemResult{{ExptRunID: 103, Status: entity.ItemRunState_Processing}}, nil).AnyTimes()
			itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			stats.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			idgen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, n int) ([]int64, error) {
				ids := make([]int64, n)
				for i := range ids {
					ids[i] = 700 + int64(i)
				}
				return ids, nil
			}).AnyTimes()
			records := map[int64]*entity.EvaluatorRecord{
				301: {ID: 301, EvaluatorVersionID: 401, ExperimentRunID: 101, Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.25), Reasoning: "T1 old score"}}},
				302: {ID: 302, EvaluatorVersionID: 402, ExperimentRunID: 103, Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.5), Reasoning: "T2 current score"}}},
			}
			recordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(_ context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
				var got []*entity.EvaluatorRecord
				for _, id := range ids {
					got = append(got, records[id])
				}
				return got, nil
			}).AnyTimes()
			recordSvc.EXPECT().BatchGetEvaluatorRecordForAggr(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ids []int64) ([]*entity.EvaluatorRecordAggr, error) {
				var got []*entity.EvaluatorRecordAggr
				for _, id := range ids {
					got = append(got, &entity.EvaluatorRecordAggr{ID: id, Status: entity.EvaluatorRunStatusSuccess, Score: records[id].EvaluatorOutputData.EvaluatorResult.Score})
				}
				return got, nil
			}).AnyTimes()
			expt := buildRetryFailureSingleSetExpt(3, 401, 402)
			svc := &ExptResultServiceImpl{ExptItemResultRepo: itemRepo, ExptTurnResultRepo: projection, ExptStatsRepo: stats, idgen: idgen, evaluatorRecordService: recordSvc, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
			_, err := svc.RecordItemRunLogs(ctx, 1, 103, 10, 3, expt)
			require.NoError(t, err)
			assert.Equal(t, tt.targetID, projection.turn.TargetResultID)
			assert.Equal(t, tt.wantScore, projection.turn.WeightedScore)
			var ids []int64
			for _, ref := range projection.refs {
				ids = append(ids, ref.EvaluatorResultID)
			}
			assert.ElementsMatch(t, tt.wantIDs, ids)
			builder := &ExptResultBuilder{exptDO: expt, SpaceID: 3, turnResultDO: []*entity.ExptTurnResult{projection.turn}, ExptTurnResultRepo: projection, evaluatorRecordService: recordSvc, ItemIDTurnID2TurnResultID: map[int64]map[int64]int64{10: {20: 21}}}
			require.NoError(t, builder.buildEvaluatorResult(ctx))
			output := builder.getTurnEvaluatorResult(ctx, 10, 20)
			var readIDs []int64
			for _, record := range output.EvaluatorRecords {
				readIDs = append(readIDs, record.ID)
			}
			assert.ElementsMatch(t, tt.wantIDs, readIDs)
			helper := &exportCSVHelper{reportEvaluatorCount: 2, colEvaluators: []*entity.ColumnEvaluator{{EvaluatorVersionID: 401}, {EvaluatorVersionID: 402}}}
			csv, err := helper.buildRowsForItems(ctx, []*entity.ItemResult{{ItemID: 10, TurnResults: []*entity.TurnResult{{TurnID: 20, ExperimentResults: []*entity.ExperimentResult{{ExperimentID: 1, Payload: &entity.ExperimentTurnPayload{EvalSet: &entity.TurnEvalSet{Turn: &entity.Turn{FieldDataList: []*entity.FieldData{}}}, EvaluatorOutput: output}}}}}}})
			require.NoError(t, err)
			require.Len(t, csv, 1)
			if tt.wantScore == nil {
				assert.Equal(t, []string{"10", "", "", "", "", "", ""}, csv[0])
			}
			aggr := &ExptAggrResultServiceImpl{exptTurnResultRepo: projection, evaluatorRecordService: recordSvc}
			groups, err := aggr.computeEvaluatorAggrGroup(ctx, 3, 1)
			require.NoError(t, err)
			assert.Len(t, groups, len(tt.wantIDs))
		})
	}
}

func TestRecordItemRunLogs_DoesNotPublishRefsBeforeApply(t *testing.T) {
	for _, stage := range []string{"idgen", "apply-error", "apply-false", "success"} {
		t.Run(stage, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
			turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			idgen := idgenmocks.NewMockIIDGenerator(ctrl)
			recordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
			failure := errors.New("projection failed")
			itemRepo.EXPECT().GetItemRunLog(gomock.Any(), int64(1), int64(103), int64(10), int64(3)).Return(&entity.ExptItemResultRunLog{ResultState: int32(entity.ExptItemResultStateLogged)}, nil)
			turnRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(1), int64(103), int64(10), int64(3)).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, TurnID: 20, TargetResultID: 201, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 301}}}}, nil)
			itemRepo.EXPECT().GetItemTurnResults(gomock.Any(), int64(3), int64(1), int64(10)).Return([]*entity.ExptTurnResult{{ID: 21, SpaceID: 3, ExptID: 1, ItemID: 10, TurnID: 20}}, nil)
			recordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301}, false, false).Return(nil, nil)
			if stage == "idgen" {
				idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return(nil, failure)
			} else {
				idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{701}, nil)
				applyErr := error(nil)
				if stage == "apply-error" {
					applyErr = failure
				}
				turnRepo.EXPECT().ApplyItemRunResults(gomock.Any(), int64(1), int64(103), int64(10), int64(3), gomock.Any(), gomock.Any()).Return(stage == "success", applyErr)
			}
			svc := &ExptResultServiceImpl{ExptItemResultRepo: itemRepo, ExptTurnResultRepo: turnRepo, idgen: idgen, evaluatorRecordService: recordSvc, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
			refs, err := svc.RecordItemRunLogs(context.Background(), 1, 103, 10, 3, nil)
			if stage == "idgen" || stage == "apply-error" {
				require.ErrorIs(t, err, failure)
			} else {
				require.NoError(t, err)
			}
			if stage == "success" {
				require.Len(t, refs, 1)
				require.Equal(t, int64(701), refs[0].ID)
				require.Equal(t, int64(301), refs[0].EvaluatorResultID)
			} else {
				require.Nil(t, refs)
			}
		})
	}
}
