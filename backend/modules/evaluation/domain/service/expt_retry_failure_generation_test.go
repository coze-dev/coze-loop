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
)

type retryGenerationFixture struct {
	expt        *entity.Experiment
	itemConfig  *entity.ExptItemConfig
	targetSpace int64
	turns       []*entity.ExptTurnResult
	records     []*entity.EvaluatorRecord
	sources     map[int64][]*entity.ExptTurnResultRunLog
	sourceErr   error
	recordErr   error
	repeats     int
}

func newRetryGenerationFixture() *retryGenerationFixture {
	return &retryGenerationFixture{
		expt: buildRetryFailureSingleSetExpt(3, 401, 402), targetSpace: 3,
		turns: []*entity.ExptTurnResult{{
			ID: 21, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, TurnID: 20, TargetResultID: 201,
			EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 301, 402: 302}},
		}},
		records: []*entity.EvaluatorRecord{
			{ID: 301, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 101, ItemID: 10, TurnID: 20, EvaluatorVersionID: 401, Status: entity.EvaluatorRunStatusSuccess},
			{ID: 302, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 102, ItemID: 10, TurnID: 20, EvaluatorVersionID: 402, Status: entity.EvaluatorRunStatusSuccess},
		},
		sources: map[int64][]*entity.ExptTurnResultRunLog{
			101: {{
				SpaceID: 3, ExptID: 1, ExptRunID: 101, ItemID: 10, TurnID: 20, TargetResultID: 201,
				EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 301}},
			}},
			102: {{
				SpaceID: 3, ExptID: 1, ExptRunID: 102, ItemID: 10, TurnID: 20, TargetResultID: 201,
				EvaluatorResultIds: &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 301}, {VersionID: 402, RecordID: 302}}},
			}},
		},
	}
}

func runRetryGenerationPreEval(t *testing.T, f *retryGenerationFixture) ([]*entity.ExptTurnResultRunLog, map[int64]int, int, error) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repeats := f.repeats
	if repeats == 0 {
		repeats = 1
	}
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	recordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	turnIDs := make([]int64, 0, len(f.turns))
	var refs []*entity.ExptTurnEvaluatorResultRef
	for _, tr := range f.turns {
		turnIDs = append(turnIDs, tr.ID)
		for versionID, recordID := range tr.EvaluatorResults.EvalVerIDToResID {
			refs = append(refs, &entity.ExptTurnEvaluatorResultRef{SpaceID: 3, ExptID: 1, ExptTurnResultID: tr.ID, EvaluatorVersionID: versionID, EvaluatorResultID: recordID})
		}
	}
	turnRepo.EXPECT().GetItemTurnResults(gomock.Any(), int64(1), int64(10), int64(3)).Return(f.turns, nil).Times(repeats)
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(3), turnIDs).Return(refs, nil).Times(2 * repeats)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), len(f.turns)).Return(turnIDs, nil).Times(repeats)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), f.targetSpace, int64(201)).DoAndReturn(func(ctx context.Context, _, _ int64) (*entity.EvalTargetRecord, error) {
		assert.True(t, contexts.CtxWriteDB(ctx))
		return &entity.EvalTargetRecord{ID: 201, ExperimentRunID: 101, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil
	}).AnyTimes()
	recordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(ctx context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
		assert.True(t, contexts.CtxWriteDB(ctx))
		wanted := make(map[int64]bool, len(ids))
		for _, id := range ids {
			wanted[id] = true
		}
		var records []*entity.EvaluatorRecord
		for _, record := range f.records {
			if wanted[record.ID] {
				records = append(records, record)
			}
		}
		return records, f.recordErr
	}).AnyTimes()
	reads := make(map[int64]int)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), gomock.Any(), []int64{10}, int64(3)).DoAndReturn(func(ctx context.Context, _, runID int64, _ []int64, _ int64) ([]*entity.ExptTurnResultRunLog, error) {
		assert.True(t, contexts.CtxWriteDB(ctx))
		reads[runID]++
		if runID == 102 && f.sourceErr != nil {
			return nil, f.sourceErr
		}
		return f.sources[runID], nil
	}).AnyTimes()
	var saved []*entity.ExptTurnResultRunLog
	writes := 0
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
		writes++
		saved = rows
		return nil
	}).AnyTimes()
	mode := &ExptRecordEvalModeFailRetry{
		resultSvc: &ExptResultServiceImpl{ExptTurnResultRepo: turnRepo}, exptTurnResultRepo: turnRepo,
		idgen: idgen, evalTargetService: targetSvc, evaluatorRecordSvc: recordSvc,
	}
	var err error
	for i := 0; i < repeats; i++ {
		err = mode.PreEval(context.Background(), &entity.ExptItemEvalCtx{
			Expt: f.expt, ItemConfig: f.itemConfig,
			Event:               &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 103 + int64(i), EvalSetItemID: 10, SpaceID: 3, ExptRunMode: entity.EvaluationModeFailRetry},
			ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)},
		})
		if err != nil {
			break
		}
	}
	return saved, reads, writes, err
}

