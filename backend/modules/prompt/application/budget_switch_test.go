// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	webprompt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	oapiprompt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestSpecificValueSaveDraftWebAndOpenAPI(t *testing.T) {
	for _, openAPI := range []bool{false, true} {
		t.Run(map[bool]string{false: "web", true: "openapi"}[openAPI], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mr := repomocks.NewMockIManageRepo(ctrl)
			ps := servicemocks.NewMockIPromptService(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			queries := 0
			models := promptEffortReaderFunc(func(_ context.Context, spaceID, modelID int64) (bool, []string, error) {
				require.Equal(t, int64(100), spaceID)
				require.Equal(t, int64(111), modelID)
				queries++
				return true, []string{"xhigh"}, nil
			})
			old := &entity.ModelConfig{ModelID: 111, Thinking: &entity.ThinkingConfig{ThinkingOption: ptr.Of(entity.ThinkingOptionEnabled), BudgetTokens: ptr.Of(int64(2048))}}
			previous := &entity.Prompt{ID: 123, SpaceID: 100, PromptDraft: &entity.PromptDraft{DraftInfo: &entity.DraftInfo{UserID: "u"}, PromptDetail: &entity.PromptDetail{ModelConfig: old}}}
			mr.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(previous, nil)
			ps.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.Prompt) (*entity.DraftInfo, error) {
				model := p.PromptDraft.PromptDetail.ModelConfig
				require.Nil(t, model.Thinking.BudgetTokens)
				data, err := json.Marshal(model.Thinking)
				require.NoError(t, err)
				var values map[string]any
				require.NoError(t, json.Unmarshal(data, &values))
				require.Equal(t, "xhigh", values["reasoning_effort_value"])
				require.NotContains(t, values, "reasoning_effort")
				return &entity.DraftInfo{}, nil
			})
			ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u"})
			if openAPI {
				var thinking oapiprompt.ThinkingConfig
				require.NoError(t, json.Unmarshal([]byte(`{"reasoning_effort_value":"xhigh"}`), &thinking))
				auth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				user := rpcmocks.NewMockIUserProvider(ctrl)
				user.EXPECT().GetUserIdInCtx(gomock.Any()).Return("u", true)
				app := &PromptOpenAPIApplicationImpl{promptManageRepo: mr, promptService: ps, auth: auth, user: user, modelReasoningEffortProvider: models}
				_, err := app.SaveDraftOApi(ctx, &openapi.SaveDraftOApiRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &oapiprompt.PromptDraft{DraftInfo: &oapiprompt.DraftInfo{}, Detail: &oapiprompt.PromptDetail{ModelConfig: &oapiprompt.ModelConfig{ModelID: ptr.Of(int64(111)), Thinking: &thinking}}}})
				require.NoError(t, err)
			} else {
				var thinking webprompt.ThinkingConfig
				require.NoError(t, json.Unmarshal([]byte(`{"reasoning_effort_value":"xhigh"}`), &thinking))
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				app := &PromptManageApplicationImpl{manageRepo: mr, promptService: ps, authRPCProvider: auth, modelReasoningEffortProvider: models}
				_, err := app.SaveDraft(ctx, &manage.SaveDraftRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &webprompt.PromptDraft{DraftInfo: &webprompt.DraftInfo{}, Detail: &webprompt.PromptDetail{ModelConfig: &webprompt.ModelConfig{ModelID: ptr.Of(int64(111)), Thinking: &thinking}}}})
				require.NoError(t, err)
			}
			require.Equal(t, int64(2048), *old.Thinking.BudgetTokens)
			require.Equal(t, 1, queries)
		})
	}
}
