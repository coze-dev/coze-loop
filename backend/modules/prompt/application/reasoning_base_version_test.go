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
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestReasoningEffort_FirstDraftUsesReferencedCommitModel(t *testing.T) {
	ctrl := gomock.NewController(t)
	mr := repomocks.NewMockIManageRepo(ctrl)
	ps := servicemocks.NewMockIPromptService(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	base := &entity.Prompt{ID: 123, SpaceID: 100, PromptBasic: &entity.PromptBasic{LatestVersion: "3"}}
	mr.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123, WithDraft: true, UserID: "u"}).Return(base, nil)
	mr.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123, WithCommit: true, CommitVersion: "2"}).Return(&entity.Prompt{PromptCommit: &entity.PromptCommit{PromptDetail: &entity.PromptDetail{ModelConfig: &entity.ModelConfig{ModelID: 111, Thinking: &entity.ThinkingConfig{ReasoningEffort: ptr.Of(entity.ReasoningEffort("tier_v2"))}}}}}, nil)
	auth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	ps.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.Prompt) (*entity.DraftInfo, error) {
		require.NotNil(t, p.PromptDraft.PromptDetail.ModelConfig)
		assert.Equal(t, "tier_v2", string(*p.PromptDraft.PromptDetail.ModelConfig.Thinking.ReasoningEffort))
		return &entity.DraftInfo{}, nil
	})
	app := &PromptManageApplicationImpl{manageRepo: mr, promptService: ps, authRPCProvider: auth}
	_, err := app.SaveDraft(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &manage.SaveDraftRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &webprompt.PromptDraft{DraftInfo: &webprompt.DraftInfo{BaseVersion: ptr.Of("2")}, Detail: &webprompt.PromptDetail{}}})
	require.NoError(t, err)
}
