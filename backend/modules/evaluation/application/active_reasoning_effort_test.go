// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	openapicommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapiEvaluator "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/evaluator"
	evaluatorservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	evaluatorconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type evaluatorEffortReaderFunc func(context.Context, int64, int64) (bool, []string, error)

func (f evaluatorEffortReaderFunc) GetReasoningEffortOptions(ctx context.Context, spaceID, modelID int64) (bool, []string, error) {
	return f(ctx, spaceID, modelID)
}

func activeEffortEvaluator(modelID int64, value, option *string, agent bool) *entity.Evaluator {
	model := &entity.ModelConfig{ModelID: gptr.Of(modelID), JSONExt: gptr.Of(`{"keep":true}`), Thinking: &entity.ThinkingConfig{ReasoningEffort: value, ThinkingOption: option}}
	result := &entity.Evaluator{ID: 123, SpaceID: 100, Name: "same display name", EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 124, Version: "0.0.0", ModelConfig: model}}
	if agent {
		result.EvaluatorType = entity.EvaluatorTypeAgent
		result.PromptEvaluatorVersion = nil
		result.AgentEvaluatorVersion = &entity.AgentEvaluatorVersion{ID: 124, Version: "0.0.0", ModelConfig: model,
			AgentConfig: &entity.AgentConfig{Runtime: gptr.Of("claude_code"), SearchProvider: gptr.Of("search_api")}}
	}
	return result
}

func activeEffortApps(t *testing.T, reader evaluatorEffortReaderFunc) (*EvaluatorHandlerImpl, *EvalOpenAPIApplication, *rpcmocks.MockIAuthProvider, *servicemocks.MockEvaluatorService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	auth, svc := rpcmocks.NewMockIAuthProvider(ctrl), servicemocks.NewMockEvaluatorService(ctrl)
	auditor, metrics := auditmocks.NewMockIAuditService(ctrl), metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
	auditor.EXPECT().Audit(gomock.Any(), gomock.Any()).Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).AnyTimes()
	metrics.EXPECT().EmitCreate(gomock.Any(), gomock.Any()).AnyTimes()
	config := confmocks.NewMockIConfiger(ctrl)
	config.EXPECT().CheckAgentEvaluatorWritable(gomock.Any()).Return(true, nil).AnyTimes()
	users := userinfomocks.NewMockUserInfoService(ctrl)
	users.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).AnyTimes()
	var models rpc.IModelReasoningEffortProvider
	if reader != nil {
		models = reader
	}
	web := NewEvaluatorHandlerImplWithModelReasoningEffortProvider(nil, config, auth, svc, nil, nil, metrics, users, auditor, nil, nil, nil, nil, nil, models).(*EvaluatorHandlerImpl)
	oapi := NewEvalOpenAPIApplicationWithModelReasoningEffortProvider(nil, nil, nil, nil, auth, nil, nil, nil, nil, &fakeOpenAPIMetric{}, nil, users, nil, nil, nil, nil, svc, nil, nil, nil, nil, nil, nil, nil, models).(*EvalOpenAPIApplication)
	return web, oapi, auth, svc
}

