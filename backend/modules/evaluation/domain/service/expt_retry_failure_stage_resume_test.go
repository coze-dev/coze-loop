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

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func buildRetryFailureSingleSetExpt(spaceID int64, evaluatorVersionIDs ...int64) *entity.Experiment {
	expt := buildMockExpt()
	expt.SpaceID = spaceID
	expt.TargetSpaceID = 0
	expt.Evaluators = make([]*entity.Evaluator, 0, len(evaluatorVersionIDs))
	expt.EvalConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{EvaluatorConcurNum: gptr.Of(1)}
	for _, versionID := range evaluatorVersionIDs {
		expt.Evaluators = append(expt.Evaluators, &entity.Evaluator{
			ID: versionID, EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: versionID},
		})
		expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf = append(
			expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf,
			&entity.EvaluatorConf{
				EvaluatorVersionID: versionID,
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{},
					TargetAdapter:  &entity.FieldAdapter{},
				},
			},
		)
	}
	return expt
}

func TestShouldSkipTargetNode_FallsBackToLoadedTargetType(t *testing.T) {
	expt := &entity.Experiment{
		TargetVersionID: 100,
		ExptType:        entity.ExptType_Offline,
		TargetType:      0,
		Target: &entity.EvalTarget{
			EvalTargetType: entity.EvalTargetTypeCustomRPCServerOnline,
		},
	}

	assert.True(t, shouldSkipTargetNode(expt))
}

func TestFailRetrySelectTurnRunLogRefs_TargetRequirementControlsEvaluatorReuse(t *testing.T) {
	const evaluatorRecordID = int64(1001)

	tests := []struct {
		name           string
		targetRequired bool
		wantReuse      bool
	}{
		{name: "target required but missing drops old evaluator refs", targetRequired: true, wantReuse: false},
		{name: "no target reuses successful evaluator refs", targetRequired: false, wantReuse: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			evalRecord := svcmocks.NewMockEvaluatorRecordService(ctrl)
			if tt.wantReuse {
				evalRecord.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{evaluatorRecordID}, false, false).
					Return([]*entity.EvaluatorRecord{{
						ID:                 evaluatorRecordID,
						EvaluatorVersionID: 10,
						Status:             entity.EvaluatorRunStatusSuccess,
					}}, nil)
			}

			targetID, evaluatorResults := failRetrySelectTurnRunLogRefs(
				context.Background(), 1, tt.targetRequired,
				&entity.ExptTurnResult{
					EvaluatorResults: &entity.EvaluatorResults{
						EvalVerIDToResID: map[int64]int64{10: evaluatorRecordID},
					},
				},
				nil,
				evalRecord,
				[]*entity.ExptTurnEvaluatorResultRef{{
					ExptTurnResultID: 0, EvaluatorVersionID: 10, EvaluatorResultID: evaluatorRecordID,
				}},
			)
			assert.Zero(t, targetID)
			if tt.wantReuse {
				require.NotNil(t, evaluatorResults)
				assert.Equal(t, map[int64]int64{10: evaluatorRecordID}, evaluatorResults.EvalVerIDToResID)
			} else {
				assert.Nil(t, evaluatorResults)
			}
		})
	}
}

func TestFailRetrySelectTurnRunLogRefs_DropsAmbiguousLegacyEvaluatorGroups(t *testing.T) {
	tests := []struct {
		name string
		tr   *entity.ExptTurnResult
		refs []*entity.ExptTurnEvaluatorResultRef
	}{
		{
			name: "missing identity metadata is not safe to reuse",
			tr: &entity.ExptTurnResult{ID: 1, EvaluatorResults: &entity.EvaluatorResults{
				EvalVerIDToResID: map[int64]int64{42: 4201},
			}},
			refs: nil,
		},
		{
			name: "alias record is not reused from a folded legacy entry",
			tr: &entity.ExptTurnResult{ID: 1, EvaluatorResults: &entity.EvaluatorResults{
				EvalVerIDToResID: map[int64]int64{42: 4202},
			}},
			refs: []*entity.ExptTurnEvaluatorResultRef{
				{ExptTurnResultID: 1, EvaluatorVersionID: 42, EvaluatorResultID: 4201, SourceType: int32(entity.EvaluatorRecordSourceTypeBuiltin), Alias: "a"},
				{ExptTurnResultID: 1, EvaluatorVersionID: 42, EvaluatorResultID: 4202, SourceType: int32(entity.EvaluatorRecordSourceTypeBuiltin), Alias: "b"},
			},
		},
		{
			name: "inline record is not reused from a folded legacy entry",
			tr: &entity.ExptTurnResult{ID: 1, EvaluatorResults: &entity.EvaluatorResults{
				EvalVerIDToResID: map[int64]int64{0: 9002},
			}},
			refs: []*entity.ExptTurnEvaluatorResultRef{
				{ExptTurnResultID: 1, EvaluatorVersionID: 0, EvaluatorResultID: 9001, SourceType: int32(entity.EvaluatorRecordSourceTypeInline), InlineKey: "inline-a"},
				{ExptTurnResultID: 1, EvaluatorVersionID: 0, EvaluatorResultID: 9002, SourceType: int32(entity.EvaluatorRecordSourceTypeInline), InlineKey: "inline-b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			evalRecord := svcmocks.NewMockEvaluatorRecordService(ctrl)

			targetID, evaluatorResults := failRetrySelectTurnRunLogRefs(
				context.Background(), 1, false, tt.tr, nil, evalRecord, tt.refs,
			)
			assert.Zero(t, targetID)
			assert.Nil(t, evaluatorResults)
		})
	}
}

type failRetryTurnUpdateMatcher struct {
	wantRunID    int64
	wantTargetID *int64
}

func (m failRetryTurnUpdateMatcher) Matches(x any) bool {
	got, ok := x.(map[string]any)
	if !ok || got["status"] != int32(entity.TurnRunState_Queueing) || got["expt_run_id"] != m.wantRunID {
		return false
	}
	value, exists := got["target_result_id"]
	if m.wantTargetID == nil {
		return !exists
	}
	return exists && value == *m.wantTargetID
}

func (m failRetryTurnUpdateMatcher) String() string {
	return "RetryFailure turn update with conditional target_result_id"
}

func TestExptFailRetryExec_ExptStart_PreservesSuccessfulTargetAndItemLogID(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID      = int64(1)
		newRunID    = int64(2)
		spaceID     = int64(3)
		itemID      = int64(10)
		turnID      = int64(20)
		targetID    = int64(30)
		itemVersion = int64(40)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(0), int64(50), spaceID).DoAndReturn(
		func(ctx context.Context, _ int64, _ []int32, _, _ int64, _ int64) ([]*entity.ExptTurnResult, int64, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "RetryFailure canonical snapshot must read from primary")
			return []*entity.ExptTurnResult{{
				ID: 100, ItemID: itemID, TurnID: turnID, ItemVersionID: itemVersion,
				Status: int32(entity.TurnRunState_Fail), TargetResultID: targetID, LogID: "turn-log",
			}}, int64(100), nil
		},
	)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(100), int64(50), spaceID).DoAndReturn(
		func(ctx context.Context, _ int64, _ []int32, _, _ int64, _ int64) ([]*entity.ExptTurnResult, int64, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "RetryFailure pagination must keep reading from primary")
			return nil, int64(0), nil
		},
	)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{
		ItemID: itemID, ItemVersionID: itemVersion + 1, LogID: "item-log",
	}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	success := entity.EvalTargetRunStatusSuccess
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{targetID}).Return([]*entity.EvalTargetRecord{{ID: targetID, Status: &success}}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	itemRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got []*entity.ExptItemResultRunLog) error {
		require.Len(t, got, 1)
		assert.Equal(t, "item-log", got[0].LogID)
		assert.Equal(t, itemVersion, got[0].ItemVersionID)
		return nil
	})
	itemRepo.EXPECT().FillItemRunLogLogIDIfEmpty(gomock.Any(), exptID, newRunID, spaceID, map[int64]string{itemID: "item-log"}).Return(nil)
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, map[string]any{
		"status": int32(entity.ItemRunState_Queueing), "expt_run_id": newRunID,
	}).Return(nil)
	turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, failRetryTurnUpdateMatcher{wantRunID: newRunID}).Return(nil)
	statsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{FailItemCnt: 1}, nil)
	statsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	configer.EXPECT().GetExptExecConf(gomock.Any(), spaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	exec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, exptStatsRepo: statsRepo,
		idgenerator: idgen, exptRepo: exptRepo, idem: idem, configer: configer,
		evalTargetService: targetSvc,
	}
	err := exec.ExptStart(context.Background(), &entity.ExptScheduleEvent{ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID}, buildMockExpt())
	require.NoError(t, err)
}

func TestExptFailRetryExec_ExptStart_ClearsFailedTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
		itemID   = int64(10)
		targetID = int64(30)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(0), int64(50), spaceID).Return([]*entity.ExptTurnResult{{
		ID: 100, ItemID: itemID, TurnID: 20, Status: int32(entity.TurnRunState_Fail), TargetResultID: targetID,
	}}, int64(100), nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(100), int64(50), spaceID).Return(nil, int64(0), nil)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID, LogID: "log"}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	failed := entity.EvalTargetRunStatusFail
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{targetID}).Return([]*entity.EvalTargetRecord{{ID: targetID, Status: &failed}}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	itemRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil)
	itemRepo.EXPECT().FillItemRunLogLogIDIfEmpty(gomock.Any(), exptID, newRunID, spaceID, map[int64]string{itemID: "log"}).Return(nil)
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, gomock.Any()).Return(nil)
	zero := int64(0)
	turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, gomock.Any(), spaceID, failRetryTurnUpdateMatcher{wantRunID: newRunID, wantTargetID: &zero}).Return(nil)
	statsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{FailItemCnt: 1}, nil)
	statsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	configer.EXPECT().GetExptExecConf(gomock.Any(), spaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	exec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, exptStatsRepo: statsRepo,
		idgenerator: idgen, exptRepo: exptRepo, idem: idem, configer: configer,
		evalTargetService: targetSvc,
	}
	err := exec.ExptStart(context.Background(), &entity.ExptScheduleEvent{ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID}, buildMockExpt())
	require.NoError(t, err)
}

