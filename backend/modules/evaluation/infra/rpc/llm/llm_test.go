// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/cloudwego/kitex/client/callopt"
	llmcommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	runtimedto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime/llmruntimeservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
)

// Only the network client is replaced; Call and all request conversion remain real.
type recordingLLMClient struct {
	llmruntimeservice.Client
	requests []*runtime.ChatRequest
}

func (c *recordingLLMClient) Chat(_ context.Context, request *runtime.ChatRequest, _ ...callopt.Option) (*runtime.ChatResponse, error) {
	c.requests = append(c.requests, request)
	return &runtime.ChatResponse{Message: &runtimedto.Message{Content: gptr.Of("OK")}}, nil
}

func TestCallRejectsThinkingUnsupportedByLegacyLLMProtocol(t *testing.T) {
	for _, tc := range []struct {
		name     string
		thinking *entity.ThinkingConfig
	}{
		{"raw", &entity.ThinkingConfig{ReasoningEffort: gptr.Of("Private_Tier")}},
		{"auto", &entity.ThinkingConfig{ReasoningEffort: gptr.Of("")}},
		{"disabled", &entity.ThinkingConfig{ThinkingOption: gptr.Of("disabled")}},
		{"budget", &entity.ThinkingConfig{BudgetTokens: gptr.Of(int64(1024))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &recordingLLMClient{}
			adapter := NewLLMRPCProvider(client)
			param := &entity.LLMCallParam{ModelConfig: &entity.ModelConfig{ModelID: gptr.Of(int64(7590122197455058946)), Thinking: tc.thinking}}
			before, err := json.Marshal(param)
			require.NoError(t, err)
			reply, err := adapter.Call(t.Context(), param)
			require.Error(t, err)
			require.Nil(t, reply)
			require.ErrorContains(t, err, "thinking configuration is not supported")
			statusErr, ok := errorx.FromStatusError(err)
			require.True(t, ok)
			require.Equal(t, int32(errno.InvalidModelConfigCode), statusErr.Code())
			require.Empty(t, client.requests)
			after, err := json.Marshal(param)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestCallWithLegacyNilThinkingKeepsOriginalRequest(t *testing.T) {
	client := &recordingLLMClient{}
	adapter := NewLLMRPCProvider(client)
	param := &entity.LLMCallParam{
		SpaceID: 7590122197455058946, UserID: gptr.Of("user-123"), EvaluatorID: "evaluator-456", Scenario: entity.ScenarioEvaluator,
		ModelConfig: &entity.ModelConfig{ModelID: gptr.Of(int64(7590122197455058003)), Temperature: gptr.Of(0.7), MaxTokens: gptr.Of(int32(4096)), TopP: gptr.Of(0.9), Protocol: gptr.Of("openai"), PresetModel: gptr.Of(false)},
		Messages:    []*entity.Message{{Role: entity.RoleUser, Content: &entity.Content{Text: gptr.Of("legacy request")}}},
	}
	before, err := json.Marshal(param)
	require.NoError(t, err)
	reply, err := adapter.Call(t.Context(), param)
	require.NoError(t, err)
	require.Equal(t, "OK", gptr.Indirect(reply.Content))
	require.Len(t, client.requests, 1)
	want := &runtime.ChatRequest{
		ModelConfig: &runtimedto.ModelConfig{ModelID: 7590122197455058003, Temperature: gptr.Of(0.7), MaxTokens: gptr.Of(int64(4096)), TopP: gptr.Of(0.9), Protocol: gptr.Of("openai"), PresetModel: gptr.Of(false)},
		Messages:    []*runtimedto.Message{{Role: runtimedto.RoleUser, Content: gptr.Of("legacy request")}},
		BizParam:    &runtimedto.BizParam{WorkspaceID: gptr.Of(int64(7590122197455058946)), UserID: gptr.Of("user-123"), Scenario: gptr.Of(llmcommon.ScenarioEvaluator), ScenarioEntityID: gptr.Of("evaluator-456")},
	}
	require.Equal(t, want, client.requests[0])
	after, err := json.Marshal(param)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
