// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	taskdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	taskapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	taskmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/task/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestTaskRPCAdapter_ListTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := taskmocks.NewMockClient(ctrl)
	adapter := NewTaskRPCAdapter(mockClient)
	ctx := context.Background()

	t.Run("成功获取任务列表", func(t *testing.T) {
		workspaceID := int64(123)
		limit := int32(10)
		offset := int32(0)
		total := int64(5)
		taskID1 := int64(1)
		taskID2 := int64(2)

		mockTasks := []*taskdomain.Task{
			{ID: &taskID1, Name: "task1"},
			{ID: &taskID2, Name: "task2"},
		}

		mockResp := &taskapi.ListTasksResponse{
			Tasks: mockTasks,
			Total: &total,
			BaseResp: &base.BaseResp{
				StatusCode: 0,
			},
		}

		expectedReq := &taskapi.ListTasksRequest{
			WorkspaceID: workspaceID,
			Limit:       &limit,
			Offset:      &offset,
		}

		mockClient.EXPECT().ListTasks(gomock.Any(), expectedReq).Return(mockResp, nil)

		param := &rpc.ListTasksParam{
			WorkspaceID: workspaceID,
			Limit:       &limit,
			Offset:      &offset,
		}

		tasks, gotTotal, err := adapter.ListTasks(ctx, param)
		assert.NoError(t, err)
		assert.NotNil(t, tasks)
		assert.Len(t, tasks, 2)
		assert.Equal(t, int64(1), *tasks[0].ID)
		assert.Equal(t, "task1", tasks[0].Name)
		assert.Equal(t, total, *gotTotal)
	})

	t.Run("客户端返回错误", func(t *testing.T) {
		workspaceID := int64(123)
		expectedErr := errors.New("client error")

		mockClient.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		param := &rpc.ListTasksParam{
			WorkspaceID: workspaceID,
		}

		tasks, gotTotal, err := adapter.ListTasks(ctx, param)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, tasks)
		assert.Nil(t, gotTotal)
	})

	t.Run("响应为nil", func(t *testing.T) {
		workspaceID := int64(123)

		mockClient.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, nil)

		param := &rpc.ListTasksParam{
			WorkspaceID: workspaceID,
		}

		tasks, gotTotal, err := adapter.ListTasks(ctx, param)
		assert.Error(t, err)
		statusErr, ok := errorx.FromStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, int32(errno.CommonRPCErrorCode), statusErr.Code())
		assert.Nil(t, tasks)
		assert.Nil(t, gotTotal)
	})

	t.Run("BaseResp状态码非0", func(t *testing.T) {
		workspaceID := int64(123)
		errorCode := int32(1001)
		errorMsg := "some error"

		mockResp := &taskapi.ListTasksResponse{
			BaseResp: &base.BaseResp{
				StatusCode:    errorCode,
				StatusMessage: errorMsg,
			},
		}

		mockClient.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(mockResp, nil)

		param := &rpc.ListTasksParam{
			WorkspaceID: workspaceID,
		}

		tasks, gotTotal, err := adapter.ListTasks(ctx, param)
		assert.Error(t, err)
		statusErr, ok := errorx.FromStatusError(err)
		assert.True(t, ok)
		assert.Equal(t, errorCode, statusErr.Code())
		assert.Contains(t, err.Error(), errorMsg)
		assert.Nil(t, tasks)
		assert.Nil(t, gotTotal)
	})

	t.Run("Tasks字段为nil", func(t *testing.T) {
		workspaceID := int64(123)
		total := int64(0)

		mockResp := &taskapi.ListTasksResponse{
			Tasks: nil,
			Total: &total,
			BaseResp: &base.BaseResp{
				StatusCode: 0,
			},
		}

		mockClient.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(mockResp, nil)

		param := &rpc.ListTasksParam{
			WorkspaceID: workspaceID,
		}

		tasks, gotTotal, err := adapter.ListTasks(ctx, param)
		assert.NoError(t, err)
		assert.Nil(t, tasks)
		assert.Equal(t, total, *gotTotal)
	})

	t.Run("Total字段为nil", func(t *testing.T) {
		workspaceID := int64(123)
		taskID := int64(1)

		mockTasks := []*taskdomain.Task{
			{ID: &taskID},
		}

		mockResp := &taskapi.ListTasksResponse{
			Tasks: mockTasks,
			Total: nil,
			BaseResp: &base.BaseResp{
				StatusCode: 0,
			},
		}

		mockClient.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(mockResp, nil)

		param := &rpc.ListTasksParam{
			WorkspaceID: workspaceID,
		}

		tasks, gotTotal, err := adapter.ListTasks(ctx, param)
		assert.NoError(t, err)
		assert.NotNil(t, tasks)
		assert.Len(t, tasks, 1)
		assert.Nil(t, gotTotal)
	})
}

func TestNewTaskRPCAdapter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := taskmocks.NewMockClient(ctrl)
	adapter := NewTaskRPCAdapter(mockClient)
	assert.NotNil(t, adapter)
	assert.IsType(t, &TaskRPCAdapter{}, adapter)
}
