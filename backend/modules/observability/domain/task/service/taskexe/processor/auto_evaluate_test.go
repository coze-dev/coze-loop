// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/cloudwego/kitex/client/callopt"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	taskentity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	traceentity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	evalrpc "github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluation"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type taskRepoMockAdapter struct {
	*repomocks.MockITaskRepo
}

func (m *taskRepoMockAdapter) IncrTaskRunFailCount(ctx context.Context, taskID, taskRunID, ttl int64) error {
	return m.MockITaskRepo.IncrTaskRunFailCount(ctx, taskID, taskRunID, ttl)
}

func (m *taskRepoMockAdapter) IncrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID, ttl int64) error {
	return m.MockITaskRepo.IncrTaskRunSuccessCount(ctx, taskID, taskRunID, ttl)
}

func (m *taskRepoMockAdapter) ListNonFinalTask(context.Context, string) ([]int64, error) {
	return nil, nil
}

func (m *taskRepoMockAdapter) AddNonFinalTask(context.Context, string, int64) error {
	return nil
}

func (m *taskRepoMockAdapter) RemoveNonFinalTask(context.Context, string, int64) error {
	return nil
}

func (m *taskRepoMockAdapter) GetTaskByCache(context.Context, int64) (*taskentity.ObservabilityTask, error) {
	return nil, nil
}

func (m *taskRepoMockAdapter) SetTask(context.Context, *taskentity.ObservabilityTask) error {
	return nil
}

func buildTestTask(t *testing.T) *taskentity.ObservabilityTask {
	t.Helper()
	start := time.Now().Add(-30 * time.Minute).UnixMilli()
	end := time.Now().Add(time.Hour).UnixMilli()
	fieldName := "field_1"
	return &taskentity.ObservabilityTask{
		ID:          101,
		WorkspaceID: 202,
		Name:        "auto-eval",
		CreatedBy:   "1001",
		TaskType:    taskentity.TaskTypeAutoEval,
		TaskStatus:  taskentity.TaskStatusUnstarted,
		EffectiveTime: &taskentity.EffectiveTime{
			StartAt: start,
			EndAt:   end,
		},
		BackfillEffectiveTime: &taskentity.EffectiveTime{
			StartAt: start,
			EndAt:   end,
		},
		Sampler: &taskentity.Sampler{
			SampleRate:    1,
			SampleSize:    10,
			IsCycle:       false,
			CycleCount:    0,
			CycleInterval: 1,
			CycleTimeUnit: taskentity.TimeUnitDay,
		},
		SpanFilter: &taskentity.SpanFilterFields{
			PlatformType: loop_span.PlatformVeAgentKit,
			Filters: loop_span.FilterFields{
				FilterFields: []*loop_span.FilterField{
					{
						FieldName: "cozeloop_agent_runtime_id",
						QueryType: gptr.Of(loop_span.QueryTypeEnumIn),
						Values:    []string{"test-agent-id"},
					},
				},
			},
		},
		TaskConfig: &taskentity.TaskConfig{
			AutoEvaluateConfigs: []*taskentity.AutoEvaluateConfig{
				{
					EvaluatorVersionID: 111,
					FieldMappings: []*taskentity.EvaluateFieldMapping{
						{
							FieldSchema: &dataset.FieldSchema{
								Name:        gptr.Of(fieldName),
								ContentType: gptr.Of(common.ContentTypeText),
								TextSchema:  gptr.Of("{}"),
							},
							TraceFieldKey:      "Input",
							TraceFieldJsonpath: "",
							EvalSetName:        gptr.Of(fieldName),
						},
					},
				},
			},
		},
	}
}

func buildTaskRunConfig(schema string) *taskentity.TaskRunConfig {
	return &taskentity.TaskRunConfig{
		AutoEvaluateRunConfig: &taskentity.AutoEvaluateRunConfig{
			ExptID:       301,
			ExptRunID:    401,
			EvalID:       501,
			SchemaID:     601,
			Schema:       gptr.Of(schema),
			EndAt:        time.Now().Add(time.Hour).UnixMilli(),
			CycleStartAt: time.Now().Add(-time.Minute).UnixMilli(),
			CycleEndAt:   time.Now().Add(time.Hour).UnixMilli(),
			Status:       task.TaskStatusRunning,
		},
	}
}

func buildSpan(input string) *loop_span.Span {
	return &loop_span.Span{
		TraceID: "1234567890abcdef1234567890abcdef",
		SpanID:  "feedc0ffeedc0ffe",
		Input:   input,
	}
}

func makeSchemaJSON(t *testing.T, fieldName string, contentType common.ContentType) string {
	t.Helper()
	fieldSchemas := []traceentity.FieldSchema{
		{
			Key:         gptr.Of(fieldName),
			Name:        fieldName,
			ContentType: traceentity.ContentType(contentType),
			TextSchema:  "{}",
		},
	}
	bytes, err := json.Marshal(fieldSchemas)
	if err != nil {
		t.Fatalf("marshal schema failed: %v", err)
	}
	return string(bytes)
}

// ==== 以下为复用 evaluation_test.go 的内容，验证 EvaluationProvider 在 Processor 上的行为 ====

// fakeExperimentClient 满足 experimentservice.Client 接口（以空桩方法实现）
type fakeExperimentClient struct {
	invokeResp *expt.InvokeExperimentResponse
	invokeErr  error
}

