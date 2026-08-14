// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	exptdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

const (
	standardEvalOutputExptCreateUnix = int64(1_700_000_000)
	standardEvalOutputItemEndUnix    = int64(1_700_000_100)
	standardEvalOutputCreatedBy      = "creator-1"
)

func TestExperimentApplication_MGetExperimentStandardEvalOutputs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	mockTargetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc, evalTargetService: mockTargetSvc, manager: mockManager}

	const (
		workspaceID    int64 = 1
		exptID         int64 = 2
		exptRunID      int64 = 3
		itemID         int64 = 4
		turnID         int64 = 5
		targetRecordID int64 = 6
	)

	mockManager.EXPECT().GetDetail(gomock.Any(), exptID, workspaceID, gomock.Any()).Return(makeStandardEvalOutputExpt(exptID, workspaceID), nil)
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockTargetSvc.EXPECT().GetEvalTarget(gomock.Any(), int64(200)).Return(&entity.EvalTarget{ID: 200, SpaceID: workspaceID, SourceTargetID: "src-200"}, nil)

	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error) {
			assert.Equal(t, workspaceID, param.SpaceID)
			assert.Equal(t, []int64{exptID}, param.ExptIDs)
			require.NotNil(t, param.BaseExptID)
			assert.Equal(t, exptID, *param.BaseExptID)
			assert.False(t, param.UseAccelerator)
			assert.Equal(t, []int64{itemID}, param.ItemIDs)
			assert.Empty(t, param.LoadEvalTargetOutputFieldKeys)
			assert.True(t, param.FullTrajectory)
			require.NotNil(t, param.LoadEvaluatorFullContent)
			assert.True(t, *param.LoadEvaluatorFullContent)
			require.NotNil(t, param.LoadEvalTargetFullContent)
			assert.True(t, *param.LoadEvalTargetFullContent)
			return makeStandardEvalOutputReportResult(exptID, exptRunID, itemID, turnID, targetRecordID), nil
		},
	)

	resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{
		WorkspaceID: workspaceID,
		ExptID:      exptID,
		ItemIds:     []int64{itemID},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetItems(), 1)
	assert.Equal(t, exptdomain.ItemRunState_Success, resp.GetItems()[0].GetStatus())
	require.NotNil(t, resp)
	require.Len(t, resp.Items, 1)
	got := resp.Items[0]
	assert.Equal(t, exptID, got.GetExptID())
	assert.Equal(t, itemID, got.GetItemID())
	assert.Equal(t, "dataset-1", got.GetDatasetKey())
	require.NotNil(t, got.Output)
	require.NotNil(t, got.Eval)
	assert.False(t, got.Output.GetContentOmitted())

	var output map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetOutput().GetText()), &output))
	assert.Contains(t, output, "detail")
	assert.NotContains(t, output, "rounds") // 平台兜底只补 detail

	var eval map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetEval().GetText()), &eval))
	assert.Contains(t, eval, "task_config")
	assert.Contains(t, eval, "detail")
	assert.NotContains(t, eval, "rounds") // 平台兜底只补 detail

	require.NotNil(t, got.Agent)
	var agent map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetAgent().GetText()), &agent))
	// agent 不再回填 runs / target_id / target_version_id / source_target_id（顶层 MQ meta 已有）。
	_, hasRuns := agent["runs"]
	assert.False(t, hasRuns)
	_, hasTargetID := agent["target_id"]
	assert.False(t, hasTargetID)
	_, hasSrcTargetID := agent["source_target_id"]
	assert.False(t, hasSrcTargetID)

	// MQ 元信息顶层字段（与 item-complete MQ 对齐）。
	assert.Equal(t, workspaceID, got.GetExptWorkspaceID())
	assert.Equal(t, exptRunID, got.GetExptRunID())
	assert.Equal(t, "group-key-1", got.GetExperimentGroupKey())
	assert.Equal(t, int64(200), got.GetEvalTargetID())
	assert.Equal(t, workspaceID, got.GetEvalTargetWorkspaceID())
	assert.Equal(t, "src-200", got.GetSourceTargetID())
	assert.Equal(t, int64(100), got.GetDatasetID())
	assert.Equal(t, workspaceID, got.GetDatasetWorkspaceID())
	assert.Equal(t, int64(1001), got.GetDatasetVersionID())
	assert.Equal(t, "1.2.0", got.GetDatasetVersionName())
	assert.Equal(t, standardEvalOutputExptCreateUnix, got.GetExperimentCreateTime())
	assert.Equal(t, standardEvalOutputCreatedBy, got.GetCreatedBy())
	assert.Equal(t, standardEvalOutputItemEndUnix, got.GetItemEndTime())
}

func TestBuildItemStandardEvalOutput_ProcessingOnlyReturnsMetadata(t *testing.T) {
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	item.SystemInfo.RunState = entity.ItemRunState_Processing

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(20), got.GetExptID())
	assert.Equal(t, int64(10), got.GetItemID())
	assert.Equal(t, "dataset-1", got.GetDatasetKey())
	assert.Equal(t, "case-1", got.GetItemKey())
	assert.Equal(t, exptdomain.ItemRunState_Processing, got.GetStatus())
	assert.Nil(t, got.Detail)
	assert.Nil(t, got.Rounds)
	assert.Nil(t, got.Agent)
	assert.Nil(t, got.Output)
	assert.Nil(t, got.Eval)
	assert.Nil(t, got.Extra)
}

