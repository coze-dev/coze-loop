// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	taskentity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	traceentity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type fakeEvaluatorAdapter struct {
	resp []*rpc.Evaluator
	err  error
}

func (f *fakeEvaluatorAdapter) BatchGetEvaluatorVersions(ctx context.Context, param *rpc.BatchGetEvaluatorVersionsParam) ([]*rpc.Evaluator, map[int64]*rpc.Evaluator, error) {
	result := make(map[int64]*rpc.Evaluator)
	for _, item := range f.resp {
		result[item.EvaluatorVersionID] = item
	}
	return f.resp, result, f.err
}

func (f *fakeEvaluatorAdapter) UpdateEvaluatorRecord(context.Context, *rpc.UpdateEvaluatorRecordParam) error {
	return nil
}

func (f *fakeEvaluatorAdapter) ListEvaluators(context.Context, *rpc.ListEvaluatorsParam) ([]*rpc.Evaluator, error) {
	return nil, nil
}

type fakeEvaluationAdapter struct {
	submitResp struct {
		exptID    int64
		exptRunID int64
		err       error
	}
	invokeResp struct {
		added int64
		err   error
	}
	finishErr error

	submitReq *rpc.SubmitExperimentReq
	invokeReq *rpc.InvokeExperimentReq
	finishReq *rpc.FinishExperimentReq
}

func (f *fakeEvaluationAdapter) SubmitExperiment(ctx context.Context, param *rpc.SubmitExperimentReq) (int64, int64, error) {
	f.submitReq = param
	return f.submitResp.exptID, f.submitResp.exptRunID, f.submitResp.err
}

func (f *fakeEvaluationAdapter) InvokeExperiment(ctx context.Context, param *rpc.InvokeExperimentReq) (int64, error) {
	f.invokeReq = param
	return f.invokeResp.added, f.invokeResp.err
}

func (f *fakeEvaluationAdapter) FinishExperiment(ctx context.Context, param *rpc.FinishExperimentReq) error {
	f.finishReq = param
	return f.finishErr
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
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  task.TaskStatusUnstarted,
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
			CycleTimeUnit: task.TimeUnitDay,
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
	fieldSchemas := []*eval_set.FieldSchema{
		{
			Key:         gptr.Of(fieldName),
			Name:        gptr.Of(fieldName),
			ContentType: gptr.Of(contentType),
		},
	}
	bytes, err := json.Marshal(fieldSchemas)
	if err != nil {
		t.Fatalf("marshal schema failed: %v", err)
	}
	return string(bytes)
}

func TestAutoEvaluteProcessor_ValidateConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	validTask := buildTestTask(t)
	validTask.EffectiveTime.StartAt = time.Now().Add(30 * time.Minute).UnixMilli()
	validTask.EffectiveTime.EndAt = time.Now().Add(2 * time.Hour).UnixMilli()

	cases := []struct {
		name      string
		config    any
		adapter   *fakeEvaluatorAdapter
		expectErr func(error) bool
	}{
		{
			name:   "invalid type",
			config: "bad",
			expectErr: func(err error) bool {
				return errors.Is(err, taskexe.ErrInvalidConfig)
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
			adapter: &fakeEvaluatorAdapter{resp: []*rpc.Evaluator{}},
			expectErr: func(err error) bool {
				status, ok := errorx.FromStatusError(err)
				return ok && status.Code() == obErrorx.CommonInvalidParamCode
			},
		},
		{
			name:      "success",
			config:    validTask,
			adapter:   &fakeEvaluatorAdapter{resp: []*rpc.Evaluator{{EvaluatorVersionID: 111}}},
			expectErr: func(err error) bool { return err == nil },
		},
	}

	for _, tt := range cases {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			proc := &AutoEvaluteProcessor{evalSvc: caseItem.adapter}
			if caseItem.adapter == nil {
				proc.evalSvc = &fakeEvaluatorAdapter{}
			}
			err := proc.ValidateConfig(ctx, caseItem.config)
			assert.True(t, caseItem.expectErr(err))
		})
	}
}

func TestAutoEvaluteProcessor_Invoke(t *testing.T) {
	t.Parallel()

	textSchema := makeSchemaJSON(t, "field_1", common.ContentTypeText)
	multiSchema := makeSchemaJSON(t, "field_1", common.ContentTypeMultiPart)

	buildTrigger := func(taskObj *taskentity.ObservabilityTask, schemaStr string) *taskexe.Trigger {
		taskRun := &taskentity.TaskRun{
			ID:            1001,
			TaskID:        taskObj.ID,
			WorkspaceID:   taskObj.WorkspaceID,
			TaskType:      task.TaskRunTypeNewData,
			RunStatus:     task.RunStatusRunning,
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
		proc := &AutoEvaluteProcessor{
			evaluationSvc: &fakeEvaluationAdapter{},
			taskRepo:      repoMock,
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
		repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(2), nil)
		repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(2), nil)
		repoMock.EXPECT().DecrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)

		proc := &AutoEvaluteProcessor{
			evaluationSvc: &fakeEvaluationAdapter{},
			taskRepo:      repoMock,
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
		repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(1), nil)
		repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(1), nil)
		repoMock.EXPECT().DecrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)

		eval := &fakeEvaluationAdapter{}
		eval.invokeResp.err = errors.New("invoke fail")

		proc := &AutoEvaluteProcessor{
			evaluationSvc: eval,
			taskRepo:      repoMock,
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
		repoMock.EXPECT().IncrTaskCount(gomock.Any(), taskObj.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().IncrTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID, gomock.Any()).Return(nil)
		repoMock.EXPECT().GetTaskCount(gomock.Any(), taskObj.ID).Return(int64(1), nil)
		repoMock.EXPECT().GetTaskRunCount(gomock.Any(), taskObj.ID, trigger.TaskRun.ID).Return(int64(1), nil)

		eval := &fakeEvaluationAdapter{}
		repoMock.EXPECT().DecrTaskCount(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		repoMock.EXPECT().DecrTaskRunCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		proc := &AutoEvaluteProcessor{
			evaluationSvc: eval,
			taskRepo:      repoMock,
		}
		err := proc.Invoke(context.Background(), trigger)
		assert.NoError(t, err)
		assert.NotNil(t, eval.invokeReq)
	})
}