func TestActiveEvaluatorReasoningEffortCreateEntrypoints(t *testing.T) {
	queryFailure, denied := errors.New("model query failed"), errors.New("authorization denied")
	cases := []struct {
		name                 string
		value, option        *string
		options              []string
		enabled, agent       bool
		queryErr, authErr    error
		wantQuery, wantError bool
	}{
		{"outside", gptr.Of("Outside"), nil, []string{"Allowed"}, true, false, nil, nil, true, true},
		{"inside", gptr.Of("Allowed"), nil, []string{"Allowed"}, true, false, nil, nil, true, false},
		{"omitted", nil, nil, nil, false, false, queryFailure, nil, false, false},
		{"Auto", gptr.Of(""), nil, nil, false, false, queryFailure, nil, false, false},
		{"none", gptr.Of("none"), nil, []string{"none"}, true, false, nil, nil, true, false},
		{"exact_case", gptr.Of("High"), nil, []string{"high"}, true, false, nil, nil, true, true},
		{"unsupported", gptr.Of("high"), nil, nil, false, false, nil, nil, true, true},
		{"legacy_options", gptr.Of("high"), nil, nil, true, false, nil, nil, true, false},
		{"query_failure", gptr.Of("Allowed"), nil, nil, true, false, queryFailure, nil, true, true},
		{"disabled", gptr.Of("Outside"), gptr.Of("disabled"), nil, false, false, queryFailure, nil, false, false},
		{"authorization_denied", gptr.Of("Allowed"), nil, nil, true, false, nil, denied, false, true},
		{"agent_inside", gptr.Of("Allowed"), nil, []string{"Allowed"}, true, true, nil, nil, true, false},
		{"agent_outside", gptr.Of("Outside"), nil, []string{"Allowed"}, true, true, nil, nil, true, true},
	}
	for _, isOpenAPI := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI]+"/"+tc.name, func(t *testing.T) {
				authorized, wrote, queries := false, false, 0
				web, oapi, auth, svc := activeEffortApps(t, func(_ context.Context, spaceID, modelID int64) (bool, []string, error) {
					require.True(t, authorized)
					require.Equal(t, int64(100), spaceID)
					require.Equal(t, int64(222), modelID)
					queries++
					return tc.enabled, tc.options, tc.queryErr
				})
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, interface{}) error { authorized = tc.authErr == nil; return tc.authErr })
				svc.EXPECT().CreateEvaluator(gomock.Any(), gomock.Any(), "").DoAndReturn(func(_ context.Context, saved *entity.Evaluator, _ string) (int64, error) {
					wrote = true
					require.Equal(t, tc.value, evaluatorModelConfig(saved).Thinking.ReasoningEffort)
					require.Equal(t, tc.option, evaluatorModelConfig(saved).Thinking.ThinkingOption)
					require.Equal(t, `{"keep":true}`, *evaluatorModelConfig(saved).JSONExt)
					return 123, nil
				}).AnyTimes()
				input := activeEffortEvaluator(222, tc.value, tc.option, tc.agent)
				var err error
				if isOpenAPI {
					_, err = oapi.CreateEvaluatorOApi(context.Background(), &openapi.CreateEvaluatorOApiRequest{WorkspaceID: gptr.Of(int64(100)), Evaluator: evaluatorconvertor.OpenAPIEvaluatorDO2DTO(input)})
				} else {
					_, err = web.CreateEvaluator(context.Background(), &evaluatorservice.CreateEvaluatorRequest{WorkspaceID: gptr.Of(int64(100)), Evaluator: evaluatorconvertor.ConvertEvaluatorDO2DTO(input)})
				}
				if tc.wantError {
					require.Error(t, err)
					require.False(t, wrote)
				} else {
					require.NoError(t, err)
					require.True(t, wrote)
				}
				if tc.authErr != nil {
					require.ErrorIs(t, err, tc.authErr)
				}
				if tc.queryErr != nil && tc.wantQuery {
					require.ErrorIs(t, err, tc.queryErr)
				}
				require.Equal(t, map[bool]int{false: 0, true: 1}[tc.wantQuery], queries)
				require.Equal(t, tc.value, evaluatorModelConfig(input).Thinking.ReasoningEffort)
			})
		}
	}
}

