package prompt

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	callopt "github.com/cloudwego/kitex/client/callopt"
	evaluationdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/execute"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/prompt/mocks"
	promptconvertor "github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/js_conv"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestExecuteInternalWirePreservesEvaluationEffort(t *testing.T) {
	for _, tc := range []struct {
		name   string
		effort *string
		want   *string
	}{
		{name: "legacy_nil"},
		{name: "auto", effort: gptr.Of(""), want: gptr.Of("")},
		{name: "none", effort: gptr.Of("none"), want: gptr.Of("none")},
		{name: "custom", effort: gptr.Of("Tier_V2"), want: gptr.Of("Tier_V2")},
		{name: "uppercase_known", effort: gptr.Of("High"), want: gptr.Of("High")},
		{name: "lowercase_known", effort: gptr.Of("high"), want: gptr.Of("high")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := &evaluationdto.ModelConfig{
				ModelID:     gptr.Of(int64(7590122197455058946)),
				Temperature: gptr.Of(0.37),
				MaxTokens:   gptr.Of(int32(1024)),
				TopP:        gptr.Of(0.83),
				Thinking: &evaluationdto.ThinkingConfig{
					ThinkingOption:  gptr.Of("enabled"),
					ReasoningEffort: tc.effort,
				},
				JSONExt: gptr.Of(`{"reasoning":{"summary":"auto"},"metadata":{"id":9007199254740993}}`),
			}
			before, err := js_conv.GetMarshaler()(model)
			require.NoError(t, err)
			raw := `{"model_config":` + string(before) + `}`
			param := &rpc.ExecutePromptParam{
				PromptID:      11,
				PromptVersion: "1.2.3",
				RuntimeParam:  gptr.Of(raw),
			}

			client := mocks.NewMockPromptExecuteClient(gomock.NewController(t))
			adapter := NewPromptRPCAdapter(nil, client)
			client.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, req *execute.ExecuteInternalRequest, _ ...callopt.Option) (*execute.ExecuteInternalResponse, error) {
					wire := make([]byte, req.BLength())
					require.Equal(t, len(wire), req.FastWriteNocopy(wire, nil))
					decoded := execute.NewExecuteInternalRequest()
					consumed, err := decoded.FastRead(wire)
					require.NoError(t, err)
					require.Equal(t, len(wire), consumed)
					require.Equal(t, int64(11), decoded.GetPromptID())
					require.Equal(t, int64(7590122197455058946), decoded.GetWorkspaceID())
					require.Equal(t, "1.2.3", decoded.GetVersion())

					config := decoded.OverridePromptParams.ModelConfig
					require.Equal(t, int64(7590122197455058946), config.GetModelID())
					require.Equal(t, gptr.Of(0.37), config.Temperature)
					require.Equal(t, gptr.Of(int32(1024)), config.MaxTokens)
					require.Equal(t, gptr.Of(0.83), config.TopP)
					require.Equal(t, `{"reasoning":{"summary":"auto"},"metadata":{"id":9007199254740993}}`, config.GetExtra())
					require.NotNil(t, config.Thinking)
					require.Equal(t, tc.want, config.Thinking.ReasoningEffortValue)
					require.Nil(t, config.Thinking.ReasoningEffort)
					require.Equal(t, "enabled", string(config.Thinking.GetThinkingOption()))

					consumer := promptconvertor.ThinkingConfigDTO2DO(config.Thinking)
					require.Equal(t, tc.want, consumer.ReasoningEffortValue)
					require.Nil(t, consumer.ReasoningEffort)
					return execute.NewExecuteInternalResponse(), nil
				},
			)
			_, err = adapter.ExecutePrompt(t.Context(), 7590122197455058946, param)
			require.NoError(t, err)
			require.Equal(t, raw, gptr.Indirect(param.RuntimeParam))
			after, err := js_conv.GetMarshaler()(model)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}
