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
	assert.Equal(t, "src-200", agent["source_target_id"])
	assert.Equal(t, "200", agent["target_id"]) // i64 已 string 化防精度丢失

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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

			got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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
	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	var output map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetOutput().GetText()), &output))
	detail := output["detail"].(map[string]any)
	assert.Equal(t, "from-object", detail["custom"]) // 对象子字段
	// 平台 detail 里原有的 file_diff 兄弟键仍在。
	assert.Contains(t, detail, "file_diff")
	// 平台不再补 output.rounds（内部 round 粒度补全已移除）。
	assert.NotContains(t, output, "rounds")
}

func TestBuildItemStandardEvalOutput_RoundsMergeByRoundID(t *testing.T) {
	// rounds 按 round_id 对齐：对象报 round_1 的补充字段，平台其他轮保留。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	// 平台 round_id 形如 round_<turnID>，此处 turnID=1 → round_1。
	injectFornaxField(item, "FORNAX_rounds", `[{"round_id":"round_1","extra_note":"obj"}]`)

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
	require.NoError(t, err)
	var rounds []any
	require.NoError(t, json.Unmarshal([]byte(got.GetRounds().GetText()), &rounds))
	require.Len(t, rounds, 1) // 对齐合并，不新增元素
	r0 := rounds[0].(map[string]any)
	assert.Equal(t, "obj", r0["extra_note"]) // 对象补充字段
	assert.Contains(t, r0, "round_no")       // 平台原字段保留
}

func TestBuildItemStandardEvalOutput_BareKeyFallback(t *testing.T) {
	// 向前兼容：对象用旧裸 key output 上报，仍被识别复用。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	injectFornaxField(item, "output", `{"detail":{"custom":"bare-key"}}`)

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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
	got, err := buildItemStandardEvalOutput(item, opt)
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
	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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

func TestMergeRoundsField_AppendsUnmatchedObjectRound(t *testing.T) {
	platform := []any{map[string]any{"round_id": "round_1", "p": true}}
	object := []any{map[string]any{"round_id": "round_2", "o": true}}
	merged := mergeRoundsField(platform, object).([]any)
	require.Len(t, merged, 2) // round_2 追加
}

func TestEvaluatorResultKey_NoCollision(t *testing.T) {
	// 同 versionID 多 alias 不撞;alias 空退化裸 versionID(旧数据不变);inline 用 InlineKey 兜底。
	cases := []struct {
		name   string
		key    int64
		record *entity.EvaluatorRecord
		want   string
	}{
		{"无 alias 退化裸 versionID", 101, &entity.EvaluatorRecord{EvaluatorVersionID: 101}, "101"},
		{"同版本 alias A", 101, &entity.EvaluatorRecord{EvaluatorVersionID: 101, Alias: "judge_A"}, "101:judge_A"},
		{"同版本 alias B", 101, &entity.EvaluatorRecord{EvaluatorVersionID: 101, Alias: "judge_B"}, "101:judge_B"},
		{"inline versionID=0 用 InlineKey", 0, &entity.EvaluatorRecord{EvaluatorVersionID: 0, InlineKey: "ik1"}, "0#ik1"},
		{"inline 第二条不同 InlineKey", 0, &entity.EvaluatorRecord{EvaluatorVersionID: 0, InlineKey: "ik2"}, "0#ik2"},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		got := evaluatorResultKey(c.key, c.record)
		assert.Equal(t, c.want, got, c.name)
		assert.False(t, seen[got], "result_key 撞了: %s (%s)", got, c.name)
		seen[got] = true
	}
}

func TestBuildItemStandardEvalOutput_PlatformEvalOutputNoInnerRounds(t *testing.T) {
	// 平台兜底的 eval / output 只补 detail，不再补内部 round 粒度 rounds（顶层 rounds 字段不受影响）。
	item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20})
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