func TestActiveEvaluatorReasoningEffortSubmitEntrypoints(t *testing.T) {
	queryFailure, denied, baselineFailure := errors.New("model query failed"), errors.New("authorization denied"), errors.New("version read failed")
	cases := []struct {
		name                              string
		id                                int64
		value, option                     *string
		published, agent, missingBaseline bool
		queryErr, authErr, baselineErr    error
		wantQuery, wantError              bool
	}{
		{"new_outside", 222, gptr.Of("Outside"), nil, false, false, false, nil, nil, nil, true, true},
		{"same_model_changed_inside", 111, gptr.Of("Allowed"), nil, true, false, false, nil, nil, nil, true, false},
		{"same_model_changed_outside", 111, gptr.Of("Outside"), nil, true, false, false, nil, nil, nil, true, true},
		{"unchanged_historical", 111, gptr.Of("Retired"), nil, true, false, false, nil, nil, nil, false, false},
		{"switch_same_display_name", 222, gptr.Of("Retired"), nil, true, false, false, nil, nil, nil, true, true},
		{"Auto", 111, gptr.Of(""), nil, true, false, false, queryFailure, nil, nil, false, false},
		{"omitted", 111, nil, nil, true, false, false, queryFailure, nil, nil, false, false},
		{"disabled", 111, gptr.Of("Outside"), gptr.Of("disabled"), true, false, false, queryFailure, nil, nil, false, false},
		{"query_failure", 222, gptr.Of("Allowed"), nil, false, false, false, queryFailure, nil, nil, true, true},
		{"baseline_failure", 111, gptr.Of("Allowed"), nil, true, false, false, nil, nil, baselineFailure, false, true},
		{"missing_baseline", 111, gptr.Of("Allowed"), nil, true, false, true, nil, nil, nil, false, true},
		{"authorization_denied", 111, gptr.Of("Allowed"), nil, true, false, false, nil, denied, nil, false, true},
		{"agent_unchanged_historical", 111, gptr.Of("Retired"), nil, true, true, false, nil, nil, nil, false, false},
		{"agent_new_inside", 222, gptr.Of("Allowed"), nil, false, true, false, nil, nil, nil, true, false},
	}
	for _, isOpenAPI := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI]+"/"+tc.name, func(t *testing.T) {
				authorized, wrote, queries := false, false, 0
				web, oapi, auth, svc := activeEffortApps(t, func(_ context.Context, spaceID, modelID int64) (bool, []string, error) {
					require.True(t, authorized)
					require.Equal(t, int64(100), spaceID)
					require.Equal(t, tc.id, modelID)
					queries++
					return true, []string{"Allowed"}, tc.queryErr
				})
				incoming := activeEffortEvaluator(tc.id, tc.value, tc.option, tc.agent)
				previous := activeEffortEvaluator(111, gptr.Of("Retired"), nil, tc.agent)
				if tc.published {
					incoming.LatestVersion = "1.0.0"
				}
				svc.EXPECT().GetEvaluator(gomock.Any(), int64(100), int64(123), false).Return(incoming, nil)
				if tc.authErr == nil && tc.published && tc.value != nil && *tc.value != "" && tc.option == nil {
					baseline := []*entity.Evaluator{previous}
					if tc.missingBaseline {
						baseline = nil
					}
					svc.EXPECT().BatchGetEvaluatorByIDAndVersion(gomock.Any(), [][2]interface{}{{int64(123), "1.0.0"}}).DoAndReturn(func(context.Context, [][2]interface{}) ([]*entity.Evaluator, error) {
						require.True(t, authorized)
						return baseline, tc.baselineErr
					})
				}
				svc.EXPECT().SubmitEvaluatorVersion(gomock.Any(), gomock.Any(), "1.0.1", "", "").DoAndReturn(func(_ context.Context, saved *entity.Evaluator, _, _, _ string) (*entity.Evaluator, error) {
					wrote = true
					require.Equal(t, tc.value, evaluatorModelConfig(saved).Thinking.ReasoningEffort)
					require.Equal(t, `{"keep":true}`, *evaluatorModelConfig(saved).JSONExt)
					return saved, nil
				}).AnyTimes()
				var err error
				if isOpenAPI {
					auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, interface{}) error { authorized = tc.authErr == nil; return tc.authErr })
					_, err = oapi.SubmitEvaluatorVersionOApi(context.Background(), &openapi.SubmitEvaluatorVersionOApiRequest{WorkspaceID: gptr.Of(int64(100)), EvaluatorID: gptr.Of(int64(123)), Version: gptr.Of("1.0.1")})
				} else {
					auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, interface{}) error { authorized = tc.authErr == nil; return tc.authErr })
					_, err = web.SubmitEvaluatorVersion(context.Background(), &evaluatorservice.SubmitEvaluatorVersionRequest{WorkspaceID: 100, EvaluatorID: 123, Version: "1.0.1"})
				}
				if tc.wantError {
					require.Error(t, err)
					require.False(t, wrote)
				} else {
					require.NoError(t, err)
					require.True(t, wrote)
				}
				for _, cause := range []error{tc.authErr, tc.baselineErr} {
					if cause != nil {
						require.ErrorIs(t, err, cause)
					}
				}
				if tc.queryErr != nil && tc.wantQuery {
					require.ErrorIs(t, err, tc.queryErr)
				}
				require.Equal(t, map[bool]int{false: 0, true: 1}[tc.wantQuery], queries)
				require.Equal(t, "Retired", *evaluatorModelConfig(previous).Thinking.ReasoningEffort)
			})
		}
	}
}

