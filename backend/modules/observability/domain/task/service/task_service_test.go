// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	componentmq "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/storage"
	storagemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/storage/mocks"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	taskrepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	buildermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	spanfiltermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// fakeProcessor 是 taskexe.Processor 的fake实现
type fakeProcessor struct {
	validateErr          error
	onTaskCreatedErr     error
	onTaskRunFinishedErr error
}

// fakeBackfillProducer 是 mq.IBackfillProducer 的fake实现
type fakeBackfillProducer struct {
	sendBackfillFunc func(ctx context.Context, message *entity.BackFillEvent) error
}

func (f *fakeBackfillProducer) SendBackfill(ctx context.Context, message *entity.BackFillEvent) error {
	if f.sendBackfillFunc != nil {
		return f.sendBackfillFunc(ctx, message)
	}
	return nil
}

func (f *fakeProcessor) ValidateConfig(ctx context.Context, config any) error {
	return f.validateErr
}

func (f *fakeProcessor) Invoke(ctx context.Context, trigger *taskexe.Trigger) error {
	return nil
}

func (f *fakeProcessor) OnTaskCreated(ctx context.Context, currentTask *entity.ObservabilityTask) error {
	return f.onTaskCreatedErr
}

func (f *fakeProcessor) OnTaskUpdated(ctx context.Context, currentTask *entity.ObservabilityTask, taskOp entity.TaskStatus) error {
	return nil
}

func (f *fakeProcessor) OnTaskFinished(ctx context.Context, param taskexe.OnTaskFinishedReq) error {
	return nil
}

func (f *fakeProcessor) OnTaskRunCreated(ctx context.Context, param taskexe.OnTaskRunCreatedReq) error {
	return nil
}

func (f *fakeProcessor) OnTaskRunFinished(ctx context.Context, param taskexe.OnTaskRunFinishedReq) error {
	return f.onTaskRunFinishedErr
}

func newTaskServiceWithProcessor(t *testing.T, ctrl *gomock.Controller, repo taskrepo.ITaskRepo, backfill componentmq.IBackfillProducer, proc taskexe.Processor, taskType entity.TaskType) ITaskService {
	t.Helper()
	tp := processor.NewTaskProcessor()
	tp.Register(taskType, proc)
	// 创建mock依赖
	idGeneratorMock := idgenmocks.NewMockIIDGenerator(ctrl)
	storageProviderMock := storagemocks.NewMockIStorageProvider(ctrl)
	tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
	buildHelperMock := buildermocks.NewMockTraceFilterProcessorBuilder(ctrl)

	// 设置mock期望
	idGeneratorMock.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil).AnyTimes()
	idGeneratorMock.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, counts int) ([]int64, error) {
		ids := make([]int64, counts)
		for i := 0; i < counts; i++ {
			ids[i] = int64(1001 + i)
		}
		return ids, nil
	}).AnyTimes()

	storageProviderMock.EXPECT().GetTraceStorage(gomock.Any(), gomock.Any(), gomock.Any()).Return(storage.Storage{
		StorageName:   "ck",
		StorageConfig: map[string]string{},
	}).AnyTimes()
	storageProviderMock.EXPECT().PrepareStorageForTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	tenantProviderMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant").AnyTimes()
	tenantProviderMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"test-tenant"}).AnyTimes()
	tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"test-tenant"}, nil).AnyTimes()
	tenantProviderMock.EXPECT().GetMetricTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"test-tenant"}, nil).AnyTimes()

	// 创建mock过滤器并设置期望
	mockFilter := spanfiltermocks.NewMockFilter(ctrl)
	// 返回有效的基本过滤器，避免权限错误
	mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{FieldName: "test"}}, true, nil).AnyTimes()
	mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, nil).AnyTimes()

	buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()
	buildHelperMock.EXPECT().BuildGetTraceProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor(nil), nil).AnyTimes()
	buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor(nil), nil).AnyTimes()
	buildHelperMock.EXPECT().BuildAdvanceInfoProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor(nil), nil).AnyTimes()
	buildHelperMock.EXPECT().BuildIngestTraceProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor(nil), nil).AnyTimes()
	buildHelperMock.EXPECT().BuildSearchTraceOApiProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor(nil), nil).AnyTimes()
	buildHelperMock.EXPECT().BuildListSpansOApiProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor(nil), nil).AnyTimes()

	service, err := NewTaskServiceImpl(repo, idGeneratorMock, backfill, tp, storageProviderMock, tenantProviderMock, buildHelperMock)
	assert.NoError(t, err)
	return service
}