func (f *fakeExperimentClient) CalculateExperimentAggrResult_(ctx context.Context, req *expt.CalculateExperimentAggrResultRequest, callOptions ...callopt.Option) (r *expt.CalculateExperimentAggrResultResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (f *fakeExperimentClient) GetAnalysisRecordFeedbackVote(ctx context.Context, req *expt.GetAnalysisRecordFeedbackVoteRequest, callOptions ...callopt.Option) (r *expt.GetAnalysisRecordFeedbackVoteResponse, err error) {
	return nil, nil
}

// IDL 方法空桩实现（除 InvokeExperiment 外，其余均返回 nil）
func (f *fakeExperimentClient) CheckExperimentName(ctx context.Context, req *expt.CheckExperimentNameRequest, callOptions ...callopt.Option) (*expt.CheckExperimentNameResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) CreateExperiment(ctx context.Context, req *expt.CreateExperimentRequest, callOptions ...callopt.Option) (*expt.CreateExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) CreateExperimentTemplate(ctx context.Context, req *expt.CreateExperimentTemplateRequest, callOptions ...callopt.Option) (*expt.CreateExperimentTemplateResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) UpdateExperimentTemplate(ctx context.Context, req *expt.UpdateExperimentTemplateRequest, callOptions ...callopt.Option) (*expt.UpdateExperimentTemplateResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) DeleteExperimentTemplate(ctx context.Context, req *expt.DeleteExperimentTemplateRequest, callOptions ...callopt.Option) (*expt.DeleteExperimentTemplateResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) ListExperimentTemplates(ctx context.Context, req *expt.ListExperimentTemplatesRequest, callOptions ...callopt.Option) (*expt.ListExperimentTemplatesResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) BatchGetExperimentTemplate(ctx context.Context, req *expt.BatchGetExperimentTemplateRequest, callOptions ...callopt.Option) (*expt.BatchGetExperimentTemplateResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) SubmitExperiment(ctx context.Context, req *expt.SubmitExperimentRequest, callOptions ...callopt.Option) (*expt.SubmitExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) BatchGetExperiments(ctx context.Context, req *expt.BatchGetExperimentsRequest, callOptions ...callopt.Option) (*expt.BatchGetExperimentsResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) ListExperiments(ctx context.Context, req *expt.ListExperimentsRequest, callOptions ...callopt.Option) (*expt.ListExperimentsResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) UpdateExperiment(ctx context.Context, req *expt.UpdateExperimentRequest, callOptions ...callopt.Option) (*expt.UpdateExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) DeleteExperiment(ctx context.Context, req *expt.DeleteExperimentRequest, callOptions ...callopt.Option) (*expt.DeleteExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) BatchDeleteExperiments(ctx context.Context, req *expt.BatchDeleteExperimentsRequest, callOptions ...callopt.Option) (*expt.BatchDeleteExperimentsResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) CloneExperiment(ctx context.Context, req *expt.CloneExperimentRequest, callOptions ...callopt.Option) (*expt.CloneExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) RunExperiment(ctx context.Context, req *expt.RunExperimentRequest, callOptions ...callopt.Option) (*expt.RunExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) RetryExperiment(ctx context.Context, req *expt.RetryExperimentRequest, callOptions ...callopt.Option) (*expt.RetryExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) KillExperiment(ctx context.Context, req *expt.KillExperimentRequest, callOptions ...callopt.Option) (*expt.KillExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) BatchGetExperimentResult_(ctx context.Context, req *expt.BatchGetExperimentResultRequest, callOptions ...callopt.Option) (*expt.BatchGetExperimentResultResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) BatchGetExperimentAggrResult_(ctx context.Context, req *expt.BatchGetExperimentAggrResultRequest, callOptions ...callopt.Option) (*expt.BatchGetExperimentAggrResultResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) InvokeExperiment(ctx context.Context, req *expt.InvokeExperimentRequest, callOptions ...callopt.Option) (*expt.InvokeExperimentResponse, error) {
	return f.invokeResp, f.invokeErr
}

func (f *fakeExperimentClient) FinishExperiment(ctx context.Context, req *expt.FinishExperimentRequest, callOptions ...callopt.Option) (*expt.FinishExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) ListExperimentStats(ctx context.Context, req *expt.ListExperimentStatsRequest, callOptions ...callopt.Option) (*expt.ListExperimentStatsResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) UpsertExptTurnResultFilter(ctx context.Context, req *expt.UpsertExptTurnResultFilterRequest, callOptions ...callopt.Option) (*expt.UpsertExptTurnResultFilterResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) AssociateAnnotationTag(ctx context.Context, req *expt.AssociateAnnotationTagReq, callOptions ...callopt.Option) (*expt.AssociateAnnotationTagResp, error) {
	return nil, nil
}

func (f *fakeExperimentClient) DeleteAnnotationTag(ctx context.Context, req *expt.DeleteAnnotationTagReq, callOptions ...callopt.Option) (*expt.DeleteAnnotationTagResp, error) {
	return nil, nil
}

func (f *fakeExperimentClient) CreateAnnotateRecord(ctx context.Context, req *expt.CreateAnnotateRecordReq, callOptions ...callopt.Option) (*expt.CreateAnnotateRecordResp, error) {
	return nil, nil
}

func (f *fakeExperimentClient) UpdateAnnotateRecord(ctx context.Context, req *expt.UpdateAnnotateRecordReq, callOptions ...callopt.Option) (*expt.UpdateAnnotateRecordResp, error) {
	return nil, nil
}

func (f *fakeExperimentClient) ExportExptResult_(ctx context.Context, req *expt.ExportExptResultRequest, callOptions ...callopt.Option) (*expt.ExportExptResultResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) ListExptResultExportRecord(ctx context.Context, req *expt.ListExptResultExportRecordRequest, callOptions ...callopt.Option) (*expt.ListExptResultExportRecordResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) GetExptResultExportRecord(ctx context.Context, req *expt.GetExptResultExportRecordRequest, callOptions ...callopt.Option) (*expt.GetExptResultExportRecordResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) InsightAnalysisExperiment(ctx context.Context, req *expt.InsightAnalysisExperimentRequest, callOptions ...callopt.Option) (*expt.InsightAnalysisExperimentResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) ListExptInsightAnalysisRecord(ctx context.Context, req *expt.ListExptInsightAnalysisRecordRequest, callOptions ...callopt.Option) (*expt.ListExptInsightAnalysisRecordResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) DeleteExptInsightAnalysisRecord(ctx context.Context, req *expt.DeleteExptInsightAnalysisRecordRequest, callOptions ...callopt.Option) (*expt.DeleteExptInsightAnalysisRecordResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) GetExptInsightAnalysisRecord(ctx context.Context, req *expt.GetExptInsightAnalysisRecordRequest, callOptions ...callopt.Option) (*expt.GetExptInsightAnalysisRecordResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) FeedbackExptInsightAnalysisReport(ctx context.Context, req *expt.FeedbackExptInsightAnalysisReportRequest, callOptions ...callopt.Option) (*expt.FeedbackExptInsightAnalysisReportResponse, error) {
	return nil, nil
}

func (f *fakeExperimentClient) ListExptInsightAnalysisComment(ctx context.Context, req *expt.ListExptInsightAnalysisCommentRequest, callOptions ...callopt.Option) (*expt.ListExptInsightAnalysisCommentResponse, error) {
	return nil, nil
}

// 使用真实 EvaluationProvider 注入 Processor，验证三种路径：BizStatus、非 BizStatus 包装、成功返回条数
func TestAutoEvaluateProcessor_Invoke_WithEvaluationProvider_BizStatusPassthrough(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 任务与触发构造
	taskObj := buildTestTask(t)
	taskObj.Sampler.SampleSize = 5
	trigger := &taskexe.Trigger{
		Task: taskObj,
		Span: buildSpan("hello"),
		TaskRun: &taskentity.TaskRun{
			ID:            1001,
			TaskID:        taskObj.ID,
			WorkspaceID:   taskObj.WorkspaceID,
			TaskType:      taskentity.TaskRunTypeNewData,
			RunStatus:     taskentity.TaskRunStatusRunning,
			TaskRunConfig: buildTaskRunConfig(makeSchemaJSON(t, "field_1", common.ContentTypeText)),
		},
	}

	// 仓库计数行为与 run 终止更新
	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
	repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(1), nil)
	repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(1), nil)
	repoMock.EXPECT().DecrTaskCount(gomock.Any(), taskObj.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().UpdateTaskRun(gomock.Any(), trigger.TaskRun).Return(nil)

	// Provider 返回 BizStatus 错误码
	client := &fakeExperimentClient{invokeErr: errorx.NewByCode(errno.ExperimentStatusNotAllowedToInvokeCode)}
	provider := evalrpc.NewEvaluationRPCProvider(client)

	proc := &AutoEvaluateProcessor{evaluationSvc: provider, taskRepo: repoAdapter}
	err := proc.Invoke(context.Background(), trigger)
	status, ok := errorx.FromStatusError(err)
	assert.True(t, ok)
	assert.EqualValues(t, errno.ExperimentStatusNotAllowedToInvokeCode, status.Code())
	assert.Equal(t, taskentity.TaskRunStatusDone, trigger.TaskRun.RunStatus)
}

