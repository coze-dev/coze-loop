// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/cloudwego/kitex/client/callopt"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/execute"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/prompt/mocks"
)

func TestPromptRPCAdapter_ExecutePrompt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManage := mocks.NewMockPromptManageClient(ctrl)
	mockExecute := mocks.NewMockPromptExecuteClient(ctrl)

	adapter := NewPromptRPCAdapter(mockManage, mockExecute)

	ctx := context.Background()
	param := &rpc.ExecutePromptParam{
		PromptID:      1,
		PromptVersion: "v1",
	}

	t.Run("success", func(t *testing.T) {
		mockExecute.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).Return(&execute.ExecuteInternalResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			Message: &prompt.Message{
				Content: gptr.Of("resp"),
			},
		}, nil)

		res, err := adapter.ExecutePrompt(ctx, 1, param)
		assert.NoError(t, err)
		assert.Equal(t, "resp", *res.Content)
	})

	t.Run("error_base_resp", func(t *testing.T) {
		mockExecute.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).Return(&execute.ExecuteInternalResponse{
			BaseResp: &base.BaseResp{StatusCode: 500, StatusMessage: "error"},
		}, nil)

		_, err := adapter.ExecutePrompt(ctx, 1, param)
		assert.Error(t, err)
	})

	t.Run("with_runtime_param", func(t *testing.T) {
		paramWithRuntime := &rpc.ExecutePromptParam{
			PromptID:      1,
			PromptVersion: "v1",
			RuntimeParam:  gptr.Of(`{"model_config":{"model_id":123,"max_tokens":100}}`),
		}
		mockExecute.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req *execute.ExecuteInternalRequest, opts ...callopt.Option) (*execute.ExecuteInternalResponse, error) {
			assert.Equal(t, int64(123), *req.OverridePromptParams.ModelConfig.ModelID)
			assert.Equal(t, int32(100), *req.OverridePromptParams.ModelConfig.MaxTokens)
			return &execute.ExecuteInternalResponse{
				BaseResp: &base.BaseResp{StatusCode: 0},
				Message:  &prompt.Message{Content: gptr.Of("resp")},
			}, nil
		})

		_, err := adapter.ExecutePrompt(ctx, 1, paramWithRuntime)
		assert.NoError(t, err)
	})

	t.Run("invalid_runtime_param", func(t *testing.T) {
		paramInvalid := &rpc.ExecutePromptParam{
			RuntimeParam: gptr.Of(`{invalid}`),
		}
		// It logs the error but continues without override
		mockExecute.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).Return(&execute.ExecuteInternalResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			Message:  &prompt.Message{Content: gptr.Of("resp")},
		}, nil)
		_, err := adapter.ExecutePrompt(ctx, 1, paramInvalid)
		assert.NoError(t, err)
	})

	t.Run("resp_nil", func(t *testing.T) {
		mockExecute.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		_, err := adapter.ExecutePrompt(ctx, 1, param)
		assert.Error(t, err)
	})
}

func TestPromptRPCAdapter_GetPrompt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManage := mocks.NewMockPromptManageClient(ctrl)
	mockExecute := mocks.NewMockPromptExecuteClient(ctrl)

	adapter := NewPromptRPCAdapter(mockManage, mockExecute)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockManage.EXPECT().GetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(&manage.GetPromptResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			Prompt:   &prompt.Prompt{ID: gptr.Of(int64(1))},
		}, nil)

		res, err := adapter.GetPrompt(ctx, 1, 1, rpc.GetPromptParams{})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), res.ID)
	})

	t.Run("success_with_version", func(t *testing.T) {
		mockManage.EXPECT().GetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(&manage.GetPromptResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			Prompt:   &prompt.Prompt{ID: gptr.Of(int64(1))},
		}, nil)

		res, err := adapter.GetPrompt(ctx, 1, 1, rpc.GetPromptParams{CommitVersion: gptr.Of("v1")})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), res.ID)
	})

	t.Run("not_found", func(t *testing.T) {
		mockManage.EXPECT().GetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(&manage.GetPromptResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			Prompt:   nil,
		}, nil)

		res, err := adapter.GetPrompt(ctx, 1, 1, rpc.GetPromptParams{})
		assert.NoError(t, err)
		assert.Nil(t, res)
	})
}

func TestPromptRPCAdapter_MGetPrompt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManage := mocks.NewMockPromptManageClient(ctrl)
	mockExecute := mocks.NewMockPromptExecuteClient(ctrl)

	adapter := NewPromptRPCAdapter(mockManage, mockExecute)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockManage.EXPECT().BatchGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(&manage.BatchGetPromptResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			Results: []*manage.PromptResult_{
				{Prompt: &prompt.Prompt{ID: gptr.Of(int64(1))}},
				{Prompt: nil},
			},
		}, nil)

		res, err := adapter.MGetPrompt(ctx, 1, []*rpc.MGetPromptQuery{{PromptID: 1, Version: gptr.Of("v1")}})
		assert.NoError(t, err)
		assert.Len(t, res, 1)
	})

	t.Run("rpc_error", func(t *testing.T) {
		mockManage.EXPECT().BatchGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
		_, err := adapter.MGetPrompt(ctx, 1, []*rpc.MGetPromptQuery{{PromptID: 1}})
		assert.Error(t, err)
	})
}

func TestPromptRPCAdapter_ListPrompt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManage := mocks.NewMockPromptManageClient(ctrl)
	mockExecute := mocks.NewMockPromptExecuteClient(ctrl)

	adapter := NewPromptRPCAdapter(mockManage, mockExecute)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockManage.EXPECT().ListPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(&manage.ListPromptResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			Prompts:  []*prompt.Prompt{{ID: gptr.Of(int64(1))}},
			Total:    gptr.Of(int32(1)),
		}, nil)

		res, total, err := adapter.ListPrompt(ctx, &rpc.ListPromptParam{SpaceID: gptr.Of(int64(1))})
		assert.NoError(t, err)
		assert.Equal(t, int32(1), *total)
		assert.Len(t, res, 1)
	})

	t.Run("error", func(t *testing.T) {
		mockManage.EXPECT().ListPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
		_, _, err := adapter.ListPrompt(ctx, &rpc.ListPromptParam{})
		assert.Error(t, err)
	})
}

func TestPromptRPCAdapter_ListPromptVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockManage := mocks.NewMockPromptManageClient(ctrl)
	mockExecute := mocks.NewMockPromptExecuteClient(ctrl)

	adapter := NewPromptRPCAdapter(mockManage, mockExecute)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockManage.EXPECT().ListCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return(&manage.ListCommitResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			PromptCommitInfos: []*prompt.CommitInfo{
				{Version: gptr.Of("v1")},
			},
			NextPageToken: gptr.Of("next"),
		}, nil)

		res, next, err := adapter.ListPromptVersion(ctx, &rpc.ListPromptVersionParam{PromptID: 1})
		assert.NoError(t, err)
		assert.Equal(t, "next", next)
		assert.Len(t, res, 1)
	})

	t.Run("no_next_page", func(t *testing.T) {
		mockManage.EXPECT().ListCommit(gomock.Any(), gomock.Any(), gomock.Any()).Return(&manage.ListCommitResponse{
			BaseResp:      &base.BaseResp{StatusCode: 0},
			NextPageToken: nil,
		}, nil)

		res, next, err := adapter.ListPromptVersion(ctx, &rpc.ListPromptVersionParam{PromptID: 1})
		assert.NoError(t, err)
		assert.Equal(t, "", next)
		assert.Len(t, res, 0)
	})
}
