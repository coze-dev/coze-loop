package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
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
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc}

	const (
		workspaceID    int64 = 1
		exptID         int64 = 2
		exptRunID      int64 = 3
		itemID         int64 = 4
		turnID         int64 = 5
		targetRecordID int64 = 6
	)

	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *rpc.AuthorizationParam) error {
			assert.Equal(t, workspaceID, param.SpaceID)
			require.Len(t, param.ActionObjects, 1)
			assert.Equal(t, consts.ActionReadExpt, gptr.Indirect(param.ActionObjects[0].Action))
			return nil
		},
	)
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error) {
			assert.Equal(t, workspaceID, param.SpaceID)
			assert.Equal(t, []int64{exptID}, param.ExptIDs)
			require.NotNil(t, param.BaseExptID)
			assert.Equal(t, exptID, *param.BaseExptID)
			assert.True(t, param.UseAccelerator)
			require.NotNil(t, param.FilterAccelerators[exptID])
			require.Len(t, param.FilterAccelerators[exptID].ItemIDs, 1)
			assert.Equal(t, "IN", param.FilterAccelerators[exptID].ItemIDs[0].Op)
			return makeStandardEvalOutputReportResult(exptID, exptRunID, itemID, turnID, targetRecordID), nil
		},
	)

	resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{
		WorkspaceID: workspaceID,
		ExptID:      exptID,
		ExptRunID:   exptRunID,
		ItemIds:     []int64{itemID},
		IncludeRaw:  gptr.Of(true),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Items, 1)
	got := resp.Items[0]
	assert.Equal(t, exptID, got.ExptID)
	assert.Equal(t, exptRunID, got.ExptRunID)
	assert.Equal(t, itemID, got.ItemID)
	require.NotNil(t, got.RawJSON)
	require.NotNil(t, got.Output)
	require.NotNil(t, got.Eval)

	var output map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetOutput()), &output))
	assert.Contains(t, output, "turns")

	var eval map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.GetEval()), &eval))
	assert.Contains(t, eval, "turns")
}

func TestExperimentApplication_MGetExperimentStandardEvalOutputs_APIKeyBypass(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Times(0)
	mockConfiger := componentmocks.NewMockIConfiger(ctrl)
	mockConfiger.EXPECT().GetStandardEvalOutputAPIKey(gomock.Any()).Return("test-key")
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Return(makeStandardEvalOutputReportResult(2, 3, 4, 5, 6), nil)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc, configer: mockConfiger}

	resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{
		WorkspaceID: 1,
		ExptID:      2,
		ExptRunID:   3,
		ItemIds:     []int64{4},
		APIKey:      gptr.Of("test-key"),
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
}

func TestExperimentApplication_MGetExperimentStandardEvalOutputs_Sections(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc}

	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Return(makeStandardEvalOutputReportResult(2, 3, 4, 5, 6), nil)

	resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{
		WorkspaceID: 1,
		ExptID:      2,
		ExptRunID:   3,
		ItemIds:     []int64{4},
		Sections:    []string{"output", "eval"},
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Nil(t, resp.Items[0].Source)
	assert.Nil(t, resp.Items[0].Detail)
	assert.Nil(t, resp.Items[0].Rounds)
	assert.NotNil(t, resp.Items[0].Output)
	assert.NotNil(t, resp.Items[0].Eval)
	assert.Nil(t, resp.Items[0].RawJSON)
}

func TestExperimentApplication_ListExperimentStandardEvalOutputs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc}

	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error) {
			assert.Equal(t, entity.NewPage(2, 10), param.Page)
			return makeStandardEvalOutputReportResult(2, 3, 4, 5, 6), nil
		},
	)

	resp, err := app.ListExperimentStandardEvalOutputs(context.Background(), &exptpb.ListExperimentStandardEvalOutputsRequest{
		WorkspaceID: 1,
		ExptID:      2,
		ExptRunID:   3,
		PageNumber:  gptr.Of(int32(2)),
		PageSize:    gptr.Of(int32(10)),
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.NotNil(t, resp.Total)
	assert.Equal(t, int64(1), *resp.Total)
}

func TestExperimentApplication_MGetExperimentStandardEvalOutputs_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockResultSvc := servicemocks.NewMockExptResultService(ctrl)
	app := &experimentApplication{auth: mockAuth, resultSvc: mockResultSvc}

	_, err := app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{WorkspaceID: 1, ExptID: 2, ExptRunID: 3})
	require.Error(t, err)

	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))
	_, err = app.MGetExperimentStandardEvalOutputs(context.Background(), &exptpb.MGetExperimentStandardEvalOutputsRequest{WorkspaceID: 1, ExptID: 2, ExptRunID: 3, ItemIds: []int64{4}})
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
							ItemID:    itemID,
							EvalSetID: 100,
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

func TestBuildItemStandardEvalOutput_ParseReportedStandardEvalOutput(t *testing.T) {
	textType := entity.ContentTypeText
	reported := `{"detail_id":"sandbox-detail","source":"fornax","rounds":[{"round_no":1}],"agent":{"agent_name":"codex"},"output":{"detail":{"file_diff":[]}},"eval":{"score":1},"extra":{}}`
	item := &entity.ItemResult{
		ItemID: 10,
		Ext:    map[string]string{"item_key": "case-10"},
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

	got, err := buildItemStandardEvalOutput(item, standardEvalOutputBuildOptions{ExptID: 20, ExptRunID: 30, IncludeRaw: true})
	require.NoError(t, err)
	assert.Equal(t, "case-10", got.GetItemKey())
	assert.Equal(t, "sandbox-detail", got.GetDetailID())
	require.NotNil(t, got.Source)
	assert.Equal(t, `"fornax"`, got.GetSource())
	require.NotNil(t, got.Agent)
	assert.Contains(t, got.GetAgent(), "codex")
	require.NotNil(t, got.RawJSON)
	assert.Contains(t, got.GetRawJSON(), "sandbox-detail")
}