func TestAutoEvaluateProcessor_Invoke_WithEvaluationProvider_WrapNonBizError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskObj := buildTestTask(t)
	taskObj.Sampler.SampleSize = 5
	trigger := &taskexe.Trigger{
		Task: taskObj,
		Span: buildSpan("hello"),
		TaskRun: &taskentity.TaskRun{
			ID:            1001,
			TaskID:        taskObj.ID,
			WorkspaceID:   taskObj.WorkspaceID,
			TaskType:      taskentity.TaskRunTypeNewData,
			RunStatus:     taskentity.TaskRunStatusRunning,
			TaskRunConfig: buildTaskRunConfig(makeSchemaJSON(t, "field_1", common.ContentTypeText)),
		},
	}

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
	repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(1), nil)
	repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(1), nil)
	repoMock.EXPECT().DecrTaskCount(gomock.Any(), taskObj.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)

	client := &fakeExperimentClient{invokeErr: fmt.Errorf("rpc fail")}
	provider := evalrpc.NewEvaluationRPCProvider(client)
	proc := &AutoEvaluateProcessor{evaluationSvc: provider, taskRepo: repoAdapter}
	err := proc.Invoke(context.Background(), trigger)
	status, ok := errorx.FromStatusError(err)
	assert.True(t, ok)
	// 错误被包装为通用RPC错误码
	assert.EqualValues(t, obErrorx.CommonRPCErrorCode, status.Code())
}

func TestAutoEvaluateProcessor_Invoke_WithEvaluationProvider_SuccessAddedItems(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskObj := buildTestTask(t)
	taskObj.Sampler.SampleSize = 5
	trigger := &taskexe.Trigger{
		Task: taskObj,
		Span: buildSpan("hello"),
		TaskRun: &taskentity.TaskRun{
			ID:            1001,
			TaskID:        taskObj.ID,
			WorkspaceID:   taskObj.WorkspaceID,
			TaskType:      taskentity.TaskRunTypeNewData,
			RunStatus:     taskentity.TaskRunStatusRunning,
			TaskRunConfig: buildTaskRunConfig(makeSchemaJSON(t, "field_1", common.ContentTypeText)),
		},
	}

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
	repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(1), nil)
	repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(1), nil)
	// 成功新增条目，不应回退计数
	repoMock.EXPECT().DecrTaskCount(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	client := &fakeExperimentClient{invokeResp: &expt.InvokeExperimentResponse{AddedItems: map[int64]int64{1: 1, 2: 1}}}
	provider := evalrpc.NewEvaluationRPCProvider(client)
	proc := &AutoEvaluateProcessor{evaluationSvc: provider, taskRepo: repoAdapter}
	err := proc.Invoke(context.Background(), trigger)
	assert.NoError(t, err)
}

func TestEvaluationProvider_InvokeExperiment_EmptyWorkspaceID_InProcessorPackage(t *testing.T) {
	t.Parallel()
	provider := evalrpc.NewEvaluationRPCProvider(&fakeExperimentClient{})

	_, err := provider.InvokeExperiment(context.Background(), &rpc.InvokeExperimentReq{
		WorkspaceID:     0,
		EvaluationSetID: 123,
	})
	status, ok := errorx.FromStatusError(err)
	assert.True(t, ok)
	assert.EqualValues(t, obErrorx.CommonInvalidParamCode, status.Code())
}

func TestEvaluationProvider_InvokeExperiment_EmptyEvaluationSetID_InProcessorPackage(t *testing.T) {
	t.Parallel()
	provider := evalrpc.NewEvaluationRPCProvider(&fakeExperimentClient{})

	_, err := provider.InvokeExperiment(context.Background(), &rpc.InvokeExperimentReq{
		WorkspaceID:     123,
		EvaluationSetID: 0,
	})
	status, ok := errorx.FromStatusError(err)
	assert.True(t, ok)
	assert.EqualValues(t, obErrorx.CommonInvalidParamCode, status.Code())
}

