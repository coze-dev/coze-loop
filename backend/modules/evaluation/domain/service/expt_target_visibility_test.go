// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	lwtMocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestExptTargetVisibility_CurrentRunLifecycle(t *testing.T) {
	for _, baseline := range []bool{true, false} {
		path := "non_baseline_builder"
		if baseline {
			path = "baseline_MGetExperimentResult"
		}
		t.Run(path, func(t *testing.T) {
			for _, tc := range []struct {
				name         string
				canonicalID  int64
				turnStatus   entity.TurnRunState
				targetStatus entity.EvalTargetRunStatus
				targetRunID  int64
				failure      bool
			}{
				{"processing", 0, entity.TurnRunState_Processing, entity.EvalTargetRunStatusAsyncInvoking, 2, false},
				{"target_failed_before_projection", 0, entity.TurnRunState_Fail, entity.EvalTargetRunStatusFail, 2, true},
				{"target_failed_after_projection", 5, entity.TurnRunState_Fail, entity.EvalTargetRunStatusFail, 2, true},
				{"normal_success_before_projection", 0, entity.TurnRunState_Success, entity.EvalTargetRunStatusSuccess, 2, false},
				{"normal_success_after_projection", 5, entity.TurnRunState_Success, entity.EvalTargetRunStatusSuccess, 2, false},
				{"reused_successful_target", 5, entity.TurnRunState_Processing, entity.EvalTargetRunStatusSuccess, 1, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					runLog := &entity.ExptTurnResultRunLog{
						SpaceID: 100, ExptID: 1, ExptRunID: 2, ItemID: 3, TurnID: 0,
						Status: tc.turnStatus, TargetResultID: 5, LogID: "target-logid",
					}
					record := &entity.EvalTargetRecord{
						ID: 5, SpaceID: 100, ExperimentRunID: tc.targetRunID, ItemID: 3, TurnID: 0,
						Status: gptr.Of(tc.targetStatus), LogID: "target-logid", TraceID: "target-trace", EvalTargetOutputData: &entity.EvalTargetOutputData{},
					}
					if tc.failure {
						runLog.ErrMsg = errno.SerializeErr(errno.NewTargetResultErr("current target failed"))
						record.EvalTargetOutputData.EvalTargetRunError = &entity.EvalTargetRunError{Code: 500, Message: "current target failed"}
					}
					canonicalStatus := entity.TurnRunState_Processing
					if tc.canonicalID > 0 {
						canonicalStatus = tc.turnStatus
					}
					got := readTargetVisibilityFixture(t, baseline, tc.canonicalID, canonicalStatus, []*entity.ExptTurnResultRunLog{runLog}, record)
					require.NotNil(t, got.EvalTargetRecord, "current target diagnostics must not depend on projection timing")
					require.Equal(t, int64(5), got.EvalTargetRecord.ID)
					require.Equal(t, tc.targetRunID, got.EvalTargetRecord.ExperimentRunID)
					require.Equal(t, tc.targetStatus, gptr.Indirect(got.EvalTargetRecord.Status))
					require.Equal(t, "target-logid", got.EvalTargetRecord.LogID)
					require.Equal(t, "target-trace", got.EvalTargetRecord.TraceID)
					require.NotNil(t, got.EvalTargetRecord.EvalTargetOutputData)
					if tc.failure {
						require.Equal(t, &entity.EvalTargetRunError{Code: 500, Message: "current target failed"}, got.EvalTargetRecord.EvalTargetOutputData.EvalTargetRunError)
					} else {
						require.Nil(t, got.EvalTargetRecord.EvalTargetOutputData.EvalTargetRunError)
					}
				})
			}
		})
	}
}

