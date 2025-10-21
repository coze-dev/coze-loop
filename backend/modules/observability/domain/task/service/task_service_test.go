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

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	componentmq "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	rpc "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	taskrepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	entitycommon "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type fakeProcessor struct {
	validateErr       error
	onCreateErr       error
	onFinishRunErr    error
	onCreateCalled    bool
	onFinishRunCalled bool
}

func (f *fakeProcessor) ValidateConfig(context.Context, any) error {
	return f.validateErr
}

func (f *fakeProcessor) Invoke(context.Context, *taskexe.Trigger) error {
	return nil
}

func (f *fakeProcessor) OnCreateTaskChange(context.Context, *entity.ObservabilityTask) error {
	f.onCreateCalled = true
	return f.onCreateErr
}

func (f *fakeProcessor) OnUpdateTaskChange(context.Context, *entity.ObservabilityTask, task.TaskStatus) error {
	return nil
}

func (f *fakeProcessor) OnFinishTaskChange(context.Context, taskexe.OnFinishTaskChangeReq) error {
	return nil
}

func (f *fakeProcessor) OnCreateTaskRunChange(context.Context, taskexe.OnCreateTaskRunChangeReq) error {
	return nil
}

func (f *fakeProcessor) OnFinishTaskRunChange(context.Context, taskexe.OnFinishTaskRunChangeReq) error {
	f.onFinishRunCalled = true
	return f.onFinishRunErr
}

type stubBackfillProducer struct {
	ch  chan *entity.BackFillEvent
	err error
}

func (s *stubBackfillProducer) SendBackfill(ctx context.Context, message *entity.BackFillEvent) error {
	if s.ch != nil {
		s.ch <- message
	}
	return s.err
}