// 覆盖 Invoke 中 onTaskRunTerminated 返回错误的分支，确保错误被记录并向上返回
func TestAutoEvaluateProcessor_Invoke_OnTaskRunTerminatedError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 构造任务与触发器
	taskObj := buildTestTask(t)
	taskObj.Sampler.SampleSize = 5
	trigger := &taskexe.Trigger{
		Task: taskObj,
		Span: buildSpan("hello"),
		TaskRun: &taskentity.TaskRun{
			ID:            2002,
			TaskID:        taskObj.ID,
			WorkspaceID:   taskObj.WorkspaceID,
			TaskType:      taskentity.TaskRunTypeNewData,
			RunStatus:     taskentity.TaskRunStatusRunning,
			TaskRunConfig: buildTaskRunConfig(makeSchemaJSON(t, "field_1", common.ContentTypeText)),
		},
	}

	// 仓库：计数递增/读取/回退，以及 UpdateTaskRun 返回错误以模拟终止失败
	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
	repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(1), nil)
	repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(1), nil)
	repoMock.EXPECT().DecrTaskCount(gomock.Any(), taskObj.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.AssignableToTypeOf(int64(0))).Return(nil)
	repoMock.EXPECT().UpdateTaskRun(gomock.Any(), trigger.TaskRun).Return(errors.New("update failed"))

	// Provider 返回 BizStatus（实验失败），触发 onTaskRunTerminated
	eval := &fakeEvaluationAdapter{}
	eval.invokeResp.err = errorx.NewByCode(errno.ExperimentStatusNotAllowedToInvokeCode)

	proc := &AutoEvaluateProcessor{evaluationSvc: eval, taskRepo: repoAdapter}
	err := proc.Invoke(context.Background(), trigger)
	assert.EqualError(t, err, "update failed")
	// 即使更新失败，RunStatus 也已被置为 Done（onTaskRunTerminated 在更新前设置状态）
	assert.Equal(t, taskentity.TaskRunStatusDone, trigger.TaskRun.RunStatus)
}

func TestAutoEvaluateProcessor_ValidateConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	validTask := buildTestTask(t)
	validTask.EffectiveTime.StartAt = time.Now().Add(30 * time.Minute).UnixMilli()
	validTask.EffectiveTime.EndAt = time.Now().Add(2 * time.Hour).UnixMilli()

	cases := []struct {
		name      string
		config    any
		adapter   *fakeEvaluatorAdapter
		setupMock func(*rpcmock.MockIEvaluatorRPCAdapter)
		expectErr func(error) bool
	}{
		{
			name:   "invalid type",
			config: "bad",
			expectErr: func(err error) bool {
				status, ok := errorx.FromStatusError(err)
				return ok && status.Code() == obErrorx.CommonInvalidParamCode
			},
		},
		{
			name: "start too early",
			config: func() *taskentity.ObservabilityTask {
				task := buildTestTask(t)
				task.EffectiveTime.StartAt = time.Now().Add(-15 * time.Minute).UnixMilli()
				return task
			}(),
			expectErr: func(err error) bool {
				status, ok := errorx.FromStatusError(err)
				return ok && status.Code() == obErrorx.CommonInvalidParamCode
			},
		},
		{
			name: "start after end",
			config: func() *taskentity.ObservabilityTask {
				task := buildTestTask(t)
				task.EffectiveTime.StartAt = task.EffectiveTime.EndAt + 1
				return task
			}(),
			expectErr: func(err error) bool {
				status, ok := errorx.FromStatusError(err)
				return ok && status.Code() == obErrorx.CommonInvalidParamCode
			},
		},
		{
			name: "missing evaluators",
			config: func() *taskentity.ObservabilityTask {
				task := buildTestTask(t)
				task.TaskConfig.AutoEvaluateConfigs = nil
				return task
			}(),
			expectErr: func(err error) bool {
				status, ok := errorx.FromStatusError(err)
				return ok && status.Code() == obErrorx.CommonInvalidParamCode
			},
		},
		{
			name:    "batch get error",
			config:  validTask,
			adapter: &fakeEvaluatorAdapter{err: errors.New("svc error")},
			expectErr: func(err error) bool {
				status, ok := errorx.FromStatusError(err)
				return ok && status.Code() == obErrorx.CommonInvalidParamCode
			},
		},
		{
			name:    "length mismatch",
			config:  validTask,
			adapter: &fakeEvaluatorAdapter{},
			expectErr: func(err error) bool {
				status, ok := errorx.FromStatusError(err)
				return ok && status.Code() == obErrorx.CommonInvalidParamCode
			},
		},
		{
			name:   "success",
			config: validTask,
			adapter: &fakeEvaluatorAdapter{
				// 返回一个有效的评估器
				resp:    []*rpc.Evaluator{{EvaluatorVersionID: 111}},
				respMap: map[int64]*rpc.Evaluator{111: {EvaluatorVersionID: 111}},
			},
			expectErr: func(err error) bool { return err == nil },
		},
	}

	for _, tt := range cases {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			var evalAdapter rpc.IEvaluatorRPCAdapter
			if caseItem.adapter != nil {
				evalAdapter = caseItem.adapter
			} else {
				// 使用默认的fake adapter
				evalAdapter = &fakeEvaluatorAdapter{}
			}

			proc := &AutoEvaluateProcessor{evalSvc: evalAdapter}
			err := proc.ValidateConfig(ctx, caseItem.config)
			assert.True(t, caseItem.expectErr(err))
		})
	}
}

func TestAutoEvaluateProcessor_Invoke(t *testing.T) {
	t.Parallel()

	textSchema := makeSchemaJSON(t, "field_1", common.ContentTypeText)
	multiSchema := makeSchemaJSON(t, "field_1", common.ContentTypeMultiPart)

	buildTrigger := func(taskObj *taskentity.ObservabilityTask, schemaStr string) *taskexe.Trigger {
		taskRun := &taskentity.TaskRun{
			ID:            1001,
			TaskID:        taskObj.ID,
			WorkspaceID:   taskObj.WorkspaceID,
			TaskType:      taskentity.TaskRunTypeNewData,
			RunStatus:     taskentity.TaskRunStatusRunning,
			TaskRunConfig: buildTaskRunConfig(schemaStr),
		}
		span := buildSpan("{\"parts\":[]}")
		return &taskexe.Trigger{Task: taskObj, Span: span, TaskRun: taskRun}
	}

	t.Run("turns empty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		taskObj := buildTestTask(t)
		taskObj.TaskConfig.AutoEvaluateConfigs[0].FieldMappings[0].FieldSchema.ContentType = gptr.Of(common.ContentTypeMultiPart)
		taskObj.TaskConfig.AutoEvaluateConfigs[0].FieldMappings[0].TraceFieldJsonpath = ""

		trigger := buildTrigger(taskObj, multiSchema)
		trigger.Span.Input = "invalid json"

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
		evalMock := &fakeEvaluationAdapter{}
		proc := &AutoEvaluateProcessor{
			evaluationSvc: evalMock,
			taskRepo:      repoAdapter,
		}
		err := proc.Invoke(context.Background(), trigger)
		assert.NoError(t, err)
	})

	t.Run("exceed limits", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		taskObj := buildTestTask(t)
		taskObj.Sampler.CycleCount = 1
		taskObj.Sampler.SampleSize = 1
		trigger := buildTrigger(taskObj, textSchema)

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
		repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(2), nil)
		repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(2), nil)
		repoMock.EXPECT().DecrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)

		evalMock := &fakeEvaluationAdapter{}
		proc := &AutoEvaluateProcessor{
			evaluationSvc: evalMock,
			taskRepo:      repoAdapter,
		}
		err := proc.Invoke(context.Background(), trigger)
		assert.NoError(t, err)
	})

	t.Run("invoke error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		taskObj := buildTestTask(t)
		taskObj.Sampler.SampleSize = 5
		trigger := buildTrigger(taskObj, textSchema)

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
		repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(1), nil)
		repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(1), nil)
		repoMock.EXPECT().DecrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)

		eval := &fakeEvaluationAdapter{}
		eval.invokeResp.err = errors.New("invoke fail")

		proc := &AutoEvaluateProcessor{
			evaluationSvc: eval,
			taskRepo:      repoAdapter,
		}
		err := proc.Invoke(context.Background(), trigger)
		assert.EqualError(t, err, "invoke fail")
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		taskObj := buildTestTask(t)
		taskObj.Sampler.SampleSize = 5
		trigger := buildTrigger(taskObj, textSchema)

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
		repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(1), nil)
		repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(1), nil)

		evalMock := &fakeEvaluationAdapter{}
		evalMock.invokeResp.addedItems = 1

		repoMock.EXPECT().DecrTaskCount(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		proc := &AutoEvaluateProcessor{
			evaluationSvc: evalMock,
			taskRepo:      repoAdapter,
		}
		err := proc.Invoke(context.Background(), trigger)
		assert.NoError(t, err)
	})
}