func TestRetryFailure_PreEvalEvaluatorGeneration(t *testing.T) {
	tests := []struct {
		name       string
		change     func(*retryGenerationFixture)
		wantIDs    map[int64]int64
		wantReads  map[int64]int
		wantTarget int64
	}{
		{name: "legal multi-generation reuse", wantIDs: map[int64]int64{401: 301, 402: 302}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "mixed target generations", change: func(f *retryGenerationFixture) { f.sources[102][0].TargetResultID = 202 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "late success from another target", change: func(f *retryGenerationFixture) {
			f.sources[102][0].TargetResultID = 202
			f.sources[102][0].Status = entity.TurnRunState_Fail
		}, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "same target but record no longer in source log", change: func(f *retryGenerationFixture) {
			f.sources[102][0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 402, RecordID: 999}}}
		}, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "source log missing", change: func(f *retryGenerationFixture) { delete(f.sources, 102) }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "source refs missing", change: func(f *retryGenerationFixture) { f.sources[102][0].EvaluatorResultIds = nil }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "old record without run ID", change: func(f *retryGenerationFixture) { f.records[1].ExperimentRunID = 0 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1}, wantTarget: 201},
		{name: "record belongs to another experiment", change: func(f *retryGenerationFixture) { f.records[1].ExperimentID = 9 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1}, wantTarget: 201},
		{name: "record belongs to another item", change: func(f *retryGenerationFixture) { f.records[1].ItemID = 11 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1}, wantTarget: 201},
		{name: "record belongs to another turn", change: func(f *retryGenerationFixture) { f.records[1].TurnID = 22 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1}, wantTarget: 201},
		{name: "source log wrong space", change: func(f *retryGenerationFixture) { f.sources[102][0].SpaceID = 9 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "source log wrong experiment", change: func(f *retryGenerationFixture) { f.sources[102][0].ExptID = 9 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "source log wrong run", change: func(f *retryGenerationFixture) { f.sources[102][0].ExptRunID = 999 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "source log wrong item", change: func(f *retryGenerationFixture) { f.sources[102][0].ItemID = 11 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "source log wrong turn", change: func(f *retryGenerationFixture) { f.sources[102][0].TurnID = 22 }, wantIDs: map[int64]int64{401: 301}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "failed source turn still contains successful evaluator", change: func(f *retryGenerationFixture) { f.sources[102][0].Status = entity.TurnRunState_Fail }, wantIDs: map[int64]int64{401: 301, 402: 302}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "multiset target and shared evaluator use separate spaces", change: func(f *retryGenerationFixture) {
			f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
			f.expt.TargetSpaceID = 80
			f.itemConfig = &entity.ExptItemConfig{TargetSourceSpaceID: 90}
			f.targetSpace = 90
			f.records[1].SpaceID = 70
		}, wantIDs: map[int64]int64{401: 301, 402: 302}, wantReads: map[int64]int{101: 1, 102: 1}, wantTarget: 201},
		{name: "no target version retains old successful records", change: func(f *retryGenerationFixture) { f.expt.TargetVersionID = 0; f.records[1].ExperimentRunID = 0 }, wantIDs: map[int64]int64{401: 301, 402: 302}, wantReads: map[int64]int{}, wantTarget: 0},
		{name: "online retains old successful records", change: func(f *retryGenerationFixture) {
			f.expt.ExptType = entity.ExptType_Online
			f.records[1].ExperimentRunID = 0
		}, wantIDs: map[int64]int64{401: 301, 402: 302}, wantReads: map[int64]int{}, wantTarget: 0},
		{name: "record only retains old successful records", change: func(f *retryGenerationFixture) {
			f.expt.TargetType = 0
			f.expt.Target.EvalTargetType = entity.EvalTargetTypeCustomRPCServerOnline
			f.records[1].ExperimentRunID = 0
		}, wantIDs: map[int64]int64{401: 301, 402: 302}, wantReads: map[int64]int{}, wantTarget: 0},
		{name: "record read failure preserves source", change: func(f *retryGenerationFixture) { f.recordErr = errors.New("record read unavailable") }, wantReads: map[int64]int{}, wantTarget: 201},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newRetryGenerationFixture()
			if tt.change != nil {
				tt.change(fixture)
			}
			saved, reads, writes, err := runRetryGenerationPreEval(t, fixture)
			if fixture.recordErr != nil {
				require.ErrorIs(t, err, fixture.recordErr)
				assert.Zero(t, writes)
				assert.Empty(t, saved)
				assert.Empty(t, reads)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, writes)
			require.Len(t, saved, 1)
			assert.Equal(t, tt.wantTarget, saved[0].TargetResultID)
			if len(tt.wantIDs) == 0 {
				assert.Nil(t, saved[0].EvaluatorResultIds)
			} else {
				require.NotNil(t, saved[0].EvaluatorResultIds)
				assert.Equal(t, tt.wantIDs, saved[0].EvaluatorResultIds.EvalVerIDToResID)
			}
			assert.Equal(t, tt.wantReads, reads)
		})
	}
}

func TestRetryFailure_PreEvalSourceReadFailureDoesNotWritePartialSnapshot(t *testing.T) {
	f := newRetryGenerationFixture()
	f.sourceErr = errors.New("source run log unavailable")
	saved, reads, writes, err := runRetryGenerationPreEval(t, f)
	require.ErrorIs(t, err, f.sourceErr)
	assert.Empty(t, saved)
	assert.Zero(t, writes)
	assert.Equal(t, map[int64]int{101: 1, 102: 1}, reads)
	assert.Equal(t, int64(201), f.turns[0].TargetResultID)
}

func TestRetryFailure_PreEvalStopsOnFirstSourceReadError(t *testing.T) {
	f := newRetryGenerationFixture()
	f.records[0].ExperimentRunID = 102
	f.sourceErr = errors.New("source run log unavailable")
	saved, reads, writes, err := runRetryGenerationPreEval(t, f)
	require.ErrorIs(t, err, f.sourceErr)
	assert.Empty(t, saved)
	assert.Zero(t, writes)
	assert.Equal(t, map[int64]int{102: 1}, reads)
}

func TestRetryFailure_PreEvalCachesSourceRunAcrossTurns(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "shared origin", true: "missing origin"}[missing], func(t *testing.T) {
			f := newRetryGenerationFixture()
			f.turns[0].EvaluatorResults = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 301}}
			f.turns = append(f.turns, &entity.ExptTurnResult{
				ID: 22, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, TurnID: 21, TargetResultID: 201,
				EvaluatorResults: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{402: 302}},
			})
			f.records[1].ExperimentRunID = 101
			f.records[1].TurnID = 21
			f.sources[101] = append(f.sources[101], &entity.ExptTurnResultRunLog{
				SpaceID: 3, ExptID: 1, ExptRunID: 101, ItemID: 10, TurnID: 21, TargetResultID: 201,
				EvaluatorResultIds: &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 402, RecordID: 302}}},
			})
			if missing {
				delete(f.sources, 101)
			}
			saved, reads, writes, err := runRetryGenerationPreEval(t, f)
			require.NoError(t, err)
			require.Equal(t, 1, writes)
			require.Len(t, saved, 2)
			assert.Equal(t, map[int64]int{101: 1}, reads)
			if missing {
				assert.Nil(t, saved[0].EvaluatorResultIds)
				assert.Nil(t, saved[1].EvaluatorResultIds)
			} else {
				require.NotNil(t, saved[0].EvaluatorResultIds)
				require.NotNil(t, saved[1].EvaluatorResultIds)
				assert.Equal(t, map[int64]int64{401: 301}, saved[0].EvaluatorResultIds.EvalVerIDToResID)
				assert.Equal(t, map[int64]int64{402: 302}, saved[1].EvaluatorResultIds.EvalVerIDToResID)
			}
		})
	}
}