func TestTaskServiceImpl_CreateTask(t *testing.T) {
	t.Parallel()

	t.Run("success with backfill", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		repoMock.EXPECT().CreateTask(gomock.Any(), gomock.AssignableToTypeOf(&entity.ObservabilityTask{})).DoAndReturn(func(ctx context.Context, taskDO *entity.ObservabilityTask) (int64, error) {
			return 1001, nil
		})
		repoMock.EXPECT().DeleteTask(gomock.Any(), gomock.Any()).Times(0)

		// 使用fake processor
		procMock := &fakeProcessor{}

		backfillCh := make(chan *entity.BackFillEvent, 1)
		backfillMock := &fakeBackfillProducer{
			sendBackfillFunc: func(ctx context.Context, event *entity.BackFillEvent) error {
				backfillCh <- event
				return nil
			},
		}

		svc := newTaskServiceWithProcessor(t, ctrl, repoMock, backfillMock, procMock, entity.TaskTypeAutoEval)

		reqTask := &entity.ObservabilityTask{
			WorkspaceID: 123,
			Name:        "task",
			TaskType:    entity.TaskTypeAutoEval,
			TaskStatus:  entity.TaskStatusUnstarted,
			SpanFilter: &entity.SpanFilterFields{
				PlatformType: loop_span.PlatformDefault, // 设置有效的平台类型
			},
			BackfillEffectiveTime: &entity.EffectiveTime{StartAt: time.Now().Add(time.Second).UnixMilli(), EndAt: time.Now().Add(2 * time.Second).UnixMilli()},
			Sampler:               &entity.Sampler{},
			EffectiveTime:         &entity.EffectiveTime{StartAt: time.Now().Add(time.Second).UnixMilli(), EndAt: time.Now().Add(2 * time.Second).UnixMilli()},
		}
		resp, err := svc.CreateTask(context.Background(), &CreateTaskReq{Task: reqTask})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			assert.Equal(t, int64(1001), *resp.TaskID)
		}

		select {
		case event := <-backfillCh:
			assert.Equal(t, reqTask.WorkspaceID, event.SpaceID)
			assert.Equal(t, int64(1001), event.TaskID)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected backfill event")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)

		// 使用fake processor
		procMock := &fakeProcessor{validateErr: errors.New("invalid config")}

		svc := newTaskServiceWithProcessor(t, ctrl, repoMock, nil, procMock, entity.TaskTypeAutoEval)

		reqTask := &entity.ObservabilityTask{WorkspaceID: 1, Name: "task", TaskType: entity.TaskTypeAutoEval, Sampler: &entity.Sampler{}, EffectiveTime: &entity.EffectiveTime{}, SpanFilter: &entity.SpanFilterFields{}}
		resp, err := svc.CreateTask(context.Background(), &CreateTaskReq{Task: reqTask})
		assert.Nil(t, resp)
		assert.Error(t, err)
		statusErr, ok := errorx.FromStatusError(err)
		if assert.True(t, ok) {
			assert.EqualValues(t, obErrorx.CommonInvalidParamCode, statusErr.Code())
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{{}}, int64(1), nil)

		// 使用fake processor
		procMock := &fakeProcessor{}

		svc := newTaskServiceWithProcessor(t, ctrl, repoMock, nil, procMock, entity.TaskTypeAutoEval)
		reqTask := &entity.ObservabilityTask{WorkspaceID: 1, Name: "task", TaskType: entity.TaskTypeAutoEval, Sampler: &entity.Sampler{}, EffectiveTime: &entity.EffectiveTime{}, SpanFilter: &entity.SpanFilterFields{}}
		resp, err := svc.CreateTask(context.Background(), &CreateTaskReq{Task: reqTask})
		assert.Nil(t, resp)
		assert.Error(t, err)
		statusErr, ok := errorx.FromStatusError(err)
		if assert.True(t, ok) {
			assert.EqualValues(t, obErrorx.CommonInvalidParamCode, statusErr.Code())
		}
	})

	t.Run("on create hook error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		repoMock.EXPECT().CreateTask(gomock.Any(), gomock.AssignableToTypeOf(&entity.ObservabilityTask{})).Return(int64(1001), nil)
		repoMock.EXPECT().DeleteTask(gomock.Any(), gomock.AssignableToTypeOf(&entity.ObservabilityTask{})).Return(nil)

		// 使用fake processor
		procMock := &fakeProcessor{onTaskCreatedErr: errors.New("hook fail")}

		svc := newTaskServiceWithProcessor(t, ctrl, repoMock, nil, procMock, entity.TaskTypeAutoEval)
		reqTask := &entity.ObservabilityTask{
			WorkspaceID: 1,
			Name:        "task",
			TaskType:    entity.TaskTypeAutoEval,
			TaskStatus:  entity.TaskStatusUnstarted,
			SpanFilter: &entity.SpanFilterFields{
				PlatformType: loop_span.PlatformDefault, // 设置有效的平台类型
			},
			Sampler:       &entity.Sampler{},
			EffectiveTime: &entity.EffectiveTime{},
		}
		resp, err := svc.CreateTask(context.Background(), &CreateTaskReq{Task: reqTask})
		assert.Nil(t, resp)
		assert.EqualError(t, err, "hook fail")
	})
}

