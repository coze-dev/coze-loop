package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	oapicommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	oapieval "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/evaluator"
	evaluatorapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	oapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	usermocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func existingAgentForPreservation() *entity.Evaluator {
	return &entity.Evaluator{ID: 111, SpaceID: 7590122197455058946, EvaluatorType: entity.EvaluatorTypeAgent, AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{ID: 222, SpaceID: 7590122197455058946, EvaluatorID: 111, Version: "draft", BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(17)), CreatedBy: &entity.UserInfo{UserID: gptr.Of("owner")}}, ModelConfig: &entity.ModelConfig{ModelID: gptr.Of(int64(333)), Thinking: &entity.ThinkingConfig{ReasoningEffort: gptr.Of("Historical_Tier")}, JSONExt: gptr.Of(`{"stop":["END"]}`)}, AgentConfig: &entity.AgentConfig{AgentType: entity.AgentType_Vibe, Runtime: gptr.Of("codex_cli"), SearchProvider: gptr.Of("serp_api")}}}
}

func TestWebDraftTextUpdatePreservesVersionConfiguration(t *testing.T) {
	for _, auto := range []bool{false, true} {
		t.Run(map[bool]string{false: "omitted", true: "explicit_auto"}[auto], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := servicemocks.NewMockEvaluatorService(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			users := usermocks.NewMockUserInfoService(ctrl)
			conf := confmocks.NewMockIConfiger(ctrl)
			previous := existingAgentForPreservation()
			snapshot, _ := json.Marshal(previous)
			svc.EXPECT().GetEvaluator(gomock.Any(), previous.SpaceID, previous.ID, false).Return(previous, nil)
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationParam) error {
				require.Equal(t, previous.SpaceID, p.SpaceID)
				return nil
			})
			conf.EXPECT().CheckAgentEvaluatorWritable(gomock.Any()).Return(true, nil)
			var stored *entity.Evaluator
			svc.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, v *entity.Evaluator) error { stored = v; return nil })
			users.EXPECT().PackUserInfo(gomock.Any(), gomock.Any())
			request := &evaluatorapi.UpdateEvaluatorDraftRequest{WorkspaceID: previous.SpaceID, EvaluatorID: previous.ID, EvaluatorType: evaluatordto.EvaluatorType_Agent, EvaluatorContent: &evaluatordto.EvaluatorContent{AgentEvaluator: &evaluatordto.AgentEvaluator{PromptConfig: &evaluatordto.AgentEvaluatorPromptConfig{MessageList: []*commondto.Message{{Content: &commondto.Content{Text: gptr.Of("new prompt")}}}}}}}
			if auto {
				request.EvaluatorContent.AgentEvaluator.ModelConfig = &commondto.ModelConfig{ModelID: gptr.Of(int64(333)), Thinking: &commondto.ThinkingConfig{ReasoningEffort: gptr.Of("")}}
			}
			handler := &EvaluatorHandlerImpl{evaluatorService: svc, auth: auth, userInfoService: users, configer: conf}
			response, err := handler.UpdateEvaluatorDraft(t.Context(), request)
			require.NoError(t, err)
			require.NotNil(t, stored.AgentEvaluatorVersion.ModelConfig)
			want := "Historical_Tier"
			if auto {
				want = ""
			}
			require.Equal(t, want, gptr.Indirect(stored.AgentEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort))
			require.Equal(t, "codex_cli", gptr.Indirect(stored.AgentEvaluatorVersion.AgentConfig.Runtime))
			require.Equal(t, "serp_api", gptr.Indirect(stored.AgentEvaluatorVersion.AgentConfig.SearchProvider))
			require.Equal(t, previous.AgentEvaluatorVersion.ModelConfig.JSONExt, stored.AgentEvaluatorVersion.ModelConfig.JSONExt)
			require.Equal(t, want, response.Evaluator.CurrentVersion.EvaluatorContent.AgentEvaluator.ModelConfig.Thinking.GetReasoningEffort())
			after, _ := json.Marshal(previous)
			require.Equal(t, snapshot, after)
			require.Equal(t, stored.SpaceID, previous.SpaceID)
		})
	}
}

func TestOpenAPIDraftTextUpdatePreservesVersionConfiguration(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := servicemocks.NewMockEvaluatorService(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	previous := existingAgentForPreservation()
	snapshot, _ := json.Marshal(previous)
	svc.EXPECT().GetEvaluator(gomock.Any(), previous.SpaceID, previous.ID, false).Return(previous, nil)
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
	var stored *entity.Evaluator
	svc.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, v *entity.Evaluator) error { stored = v; return nil })
	handler := &EvalOpenAPIApplication{evaluatorService: svc, auth: auth, metric: &fakeOpenAPIMetric{}}
	request := &oapi.UpdateEvaluatorDraftOApiRequest{WorkspaceID: gptr.Of(previous.SpaceID), EvaluatorID: gptr.Of(previous.ID), EvaluatorType: gptr.Of(oapieval.EvaluatorTypeAgent), EvaluatorContent: &oapieval.EvaluatorContent{AgentEvaluator: &oapieval.AgentEvaluator{PromptConfig: &oapieval.AgentEvaluatorPromptConfig{MessageList: []*oapicommon.Message{{Content: &oapicommon.Content{Text: gptr.Of("new prompt")}}}}}}}
	response, err := handler.UpdateEvaluatorDraftOApi(t.Context(), request)
	require.NoError(t, err)
	require.NotNil(t, stored.AgentEvaluatorVersion.ModelConfig)
	require.Equal(t, "Historical_Tier", gptr.Indirect(stored.AgentEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort))
	require.Equal(t, "codex_cli", gptr.Indirect(stored.AgentEvaluatorVersion.AgentConfig.Runtime))
	require.Equal(t, previous.AgentEvaluatorVersion.BaseInfo, stored.AgentEvaluatorVersion.BaseInfo)
	require.Equal(t, "Historical_Tier", response.Data.Evaluator.CurrentVersion.EvaluatorContent.AgentEvaluator.ModelConfig.Thinking.GetReasoningEffort())
	after, _ := json.Marshal(previous)
	require.Equal(t, snapshot, after)
}