func TestBuildItemStandardEvalOutput_FailOnlyReturnsMetadata(t *testing.T) {
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	item.SystemInfo.RunState = entity.ItemRunState_Fail

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	assert.Equal(t, exptdomain.ItemRunState_Fail, got.GetStatus())
	assert.Nil(t, got.Detail)
	assert.Nil(t, got.Rounds)
	assert.Nil(t, got.Agent)
	assert.Nil(t, got.Output)
	assert.Nil(t, got.Eval)
	assert.Nil(t, got.Extra)
}

func TestBuildItemStandardEvalOutput_ItemEndTime(t *testing.T) {
	endTime := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name            string
		runState        entity.ItemRunState
		endTime         *time.Time
		wantItemEndTime bool
	}{
		{name: "unknown does not fill", runState: entity.ItemRunState_Unknown, endTime: &endTime},
		{name: "queueing does not fill", runState: entity.ItemRunState_Queueing, endTime: &endTime},
		{name: "processing does not fill", runState: entity.ItemRunState_Processing, endTime: &endTime},
		{name: "success fills", runState: entity.ItemRunState_Success, endTime: &endTime, wantItemEndTime: true},
		{name: "fail fills", runState: entity.ItemRunState_Fail, endTime: &endTime, wantItemEndTime: true},
		{name: "terminal fills", runState: entity.ItemRunState_Terminal, endTime: &endTime, wantItemEndTime: true},
		{name: "nil end time does not fill", runState: entity.ItemRunState_Success},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
			item.SystemInfo.RunState = tt.runState
			item.SystemInfo.EndTime = tt.endTime

			got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
			require.NoError(t, err)
			if !tt.wantItemEndTime {
				assert.Nil(t, got.ItemEndTime)
				return
			}
			require.NotNil(t, got.ItemEndTime)
			assert.Equal(t, endTime.Unix(), got.GetItemEndTime())
		})
	}
}

func TestExperimentApplication_MGetExperimentStandardEvalOutputs_ItemIDsLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Times(0)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Times(0)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc}

	itemIDs := make([]int64, maxStandardEvalOutputMGetItemIDs+1)
	for i := range itemIDs {
		itemIDs[i] = int64(i + 1)
	}
	_, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{WorkspaceID: 1, ExptID: 2, ItemIds: itemIDs})
	require.Error(t, err)
}

func TestExperimentApplication_MGetExperimentStandardEvalOutputs_Auth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 已恢复真鉴权：走 e.auth.Authorization（外部 caller 由 auth_whitelist 放行）。
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Return(makeStandardEvalOutputReportResult(2, 3, 4, 5, 6), nil)
	mockTargetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	mockTargetSvc.EXPECT().GetEvalTarget(gomock.Any(), int64(200)).Return(&entity.EvalTarget{ID: 200, SpaceID: 1, SourceTargetID: "src-200"}, nil)
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockManager.EXPECT().GetDetail(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(makeStandardEvalOutputExpt(2, 1), nil)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc, evalTargetService: mockTargetSvc, manager: mockManager}

	resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{
		WorkspaceID: 1,
		ExptID:      2,
		ItemIds:     []int64{4},
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
}

func TestExperimentApplication_MGetExperimentStandardEvalOutputs_ProcessingOmitsItemEndTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	result := makeStandardEvalOutputReportResult(2, 3, 4, 5, 6)
	result.ItemResults[0].SystemInfo.RunState = entity.ItemRunState_Processing
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Return(result, nil)
	mockTargetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	mockTargetSvc.EXPECT().GetEvalTarget(gomock.Any(), int64(200)).Return(&entity.EvalTarget{ID: 200, SpaceID: 1, SourceTargetID: "src-200"}, nil)
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockManager.EXPECT().GetDetail(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(makeStandardEvalOutputExpt(2, 1), nil)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc, evalTargetService: mockTargetSvc, manager: mockManager}

	resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{
		WorkspaceID: 1,
		ExptID:      2,
		ItemIds:     []int64{4},
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, exptdomain.ItemRunState_Processing, resp.Items[0].GetStatus())
	assert.Nil(t, resp.Items[0].ItemEndTime)
}