func TestTaskServiceImpl_UpdateTask(t *testing.T) {
	t.Parallel()

	t.Run("repo error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(nil, errors.New("repo fail"))

		svc := &TaskServiceImpl{TaskRepo: repoMock}
		err := svc.UpdateTask(context.Background(), &UpdateTaskReq{TaskID: 1, WorkspaceID: 2})
		assert.EqualError(t, err, "repo fail")
	})

	t.Run("task not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(nil, nil)

		svc := &TaskServiceImpl{TaskRepo: repoMock}
		err := svc.UpdateTask(context.Background(), &UpdateTaskReq{TaskID: 1, WorkspaceID: 2})
		statusErr, ok := errorx.FromStatusError(err)
		if assert.True(t, ok) {
			assert.EqualValues(t, obErrorx.CommercialCommonInvalidParamCodeCode, statusErr.Code())
		}
	})

	t.Run("user parse failed", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		taskDO := &entity.ObservabilityTask{TaskType: entity.TaskTypeAutoEval, TaskStatus: entity.TaskStatusUnstarted, EffectiveTime: &entity.EffectiveTime{}, Sampler: &entity.Sampler{}}
		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)

		// 使用fake processor
		procMock := &fakeProcessor{}

		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, procMock)
		svc := &TaskServiceImpl{TaskRepo: repoMock, taskProcessor: *tp}

		err := svc.UpdateTask(context.Background(), &UpdateTaskReq{TaskID: 1, WorkspaceID: 2})
		statusErr, ok := errorx.FromStatusError(err)
		if assert.True(t, ok) {
			assert.EqualValues(t, obErrorx.UserParseFailedCode, statusErr.Code())
		}
	})

	t.Run("disable success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		startAt := time.Now().Add(2 * time.Hour).UnixMilli()
		repoMock := repomocks.NewMockITaskRepo(ctrl)
		now := time.Now()
		taskDO := &entity.ObservabilityTask{
			TaskType:      entity.TaskTypeAutoEval,
			TaskStatus:    entity.TaskStatusUnstarted,
			EffectiveTime: &entity.EffectiveTime{StartAt: startAt, EndAt: startAt + 3600000},
			Sampler:       &entity.Sampler{SampleRate: 0.1},
			TaskRuns:      []*entity.TaskRun{{RunStatus: entity.TaskRunStatusRunning}},
			UpdatedAt:     now,
			UpdatedBy:     "user1",
		}

		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)
		repoMock.EXPECT().RemoveNonFinalTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		repoMock.EXPECT().UpdateTask(gomock.Any(), taskDO).Return(nil)

		// 使用fake processor
		procMock := &fakeProcessor{}

		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, procMock)
		svc := &TaskServiceImpl{TaskRepo: repoMock, taskProcessor: *tp}

		desc := "updated"
		newStart := startAt + 1000
		newEnd := startAt + 7200000
		sampleRate := 0.5
		err := svc.UpdateTask(session.WithCtxUser(context.Background(), &session.User{ID: "user1"}), &UpdateTaskReq{
			TaskID:        1,
			WorkspaceID:   2,
			Description:   &desc,
			EffectiveTime: &entity.EffectiveTime{StartAt: newStart, EndAt: newEnd},
			SampleRate:    &sampleRate,
			TaskStatus:    gptr.Of(entity.TaskStatusDisabled),
			UserID:        "user1",
		})
		assert.NoError(t, err)
		assert.Equal(t, entity.TaskStatusDisabled, taskDO.TaskStatus)
		assert.Equal(t, "user1", taskDO.UpdatedBy)
		if assert.NotNil(t, taskDO.Description) {
			assert.Equal(t, desc, *taskDO.Description)
		}
		assert.NotNil(t, taskDO.EffectiveTime)
		assert.Equal(t, newStart, taskDO.EffectiveTime.StartAt)
		assert.Equal(t, sampleRate, taskDO.Sampler.SampleRate)
	})

	t.Run("disable remove non final task error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		taskDO := &entity.ObservabilityTask{
			TaskType:      entity.TaskTypeAutoEval,
			TaskStatus:    entity.TaskStatusUnstarted,
			EffectiveTime: &entity.EffectiveTime{StartAt: time.Now().UnixMilli(), EndAt: time.Now().Add(time.Hour).UnixMilli()},
			Sampler:       &entity.Sampler{},
			TaskRuns:      []*entity.TaskRun{{RunStatus: entity.TaskRunStatusRunning}},
		}

		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)
		repoMock.EXPECT().RemoveNonFinalTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("remove fail"))
		repoMock.EXPECT().UpdateTask(gomock.Any(), taskDO).Return(nil)

		// 使用fake processor
		procMock := &fakeProcessor{}

		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, procMock)
		svc := &TaskServiceImpl{TaskRepo: repoMock, taskProcessor: *tp}

		sampleRate := 0.6
		err := svc.UpdateTask(session.WithCtxUser(context.Background(), &session.User{ID: "user"}), &UpdateTaskReq{
			TaskID:      1,
			WorkspaceID: 2,
			SampleRate:  &sampleRate,
			TaskStatus:  gptr.Of(entity.TaskStatusDisabled),
			UserID:      "user",
		})
		assert.NoError(t, err)
	})

	t.Run("finish hook error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		startAt := time.Now().Add(2 * time.Hour).UnixMilli()
		repoMock := repomocks.NewMockITaskRepo(ctrl)
		taskDO := &entity.ObservabilityTask{
			TaskType:      entity.TaskTypeAutoEval,
			TaskStatus:    entity.TaskStatusUnstarted,
			EffectiveTime: &entity.EffectiveTime{StartAt: startAt, EndAt: startAt + 3600000},
			Sampler:       &entity.Sampler{},
			TaskRuns:      []*entity.TaskRun{{RunStatus: entity.TaskRunStatusRunning}},
		}

		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)
		repoMock.EXPECT().UpdateTask(gomock.Any(), gomock.Any()).Times(0)

		// 使用fake processor
		procMock := &fakeProcessor{onTaskRunFinishedErr: errors.New("finish fail")}

		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, procMock)
		svc := &TaskServiceImpl{TaskRepo: repoMock, taskProcessor: *tp}

		newStart := startAt + 1000
		newEnd := startAt + 7200000
		sampleRate := 0.3
		err := svc.UpdateTask(session.WithCtxUser(context.Background(), &session.User{ID: "user"}), &UpdateTaskReq{
			TaskID:        1,
			WorkspaceID:   2,
			EffectiveTime: &entity.EffectiveTime{StartAt: newStart, EndAt: newEnd},
			SampleRate:    &sampleRate,
			TaskStatus:    gptr.Of(entity.TaskStatusDisabled),
			UserID:        "user",
		})
		assert.EqualError(t, err, "finish fail")
	})
}