func TestExptFailRetryExec_ExptStart_ReusesSuccessfulTargetFromCurrentRunOnRedrive(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID          = int64(1)
		newRunID        = int64(2)
		spaceID         = int64(3)
		itemID          = int64(10)
		turnID          = int64(20)
		oldTargetID     = int64(30)
		currentTargetID = int64(31)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(0), int64(50), spaceID).Return([]*entity.ExptTurnResult{{
		ID: 100, ItemID: itemID, TurnID: turnID, Status: int32(entity.TurnRunState_Fail), TargetResultID: oldTargetID,
	}}, int64(100), nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(100), int64(50), spaceID).Return(nil, int64(0), nil)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID, LogID: "old-item-log"}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).DoAndReturn(
		func(ctx context.Context, _, _ int64, _ []int64, _ int64) ([]*entity.ExptItemResultRunLog, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "current-run item log must read from primary on redrive")
			return []*entity.ExptItemResultRunLog{{
				ExptID: exptID, ExptRunID: newRunID, ItemID: itemID, LogID: "current-run-log",
			}}, nil
		},
	)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).DoAndReturn(
		func(ctx context.Context, _, _ int64, _ []int64, _ int64) ([]*entity.ExptTurnResultRunLog, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "current-run turn log must read from primary on redrive")
			return []*entity.ExptTurnResultRunLog{{
				ExptID: exptID, ExptRunID: newRunID, ItemID: itemID, TurnID: turnID, TargetResultID: currentTargetID,
				EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{1: 101}},
			}}, nil
		},
	)
	success := entity.EvalTargetRunStatusSuccess
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{currentTargetID}).DoAndReturn(
		func(ctx context.Context, _ int64, _ []int64) ([]*entity.EvalTargetRecord, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "current-run target record must read from primary on redrive")
			return []*entity.EvalTargetRecord{{ID: currentTargetID, Status: &success}}, nil
		},
	)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	itemRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, got []*entity.ExptItemResultRunLog) error {
		require.Len(t, got, 1)
		assert.Equal(t, "current-run-log", got[0].LogID)
		return nil
	})
	itemRepo.EXPECT().FillItemRunLogLogIDIfEmpty(gomock.Any(), exptID, newRunID, spaceID, map[int64]string{itemID: "current-run-log"}).Return(nil)
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, gomock.Any()).Return(nil)
	wantTargetID := currentTargetID
	turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, []*entity.ItemTurnID{{ItemID: itemID, TurnID: turnID}}, spaceID, failRetryTurnUpdateMatcher{
		wantRunID: newRunID, wantTargetID: &wantTargetID,
	}).Return(nil)
	statsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{FailItemCnt: 1}, nil)
	statsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	configer.EXPECT().GetExptExecConf(gomock.Any(), spaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	exec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, exptStatsRepo: statsRepo,
		idgenerator: idgen, exptRepo: exptRepo, idem: idem, configer: configer,
		evalTargetService: targetSvc,
	}
	ctx := context.Background()
	expt := buildMockExpt()
	err := exec.ExptStart(ctx, &entity.ExptScheduleEvent{ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID}, expt)
	require.NoError(t, err)

	evalSetItem := &entity.EvaluationSetItem{
		ItemID:   itemID,
		Turns:    []*entity.Turn{{ID: turnID, ItemID: itemID}},
		BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))},
	}
	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{
			ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, EvalSetItemID: itemID,
			ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"},
		},
		Expt: expt, EvalSetItem: evalSetItem,
		ExistItemEvalResult: &entity.ExptItemEvalResult{
			ItemResultRunLog: &entity.ExptItemResultRunLog{ExptRunID: newRunID, ItemID: itemID, LogID: "current-run-log"},
			TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				turnID: {ExptRunID: newRunID, ItemID: itemID, TurnID: turnID, TargetResultID: currentTargetID},
			},
		},
	}
	require.NoError(t, (&ExptRecordEvalModeFailRetry{}).PreEval(ctx, eiec))
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, nil)
	configer.EXPECT().BuildEvalExt(gomock.Any(), spaceID, gomock.Any()).Return(nil)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), spaceID, currentTargetID).Return(&entity.EvalTargetRecord{
		ID: currentTargetID, Status: &success,
		EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}},
	}, nil)
	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo: turnRepo, ItemResultRepo: itemRepo, Configer: configer,
		evalTargetService: targetSvc,
	}
	etec, err := executor.buildExptTurnEvalCtx(ctx, evalSetItem.Turns[0], eiec, nil)
	require.NoError(t, err)
	targetResult, err := (&DefaultExptTurnEvaluationImpl{evalTargetService: targetSvc}).CallTarget(ctx, etec)
	require.NoError(t, err)
	assert.Equal(t, currentTargetID, targetResult.ID)
}

func TestExptFailRetryExec_ExptStart_TargetBatchReadFailureIsObservableAndWriteFree(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
		itemID   = int64(10)
		targetID = int64(30)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)

	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(0), int64(50), spaceID).Return([]*entity.ExptTurnResult{{
		ID: 100, ItemID: itemID, TurnID: 20, Status: int32(entity.TurnRunState_Fail), TargetResultID: targetID,
	}}, int64(100), nil)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	dependencyErr := errors.New("target repo unavailable")
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{targetID}).Return(nil, dependencyErr)
	metric.EXPECT().EmitRetryStartDependencyFailure(spaceID, "eval_target_record")

	exec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, idem: idem,
		evalTargetService: targetSvc, metric: metric,
	}
	err := exec.ExptStart(context.Background(), &entity.ExptScheduleEvent{ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID}, buildMockExpt())
	require.Error(t, err)
	assert.True(t, isSchedulerInfraError(err))
	assert.ErrorIs(t, err, dependencyErr)
	assert.Contains(t, err.Error(), "eval_target_record")
}

func TestExptFailRetryExec_BuildPagePlan_MultiSetGroupsTargetReadsBySourceSpace(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID         = int64(1)
		runID          = int64(2)
		consumerSpace  = int64(3)
		topTargetSpace = int64(100)
		rowTargetSpace = int64(200)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	itemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	itemIDs := []int64{10, 11, 12}

	itemRepo.EXPECT().BatchGet(gomock.Any(), consumerSpace, exptID, itemIDs).Return([]*entity.ExptItemResult{
		{ItemID: 10, LogID: "log-10"},
		{ItemID: 11, LogID: "log-11"},
		{ItemID: 12, LogID: "log-12"},
	}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, itemIDs, consumerSpace).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, itemIDs, consumerSpace).Return(nil, nil)
	itemRefRepo.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), consumerSpace, exptID, itemIDs).Return([]*entity.ExptItemRef{
		{ItemID: 10, ItemConfig: &entity.ExptItemConfig{TargetSourceSpaceID: rowTargetSpace}},
		{ItemID: 11, ItemConfig: &entity.ExptItemConfig{}},
		{ItemID: 12, ItemConfig: &entity.ExptItemConfig{TargetSourceSpaceID: rowTargetSpace}},
	}, nil)
	success := entity.EvalTargetRunStatusSuccess
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), topTargetSpace, []int64{110}).Return([]*entity.EvalTargetRecord{{ID: 110, Status: &success}}, nil)
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), rowTargetSpace, []int64{100, 120}).Return([]*entity.EvalTargetRecord{
		{ID: 100, Status: &success},
		{ID: 120, Status: &success},
	}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 3).Return([]int64{900, 901, 902}, nil)

	expt := buildMockExpt()
	expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	expt.TargetSpaceID = topTargetSpace
	exec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo,
		exptTurnResultRepo: turnRepo,
		exptItemRefRepo:    itemRefRepo,
		evalTargetService:  targetSvc,
		idgenerator:        idgen,
	}
	plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: runID, SpaceID: consumerSpace,
	}, expt, []*entity.ExptTurnResult{
		{ItemID: 10, TurnID: 20, TargetResultID: 100},
		{ItemID: 10, TurnID: 21, TargetResultID: 100},
		{ItemID: 11, TurnID: 22, TargetResultID: 110},
		{ItemID: 12, TurnID: 23, TargetResultID: 120},
	})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Empty(t, plan.clearTargetTurns)
	assert.ElementsMatch(t, []*entity.ItemTurnID{
		{ItemID: 10, TurnID: 20},
		{ItemID: 10, TurnID: 21},
		{ItemID: 11, TurnID: 22},
		{ItemID: 12, TurnID: 23},
	}, plan.preserveTargetTurns)
	assert.Empty(t, plan.restoreTurnsByTargetID)
}

func TestExptFailRetryExec_BuildPagePlan_MultiSetMissingItemRefFallsBackToExperimentTargetSpace(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID  = int64(1)
		runID   = int64(2)
		spaceID = int64(3)
		itemID  = int64(10)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	itemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)

	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
	itemRefRepo.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemRef{{
		ItemID: itemID, ItemConfig: nil,
	}}, nil)
	success := entity.EvalTargetRunStatusSuccess
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), int64(99), []int64{30}).Return([]*entity.EvalTargetRecord{{ID: 30, Status: &success}}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)

	expt := buildMockExpt()
	expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	expt.TargetSpaceID = 99
	exec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo,
		exptTurnResultRepo: turnRepo,
		exptItemRefRepo:    itemRefRepo,
		evalTargetService:  targetSvc,
		idgenerator:        idgen,
	}
	plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: runID, SpaceID: spaceID,
	}, expt, []*entity.ExptTurnResult{{ItemID: itemID, TurnID: 20, TargetResultID: 30}})

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, []*entity.ItemTurnID{{ItemID: itemID, TurnID: 20}}, plan.preserveTargetTurns)
}