func TestExperimentApplication_ListExperimentStandardEvalOutputs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	mockTargetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
	mockTargetSvc.EXPECT().GetEvalTarget(gomock.Any(), int64(200)).Return(&entity.EvalTarget{ID: 200, SpaceID: 1, SourceTargetID: "src-200"}, nil)
	mockManager := servicemocks.NewMockIExptManager(ctrl)
	mockManager.EXPECT().GetDetail(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(makeStandardEvalOutputExpt(2, 1), nil)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc, evalTargetService: mockTargetSvc, manager: mockManager}

	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error) {
			assert.Equal(t, entity.NewPage(2, 10), param.Page)
			assert.True(t, param.UseAccelerator)
			assert.Empty(t, param.LoadEvalTargetOutputFieldKeys)
			assert.True(t, param.FullTrajectory)
			require.NotNil(t, param.LoadEvaluatorFullContent)
			assert.True(t, *param.LoadEvaluatorFullContent)
			require.NotNil(t, param.LoadEvalTargetFullContent)
			assert.True(t, *param.LoadEvalTargetFullContent)
			return makeStandardEvalOutputReportResult(2, 3, 4, 5, 6), nil
		},
	)

	resp, err := app.ListExperimentStandardEvalOutputs(context.Background(), &exptpb.ListExperimentStandardEvalOutputsRequest{
		WorkspaceID: 1,
		ExptID:      2,
		PageNumber:  gptr.Of(int32(2)),
		PageSize:    gptr.Of(int32(10)),
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.NotNil(t, resp.Total)
	assert.Equal(t, int64(1), *resp.Total)
	got := resp.Items[0]
	assert.Equal(t, standardEvalOutputExptCreateUnix, got.GetExperimentCreateTime())
	assert.Equal(t, standardEvalOutputCreatedBy, got.GetCreatedBy())
	assert.Equal(t, standardEvalOutputItemEndUnix, got.GetItemEndTime())
}

func TestExperimentApplication_ListExperimentStandardEvalOutputs_OnlyItemIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	// 精简模式：只调 GetItemIDListByExptID（参数顺序 exptID, spaceID），不走重的 MGetExperimentResult。
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Times(0)
	mockResultSvc.EXPECT().GetItemIDListByExptID(gomock.Any(), int64(2), int64(1)).Return([]int64{11, 22, 33}, nil)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc}

	resp, err := app.ListExperimentStandardEvalOutputs(context.Background(), &exptpb.ListExperimentStandardEvalOutputsRequest{
		WorkspaceID: 1,
		ExptID:      2,
		ItemIDOnly:  gptr.Of(true),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetItems(), 3)
	gotIDs := make([]int64, 0, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		assert.Equal(t, int64(2), it.GetExptID())
		// 精简模式仅填 item_id，其余内容块 / dataset_key 均为空。
		assert.Empty(t, it.GetDatasetKey())
		assert.Nil(t, it.Detail)
		gotIDs = append(gotIDs, it.GetItemID())
	}
	assert.Equal(t, []int64{11, 22, 33}, gotIDs)
	assert.Equal(t, int64(3), resp.GetTotal())
}

func TestExperimentApplication_MGetExperimentStandardEvalOutputs_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc}

	_, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{WorkspaceID: 1, ExptID: 2})
	require.Error(t, err)

	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))
	_, err = app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{WorkspaceID: 1, ExptID: 2, ItemIds: []int64{4}})
	require.Error(t, err)
}

func makeStandardEvalOutputReportResult(exptID, exptRunID, itemID, turnID, targetRecordID int64) *entity.MGetExperimentReportResult {
	textType := entity.ContentTypeText
	answer := "world"
	question := "hello"
	score := 0.8
	latency := int64(123)
	turnIndex := int64(0)
	itemEndTime := time.Unix(standardEvalOutputItemEndUnix, 0)
	return &entity.MGetExperimentReportResult{
		Total: 1,
		ItemResults: []*entity.ItemResult{{
			ItemID:    itemID,
			ItemIndex: gptr.Of(turnIndex),
			Ext:       map[string]string{"dataset_key": "dataset-1", "item_key": "case-1"},
			SystemInfo: &entity.ItemSystemInfo{
				RunState: entity.ItemRunState_Success,
				EndTime:  &itemEndTime,
			},
			TurnResults: []*entity.TurnResult{{
				TurnID:    turnID,
				TurnIndex: gptr.Of(turnIndex),
				ExperimentResults: []*entity.ExperimentResult{{
					ExperimentID: exptID,
					Payload: &entity.ExperimentTurnPayload{
						TurnID: turnID,
						EvalSet: &entity.TurnEvalSet{
							ItemID:     itemID,
							EvalSetID:  100,
							DatasetKey: "dataset-from-data",
							ItemKey:    "case-from-data",
							Turn: &entity.Turn{ID: turnID, ItemID: itemID, FieldDataList: []*entity.FieldData{{
								Key: "question", Name: "Question", Content: &entity.Content{ContentType: &textType, Text: &question},
							}}},
						},
						TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{
							ID:              targetRecordID,
							TargetID:        200,
							TargetVersionID: 300,
							ExperimentRunID: exptRunID,
							ItemID:          itemID,
							TurnID:          turnID,
							TraceID:         "trace-1",
							LogID:           "log-1",
							EvalTargetInputData: &entity.EvalTargetInputData{Ext: map[string]string{
								consts.TargetExecuteExtRuntimeParamKey: `{"model":"x"}`,
							}},
							EvalTargetOutputData: &entity.EvalTargetOutputData{
								OutputFields:    map[string]*entity.Content{"actual_output": {ContentType: &textType, Text: &answer}},
								Ext:             map[string]string{"ext_key": "ext_val"},
								EvalTargetUsage: &entity.EvalTargetUsage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
								TimeConsumingMS: &latency,
							},
						}},
						EvaluatorOutput: &entity.TurnEvaluatorOutput{
							WeightedScore: &score,
							EvaluatorRecords: map[int64]*entity.EvaluatorRecord{101: {
								ID:                  1001,
								ExperimentID:        exptID,
								ExperimentRunID:     exptRunID,
								ItemID:              itemID,
								TurnID:              turnID,
								EvaluatorVersionID:  101,
								EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: &score, Reasoning: "good"}},
							}},
						},
						SystemInfo: &entity.TurnSystemInfo{TurnRunState: entity.TurnRunState_Success, LogID: gptr.Of("turn-log-1")},
					},
				}},
			}},
		}},
	}
}