func TestTaskServiceImpl_ListTasks(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)

		svc := &TaskServiceImpl{TaskRepo: repoMock}
		resp, err := svc.ListTasks(context.Background(), &ListTasksReq{WorkspaceID: 1})
		assert.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)

		hiddenField := &loop_span.FilterField{FieldName: "hidden", Values: []string{"1"}, Hidden: true}
		visibleField := &loop_span.FilterField{FieldName: "visible", Values: []string{"val"}}
		childVisible := &loop_span.FilterField{FieldName: "child", Values: []string{"child"}}
		childHidden := &loop_span.FilterField{FieldName: "child_hidden", Values: []string{"child_hidden"}, Hidden: true}
		parentField := &loop_span.FilterField{SubFilter: &loop_span.FilterFields{QueryAndOr: gptr.Of(loop_span.QueryAndOrEnumAnd), FilterFields: []*loop_span.FilterField{childVisible, childHidden}}}
		taskDO := &entity.ObservabilityTask{
			ID:            1,
			Name:          "task",
			WorkspaceID:   2,
			TaskType:      entity.TaskTypeAutoEval,
			TaskStatus:    entity.TaskStatusUnstarted,
			CreatedBy:     "user1",
			UpdatedBy:     "user2",
			EffectiveTime: &entity.EffectiveTime{},
			Sampler:       &entity.Sampler{},
			SpanFilter: &entity.SpanFilterFields{Filters: loop_span.FilterFields{
				QueryAndOr:   gptr.Of(loop_span.QueryAndOrEnumAnd),
				FilterFields: []*loop_span.FilterField{hiddenField, visibleField, parentField},
			}},
		}
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskDO}, int64(1), nil)

		svc := &TaskServiceImpl{TaskRepo: repoMock}
		resp, err := svc.ListTasks(context.Background(), &ListTasksReq{WorkspaceID: 2, TaskFilters: &entity.TaskFilterFields{}})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			assert.EqualValues(t, 1, resp.Total)
			assert.Len(t, resp.Tasks, 1)
			task := resp.Tasks[0]
			if assert.NotNil(t, task.SpanFilter) {
				fields := task.SpanFilter.Filters.FilterFields
				assert.Len(t, fields, 2)
				assert.Equal(t, "visible", fields[0].FieldName)
				assert.Equal(t, []string{"val"}, fields[0].Values)
				if sub := fields[1].SubFilter; assert.NotNil(t, sub) {
					subFields := sub.FilterFields
					assert.Len(t, subFields, 1)
					assert.Equal(t, "child", subFields[0].FieldName)
				}
			}
		}
	})
}