func TestRetryFailure_ExptStartThroughExecution_ReusesSuccessfulTargetAndOnlyFailedEvaluatorRuns(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID          = int64(1)
		runID           = int64(2)
		spaceID         = int64(3)
		itemID          = int64(10)
		turnID          = int64(20)
		turnResultID    = int64(21)
		targetRecordID  = int64(30)
		evaluatorAVerID = int64(41)
		evaluatorBVerID = int64(42)
		evaluatorARecID = int64(401)
		evaluatorBRecID = int64(402)
		newEvaluatorBID = int64(403)
	)

	ctx := context.Background()
	expt := buildRetryFailureSingleSetExpt(spaceID, evaluatorAVerID, evaluatorBVerID)
	evalSetItem := &entity.EvaluationSetItem{ItemID: itemID, Turns: []*entity.Turn{{ID: turnID, ItemID: itemID}}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}}
	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{
			ExptID: exptID, ExptRunID: runID, SpaceID: spaceID, EvalSetItemID: itemID,
			ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"},
		},
		Expt: expt, EvalSetItem: evalSetItem,
		ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)},
	}

	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	evaluatorRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	canonicalTurnResult := &entity.ExptTurnResult{
		ID: turnResultID, ExptID: exptID, ItemID: itemID, TurnID: turnID, TargetResultID: targetRecordID,
		Status: int32(entity.TurnRunState_Fail), LogID: "turn-log",
		EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{
			evaluatorAVerID: evaluatorARecID,
			evaluatorBVerID: evaluatorBRecID,
		}},
	}
	targetStatus := entity.EvalTargetRunStatusSuccess
	targetRecord := &entity.EvalTargetRecord{
		ID: targetRecordID, Status: &targetStatus,
		EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}},
	}
	evaluatorARecord := &entity.EvaluatorRecord{ID: evaluatorARecID, EvaluatorVersionID: evaluatorAVerID, Status: entity.EvaluatorRunStatusSuccess}

	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(0), int64(50), spaceID).Return([]*entity.ExptTurnResult{canonicalTurnResult}, turnResultID, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), turnResultID, int64(50), spaceID).Return(nil, int64(0), nil)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID, LogID: "item-log"}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{targetRecordID}).Return([]*entity.EvalTargetRecord{targetRecord}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{800}, nil)
	itemRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil)
	itemRepo.EXPECT().FillItemRunLogLogIDIfEmpty(gomock.Any(), exptID, runID, spaceID, map[int64]string{itemID: "item-log"}).Return(nil)
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, gomock.Any()).Return(nil)
	turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, []*entity.ItemTurnID{{ItemID: itemID, TurnID: turnID}}, spaceID, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, _ []*entity.ItemTurnID, _ int64, fields map[string]any) error {
			canonicalTurnResult.Status = fields["status"].(int32)
			canonicalTurnResult.ExptRunID = fields["expt_run_id"].(int64)
			if targetResultID, ok := fields["target_result_id"].(int64); ok {
				canonicalTurnResult.TargetResultID = targetResultID
			}
			return nil
		},
	)
	statsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{FailItemCnt: 1}, nil)
	statsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	configer.EXPECT().GetExptExecConf(gomock.Any(), spaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	startExec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, exptStatsRepo: statsRepo,
		idgenerator: idgen, exptRepo: exptRepo, idem: idem, configer: configer, evalTargetService: targetSvc,
	}
	require.NoError(t, startExec.ExptStart(ctx, &entity.ExptScheduleEvent{ExptID: exptID, ExptRunID: runID, SpaceID: spaceID}, expt))
	assert.Equal(t, runID, canonicalTurnResult.ExptRunID)
	assert.Equal(t, targetRecordID, canonicalTurnResult.TargetResultID)

	resultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), exptID, itemID, spaceID, gomock.Any()).DoAndReturn(
		func(ctx context.Context, _, _, _ int64, _ *entity.Session) ([]*entity.ExptTurnResult, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "RetryFailure PreEval canonical snapshot must read from primary")
			return []*entity.ExptTurnResult{canonicalTurnResult}, nil
		},
	)
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{turnResultID}).DoAndReturn(
		func(ctx context.Context, _ int64, _ []int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "RetryFailure PreEval evaluator refs must read from primary")
			return []*entity.ExptTurnEvaluatorResultRef{
				{ExptTurnResultID: turnResultID, EvaluatorVersionID: evaluatorAVerID, EvaluatorResultID: evaluatorARecID},
				{ExptTurnResultID: turnResultID, EvaluatorVersionID: evaluatorBVerID, EvaluatorResultID: evaluatorBRecID},
			}, nil
		},
	)
	var targetReadFromPrimary int
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), spaceID, targetRecordID).DoAndReturn(
		func(ctx context.Context, _, _ int64) (*entity.EvalTargetRecord, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "RetryFailure target records must read from primary")
			targetReadFromPrimary++
			return targetRecord, nil
		},
	).Times(2)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(
		func(ctx context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "RetryFailure PreEval evaluator records must read from primary")
			assert.ElementsMatch(t, []int64{evaluatorARecID, evaluatorBRecID}, ids)
			return []*entity.EvaluatorRecord{
				evaluatorARecord,
				{ID: evaluatorBRecID, EvaluatorVersionID: evaluatorBVerID, Status: entity.EvaluatorRunStatusFail},
			}, nil
		},
	)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
		require.Len(t, logs, 1)
		assert.Equal(t, targetRecordID, logs[0].TargetResultID)
		require.NotNil(t, logs[0].EvaluatorResultIds)
		assert.Equal(t, map[int64]int64{evaluatorAVerID: evaluatorARecID}, logs[0].EvaluatorResultIds.EvalVerIDToResID)
		return nil
	})
	preEval := &ExptRecordEvalModeFailRetry{
		resultSvc: resultSvc, exptTurnResultRepo: turnRepo, idgen: idgen,
		evalTargetService: targetSvc, evaluatorRecordSvc: evaluatorRecordSvc,
	}
	require.NoError(t, preEval.PreEval(ctx, eiec))

	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, nil)
	configer.EXPECT().BuildEvalExt(gomock.Any(), spaceID, gomock.Any()).Return(nil)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{evaluatorARecID}, false, false).DoAndReturn(
		func(ctx context.Context, _ []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
			assert.True(t, contexts.CtxWriteDB(ctx), "RetryFailure evaluator records must read from primary")
			return []*entity.EvaluatorRecord{evaluatorARecord}, nil
		},
	)
	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo: turnRepo, ItemResultRepo: itemRepo, Configer: configer,
		evalTargetService: targetSvc, evaluatorRecordService: evaluatorRecordSvc,
	}
	etec, err := executor.buildExptTurnEvalCtx(ctx, evalSetItem.Turns[0], eiec, nil)
	require.NoError(t, err)

	evaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
	benefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	benefitSvc.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
	evaluatorSvc.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, bool, error) {
		assert.Equal(t, evaluatorBVerID, req.EvaluatorVersionID)
		return nil, false, nil
	})
	evaluatorSvc.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		assert.Equal(t, evaluatorBVerID, req.EvaluatorVersionID)
		return &entity.EvaluatorRecord{ID: newEvaluatorBID, EvaluatorVersionID: evaluatorBVerID, Status: entity.EvaluatorRunStatusSuccess}, nil
	})
	metric.EXPECT().EmitTurnExecEvaluatorResult(spaceID, false)
	turnEval := &DefaultExptTurnEvaluationImpl{
		metric: metric, evalTargetService: targetSvc, evaluatorService: evaluatorSvc, benefitService: benefitSvc,
		evaluatorRecordService: evaluatorRecordSvc,
	}
	targetResult, err := turnEval.CallTarget(ctx, etec)
	require.NoError(t, err)
	assert.Equal(t, targetRecordID, targetResult.ID)
	assert.Equal(t, 2, targetReadFromPrimary, "RetryFailure PreEval and execution target reads must use primary")
	evaluatorResults, err := turnEval.CallEvaluators(ctx, etec, targetResult)
	require.NoError(t, err)
	require.Len(t, evaluatorResults, 2)
	byID := make(map[int64]*entity.EvaluatorRecord, len(evaluatorResults))
	for _, record := range evaluatorResults {
		byID[record.ID] = record
	}
	assert.Same(t, evaluatorARecord, byID[evaluatorARecID])
	assert.Equal(t, evaluatorBVerID, byID[newEvaluatorBID].EvaluatorVersionID)
}