// makeStandardEvalOutputExpt 构造标准输出 MQ 元信息测试用的实验详情，
// 主评测集 id=100（与 makeStandardEvalOutputReportResult 的 payload EvalSetID 对齐），target id=200。
func makeStandardEvalOutputExpt(exptID, spaceID int64) *entity.Experiment {
	createdAt := time.Unix(standardEvalOutputExptCreateUnix, 0)
	return &entity.Experiment{
		ID:                 exptID,
		SpaceID:            spaceID,
		CreatedBy:          standardEvalOutputCreatedBy,
		CreatedAt:          &createdAt,
		LatestRunID:        3,
		ExperimentGroupKey: "group-key-1",
		TargetID:           200,
		EvalSetID:          100,
		EvalSetSourceType:  entity.ExptEvalSetSourceType_SingleSet,
		Target:             &entity.EvalTarget{ID: 200, SpaceID: spaceID, SourceTargetID: "src-200"},
		EvalSet: &entity.EvaluationSet{
			ID:      100,
			SpaceID: spaceID,
			EvaluationSetVersion: &entity.EvaluationSetVersion{
				ID:      1001,
				Version: "1.2.0",
			},
		},
	}
}

func TestBuildItemStandardEvalOutput_FillsKeysFromEvalSetWhenExtMissing(t *testing.T) {
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	item.Ext = nil

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	assert.Equal(t, "dataset-from-data", got.GetDatasetKey())
	assert.Equal(t, "case-from-data", got.GetItemKey())

	var eval map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetEval().GetText()), &eval))
	taskConfig, ok := eval["task_config"].(map[string]any)
	require.True(t, ok)
	items, ok := taskConfig["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	entry, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "dataset-from-data", entry["dataset_key"])
	assert.Equal(t, "case-from-data", entry["item_key"])
}

func TestBuildItemStandardEvalOutput_ExtKeysTakePrecedence(t *testing.T) {
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	item.Ext = map[string]string{"dataset_key": "dataset-from-ext", "item_key": "case-from-ext"}

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	assert.Equal(t, "dataset-from-ext", got.GetDatasetKey())
	assert.Equal(t, "case-from-ext", got.GetItemKey())
}

func TestBuildItemStandardEvalOutput_ParseReportedStandardEvalOutput(t *testing.T) {
	textType := entity.ContentTypeText
	reported := `{"detail_id":"sandbox-detail","source":"fornax","rounds":[{"round_no":1}],"agent":{"agent_name":"codex"},"output":{"detail":{"file_diff":[]}},"eval":{"score":1},"extra":{}}`
	item := &entity.ItemResult{
		ItemID: 10,
		Ext:    map[string]string{"dataset_key": "dataset-1", "item_key": "case-10"},
		SystemInfo: &entity.ItemSystemInfo{
			RunState: entity.ItemRunState_Success,
		},
		TurnResults: []*entity.TurnResult{{ExperimentResults: []*entity.ExperimentResult{{
			ExperimentID: 20,
			Payload: &entity.ExperimentTurnPayload{
				TurnID: 1,
				TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{
					ExperimentRunID: 30,
					EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{
						consts.EvalTargetOutputFieldKeyActualOutput: {ContentType: &textType, Text: &reported},
					}},
				}},
			},
		}}}},
	}

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	assert.Equal(t, "case-10", got.GetItemKey())
	assert.Equal(t, "dataset-1", got.GetDatasetKey())
	require.NotNil(t, got.Agent)
	assert.Contains(t, got.GetAgent().GetText(), "codex")
}

func TestBuildItemStandardEvalOutput_ParseReportedStandardEvalOutputFields(t *testing.T) {
	textType := entity.ContentTypeText
	source := `{"type":"fornax"}`
	rounds := `[{"round_no":1}]`
	agent := `{"agent_name":"codex"}`
	output := `{"detail":{"file_diff":[]}}`
	eval := `{"score":1}`
	extra := `{"sandbox_log":"https://example.com/log"}`
	item := &entity.ItemResult{
		ItemID: 10,
		Ext:    map[string]string{"dataset_key": "dataset-1", "item_key": "case-10"},
		SystemInfo: &entity.ItemSystemInfo{
			RunState: entity.ItemRunState_Success,
		},
		TurnResults: []*entity.TurnResult{{ExperimentResults: []*entity.ExperimentResult{{
			ExperimentID: 20,
			Payload: &entity.ExperimentTurnPayload{
				TurnID: 1,
				TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{
					ExperimentRunID: 30,
					EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{
						"source": {ContentType: &textType, Text: &source},
						"rounds": {ContentType: &textType, Text: &rounds},
						"agent":  {ContentType: &textType, Text: &agent},
						"output": {ContentType: &textType, Text: &output},
						"eval":   {ContentType: &textType, Text: &eval},
						"extra":  {ContentType: &textType, Text: &extra},
					}},
				}},
			},
		}}}},
	}

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	assert.Equal(t, "case-10", got.GetItemKey())
	assert.Equal(t, "dataset-1", got.GetDatasetKey())
	require.NotNil(t, got.Agent)
	assert.Contains(t, got.GetAgent().GetText(), "codex")
	require.NotNil(t, got.Output)
	assert.Contains(t, got.GetOutput().GetText(), "file_diff")
	require.NotNil(t, got.Eval)
	assert.Contains(t, got.GetEval().GetText(), "score")
}

