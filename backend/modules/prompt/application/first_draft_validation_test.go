package application

import (
	"bytes"
	"context"
	"encoding/json"
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
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestFirstDraftValueValidation(t *testing.T) {
	cases := []struct {
		name     string
		thinking string
	}{
		{"conflict", `{"thinking_option":"enabled","reasoning_effort_value":"high","budget_tokens":2048}`},
		{"disabled", `{"thinking_option":"disabled","reasoning_effort_value":"high","budget_tokens":2048}`},
		{"legacy", `{"thinking_option":"enabled","reasoning_effort":"high","budget_tokens":2048}`},
		{"auto", `{"thinking_option":"enabled","reasoning_effort_value":"","budget_tokens":2048}`},
		{"model_only", ""},
		{"empty_config", ""},
		{"nil_model", ""},
	}
	for _, entry := range []string{"web", "openapi"} {
		t.Run(entry, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					ctrl := gomock.NewController(t)
					mr := repomocks.NewMockIManageRepo(ctrl)
					ps := servicemocks.NewMockIPromptService(ctrl)
					auth := rpcmocks.NewMockIAuthProvider(ctrl)
					previous := &entity.Prompt{ID: 123, SpaceID: 100, PromptBasic: &entity.PromptBasic{}}
					previousBefore, err := json.Marshal(previous)
					require.NoError(t, err)
					mr.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(previous, nil).Times(1)
					var savedModel *entity.ModelConfig
					var savedJSON []byte
					saveCalls := 0
					ps.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.Prompt) (*entity.DraftInfo, error) {
						saveCalls++
						var captureErr error
						savedJSON, captureErr = json.Marshal(p.PromptDraft.PromptDetail.ModelConfig)
						require.NoError(t, captureErr)
						require.NoError(t, json.Unmarshal(savedJSON, &savedModel))
						return &entity.DraftInfo{}, nil
					}).AnyTimes()
					ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u"})
					var inputBefore, inputAfter []byte
					var callErr error
					if entry == "openapi" {
						var thinking oapiprompt.ThinkingConfig
						config := &oapiprompt.ModelConfig{ModelID: ptr.Of(int64(111))}
						if tc.thinking != "" {
							require.NoError(t, json.Unmarshal([]byte(tc.thinking), &thinking))
							config.Thinking = &thinking
						}
						if tc.name == "empty_config" {
							config = &oapiprompt.ModelConfig{}
						}
						if tc.name == "nil_model" {
							config = nil
						}
						inputBefore, err = json.Marshal(config)
						require.NoError(t, err)
						auth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
						user := rpcmocks.NewMockIUserProvider(ctrl)
						user.EXPECT().GetUserIdInCtx(gomock.Any()).Return("u", true)
						app := &PromptOpenAPIApplicationImpl{promptManageRepo: mr, promptService: ps, auth: auth, user: user}
						_, callErr = app.SaveDraftOApi(ctx, &openapi.SaveDraftOApiRequest{PromptID: ptr.Of(int64(123)), WorkspaceID: ptr.Of(int64(100)), PromptDraft: &oapiprompt.PromptDraft{DraftInfo: &oapiprompt.DraftInfo{}, Detail: &oapiprompt.PromptDetail{ModelConfig: config}}})
						inputAfter, err = json.Marshal(config)
						require.NoError(t, err)
					} else {
						var thinking webprompt.ThinkingConfig
						config := &webprompt.ModelConfig{ModelID: ptr.Of(int64(111))}
						if tc.thinking != "" {
							require.NoError(t, json.Unmarshal([]byte(tc.thinking), &thinking))
							config.Thinking = &thinking
						}
						if tc.name == "empty_config" {
							config = &webprompt.ModelConfig{}
						}
						if tc.name == "nil_model" {
							config = nil
						}
						inputBefore, err = json.Marshal(config)
						require.NoError(t, err)
						auth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
						app := &PromptManageApplicationImpl{manageRepo: mr, promptService: ps, authRPCProvider: auth}
						_, callErr = app.SaveDraft(ctx, &manage.SaveDraftRequest{PromptID: ptr.Of(int64(123)), PromptDraft: &webprompt.PromptDraft{DraftInfo: &webprompt.DraftInfo{}, Detail: &webprompt.PromptDetail{ModelConfig: config}}})
						inputAfter, err = json.Marshal(config)
						require.NoError(t, err)
					}
					errorText := ""
					if callErr != nil {
						errorText = callErr.Error()
					}
					previousAfter, err := json.Marshal(previous)
					require.NoError(t, err)
					observation, err := json.Marshal(map[string]any{
						"entry": entry, "case": tc.name, "save_calls": saveCalls,
						"error": errorText, "saved_model_json": string(savedJSON),
						"input_unchanged":    bytes.Equal(inputBefore, inputAfter),
						"previous_unchanged": bytes.Equal(previousBefore, previousAfter),
					})
					require.NoError(t, err)
					t.Logf("first_draft_observation=%s", observation)
					assert.Equal(t, inputBefore, inputAfter, "input model config must remain unchanged")
					assert.Equal(t, previousBefore, previousAfter, "source prompt must remain unchanged")
					if tc.name == "conflict" {
						assert.Error(t, callErr, "new Value high with explicit budget must be rejected on the first save")
						assert.Zero(t, saveCalls, "invalid config must not reach SaveDraft")
						return
					}
					require.NoError(t, callErr)
					require.Equal(t, 1, saveCalls)
					if tc.name == "nil_model" {
						assert.Nil(t, savedModel)
						return
					}
					require.NotNil(t, savedModel)
					if tc.name == "model_only" || tc.name == "empty_config" {
						assert.Nil(t, savedModel.Thinking)
						wantID := int64(111)
						if tc.name == "empty_config" {
							wantID = 0
						}
						assert.Equal(t, wantID, savedModel.ModelID)
						return
					}
					require.NotNil(t, savedModel.Thinking)
					if tc.name == "disabled" {
						assert.Nil(t, savedModel.Thinking.BudgetTokens, "disabled Value config must suppress budget before persistence")
						require.NotNil(t, savedModel.Thinking.ReasoningEffortValue)
						assert.Empty(t, *savedModel.Thinking.ReasoningEffortValue, "disabled Value config must clear the selected effort before persistence")
						return
					}
					if tc.name == "auto" {
						require.NotNil(t, savedModel.Thinking.ReasoningEffortValue)
						assert.Empty(t, *savedModel.Thinking.ReasoningEffortValue)
						assert.Nil(t, savedModel.Thinking.ReasoningEffort)
						require.NotNil(t, savedModel.Thinking.BudgetTokens)
						assert.Equal(t, int64(2048), *savedModel.Thinking.BudgetTokens)
						return
					}
					assert.Nil(t, savedModel.Thinking.ReasoningEffortValue, "legacy request must not gain Value presence")
					require.NotNil(t, savedModel.Thinking.ReasoningEffort)
					assert.Equal(t, entity.ReasoningEffortHigh, *savedModel.Thinking.ReasoningEffort)
					require.NotNil(t, savedModel.Thinking.BudgetTokens)
					assert.Equal(t, int64(2048), *savedModel.Thinking.BudgetTokens)
				})
			}
		})
	}
}