func TestExptTargetVisibility_DoesNotInventOrRestoreOldTarget(t *testing.T) {
	for _, baseline := range []bool{true, false} {
		path := "non_baseline_builder"
		if baseline {
			path = "baseline_MGetExperimentResult"
		}
		t.Run(path, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				logs []*entity.ExptTurnResultRunLog
			}{
				{"no_current_run_log", nil},
				{"nil_run_log", []*entity.ExptTurnResultRunLog{nil}},
				{"new_run_reference_cleared", []*entity.ExptTurnResultRunLog{{SpaceID: 100, ExptID: 1, ExptRunID: 2, ItemID: 3}}},
				{"invalid_target_id", []*entity.ExptTurnResultRunLog{{SpaceID: 100, ExptID: 1, ExptRunID: 2, ItemID: 3, TargetResultID: -1}}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					got := readTargetVisibilityFixture(t, baseline, 0, entity.TurnRunState_Processing, tc.logs, nil)
					require.Nil(t, got.EvalTargetRecord)
				})
			}
		})
	}
}

func readTargetVisibilityFixture(t *testing.T, baseline bool, canonicalTargetID int64, canonicalStatus entity.TurnRunState,
	currentLogs []*entity.ExptTurnResultRunLog, currentRecord *entity.EvalTargetRecord,
) *entity.TurnTargetOutput {
	t.Helper()
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	const spaceID, exptID, runID, itemID, turnID, resultID int64 = 100, 1, 2, 3, 0, 4
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	annotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
	targetSvc := svcMocks.NewMockIEvalTargetService(ctrl)
	setVersionSvc := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	setItemSvc := svcMocks.NewMockEvaluationSetItemService(ctrl)
	evaluatorSvc := svcMocks.NewMockEvaluatorRecordService(ctrl)
	analysisSvc := svcMocks.NewMockIEvaluationAnalysisService(ctrl)
	metric := metricsMocks.NewMockExptMetric(ctrl)
	lwt := lwtMocks.NewMockILatestWriteTracker(ctrl)
	expt := &entity.Experiment{ID: exptID, SpaceID: spaceID, EvalSetID: 10, EvalSetVersionID: 11, ExptType: entity.ExptType_Offline}
	canonical := &entity.ExptTurnResult{
		ID: resultID, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID,
		TurnID: turnID, TargetResultID: canonicalTargetID, Status: int32(canonicalStatus),
	}
	// A previous failed generation exists, but only the requested current generation may be read.
	logsByRun := map[int64][]*entity.ExptTurnResultRunLog{
		1: {{
			SpaceID: spaceID, ExptID: exptID, ExptRunID: 1, ItemID: itemID, TurnID: turnID, TargetResultID: 6,
			Status: entity.TurnRunState_Fail, ErrMsg: errno.SerializeErr(errno.NewTargetResultErr("previous target failed")),
		}},
		runID: currentLogs,
	}
	if baseline || canonicalTargetID == 0 {
		turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).
			DoAndReturn(func(_ context.Context, _, requestedRunID int64, _ []int64, _ int64) ([]*entity.ExptTurnResultRunLog, error) {
				return logsByRun[requestedRunID], nil
			}).MinTimes(1).MaxTimes(2)
	}
	exptRepo.EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(expt, nil)
	turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{resultID}).Return(nil, nil)
	evaluatorSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{}, false, false).Return(nil, nil)
	setVersionSvc.EXPECT().BatchGetEvaluationSetVersions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil()).Return([]*entity.BatchGetEvaluationSetVersionsResult{}, nil).AnyTimes()
	setItemSvc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), &entity.BatchGetEvaluationSetItemsParam{
		SpaceID: spaceID, EvaluationSetID: 10, VersionID: gptr.Of(int64(11)), ItemIDs: []int64{itemID},
	}).Return([]*entity.EvaluationSetItem{{ItemID: itemID, Turns: []*entity.Turn{{ID: turnID, ItemID: itemID}}}}, nil)
	oldRecord := &entity.EvalTargetRecord{
		ID: 6, SpaceID: spaceID, ExperimentRunID: 1, ItemID: itemID, TurnID: turnID,
		Status: gptr.Of(entity.EvalTargetRunStatusFail), EvalTargetOutputData: &entity.EvalTargetOutputData{EvalTargetRunError: &entity.EvalTargetRunError{Code: 500, Message: "previous target failed"}},
	}
	targetSvc.EXPECT().BatchGetRecordByIDs(gomock.Any(), spaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, ids []int64) ([]*entity.EvalTargetRecord, error) {
			var records []*entity.EvalTargetRecord
			for _, id := range ids {
				if currentRecord != nil && id == currentRecord.ID {
					records = append(records, currentRecord)
				}
				if id == oldRecord.ID {
					records = append(records, oldRecord)
				}
			}
			return records, nil
		})
	annotateRepo.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), spaceID, []int64{resultID}).Return(nil, nil)
	annotateRepo.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), spaceID, []int64{}).Return(nil, nil)
	if !baseline {
		turnRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptTurnResult{canonical}, nil)
		builder := &ExptResultBuilder{
			ExptID: exptID, BaselineExptID: 99, SpaceID: spaceID, ItemIDs: []int64{itemID},
			ExperimentRepo: exptRepo, ExptTurnResultRepo: turnRepo, ExptAnnotateRepo: annotateRepo,
			evalTargetService: targetSvc, evaluationSetVersionService: setVersionSvc, evaluationSetItemService: setItemSvc,
			evaluatorRecordService: evaluatorSvc, analysisService: analysisSvc,
		}
		require.NoError(t, builder.build(ctx))
		return builder.getTurnTargetOutput(ctx, itemID, turnID)
	}
	metric.EXPECT().EmitGetExptResult(spaceID, false)
	lwt.EXPECT().CheckWriteFlagByID(gomock.Any(), platestwrite.ResourceTypeExperiment, exptID).Return(false)
	exptRepo.EXPECT().MGetByID(gomock.Any(), []int64{exptID}, spaceID).Return([]*entity.Experiment{expt}, nil)
	exptRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{exptID}, spaceID).Return(nil, nil)
	setVersionSvc.EXPECT().GetEvaluationSetVersion(gomock.Any(), spaceID, int64(11), gptr.Of(true), nil).
		Return(&entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{}}, nil, nil)
	turnRepo.EXPECT().ListTurnResult(gomock.Any(), spaceID, exptID, gomock.Nil(), entity.Page{}, false).Return([]*entity.ExptTurnResult{canonical}, int64(1), nil)
	itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return([]*entity.ExptItemResult{{SpaceID: spaceID, ExptID: exptID, ItemID: itemID, Status: entity.ItemRunState_Processing}}, nil).Times(2)
	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), exptID, runID, []int64{itemID}, spaceID).Return(nil, nil)
	annotateRepo.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), []int64{exptID}, spaceID).Return(nil, nil)
	analysisSvc.EXPECT().BatchGetAnalysisRecordByUniqueKeys(gomock.Any(), []string{"100_1_3_0"}).Return(map[string]*entity.AnalysisRecord{}, nil)
	svc := ExptResultServiceImpl{
		ExptTurnResultRepo: turnRepo, ExperimentRepo: exptRepo, ExptItemResultRepo: itemRepo,
		ExptAnnotateRepo: annotateRepo, evalTargetService: targetSvc, evaluationSetVersionService: setVersionSvc,
		evaluationSetItemService: setItemSvc, evaluatorRecordService: evaluatorSvc, analysisService: analysisSvc, Metric: metric, lwt: lwt,
	}
	got, err := svc.MGetExperimentResult(ctx, &entity.MGetExperimentResultParam{SpaceID: spaceID, ExptIDs: []int64{exptID}, BaseExptID: gptr.Of(exptID)})
	require.NoError(t, err)
	require.Len(t, got.ItemResults, 1)
	require.Len(t, got.ItemResults[0].TurnResults, 1)
	require.Len(t, got.ItemResults[0].TurnResults[0].ExperimentResults, 1)
	payload := got.ItemResults[0].TurnResults[0].ExperimentResults[0].Payload
	require.NotNil(t, payload)
	require.NotNil(t, payload.TargetOutput)
	return payload.TargetOutput
}