func TestRetryFailure_DifferentTargetGenerationRerunsEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	const (
		exptID             int64 = 1
		spaceID            int64 = 3
		itemID             int64 = 10
		turnID             int64 = 20
		turnResultID       int64 = 21
		run1               int64 = 101
		run2               int64 = 102
		run3               int64 = 103
		target1            int64 = 201
		target2            int64 = 202
		oldEvaluatorID     int64 = 301
		freshEvaluatorID   int64 = 302
		evaluatorVersionID int64 = 401
	)
	ctx := context.Background()
	expt := buildRetryFailureSingleSetExpt(spaceID, evaluatorVersionID)
	expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].IngressConf.TargetAdapter.FieldConfs = []*entity.FieldConf{{FieldName: "answer", FromField: "answer"}}
	item := &entity.EvaluationSetItem{ItemID: itemID, Turns: []*entity.Turn{{ID: turnID, ItemID: itemID}}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}}
	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	configer := configmocks.NewMockIConfiger(ctrl)
	idgen := idgenmocks.NewMockIIDGenerator(ctrl)
	targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	evaluatorRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
	evaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
	benefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	canonicalTurn := &entity.ExptTurnResult{ID: turnResultID, ExptID: exptID, ExptRunID: run1, SpaceID: spaceID, ItemID: itemID, TurnID: turnID, TargetResultID: target1, Status: int32(entity.TurnRunState_Success), LogID: "original-log"}
	canonicalItem := &entity.ExptItemResult{ItemID: itemID, ExptRunID: run2, Status: entity.ItemRunState_Processing, LogID: "retry-log"}
	canonicalRefs := []*entity.ExptTurnEvaluatorResultRef{{ExptID: exptID, SpaceID: spaceID, ExptTurnResultID: turnResultID, EvaluatorVersionID: evaluatorVersionID, EvaluatorResultID: oldEvaluatorID}}
	oldRecord := &entity.EvaluatorRecord{
		ID: oldEvaluatorID, SpaceID: spaceID, ExperimentID: exptID, ExperimentRunID: run1, ItemID: itemID, TurnID: turnID, EvaluatorVersionID: evaluatorVersionID, Status: entity.EvaluatorRunStatusSuccess,
		EvaluatorInputData:  &entity.EvaluatorInputData{InputFields: map[string]*entity.Content{"answer": {Text: gptr.Of("output-from-T1")}}},
		EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.9)}},
	}
	status := entity.EvalTargetRunStatusSuccess
	t2 := &entity.EvalTargetRecord{ID: target2, ExperimentRunID: run2, Status: &status, EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{"answer": {Text: gptr.Of("output-from-T2")}}}}
	var r2log, r3log *entity.ExptTurnResultRunLog
	turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
		require.Len(t, rows, 1)
		if rows[0].ExptRunID == run2 {
			r2log = rows[0]
		} else {
			r3log = rows[0]
		}
		return nil
	}).Times(2)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{canonicalItem}, nil).AnyTimes()
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), spaceID, exptID, []int64{itemID}, gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, fields map[string]any) error {
		if v, ok := fields["status"].(int32); ok {
			canonicalItem.Status = entity.ItemRunState(v)
		}
		return nil
	}).AnyTimes()
	configer.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl()).AnyTimes()
	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, ItemResultRepo: itemRepo, Configer: configer, evalTargetService: targetSvc, evaluatorRecordService: evaluatorRecordSvc}
	turnEval := &DefaultExptTurnEvaluationImpl{evalTargetService: targetSvc, evaluatorService: evaluatorSvc, evaluatorRecordService: evaluatorRecordSvc, benefitService: benefitSvc, metric: metric}
	// R2 has finished target T2. The real evaluator stage fails before creating a record.
	r2ctx := &entity.ExptTurnEvalCtx{Turn: item.Turns[0], ExptTurnRunResult: &entity.ExptTurnRunResult{}, ExptItemEvalCtx: &entity.ExptItemEvalCtx{
		Expt: expt, EvalSetItem: item,
		Event:               &entity.ExptItemEvalEvent{ExptID: exptID, ExptRunID: run2, SpaceID: spaceID, EvalSetItemID: itemID, ExptRunMode: entity.EvaluationModeRetryAll, Session: &entity.Session{UserID: "u"}},
		ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{turnID: {ID: 700, ExptID: exptID, ExptRunID: run2, SpaceID: spaceID, ItemID: itemID, TurnID: turnID}}},
	}}
	benefitSvc.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, errors.New("quota denied before evaluator record"))
	r2evals, r2err := turnEval.CallEvaluators(ctx, r2ctx, t2)
	require.Error(t, r2err)
	require.Empty(t, r2evals)
	require.NoError(t, executor.storeTurnRunResult(ctx, r2ctx, &entity.ExptTurnRunResult{TargetResult: t2, EvaluatorResults: r2evals, EvalErr: r2err}))
	require.NotNil(t, r2log)
	require.Equal(t, target2, r2log.TargetResultID)
	require.Empty(t, r2log.EvaluatorResultIds.Registered)
	// The upgrade starts from an old R2 projection: T2 was stored, but E1 remained in canonical refs.
	canonicalTurn.TargetResultID = r2log.TargetResultID
	canonicalTurn.ExptRunID = run2
	canonicalTurn.Status = int32(r2log.Status)
	resultSvc := &ExptResultServiceImpl{ExptItemResultRepo: itemRepo, ExptTurnResultRepo: turnRepo, ExptStatsRepo: statsRepo, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil), evaluatorRecordService: evaluatorRecordSvc, idgen: idgen}
	require.Equal(t, target2, canonicalTurn.TargetResultID)
	require.Equal(t, int32(entity.TurnRunState_Fail), canonicalTurn.Status)
	// R3 uses the actual changed ExptStart and repository update fields.
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), int64(0), int64(50), spaceID).Return([]*entity.ExptTurnResult{canonicalTurn}, turnResultID, nil)
	turnRepo.EXPECT().ScanTurnResults(gomock.Any(), exptID, gomock.Any(), turnResultID, int64(50), spaceID).Return(nil, int64(0), nil)

	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, run3, []int64{itemID}, spaceID).Return(nil, nil)
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, []int64{target2}).Return([]*entity.EvalTargetRecord{t2}, nil)
	idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{800}, nil).AnyTimes()
	itemRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).Return(nil)

	turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), exptID, []*entity.ItemTurnID{{ItemID: itemID, TurnID: turnID}}, spaceID, gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ []*entity.ItemTurnID, _ int64, fields map[string]any) error {
		canonicalTurn.Status = fields["status"].(int32)
		canonicalTurn.ExptRunID = fields["expt_run_id"].(int64)
		if id, ok := fields["target_result_id"].(int64); ok {
			canonicalTurn.TargetResultID = id
		}
		return nil
	})
	statsRepo.EXPECT().Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{FailItemCnt: 1}, nil)
	statsRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	exptRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	configer.EXPECT().GetExptExecConf(gomock.Any(), spaceID).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	start := &ExptFailRetryExec{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo, exptStatsRepo: statsRepo, idgenerator: idgen, exptRepo: exptRepo, idem: idem, configer: configer, evalTargetService: targetSvc}
	require.NoError(t, start.ExptStart(ctx, &entity.ExptScheduleEvent{ExptID: exptID, ExptRunID: run3, SpaceID: spaceID}, expt))
	require.Equal(t, target2, canonicalTurn.TargetResultID)
	eiec := &entity.ExptItemEvalCtx{Expt: expt, EvalSetItem: item, Event: &entity.ExptItemEvalEvent{ExptID: exptID, ExptRunID: run3, SpaceID: spaceID, EvalSetItemID: itemID, ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"}}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)}}
	// The real preload service reconstructs refs; it is not replaced with a mock conclusion.
	turnRepo.EXPECT().GetItemTurnResults(gomock.Any(), exptID, itemID, spaceID).Return([]*entity.ExptTurnResult{canonicalTurn}, nil)
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{turnResultID}).Return(canonicalRefs, nil).Times(2)
	targetSvc.EXPECT().GetRecordByID(gomock.Any(), spaceID, target2).Return(t2, nil).Times(2)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{oldEvaluatorID}, false, false).Return([]*entity.EvaluatorRecord{oldRecord}, nil).Times(1)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, run1, []int64{itemID}, spaceID).Return([]*entity.ExptTurnResultRunLog{{SpaceID: spaceID, ExptID: exptID, ExptRunID: run1, ItemID: itemID, TurnID: turnID, TargetResultID: target1, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{evaluatorVersionID: oldEvaluatorID}}}}, nil)
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).Return(nil)
	pre := &ExptRecordEvalModeFailRetry{resultSvc: resultSvc, exptTurnResultRepo: turnRepo, idgen: idgen, evalTargetService: targetSvc, evaluatorRecordSvc: evaluatorRecordSvc}
	require.NoError(t, pre.PreEval(ctx, eiec))
	configer.EXPECT().BuildEvalExt(gomock.Any(), spaceID, gomock.Any()).Return(nil)
	etec, err := executor.buildExptTurnEvalCtx(ctx, item.Turns[0], eiec, nil)
	require.NoError(t, err)
	freshCalls := 0
	benefitSvc.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).AnyTimes()
	evaluatorSvc.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).AnyTimes()
	evaluatorSvc.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		freshCalls++
		assert.Equal(t, "output-from-T2", req.InputData.InputFields["answer"].GetText())
		return &entity.EvaluatorRecord{ID: freshEvaluatorID, SpaceID: spaceID, ExperimentID: exptID, ItemID: itemID, TurnID: turnID, EvaluatorVersionID: evaluatorVersionID, ExperimentRunID: run3, Status: entity.EvaluatorRunStatusSuccess}, nil
	}).AnyTimes()
	metric.EXPECT().EmitTurnExecEvaluatorResult(spaceID, false).AnyTimes()
	targetResult, err := turnEval.CallTarget(ctx, etec)
	require.NoError(t, err)
	require.Equal(t, target2, targetResult.ID)
	results, err := turnEval.CallEvaluators(ctx, etec, targetResult)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, executor.storeTurnRunResult(ctx, etec, &entity.ExptTurnRunResult{TargetResult: targetResult, EvaluatorResults: results}))
	require.NotNil(t, r3log)
	t.Logf("R3 selected target=%d output=%s, evaluator=%d originating_run=%d, new evaluator calls=%d, persisted status=%v", targetResult.ID, targetResult.EvalTargetOutputData.OutputFields["answer"].GetText(), results[0].ID, results[0].ExperimentRunID, freshCalls, r3log.Status)
	if results[0].EvaluatorInputData != nil {
		t.Logf("R3 selected evaluator input=%s score=%v", results[0].EvaluatorInputData.InputFields["answer"].GetText(), *results[0].EvaluatorOutputData.EvaluatorResult.Score)
	}
	assert.Equal(t, 1, freshCalls, "T2 never had a completed evaluator: RetryFailure must actually evaluate T2")
	assert.NotEqual(t, oldEvaluatorID, results[0].ID, "must not attach T1 score to T2 output")

	// R3 must publish the fresh evaluator reference together with T2.
	itemRepo.EXPECT().GetItemRunLog(gomock.Any(), exptID, run3, itemID, spaceID).Return(&entity.ExptItemResultRunLog{Status: int32(entity.ItemRunState_Success), ResultState: int32(entity.ExptItemResultStateLogged), LogID: "r3-log"}, nil)
	turnRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), exptID, run3, itemID, spaceID).Return([]*entity.ExptTurnResultRunLog{r3log}, nil)
	itemRepo.EXPECT().GetItemTurnResults(gomock.Any(), spaceID, exptID, itemID).Return([]*entity.ExptTurnResult{canonicalTurn}, nil)
	evaluatorRecordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{freshEvaluatorID}, false, false).Return(results, nil)
	turnRepo.EXPECT().CreateTurnEvaluatorRefs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, refs []*entity.ExptTurnEvaluatorResultRef) error {
		require.Len(t, refs, 1)
		assert.Equal(t, freshEvaluatorID, refs[0].EvaluatorResultID)
		canonicalRefs = refs
		return nil
	})
	turnRepo.EXPECT().SaveTurnResults(gomock.Any(), gomock.Any()).Return(nil)
	itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), exptID, run3, []int64{itemID}, gomock.Any(), spaceID).Return(nil)
	statsRepo.EXPECT().ArithOperateCount(gomock.Any(), exptID, spaceID, gomock.Any()).Return(nil)
	_, err = resultSvc.RecordItemRunLogs(ctx, exptID, run3, itemID, spaceID, expt)
	require.NoError(t, err)
	assert.Equal(t, run3, canonicalTurn.ExptRunID)
	assert.Equal(t, target2, canonicalTurn.TargetResultID)
	assert.Equal(t, int32(entity.TurnRunState_Success), canonicalTurn.Status)
	assert.Equal(t, freshEvaluatorID, canonicalRefs[0].EvaluatorResultID)
}