func TestActiveEvaluatorReasoningEffortDisabledAndProjectedIdentityUpdate(t *testing.T) {
	for _, isOpenAPI := range []bool{false, true} {
		for _, projected := range []bool{false, true} {
			t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI]+"/"+map[bool]string{false: "inherited_disabled", true: "numeric_provider_projection"}[projected], func(t *testing.T) {
				web, oapi, auth, svc := activeEffortApps(t, func(context.Context, int64, int64) (bool, []string, error) {
					t.Fatal("unchanged historical or disabled configuration must not query current model")
					return false, nil, nil
				})
				old := activeEffortEvaluator(111, gptr.Of("Retired"), gptr.Of("disabled"), false)
				next := activeEffortEvaluator(111, gptr.Of("Outside"), nil, false)
				next.PromptEvaluatorVersion.ModelConfig.JSONExt = nil
				if projected {
					old.PromptEvaluatorVersion.ModelConfig.ModelID = gptr.Of(int64(0))
					old.PromptEvaluatorVersion.ModelConfig.ProviderModelID = gptr.Of("111")
					old.PromptEvaluatorVersion.ModelConfig.Thinking.ThinkingOption = nil
					next.PromptEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort = gptr.Of("Retired")
				}
				svc.EXPECT().GetEvaluator(gomock.Any(), int64(100), int64(123), false).Return(old, nil)
				wrote := false
				svc.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, saved *entity.Evaluator) error {
					wrote = true
					require.NoError(t, saved.ValidateVersionSettings())
					require.Equal(t, `{"keep":true}`, *evaluatorModelConfig(saved).JSONExt)
					if !projected {
						require.Equal(t, "disabled", *evaluatorModelConfig(saved).Thinking.ThinkingOption)
						require.Nil(t, evaluatorModelConfig(saved).Thinking.BudgetTokens)
					}
					return nil
				}).AnyTimes()
				var err error
				if isOpenAPI {
					auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
					_, err = oapi.UpdateEvaluatorDraftOApi(context.Background(), &openapi.UpdateEvaluatorDraftOApiRequest{WorkspaceID: gptr.Of(int64(100)), EvaluatorID: gptr.Of(int64(123)), EvaluatorType: gptr.Of(openapiEvaluator.EvaluatorTypePrompt), EvaluatorContent: evaluatorconvertor.OpenAPIEvaluatorDO2DTO(next).CurrentVersion.EvaluatorContent})
				} else {
					auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
					_, err = web.UpdateEvaluatorDraft(context.Background(), &evaluatorservice.UpdateEvaluatorDraftRequest{WorkspaceID: 100, EvaluatorID: 123, EvaluatorType: evaluatordto.EvaluatorType_Prompt, EvaluatorContent: evaluatorconvertor.ConvertEvaluatorDO2DTO(next).CurrentVersion.EvaluatorContent})
				}
				require.NoError(t, err)
				require.True(t, wrote)
				require.Equal(t, "Retired", *evaluatorModelConfig(old).Thinking.ReasoningEffort)
			})
		}
	}
}