// Physical multi-turn items are not currently reachable through the EvaluationSet
// experiment adapter. This test protects the service-chain behavior for when that
// adapter eventually exposes multiple turns; it is not PPE end-to-end coverage.
func TestRetryFailure_MultiTurnItem_ConvergesAllTurnsAndOnlyRerunsFailedStage(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID         = int64(1)
		oldRunID       = int64(2)
		newRunID       = int64(3)
		spaceID        = int64(4)
		itemID         = int64(10)
		turn1ID        = int64(20)
		turn2ID        = int64(21)
		turn1ResultID  = int64(30)
		turn2ResultID  = int64(31)
		turn1TargetID  = int64(40)
		turn2TargetID  = int64(41)
		evaluatorVerID = int64(50)
		turn1EvalRecID = int64(60)
		turn2OldEvalID = int64(61)
		turn2NewEvalID = int64(62)
		itemRunLogID   = int64(70)
		turn1RunLogID  = int64(71)
		turn2RunLogID  = int64(72)
		turn1RefID     = int64(80)
		turn2RefID     = int64(81)
	)

	ctx := context.Background()
	expt := buildRetryFailureSingleSetExpt(spaceID, evaluatorVerID)
	turn1 := &entity.Turn{ID: turn1ID, ItemID: itemID}
	turn2 := &entity.Turn{ID: turn2ID, ItemID: itemID}
	evalSetItem := &entity.EvaluationSetItem{
		ItemID: itemID,
		Turns:  []*entity.Turn{turn1, turn2},
		BaseInfo: &entity.BaseInfo{
			CreatedAt: gptr.Of(int64(1)),
		},
	}
	canonicalTurns := []*entity.ExptTurnResult{
		{
			ID: turn1ResultID, ExptID: exptID, ExptRunID: oldRunID, ItemID: itemID, TurnID: turn1ID,
			Status: int32(entity.TurnRunState_Success), TargetResultID: turn1TargetID, LogID: "turn-1-log",
			EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{evaluatorVerID: turn1EvalRecID}},
		},
		{
			ID: turn2ResultID, ExptID: exptID, ExptRunID: oldRunID, ItemID: itemID, TurnID: turn2ID,
			Status: int32(entity.TurnRunState_Fail), TargetResultID: turn2TargetID, LogID: "turn-2-log",
			ErrMsg:           errno.SerializeErr(errno.NewTurnOtherErr("evaluator temporarily unavailable", nil)),
			EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{evaluatorVerID: turn2OldEvalID}},
		},
	}
	turnByID := map[int64]*entity.ExptTurnResult{turn1ID: canonicalTurns[0], turn2ID: canonicalTurns[1]}
	targetStatus := entity.EvalTargetRunStatusSuccess
	targetRecords := map[int64]*entity.EvalTargetRecord{
		turn1TargetID: {
			ID: turn1TargetID, Status: &targetStatus,
			EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}},
		},
		turn2TargetID: {
			ID: turn2TargetID, Status: &targetStatus,
			EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}},
		},
	}
	targetIDByTurnID := map[int64]int64{
		turn1ID: turn1TargetID,
		turn2ID: turn2TargetID,
	}
	turn1EvalRecord := &entity.EvaluatorRecord{
		ID: turn1EvalRecID, EvaluatorVersionID: evaluatorVerID, Status: entity.EvaluatorRunStatusSuccess,
	}
	turn2OldEvalRecord := &entity.EvaluatorRecord{
		ID: turn2OldEvalID, EvaluatorVersionID: evaluatorVerID, Status: entity.EvaluatorRunStatusFail,
	}
	turn2NewEvalRecord := &entity.EvaluatorRecord{
		ID: turn2NewEvalID, EvaluatorVersionID: evaluatorVerID, Status: entity.EvaluatorRunStatusSuccess,
	}

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	evaluatorRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	resultSvc := svcmocks.NewMockExptResultService(ctrl)

	var itemRunLog *entity.ExptItemResultRunLog
	turnRunLogs := make(map[int64]*entity.ExptTurnResultRunLog, 2)

	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(0), int64(50), spaceID).
		Return([]*entity.ExptTurnResult{canonicalTurns[1]}, turn2ResultID, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), turn2ResultID, int64(50), spaceID).
		Return(nil, int64(0), nil)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).
		Return([]*entity.ExptItemResult{{ItemID: itemID, Status: entity.ItemRunState_Fail, LogID: "item-log"}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{turn2TargetID}).
		Return([]*entity.EvalTargetRecord{targetRecords[turn2TargetID]}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{itemRunLogID}, nil)
	itemRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptItemResultRunLog) error {
		require.Len(t, logs, 1)
		itemRunLog = logs[0]
		assert.Equal(t, "item-log", itemRunLog.LogID)
		return nil
	})
	itemRepo.EXPECT().FillItemRunLogLogIDIfEmpty(gomock.Any(), exptID, newRunID, spaceID, map[int64]string{itemID: "item-log"}).Return(nil)
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, gomock.Any()).Return(nil)
	turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, []*entity.ItemTurnID{{ItemID: itemID, TurnID: turn2ID}}, spaceID, failRetryTurnUpdateMatcher{
		wantRunID: newRunID,
	}).DoAndReturn(
		func(_ context.Context, _ int64, _ []*entity.ItemTurnID, _ int64, fields map[string]any) error {
			canonicalTurns[1].Status = fields["status"].(int32)
			canonicalTurns[1].ExptRunID = fields["expt_run_id"].(int64)
			return nil
		},
	)
	statsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{FailItemCnt: 1}, nil)
	statsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	configer.EXPECT().GetExptExecConf(gomock.Any(), spaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	startExec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, exptStatsRepo: statsRepo,
		idgenerator: idgen, exptRepo: exptRepo, idem: idem, configer: configer, evalTargetService: targetSvc,
	}
	require.NoError(t, startExec.ExptStart(ctx, &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID,
	}, expt))
	assert.Equal(t, oldRunID, canonicalTurns[0].ExptRunID, "successful sibling converges only when the item is recorded")
	assert.Equal(t, newRunID, canonicalTurns[1].ExptRunID)
	assert.Equal(t, turn2TargetID, canonicalTurns[1].TargetResultID)
	require.NotNil(t, itemRunLog)

	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{
			ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID, EvalSetItemID: itemID,
			ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"},
		},
		Expt: expt, EvalSetItem: evalSetItem,
		ExistItemEvalResult: &entity.ExptItemEvalResult{
			ItemResultRunLog:  itemRunLog,
			TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog),
		},
	}
	resultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), exptID, itemID, spaceID, gomock.Any()).
		Return(canonicalTurns, nil)
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{turn1ResultID, turn2ResultID}).
		Return([]*entity.ExptTurnEvaluatorResultRef{
			{ExptTurnResultID: turn1ResultID, EvaluatorVersionID: evaluatorVerID, EvaluatorResultID: turn1EvalRecID},
			{ExptTurnResultID: turn2ResultID, EvaluatorVersionID: evaluatorVerID, EvaluatorResultID: turn2OldEvalID},
		}, nil)
	for _, targetID := range []int64{turn1TargetID, turn2TargetID} {
		targetSvc.EXPECT().GetRecordByID(gomock.Any(), spaceID, targetID).Return(targetRecords[targetID], nil).Times(2)
	}
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{turn1EvalRecID}, false, false).
		Return([]*entity.EvaluatorRecord{turn1EvalRecord}, nil)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{turn2OldEvalID}, false, false).
		Return([]*entity.EvaluatorRecord{turn2OldEvalRecord}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{turn1RunLogID, turn2RunLogID}, nil)
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
		require.Len(t, logs, 2)
		for _, log := range logs {
			turnRunLogs[log.TurnID] = log
		}
		assert.Equal(t, turn1TargetID, turnRunLogs[turn1ID].TargetResultID)
		assert.Equal(t, map[int64]int64{evaluatorVerID: turn1EvalRecID}, turnRunLogs[turn1ID].EvaluatorResultIds.EvalVerIDToResID)
		assert.Equal(t, turn2TargetID, turnRunLogs[turn2ID].TargetResultID)
		assert.Nil(t, turnRunLogs[turn2ID].EvaluatorResultIds)
		return nil
	})
	preEval := &ExptRecordEvalModeFailRetry{
		resultSvc: resultSvc, exptTurnResultRepo: turnRepo, idgen: idgen,
		evalTargetService: targetSvc, evaluatorRecordSvc: evaluatorRecordSvc,
	}
	require.NoError(t, preEval.PreEval(ctx, eiec))

	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, nil).Times(2)
	configer.EXPECT().BuildEvalExt(gomock.Any(), spaceID, gomock.Any()).Return(nil).Times(2)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{turn1EvalRecID}, false, false).
		Return([]*entity.EvaluatorRecord{turn1EvalRecord}, nil).Times(1)

	evaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
	benefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	// Across two turns, exactly one benefit charge and one evaluator call proves
	// the successful sibling is reused instead of being executed again.
	benefitSvc.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(1)
	evaluatorSvc.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, bool, error) {
		assert.Equal(t, evaluatorVerID, req.EvaluatorVersionID)
		return nil, false, nil
	}).Times(1)
	evaluatorSvc.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		assert.Equal(t, evaluatorVerID, req.EvaluatorVersionID)
		return turn2NewEvalRecord, nil
	}).Times(1)
	metric.EXPECT().EmitTurnExecEvaluatorResult(spaceID, false).Times(1)
	turnEval := &DefaultExptTurnEvaluationImpl{
		metric: metric, evalTargetService: targetSvc, evaluatorService: evaluatorSvc, benefitService: benefitSvc,
		evaluatorRecordService: evaluatorRecordSvc,
	}
	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo: turnRepo, ItemResultRepo: itemRepo, Configer: configer,
		evalTargetService: targetSvc, evaluatorRecordService: evaluatorRecordSvc,
	}
	for _, turn := range evalSetItem.Turns {
		etec, err := executor.buildExptTurnEvalCtx(ctx, turn, eiec, nil)
		require.NoError(t, err)
		targetResult, err := turnEval.CallTarget(ctx, etec)
		require.NoError(t, err)
		assert.Equal(t, targetIDByTurnID[turn.ID], targetResult.ID)
		evaluatorResults, err := turnEval.CallEvaluators(ctx, etec, targetResult)
		require.NoError(t, err)
		result := &entity.ExptTurnRunResult{TargetResult: targetResult, EvaluatorResults: evaluatorResults}
		turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
			require.Len(t, logs, 1)
			turnRunLogs[logs[0].TurnID] = logs[0]
			return nil
		})
		require.NoError(t, executor.storeTurnRunResult(ctx, etec, result))
	}
	assert.Equal(t, turn1EvalRecID, turnRunLogs[turn1ID].EvaluatorResultIds.Registered[0].RecordID)
	assert.Equal(t, turn2NewEvalID, turnRunLogs[turn2ID].EvaluatorResultIds.Registered[0].RecordID)

	itemRunLog.Status = int32(entity.ItemRunState_Success)
	itemRunLog.ResultState = int32(entity.ExptItemResultStateLogged)
	itemRepo.EXPECT().GetItemRunLog(gomock.Any(), exptID, newRunID, itemID, spaceID).Return(itemRunLog, nil)
	turnRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), exptID, newRunID, itemID, spaceID).DoAndReturn(
		func(_ context.Context, _, _, _, _ int64) ([]*entity.ExptTurnResultRunLog, error) {
			return []*entity.ExptTurnResultRunLog{turnRunLogs[turn1ID], turnRunLogs[turn2ID]}, nil
		},
	)
	itemRepo.EXPECT().GetItemTurnResults(gomock.Any(), spaceID, exptID, itemID).Return(canonicalTurns, nil)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).
		Return([]*entity.ExptItemResult{{ItemID: itemID, Status: entity.ItemRunState_Processing}}, nil)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{turn1EvalRecID}, false, false).
		Return([]*entity.EvaluatorRecord{turn1EvalRecord}, nil).Times(1)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{turn2NewEvalID}, false, false).
		Return([]*entity.EvaluatorRecord{turn2NewEvalRecord}, nil).Times(1)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{turn1RefID, turn2RefID}, nil)
	turnRepo.EXPECT().CreateTurnEvaluatorRefs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, refs []*entity.ExptTurnEvaluatorResultRef) error {
		require.Len(t, refs, 2)
		byTurnResultID := make(map[int64]int64, len(refs))
		for _, ref := range refs {
			byTurnResultID[ref.ExptTurnResultID] = ref.EvaluatorResultID
		}
		assert.Equal(t, turn1EvalRecID, byTurnResultID[turn1ResultID])
		assert.Equal(t, turn2NewEvalID, byTurnResultID[turn2ResultID])
		return nil
	})
	turnRepo.EXPECT().SaveTurnResults(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, turns []*entity.ExptTurnResult) error {
		require.Len(t, turns, 2)
		for _, turn := range turns {
			assert.Equal(t, newRunID, turn.ExptRunID)
			assert.Equal(t, int32(entity.TurnRunState_Success), turn.Status)
			assert.Empty(t, turn.ErrMsg)
			assert.Equal(t, turnRunLogs[turn.TurnID].TargetResultID, turn.TargetResultID)
			assert.Equal(t, turnRunLogs[turn.TurnID].LogID, turn.LogID)
			assert.Same(t, turnByID[turn.TurnID], turn)
		}
		return nil
	})
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, _ int64, _ []int64, fields map[string]any) error {
			assert.Equal(t, "item-log", fields["log_id"])
			return nil
		},
	)
	itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), exptID, newRunID, []int64{itemID}, gomock.Any(), spaceID).Return(nil)
	statsRepo.EXPECT().ArithOperateCount(gomock.Any(), exptID, spaceID, gomock.Any()).Return(nil)

	resultRecorder := ExptResultServiceImpl{
		ExptItemResultRepo: itemRepo, ExptTurnResultRepo: turnRepo, ExptStatsRepo: statsRepo,
		evaluatorRecordService: evaluatorRecordSvc, idgen: idgen,
		scoreCalculator: NewEvaluatorScoreCalculator(nil, nil),
	}
	refs, err := resultRecorder.RecordItemRunLogs(ctx, exptID, newRunID, itemID, spaceID, expt)
	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, newRunID, canonicalTurns[0].ExptRunID)
	assert.Equal(t, newRunID, canonicalTurns[1].ExptRunID)
	assert.Equal(t, turn1TargetID, canonicalTurns[0].TargetResultID)
	assert.Equal(t, turn2TargetID, canonicalTurns[1].TargetResultID)
	assert.Equal(t, "turn-1-log", canonicalTurns[0].LogID)
	assert.Equal(t, "turn-2-log", canonicalTurns[1].LogID)
}

