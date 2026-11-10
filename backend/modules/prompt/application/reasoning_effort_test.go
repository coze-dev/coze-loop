// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	webprompt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	oapiprompt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestReasoningEffort_SaveDraftOmittedModelPreserved(t *testing.T) {
	for _, isOpenAPI := range []bool{false, true} {
		t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mr := repomocks.NewMockIManageRepo(ctrl)
			ps := servicemocks.NewMockIPromptService(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			previous := &entity.Prompt{ID: 123, SpaceID: 100, PromptDraft: &entity.PromptDraft{DraftInfo: &entity.DraftInfo{UserID: "u"}, PromptDetail: &entity.PromptDetail{ModelConfig: &entity.ModelConfig{ModelID: 111, Extra: ptr.Of(`{"reasoning":{"summary":"auto"}}`), Thinking: &entity.ThinkingConfig{ReasoningEffort: ptr.Of(entity.ReasoningEffort("tier_v2"))}}}}}
			mr.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p repo.GetPromptParam) (*entity.Prompt, error) {
				assert.True(t, p.WithDraft)
				assert.Equal(t, "u", p.UserID)
				return previous, nil
			})
			ps.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.Prompt) (*entity.DraftInfo, error) {
				require.NotNil(t, p.PromptDraft.PromptDetail.ModelConfig)
				assert.Equal(t, "tier_v2", string(*p.PromptDraft.PromptDetail.ModelConfig.Thinking.ReasoningEffort))
				assert.Equal(t, previous.PromptDraft.PromptDetail.ModelConfig.Extra, p.PromptDraft.PromptDetail.ModelConfig.Extra)
				assert.NotSame(t, previous.PromptDraft.PromptDetail.ModelConfig, p.PromptDraft.PromptDetail.ModelConfig)
				return &entity.DraftInfo{}, nil
			})
			if isOpenAPI {
				auth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				user := rpcmocks.NewMockIUserProvider(ctrl)
				user.EXPECT().GetUserIdInCtx(gomock.Any()).Return("u", true)
				app := &PromptOpenAPIApplicationImpl{promptManageRepo: mr, promptService: ps, auth: auth, user: user}
				ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u"})
				_, err := app.SaveDraftOApi(ctx, &openapi.SaveDraftOApiRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &oapiprompt.PromptDraft{DraftInfo: &oapiprompt.DraftInfo{}, Detail: &oapiprompt.PromptDetail{}}})
				require.NoError(t, err)
			} else {
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				app := &PromptManageApplicationImpl{manageRepo: mr, promptService: ps, authRPCProvider: auth}
				ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u"})
				_, err := app.SaveDraft(ctx, &manage.SaveDraftRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &webprompt.PromptDraft{DraftInfo: &webprompt.DraftInfo{}, Detail: &webprompt.PromptDetail{}}})
				require.NoError(t, err)
			}
			assert.Equal(t, "tier_v2", string(*previous.PromptDraft.PromptDetail.ModelConfig.Thinking.ReasoningEffort))
		})
	}
}