func TestBuildItemStandardEvalOutput_DoesNotMisclassifyOrdinaryJSONActualOutput(t *testing.T) {
	textType := entity.ContentTypeText
	reported := `{"output":"ordinary json"}`
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	item.TurnResults[0].ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields[consts.EvalTargetOutputFieldKeyActualOutput] = &entity.Content{ContentType: &textType, Text: &reported}

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	require.NotNil(t, got.Output)
	assert.Contains(t, got.Output.GetText(), "actual_output")
	assert.Contains(t, got.Output.GetText(), "detail")
}

// --- FORNAX_ 前缀逐字段深合并 ---

// injectFornaxField 在 item 首个 payload 的 target OutputFields 写入一个字段（key 原样，调用方决定是否带 FORNAX_ 前缀）。
func injectFornaxField(item *entity.ItemResult, key, jsonText string) {
	textType := entity.ContentTypeText
	fields := item.TurnResults[0].ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
	txt := jsonText
	fields[key] = &entity.Content{ContentType: &textType, Text: &txt}
}

func TestBuildItemStandardEvalOutput_NoFornaxFields_AllPlatform(t *testing.T) {
	// 对象未报任何 FORNAX_ 字段 → 七字段全平台兜底（等价现状）。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	require.NotNil(t, got.Eval)
	var eval map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetEval().GetText()), &eval))
	assert.Contains(t, eval, "task_config") // 平台兜底结构特征
}

func TestBuildItemStandardEvalOutput_FornaxEvalOnly_OthersPlatform(t *testing.T) {
	// 对象只报 FORNAX_eval，其余平台兜底。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	injectFornaxField(item, "FORNAX_eval", `{"detail":{"eval_result":{"score":0.99}}}`)

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	// eval 深合并：对象的 score 覆盖，平台的 task_config 保留。
	var eval map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetEval().GetText()), &eval))
	assert.Contains(t, eval, "task_config")
	detail := eval["detail"].(map[string]any)
	evalResult := detail["eval_result"].(map[string]any)
	assert.EqualValues(t, 0.99, evalResult["score"])
	// output 仍是平台兜底。
	var output map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetOutput().GetText()), &output))
	assert.Contains(t, output, "detail")
}

func TestBuildItemStandardEvalOutput_SubfieldConflictObjectWins(t *testing.T) {
	// 子字段冲突：对象 output.detail.custom 覆盖，平台已有的 detail 兄弟子字段保留。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	injectFornaxField(item, "FORNAX_output", `{"detail":{"custom":"from-object"}}`)

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	var output map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetOutput().GetText()), &output))
	detail := output["detail"].(map[string]any)
	assert.Equal(t, "from-object", detail["custom"]) // 对象子字段
	// 平台 detail 里原有的 output 兄弟键仍在（深合并保留平台未冲突子字段）。
	assert.Contains(t, detail, "output")
	// 平台不再补 file_diff（空不回填）、不补 output.rounds。
	assert.NotContains(t, detail, "file_diff")
	assert.NotContains(t, output, "rounds")
}

func TestBuildItemStandardEvalOutput_ObjectRoundsWinsWholesale(t *testing.T) {
	// rounds 语义：对象上报了 rounds → 整体采用对象的，平台不合并、不补轮次。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	injectFornaxField(item, "FORNAX_rounds", `[{"round_id":"r-obj","extra_note":"obj"}]`)

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	var rounds []any
	require.NoError(t, json.Unmarshal([]byte(got.GetRounds().GetText()), &rounds))
	require.Len(t, rounds, 1)
	r0 := rounds[0].(map[string]any)
	assert.Equal(t, "r-obj", r0["round_id"])
	assert.Equal(t, "obj", r0["extra_note"])
	assert.NotContains(t, r0, "round_no") // 平台字段不再混入(对象整体采用)
}

func TestBuildItemStandardEvalOutput_PlatformRoundIDIsTurnID(t *testing.T) {
	// 对象未报 rounds → 平台补,每轮 round_id = TurnID(此处 turnID=1)。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	var rounds []any
	require.NoError(t, json.Unmarshal([]byte(got.GetRounds().GetText()), &rounds))
	require.Len(t, rounds, 1)
	r0 := rounds[0].(map[string]any)
	assert.Equal(t, "1", r0["round_id"]) // round_id = TurnID,不再是 round_0/round_1
}

func TestBuildItemStandardEvalOutput_BareKeyFallback(t *testing.T) {
	// 向前兼容：对象用旧裸 key output 上报，仍被识别复用。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	injectFornaxField(item, "output", `{"detail":{"custom":"bare-key"}}`)

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	var output map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetOutput().GetText()), &output))
	detail := output["detail"].(map[string]any)
	assert.Equal(t, "bare-key", detail["custom"])
}

func TestBuildItemStandardEvalOutput_FornaxPrefixOverBareKey(t *testing.T) {
	// FORNAX_ 前缀优先于裸 key。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	injectFornaxField(item, "output", `{"detail":{"src":"bare"}}`)
	injectFornaxField(item, "FORNAX_output", `{"detail":{"src":"prefixed"}}`)

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	var output map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetOutput().GetText()), &output))
	detail := output["detail"].(map[string]any)
	assert.Equal(t, "prefixed", detail["src"])
}

