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

	"github.com/coze-dev/coze-loop/backend/infra/db"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type retrySourceErrorTurnDAO struct {
	mysql.ExptTurnResultDAO
	canonical []*model.ExptTurnResult
	logs      map[[3]int64]*model.ExptTurnResultRunLog
}

func (d *retrySourceErrorTurnDAO) BatchGet(context.Context, int64, int64, []int64, ...db.Option) ([]*model.ExptTurnResult, error) {
	return d.canonical, nil
}

func (d *retrySourceErrorTurnDAO) BatchCreateNXRunLog(_ context.Context, rows []*model.ExptTurnResultRunLog, _ ...db.Option) error {
	for _, row := range rows {
		key := [3]int64{row.ExptRunID, row.ItemID, row.TurnID}
		if d.logs[key] == nil {
			copied := *row
			d.logs[key] = &copied
		}
	}
	return nil
}

func (d *retrySourceErrorTurnDAO) UpdateTurnRunLogWithItemIDs(_ context.Context, spaceID, exptID, runID int64, itemIDs []int64, fields map[string]any, _ ...db.Option) error {
	for _, row := range d.logs {
		for _, itemID := range itemIDs {
			if row.SpaceID != spaceID || row.ExptID != exptID || row.ExptRunID != runID || row.ItemID != itemID {
				continue
			}
			if status, ok := fields["status"].(int32); ok {
				row.Status = status
			}
			if errMsg, ok := fields["err_msg"].([]byte); ok {
				row.ErrMsg = &errMsg
			}
		}
	}
	return nil
}