func TestAutoEvaluateProcessor_OnUpdateTaskChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name    string
		initial taskentity.TaskStatus
		op      taskentity.TaskStatus
		expect  taskentity.TaskStatus
	}{
		{"success", taskentity.TaskStatusRunning, taskentity.TaskStatusSuccess, taskentity.TaskStatusSuccess},
		{"running", taskentity.TaskStatusPending, taskentity.TaskStatusRunning, taskentity.TaskStatusRunning},
		{"disable", taskentity.TaskStatusRunning, taskentity.TaskStatusDisabled, taskentity.TaskStatusDisabled},
		{"pending", taskentity.TaskStatusUnstarted, taskentity.TaskStatusPending, taskentity.TaskStatusPending},
	}

	for _, tt := range cases {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repoMock := repomocks.NewMockITaskRepo(ctrl)
			repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
			repoMock.EXPECT().UpdateTask(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.ObservabilityTask{})).DoAndReturn(
				func(_ context.Context, taskObj *taskentity.ObservabilityTask) error {
					assert.Equal(t, caseItem.expect, taskObj.TaskStatus)
					return nil
				})

			proc := &AutoEvaluateProcessor{taskRepo: repoAdapter}
			taskObj := &taskentity.ObservabilityTask{TaskStatus: caseItem.initial}
			err := proc.OnTaskUpdated(ctx, taskObj, caseItem.op)
			assert.NoError(t, err)
		})
	}

	t.Run("invalid op", func(t *testing.T) {
		proc := &AutoEvaluateProcessor{}
		err := proc.OnTaskUpdated(ctx, &taskentity.ObservabilityTask{}, "unknown")
		assert.Error(t, err)
	})
}

func TestAutoEvaluateProcessor_OnCreateTaskRunChange(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	datasetProvider := rpcmock.NewMockIDatasetProvider(ctrl)
	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}

	taskObj := buildTestTask(t)
	param := taskexe.OnTaskRunCreatedReq{
		CurrentTask: taskObj,
		RunType:     taskentity.TaskRunTypeNewData,
		RunStartAt:  time.Now().Add(-time.Minute).UnixMilli(),
		RunEndAt:    time.Now().Add(time.Hour).UnixMilli(),
	}

	datasetProvider.EXPECT().CreateDataset(gomock.Any(), gomock.AssignableToTypeOf(&traceentity.Dataset{})).Return(int64(9001), nil)
	datasetProvider.EXPECT().GetDataset(gomock.Any(), taskObj.WorkspaceID, int64(9001), traceentity.DatasetCategory_Evaluation).
		Return(&traceentity.Dataset{DatasetVersion: traceentity.DatasetVersion{DatasetSchema: traceentity.DatasetSchema{ID: 7001}}}, nil)
	repoMock.EXPECT().CreateTaskRun(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.TaskRun{})).Return(int64(1), nil)

	adaptor := service.NewDatasetServiceAdaptor()
	adaptor.Register(traceentity.DatasetCategory_Evaluation, datasetProvider)

	evalAdapter := &fakeEvaluationAdapter{}
	evalAdapter.submitResp.exptID = 1111
	evalAdapter.submitResp.exptRunID = 2222

	proc := &AutoEvaluateProcessor{
		datasetServiceAdaptor: adaptor,
		evaluationSvc:         evalAdapter,
		taskRepo:              repoAdapter,
		aid:                   321,
		evalTargetBuilder:     &EvalTargetBuilderImpl{},
	}

	ctx := session.WithCtxUser(context.Background(), &session.User{ID: taskObj.CreatedBy})
	err := proc.OnTaskRunCreated(ctx, param)
	assert.NoError(t, err)
}

func TestAutoEvaluateProcessor_OnFinishTaskRunChange(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
	evalMock := &fakeEvaluationAdapter{}
	// 不需要设置EXPECT，因为fake实现默认返回nil

	taskRun := &taskentity.TaskRun{
		ID: 8001,
		TaskRunConfig: &taskentity.TaskRunConfig{
			AutoEvaluateRunConfig: &taskentity.AutoEvaluateRunConfig{
				ExptID:    9001,
				ExptRunID: 9002,
			},
		},
	}
	repoMock.EXPECT().UpdateTaskRun(gomock.Any(), taskRun).Return(nil)

	proc := &AutoEvaluateProcessor{
		taskRepo:      repoAdapter,
		evaluationSvc: evalMock,
	}

	err := proc.OnTaskRunFinished(context.Background(), taskexe.OnTaskRunFinishedReq{
		Task:    &taskentity.ObservabilityTask{WorkspaceID: 1234, CreatedBy: "1001"},
		TaskRun: taskRun,
	})
	assert.NoError(t, err)
	assert.Equal(t, taskentity.TaskRunStatusDone, taskRun.RunStatus)
}