func TestRetryFailure_PreEvalThroughExecution_FailedTargetRerunsTargetAndAllEvaluators(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID          = int64(1)
		runID           = int64(2)
		spaceID         = int64(3)
		itemID          = int64(10)
		turnID          = int64(20)
		turnResultID    = int64(21)
		oldTargetID     = int64(30)
		newTargetID     = int64(31)
		evaluatorAVerID = int64(41)
		evaluatorBVerID = int64(42)
	)

	ctx := context.Background()
	expt := buildRetryFailureSingleSetExpt(spaceID, evaluatorAVerID, evaluatorBVerID)
	evalSetItem := &entity.EvaluationSetItem{ItemID: itemID, Turns: []*entity.Turn{{ID: turnID, ItemID: itemID}}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}}
	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{
			ExptID: exptID, ExptRunID: runID, SpaceID: spaceID, EvalSetItemID: itemID,
			ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"},
		},
		Expt: expt, EvalSetItem: evalSetItem,
		ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)},
	}

	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	evaluatorRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	oldTurnResult := &entity.ExptTurnResult{
		ID: turnResultID, ExptID: exptID, ItemID: itemID, TurnID: turnID, TargetResultID: oldTargetID,
		EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{evaluatorAVerID: 401}},
	}
	failedStatus := entity.EvalTargetRunStatusFail
	resultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), exptID, itemID, spaceID, gomock.Any()).Return([]*entity.ExptTurnResult{oldTurnResult}, nil)
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{turnResultID}).Return([]*entity.ExptTurnEvaluatorResultRef{{
		ExptTurnResultID: turnResultID, EvaluatorVersionID: evaluatorAVerID, EvaluatorResultID: 401,
	}}, nil)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), spaceID, oldTargetID).Return(&entity.EvalTargetRecord{ID: oldTargetID, Status: &failedStatus}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
		require.Len(t, logs, 1)
		assert.Zero(t, logs[0].TargetResultID)
		assert.Nil(t, logs[0].EvaluatorResultIds)
		return nil
	})
	preEval := &ExptRecordEvalModeFailRetry{
		resultSvc: resultSvc, exptTurnResultRepo: turnRepo, idgen: idgen,
		evalTargetService: targetSvc, evaluatorRecordSvc: evaluatorRecordSvc,
	}
	require.NoError(t, preEval.PreEval(ctx, eiec))

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, nil)
	configer.EXPECT().BuildEvalExt(gomock.Any(), spaceID, gomock.Any()).Return(nil)
	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo: turnRepo, ItemResultRepo: itemRepo, Configer: configer,
		evalTargetService: targetSvc, evaluatorRecordService: evaluatorRecordSvc,
	}
	etec, err := executor.buildExptTurnEvalCtx(ctx, evalSetItem.Turns[0], eiec, nil)
	require.NoError(t, err)

	evaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
	benefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	benefitSvc.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).Times(2)
	metric.EXPECT().EmitTurnExecTargetResult(spaceID, false)
	newTargetStatus := entity.EvalTargetRunStatusSuccess
	targetSvc.EXPECT().ExecuteTarget(gomock.Any(), spaceID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvalTargetRecord{
		ID: newTargetID, Status: &newTargetStatus,
		EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}},
	}, nil)
	evaluatorSvc.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(2)
	evaluatorSvc.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		return &entity.EvaluatorRecord{ID: 1000 + req.EvaluatorVersionID, EvaluatorVersionID: req.EvaluatorVersionID, Status: entity.EvaluatorRunStatusSuccess}, nil
	}).Times(2)
	metric.EXPECT().EmitTurnExecEvaluatorResult(spaceID, false).Times(2)
	turnEval := &DefaultExptTurnEvaluationImpl{
		metric: metric, evalTargetService: targetSvc, evaluatorService: evaluatorSvc, benefitService: benefitSvc,
		evaluatorRecordService: evaluatorRecordSvc,
	}
	targetResult, err := turnEval.CallTarget(ctx, etec)
	require.NoError(t, err)
	assert.Equal(t, newTargetID, targetResult.ID)
	evaluatorResults, err := turnEval.CallEvaluators(ctx, etec, targetResult)
	require.NoError(t, err)
	require.Len(t, evaluatorResults, 2)
	assert.NotNil(t, (&entity.ExptTurnRunResult{EvaluatorResults: evaluatorResults}).GetEvaluatorRecord(evaluatorAVerID))
	assert.NotNil(t, (&entity.ExptTurnRunResult{EvaluatorResults: evaluatorResults}).GetEvaluatorRecord(evaluatorBVerID))
}