func (d *retrySourceErrorTurnDAO) GetItemTurnRunLogs(_ context.Context, exptID, runID, itemID, spaceID int64, _ ...db.Option) ([]*model.ExptTurnResultRunLog, error) {
	var rows []*model.ExptTurnResultRunLog
	for _, row := range d.logs {
		if row.SpaceID == spaceID && row.ExptID == exptID && row.ExptRunID == runID && row.ItemID == itemID {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func TestRetryFailure_SourceReadErrorOuterClosurePreservesTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	const exptID, runID, spaceID, itemID = int64(1), int64(103), int64(3), int64(10)
	canonical := []*entity.ExptTurnResult{
		{ID: 21, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, TurnID: 20, TargetResultID: 201},
		{ID: 22, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, TurnID: 21, TargetResultID: 202},
	}
	refs := []*entity.ExptTurnEvaluatorResultRef{
		{SpaceID: spaceID, ExptID: exptID, ExptTurnResultID: 21, EvaluatorVersionID: 401, EvaluatorResultID: 301},
		{SpaceID: spaceID, ExptID: exptID, ExptTurnResultID: 22, EvaluatorVersionID: 401, EvaluatorResultID: 302},
	}
	records := []*entity.EvaluatorRecord{
		{ID: 301, SpaceID: spaceID, ExperimentID: exptID, ExperimentRunID: 101, ItemID: itemID, TurnID: 20, EvaluatorVersionID: 401, Status: entity.EvaluatorRunStatusSuccess},
		{ID: 302, SpaceID: spaceID, ExperimentID: exptID, ExperimentRunID: 102, ItemID: itemID, TurnID: 21, EvaluatorVersionID: 401, Status: entity.EvaluatorRunStatusSuccess},
	}
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, n int) ([]int64, error) {
		ids := make([]int64, n)
		for i := range ids {
			ids[i] = 800 + int64(i)
		}
		return ids, nil
	}).AnyTimes()
	dao := &retrySourceErrorTurnDAO{logs: make(map[[3]int64]*model.ExptTurnResultRunLog)}
	for _, tr := range canonical {
		dao.canonical = append(dao.canonical, convert.NewExptTurnResultConvertor().DO2PO(tr))
	}
	realTurnRepo := experimentrepo.NewExptTurnResultRepo(idgen, dao, nil)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	turnRepo.EXPECT().GetItemTurnResults(gomock.Any(), exptID, itemID, spaceID).Return(canonical, nil).AnyTimes()
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{21, 22}).Return(refs, nil).AnyTimes()
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(realTurnRepo.BatchCreateNXRunLog).AnyTimes()
	terminalWrites := 0
	turnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), spaceID, exptID, runID, []int64{itemID}, entity.TurnRunState_Fail).DoAndReturn(func(ctx context.Context, spaceID, exptID, runID int64, ids []int64, state entity.TurnRunState) error {
		terminalWrites++
		return realTurnRepo.CreateOrUpdateItemsTurnRunLogStatus(ctx, spaceID, exptID, runID, ids, state)
	}).AnyTimes()
	turnRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), exptID, runID, itemID, spaceID).DoAndReturn(realTurnRepo.GetItemTurnRunLogs).AnyTimes()
	turnRepo.EXPECT().CreateTurnEvaluatorRefs(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	turnRepo.EXPECT().SaveTurnResults(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	sourceErr := errors.New("source run log unavailable")
	sourceUnavailable := true
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, gomock.Any(), []int64{itemID}, spaceID).DoAndReturn(func(_ context.Context, _, sourceRunID int64, _ []int64, _ int64) ([]*entity.ExptTurnResultRunLog, error) {
		if sourceRunID == 102 && sourceUnavailable {
			return nil, sourceErr
		}
		turnID, targetID, recordID := int64(20), int64(201), int64(301)
		if sourceRunID == 102 {
			turnID, targetID, recordID = 21, 202, 302
		}
		return []*entity.ExptTurnResultRunLog{{SpaceID: spaceID, ExptID: exptID, ExptRunID: sourceRunID, ItemID: itemID, TurnID: turnID, TargetResultID: targetID, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: recordID}}}}, nil
	}).AnyTimes()
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), spaceID, gomock.Any()).DoAndReturn(func(_ context.Context, _, id int64) (*entity.EvalTargetRecord, error) {
		return &entity.EvalTargetRecord{ID: id, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{}}, nil
	}).AnyTimes()
	recordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	recordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(_ context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
		var got []*entity.EvaluatorRecord
		for _, id := range ids {
			for _, record := range records {
				if record.ID == id {
					got = append(got, record)
				}
			}
		}
		return got, nil
	}).AnyTimes()
	itemLog := &entity.ExptItemResultRunLog{ExptID: exptID, ExptRunID: runID, ItemID: itemID, SpaceID: spaceID, Status: int32(entity.ItemRunState_Processing)}
	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	applyItemFields := func(_ context.Context, _, _ int64, _ []int64, fields map[string]any, _ int64) error {
		if status, ok := fields["status"].(int32); ok {
			itemLog.Status = status
		}
		if state, ok := fields["result_state"].(int32); ok {
			itemLog.ResultState = state
		}
		return nil
	}
	itemRepo.EXPECT().UpdateItemRunLogIfNotTerminal(gomock.Any(), exptID, runID, []int64{itemID}, gomock.Any(), spaceID).DoAndReturn(applyItemFields).AnyTimes()
	itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, gomock.Any(), spaceID).DoAndReturn(applyItemFields).AnyTimes()
	itemRepo.EXPECT().GetItemRunLog(gomock.Any(), exptID, runID, itemID, spaceID).Return(itemLog, nil).AnyTimes()
	itemRepo.EXPECT().GetItemTurnResults(gomock.Any(), spaceID, exptID, itemID).Return(canonical, nil).AnyTimes()
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID, Status: entity.ItemRunState_Processing}}, nil).AnyTimes()
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, gomock.Any()).Return(nil).AnyTimes()
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)
	stats.EXPECT().ArithOperateCount(gomock.Any(), exptID, spaceID, gomock.Any()).Return(nil).AnyTimes()
	configer := configmocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetErrRetryConf(gomock.Any(), spaceID, gomock.Any()).Return(&entity.RetryConf{RetryTimes: 1}).AnyTimes()
	configer.EXPECT().BuildEvalExt(gomock.Any(), spaceID, gomock.Any()).Return(nil).AnyTimes()
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitItemExecResult(spaceID, int64(entity.EvaluationModeFailRetry), true, false, gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	expt := buildRetryFailureSingleSetExpt(spaceID, 401)
	resultSvc := &ExptResultServiceImpl{ExptItemResultRepo: itemRepo, ExptTurnResultRepo: turnRepo, ExptStatsRepo: stats, idgen: idgen, evaluatorRecordService: recordSvc, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
	mode := &ExptRecordEvalModeFailRetry{resultSvc: resultSvc, exptTurnResultRepo: turnRepo, idgen: idgen, evalTargetService: targetSvc, evaluatorRecordSvc: recordSvc}
	eiec := &entity.ExptItemEvalCtx{Expt: expt, Event: &entity.ExptItemEvalEvent{ExptID: exptID, ExptRunID: runID, EvalSetItemID: itemID, SpaceID: spaceID, ExptRunMode: entity.EvaluationModeFailRetry, RetryTimes: 1}, ExistItemEvalResult: &entity.ExptItemEvalResult{ItemResultRunLog: itemLog, TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)}}
	outer := &ExptItemEventEvalServiceImpl{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, configer: configer, metric: metric}
	executionCalls := 0
	handler := outer.HandleEventErr(func(ctx context.Context, _ *entity.ExptItemEvalEvent) error {
		if err := mode.PreEval(ctx, eiec); err != nil {
			return err
		}
		executionCalls++
		return nil
	})
	outerErr := handler(ctx, eiec.Event)
	if outerErr == nil {
		_, err := resultSvc.RecordItemRunLogs(ctx, exptID, runID, itemID, spaceID, expt)
		require.NoError(t, err)
	}
	assert.ErrorIs(t, outerErr, sourceErr)
	assert.Zero(t, executionCalls, "dependency failure must not enter target/evaluator execution")
	assert.Zero(t, terminalWrites, "exhausted snapshot read must not create empty failed runlogs")
	assert.Empty(t, dao.logs, "no replacement snapshot before dependency recovery")
	assert.Equal(t, int64(201), canonical[0].TargetResultID)
	assert.Equal(t, int64(202), canonical[1].TargetResultID)
	if outerErr == nil {
		return
	}
	sourceUnavailable = false
	next := &entity.ExptItemEvalCtx{Expt: expt, Event: &entity.ExptItemEvalEvent{ExptID: exptID, ExptRunID: 104, EvalSetItemID: itemID, SpaceID: spaceID, ExptRunMode: entity.EvaluationModeFailRetry}, EvalSetItem: &entity.EvaluationSetItem{ItemID: itemID, Turns: []*entity.Turn{{ID: 20, ItemID: itemID}, {ID: 21, ItemID: itemID}}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)}}
	require.NoError(t, mode.PreEval(ctx, next))
	executor := &ExptItemEvalCtxExecutor{ItemResultRepo: itemRepo, Configer: configer, evalTargetService: targetSvc, evaluatorRecordService: recordSvc}
	turnEval := &DefaultExptTurnEvaluationImpl{evalTargetService: targetSvc}
	for _, turn := range next.EvalSetItem.Turns {
		wantTarget, wantRecord := map[int64]int64{20: 201, 21: 202}[turn.ID], map[int64]int64{20: 301, 21: 302}[turn.ID]
		runlog := next.GetExistTurnResultRunLog(turn.ID)
		require.Equal(t, wantTarget, runlog.TargetResultID)
		require.Equal(t, map[int64]int64{401: wantRecord}, runlog.EvaluatorResultIds.EvalVerIDToResID)
		turnCtx, err := executor.buildExptTurnEvalCtx(ctx, turn, next, nil)
		require.NoError(t, err)
		target, err := turnEval.CallTarget(ctx, turnCtx)
		require.NoError(t, err)
		assert.Equal(t, wantTarget, target.ID)
		evaluators, err := turnEval.CallEvaluators(ctx, turnCtx, target)
		require.NoError(t, err)
		require.Len(t, evaluators, 1)
		assert.Equal(t, wantRecord, evaluators[0].ID)
	}
}