func newTaskServiceWithProcessor(t *testing.T, repo taskrepo.ITaskRepo, userProvider rpc.IUserProvider, backfill componentmq.IBackfillProducer, proc taskexe.Processor, taskType task.TaskType) *TaskServiceImpl {
	t.Helper()
	tp := processor.NewTaskProcessor()
	tp.Register(taskType, proc)
	service, err := NewTaskServiceImpl(repo, userProvider, nil, backfill, tp)
	assert.NoError(t, err)
	return service.(*TaskServiceImpl)
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

		proc := &fakeProcessor{}
		backfillCh := make(chan *entity.BackFillEvent, 1)
		backfill := &stubBackfillProducer{ch: backfillCh}

		svc := newTaskServiceWithProcessor(t, repoMock, nil, backfill, proc, task.TaskTypeAutoEval)

		reqTask := &entity.ObservabilityTask{
			WorkspaceID:           123,
			Name:                  "task",
			TaskType:              task.TaskTypeAutoEval,
			TaskStatus:            task.TaskStatusUnstarted,
			BackfillEffectiveTime: &entity.EffectiveTime{StartAt: time.Now().Add(time.Second).UnixMilli(), EndAt: time.Now().Add(2 * time.Second).UnixMilli()},
			Sampler:               &entity.Sampler{},
			EffectiveTime:         &entity.EffectiveTime{StartAt: time.Now().Add(time.Second).UnixMilli(), EndAt: time.Now().Add(2 * time.Second).UnixMilli()},
		}
		resp, err := svc.CreateTask(context.Background(), &CreateTaskReq{Task: reqTask})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			assert.Equal(t, int64(1001), *resp.TaskID)
		}
		assert.True(t, proc.onCreateCalled)

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

		proc := &fakeProcessor{validateErr: errors.New("invalid config")}
		svc := newTaskServiceWithProcessor(t, repoMock, nil, nil, proc, task.TaskTypeAutoEval)

		reqTask := &entity.ObservabilityTask{WorkspaceID: 1, Name: "task", TaskType: task.TaskTypeAutoEval, Sampler: &entity.Sampler{}, EffectiveTime: &entity.EffectiveTime{}}
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

		proc := &fakeProcessor{}
		svc := newTaskServiceWithProcessor(t, repoMock, nil, nil, proc, task.TaskTypeAutoEval)
		reqTask := &entity.ObservabilityTask{WorkspaceID: 1, Name: "task", TaskType: task.TaskTypeAutoEval, Sampler: &entity.Sampler{}, EffectiveTime: &entity.EffectiveTime{}}
		resp, err := svc.CreateTask(context.Background(), &CreateTaskReq{Task: reqTask})
		assert.Nil(t, resp)
		assert.Error(t, err)
		statusErr, ok := errorx.FromStatusError(err)
		if assert.True(t, ok) {
			assert.EqualValues(t, obErrorx.CommonInvalidParamCode, statusErr.Code())
		}
		assert.False(t, proc.onCreateCalled)
	})

	t.Run("on create hook error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockITaskRepo(ctrl)
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)
		repoMock.EXPECT().CreateTask(gomock.Any(), gomock.AssignableToTypeOf(&entity.ObservabilityTask{})).Return(int64(1001), nil)
		repoMock.EXPECT().DeleteTask(gomock.Any(), gomock.AssignableToTypeOf(&entity.ObservabilityTask{})).Return(nil)

		proc := &fakeProcessor{onCreateErr: errors.New("hook fail")}
		svc := newTaskServiceWithProcessor(t, repoMock, nil, nil, proc, task.TaskTypeAutoEval)
		reqTask := &entity.ObservabilityTask{WorkspaceID: 1, Name: "task", TaskType: task.TaskTypeAutoEval, Sampler: &entity.Sampler{}, EffectiveTime: &entity.EffectiveTime{}}
		resp, err := svc.CreateTask(context.Background(), &CreateTaskReq{Task: reqTask})
		assert.Nil(t, resp)
		assert.EqualError(t, err, "hook fail")
		assert.True(t, proc.onCreateCalled)
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
		taskDO := &entity.ObservabilityTask{TaskType: task.TaskTypeAutoEval, TaskStatus: task.TaskStatusUnstarted, EffectiveTime: &entity.EffectiveTime{}, Sampler: &entity.Sampler{}}
		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)

		proc := &fakeProcessor{}
		svc := &TaskServiceImpl{TaskRepo: repoMock}
		svc.taskProcessor.Register(task.TaskTypeAutoEval, proc)

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
			TaskType:      task.TaskTypeAutoEval,
			TaskStatus:    task.TaskStatusUnstarted,
			EffectiveTime: &entity.EffectiveTime{StartAt: startAt, EndAt: startAt + 3600000},
			Sampler:       &entity.Sampler{SampleRate: 0.1},
			TaskRuns:      []*entity.TaskRun{{RunStatus: task.RunStatusRunning}},
			UpdatedAt:     now,
			UpdatedBy:     "",
		}

		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)
		repoMock.EXPECT().RemoveNonFinalTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		repoMock.EXPECT().UpdateTask(gomock.Any(), taskDO).Return(nil)

		proc := &fakeProcessor{}
		svc := &TaskServiceImpl{TaskRepo: repoMock}
		svc.taskProcessor.Register(task.TaskTypeAutoEval, proc)

		desc := "updated"
		newStart := startAt + 1000
		newEnd := startAt + 7200000
		sampleRate := 0.5
		err := svc.UpdateTask(session.WithCtxUser(context.Background(), &session.User{ID: "user1"}), &UpdateTaskReq{
			TaskID:        1,
			WorkspaceID:   2,
			Description:   &desc,
			EffectiveTime: &task.EffectiveTime{StartAt: &newStart, EndAt: &newEnd},
			SampleRate:    &sampleRate,
			TaskStatus:    gptr.Of(task.TaskStatusDisabled),
		})
		assert.NoError(t, err)
		assert.True(t, proc.onFinishRunCalled)
		assert.Equal(t, task.TaskStatusDisabled, taskDO.TaskStatus)
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
			TaskType:      task.TaskTypeAutoEval,
			TaskStatus:    task.TaskStatusUnstarted,
			EffectiveTime: &entity.EffectiveTime{StartAt: time.Now().UnixMilli(), EndAt: time.Now().Add(time.Hour).UnixMilli()},
			Sampler:       &entity.Sampler{},
			TaskRuns:      []*entity.TaskRun{{RunStatus: task.RunStatusRunning}},
		}

		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)
		repoMock.EXPECT().RemoveNonFinalTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("remove fail"))
		repoMock.EXPECT().UpdateTask(gomock.Any(), taskDO).Return(nil)

		proc := &fakeProcessor{}
		svc := &TaskServiceImpl{TaskRepo: repoMock}
		svc.taskProcessor.Register(task.TaskTypeAutoEval, proc)

		sampleRate := 0.6
		err := svc.UpdateTask(session.WithCtxUser(context.Background(), &session.User{ID: "user"}), &UpdateTaskReq{
			TaskID:      1,
			WorkspaceID: 2,
			SampleRate:  &sampleRate,
			TaskStatus:  gptr.Of(task.TaskStatusDisabled),
		})
		assert.NoError(t, err)
		assert.True(t, proc.onFinishRunCalled)
	})

	t.Run("finish hook error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		startAt := time.Now().Add(2 * time.Hour).UnixMilli()
		repoMock := repomocks.NewMockITaskRepo(ctrl)
		taskDO := &entity.ObservabilityTask{
			TaskType:      task.TaskTypeAutoEval,
			TaskStatus:    task.TaskStatusUnstarted,
			EffectiveTime: &entity.EffectiveTime{StartAt: startAt, EndAt: startAt + 3600000},
			Sampler:       &entity.Sampler{},
			TaskRuns:      []*entity.TaskRun{{RunStatus: task.RunStatusRunning}},
		}

		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)
		repoMock.EXPECT().UpdateTask(gomock.Any(), gomock.Any()).Times(0)

		proc := &fakeProcessor{onFinishRunErr: errors.New("finish fail")}
		svc := &TaskServiceImpl{TaskRepo: repoMock}
		svc.taskProcessor.Register(task.TaskTypeAutoEval, proc)

		newStart := startAt + 1000
		newEnd := startAt + 7200000
		sampleRate := 0.3
		err := svc.UpdateTask(session.WithCtxUser(context.Background(), &session.User{ID: "user"}), &UpdateTaskReq{
			TaskID:        1,
			WorkspaceID:   2,
			EffectiveTime: &task.EffectiveTime{StartAt: &newStart, EndAt: &newEnd},
			SampleRate:    &sampleRate,
			TaskStatus:    gptr.Of(task.TaskStatusDisabled),
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
		userMock := rpcmock.NewMockIUserProvider(ctrl)

		hiddenField := &filter.FilterField{FieldName: gptr.Of("hidden"), Values: []string{"1"}, Hidden: gptr.Of(true)}
		visibleField := &filter.FilterField{FieldName: gptr.Of("visible"), Values: []string{"val"}}
		childVisible := &filter.FilterField{FieldName: gptr.Of("child"), Values: []string{"child"}}
		childHidden := &filter.FilterField{FieldName: gptr.Of("child_hidden"), Values: []string{"child_hidden"}, Hidden: gptr.Of(true)}
		parentField := &filter.FilterField{SubFilter: &filter.FilterFields{FilterFields: []*filter.FilterField{childVisible, childHidden}}}
		emptyField := &filter.FilterField{FieldName: gptr.Of("   ")}

		taskDO := &entity.ObservabilityTask{
			ID:            1,
			Name:          "task",
			WorkspaceID:   2,
			TaskType:      task.TaskTypeAutoEval,
			TaskStatus:    task.TaskStatusUnstarted,
			CreatedBy:     "user1",
			UpdatedBy:     "user2",
			EffectiveTime: &entity.EffectiveTime{},
			Sampler:       &entity.Sampler{},
			SpanFilter: &filter.SpanFilterFields{Filters: &filter.FilterFields{FilterFields: []*filter.FilterField{
				hiddenField,
				visibleField,
				emptyField,
				parentField,
			}}},
		}
		repoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskDO}, int64(1), nil)
		userMock.EXPECT().GetUserInfo(gomock.Any(), gomock.Any()).Return(nil, map[string]*entitycommon.UserInfo{}, nil)

		svc := &TaskServiceImpl{TaskRepo: repoMock, userProvider: userMock}
		resp, err := svc.ListTasks(context.Background(), &ListTasksReq{WorkspaceID: 2, TaskFilters: &filter.TaskFilterFields{}})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			assert.EqualValues(t, 1, *resp.Total)
			assert.Len(t, resp.Tasks, 1)
			filterFields := resp.Tasks[0].GetRule().GetSpanFilters().GetFilters()
			if assert.NotNil(t, filterFields) {
				fields := filterFields.GetFilterFields()
				assert.Len(t, fields, 2)
				assert.Equal(t, "visible", fields[0].GetFieldName())
				assert.Equal(t, []string{"val"}, fields[0].GetValues())
				sub := fields[1].GetSubFilter()
				if assert.NotNil(t, sub) {
					subFields := sub.GetFilterFields()
					assert.Len(t, subFields, 1)
					assert.Equal(t, "child", subFields[0].GetFieldName())
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
		userMock := rpcmock.NewMockIUserProvider(ctrl)

		subHidden := &filter.FilterField{FieldName: gptr.Of("inner_hidden"), Values: []string{"v"}, Hidden: gptr.Of(true)}
		subVisible := &filter.FilterField{FieldName: gptr.Of("inner_visible"), Values: []string{"v"}}
		parent := &filter.FilterField{SubFilter: &filter.FilterFields{FilterFields: []*filter.FilterField{subHidden, subVisible}}}
		visible := &filter.FilterField{FieldName: gptr.Of("outer_visible"), Values: []string{"v"}}
		hidden := &filter.FilterField{FieldName: gptr.Of("outer_hidden"), Values: []string{"v"}, Hidden: gptr.Of(true)}

		taskDO := &entity.ObservabilityTask{
			TaskType:      task.TaskTypeAutoEval,
			TaskStatus:    task.TaskStatusUnstarted,
			CreatedBy:     "user1",
			UpdatedBy:     "user2",
			EffectiveTime: &entity.EffectiveTime{},
			Sampler:       &entity.Sampler{},
			SpanFilter: &filter.SpanFilterFields{Filters: &filter.FilterFields{FilterFields: []*filter.FilterField{
				hidden,
				visible,
				parent,
			}}},
		}

		repoMock.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Any(), gomock.Nil()).Return(taskDO, nil)
		userMock.EXPECT().GetUserInfo(gomock.Any(), gomock.Any()).Return(nil, map[string]*entitycommon.UserInfo{}, nil)

		svc := &TaskServiceImpl{TaskRepo: repoMock, userProvider: userMock}
		resp, err := svc.GetTask(context.Background(), &GetTaskReq{TaskID: 1, WorkspaceID: 2})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			filters := resp.Task.GetRule().GetSpanFilters().GetFilters()
			if assert.NotNil(t, filters) {
				fields := filters.GetFilterFields()
				assert.Len(t, fields, 2)
				assert.Equal(t, "outer_visible", fields[0].GetFieldName())
				sub := fields[1].GetSubFilter()
				if assert.NotNil(t, sub) {
					subFields := sub.GetFilterFields()
					assert.Len(t, subFields, 1)
					assert.Equal(t, "inner_visible", subFields[0].GetFieldName())
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

		svc := &TaskServiceImpl{TaskRepo: repoMock}
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

		svc := &TaskServiceImpl{TaskRepo: repoMock}
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

		svc := &TaskServiceImpl{TaskRepo: repoMock}
		resp, err := svc.CheckTaskName(context.Background(), &CheckTaskNameReq{WorkspaceID: 1, Name: "task"})
		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			assert.True(t, *resp.Pass)
		}
	})
}