func TestTaskServiceImpl_GetTask(t *testing.T) {
	t.Parallel()

	t.Run("repo error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(nil, errors.New("repo fail"))

		svc := &TaskServiceImpl{TaskRepo: repoMock}
		resp, err := svc.GetTask(context.Background(), &GetTaskReq{TaskID: 1, WorkspaceID: 2})
		assert.Nil(t, resp)
		assert.EqualError(t, err, "repo fail")
	})

	t.Run("task nil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(nil, nil)

		svc := &TaskServiceImpl{TaskRepo: repoMock}
		resp, err := svc.GetTask(context.Background(), &GetTaskReq{TaskID: 1, WorkspaceID: 2})
		assert.Nil(t, resp)
		assert.NoError(t, err)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)

		subHidden := &loop_span.FilterField{FieldName: "inner_hidden", Values: []string{"v"}, Hidden: true}
		subVisible := &loop_span.FilterField{FieldName: "inner_visible", Values: []string{"v"}}
		parent := &loop_span.FilterField{SubFilter: &loop_span.FilterFields{QueryAndOr: gptr.Of(loop_span.QueryAndOrEnumAnd), FilterFields: []*loop_span.FilterField{subHidden, subVisible}}}
		visible := &loop_span.FilterField{FieldName: "outer_visible", Values: []string{"v"}}
		hidden := &loop_span.FilterField{FieldName: "outer_hidden", Values: []string{"v"}, Hidden: true}

		taskDO := &entity.ObservabilityTask{
			TaskType:      entity.TaskTypeAutoEval,
			TaskStatus:    entity.TaskStatusUnstarted,
			CreatedBy:     "user1",
			UpdatedBy:     "user2",
			EffectiveTime: &entity.EffectiveTime{},
			Sampler:       &entity.Sampler{},
			SpanFilter: &entity.SpanFilterFields{Filters: loop_span.FilterFields{
				QueryAndOr:   gptr.Of(loop_span.QueryAndOrEnumAnd),
				FilterFields: []*loop_span.FilterField{hidden, visible, parent},
			}},
		}

		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)

		svc := &TaskServiceImpl{TaskRepo: repoMock}
		resp, err := svc.GetTask(context.Background(), &GetTaskReq{TaskID: 1, WorkspaceID: 2})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			task := resp.Task
			if assert.NotNil(t, task.SpanFilter) {
				fields := task.SpanFilter.Filters.FilterFields
				assert.Len(t, fields, 2)
				assert.Equal(t, "outer_visible", fields[0].FieldName)
				if sub := fields[1].SubFilter; assert.NotNil(t, sub) {
					subFields := sub.FilterFields
					assert.Len(t, subFields, 1)
					assert.Equal(t, "inner_visible", subFields[0].FieldName)
				}
			}
		}
	})
}

