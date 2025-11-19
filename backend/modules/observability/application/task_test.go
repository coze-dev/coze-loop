// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	commondomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	filterdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	taskdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	taskapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	svc "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service"
	svcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/mocks"
	tracehubmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/tracehub/mocks"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func ctxWithAppID(appID int32) context.Context {
	return session.WithCtxUser(context.Background(), &session.User{ID: "uid", AppID: appID})
}

func assertErrorCode(t *testing.T, err error, code int32) {
	t.Helper()
	statusErr, ok := errorx.FromStatusError(err)
	if !assert.True(t, ok, "error should be StatusError") {
		return
	}
	assert.Equal(t, code, statusErr.Code())
}

func TestTaskApplication_CheckTaskName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		fieldsBuilder func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider)
		ctx           context.Context
		req           *taskapi.CheckTaskNameRequest
		expectResp    *taskapi.CheckTaskNameResponse
		expectErr     error
		expectErrCode int32
	}{
		{
			name:          "nil request",
			ctx:           context.Background(),
			req:           nil,
			expectResp:    taskapi.NewCheckTaskNameResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name: "invalid workspace",
			ctx:  context.Background(),
			req: &taskapi.CheckTaskNameRequest{
				WorkspaceID: 0,
			},
			expectResp:    taskapi.NewCheckTaskNameResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name: "auth error with trace app id",
			ctx:  ctxWithAppID(717152),
			req: &taskapi.CheckTaskNameRequest{
				WorkspaceID: 101,
				Name:        "task",
			},
			expectErr: errors.New("auth error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(101, 10), false).Return(errors.New("auth error"))
				return nil, auth
			},
		},
		{
			name: "service error",
			ctx:  context.Background(),
			req: &taskapi.CheckTaskNameRequest{
				WorkspaceID: 201,
				Name:        "dup",
			},
			expectResp: taskapi.NewCheckTaskNameResponse(),
			expectErr:  errors.New("service error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(201, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().CheckTaskName(gomock.Any(), &svc.CheckTaskNameReq{WorkspaceID: 201, Name: "dup"}).Return(nil, errors.New("service error"))
				return s, auth
			},
		},
		{
			name: "pass true",
			ctx:  context.Background(),
			req: &taskapi.CheckTaskNameRequest{
				WorkspaceID: 301,
				Name:        "ok",
			},
			expectResp: &taskapi.CheckTaskNameResponse{Pass: gptr.Of(true)},
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(301, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().CheckTaskName(gomock.Any(), &svc.CheckTaskNameReq{WorkspaceID: 301, Name: "ok"}).Return(&svc.CheckTaskNameResp{Pass: gptr.Of(true)}, nil)
				return s, auth
			},
		},
		{
			name: "pass false with trace app id",
			ctx:  ctxWithAppID(717152),
			req: &taskapi.CheckTaskNameRequest{
				WorkspaceID: 401,
				Name:        "dup",
			},
			expectResp: &taskapi.CheckTaskNameResponse{Pass: gptr.Of(false)},
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(401, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().CheckTaskName(gomock.Any(), &svc.CheckTaskNameReq{WorkspaceID: 401, Name: "dup"}).Return(&svc.CheckTaskNameResp{Pass: gptr.Of(false)}, nil)
				return s, auth
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			taskSvc, authSvc := caseItem.fieldsBuilder(ctrl)
			app := &TaskApplication{
				taskSvc: taskSvc,
				authSvc: authSvc,
			}
			resp, err := app.CheckTaskName(caseItem.ctx, caseItem.req)

			if caseItem.expectErr != nil {
				assert.EqualError(t, err, caseItem.expectErr.Error())
			} else if caseItem.expectErrCode != 0 {
				assert.Error(t, err)
				assertErrorCode(t, err, caseItem.expectErrCode)
			} else {
				assert.NoError(t, err)
			}

			if caseItem.expectResp != nil {
				assert.Equal(t, caseItem.expectResp, resp)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func TestTaskApplication_CreateTask(t *testing.T) {
	t.Parallel()

	newValidTask := func() *taskdto.Task {
		return &taskdto.Task{
			Name:        "task",
			WorkspaceID: gptr.Of(int64(123)),
			TaskType:    taskdto.TaskTypeAutoEval,
			Rule: &taskdto.Rule{
				SpanFilters: &filterdto.SpanFilterFields{
					PlatformType: gptr.Of(commondomain.PlatformTypeCozeloop),
					SpanListType: gptr.Of(commondomain.SpanListTypeRootSpan),
					Filters: &filterdto.FilterFields{
						FilterFields: []*filterdto.FilterField{},
					},
				},
				EffectiveTime: &taskdto.EffectiveTime{
					StartAt: gptr.Of(time.Now().Add(time.Hour).UnixMilli()),
					EndAt:   gptr.Of(time.Now().Add(2 * time.Hour).UnixMilli()),
				},
			},
		}
	}

	taskForAuth := newValidTask()
	taskForSvcErr := newValidTask()
	taskForSuccess := newValidTask()

	tests := []struct {
		name          string
		ctx           context.Context
		req           *taskapi.CreateTaskRequest
		fieldsBuilder func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider)
		expectResp    *taskapi.CreateTaskResponse
		expectErr     error
		expectErrCode int32
	}{
		{
			name:          "nil request",
			ctx:           context.Background(),
			req:           nil,
			expectResp:    taskapi.NewCreateTaskResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name: "task nil",
			ctx:  context.Background(),
			req: &taskapi.CreateTaskRequest{
				Task: nil,
			},
			expectResp:    taskapi.NewCreateTaskResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name:       "auth error",
			ctx:        ctxWithAppID(1),
			req:        &taskapi.CreateTaskRequest{Task: taskForAuth},
			expectResp: taskapi.NewCreateTaskResponse(),
			expectErr:  errors.New("auth error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskCreate, strconv.FormatInt(123, 10), false).Return(errors.New("auth error"))
				return nil, auth
			},
		},
		{
			name:       "service error",
			ctx:        ctxWithAppID(1),
			req:        &taskapi.CreateTaskRequest{Task: taskForSvcErr},
			expectResp: taskapi.NewCreateTaskResponse(),
			expectErr:  errors.New("svc error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskCreate, strconv.FormatInt(123, 10), false).Return(nil)
				svcMock := svcmock.NewMockITaskService(ctrl)
				svcMock.EXPECT().CreateTask(gomock.Any(), gomock.AssignableToTypeOf(&svc.CreateTaskReq{})).Return(nil, errors.New("svc error"))
				return svcMock, auth
			},
		},
		{
			name:       "success with trace app",
			ctx:        ctxWithAppID(717152),
			req:        &taskapi.CreateTaskRequest{Task: taskForSuccess},
			expectResp: &taskapi.CreateTaskResponse{TaskID: gptr.Of(int64(1000))},
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskCreate, strconv.FormatInt(123, 10), false).Return(nil)
				svcMock := svcmock.NewMockITaskService(ctrl)
				svcMock.EXPECT().CreateTask(gomock.Any(), gomock.AssignableToTypeOf(&svc.CreateTaskReq{})).Return(&svc.CreateTaskResp{TaskID: gptr.Of(int64(1000))}, nil)
				return svcMock, auth
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			taskSvc, authSvc := caseItem.fieldsBuilder(ctrl)
			app := &TaskApplication{
				taskSvc: taskSvc,
				authSvc: authSvc,
			}

			resp, err := app.CreateTask(caseItem.ctx, caseItem.req)

			if caseItem.expectErr != nil {
				assert.EqualError(t, err, caseItem.expectErr.Error())
			} else if caseItem.expectErrCode != 0 {
				assert.Error(t, err)
				assertErrorCode(t, err, caseItem.expectErrCode)
			} else {
				assert.NoError(t, err)
			}

			if caseItem.expectResp != nil {
				assert.Equal(t, caseItem.expectResp, resp)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func TestTaskApplication_UpdateTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		ctx           context.Context
		req           *taskapi.UpdateTaskRequest
		fieldsBuilder func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider)
		expectResp    *taskapi.UpdateTaskResponse
		expectErr     error
		expectErrCode int32
	}{
		{
			name:          "nil request",
			ctx:           context.Background(),
			req:           nil,
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name: "invalid workspace",
			ctx:  context.Background(),
			req: &taskapi.UpdateTaskRequest{
				TaskID:      1,
				WorkspaceID: 0,
			},
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name:      "auth error",
			ctx:       ctxWithAppID(717152),
			req:       &taskapi.UpdateTaskRequest{TaskID: 11, WorkspaceID: 22},
			expectErr: errors.New("auth error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckTaskPermission(gomock.Any(), rpc.AuthActionTraceTaskEdit, strconv.FormatInt(22, 10), strconv.FormatInt(11, 10)).Return(errors.New("auth error"))
				return nil, auth
			},
		},
		{
			name:       "service error",
			ctx:        context.Background(),
			req:        &taskapi.UpdateTaskRequest{TaskID: 33, WorkspaceID: 44, Session: &commondomain.Session{UserID: gptr.Of("1")}},
			expectResp: taskapi.NewUpdateTaskResponse(),
			expectErr:  errors.New("svc error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckTaskPermission(gomock.Any(), rpc.AuthActionTraceTaskEdit, strconv.FormatInt(44, 10), strconv.FormatInt(33, 10)).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().UpdateTask(gomock.Any(), &svc.UpdateTaskReq{
					TaskID:      33,
					WorkspaceID: 44,
					TaskStatus:  nil,
					Description: nil,
					UserID:      "1",
				}).Return(errors.New("svc error"))
				return s, auth
			},
		},
		{
			name:       "success",
			ctx:        context.Background(),
			req:        &taskapi.UpdateTaskRequest{TaskID: 55, WorkspaceID: 66, Session: &commondomain.Session{UserID: gptr.Of("1")}},
			expectResp: taskapi.NewUpdateTaskResponse(),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckTaskPermission(gomock.Any(), rpc.AuthActionTraceTaskEdit, strconv.FormatInt(66, 10), strconv.FormatInt(55, 10)).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().UpdateTask(gomock.Any(), &svc.UpdateTaskReq{
					TaskID:      55,
					WorkspaceID: 66,
					TaskStatus:  nil,
					Description: nil,
					UserID:      "1",
				}).Return(nil)
				return s, auth
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			taskSvc, authSvc := caseItem.fieldsBuilder(ctrl)
			app := &TaskApplication{
				taskSvc: taskSvc,
				authSvc: authSvc,
			}
			resp, err := app.UpdateTask(caseItem.ctx, caseItem.req)

			if caseItem.expectErr != nil {
				assert.EqualError(t, err, caseItem.expectErr.Error())
			} else if caseItem.expectErrCode != 0 {
				assert.Error(t, err)
				assertErrorCode(t, err, caseItem.expectErrCode)
			} else {
				assert.NoError(t, err)
			}

			if caseItem.expectResp != nil {
				assert.Equal(t, caseItem.expectResp, resp)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func TestTaskApplication_ListTasks(t *testing.T) {
	t.Parallel()

	taskListResp := &svc.ListTasksResp{
		Tasks: []*entity.ObservabilityTask{{Name: "task1"}},
		Total: int64(1),
	}
	tests := []struct {
		name          string
		ctx           context.Context
		req           *taskapi.ListTasksRequest
		fieldsBuilder func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider, rpc.IUserProvider)
		expectResp    *taskapi.ListTasksResponse
		expectErr     error
		expectErrCode int32
	}{
		{
			name:          "nil request",
			ctx:           context.Background(),
			req:           nil,
			expectResp:    taskapi.NewListTasksResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider, rpc.IUserProvider) {
				return nil, nil, nil
			},
		},
		{
			name:          "invalid workspace",
			ctx:           context.Background(),
			req:           &taskapi.ListTasksRequest{WorkspaceID: 0},
			expectResp:    taskapi.NewListTasksResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider, rpc.IUserProvider) {
				return nil, nil, nil
			},
		},
		{
			name:       "auth error",
			ctx:        ctxWithAppID(717152),
			req:        &taskapi.ListTasksRequest{WorkspaceID: 123},
			expectResp: taskapi.NewListTasksResponse(),
			expectErr:  errors.New("auth error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider, rpc.IUserProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(123, 10), false).Return(errors.New("auth error"))
				return nil, auth, nil
			},
		},
		{
			name:       "service error",
			ctx:        context.Background(),
			req:        &taskapi.ListTasksRequest{WorkspaceID: 456},
			expectResp: taskapi.NewListTasksResponse(),
			expectErr:  errors.New("svc error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider, rpc.IUserProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(456, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().ListTasks(gomock.Any(), &svc.ListTasksReq{
					WorkspaceID: 456,
				}).Return(nil, errors.New("svc error"))
				return s, auth, nil
			},
		},
		{
			name: "success",
			ctx:  context.Background(),
			req:  &taskapi.ListTasksRequest{WorkspaceID: 789},
			expectResp: &taskapi.ListTasksResponse{
				Tasks: tconv.TaskDOs2DTOs(context.Background(), taskListResp.Tasks, map[string]*common.UserInfo{}),
				Total: lo.ToPtr(taskListResp.Total),
			},
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider, rpc.IUserProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(789, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().ListTasks(gomock.Any(), &svc.ListTasksReq{
					WorkspaceID: 789,
				}).Return(taskListResp, nil)
				user := rpcmock.NewMockIUserProvider(ctrl)
				user.EXPECT().GetUserInfo(gomock.Any(), gomock.Any()).Return(nil, nil, nil)
				return s, auth, user
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			taskSvc, authSvc, userSvc := caseItem.fieldsBuilder(ctrl)
			app := &TaskApplication{
				taskSvc: taskSvc,
				authSvc: authSvc,
				userSvc: userSvc,
			}
			resp, err := app.ListTasks(caseItem.ctx, caseItem.req)

			if caseItem.expectErr != nil {
				assert.EqualError(t, err, caseItem.expectErr.Error())
			} else if caseItem.expectErrCode != 0 {
				assert.Error(t, err)
				assertErrorCode(t, err, caseItem.expectErrCode)
			} else {
				assert.NoError(t, err)
			}

			if caseItem.expectResp != nil {
				assert.Equal(t, caseItem.expectResp, resp)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func TestTaskApplication_GetTask(t *testing.T) {
	t.Parallel()

	taskResp := &svc.GetTaskResp{Task: &entity.ObservabilityTask{Name: "task"}}

	tests := []struct {
		name          string
		ctx           context.Context
		req           *taskapi.GetTaskRequest
		fieldsBuilder func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider)
		expectResp    *taskapi.GetTaskResponse
		expectErr     error
		expectErrCode int32
	}{
		{
			name:          "nil request",
			ctx:           context.Background(),
			req:           nil,
			expectResp:    taskapi.NewGetTaskResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name:          "invalid workspace",
			ctx:           context.Background(),
			req:           &taskapi.GetTaskRequest{WorkspaceID: 0},
			expectResp:    taskapi.NewGetTaskResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name:       "auth error",
			ctx:        ctxWithAppID(717152),
			req:        &taskapi.GetTaskRequest{WorkspaceID: 100, TaskID: 1},
			expectResp: taskapi.NewGetTaskResponse(),
			expectErr:  errors.New("auth error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(100, 10), false).Return(errors.New("auth error"))
				return nil, auth
			},
		},
		{
			name:       "service error",
			ctx:        context.Background(),
			req:        &taskapi.GetTaskRequest{WorkspaceID: 101, TaskID: 2},
			expectResp: taskapi.NewGetTaskResponse(),
			expectErr:  errors.New("svc error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(101, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().GetTask(gomock.Any(), &svc.GetTaskReq{WorkspaceID: 101, TaskID: 2}).Return(nil, errors.New("svc error"))
				return s, auth
			},
		},
		{
			name:       "success",
			ctx:        context.Background(),
			req:        &taskapi.GetTaskRequest{WorkspaceID: 202, TaskID: 3},
			expectResp: &taskapi.GetTaskResponse{Task: tconv.TaskDO2DTO(context.Background(), taskResp.Task, map[string]*common.UserInfo{})},
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(202, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().GetTask(gomock.Any(), &svc.GetTaskReq{WorkspaceID: 202, TaskID: 3}).Return(taskResp, nil)
				return s, auth
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			taskSvc, authSvc := caseItem.fieldsBuilder(ctrl)
			app := &TaskApplication{
				taskSvc: taskSvc,
				authSvc: authSvc,
			}
			resp, err := app.GetTask(caseItem.ctx, caseItem.req)

			if caseItem.expectErr != nil {
				assert.EqualError(t, err, caseItem.expectErr.Error())
			} else if caseItem.expectErrCode != 0 {
				assert.Error(t, err)
				assertErrorCode(t, err, caseItem.expectErrCode)
			} else {
				assert.NoError(t, err)
			}

			if caseItem.expectResp != nil {
				assert.Equal(t, caseItem.expectResp, resp)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func TestTaskApplication_SpanTrigger(t *testing.T) {
	t.Parallel()

	event := &entity.RawSpan{}

	tests := []struct {
		name      string
		mockSvc   func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService
		expectErr bool
	}{
		{
			name: "trace hub error",
			mockSvc: func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService {
				svc := tracehubmock.NewMockITraceHubService(ctrl)
				svc.EXPECT().SpanTrigger(gomock.Any(), event).Return(errors.New("hub error"))
				return svc
			},
			expectErr: true,
		},
		{
			name: "success",
			mockSvc: func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService {
				svc := tracehubmock.NewMockITraceHubService(ctrl)
				svc.EXPECT().SpanTrigger(gomock.Any(), event).Return(nil)
				return svc
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			traceSvc := caseItem.mockSvc(ctrl)
			app := &TaskApplication{tracehubSvc: traceSvc}
			span := &loop_span.Span{}
			err := app.SpanTrigger(context.Background(), event, span)
			if caseItem.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskApplication_CallBack(t *testing.T) {
	t.Parallel()

	event := &entity.AutoEvalEvent{}
	tests := []struct {
		name      string
		mockSvc   func(ctrl *gomock.Controller) *svcmock.MockITaskCallbackService
		expectErr bool
	}{
		{
			name: "trace hub error",
			mockSvc: func(ctrl *gomock.Controller) *svcmock.MockITaskCallbackService {
				svc := svcmock.NewMockITaskCallbackService(ctrl)
				svc.EXPECT().AutoEvalCallback(gomock.Any(), event).Return(errors.New("hub error"))
				return svc
			},
			expectErr: true,
		},
		{
			name: "success",
			mockSvc: func(ctrl *gomock.Controller) *svcmock.MockITaskCallbackService {
				svc := svcmock.NewMockITaskCallbackService(ctrl)
				svc.EXPECT().AutoEvalCallback(gomock.Any(), event).Return(nil)
				return svc
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			traceSvc := caseItem.mockSvc(ctrl)
			app := &TaskApplication{taskCallbackSvc: traceSvc}
			err := app.AutoEvalCallback(context.Background(), event)
			if caseItem.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskApplication_Correction(t *testing.T) {
	t.Parallel()

	event := &entity.CorrectionEvent{}
	tests := []struct {
		name      string
		mockSvc   func(ctrl *gomock.Controller) *svcmock.MockITaskCallbackService
		expectErr bool
	}{
		{
			name: "trace hub error",
			mockSvc: func(ctrl *gomock.Controller) *svcmock.MockITaskCallbackService {
				svc := svcmock.NewMockITaskCallbackService(ctrl)
				svc.EXPECT().AutoEvalCorrection(gomock.Any(), event).Return(errors.New("hub error"))
				return svc
			},
			expectErr: true,
		},
		{
			name: "success",
			mockSvc: func(ctrl *gomock.Controller) *svcmock.MockITaskCallbackService {
				svc := svcmock.NewMockITaskCallbackService(ctrl)
				svc.EXPECT().AutoEvalCorrection(gomock.Any(), event).Return(nil)
				return svc
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			traceSvc := caseItem.mockSvc(ctrl)
			app := &TaskApplication{taskCallbackSvc: traceSvc}
			err := app.AutoEvalCorrection(context.Background(), event)
			if caseItem.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskApplication_BackFill(t *testing.T) {
	t.Parallel()

	event := &entity.BackFillEvent{}
	tests := []struct {
		name      string
		mockSvc   func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService
		expectErr bool
	}{
		{
			name: "trace hub error",
			mockSvc: func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService {
				svc := tracehubmock.NewMockITraceHubService(ctrl)
				svc.EXPECT().BackFill(gomock.Any(), event).Return(errors.New("hub error"))
				return svc
			},
			expectErr: true,
		},
		{
			name: "success",
			mockSvc: func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService {
				svc := tracehubmock.NewMockITraceHubService(ctrl)
				svc.EXPECT().BackFill(gomock.Any(), event).Return(nil)
				return svc
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			traceSvc := caseItem.mockSvc(ctrl)
			app := &TaskApplication{tracehubSvc: traceSvc}
			err := app.BackFill(context.Background(), event)
			if caseItem.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