func TestBuildItemStandardEvalOutput_ContentOmittedNotMerged(t *testing.T) {
	// 内容省略的大对象（S3）→ 不深合并，原样透出并保留省略语义。
	textType := entity.ContentTypeText
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	preview := `{"detail":{"partial":true}}` // 预览片段（合法 JSON 也不合并）
	fields := item.TurnResults[0].ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
	fields["FORNAX_output"] = &entity.Content{
		ContentType:    &textType,
		Text:           &preview,
		ContentOmitted: gptr.Of(true),
		FullContent:    &entity.ObjectStorage{URI: gptr.Of("eval:record:field:uuid-1")},
	}

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	require.NotNil(t, got.Output)
	assert.True(t, got.Output.GetContentOmitted())
	require.NotNil(t, got.Output.FullContent)
	assert.Equal(t, "eval:record:field:uuid-1", got.Output.FullContent.GetURI())
	// 原样透出预览文本，不与平台合并（不含平台 rounds 键）。
	assert.Equal(t, preview, got.Output.GetText())
}

func TestBuildItemStandardEvalOutput_NonJSONFornaxFieldNoPanic(t *testing.T) {
	// 对象 FORNAX_output.text 非合法 JSON → 原样透出，不 panic。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	injectFornaxField(item, "FORNAX_output", `this is not json`)

	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	require.NotNil(t, got.Output)
	assert.Equal(t, "this is not json", got.Output.GetText())
}

func TestBuildItemStandardEvalOutput_EvaluatorIDFilled(t *testing.T) {
	// 平台兜底的 eval.detail.eval_result.results.<key> 含 evaluator_id 与 evaluator_version_id。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	opt := standardEvalOutputBuildOptions{
		ExptID: 20,
		EvaluatorByVersionID: map[int64]*entity.ColumnEvaluator{
			101: {EvaluatorVersionID: 101, EvaluatorID: 9001, Name: gptr.Of("完整性"), Version: gptr.Of("0.0.1")},
		},
	}
	got, err := buildItemStandardEvalOutput(context.Background(), item, opt)
	require.NoError(t, err)
	var eval map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetEval().GetText()), &eval))
	// 平台兜底只补 detail、不补 round 粒度；evaluator_id/version_id 在 detail.eval_result.results。
	assert.NotContains(t, eval, "rounds")
	detail := eval["detail"].(map[string]any)
	evalResult := detail["eval_result"].(map[string]any)
	results := evalResult["results"].(map[string]any)
	require.NotEmpty(t, results)
	for _, resv := range results {
		res := resv.(map[string]any)
		assert.Equal(t, "9001", res["evaluator_id"])        // i64 已 string 化
		assert.Equal(t, "101", res["evaluator_version_id"]) // i64 已 string 化
		assert.Equal(t, "完整性", res["evaluator_name"])
	}
}

func TestBuildItemStandardEvalOutput_AgentOmitsEmptyKeys(t *testing.T) {
	// agent 中无值字段不填 key（不出现空串占位）。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	// runtime_param 为 {"model":"x"}，故 model_name 有值；agent_name/thinking_effort 等无值应缺席。
	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	var agent map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetAgent().GetText()), &agent))
	assert.Equal(t, "x", agent["model_name"])
	_, hasAgentName := agent["agent_name"]
	assert.False(t, hasAgentName, "空值的 agent_name 不应出现")
	_, hasEffort := agent["thinking_effort"]
	assert.False(t, hasEffort, "空值的 thinking_effort 不应出现")
}

func TestDeepMergeStandardEvalOutput_MapRecursiveObjectWins(t *testing.T) {
	platform := map[string]any{"a": 1.0, "nested": map[string]any{"x": "p", "y": "p"}}
	object := map[string]any{"b": 2.0, "nested": map[string]any{"y": "o"}}
	merged := deepMergeStandardEvalOutput(platform, object).(map[string]any)
	assert.EqualValues(t, 1, merged["a"])
	assert.EqualValues(t, 2, merged["b"])
	nested := merged["nested"].(map[string]any)
	assert.Equal(t, "p", nested["x"]) // 平台独有保留
	assert.Equal(t, "o", nested["y"]) // 冲突对象覆盖
}

func TestDeepMergeStandardEvalOutput_ArrayObjectOverrides(t *testing.T) {
	// 非 rounds 的普通数组：对象整体覆盖平台。
	merged := deepMergeStandardEvalOutput([]any{1.0, 2.0}, []any{9.0})
	arr := merged.([]any)
	require.Len(t, arr, 1)
	assert.EqualValues(t, 9, arr[0])
}