func TestActiveEvaluatorReasoningEffortAgentUpdate(t *testing.T) {
	for _, isOpenAPI := range []bool{false, true} {
		for _, value := range []string{"Allowed", "Outside", "Retired"} {
			t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI]+"/"+value, func(t *testing.T) {
				modelID := int64(222)
				if value == "Retired" {
					modelID = 111
				}
				queries, wrote := 0, false
				web, oapi, auth, svc := activeEffortApps(t, func(_ context.Context, spaceID, id int64) (bool, []string, error) {
					queries++
					require.Equal(t, int64(100), spaceID)
					require.Equal(t, modelID, id)
					return true, []string{"Allowed"}, nil
				})
				old := activeEffortEvaluator(111, gptr.Of("Retired"), nil, true)
				next := activeEffortEvaluator(modelID, gptr.Of(value), nil, true)
				next.AgentEvaluatorVersion.AgentConfig = nil
				if modelID == 111 {
					next.AgentEvaluatorVersion.ModelConfig.JSONExt = nil
				}
				svc.EXPECT().GetEvaluator(gomock.Any(), int64(100), int64(123), false).Return(old, nil)
				svc.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, saved *entity.Evaluator) error {
					wrote = true
					require.Equal(t, value, *saved.AgentEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort)
					require.Equal(t, "claude_code", *saved.AgentEvaluatorVersion.AgentConfig.Runtime)
					require.Equal(t, "search_api", *saved.AgentEvaluatorVersion.AgentConfig.SearchProvider)
					require.Equal(t, `{"keep":true}`, *saved.AgentEvaluatorVersion.ModelConfig.JSONExt)
					return nil
				}).AnyTimes()
				var err error
				if isOpenAPI {
					auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
					_, err = oapi.UpdateEvaluatorDraftOApi(context.Background(), &openapi.UpdateEvaluatorDraftOApiRequest{WorkspaceID: gptr.Of(int64(100)), EvaluatorID: gptr.Of(int64(123)), EvaluatorType: gptr.Of(openapiEvaluator.EvaluatorTypeAgent), EvaluatorContent: evaluatorconvertor.OpenAPIEvaluatorDO2DTO(next).CurrentVersion.EvaluatorContent})
				} else {
					auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
					_, err = web.UpdateEvaluatorDraft(context.Background(), &evaluatorservice.UpdateEvaluatorDraftRequest{WorkspaceID: 100, EvaluatorID: 123, EvaluatorType: evaluatordto.EvaluatorType_Agent, EvaluatorContent: evaluatorconvertor.ConvertEvaluatorDO2DTO(next).CurrentVersion.EvaluatorContent})
				}
				if value == "Outside" {
					require.Error(t, err)
					require.False(t, wrote)
				} else {
					require.NoError(t, err)
					require.True(t, wrote)
				}
				if value == "Retired" {
					require.Equal(t, 0, queries)
				} else {
					require.Equal(t, 1, queries)
				}
				require.Equal(t, "Retired", *old.AgentEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort)
			})
		}
	}
}

func TestActiveEvaluatorReasoningEffortMissingProviderCreate(t *testing.T) {
	for _, isOpenAPI := range []bool{false, true} {
		t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI], func(t *testing.T) {
			web, oapi, auth, svc := activeEffortApps(t, nil)
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			wrote := false
			svc.EXPECT().CreateEvaluator(gomock.Any(), gomock.Any(), "").DoAndReturn(func(context.Context, *entity.Evaluator, string) (int64, error) { wrote = true; return 123, nil }).AnyTimes()
			incoming := activeEffortEvaluator(222, gptr.Of("Allowed"), nil, false)
			var err error
			if isOpenAPI {
				_, err = oapi.CreateEvaluatorOApi(context.Background(), &openapi.CreateEvaluatorOApiRequest{WorkspaceID: gptr.Of(int64(100)), Evaluator: evaluatorconvertor.OpenAPIEvaluatorDO2DTO(incoming)})
			} else {
				_, err = web.CreateEvaluator(context.Background(), &evaluatorservice.CreateEvaluatorRequest{WorkspaceID: gptr.Of(int64(100)), Evaluator: evaluatorconvertor.ConvertEvaluatorDO2DTO(incoming)})
			}
			require.ErrorContains(t, err, "model capability query is unavailable")
			require.False(t, wrote)
		})
	}
}