func TestAutoEvaluateProcessor_OnTaskRunTerminated(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}

	taskRun := &taskentity.TaskRun{ID: 8002, RunStatus: taskentity.TaskRunStatusRunning}
	repoMock.EXPECT().UpdateTaskRun(gomock.Any(), taskRun).Return(nil)

	proc := &AutoEvaluateProcessor{taskRepo: repoAdapter}
	err := proc.onTaskRunTerminated(context.Background(), taskRun)
	assert.NoError(t, err)
	assert.Equal(t, taskentity.TaskRunStatusDone, taskRun.RunStatus)
}

func TestAutoEvaluateProcessor_OnTaskRunTerminated_Error(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}

	taskRun := &taskentity.TaskRun{ID: 8010, RunStatus: taskentity.TaskRunStatusRunning}
	repoMock.EXPECT().UpdateTaskRun(gomock.Any(), taskRun).Return(errors.New("update failed"))

	proc := &AutoEvaluateProcessor{taskRepo: repoAdapter}
	err := proc.onTaskRunTerminated(context.Background(), taskRun)
	// 断言错误向上返回
	assert.EqualError(t, err, "update failed")
	// 即使更新失败，RunStatus 已置为 Done
	assert.Equal(t, taskentity.TaskRunStatusDone, taskRun.RunStatus)
}

func TestAutoEvaluateProcessor_OnFinishTaskChange(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
	evalAdapter := &fakeEvaluationAdapter{}

	taskObj := &taskentity.ObservabilityTask{TaskStatus: taskentity.TaskStatusRunning, WorkspaceID: 123, CreatedBy: "1001"}
	taskRun := &taskentity.TaskRun{TaskRunConfig: &taskentity.TaskRunConfig{AutoEvaluateRunConfig: &taskentity.AutoEvaluateRunConfig{ExptID: 1, ExptRunID: 2}}}

	repoMock.EXPECT().UpdateTaskRun(gomock.Any(), gomock.Any()).Return(nil)
	repoMock.EXPECT().UpdateTask(gomock.Any(), taskObj).Return(nil)

	proc := &AutoEvaluateProcessor{
		evaluationSvc: evalAdapter,
		taskRepo:      repoAdapter,
	}

	err := proc.OnTaskFinished(context.Background(), taskexe.OnTaskFinishedReq{
		Task:     taskObj,
		TaskRun:  taskRun,
		IsFinish: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, taskentity.TaskStatusSuccess, taskObj.TaskStatus)
}

func TestAutoEvaluateProcessor_OnFinishTaskChange_Error(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
	evalMock := &fakeEvaluationAdapter{}
	evalMock.finishResp.err = errors.New("finish fail")

	proc := &AutoEvaluateProcessor{
		evaluationSvc: evalMock,
		taskRepo:      repoAdapter,
	}

	err := proc.OnTaskFinished(context.Background(), taskexe.OnTaskFinishedReq{
		Task:    &taskentity.ObservabilityTask{WorkspaceID: 123, CreatedBy: "1001"},
		TaskRun: &taskentity.TaskRun{TaskRunConfig: &taskentity.TaskRunConfig{AutoEvaluateRunConfig: &taskentity.AutoEvaluateRunConfig{ExptID: 1, ExptRunID: 2}}},
	})
	assert.EqualError(t, err, "finish fail")
}

func TestAutoEvaluateProcessor_Invoke_CycleTask_ExperimentFailedOnlyFinishRun(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	evalAdapter := &fakeEvaluationAdapter{}

	workspaceID := int64(12345)
	taskID := int64(1002)
	runID := int64(2003)

	schema := []*traceentity.FieldSchema{{
		Key:         gptr.Of("input_text"),
		Name:        "input_text",
		Description: "",
		ContentType: traceentity.ContentType_Text,
	}}
	schemaBytes, _ := json.Marshal(schema)
	schemaStr := string(schemaBytes)

	taskObj := &taskentity.ObservabilityTask{
		ID:          taskID,
		WorkspaceID: workspaceID,
		TaskType:    taskentity.TaskTypeAutoEval,
		TaskStatus:  taskentity.TaskStatusRunning,
		Sampler: &taskentity.Sampler{
			SampleRate: 1,
			SampleSize: 1000,
			IsCycle:    true,
		},
		EffectiveTime: &taskentity.EffectiveTime{
			StartAt: time.Now().Add(-time.Hour).UnixMilli(),
			EndAt:   time.Now().Add(time.Hour).UnixMilli(),
		},
		CreatedBy: "1001",
	}

	taskRun := &taskentity.TaskRun{
		ID:          runID,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    taskentity.TaskRunTypeNewData,
		RunStatus:   taskentity.TaskRunStatusRunning,
		RunStartAt:  time.Now().Add(-30 * time.Minute),
		RunEndAt:    time.Now().Add(30 * time.Minute),
		TaskRunConfig: &taskentity.TaskRunConfig{
			AutoEvaluateRunConfig: &taskentity.AutoEvaluateRunConfig{
				ExptID:       3003,
				ExptRunID:    3004,
				EvalID:       4004,
				SchemaID:     5005,
				Schema:       gptr.Of(schemaStr),
				EndAt:        time.Now().Add(30 * time.Minute).UnixMilli(),
				CycleStartAt: time.Now().Add(-30 * time.Minute).UnixMilli(),
				CycleEndAt:   time.Now().Add(30 * time.Minute).UnixMilli(),
				Status:       string(taskentity.TaskRunStatusRunning),
			},
		},
	}

	mappings := []*taskentity.EvaluateFieldMapping{{
		EvalSetName:        gptr.Of("input_text"),
		TraceFieldKey:      "Input",
		TraceFieldJsonpath: "",
	}}
	taskObj.TaskConfig = &taskentity.TaskConfig{AutoEvaluateConfigs: []*taskentity.AutoEvaluateConfig{{FieldMappings: mappings}}}

	span := &loop_span.Span{TraceID: "trace-xyz", SpanID: "span-abc", StartTime: time.Now().UnixMilli(), Input: "hello world"}

	// Counts expectations (TTL 任意值)
	repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskID, gomock.AssignableToTypeOf(int64(0))).Return(nil).AnyTimes()
	repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskID, runID, gomock.AssignableToTypeOf(int64(0))).Return(nil).AnyTimes()
	repoMock.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(0), nil).AnyTimes()
	repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskID, runID).Return(int64(0), nil).AnyTimes()
	repoMock.EXPECT().DecrTaskCount(gomock.Any(), taskID, gomock.AssignableToTypeOf(int64(0))).Return(nil).Times(1)
	repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), taskID, runID, gomock.AssignableToTypeOf(int64(0))).Return(nil).Times(1)

	// Cycle: 不禁用、不移除，仅结束 run
	repoMock.EXPECT().UpdateTask(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.ObservabilityTask{})).Times(0)
	repoMock.EXPECT().RemoveNonFinalTask(gomock.Any(), strconv.FormatInt(workspaceID, 10), taskID).Times(0)
	repoMock.EXPECT().UpdateTaskRun(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.TaskRun{})).Return(nil).Times(1)

	evalAdapter.invokeResp.err = errorx.NewByCode(601204012)

	proc := &AutoEvaluateProcessor{taskRepo: repoMock, evaluationSvc: evalAdapter}

	err := proc.Invoke(context.Background(), &taskexe.Trigger{Task: taskObj, Span: span, TaskRun: taskRun})
	// 验证返回 BizStatus 错误码
	status, ok := errorx.FromStatusError(err)
	assert.True(t, ok)
	assert.EqualValues(t, 601204012, status.Code())
	assert.Equal(t, taskentity.TaskStatusRunning, taskObj.TaskStatus)
	assert.Equal(t, taskentity.TaskRunStatusDone, taskRun.RunStatus)
	// 验证 Invoke 参数的 Session 与 Ext 写入
	require.NotNil(t, evalAdapter.lastInvoke)
	assert.EqualValues(t, int64(1001), evalAdapter.lastInvoke.Session.GetUserID())
	assert.Equal(t, strconv.FormatInt(workspaceID, 10), evalAdapter.lastInvoke.Ext["workspace_id"])
	assert.Equal(t, strconv.FormatInt(taskID, 10), evalAdapter.lastInvoke.Ext["task_id"])
	assert.Equal(t, strconv.FormatInt(runID, 10), evalAdapter.lastInvoke.Ext["task_run_id"])
	assert.Equal(t, string(taskObj.GetPlatformType()), evalAdapter.lastInvoke.Ext["platform_type"])
	assert.NotEmpty(t, evalAdapter.lastInvoke.Ext["span_start_time"])
	assert.NotEmpty(t, evalAdapter.lastInvoke.Ext["span_end_time"])
}

