// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

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
	assert.Contains(t, output, "rounds")

	var eval map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetEval().GetText()), &eval))
	assert.Contains(t, eval, "task_config")
	assert.Contains(t, eval, "detail")
	assert.Contains(t, eval, "rounds")

	require.NotNil(t, got.Agent)
	var agent map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetAgent().GetText()), &agent))
	assert.Equal(t, "src-200", agent["source_target_id"])
	assert.EqualValues(t, 200, agent["target_id"])

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
	return &entity.MGetExperimentReportResult{
		Total: 1,
		ItemResults: []*entity.ItemResult{{
			ItemID:    itemID,
			ItemIndex: gptr.Of(turnIndex),
			Ext:       map[string]string{"dataset_key": "dataset-1", "item_key": "case-1"},
			SystemInfo: &entity.ItemSystemInfo{
				RunState: entity.ItemRunState_Success,
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
	return &entity.Experiment{
		ID:                 exptID,
		SpaceID:            spaceID,
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
	assert.Contains(t, got.Output.GetText(), "rounds")
}