func TestTaskServiceImpl_shouldTriggerBackfill(t *testing.T) {
	service := &TaskServiceImpl{}

	t.Run("task type mismatch", func(t *testing.T) {
		taskDO := &entity.ObservabilityTask{TaskType: "other"}
		assert.False(t, service.shouldTriggerBackfill(taskDO))
	})

	t.Run("missing effective time", func(t *testing.T) {
		taskDO := &entity.ObservabilityTask{TaskType: task.TaskTypeAutoEval}
		assert.False(t, service.shouldTriggerBackfill(taskDO))
	})

	t.Run("valid", func(t *testing.T) {
		taskDO := &entity.ObservabilityTask{
			TaskType:              task.TaskTypeAutoDataReflow,
			BackfillEffectiveTime: &entity.EffectiveTime{StartAt: 1, EndAt: 2},
		}
		assert.True(t, service.shouldTriggerBackfill(taskDO))
	})
}

func TestTaskServiceImpl_sendBackfillMessage(t *testing.T) {
	t.Run("producer nil", func(t *testing.T) {
		svc := &TaskServiceImpl{}
		err := svc.sendBackfillMessage(context.Background(), &entity.BackFillEvent{})
		statusErr, ok := errorx.FromStatusError(err)
		if assert.True(t, ok) {
			assert.EqualValues(t, obErrorx.CommonInternalErrorCode, statusErr.Code())
		}
	})

	t.Run("success", func(t *testing.T) {
		ch := make(chan *entity.BackFillEvent, 1)
		svc := &TaskServiceImpl{backfillProducer: &stubBackfillProducer{ch: ch}}
		err := svc.sendBackfillMessage(context.Background(), &entity.BackFillEvent{TaskID: 1})
		assert.NoError(t, err)
		select {
		case event := <-ch:
			assert.Equal(t, int64(1), event.TaskID)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected send backfill message")
		}
	})
}