func TestAutoEvaluateProcessor_OnCreateTaskChange(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	datasetProvider := rpcmock.NewMockIDatasetProvider(ctrl)
	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}

	adaptor := service.NewDatasetServiceAdaptor()
	adaptor.Register(traceentity.DatasetCategory_Evaluation, datasetProvider)

	evalMock := &fakeEvaluationAdapter{}
	evalMock.submitResp.exptID = 111
	evalMock.submitResp.exptRunID = 222

	proc := &AutoEvaluateProcessor{
		datasetServiceAdaptor: adaptor,
		evaluationSvc:         evalMock,
		taskRepo:              repoAdapter,
		aid:                   321,
		evalTargetBuilder:     &EvalTargetBuilderImpl{},
	}

	taskObj := buildTestTask(t)
	taskObj.TaskStatus = taskentity.TaskStatusPending

	var runTypes []taskentity.TaskRunType
	var statuses []taskentity.TaskStatus

	getBackfill := repoMock.EXPECT().GetBackfillTaskRun(gomock.Any(), (*int64)(nil), taskObj.ID).Return(nil, nil)
	createDatasetBackfill := datasetProvider.EXPECT().CreateDataset(gomock.Any(), gomock.AssignableToTypeOf(&traceentity.Dataset{})).Return(int64(9101), nil)
	getDatasetBackfill := datasetProvider.EXPECT().GetDataset(gomock.Any(), taskObj.WorkspaceID, int64(9101), traceentity.DatasetCategory_Evaluation).
		Return(&traceentity.Dataset{DatasetVersion: traceentity.DatasetVersion{DatasetSchema: traceentity.DatasetSchema{ID: 7101}}}, nil)
	createTaskRunBackfill := repoMock.EXPECT().CreateTaskRun(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.TaskRun{}))
	createTaskRunBackfill.DoAndReturn(func(_ context.Context, run *taskentity.TaskRun) (int64, error) {
		runTypes = append(runTypes, run.TaskType)
		return int64(len(runTypes)), nil
	})
	updateTaskBackfill := repoMock.EXPECT().UpdateTask(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.ObservabilityTask{}))
	updateTaskBackfill.DoAndReturn(func(_ context.Context, obj *taskentity.ObservabilityTask) error {
		statuses = append(statuses, obj.TaskStatus)
		return nil
	})
	createDatasetNewData := datasetProvider.EXPECT().CreateDataset(gomock.Any(), gomock.AssignableToTypeOf(&traceentity.Dataset{})).Return(int64(9101), nil)
	getDatasetNewData := datasetProvider.EXPECT().GetDataset(gomock.Any(), taskObj.WorkspaceID, int64(9101), traceentity.DatasetCategory_Evaluation).
		Return(&traceentity.Dataset{DatasetVersion: traceentity.DatasetVersion{DatasetSchema: traceentity.DatasetSchema{ID: 7101}}}, nil)
	createTaskRunNewData := repoMock.EXPECT().CreateTaskRun(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.TaskRun{}))
	createTaskRunNewData.DoAndReturn(func(_ context.Context, run *taskentity.TaskRun) (int64, error) {
		runTypes = append(runTypes, run.TaskType)
		return int64(len(runTypes)), nil
	})
	updateTaskNewData := repoMock.EXPECT().UpdateTask(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.ObservabilityTask{}))
	updateTaskNewData.DoAndReturn(func(_ context.Context, obj *taskentity.ObservabilityTask) error {
		statuses = append(statuses, obj.TaskStatus)
		return nil
	})

	gomock.InOrder(
		getBackfill,
		createDatasetBackfill,
		getDatasetBackfill,
		createTaskRunBackfill,
		updateTaskBackfill,
		createDatasetNewData,
		getDatasetNewData,
		createTaskRunNewData,
		updateTaskNewData,
	)

	err := proc.OnTaskCreated(context.Background(), taskObj)
	assert.NoError(t, err)
	assert.Equal(t, []taskentity.TaskRunType{taskentity.TaskRunTypeBackFill, taskentity.TaskRunTypeNewData}, runTypes)
	assert.Equal(t, []taskentity.TaskStatus{taskentity.TaskStatusRunning, taskentity.TaskStatusRunning}, statuses)
	assert.Equal(t, taskentity.TaskStatusRunning, taskObj.TaskStatus)
}

