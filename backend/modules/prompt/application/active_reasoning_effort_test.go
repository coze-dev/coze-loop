// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type promptEffortReaderFunc func(context.Context, int64, int64) (bool, []string, error)

func (f promptEffortReaderFunc) GetReasoningEffortOptions(ctx context.Context, spaceID, modelID int64) (bool, []string, error) {
	return f(ctx, spaceID, modelID)
}

func TestActiveReasoningEffortPublishEntrypoints(t *testing.T) {
	for _, isOpenAPI := range []bool{false, true} {
		for _, value := range []string{"Outside", "Retired"} {
			t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI]+"/"+value, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				mr, ps := repomocks.NewMockIManageRepo(ctrl), servicemocks.NewMockIPromptService(ctrl)
				auth, user := rpcmocks.NewMockIAuthProvider(ctrl), rpcmocks.NewMockIUserProvider(ctrl)
				draft := &entity.Prompt{ID: 123, SpaceID: 100, PromptBasic: &entity.PromptBasic{LatestVersion: "1.0.0"}, PromptDraft: &entity.PromptDraft{DraftInfo: &entity.DraftInfo{UserID: "u", BaseVersion: "1.0.0"}, PromptDetail: &entity.PromptDetail{ModelConfig: &entity.ModelConfig{ModelID: 111, Thinking: &entity.ThinkingConfig{ReasoningEffortValue: ptr.Of(value)}}}}}
				base := &entity.Prompt{ID: 123, SpaceID: 100, PromptCommit: &entity.PromptCommit{PromptDetail: &entity.PromptDetail{ModelConfig: &entity.ModelConfig{ModelID: 111, Thinking: &entity.ThinkingConfig{ReasoningEffortValue: ptr.Of("Retired")}}}}}
				authorized, queries, wrote := false, 0, false
				mr.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param repo.GetPromptParam) (*entity.Prompt, error) {
					if param.WithCommit {
						require.True(t, authorized)
						require.Equal(t, "1.0.0", param.CommitVersion)
						return base, nil
					}
					return draft, nil
				}).AnyTimes()
				mr.EXPECT().CommitDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, repo.CommitDraftParam) error { wrote = true; return nil }).AnyTimes()
				reader := promptEffortReaderFunc(func(_ context.Context, spaceID, modelID int64) (bool, []string, error) {
					require.True(t, authorized)
					require.Equal(t, int64(100), spaceID)
					require.Equal(t, int64(111), modelID)
					queries++
					return true, []string{"Allowed"}, nil
				})
				ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u"})
				var err error
				if isOpenAPI {
					auth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, []int64, string) error { authorized = true; return nil })
					user.EXPECT().GetUserIdInCtx(gomock.Any()).Return("u", true)
					app := &PromptOpenAPIApplicationImpl{promptManageRepo: mr, promptService: ps, auth: auth, user: user, modelReasoningEffortProvider: reader}
					_, err = app.CommitDraftOApi(ctx, &openapi.CommitDraftOApiRequest{PromptID: ptr.Of(int64(123)), CommitVersion: ptr.Of("1.0.1")})
				} else {
					auth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, []int64, string) error { authorized = true; return nil })
					audit := rpcmocks.NewMockIAuditProvider(ctrl)
					audit.EXPECT().AuditPrompt(gomock.Any(), gomock.Any()).Return(nil)
					app := &PromptManageApplicationImpl{manageRepo: mr, promptService: ps, authRPCProvider: auth, auditRPCProvider: audit, modelReasoningEffortProvider: reader}
					_, err = app.CommitDraft(ctx, &manage.CommitDraftRequest{PromptID: ptr.Of(int64(123)), CommitVersion: ptr.Of("1.0.1")})
				}
				if value == "Outside" {
					require.Error(t, err)
					require.False(t, wrote)
					require.Equal(t, 1, queries)
				} else {
					require.NoError(t, err)
					require.True(t, wrote)
					require.Zero(t, queries)
				}
				require.Equal(t, value, *draft.PromptDraft.PromptDetail.ModelConfig.Thinking.ReasoningEffortValue)
			})
		}
	}
}

