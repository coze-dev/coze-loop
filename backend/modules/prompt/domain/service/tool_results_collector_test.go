// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestToolResultsCollector_CollectToolResults(t *testing.T) {
	collector := service.NewToolResultsCollector()

	t.Run("nil mock tools returns empty map", func(t *testing.T) {
		got, err := collector.CollectToolResults(context.Background(), service.CollectToolResultsParam{
			MockTools: nil,
		})
		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("builds tool result map", func(t *testing.T) {
		got, err := collector.CollectToolResults(context.Background(), service.CollectToolResultsParam{
			MockTools: []*entity.MockTool{
				{Name: "tool_a", MockResponse: "{\"ok\":true}"},
				{Name: "tool_b", MockResponse: "b"},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"tool_a": "{\"ok\":true}",
			"tool_b": "b",
		}, got)
	})

	t.Run("skips nil and empty name entries", func(t *testing.T) {
		got, err := collector.CollectToolResults(context.Background(), service.CollectToolResultsParam{
			MockTools: []*entity.MockTool{
				nil,
				{Name: "", MockResponse: "ignored"},
				{Name: "tool_a", MockResponse: "a"},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"tool_a": "a",
		}, got)
	})
}

func TestPromptServiceImpl_Execute_UsesToolResultsCollector(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockToolConfigProvider := servicemocks.NewMockIToolConfigProvider(ctrl)
	mockToolResultsCollector := servicemocks.NewMockIToolResultsCollector(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockLLM := rpcmocks.NewMockILLMProvider(ctrl)

	prompt := &entity.Prompt{
		ID:        2,
		SpaceID:   1,
		PromptKey: "prompt_key",
	}
	mockTools := []*entity.MockTool{
		{Name: "tool_a", MockResponse: "a"},
	}

	mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123456789), nil)
	mockToolConfigProvider.EXPECT().
		GetToolConfig(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil, nil)
	mockLLM.EXPECT().
		Call(gomock.Any(), gomock.Any()).
		Return(&entity.ReplyItem{
			Message: &entity.Message{
				Role:    entity.RoleAssistant,
				Content: ptr.Of("ok"),
			},
		}, nil)
	mockToolResultsCollector.EXPECT().
		CollectToolResults(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, param service.CollectToolResultsParam) (map[string]string, error) {
			assert.Same(t, prompt, param.Prompt)
			assert.Equal(t, mockTools, param.MockTools)
			assert.NotNil(t, param.Reply)
			assert.NotNil(t, param.Reply.Item)
			return map[string]string{"tool_a": "a"}, nil
		})

	svc := service.NewPromptService(
		service.NewPromptFormatter(),
		mockToolConfigProvider,
		mockToolResultsCollector,
		mockIDGen,
		nil,
		nil,
		nil,
		nil,
		nil,
		mockLLM,
		nil,
		service.NewCozeLoopSnippetParser(),
	)

	reply, err := svc.Execute(context.Background(), service.ExecuteParam{
		Prompt:         prompt,
		SingleStep:     true,
		DisableTracing: true,
		MockTools:      mockTools,
	})
	assert.NoError(t, err)
	assert.NotNil(t, reply)
	assert.NotNil(t, reply.Item)
	assert.NotNil(t, reply.Item.Message)
	assert.Equal(t, "ok", ptr.From(reply.Item.Message.Content))
}