func TestTaskServiceImpl_CheckTaskName(t *testing.T) {
	t.Parallel()

	t.Run("repo error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("repo fail"))

		// 使用mock依赖创建服务
		idGeneratorMock := idgenmocks.NewMockIIDGenerator(ctrl)
		storageProviderMock := storagemocks.NewMockIStorageProvider(ctrl)
		tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
		buildHelperMock := buildermocks.NewMockTraceFilterProcessorBuilder(ctrl)

		// 设置基本mock期望
		idGeneratorMock.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil).AnyTimes()
		storageProviderMock.EXPECT().GetTraceStorage(gomock.Any(), gomock.Any(), gomock.Any()).Return(storage.Storage{
			StorageName:   "ck",
			StorageConfig: map[string]string{},
		}).AnyTimes()
		tenantProviderMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant").AnyTimes()
		// 创建mock过滤器并设置期望
		mockFilter := spanfiltermocks.NewMockFilter(ctrl)
		// 返回有效的基本过滤器，避免权限错误
		mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{FieldName: "test"}}, true, nil).AnyTimes()
		mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, nil).AnyTimes()

		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()

		svc, err := NewTaskServiceImpl(
			repoMock,
			idGeneratorMock,
			nil,
			processor.NewTaskProcessor(),
			storageProviderMock,
			tenantProviderMock,
			buildHelperMock,
		)
		assert.NoError(t, err)
		resp, err := svc.CheckTaskName(context.Background(), &CheckTaskNameReq{WorkspaceID: 1, Name: "task"})
		assert.Nil(t, resp)
		assert.EqualError(t, err, "repo fail")
	})

	t.Run("duplicate", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{{}}, int64(1), nil)

		// 使用mock依赖创建服务
		idGeneratorMock := idgenmocks.NewMockIIDGenerator(ctrl)
		storageProviderMock := storagemocks.NewMockIStorageProvider(ctrl)
		tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
		buildHelperMock := buildermocks.NewMockTraceFilterProcessorBuilder(ctrl)

		// 设置基本mock期望
		idGeneratorMock.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil).AnyTimes()
		storageProviderMock.EXPECT().GetTraceStorage(gomock.Any(), gomock.Any(), gomock.Any()).Return(storage.Storage{
			StorageName:   "ck",
			StorageConfig: map[string]string{},
		}).AnyTimes()
		tenantProviderMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant").AnyTimes()
		// 创建mock过滤器并设置期望
		mockFilter := spanfiltermocks.NewMockFilter(ctrl)
		// 返回有效的基本过滤器，避免权限错误
		mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{FieldName: "test"}}, true, nil).AnyTimes()
		mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, nil).AnyTimes()

		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()

		svc, err := NewTaskServiceImpl(
			repoMock,
			idGeneratorMock,
			nil,
			processor.NewTaskProcessor(),
			storageProviderMock,
			tenantProviderMock,
			buildHelperMock,
		)
		assert.NoError(t, err)
		resp, err := svc.CheckTaskName(context.Background(), &CheckTaskNameReq{WorkspaceID: 1, Name: "task"})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			assert.False(t, *resp.Pass)
		}
	})

	t.Run("available", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)

		// 使用mock依赖创建服务
		idGeneratorMock := idgenmocks.NewMockIIDGenerator(ctrl)
		storageProviderMock := storagemocks.NewMockIStorageProvider(ctrl)
		tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
		buildHelperMock := buildermocks.NewMockTraceFilterProcessorBuilder(ctrl)

		// 设置基本mock期望
		idGeneratorMock.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil).AnyTimes()
		storageProviderMock.EXPECT().GetTraceStorage(gomock.Any(), gomock.Any(), gomock.Any()).Return(storage.Storage{
			StorageName:   "ck",
			StorageConfig: map[string]string{},
		}).AnyTimes()
		tenantProviderMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant").AnyTimes()
		// 创建mock过滤器并设置期望
		mockFilter := spanfiltermocks.NewMockFilter(ctrl)
		// 返回有效的基本过滤器，避免权限错误
		mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{FieldName: "test"}}, true, nil).AnyTimes()
		mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, nil).AnyTimes()

		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()

		svc, err := NewTaskServiceImpl(
			repoMock,
			idGeneratorMock,
			nil,
			processor.NewTaskProcessor(),
			storageProviderMock,
			tenantProviderMock,
			buildHelperMock,
		)
		assert.NoError(t, err)
		resp, err := svc.CheckTaskName(context.Background(), &CheckTaskNameReq{WorkspaceID: 1, Name: "task"})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			assert.True(t, *resp.Pass)
		}
	})
}