func TestEvaluatorResultKey_NoCollision(t *testing.T) {
	// result_key = name:version:alias(从 ColumnEvaluator 反查);同评估器多 alias 不撞;
	// 反查不到(老数据/inline)时退化 versionID(+inlineKey)兜底。
	opt := standardEvalOutputBuildOptions{
		EvaluatorByVersionID: map[int64]*entity.ColumnEvaluator{
			101: {EvaluatorVersionID: 101, EvaluatorID: 9001, Name: gptr.Of("完整性"), Version: gptr.Of("1.0.0")},
		},
	}
	cases := []struct {
		name   string
		key    int64
		record *entity.EvaluatorRecord
		want   string
	}{
		{"主键 name:version:空别名", 101, &entity.EvaluatorRecord{EvaluatorVersionID: 101}, "完整性:1.0.0:"},
		{"同评估器 alias A", 101, &entity.EvaluatorRecord{EvaluatorVersionID: 101, Alias: "judge_A"}, "完整性:1.0.0:judge_A"},
		{"同评估器 alias B", 101, &entity.EvaluatorRecord{EvaluatorVersionID: 101, Alias: "judge_B"}, "完整性:1.0.0:judge_B"},
		{"反查不到-退化 versionID", 202, &entity.EvaluatorRecord{EvaluatorVersionID: 202}, "202"},
		{"inline 退化 versionID#inlineKey", 0, &entity.EvaluatorRecord{EvaluatorVersionID: 0, InlineKey: "ik1"}, "0#ik1"},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		got := evaluatorResultKey(opt, c.key, c.record)
		assert.Equal(t, c.want, got, c.name)
		assert.False(t, seen[got], "result_key 撞了: %s (%s)", got, c.name)
		seen[got] = true
	}
}

func TestBuildItemStandardEvalOutput_SnowflakeI64StringifiedNoPrecisionLoss(t *testing.T) {
	// i64 精度铁律：agent_id / evaluator_id / evaluator_version_id / source.item_id / source.expt_id
	// 在 inline JSON 里都必须是 string，雪花大 id 不被 JSON number 抹掉尾部精度。
	const (
		bigExptID      int64 = 7590093945404251906
		bigItemID      int64 = 7590093945404251907
		bigTargetID    int64 = 7590093945404251908
		bigVersionID   int64 = 7590093945404251909
		bigEvaluatorID int64 = 7590093945404251910
	)
	item := makeStandardEvalOutputReportResult(bigExptID, 30, bigItemID, 1, 100).ItemResults[0]
	// 把评估器 record 的 version_id 换成大雪花 id（原 fixture key=101）。
	oldRec := item.TurnResults[0].ExperimentResults[0].Payload.EvaluatorOutput.EvaluatorRecords[101]
	oldRec.EvaluatorVersionID = bigVersionID
	delete(item.TurnResults[0].ExperimentResults[0].Payload.EvaluatorOutput.EvaluatorRecords, 101)
	item.TurnResults[0].ExperimentResults[0].Payload.EvaluatorOutput.EvaluatorRecords[bigVersionID] = oldRec
	// 把 target_id 换成大雪花 id。
	item.TurnResults[0].ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.TargetID = bigTargetID

	opt := standardEvalOutputBuildOptions{
		ExptID: bigExptID,
		EvaluatorByVersionID: map[int64]*entity.ColumnEvaluator{
			bigVersionID: {EvaluatorVersionID: bigVersionID, EvaluatorID: bigEvaluatorID, Name: gptr.Of("n"), Version: gptr.Of("v")},
		},
	}
	got, err := buildItemStandardEvalOutput(context.Background(), item, opt)
	require.NoError(t, err)

	// 断言 raw JSON 文本里对应字段是带引号的 string（雪花值完整、无精度丢失）。
	// json.Number 反序列化会引入 float64 精度问题，故直接在原始文本上核对。
	agentText := got.GetAgent().GetText()
	assert.Contains(t, agentText, `"agent_id":"7590093945404251908"`, "agent_id 必须 string 化且值完整: %s", agentText)

	evalText := got.GetEval().GetText()
	assert.Contains(t, evalText, `"evaluator_id":"7590093945404251910"`, "evaluator_id 必须 string 化且值完整: %s", evalText)
	assert.Contains(t, evalText, `"evaluator_version_id":"7590093945404251909"`, "evaluator_version_id 必须 string 化且值完整: %s", evalText)

	// source 由平台生成（不 emit 到顶层 StandardEvalOutputContent 字段），改从 buildStandardEvalOutputJSON 侧核对：
	// source.expt_id / source.item_id、detail.item_id 都走 int64String，须是 string。
	std := buildStandardEvalOutputJSON(item, opt)
	srcText, err := json.MarshalString(std.Source)
	require.NoError(t, err)
	assert.Contains(t, srcText, `"expt_id":"7590093945404251906"`, "source.expt_id 必须 string 化: %s", srcText)
	assert.Contains(t, srcText, `"item_id":"7590093945404251907"`, "source.item_id 必须 string 化: %s", srcText)

	detailText, err := json.MarshalString(std.Detail)
	require.NoError(t, err)
	assert.Contains(t, detailText, `"item_id":"7590093945404251907"`, "detail.item_id 必须 string 化: %s", detailText)
}