func TestActiveReasoningEffortSaveEntrypoints(t *testing.T) {
	queryFailure := errors.New("model query failure")
	cases := []struct {
		name       string
		modelID    int64
		value      *string
		enabled    bool
		options    []string
		queryError error
		wantQuery  bool
		wantError  bool
	}{
		{"outside", 222, ptr.Of("Outside"), true, []string{"Allowed"}, nil, true, true},
		{"inside", 222, ptr.Of("Allowed"), true, []string{"Allowed"}, nil, true, false},
		{"same_model_changed", 111, ptr.Of("Allowed"), true, []string{"Allowed"}, nil, true, false},
		{"deleted_historical_same_value", 111, ptr.Of("Retired"), true, []string{"Allowed"}, nil, false, false},
		{"omitted_effort", 111, nil, true, nil, queryFailure, false, false},
		{"Auto", 222, ptr.Of(""), false, nil, queryFailure, false, false},
		{"none_is_value", 222, ptr.Of("none"), true, []string{"none"}, nil, true, false},
		{"case_exact", 222, ptr.Of("High"), true, []string{"high"}, nil, true, true},
		{"no_capability", 222, ptr.Of("high"), false, nil, nil, true, true},
		{"nil_options_fallback", 222, ptr.Of("high"), true, nil, nil, true, false},
		{"query_failure", 222, ptr.Of("Allowed"), true, nil, queryFailure, true, true},
	}
	for _, isOpenAPI := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI]+"/"+tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				mr, ps := repomocks.NewMockIManageRepo(ctrl), servicemocks.NewMockIPromptService(ctrl)
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				previous := &entity.Prompt{ID: 123, SpaceID: 100, PromptDraft: &entity.PromptDraft{DraftInfo: &entity.DraftInfo{UserID: "u"}, PromptDetail: &entity.PromptDetail{ModelConfig: &entity.ModelConfig{ModelID: 111, Thinking: &entity.ThinkingConfig{ReasoningEffortValue: ptr.Of("Retired")}}}}}
				mr.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(previous, nil)
				authorized, queries, wrote := false, 0, false
				reader := promptEffortReaderFunc(func(_ context.Context, spaceID, modelID int64) (bool, []string, error) {
					require.True(t, authorized)
					require.Equal(t, int64(100), spaceID)
					require.Equal(t, tc.modelID, modelID)
					queries++
					return tc.enabled, tc.options, tc.queryError
				})
				ps.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, saved *entity.Prompt) (*entity.DraftInfo, error) {
					wrote = true
					want := tc.value
					if want == nil {
						want = ptr.Of("Retired")
					}
					require.Equal(t, *want, *saved.PromptDraft.PromptDetail.ModelConfig.Thinking.ReasoningEffortValue)
					return &entity.DraftInfo{}, nil
				}).AnyTimes()
				ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u"})
				var err error
				if isOpenAPI {
					auth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, []int64, string) error { authorized = true; return nil })
					user := rpcmocks.NewMockIUserProvider(ctrl)
					user.EXPECT().GetUserIdInCtx(gomock.Any()).Return("u", true)
					app := &PromptOpenAPIApplicationImpl{promptManageRepo: mr, promptService: ps, auth: auth, user: user, modelReasoningEffortProvider: reader}
					_, err = app.SaveDraftOApi(ctx, &openapi.SaveDraftOApiRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &oapiprompt.PromptDraft{DraftInfo: &oapiprompt.DraftInfo{}, Detail: &oapiprompt.PromptDetail{ModelConfig: &oapiprompt.ModelConfig{ModelID: ptr.Of(tc.modelID), Thinking: &oapiprompt.ThinkingConfig{ReasoningEffortValue: tc.value}}}}})
				} else {
					auth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, []int64, string) error { authorized = true; return nil })
					app := &PromptManageApplicationImpl{manageRepo: mr, promptService: ps, authRPCProvider: auth, modelReasoningEffortProvider: reader}
					_, err = app.SaveDraft(ctx, &manage.SaveDraftRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &webprompt.PromptDraft{DraftInfo: &webprompt.DraftInfo{}, Detail: &webprompt.PromptDetail{ModelConfig: &webprompt.ModelConfig{ModelID: ptr.Of(tc.modelID), Thinking: &webprompt.ThinkingConfig{ReasoningEffortValue: tc.value}}}}})
				}
				if tc.wantError {
					require.Error(t, err)
					require.False(t, wrote)
					if tc.queryError != nil {
						require.ErrorIs(t, err, tc.queryError)
					}
				} else {
					require.NoError(t, err)
					require.True(t, wrote)
				}
				require.Equal(t, map[bool]int{false: 0, true: 1}[tc.wantQuery], queries)
				require.Equal(t, "Retired", *previous.PromptDraft.PromptDetail.ModelConfig.Thinking.ReasoningEffortValue)
			})
		}
	}
}