func TestRetryFailure_PreEvalThroughExecution_RecordOnlyReusesSuccessfulEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID          = int64(1)
		runID           = int64(2)
		spaceID         = int64(3)
		itemID          = int64(10)
		turnID          = int64(20)
		turnResultID    = int64(21)
		evaluatorAVerID = int64(41)
		evaluatorBVerID = int64(42)
		evaluatorARecID = int64(401)
		evaluatorBRecID = int64(402)
		newEvaluatorBID = int64(403)
	)

	ctx := context.Background()
	expt := buildRetryFailureSingleSetExpt(spaceID, evaluatorAVerID, evaluatorBVerID)
	expt.TargetType = 0
	expt.TargetVersionID = 100
	expt.Target.EvalTargetType = entity.EvalTargetTypeCustomRPCServerOnline
	evalSetItem := &entity.EvaluationSetItem{
		ItemID:   itemID,
		Turns:    []*entity.Turn{{ID: turnID, ItemID: itemID}},
		BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))},
	}
	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{
			ExptID: exptID, ExptRunID: runID, SpaceID: spaceID, EvalSetItemID: itemID,
			ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"},
		},
		Expt: expt, EvalSetItem: evalSetItem,
		ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)},
	}

	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	evaluatorRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	oldTurnResult := &entity.ExptTurnResult{
		ID: turnResultID, ExptID: exptID, ItemID: itemID, TurnID: turnID,
		EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{
			evaluatorAVerID: evaluatorARecID,
			evaluatorBVerID: evaluatorBRecID,
		}},
	}
	evaluatorARecord := &entity.EvaluatorRecord{ID: evaluatorARecID, EvaluatorVersionID: evaluatorAVerID, Status: entity.EvaluatorRunStatusSuccess}

	resultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), exptID, itemID, spaceID, gomock.Any()).Return([]*entity.ExptTurnResult{oldTurnResult}, nil)
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{turnResultID}).Return([]*entity.ExptTurnEvaluatorResultRef{
		{ExptTurnResultID: turnResultID, EvaluatorVersionID: evaluatorAVerID, EvaluatorResultID: evaluatorARecID},
		{ExptTurnResultID: turnResultID, EvaluatorVersionID: evaluatorBVerID, EvaluatorResultID: evaluatorBRecID},
	}, nil)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(
		func(_ context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
			assert.ElementsMatch(t, []int64{evaluatorARecID, evaluatorBRecID}, ids)
			return []*entity.EvaluatorRecord{
				evaluatorARecord,
				{ID: evaluatorBRecID, EvaluatorVersionID: evaluatorBVerID, Status: entity.EvaluatorRunStatusFail},
			}, nil
		},
	)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, runLogs []*entity.ExptTurnResultRunLog) error {
		require.Len(t, runLogs, 1)
		assert.Zero(t, runLogs[0].TargetResultID)
		require.NotNil(t, runLogs[0].EvaluatorResultIds)
		assert.Equal(t, map[int64]int64{evaluatorAVerID: evaluatorARecID}, runLogs[0].EvaluatorResultIds.EvalVerIDToResID)
		return nil
	})
	preEval := &ExptRecordEvalModeFailRetry{
		resultSvc: resultSvc, exptTurnResultRepo: turnRepo, idgen: idgen,
		evalTargetService: targetSvc, evaluatorRecordSvc: evaluatorRecordSvc,
	}
	require.NoError(t, preEval.PreEval(ctx, eiec))

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, nil)
	configer.EXPECT().BuildEvalExt(gomock.Any(), spaceID, gomock.Any()).Return(nil)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{evaluatorARecID}, false, false).Return([]*entity.EvaluatorRecord{evaluatorARecord}, nil)
	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo: turnRepo, ItemResultRepo: itemRepo, Configer: configer,
		evalTargetService: targetSvc, evaluatorRecordService: evaluatorRecordSvc,
	}
	etec, err := executor.buildExptTurnEvalCtx(ctx, evalSetItem.Turns[0], eiec, nil)
	require.NoError(t, err)

	evaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
	benefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	benefitSvc.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
	evaluatorSvc.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, bool, error) {
		assert.Equal(t, evaluatorBVerID, req.EvaluatorVersionID)
		return nil, false, nil
	})
	evaluatorSvc.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		assert.Equal(t, evaluatorBVerID, req.EvaluatorVersionID)
		return &entity.EvaluatorRecord{ID: newEvaluatorBID, EvaluatorVersionID: evaluatorBVerID, Status: entity.EvaluatorRunStatusSuccess}, nil
	})
	metric.EXPECT().EmitTurnExecEvaluatorResult(spaceID, false)
	turnEval := &DefaultExptTurnEvaluationImpl{
		metric: metric, evalTargetService: targetSvc, evaluatorService: evaluatorSvc, benefitService: benefitSvc,
		evaluatorRecordService: evaluatorRecordSvc,
	}
	targetResult, err := turnEval.CallTarget(ctx, etec)
	require.NoError(t, err)
	assert.Zero(t, targetResult.ID)
	evaluatorResults, err := turnEval.CallEvaluators(ctx, etec, targetResult)
	require.NoError(t, err)
	require.Len(t, evaluatorResults, 2)
	assert.Same(t, evaluatorARecord, (&entity.ExptTurnRunResult{EvaluatorResults: evaluatorResults}).GetEvaluatorRecord(evaluatorAVerID))
	assert.Equal(t, newEvaluatorBID, (&entity.ExptTurnRunResult{EvaluatorResults: evaluatorResults}).GetEvaluatorRecord(evaluatorBVerID).ID)
}

func TestRetryFailure_PreEvalThroughExecution_FailedTargetAgainSkipsEvaluators(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID       = int64(1)
		runID        = int64(2)
		spaceID      = int64(3)
		itemID       = int64(10)
		turnID       = int64(20)
		turnResultID = int64(21)
		oldTargetID  = int64(30)
		newTargetID  = int64(31)
	)

	ctx := context.Background()
	expt := buildRetryFailureSingleSetExpt(spaceID, 41)
	evalSetItem := &entity.EvaluationSetItem{
		ItemID:   itemID,
		Turns:    []*entity.Turn{{ID: turnID, ItemID: itemID}},
		BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))},
	}
	eiec := &entity.ExptItemEvalCtx{
		Event: &entity.ExptItemEvalEvent{
			ExptID: exptID, ExptRunID: runID, SpaceID: spaceID, EvalSetItemID: itemID,
			ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"},
		},
		Expt: expt, EvalSetItem: evalSetItem,
		ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)},
	}

	resultSvc := svcmocks.NewMockExptResultService(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	evaluatorRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	failedStatus := entity.EvalTargetRunStatusFail
	resultSvc.EXPECT().GetExptItemTurnResults(gomock.Any(), exptID, itemID, spaceID, gomock.Any()).Return([]*entity.ExptTurnResult{{
		ID: turnResultID, ExptID: exptID, ItemID: itemID, TurnID: turnID, TargetResultID: oldTargetID,
		EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{41: 401}},
	}}, nil)
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{turnResultID}).Return([]*entity.ExptTurnEvaluatorResultRef{{
		ExptTurnResultID: turnResultID, EvaluatorVersionID: 41, EvaluatorResultID: 401,
	}}, nil)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), spaceID, oldTargetID).Return(&entity.EvalTargetRecord{ID: oldTargetID, Status: &failedStatus}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, runLogs []*entity.ExptTurnResultRunLog) error {
		require.Len(t, runLogs, 1)
		assert.Zero(t, runLogs[0].TargetResultID)
		assert.Nil(t, runLogs[0].EvaluatorResultIds)
		return nil
	})
	preEval := &ExptRecordEvalModeFailRetry{
		resultSvc: resultSvc, exptTurnResultRepo: turnRepo, idgen: idgen,
		evalTargetService: targetSvc, evaluatorRecordSvc: evaluatorRecordSvc,
	}
	require.NoError(t, preEval.PreEval(ctx, eiec))

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, nil)
	configer.EXPECT().BuildEvalExt(gomock.Any(), spaceID, gomock.Any()).Return(nil)
	executor := &ExptItemEvalCtxExecutor{
		TurnResultRepo: turnRepo, ItemResultRepo: itemRepo, Configer: configer,
		evalTargetService: targetSvc, evaluatorRecordService: evaluatorRecordSvc,
	}
	etec, err := executor.buildExptTurnEvalCtx(ctx, evalSetItem.Turns[0], eiec, nil)
	require.NoError(t, err)

	evaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
	benefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	benefitSvc.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
	metric.EXPECT().EmitTurnExecEval(spaceID, int64(entity.EvaluationModeFailRetry))
	metric.EXPECT().EmitTurnExecTargetResult(spaceID, false)
	metric.EXPECT().EmitTurnExecResult(spaceID, int64(entity.EvaluationModeFailRetry), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	newFailedStatus := entity.EvalTargetRunStatusFail
	targetSvc.EXPECT().ExecuteTarget(gomock.Any(), spaceID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvalTargetRecord{
		ID: newTargetID, Status: &newFailedStatus,
		EvalTargetOutputData: &entity.EvalTargetOutputData{EvalTargetRunError: &entity.EvalTargetRunError{Code: 1, Message: "target failed again"}},
	}, nil)
	turnEval := &DefaultExptTurnEvaluationImpl{
		metric: metric, evalTargetService: targetSvc, evaluatorService: evaluatorSvc, benefitService: benefitSvc,
		evaluatorRecordService: evaluatorRecordSvc,
	}
	result := turnEval.Eval(ctx, etec)
	require.NotNil(t, result)
	require.NotNil(t, result.TargetResult)
	assert.Equal(t, newTargetID, result.TargetResult.ID)
	assert.Empty(t, result.EvaluatorResults)
}

func TestExptRecordEvalModeFailRetry_PreEval_ExistingRunLogsShortCircuit(t *testing.T) {
	eiec := &entity.ExptItemEvalCtx{
		ExistItemEvalResult: &entity.ExptItemEvalResult{
			ItemResultRunLog: &entity.ExptItemResultRunLog{LogID: "item-log"},
			TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{
				20: {TurnID: 20, TargetResultID: 30, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{41: 401}}},
			},
		},
	}

	mode := &ExptRecordEvalModeFailRetry{}
	require.NoError(t, mode.PreEval(context.Background(), eiec))
	assert.Equal(t, int64(30), eiec.ExistItemEvalResult.TurnResultRunLogs[20].TargetResultID)
	assert.Equal(t, map[int64]int64{41: 401}, eiec.ExistItemEvalResult.TurnResultRunLogs[20].EvaluatorResultIds.EvalVerIDToResID)
}