func TestActiveEvaluatorReasoningEffortUpdateEntrypoints(t *testing.T) {
	queryFailure := errors.New("model query failed")
	cases := []struct {
		name                 string
		id                   int64
		value                *string
		enabled              bool
		options              []string
		queryError           error
		wantQuery, wantError bool
	}{
		{"outside", 222, gptr.Of("Outside"), true, []string{"Allowed"}, nil, true, true},
		{"inside", 222, gptr.Of("Allowed"), true, []string{"Allowed"}, nil, true, false},
		{"unchanged_historical", 111, gptr.Of("Retired"), true, []string{"Allowed"}, nil, false, false},
		{"omitted", 111, nil, false, nil, queryFailure, false, false},
		{"Auto", 222, gptr.Of(""), false, nil, queryFailure, false, false},
		{"none_value", 222, gptr.Of("none"), true, []string{"none"}, nil, true, false},
		{"case", 222, gptr.Of("High"), true, []string{"high"}, nil, true, true},
		{"unsupported", 222, gptr.Of("high"), false, nil, nil, true, true},
		{"legacy_options", 222, gptr.Of("high"), true, nil, nil, true, false},
		{"query_failure", 222, gptr.Of("Allowed"), true, nil, queryFailure, true, true},
	}
	for _, isOpenAPI := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(map[bool]string{false: "web", true: "openapi"}[isOpenAPI]+"/"+tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				auth, svc := rpcmocks.NewMockIAuthProvider(ctrl), servicemocks.NewMockEvaluatorService(ctrl)
				users := userinfomocks.NewMockUserInfoService(ctrl)
				users.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).AnyTimes()
				old := &entity.Evaluator{ID: 123, SpaceID: 100, Name: "evaluator", EvaluatorType: entity.EvaluatorTypePrompt,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ModelConfig: &entity.ModelConfig{ModelID: gptr.Of(int64(111)), Thinking: &entity.ThinkingConfig{ReasoningEffort: gptr.Of("Retired")}}}}
				svc.EXPECT().GetEvaluator(gomock.Any(), gomock.Any(), gomock.Any(), false).Return(old, nil)
				authorized, queries, wrote := false, 0, false
				reader := evaluatorEffortReaderFunc(func(_ context.Context, spaceID, modelID int64) (bool, []string, error) {
					require.True(t, authorized)
					require.Equal(t, int64(100), spaceID)
					require.Equal(t, tc.id, modelID)
					queries++
					return tc.enabled, tc.options, tc.queryError
				})
				svc.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, saved *entity.Evaluator) error {
					wrote = true
					want := tc.value
					if want == nil {
						want = gptr.Of("Retired")
					}
					require.Equal(t, *want, *saved.PromptEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort)
					return nil
				}).AnyTimes()
				var err error
				if isOpenAPI {
					auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, interface{}) error { authorized = true; return nil })
					app := &EvalOpenAPIApplication{auth: auth, evaluatorService: svc, metric: &fakeOpenAPIMetric{}, modelReasoningEffortProvider: reader}
					_, err = app.UpdateEvaluatorDraftOApi(context.Background(), &openapi.UpdateEvaluatorDraftOApiRequest{
						WorkspaceID: gptr.Of(int64(100)), EvaluatorID: gptr.Of(int64(123)), EvaluatorType: gptr.Of(openapiEvaluator.EvaluatorTypePrompt),
						EvaluatorContent: &openapiEvaluator.EvaluatorContent{PromptEvaluator: &openapiEvaluator.PromptEvaluator{
							ModelConfig: &openapicommon.ModelConfig{ModelID: gptr.Of(tc.id), Thinking: &openapicommon.ThinkingConfig{ReasoningEffort: tc.value}},
						}},
					})
				} else {
					auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, interface{}) error { authorized = true; return nil })
					app := &EvaluatorHandlerImpl{auth: auth, evaluatorService: svc, userInfoService: users, modelReasoningEffortProvider: reader}
					_, err = app.UpdateEvaluatorDraft(context.Background(), &evaluatorservice.UpdateEvaluatorDraftRequest{
						WorkspaceID: 100, EvaluatorID: 123, EvaluatorType: evaluatordto.EvaluatorType_Prompt,
						EvaluatorContent: &evaluatordto.EvaluatorContent{PromptEvaluator: &evaluatordto.PromptEvaluator{
							ModelConfig: &common.ModelConfig{ModelID: gptr.Of(tc.id), Thinking: &common.ThinkingConfig{ReasoningEffort: tc.value}},
						}},
					})
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
				require.Equal(t, "Retired", *old.PromptEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort)
			})
		}
	}
}
