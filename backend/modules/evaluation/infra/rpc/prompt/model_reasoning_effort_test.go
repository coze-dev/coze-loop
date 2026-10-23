package prompt

import (
	"context"
	"github.com/bytedance/gg/gptr"
	callopt "github.com/cloudwego/kitex/client/callopt"
	promptdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/execute"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/prompt/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestPromptATypedThinkingAndExtraCrossesOSSInternalRPC(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mocks.NewMockPromptExecuteClient(ctrl)
	adapter := &PromptRPCAdapter{executeClient: client}
	raw := `{"model_config":{"model_id":"7590122197455058946","thinking":{"reasoning_effort":"Tier_V2","thinking_option":"enabled"},"json_ext":"{\"reasoning\":{\"summary\":\"auto\"}}"}}`
	client.EXPECT().ExecuteInternal(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *execute.ExecuteInternalRequest, _ ...callopt.Option) (*execute.ExecuteInternalResponse, error) {
		require.NotNil(t, req.OverridePromptParams.ModelConfig.Thinking)
		require.Equal(t, "Tier_V2", string(*req.OverridePromptParams.ModelConfig.Thinking.ReasoningEffort))
		require.Equal(t, gptr.Of(promptdto.ThinkingOption("enabled")), req.OverridePromptParams.ModelConfig.Thinking.ThinkingOption)
		require.JSONEq(t, `{"reasoning":{"summary":"auto"}}`, gptr.Indirect(req.OverridePromptParams.ModelConfig.Extra))
		return &execute.ExecuteInternalResponse{}, nil
	})
	_, err := adapter.ExecutePrompt(t.Context(), 7590122197455058946, &rpc.ExecutePromptParam{PromptID: 11, RuntimeParam: gptr.Of(raw)})
	require.NoError(t, err)
}