func TestAutoEvaluateProcessor_OnCreateTaskChange_GetBackfillError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}

	repoMock.EXPECT().GetBackfillTaskRun(gomock.Any(), (*int64)(nil), gomock.Any()).Return(nil, errors.New("db error"))

	proc := &AutoEvaluateProcessor{taskRepo: repoAdapter}

	err := proc.OnTaskCreated(context.Background(), buildTestTask(t))
	assert.EqualError(t, err, "db error")
}

func TestAutoEvaluateProcessor_OnCreateTaskChange_CreateDatasetError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	datasetProvider := rpcmock.NewMockIDatasetProvider(ctrl)
	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}

	adaptor := service.NewDatasetServiceAdaptor()
	adaptor.Register(traceentity.DatasetCategory_Evaluation, datasetProvider)

	proc := &AutoEvaluateProcessor{
		datasetServiceAdaptor: adaptor,
		taskRepo:              repoAdapter,
		evaluationSvc:         &fakeEvaluationAdapter{},
		evalTargetBuilder:     &EvalTargetBuilderImpl{},
	}

	repoMock.EXPECT().GetBackfillTaskRun(gomock.Any(), (*int64)(nil), gomock.Any()).Return(nil, nil)
	datasetProvider.EXPECT().CreateDataset(gomock.Any(), gomock.AssignableToTypeOf(&traceentity.Dataset{})).Return(int64(0), errors.New("create fail"))

	err := proc.OnTaskCreated(context.Background(), buildTestTask(t))
	assert.EqualError(t, err, "create fail")
}

func TestAutoEvaluateProcessor_getSession(t *testing.T) {
	t.Parallel()
	proc := &AutoEvaluateProcessor{aid: 567}

	taskObj := &taskentity.ObservabilityTask{CreatedBy: "42"}

	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "100"})
	s := proc.getSession(ctx, taskObj)
	assert.EqualValues(t, 100, *s.UserID)
	assert.EqualValues(t, 567, *s.AppID)

	s = proc.getSession(context.Background(), taskObj)
	assert.EqualValues(t, 42, *s.UserID)
}

func TestAutoEvaluateProcessor_OnTaskUpdated_InvalidStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	proc := &AutoEvaluateProcessor{}

	taskObj := &taskentity.ObservabilityTask{TaskStatus: taskentity.TaskStatusRunning}

	err := proc.OnTaskUpdated(ctx, taskObj, "invalid_status")
	assert.Error(t, err)
}

func TestAutoEvaluateProcessor_OnTaskFinished_NoAutoEvalConfig(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	repoAdapter := &taskRepoMockAdapter{MockITaskRepo: repoMock}
	evalAdapter := &fakeEvaluationAdapter{}

	proc := &AutoEvaluateProcessor{
		evaluationSvc: evalAdapter,
		taskRepo:      repoAdapter,
	}

	taskObj := &taskentity.ObservabilityTask{TaskStatus: taskentity.TaskStatusRunning, WorkspaceID: 123}
	taskRun := &taskentity.TaskRun{TaskRunConfig: nil} // No auto eval config

	// Mock the UpdateTask call
	repoMock.EXPECT().UpdateTask(gomock.Any(), taskObj).Return(nil)

	err := proc.OnTaskFinished(context.Background(), taskexe.OnTaskFinishedReq{
		Task:     taskObj,
		TaskRun:  taskRun,
		IsFinish: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, taskentity.TaskStatusSuccess, taskObj.TaskStatus)
}

func TestAutoEvaluateProcessor_NewAutoEvaluateProcessor(t *testing.T) {
	t.Parallel()

	// Create mock dependencies
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	datasetServiceAdaptor := service.NewDatasetServiceAdaptor()
	evalMock := &fakeEvaluatorAdapter{}
	evaluationMock := &fakeEvaluationAdapter{}
	taskRepo := repomocks.NewMockITaskRepo(ctrl)

	// Test constructor
	proc := NewAutoEvaluateProcessor(123, datasetServiceAdaptor, evalMock, evaluationMock, taskRepo, &EvalTargetBuilderImpl{})

	assert.NotNil(t, proc)
	assert.Equal(t, int32(123), proc.aid)
	assert.Equal(t, datasetServiceAdaptor, proc.datasetServiceAdaptor)
	assert.Equal(t, evalMock, proc.evalSvc)
	assert.Equal(t, evaluationMock, proc.evaluationSvc)
	assert.Equal(t, taskRepo, proc.taskRepo)
}

// fakeEvaluatorAdapter 是 IEvaluatorRPCAdapter 的fake实现
type fakeEvaluatorAdapter struct {
	err     error
	resp    []*rpc.Evaluator
	respMap map[int64]*rpc.Evaluator
}

func (f *fakeEvaluatorAdapter) BatchGetEvaluatorVersions(ctx context.Context, param *rpc.BatchGetEvaluatorVersionsParam) ([]*rpc.Evaluator, map[int64]*rpc.Evaluator, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	if f.resp != nil || f.respMap != nil {
		return f.resp, f.respMap, nil
	}
	return nil, nil, nil
}

func (f *fakeEvaluatorAdapter) UpdateEvaluatorRecord(ctx context.Context, param *rpc.UpdateEvaluatorRecordParam) error {
	return nil
}

func (f *fakeEvaluatorAdapter) ListEvaluators(ctx context.Context, param *rpc.ListEvaluatorsParam) ([]*rpc.Evaluator, error) {
	return nil, nil
}

// fakeEvaluationAdapter 是 IEvaluationRPCAdapter 的fake实现
type fakeEvaluationAdapter struct {
	submitResp struct {
		exptID    int64
		exptRunID int64
		err       error
	}
	invokeResp struct {
		addedItems int64
		err        error
	}
	finishResp struct {
		err error
	}
	// capture last invoke param for assertions
	lastInvoke *rpc.InvokeExperimentReq
}

func (f *fakeEvaluationAdapter) SubmitExperiment(ctx context.Context, param *rpc.SubmitExperimentReq) (exptID, exptRunID int64, err error) {
	return f.submitResp.exptID, f.submitResp.exptRunID, f.submitResp.err
}

func (f *fakeEvaluationAdapter) InvokeExperiment(ctx context.Context, param *rpc.InvokeExperimentReq) (addedItems int64, err error) {
	f.lastInvoke = param
	return f.invokeResp.addedItems, f.invokeResp.err
}

func (f *fakeEvaluationAdapter) FinishExperiment(ctx context.Context, param *rpc.FinishExperimentReq) error {
	return f.finishResp.err
}