func TestAutoEvaluteProcessor_OnUpdateTaskChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name    string
		initial string
		op      task.TaskStatus
		expect  string
	}{
		{"success", task.TaskStatusRunning, task.TaskStatusSuccess, task.TaskStatusSuccess},
		{"running", task.TaskStatusPending, task.TaskStatusRunning, task.TaskStatusRunning},
		{"disable", task.TaskStatusRunning, task.TaskStatusDisabled, task.TaskStatusDisabled},
		{"pending", task.TaskStatusUnstarted, task.TaskStatusPending, task.TaskStatusPending},
	}

	for _, tt := range cases {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repoMock := repomocks.NewMockITaskRepo(ctrl)
			repoMock.EXPECT().UpdateTask(gomock.Any(), gomock.AssignableToTypeOf(&taskentity.ObservabilityTask{})).DoAndReturn(
				func(_ context.Context, taskObj *taskentity.ObservabilityTask) error {
					assert.Equal(t, caseItem.expect, taskObj.TaskStatus)
					return nil
				})

			proc := &AutoEvaluteProcessor{taskRepo: repoMock}
			taskObj := &taskentity.ObservabilityTask{TaskStatus: caseItem.initial}
			err := proc.OnUpdateTaskChange(ctx, taskObj, caseItem.op)
			assert.NoError(t, err)
		})
	}

	t.Run("invalid op", func(t *testing.T) {
		proc := &AutoEvaluteProcessor{}
		err := proc.OnUpdateTaskChange(ctx, &taskentity.ObservabilityTask{}, "unknown")
		assert.Error(t, err)
	})
}

func TestAutoEvaluteProcessor_OnCreateTaskRunChange(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	datasetProvider := rpcmock.NewMockIDatasetProvider(ctrl)
	repoMock := repomocks.NewMockITaskRepo(ctrl)

	taskObj := buildTestTask(t)
	param := taskexe.OnCreateTaskRunChangeReq{
		CurrentTask: taskObj,
		RunType:     task.TaskRunTypeNewData,
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

	proc := &AutoEvaluteProcessor{
		datasetServiceAdaptor: adaptor,
		evaluationSvc:         evalAdapter,
		taskRepo:              repoMock,
		aid:                   321,
	}

	ctx := session.WithCtxUser(context.Background(), &session.User{ID: taskObj.CreatedBy})
	err := proc.OnCreateTaskRunChange(ctx, param)
	assert.NoError(t, err)
	assert.NotNil(t, evalAdapter.submitReq)
	assert.Equal(t, int64(9001), *evalAdapter.submitReq.EvalSetID)
}

func TestAutoEvaluteProcessor_OnFinishTaskRunChange(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	evalAdapter := &fakeEvaluationAdapter{}

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

	proc := &AutoEvaluteProcessor{
		taskRepo:      repoMock,
		evaluationSvc: evalAdapter,
	}

	err := proc.OnFinishTaskRunChange(context.Background(), taskexe.OnFinishTaskRunChangeReq{
		Task:    &taskentity.ObservabilityTask{WorkspaceID: 1234},
		TaskRun: taskRun,
	})
	assert.NoError(t, err)
	assert.NotNil(t, evalAdapter.finishReq)
	assert.Equal(t, task.RunStatusDone, taskRun.RunStatus)
}

func TestAutoEvaluteProcessor_OnFinishTaskChange(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITaskRepo(ctrl)
	evalAdapter := &fakeEvaluationAdapter{}

	taskObj := &taskentity.ObservabilityTask{TaskStatus: task.TaskStatusRunning, WorkspaceID: 123}
	taskRun := &taskentity.TaskRun{TaskRunConfig: &taskentity.TaskRunConfig{AutoEvaluateRunConfig: &taskentity.AutoEvaluateRunConfig{ExptID: 1, ExptRunID: 2}}}

	repoMock.EXPECT().UpdateTaskRun(gomock.Any(), gomock.Any()).Return(nil)
	repoMock.EXPECT().UpdateTask(gomock.Any(), taskObj).Return(nil)

	proc := &AutoEvaluteProcessor{
		evaluationSvc: evalAdapter,
		taskRepo:      repoMock,
	}

	err := proc.OnFinishTaskChange(context.Background(), taskexe.OnFinishTaskChangeReq{
		Task:     taskObj,
		TaskRun:  taskRun,
		IsFinish: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, task.TaskStatusSuccess, taskObj.TaskStatus)
}

func TestAutoEvaluteProcessor_getSession(t *testing.T) {
	t.Parallel()
	proc := &AutoEvaluteProcessor{aid: 567}

	taskObj := &taskentity.ObservabilityTask{CreatedBy: "42"}

	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "100"})
	s := proc.getSession(ctx, taskObj)
	assert.EqualValues(t, 100, *s.UserID)
	assert.EqualValues(t, 567, *s.AppID)

	s = proc.getSession(context.Background(), taskObj)
	assert.EqualValues(t, 42, *s.UserID)
}