func TestExptResultBuilder_FillProcessingTargetResultID_RetryFailureProjection(t *testing.T) {
	const (
		spaceID       = int64(100)
		exptID        = int64(200)
		newRunID      = int64(300)
		itemID        = int64(400)
		turnID        = int64(500)
		successTarget = int64(600)
	)

	t.Run("successful canonical target remains visible without run-log fallback", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		builder := &ExptResultBuilder{
			ExptID: exptID, SpaceID: spaceID, ExptTurnResultRepo: turnRepo,
			turnResultDO: []*entity.ExptTurnResult{{
				ExptRunID: newRunID, ItemID: itemID, TurnID: turnID, TargetResultID: successTarget,
			}},
		}

		require.NoError(t, builder.fillProcessingTargetResultID(context.Background()))
		assert.Equal(t, successTarget, builder.turnResultDO[0].TargetResultID)
	})

	t.Run("cleared canonical target only reads the new run and does not restore an old failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return([]*entity.ExptTurnResultRunLog{{
			ExptRunID: newRunID, ItemID: itemID, TurnID: turnID, TargetResultID: 0,
		}}, nil)
		builder := &ExptResultBuilder{
			ExptID: exptID, SpaceID: spaceID, ExptTurnResultRepo: turnRepo,
			turnResultDO: []*entity.ExptTurnResult{{
				ExptRunID: newRunID, ItemID: itemID, TurnID: turnID, TargetResultID: 0,
			}},
		}

		require.NoError(t, builder.fillProcessingTargetResultID(context.Background()))
		assert.Zero(t, builder.turnResultDO[0].TargetResultID)
	})
}

func TestExptFailRetryExec_BuildPagePlan_LogIDFallbacks(t *testing.T) {
	const (
		exptID  = int64(1)
		runID   = int64(2)
		spaceID = int64(3)
		itemID  = int64(10)
	)

	tests := []struct {
		name       string
		turns      []*entity.ExptTurnResult
		wantLogID  string
		wantNonNil bool
	}{
		{
			name: "uses first non-empty turn log by canonical turn result ID",
			turns: []*entity.ExptTurnResult{
				{ID: 200, ItemID: itemID, TurnID: 22, LogID: "later-log"},
				{ID: 100, ItemID: itemID, TurnID: 21, LogID: "first-log"},
			},
			wantLogID: "first-log",
		},
		{
			name: "generates one non-empty log when all historical log IDs are empty",
			turns: []*entity.ExptTurnResult{
				{ID: 100, ItemID: itemID, TurnID: 21},
			},
			wantNonNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
			turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			idgen := idgenmocks.NewMockIIDGenerator(ctrl)
			itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID}}, nil)
			itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
			turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
			idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
			expt := buildMockExpt()
			expt.TargetVersionID = 0
			exec := &ExptFailRetryExec{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, idgenerator: idgen}

			plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
				ExptID: exptID, ExptRunID: runID, SpaceID: spaceID,
			}, expt, tt.turns)
			require.NoError(t, err)
			require.Len(t, plan.itemRunLogs, 1)
			if tt.wantNonNil {
				assert.NotEmpty(t, plan.itemIDToLogID[itemID])
			} else {
				assert.Equal(t, tt.wantLogID, plan.itemIDToLogID[itemID])
			}
			assert.Equal(t, plan.itemIDToLogID[itemID], plan.itemRunLogs[0].LogID)
		})
	}
}

func TestExptFailRetryExec_BuildPagePlan_SingleSetBatchesAndDeduplicatesTargetReads(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID  = int64(1)
		runID   = int64(2)
		spaceID = int64(3)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	itemIDs := []int64{10, 11}

	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, itemIDs).Return([]*entity.ExptItemResult{
		{ItemID: 10, LogID: "log-10"},
		{ItemID: 11, LogID: "log-11"},
	}, nil).Times(1)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, itemIDs, spaceID).Return(nil, nil).Times(1)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, itemIDs, spaceID).Return(nil, nil).Times(1)
	success := entity.EvalTargetRunStatusSuccess
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{30, 31}).Return([]*entity.EvalTargetRecord{
		{ID: 30, Status: &success},
		{ID: 31, Status: &success},
	}, nil).Times(1)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{900, 901}, nil)

	exec := &ExptFailRetryExec{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, evalTargetService: targetSvc, idgenerator: idgen}
	plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: runID, SpaceID: spaceID,
	}, buildMockExpt(), []*entity.ExptTurnResult{
		{ItemID: 10, TurnID: 20, TargetResultID: 30},
		{ItemID: 10, TurnID: 21, TargetResultID: 30},
		{ItemID: 11, TurnID: 22, TargetResultID: 31},
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []*entity.ItemTurnID{
		{ItemID: 10, TurnID: 20},
		{ItemID: 10, TurnID: 21},
		{ItemID: 11, TurnID: 22},
	}, plan.preserveTargetTurns)
	assert.Empty(t, plan.restoreTurnsByTargetID)
}

func TestExptResultBuilder_FillProcessingTargetResultID_DoesNotRestoreFailedCurrentRunTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		spaceID        = int64(100)
		exptID         = int64(200)
		newRunID       = int64(300)
		itemID         = int64(400)
		turnID         = int64(500)
		failedTargetID = int64(600)
	)

	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return([]*entity.ExptTurnResultRunLog{{
		ExptRunID: newRunID, ItemID: itemID, TurnID: turnID,
		Status: entity.TurnRunState_Fail, TargetResultID: failedTargetID,
		ErrMsg: errno.SerializeErr(errno.NewTargetResultErr("target failed")),
	}}, nil)
	builder := &ExptResultBuilder{
		ExptID: exptID, SpaceID: spaceID, ExptTurnResultRepo: turnRepo,
		turnResultDO: []*entity.ExptTurnResult{{
			ExptRunID: newRunID, ItemID: itemID, TurnID: turnID,
			Status: int32(entity.TurnRunState_Queueing), TargetResultID: 0,
		}},
	}

	require.NoError(t, builder.fillProcessingTargetResultID(context.Background()))
	assert.Zero(t, builder.turnResultDO[0].TargetResultID)
}

func TestExptResultBuilder_FillProcessingTargetResultID_RestoresSuccessfulTargetFromFailedTurn(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		spaceID        = int64(100)
		exptID         = int64(200)
		newRunID       = int64(300)
		itemID         = int64(400)
		turnID         = int64(500)
		targetResultID = int64(600)
	)

	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return([]*entity.ExptTurnResultRunLog{{
		ExptRunID: newRunID, ItemID: itemID, TurnID: turnID,
		Status: entity.TurnRunState_Fail, TargetResultID: targetResultID,
		ErrMsg: errno.SerializeErr(errno.NewEvaluatorResultErr("evaluator failed")),
	}}, nil)
	builder := &ExptResultBuilder{
		ExptID: exptID, SpaceID: spaceID, ExptTurnResultRepo: turnRepo,
		turnResultDO: []*entity.ExptTurnResult{{
			ExptRunID: newRunID, ItemID: itemID, TurnID: turnID,
			Status: int32(entity.TurnRunState_Queueing), TargetResultID: 0,
		}},
	}

	require.NoError(t, builder.fillProcessingTargetResultID(context.Background()))
	assert.Equal(t, targetResultID, builder.turnResultDO[0].TargetResultID)
}

func TestExptResultBuilder_FillProcessingTargetResultID_RestoresTerminalCurrentRunTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		spaceID        = int64(100)
		exptID         = int64(200)
		newRunID       = int64(300)
		itemID         = int64(400)
		turnID         = int64(500)
		failedTargetID = int64(600)
	)

	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return([]*entity.ExptTurnResultRunLog{{
		ExptRunID: newRunID, ItemID: itemID, TurnID: turnID,
		Status: entity.TurnRunState_Terminal, TargetResultID: failedTargetID,
	}}, nil)
	builder := &ExptResultBuilder{
		ExptID: exptID, SpaceID: spaceID, ExptTurnResultRepo: turnRepo,
		turnResultDO: []*entity.ExptTurnResult{{
			ExptRunID: newRunID, ItemID: itemID, TurnID: turnID,
			Status: int32(entity.TurnRunState_Queueing), TargetResultID: 0,
		}},
	}

	require.NoError(t, builder.fillProcessingTargetResultID(context.Background()))
	assert.Equal(t, failedTargetID, builder.turnResultDO[0].TargetResultID)
}

func TestExptResultBuilder_FillTargetResultID_NonRetryMultiTurnItemInFlight(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		spaceID          = int64(100)
		exptID           = int64(200)
		exptRunID        = int64(300)
		itemID           = int64(400)
		finishedTurnID   = int64(500)
		processingTurnID = int64(501)
		finishedTargetID = int64(600)
		pendingTargetID  = int64(601)
	)

	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, exptRunID, []int64{itemID}, spaceID).Return([]*entity.ExptTurnResultRunLog{
		{ExptRunID: exptRunID, ItemID: itemID, TurnID: finishedTurnID, Status: entity.TurnRunState_Success, TargetResultID: finishedTargetID},
		{ExptRunID: exptRunID, ItemID: itemID, TurnID: processingTurnID, Status: entity.TurnRunState_Processing, TargetResultID: pendingTargetID},
	}, nil)
	builder := &ExptResultBuilder{
		ExptID: exptID, SpaceID: spaceID, ExptTurnResultRepo: turnRepo,
		turnResultDO: []*entity.ExptTurnResult{
			{ExptRunID: exptRunID, ItemID: itemID, TurnID: finishedTurnID, Status: int32(entity.TurnRunState_Processing)},
			{ExptRunID: exptRunID, ItemID: itemID, TurnID: processingTurnID, Status: int32(entity.TurnRunState_Processing)},
		},
	}

	require.NoError(t, builder.fillProcessingTargetResultID(context.Background()))
	assert.Equal(t, finishedTargetID, builder.turnResultDO[0].TargetResultID)
	assert.Equal(t, pendingTargetID, builder.turnResultDO[1].TargetResultID)
}

