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
	loop_span "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	traceSvc "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	traceSvcMock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	span_filter "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	filtermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
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

func TestTaskApplication_buildSpanFilters(t *testing.T) {
	t.Parallel()

	type fields struct {
		builder traceSvc.TraceFilterProcessorBuilder
	}

	type args struct {
		spanFilters *filterdto.SpanFilterFields
		workspaceID int64
	}

	tests := []struct {
		name          string
		fieldsBuilder func(ctrl *gomock.Controller, t *testing.T, a args) fields
		args          args
		assertFunc    func(t *testing.T, original *filterdto.SpanFilterFields, got *entity.SpanFilterFields, err error)
	}{
		{
			name: "non supported platform returns original",
			fieldsBuilder: func(ctrl *gomock.Controller, t *testing.T, a args) fields {
				return fields{}
			},
			args: args{
				spanFilters: &filterdto.SpanFilterFields{
					Filters: &filterdto.FilterFields{
						FilterFields: []*filterdto.FilterField{
							{
								FieldName: gptr.Of("custom_field"),
								FieldType: gptr.Of(filterdto.FieldTypeString),
								Values:    []string{"value"},
							},
						},
					},
					PlatformType: gptr.Of(commondomain.PlatformTypeCozeloop),
					SpanListType: gptr.Of(commondomain.SpanListTypeRootSpan),
				},
				workspaceID: 100,
			},
			assertFunc: func(t *testing.T, original *filterdto.SpanFilterFields, got *entity.SpanFilterFields, err error) {
				assert.NoError(t, err)
				if assert.NotNil(t, got) {
					assert.Equal(t, commondomain.PlatformTypeCozeloop, got.PlatformType)
					assert.Equal(t, commondomain.SpanListTypeRootSpan, got.SpanListType)
					dtoFilters := original.GetFilters().GetFilterFields()
					if assert.Len(t, got.Filters.FilterFields, len(dtoFilters)) && len(dtoFilters) > 0 {
						firstDTO := dtoFilters[0]
						firstDomain := got.Filters.FilterFields[0]
						if assert.NotNil(t, firstDTO.FieldName) {
							assert.Equal(t, *firstDTO.FieldName, firstDomain.FieldName)
						}
						if assert.NotNil(t, firstDTO.FieldType) {
							assert.Equal(t, loop_span.FieldType(*firstDTO.FieldType), firstDomain.FieldType)
						}
						assert.Equal(t, firstDTO.Values, firstDomain.Values)
						assert.False(t, firstDomain.Hidden)
					}
				}
			},
		},
		{
			name: "build platform filter error",
			fieldsBuilder: func(ctrl *gomock.Controller, t *testing.T, a args) fields {
				builder := traceSvcMock.NewMockTraceFilterProcessorBuilder(ctrl)
				builder.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformType(commondomain.PlatformTypeCozeBot)).Return(nil, errors.New("build platform error"))
				return fields{builder: builder}
			},
			args: args{
				spanFilters: &filterdto.SpanFilterFields{
					Filters: &filterdto.FilterFields{
						FilterFields: []*filterdto.FilterField{},
					},
					PlatformType: gptr.Of(commondomain.PlatformTypeCozeBot),
					SpanListType: gptr.Of(commondomain.SpanListTypeRootSpan),
				},
				workspaceID: 200,
			},
			assertFunc: func(t *testing.T, original *filterdto.SpanFilterFields, got *entity.SpanFilterFields, err error) {
				assert.Nil(t, got)
				assert.EqualError(t, err, "build platform error")
			},
		},
		{
			name: "build basic span filter error",
			fieldsBuilder: func(ctrl *gomock.Controller, t *testing.T, a args) fields {
				builder := traceSvcMock.NewMockTraceFilterProcessorBuilder(ctrl)
				platformFilter := filtermocks.NewMockFilter(ctrl)
				builder.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformType(commondomain.PlatformTypeWorkflow)).Return(platformFilter, nil)
				platformFilter.EXPECT().
					BuildBasicSpanFilter(gomock.Any(), gomock.AssignableToTypeOf(&span_filter.SpanEnv{})).
					DoAndReturn(func(_ context.Context, env *span_filter.SpanEnv) ([]*loop_span.FilterField, bool, error) {
						assert.Equal(t, a.workspaceID, env.WorkspaceID)
						return nil, false, errors.New("build basic error")
					})
				return fields{builder: builder}
			},
			args: args{
				spanFilters: &filterdto.SpanFilterFields{
					Filters: &filterdto.FilterFields{
						FilterFields: []*filterdto.FilterField{},
					},
					PlatformType: gptr.Of(commondomain.PlatformTypeWorkflow),
					SpanListType: gptr.Of(commondomain.SpanListTypeRootSpan),
				},
				workspaceID: 300,
			},
			assertFunc: func(t *testing.T, original *filterdto.SpanFilterFields, got *entity.SpanFilterFields, err error) {
				assert.Nil(t, got)
				assert.EqualError(t, err, "build basic error")
			},
		},
		{
			name: "empty basic filter without force returns nil",
			fieldsBuilder: func(ctrl *gomock.Controller, t *testing.T, a args) fields {
				builder := traceSvcMock.NewMockTraceFilterProcessorBuilder(ctrl)
				platformFilter := filtermocks.NewMockFilter(ctrl)
				builder.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformType(commondomain.PlatformTypeInnerCozeBot)).Return(platformFilter, nil)
				platformFilter.EXPECT().
					BuildBasicSpanFilter(gomock.Any(), gomock.AssignableToTypeOf(&span_filter.SpanEnv{})).
					DoAndReturn(func(_ context.Context, env *span_filter.SpanEnv) ([]*loop_span.FilterField, bool, error) {
						assert.Equal(t, a.workspaceID, env.WorkspaceID)
						return []*loop_span.FilterField{}, false, nil
					})
				return fields{builder: builder}
			},
			args: args{
				spanFilters: &filterdto.SpanFilterFields{
					Filters: &filterdto.FilterFields{
						FilterFields: []*filterdto.FilterField{},
					},
					PlatformType: gptr.Of(commondomain.PlatformTypeInnerCozeBot),
					SpanListType: gptr.Of(commondomain.SpanListTypeRootSpan),
				},
				workspaceID: 400,
			},
			assertFunc: func(t *testing.T, original *filterdto.SpanFilterFields, got *entity.SpanFilterFields, err error) {
				assert.NoError(t, err)
				assert.Nil(t, got)
			},
		},
		{
			name: "merge platform filters success",
			fieldsBuilder: func(ctrl *gomock.Controller, t *testing.T, a args) fields {
				builder := traceSvcMock.NewMockTraceFilterProcessorBuilder(ctrl)
				platformFilter := filtermocks.NewMockFilter(ctrl)
				builder.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformType(commondomain.PlatformTypeProject)).Return(platformFilter, nil)
				platformFilter.EXPECT().
					BuildBasicSpanFilter(gomock.Any(), gomock.AssignableToTypeOf(&span_filter.SpanEnv{})).
					DoAndReturn(func(_ context.Context, env *span_filter.SpanEnv) ([]*loop_span.FilterField, bool, error) {
						assert.Equal(t, a.workspaceID, env.WorkspaceID)
						return []*loop_span.FilterField{
							{
								FieldName: loop_span.SpanFieldSpaceId,
								FieldType: loop_span.FieldTypeString,
								Values:    []string{"tenant"},
							},
						}, false, nil
					})
				return fields{builder: builder}
			},
			args: args{
				spanFilters: &filterdto.SpanFilterFields{
					Filters: &filterdto.FilterFields{
						FilterFields: []*filterdto.FilterField{
							{
								FieldName: gptr.Of("custom_field"),
								FieldType: gptr.Of(filterdto.FieldTypeString),
								Values:    []string{"origin"},
							},
						},
					},
					PlatformType: gptr.Of(commondomain.PlatformTypeProject),
					SpanListType: gptr.Of(commondomain.SpanListTypeRootSpan),
				},
				workspaceID: 500,
			},
			assertFunc: func(t *testing.T, original *filterdto.SpanFilterFields, got *entity.SpanFilterFields, err error) {
				assert.NoError(t, err)
				if assert.NotNil(t, got) {
					assert.Equal(t, commondomain.PlatformTypeProject, got.PlatformType)
					assert.Equal(t, commondomain.SpanListTypeRootSpan, got.SpanListType)
					originalFilters := original.GetFilters().GetFilterFields()
					if assert.Len(t, got.Filters.FilterFields, len(originalFilters)+1) && len(originalFilters) > 0 {
						firstDomain := got.Filters.FilterFields[0]
						firstDTO := originalFilters[0]
						if assert.NotNil(t, firstDTO.FieldName) {
							assert.Equal(t, *firstDTO.FieldName, firstDomain.FieldName)
						}
						assert.False(t, firstDomain.Hidden)
						appended := got.Filters.FilterFields[len(originalFilters)]
						assert.Equal(t, loop_span.SpanFieldSpaceId, appended.FieldName)
						assert.True(t, appended.Hidden)
						assert.Equal(t, []string{"tenant"}, appended.Values)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		caseItem := tt
		t.Run(caseItem.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fields := caseItem.fieldsBuilder(ctrl, t, caseItem.args)
			app := &TaskApplication{
				buildHelper: fields.builder,
			}

			got, err := app.buildSpanFilters(context.Background(), caseItem.args.spanFilters, caseItem.args.workspaceID)

			caseItem.assertFunc(t, caseItem.args.spanFilters, got, err)
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
			req:        &taskapi.UpdateTaskRequest{TaskID: 33, WorkspaceID: 44},
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
				}).Return(errors.New("svc error"))
				return s, auth
			},
		},
		{
			name:       "success",
			ctx:        context.Background(),
			req:        &taskapi.UpdateTaskRequest{TaskID: 55, WorkspaceID: 66},
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
		Tasks: []*taskdto.Task{{Name: "task1"}},
		Total: gptr.Of(int64(1)),
	}
	tests := []struct {
		name          string
		ctx           context.Context
		req           *taskapi.ListTasksRequest
		fieldsBuilder func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider)
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
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name:          "invalid workspace",
			ctx:           context.Background(),
			req:           &taskapi.ListTasksRequest{WorkspaceID: 0},
			expectResp:    taskapi.NewListTasksResponse(),
			expectErrCode: obErrorx.CommercialCommonInvalidParamCodeCode,
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				return nil, nil
			},
		},
		{
			name:       "auth error",
			ctx:        ctxWithAppID(717152),
			req:        &taskapi.ListTasksRequest{WorkspaceID: 123},
			expectResp: taskapi.NewListTasksResponse(),
			expectErr:  errors.New("auth error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(123, 10), false).Return(errors.New("auth error"))
				return nil, auth
			},
		},
		{
			name:       "service error",
			ctx:        context.Background(),
			req:        &taskapi.ListTasksRequest{WorkspaceID: 456},
			expectResp: taskapi.NewListTasksResponse(),
			expectErr:  errors.New("svc error"),
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(456, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().ListTasks(gomock.Any(), &svc.ListTasksReq{
					WorkspaceID: 456,
				}).Return(nil, errors.New("svc error"))
				return s, auth
			},
		},
		{
			name:       "success",
			ctx:        context.Background(),
			req:        &taskapi.ListTasksRequest{WorkspaceID: 789},
			expectResp: &taskapi.ListTasksResponse{Tasks: taskListResp.Tasks, Total: taskListResp.Total},
			fieldsBuilder: func(ctrl *gomock.Controller) (svc.ITaskService, rpc.IAuthProvider) {
				auth := rpcmock.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, strconv.FormatInt(789, 10), false).Return(nil)
				s := svcmock.NewMockITaskService(ctrl)
				s.EXPECT().ListTasks(gomock.Any(), &svc.ListTasksReq{
					WorkspaceID: 789,
				}).Return(taskListResp, nil)
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

	taskResp := &svc.GetTaskResp{Task: &taskdto.Task{Name: "task"}}

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
			expectResp: &taskapi.GetTaskResponse{Task: taskResp.Task},
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
			err := app.SpanTrigger(context.Background(), event)
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
		mockSvc   func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService
		expectErr bool
	}{
		{
			name: "trace hub error",
			mockSvc: func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService {
				svc := tracehubmock.NewMockITraceHubService(ctrl)
				svc.EXPECT().CallBack(gomock.Any(), event).Return(errors.New("hub error"))
				return svc
			},
			expectErr: true,
		},
		{
			name: "success",
			mockSvc: func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService {
				svc := tracehubmock.NewMockITraceHubService(ctrl)
				svc.EXPECT().CallBack(gomock.Any(), event).Return(nil)
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
			err := app.CallBack(context.Background(), event)
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
		mockSvc   func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService
		expectErr bool
	}{
		{
			name: "trace hub error",
			mockSvc: func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService {
				svc := tracehubmock.NewMockITraceHubService(ctrl)
				svc.EXPECT().Correction(gomock.Any(), event).Return(errors.New("hub error"))
				return svc
			},
			expectErr: true,
		},
		{
			name: "success",
			mockSvc: func(ctrl *gomock.Controller) *tracehubmock.MockITraceHubService {
				svc := tracehubmock.NewMockITraceHubService(ctrl)
				svc.EXPECT().Correction(gomock.Any(), event).Return(nil)
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
			err := app.Correction(context.Background(), event)
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
