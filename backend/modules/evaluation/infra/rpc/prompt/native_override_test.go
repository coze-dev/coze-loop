// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaltargetdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	promptdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/execute"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/prompt/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/js_conv"
)

func TestNativePromptOverrideGeneratedRuntimeRoundtrip(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  *evaltargetdto.PromptRuntimeParam
		prefix []byte
	}{
		{"old_field_1", &evaltargetdto.PromptRuntimeParam{ModelConfig: &commondto.ModelConfig{
			ModelID:  gptr.Of(int64(9007199254740993)),
			Thinking: &commondto.ThinkingConfig{ThinkingOption: gptr.Of("enabled"), ReasoningEffort: gptr.Of("Tier_V2"), BudgetTokens: gptr.Of(int64(64))},
		}}, []byte{12, 0, 1}},
		{"native_field_2", &evaltargetdto.PromptRuntimeParam{PromptModelConfig: gptr.Of(`{"protocol":"oss_prompt","model_config":{"thinking":{"reasoning_effort":"high"}}}`)}, []byte{11, 0, 2}},
		{"present_empty_string", &evaltargetdto.PromptRuntimeParam{PromptModelConfig: gptr.Of("")}, []byte{11, 0, 2}},
		{"neither_field", &evaltargetdto.PromptRuntimeParam{}, []byte{0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := js_conv.GetMarshaler()(tt.input)
			require.NoError(t, err)
			var jsonDecoded evaltargetdto.PromptRuntimeParam
			require.NoError(t, js_conv.GetUnmarshaler()(encoded, &jsonDecoded))
			require.Equal(t, tt.input, &jsonDecoded)
			wire := make([]byte, tt.input.BLength())
			require.Equal(t, len(wire), tt.input.FastWrite(wire))
			require.Equal(t, tt.prefix, wire[:len(tt.prefix)])
			var thriftDecoded evaltargetdto.PromptRuntimeParam
			read, err := thriftDecoded.FastRead(wire)
			require.NoError(t, err)
			require.Equal(t, len(wire), read)
			require.Equal(t, tt.input, &thriftDecoded)
		})
	}
}

func nativeOSSRuntime(t *testing.T, config *promptdto.ModelConfig) *string {
	t.Helper()
	b, err := js_conv.GetMarshaler()(struct {
		Protocol    string                 `json:"protocol"`
		ModelConfig *promptdto.ModelConfig `json:"model_config"`
	}{"oss_prompt", config})
	require.NoError(t, err)
	return nativeOSSRawRuntime(t, string(b), false)
}

func nativeOSSRawRuntime(t *testing.T, envelope string, legacy bool) *string {
	t.Helper()
	b, err := json.Marshal(envelope)
	require.NoError(t, err)
	fields := map[string]json.RawMessage{"prompt_model_config": b}
	if legacy {
		fields["model_config"] = json.RawMessage(`{"model_id":"123"}`)
	}
	raw, err := json.Marshal(fields)
	require.NoError(t, err)
	return gptr.Of(string(raw))
}

func TestNativePromptOverrideOSSRetainsCompleteConfig(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value *string
	}{
		{"legacy_high", nil},
		{"explicit_auto", gptr.Of("")},
		{"exact_value", gptr.Of("Tier_V2")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := &promptdto.ModelConfig{
				ModelID: gptr.Of(int64(9007199254740993)), MaxTokens: gptr.Of(int32(4096)),
				Temperature: gptr.Of(0.7), TopK: gptr.Of(int32(0)), TopP: gptr.Of(0.8),
				PresencePenalty: gptr.Of(0.2), FrequencyPenalty: gptr.Of(-0.3), JSONMode: gptr.Of(false),
				Extra: gptr.Of(`{"reasoning":{"effort":"low","summary":"auto"},"large":9007199254740993}`),
				Thinking: &promptdto.ThinkingConfig{
					BudgetTokens: gptr.Of(int64(64)), ThinkingOption: gptr.Of(promptdto.ThinkingOptionEnabled),
					ReasoningEffort: gptr.Of(promptdto.ReasoningEffortHigh), ReasoningEffortValue: tt.value,
				},
				ParamConfigValues: []*promptdto.ParamConfigValue{{
					Name: gptr.Of("custom"), Label: gptr.Of("Custom parameter"),
					Value: &promptdto.ParamOption{Value: gptr.Of("literal"), Label: gptr.Of("Literal")},
				}},
			}
			client := mocks.NewMockPromptExecuteClient(gomock.NewController(t))
			client.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *execute.ExecuteInternalRequest, _ ...callopt.Option) (*execute.ExecuteInternalResponse, error) {
				require.NotNil(t, req.OverridePromptParams)
				require.Equal(t, want, req.OverridePromptParams.ModelConfig)
				wire := make([]byte, req.OverridePromptParams.ModelConfig.BLength())
				require.Equal(t, len(wire), req.OverridePromptParams.ModelConfig.FastWrite(wire))
				var decoded promptdto.ModelConfig
				read, err := decoded.FastRead(wire)
				require.NoError(t, err)
				require.Equal(t, len(wire), read)
				require.Equal(t, want, &decoded)
				require.Equal(t, "published-version", req.GetVersion())
				return &execute.ExecuteInternalResponse{}, nil
			})
			_, err := (&PromptRPCAdapter{executeClient: client}).ExecutePrompt(t.Context(), 1, &rpc.ExecutePromptParam{
				PromptID: 11, PromptVersion: "published-version", RuntimeParam: nativeOSSRuntime(t, want),
			})
			require.NoError(t, err)
		})
	}
}