func TestExptFailRetryExec_BuildPagePlan_ReadFallbacksAndDependencyFailure(t *testing.T) {
	const (
		exptID  = int64(1)
		runID   = int64(2)
		spaceID = int64(3)
		itemID  = int64(10)
	)

	t.Run("item result batch read failure degrades to turn log ID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, errors.New("item result unavailable"))
		itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
		turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
		success := entity.EvalTargetRunStatusSuccess
		targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{30}).Return([]*entity.EvalTargetRecord{{ID: 30, Status: &success}}, nil)
		idgen := idgenmocks.NewMockIIDGenerator(ctrl)
		idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)

		exec := &ExptFailRetryExec{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, evalTargetService: targetSvc, idgenerator: idgen}
		plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
			ExptID: exptID, ExptRunID: runID, SpaceID: spaceID,
		}, buildMockExpt(), []*entity.ExptTurnResult{{ID: 100, ItemID: itemID, TurnID: 20, TargetResultID: 30, LogID: "turn-log"}})

		require.NoError(t, err)
		assert.Equal(t, "turn-log", plan.itemIDToLogID[itemID])
		assert.Zero(t, plan.itemRunLogs[0].ItemVersionID)
		assert.Equal(t, []*entity.ItemTurnID{{ItemID: itemID, TurnID: 20}}, plan.preserveTargetTurns)
	})

	t.Run("missing item result row degrades to turn log ID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		idgen := idgenmocks.NewMockIIDGenerator(ctrl)
		itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, nil)
		itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
		turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
		success := entity.EvalTargetRunStatusSuccess
		targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{30}).Return([]*entity.EvalTargetRecord{{ID: 30, Status: &success}}, nil)
		idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)

		exec := &ExptFailRetryExec{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, evalTargetService: targetSvc, idgenerator: idgen}
		plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
			ExptID: exptID, ExptRunID: runID, SpaceID: spaceID,
		}, buildMockExpt(), []*entity.ExptTurnResult{{ID: 100, ItemID: itemID, ItemVersionID: 9, TurnID: 20, TargetResultID: 30, LogID: "turn-log"}})

		require.NoError(t, err)
		assert.Equal(t, "turn-log", plan.itemIDToLogID[itemID])
		assert.Equal(t, int64(9), plan.itemRunLogs[0].ItemVersionID)
		assert.Equal(t, []*entity.ItemTurnID{{ItemID: itemID, TurnID: 20}}, plan.preserveTargetTurns)
	})

	t.Run("current item run log read failure degrades to canonical item log ID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		idgen := idgenmocks.NewMockIIDGenerator(ctrl)
		itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID, LogID: "item-log"}}, nil)
		itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, errors.New("run log unavailable"))
		turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
		idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)

		expt := buildMockExpt()
		expt.TargetVersionID = 0
		exec := &ExptFailRetryExec{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, idgenerator: idgen}
		plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
			ExptID: exptID, ExptRunID: runID, SpaceID: spaceID,
		}, expt, []*entity.ExptTurnResult{{ID: 100, ItemID: itemID, TurnID: 20}})

		require.NoError(t, err)
		assert.Equal(t, "item-log", plan.itemIDToLogID[itemID])
	})

	t.Run("multi-set item ref batch read failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
		itemRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)
		metric := metricsmocks.NewMockExptMetric(ctrl)
		itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID}}, nil)
		itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
		turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
		itemRefRepo.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, errors.New("item ref unavailable"))
		metric.EXPECT().EmitRetryStartDependencyFailure(spaceID, "expt_item_ref")

		expt := buildMockExpt()
		expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
		exec := &ExptFailRetryExec{
			exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo,
			exptItemRefRepo: itemRefRepo, metric: metric,
		}
		plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
			ExptID: exptID, ExptRunID: runID, SpaceID: spaceID,
		}, expt, []*entity.ExptTurnResult{{ItemID: itemID, TurnID: 20, TargetResultID: 30}})

		require.Error(t, err)
		assert.Nil(t, plan)
		assert.True(t, isSchedulerInfraError(err))
		assert.Contains(t, err.Error(), "expt_item_ref")
	})
}

func TestExptFailRetryExec_BuildPagePlan_MissingTargetServiceFallsBackToClear(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID  = int64(1)
		runID   = int64(2)
		spaceID = int64(3)
		itemID  = int64(10)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID, LogID: "item-log"}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	metric.EXPECT().EmitRetryStartDependencyFailure(spaceID, "eval_target_service")

	exec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo,
		idgenerator: idgen, metric: metric,
	}
	plan, err := exec.buildPagePlan(context.Background(), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: runID, SpaceID: spaceID,
	}, buildMockExpt(), []*entity.ExptTurnResult{{ItemID: itemID, TurnID: 20, TargetResultID: 30}})

	require.NoError(t, err)
	assert.Empty(t, plan.preserveTargetTurns)
	assert.Empty(t, plan.restoreTurnsByTargetID)
	assert.Equal(t, []*entity.ItemTurnID{{ItemID: itemID, TurnID: 20}}, plan.clearTargetTurns)
}

// Helper-level compatibility only: RetryFailure currently receives the legacy
// version-to-record map from GetExptItemTurnResults, so this does not assert
// that alias/inline reuse is reachable end to end on the current read path.
func TestPruneSuccessfulEvaluatorRecords_HelperPreservesUnambiguousNewFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		registeredRecordID = int64(1001)
		inlineRecordID     = int64(1002)
	)

	evalRecord := svcmocks.NewMockEvaluatorRecordService(ctrl)
	evalRecord.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{registeredRecordID, inlineRecordID}, false, false).Return([]*entity.EvaluatorRecord{
		{ID: registeredRecordID, EvaluatorVersionID: 10, Alias: "judge-a", SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess},
		{ID: inlineRecordID, InlineKey: "inline-a", SourceType: entity.EvaluatorRecordSourceTypeInline, Status: entity.EvaluatorRunStatusSuccess},
	}, nil)

	got := pruneSuccessfulEvaluatorRecords(context.Background(), evalRecord, &entity.ExptTurnResult{
		EvaluatorResults: &entity.EvaluatorResults{
			Registered: []*entity.RegisteredEvalResult{{VersionID: 10, Alias: "judge-a", RecordID: registeredRecordID}},
			Inline:     []*entity.InlineEvalResult{{InlineKey: "inline-a", RecordID: inlineRecordID}},
		},
	})

	require.NotNil(t, got)
	assert.Equal(t, []*entity.RegisteredEvalResult{{VersionID: 10, Alias: "judge-a", RecordID: registeredRecordID}}, got.Registered)
	assert.Equal(t, []*entity.InlineEvalResult{{InlineKey: "inline-a", RecordID: inlineRecordID}}, got.Inline)
}

func TestPruneSuccessfulEvaluatorRecords_DoesNotMapAliasRecordIntoLegacyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	const recordID = int64(1001)

	evalRecord := svcmocks.NewMockEvaluatorRecordService(ctrl)
	evalRecord.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{recordID}, false, false).Return([]*entity.EvaluatorRecord{{
		ID: recordID, EvaluatorVersionID: 10, Alias: "judge-a",
		SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess,
	}}, nil)

	got := pruneSuccessfulEvaluatorRecords(context.Background(), evalRecord, &entity.ExptTurnResult{
		EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{10: recordID}},
	})
	assert.Nil(t, got)
}

func TestExptFailRetryExec_ExptStart_BatchesUnchangedSuccessfulTargetsIntoOneUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID   = int64(1)
		newRunID = int64(2)
		spaceID  = int64(3)
		itemID   = int64(10)
	)

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)

	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(0), int64(50), spaceID).Return([]*entity.ExptTurnResult{
		{ID: 100, ItemID: itemID, TurnID: 20, Status: int32(entity.TurnRunState_Fail), TargetResultID: 30},
		{ID: 101, ItemID: itemID, TurnID: 21, Status: int32(entity.TurnRunState_Fail), TargetResultID: 31},
	}, int64(101), nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(101), int64(50), spaceID).Return(nil, int64(0), nil)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{ItemID: itemID, LogID: "item-log"}}, nil)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, newRunID, []int64{itemID}, spaceID).Return(nil, nil)
	success := entity.EvalTargetRunStatusSuccess
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{30, 31}).Return([]*entity.EvalTargetRecord{
		{ID: 30, Status: &success},
		{ID: 31, Status: &success},
	}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
	itemRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil)
	itemRepo.EXPECT().FillItemRunLogLogIDIfEmpty(gomock.Any(), exptID, newRunID, spaceID, map[int64]string{itemID: "item-log"}).Return(nil)
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, gomock.Any()).Return(nil)
	turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, []*entity.ItemTurnID{
		{ItemID: itemID, TurnID: 20},
		{ItemID: itemID, TurnID: 21},
	}, spaceID, failRetryTurnUpdateMatcher{wantRunID: newRunID}).Return(nil).Times(1)
	statsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{FailItemCnt: 1}, nil)
	statsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	configer.EXPECT().GetExptExecConf(gomock.Any(), spaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	exec := &ExptFailRetryExec{
		exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, exptStatsRepo: statsRepo,
		idgenerator: idgen, exptRepo: exptRepo, idem: idem, configer: configer,
		evalTargetService: targetSvc,
	}
	require.NoError(t, exec.ExptStart(context.Background(), &entity.ExptScheduleEvent{
		ExptID: exptID, ExptRunID: newRunID, SpaceID: spaceID,
	}, buildMockExpt()))
}