func TestTaskServiceImpl_sendBackfillMessage(t *testing.T) {
	t.Run("producer nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// 使用mock依赖创建服务
		repoMock := repomocks.NewMockITaskRepo(ctrl)
		idGeneratorMock := idgenmocks.NewMockIIDGenerator(ctrl)
		storageProviderMock := storagemocks.NewMockIStorageProvider(ctrl)
		tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
		buildHelperMock := buildermocks.NewMockTraceFilterProcessorBuilder(ctrl)

		// 设置基本mock期望
		idGeneratorMock.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil).AnyTimes()
		storageProviderMock.EXPECT().GetTraceStorage(gomock.Any(), gomock.Any(), gomock.Any()).Return(storage.Storage{
			StorageName:   "ck",
			StorageConfig: map[string]string{},
		}).AnyTimes()
		tenantProviderMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant").AnyTimes()
		// 创建mock过滤器并设置期望
		mockFilter := spanfiltermocks.NewMockFilter(ctrl)
		// 返回有效的基本过滤器，避免权限错误
		mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{FieldName: "test"}}, true, nil).AnyTimes()
		mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, nil).AnyTimes()

		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()

		// 创建一个没有backfillProducer的服务
		service, err := NewTaskServiceImpl(
			repoMock,
			idGeneratorMock,
			nil, // backfillProducer为nil
			processor.NewTaskProcessor(),
			storageProviderMock,
			tenantProviderMock,
			buildHelperMock,
		)
		assert.NoError(t, err)
		err = service.SendBackfillMessage(context.Background(), &entity.BackFillEvent{})
		statusErr, ok := errorx.FromStatusError(err)
		if assert.True(t, ok) {
			assert.EqualValues(t, obErrorx.CommonInternalErrorCode, statusErr.Code())
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ch := make(chan *entity.BackFillEvent, 1)

		// 使用mock依赖创建服务
		repoMock := repomocks.NewMockITaskRepo(ctrl)
		idGeneratorMock := idgenmocks.NewMockIIDGenerator(ctrl)
		storageProviderMock := storagemocks.NewMockIStorageProvider(ctrl)
		tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
		buildHelperMock := buildermocks.NewMockTraceFilterProcessorBuilder(ctrl)
		backfillMock := &fakeBackfillProducer{
			sendBackfillFunc: func(ctx context.Context, event *entity.BackFillEvent) error {
				ch <- event
				return nil
			},
		}

		// 设置基本mock期望
		idGeneratorMock.EXPECT().GenID(gomock.Any()).Return(int64(1001), nil).AnyTimes()
		storageProviderMock.EXPECT().GetTraceStorage(gomock.Any(), gomock.Any(), gomock.Any()).Return(storage.Storage{
			StorageName:   "ck",
			StorageConfig: map[string]string{},
		}).AnyTimes()
		tenantProviderMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant").AnyTimes()
		// 创建mock过滤器并设置期望
		mockFilter := spanfiltermocks.NewMockFilter(ctrl)
		// 返回有效的基本过滤器，避免权限错误
		mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{FieldName: "test"}}, true, nil).AnyTimes()
		mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, nil).AnyTimes()

		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()

		// backfillMock是fake实现，不需要EXPECT

		service, err := NewTaskServiceImpl(
			repoMock,
			idGeneratorMock,
			backfillMock,
			processor.NewTaskProcessor(),
			storageProviderMock,
			tenantProviderMock,
			buildHelperMock,
		)
		assert.NoError(t, err)
		err = service.SendBackfillMessage(context.Background(), &entity.BackFillEvent{TaskID: 1})
		assert.NoError(t, err)
		select {
		case event := <-ch:
			assert.Equal(t, int64(1), event.TaskID)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected send backfill message")
		}
	})
}
