package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	web "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	oapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	domainservice "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestInitialDraftWritesValidateModelBeforeRepository(t *testing.T) {
	cases := []struct{ name, thinking string }{
		{"conflict", `{"thinking_option":"enabled","reasoning_effort_value":"high","budget_tokens":2048}`},
		{"disabled", `{"thinking_option":"disabled","reasoning_effort_value":"high","budget_tokens":2048}`},
		{"legacy", `{"thinking_option":"enabled","reasoning_effort":"high","budget_tokens":2048}`},
		{"auto", `{"thinking_option":"enabled","reasoning_effort_value":"","budget_tokens":2048}`},
		{"model_only", ""},
		{"empty_config", ""},
		{"nil_config", ""},
	}
	for _, entry := range []string{"web_create", "web_save", "openapi_save"} {
		for _, tc := range cases {
			t.Run(entry+"/"+tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				mr := repomocks.NewMockIManageRepo(ctrl)
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				previous := &entity.Prompt{ID: 123, SpaceID: 100, PromptBasic: &entity.PromptBasic{}}
				previousBefore, err := json.Marshal(previous)
				require.NoError(t, err)
				var model *entity.ModelConfig
				var stored []byte
				writes := 0
				capture := func(p *entity.Prompt) {
					writes++
					stored, err = json.Marshal(p.PromptDraft.PromptDetail.ModelConfig)
					require.NoError(t, err)
					require.NoError(t, json.Unmarshal(stored, &model))
				}
				if entry == "web_create" {
					mr.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.Prompt) (int64, error) { capture(p); return 123, nil }).AnyTimes()
				} else {
					mr.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(previous, nil).Times(1)
					mr.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.Prompt) (*entity.DraftInfo, error) {
						capture(p)
						return &entity.DraftInfo{}, nil
					}).AnyTimes()
				}
				ps := domainservice.NewPromptService(nil, nil, nil, nil, nil, nil, mr, nil, nil, nil, nil, nil)
				ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u"})
				var before, after []byte
				var callErr error
				if entry == "openapi_save" {
					var thinking oapi.ThinkingConfig
					cfg := &oapi.ModelConfig{ModelID: ptr.Of(int64(111))}
					if tc.thinking != "" {
						require.NoError(t, json.Unmarshal([]byte(tc.thinking), &thinking))
						cfg.Thinking = &thinking
					}
					if tc.name == "empty_config" {
						cfg = &oapi.ModelConfig{}
					}
					if tc.name == "nil_config" {
						cfg = nil
					}
					before, err = json.Marshal(cfg)
					require.NoError(t, err)
					auth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					user := rpcmocks.NewMockIUserProvider(ctrl)
					user.EXPECT().GetUserIdInCtx(gomock.Any()).Return("u", true)
					app := &PromptOpenAPIApplicationImpl{promptManageRepo: mr, promptService: ps, auth: auth, user: user}
					_, callErr = app.SaveDraftOApi(ctx, &openapi.SaveDraftOApiRequest{PromptID: ptr.Of(int64(123)), WorkspaceID: ptr.Of(int64(100)), PromptDraft: &oapi.PromptDraft{DraftInfo: &oapi.DraftInfo{}, Detail: &oapi.PromptDetail{ModelConfig: cfg}}})
					after, err = json.Marshal(cfg)
					require.NoError(t, err)
				} else {
					var thinking web.ThinkingConfig
					cfg := &web.ModelConfig{ModelID: ptr.Of(int64(111))}
					if tc.thinking != "" {
						require.NoError(t, json.Unmarshal([]byte(tc.thinking), &thinking))
						cfg.Thinking = &thinking
					}
					if tc.name == "empty_config" {
						cfg = &web.ModelConfig{}
					}
					if tc.name == "nil_config" {
						cfg = nil
					}
					before, err = json.Marshal(cfg)
					require.NoError(t, err)
					app := &PromptManageApplicationImpl{manageRepo: mr, promptService: ps, authRPCProvider: auth}
					if entry == "web_create" {
						auth.EXPECT().CheckSpacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
						audit := rpcmocks.NewMockIAuditProvider(ctrl)
						audit.EXPECT().AuditPrompt(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
						app.auditRPCProvider = audit
						_, callErr = app.CreatePrompt(ctx, &manage.CreatePromptRequest{WorkspaceID: ptr.Of(int64(100)), PromptKey: ptr.Of("first_write_review"), PromptName: ptr.Of("first write"), DraftDetail: &web.PromptDetail{ModelConfig: cfg}})
					} else {
						auth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
						_, callErr = app.SaveDraft(ctx, &manage.SaveDraftRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &web.PromptDraft{DraftInfo: &web.DraftInfo{}, Detail: &web.PromptDetail{ModelConfig: cfg}}})
					}
					after, err = json.Marshal(cfg)
					require.NoError(t, err)
				}
				previousAfter, err := json.Marshal(previous)
				require.NoError(t, err)
				errorText := ""
				if callErr != nil {
					errorText = callErr.Error()
				}
				obs, err := json.Marshal(map[string]any{"entry": entry, "case": tc.name, "error": errorText, "repository_write_calls": writes, "model_json_at_repository": string(stored), "input_unchanged": string(before) == string(after), "previous_unchanged": string(previousBefore) == string(previousAfter)})
				require.NoError(t, err)
				t.Logf("new_draft_write_observation=%s", obs)
				assert.Equal(t, before, after)
				assert.Equal(t, previousBefore, previousAfter)
				if tc.name == "conflict" {
					assert.Error(t, callErr)
					assert.Zero(t, writes)
					return
				}
				require.NoError(t, callErr)
				require.Equal(t, 1, writes)
				if tc.name == "nil_config" {
					assert.Nil(t, model)
					return
				}
				require.NotNil(t, model)
				if tc.name == "model_only" || tc.name == "empty_config" {
					assert.Nil(t, model.Thinking)
					wantID := int64(111)
					if tc.name == "empty_config" {
						wantID = 0
					}
					assert.Equal(t, wantID, model.ModelID)
					return
				}
				require.NotNil(t, model.Thinking)
				if tc.name == "disabled" {
					assert.Nil(t, model.Thinking.BudgetTokens)
					require.NotNil(t, model.Thinking.ReasoningEffortValue)
					assert.Empty(t, *model.Thinking.ReasoningEffortValue)
					return
				}
				if tc.name == "auto" {
					require.NotNil(t, model.Thinking.ReasoningEffortValue)
					assert.Empty(t, *model.Thinking.ReasoningEffortValue)
					assert.Nil(t, model.Thinking.ReasoningEffort)
					require.NotNil(t, model.Thinking.BudgetTokens)
					assert.Equal(t, int64(2048), *model.Thinking.BudgetTokens)
					return
				}
				assert.Nil(t, model.Thinking.ReasoningEffortValue)
				require.NotNil(t, model.Thinking.ReasoningEffort)
				assert.Equal(t, entity.ReasoningEffortHigh, *model.Thinking.ReasoningEffort)
				require.NotNil(t, model.Thinking.BudgetTokens)
				assert.Equal(t, int64(2048), *model.Thinking.BudgetTokens)
			})
		}
	}
}