func TestEvaluatorResultKey_MetaPresentButNameEmpty_DegradesToVersionID(t *testing.T) {
	// name 反查到 ColumnEvaluator 但 Name 为空 → 退化用 versionID(+inlineKey) 兜底，不产出 ":version:" 空 name 前缀。
	opt := standardEvalOutputBuildOptions{
		EvaluatorByVersionID: map[int64]*entity.ColumnEvaluator{
			// meta 存在，但 Name 为 nil（空）、Version 有值。
			303: {EvaluatorVersionID: 303, Version: gptr.Of("2.0.0")},
		},
	}
	// meta 命中但 name 空 → 走兜底分支 EncodeEvaluatorInstanceKey(key, alias)。
	got := evaluatorResultKey(opt, 303, &entity.EvaluatorRecord{EvaluatorVersionID: 303, Alias: "a1"})
	wantFallback := entity.EncodeEvaluatorInstanceKey(303, "a1")
	assert.Equal(t, wantFallback, got)
	// 不能以裸冒号 name 前缀开头（即不是 ":2.0.0:a1" 这种空 name 形态）。
	assert.NotEqual(t, ":2.0.0:a1", got)

	// name 空 + 有 inlineKey → 兜底再拼 #inlineKey。
	gotInline := evaluatorResultKey(opt, 303, &entity.EvaluatorRecord{EvaluatorVersionID: 303, Alias: "a1", InlineKey: "ik9"})
	assert.Equal(t, wantFallback+"#ik9", gotInline)
}

func TestContextFromPayload_OnlyFilledTraceAndLogNoEmptyKeys(t *testing.T) {
	// context 只填有值的 log_id / trace_id，不含 message_id / thread_id / start_time / end_time。
	payload := &entity.ExperimentTurnPayload{
		TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{
			TraceID: "trace-xyz",
			LogID:   "log-xyz",
		}},
	}
	ctx := contextFromPayload(payload)
	assert.Equal(t, "trace-xyz", ctx["trace_id"])
	assert.Equal(t, "log-xyz", ctx["log_id"])
	assert.Len(t, ctx, 2, "context 只应含 log_id / trace_id 两个键: %v", ctx)
	for _, k := range []string{"message_id", "thread_id", "start_time", "end_time"} {
		_, has := ctx[k]
		assert.False(t, has, "context 不应含 %s", k)
	}
}

func TestContextFromPayload_NoTraceNoLog_EmptyMap(t *testing.T) {
	// 无 trace / log 的 payload → context 为空 map（不硬塞空串占位）。
	assert.Empty(t, contextFromPayload(nil))
	payload := &entity.ExperimentTurnPayload{
		TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{}},
	}
	ctx := contextFromPayload(payload)
	assert.Empty(t, ctx, "无 trace/log 时 context 应为空: %v", ctx)
}

func TestContextFromPayload_TurnSystemInfoLogIDUsedWhenRecordLogIDEmpty(t *testing.T) {
	// record.LogID 为空时回退用 TurnSystemInfo.LogID；record.LogID 非空时优先 record。
	fallback := &entity.ExperimentTurnPayload{
		SystemInfo:   &entity.TurnSystemInfo{LogID: gptr.Of("turn-log")},
		TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{}},
	}
	assert.Equal(t, "turn-log", contextFromPayload(fallback)["log_id"])

	recordWins := &entity.ExperimentTurnPayload{
		SystemInfo:   &entity.TurnSystemInfo{LogID: gptr.Of("turn-log")},
		TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{LogID: "record-log"}},
	}
	assert.Equal(t, "record-log", contextFromPayload(recordWins)["log_id"])
}

func TestTokensFromPayload_OnlyFilledStats(t *testing.T) {
	// tokens 只填有值的统计项，无值不塞 0。
	payload := &entity.ExperimentTurnPayload{
		TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{
			EvalTargetOutputData: &entity.EvalTargetOutputData{
				EvalTargetUsage: &entity.EvalTargetUsage{InputTokens: 5, TotalTokens: 5},
			},
		}},
	}
	tokens := tokensFromPayload(payload)
	assert.EqualValues(t, 5, tokens["prompt_tokens"])
	assert.EqualValues(t, 5, tokens["total_tokens"])
	_, hasCompletion := tokens["completion_tokens"]
	assert.False(t, hasCompletion, "completion_tokens 为 0 不应出现: %v", tokens)
	assert.Len(t, tokens, 2)
}

func TestTokensFromPayload_NoUsage_EmptyMap(t *testing.T) {
	// 无 usage（nil payload / nil usage）→ tokens 为空 map。
	assert.Empty(t, tokensFromPayload(nil))
	noUsage := &entity.ExperimentTurnPayload{
		TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{
			EvalTargetOutputData: &entity.EvalTargetOutputData{},
		}},
	}
	assert.Empty(t, tokensFromPayload(noUsage), "无 usage 时 tokens 应为空")
	// 全 0 usage → 也不填任何 key。
	zeroUsage := &entity.ExperimentTurnPayload{
		TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{
			EvalTargetOutputData: &entity.EvalTargetOutputData{EvalTargetUsage: &entity.EvalTargetUsage{}},
		}},
	}
	assert.Empty(t, tokensFromPayload(zeroUsage), "全 0 usage 时 tokens 应为空")
}

func TestBuildItemStandardEvalOutput_PlatformEvalOutputNoInnerRounds(t *testing.T) {
	// 平台兜底的 eval / output 只补 detail，不再补内部 round 粒度 rounds（顶层 rounds 字段不受影响）。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)

	var eval map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetEval().GetText()), &eval))
	assert.Contains(t, eval, "detail")
	assert.NotContains(t, eval, "rounds")

	var output map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetOutput().GetText()), &output))
	assert.Contains(t, output, "detail")
	assert.NotContains(t, output, "rounds")

	// 顶层 rounds 字段仍由平台兜底产出（每轮 query/latency/context），不受影响。
	require.NotNil(t, got.Rounds)
}