func TestNativePromptOverrideOSSRejectsBeforeRPC(t *testing.T) {
	valid := `{"protocol":"oss_prompt","model_config":{}}`
	for _, tt := range []struct {
		name string
		raw  *string
	}{
		{"invalid_runtime", gptr.Of(`{invalid}`)},
		{"runtime_array", gptr.Of(`[]`)},
		{"runtime_null", gptr.Of(`null`)},
		{"invalid_legacy_shape", gptr.Of(`{"model_config":[]}`)},
		{"both_forms", nativeOSSRawRuntime(t, valid, true)},
		{"non_string_override", gptr.Of(`{"prompt_model_config":{}}`)},
		{"empty_envelope", nativeOSSRawRuntime(t, "", false)},
		{"invalid_envelope", nativeOSSRawRuntime(t, `{invalid}`, false)},
		{"envelope_array", nativeOSSRawRuntime(t, `[]`, false)},
		{"unknown_protocol", nativeOSSRawRuntime(t, `{"protocol":"other","model_config":{}}`, false)},
		{"missing_protocol", nativeOSSRawRuntime(t, `{"model_config":{}}`, false)},
		{"missing_config", nativeOSSRawRuntime(t, `{"protocol":"oss_prompt"}`, false)},
		{"null_config", nativeOSSRawRuntime(t, `{"protocol":"oss_prompt","model_config":null}`, false)},
		{"array_config", nativeOSSRawRuntime(t, `{"protocol":"oss_prompt","model_config":[]}`, false)},
		{"string_config", nativeOSSRawRuntime(t, `{"protocol":"oss_prompt","model_config":"{}"}`, false)},
		{"unknown_envelope_field", nativeOSSRawRuntime(t, `{"protocol":"oss_prompt","model_config":{},"extra":1}`, false)},
		{"unknown_native_field", nativeOSSRawRuntime(t, `{"protocol":"oss_prompt","model_config":{"ModelKey":"cannot-drop"}}`, false)},
		{"unknown_thinking_field", nativeOSSRawRuntime(t, `{"protocol":"oss_prompt","model_config":{"thinking":{"Enabled":false}}}`, false)},
		{"wrong_value_type", nativeOSSRawRuntime(t, `{"protocol":"oss_prompt","model_config":{"thinking":{"reasoning_effort_value":1}}}`, false)},
		{"flow_target_mismatch", nativeOSSRawRuntime(t, `{"protocol":"flow_prompt","model_config":{"ID":"9007199254740993","Thinking":{"Enabled":false}}}`, false)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			client := mocks.NewMockPromptExecuteClient(gomock.NewController(t))
			client.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *execute.ExecuteInternalRequest, ...callopt.Option) (*execute.ExecuteInternalResponse, error) {
				called = true
				return &execute.ExecuteInternalResponse{}, nil
			}).AnyTimes()
			_, err := (&PromptRPCAdapter{executeClient: client}).ExecutePrompt(t.Context(), 1, &rpc.ExecutePromptParam{RuntimeParam: tt.raw})
			require.Error(t, err)
			require.False(t, called, "invalid override must fail before RPC")
		})
	}
}

func TestNativePromptOverrideOSSAbsentUsesPublishedVersion(t *testing.T) {
	for _, raw := range []*string{nil, gptr.Of(`{}`), gptr.Of(`{"model_config":null,"prompt_model_config":null}`)} {
		client := mocks.NewMockPromptExecuteClient(gomock.NewController(t))
		client.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *execute.ExecuteInternalRequest, _ ...callopt.Option) (*execute.ExecuteInternalResponse, error) {
			require.Nil(t, req.OverridePromptParams)
			require.Equal(t, "published-version", req.GetVersion())
			return &execute.ExecuteInternalResponse{}, nil
		})
		_, err := (&PromptRPCAdapter{executeClient: client}).ExecutePrompt(t.Context(), 1, &rpc.ExecutePromptParam{PromptVersion: "published-version", RuntimeParam: raw})
		require.NoError(t, err)
	}
}