func TestRetryFailure_PreEvalSourceCacheDoesNotOutliveInvocation(t *testing.T) {
	f := newRetryGenerationFixture()
	f.repeats = 2
	saved, reads, writes, err := runRetryGenerationPreEval(t, f)
	require.NoError(t, err)
	assert.Equal(t, 2, writes)
	assert.Equal(t, map[int64]int{101: 2, 102: 2}, reads)
	require.Len(t, saved, 1)
	assert.Equal(t, int64(104), saved[0].ExptRunID)
	assert.Equal(t, map[int64]int64{401: 301, 402: 302}, saved[0].EvaluatorResultIds.EvalVerIDToResID)
}

func TestRetryFailure_SourceLogMembershipPreservesNewFormatIdentity(t *testing.T) {
	for _, tt := range []struct {
		name       string
		change     func(*entity.ExptTurnResultRunLog)
		registered bool
		inline     bool
	}{
		{name: "both identities match", registered: true, inline: true},
		{name: "registered alias mismatch", change: func(rl *entity.ExptTurnResultRunLog) { rl.EvaluatorResultIds.Registered[0].Alias = "judge-b" }, inline: true},
		{name: "registered version mismatch", change: func(rl *entity.ExptTurnResultRunLog) { rl.EvaluatorResultIds.Registered[0].VersionID = 999 }, inline: true},
		{name: "inline key mismatch", change: func(rl *entity.ExptTurnResultRunLog) { rl.EvaluatorResultIds.Inline[0].InlineKey = "inline-b" }, registered: true},
		{name: "both records absent", change: func(rl *entity.ExptTurnResultRunLog) {
			rl.EvaluatorResultIds.Registered[0].RecordID = 999
			rl.EvaluatorResultIds.Inline[0].RecordID = 998
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
			recordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
			tr := &entity.ExptTurnResult{
				SpaceID: 3, ExptID: 1, ItemID: 10, TurnID: 20, TargetResultID: 201,
				EvaluatorResults: &entity.EvaluatorResults{
					Registered: []*entity.RegisteredEvalResult{{VersionID: 401, Alias: "judge-a", RecordID: 301}},
					Inline:     []*entity.InlineEvalResult{{InlineKey: "inline-a", RecordID: 302}},
				},
			}
			origin := &entity.ExptTurnResultRunLog{
				SpaceID: 3, ExptID: 1, ExptRunID: 101, ItemID: 10, TurnID: 20, TargetResultID: 201,
				EvaluatorResultIds: &entity.EvaluatorResults{
					Registered: []*entity.RegisteredEvalResult{{VersionID: 401, Alias: "judge-a", RecordID: 301}, nil},
					Inline:     []*entity.InlineEvalResult{{InlineKey: "inline-a", RecordID: 302}, nil},
				},
			}
			if tt.change != nil {
				tt.change(origin)
			}
			targetSvc.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(&entity.EvalTargetRecord{ID: 201, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil)
			recordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{301, 302}, false, false).Return([]*entity.EvaluatorRecord{
				{ID: 301, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 101, ItemID: 10, TurnID: 20, EvaluatorVersionID: 401, Alias: "judge-a", SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess},
				{ID: 302, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 101, ItemID: 10, TurnID: 20, InlineKey: "inline-a", SourceType: entity.EvaluatorRecordSourceTypeInline, Status: entity.EvaluatorRunStatusSuccess},
			}, nil)
			turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(101), []int64{10}, int64(3)).Return([]*entity.ExptTurnResultRunLog{nil, origin}, nil)
			sources := &retryEvaluatorSourceLogs{repo: turnRepo, event: &entity.ExptItemEvalEvent{ExptID: 1, EvalSetItemID: 10, SpaceID: 3}, logsByRun: make(map[int64]map[int64]*entity.ExptTurnResultRunLog)}
			targetID, got, selectErr := failRetrySelectTurnRunLogRefs(context.Background(), 3, true, tr, targetSvc, recordSvc, []*entity.ExptTurnEvaluatorResultRef{{ExptTurnResultID: tr.ID, EvaluatorVersionID: 401, Alias: "judge-a", EvaluatorResultID: 301}, {ExptTurnResultID: tr.ID, InlineKey: "inline-a", SourceType: int32(entity.EvaluatorRecordSourceTypeInline), EvaluatorResultID: 302}}, sources)
			require.NoError(t, selectErr)
			assert.Equal(t, int64(201), targetID)
			if !tt.registered && !tt.inline {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			if tt.registered {
				assert.Equal(t, []*entity.RegisteredEvalResult{{VersionID: 401, Alias: "judge-a", RecordID: 301}}, got.Registered)
			} else {
				assert.Empty(t, got.Registered)
			}
			if tt.inline {
				assert.Equal(t, []*entity.InlineEvalResult{{InlineKey: "inline-a", RecordID: 302}}, got.Inline)
			} else {
				assert.Empty(t, got.Inline)
			}
		})
	}
}
