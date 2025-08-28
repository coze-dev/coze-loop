// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestPromptSourceEvalTargetServiceImpl_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := NewPromptSourceEvalTargetServiceImpl(mockPromptRPCAdapter)

	tests := []struct {
		name           string
		spaceID        int64
		param          *entity.ExecuteEvalTargetParam
		mockSetup      func()
		wantOutputData *entity.EvalTargetOutputData
		wantStatus     entity.EvalTargetRunStatus
		wantErr        bool
		wantErrCode    int32
	}{
		{
			name:    "successful execution - return text content",
			spaceID: 123,
			param: &entity.ExecuteEvalTargetParam{
				TargetID:            1,
				SourceTargetID:      "456",
				SourceTargetVersion: "v1",
				Input: &entity.EvalTargetInputData{
					InputFields: map[string]*entity.Content{
						"var1": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("test input"),
						},
						"var2": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("[{\"content\":{}}]"),
						},
					},
					HistoryMessages: []*entity.Message{
						{
							Role: entity.RoleUser,
							Content: &entity.Content{
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("test message"),
							},
						},
					},
				},
				TargetType: entity.EvalTargetTypeLoopPrompt,
			},
			mockSetup: func() {
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), gomock.Any()).
					Return(&rpc.ExecutePromptResult{
						Content: gptr.Of("test output"),
						TokenUsage: &entity.TokenUsage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					}, nil)
			},
			wantOutputData: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{
					consts.OutputSchemaKey: {
						ContentType: gptr.Of(entity.ContentTypeText),
						Format:      gptr.Of(entity.Markdown),
						Text:        gptr.Of("test output"),
					},
				},
				EvalTargetUsage: &entity.EvalTargetUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
			wantStatus: entity.EvalTargetRunStatusSuccess,
			wantErr:    false,
		},
		{
			name:    "execution failed - invalid SourceTargetID",
			spaceID: 123,
			param: &entity.ExecuteEvalTargetParam{
				TargetID:            1,
				SourceTargetID:      "invalid",
				SourceTargetVersion: "v1",
				Input:               &entity.EvalTargetInputData{},
				TargetType:          entity.EvalTargetTypeLoopPrompt,
			},
			mockSetup:   func() {},
			wantStatus:  entity.EvalTargetRunStatusFail,
			wantErr:     true,
			wantErrCode: errno.CommonInvalidParamCode,
		},
		{
			name:    "execution failed - RPC call error",
			spaceID: 123,
			param: &entity.ExecuteEvalTargetParam{
				TargetID:            1,
				SourceTargetID:      "456",
				SourceTargetVersion: "v1",
				Input:               &entity.EvalTargetInputData{},
				TargetType:          entity.EvalTargetTypeLoopPrompt,
			},
			mockSetup: func() {
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), gomock.Any()).
					Return(nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantStatus:  entity.EvalTargetRunStatusFail,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name:    "successful execution - return tool call results",
			spaceID: 123,
			param: &entity.ExecuteEvalTargetParam{
				TargetID:            1,
				SourceTargetID:      "456",
				SourceTargetVersion: "v1",
				Input:               &entity.EvalTargetInputData{},
				TargetType:          entity.EvalTargetTypeLoopPrompt,
			},
			mockSetup: func() {
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), gomock.Any()).
					Return(&rpc.ExecutePromptResult{
						ToolCalls: []*entity.ToolCall{
							{
								Type: entity.ToolTypeFunction,
								FunctionCall: &entity.FunctionCall{
									Name:      "test_function",
									Arguments: gptr.Of("{}"),
								},
							},
						},
						TokenUsage: &entity.TokenUsage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					}, nil)
			},
			wantOutputData: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{
					consts.OutputSchemaKey: {
						ContentType: gptr.Of(entity.ContentTypeText),
						Format:      gptr.Of(entity.Markdown),
						Text:        gptr.Of(`[{"index":0,"id":"","type":1,"function_call":{"name":"test_function","arguments":"{}"}}]`),
					},
				},
				EvalTargetUsage: &entity.EvalTargetUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
			wantStatus: entity.EvalTargetRunStatusSuccess,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			gotOutputData, gotStatus, err := service.Execute(context.Background(), tt.spaceID, tt.param)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantStatus, gotStatus)

			if tt.wantOutputData != nil {
				// Validate output fields
				assert.Equal(t, gptr.Indirect(tt.wantOutputData.OutputFields[consts.OutputSchemaKey].Text), gptr.Indirect(gotOutputData.OutputFields[consts.OutputSchemaKey].Text))
				// Validate usage
				if tt.wantOutputData.EvalTargetUsage != nil {
					assert.Equal(t, tt.wantOutputData.EvalTargetUsage.InputTokens, gotOutputData.EvalTargetUsage.InputTokens)
					assert.Equal(t, tt.wantOutputData.EvalTargetUsage.OutputTokens, gotOutputData.EvalTargetUsage.OutputTokens)
				}
				// Validate execution time
				assert.NotNil(t, gotOutputData.TimeConsumingMS)
				assert.GreaterOrEqual(t, *gotOutputData.TimeConsumingMS, int64(0))
			}
		})
	}
}

func TestPromptSourceEvalTargetServiceImpl_BuildBySource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := &PromptSourceEvalTargetServiceImpl{
		promptRPCAdapter: mockPromptRPCAdapter,
	}

	ctx := context.Background()
	defaultSpaceID := int64(123)
	defaultSourceTargetIDStr := "456"
	defaultSourceTargetIDInt, _ := strconv.ParseInt(defaultSourceTargetIDStr, 10, 64)
	defaultSourceTargetVersion := "v1.0.0"

	tests := []struct {
		name                string
		sourceTargetID      string
		sourceTargetVersion string
		mockSetup           func()
		wantEvalTargetCheck func(t *testing.T, evalTarget *entity.EvalTarget)
		wantErr             bool
		wantErrCheck        func(t *testing.T, err error)
	}{
		{
			name:                "success scenario - complete data",
			sourceTargetID:      defaultSourceTargetIDStr,
			sourceTargetVersion: defaultSourceTargetVersion,
			mockSetup: func() {
				mockPrompt := &rpc.LoopPrompt{
					ID:        defaultSourceTargetIDInt,
					PromptKey: "test_prompt_key",
					PromptCommit: &rpc.PromptCommit{
						Detail: &rpc.PromptDetail{
							PromptTemplate: &rpc.PromptTemplate{
								VariableDefs: []*rpc.VariableDef{
									{Key: gptr.Of("var1"), Type: gptr.Of(rpc.VariableTypeString)},
									{Key: gptr.Of("var2"), Type: gptr.Of(rpc.VariableTypeInteger)},
									{Key: gptr.Of("var3"), Type: gptr.Of(rpc.VariableTypeBoolean)},
									{Key: gptr.Of("var4"), Type: gptr.Of(rpc.VariableTypeFloat)},
									{Key: gptr.Of("var5"), Type: gptr.Of(rpc.VariableTypeObject)},
									{Key: gptr.Of("var6"), Type: gptr.Of(rpc.VariableTypeArrayInteger)},
									{Key: gptr.Of("var7"), Type: gptr.Of(rpc.VariableTypeArrayString)},
									{Key: gptr.Of("var8"), Type: gptr.Of(rpc.VariableTypeArrayFloat)},
									{Key: gptr.Of("var9"), Type: gptr.Of(rpc.VariableTypeArrayBoolean)},
									{Key: gptr.Of("var10"), Type: gptr.Of(rpc.VariableTypeArrayObject)},
								},
							},
						},
						CommitInfo: &rpc.CommitInfo{
							Version: gptr.Of(defaultSourceTargetVersion),
						},
					},
					PromptBasic: &rpc.PromptBasic{
						DisplayName: gptr.Of("Test Prompt"),
					},
				}
				mockPromptRPCAdapter.EXPECT().GetPrompt(
					ctx,
					defaultSpaceID,
					defaultSourceTargetIDInt,
					rpc.GetPromptParams{CommitVersion: &defaultSourceTargetVersion},
				).Return(mockPrompt, nil)
			},
			wantEvalTargetCheck: func(t *testing.T, evalTarget *entity.EvalTarget) {
				assert.NotNil(t, evalTarget)
				assert.Equal(t, defaultSpaceID, evalTarget.SpaceID)
				assert.Equal(t, defaultSourceTargetIDStr, evalTarget.SourceTargetID)
				assert.Equal(t, entity.EvalTargetTypeLoopPrompt, evalTarget.EvalTargetType)

				assert.NotNil(t, evalTarget.EvalTargetVersion)
				assert.Equal(t, defaultSpaceID, evalTarget.EvalTargetVersion.SpaceID)
				assert.Equal(t, defaultSourceTargetVersion, evalTarget.EvalTargetVersion.SourceTargetVersion)
				assert.Equal(t, entity.EvalTargetTypeLoopPrompt, evalTarget.EvalTargetVersion.EvalTargetType)

				assert.NotNil(t, evalTarget.EvalTargetVersion.Prompt)
				assert.Equal(t, defaultSourceTargetIDInt, evalTarget.EvalTargetVersion.Prompt.PromptID)
				assert.Equal(t, defaultSourceTargetVersion, evalTarget.EvalTargetVersion.Prompt.Version)

				assert.Len(t, evalTarget.EvalTargetVersion.InputSchema, 10)
				if len(evalTarget.EvalTargetVersion.InputSchema) == 10 {
					assert.Equal(t, "var1", *evalTarget.EvalTargetVersion.InputSchema[0].Key)
					assert.Equal(t, []entity.ContentType{entity.ContentTypeText}, evalTarget.EvalTargetVersion.InputSchema[0].SupportContentTypes)
					assert.Equal(t, consts.StringJsonSchema, *evalTarget.EvalTargetVersion.InputSchema[0].JsonSchema)
					assert.Equal(t, "var2", *evalTarget.EvalTargetVersion.InputSchema[1].Key)
				}

				assert.Len(t, evalTarget.EvalTargetVersion.OutputSchema, 1)
				if len(evalTarget.EvalTargetVersion.OutputSchema) == 1 {
					assert.Equal(t, consts.OutputSchemaKey, *evalTarget.EvalTargetVersion.OutputSchema[0].Key)
					assert.Equal(t, []entity.ContentType{entity.ContentTypeText, entity.ContentTypeMultipart}, evalTarget.EvalTargetVersion.OutputSchema[0].SupportContentTypes)
					assert.Equal(t, consts.StringJsonSchema, *evalTarget.EvalTargetVersion.OutputSchema[0].JsonSchema)
				}
			},
			wantErr: false,
		},
		{
			name:                "success scenario - PromptCommit.Detail.PromptTemplate.VariableDefs is empty",
			sourceTargetID:      defaultSourceTargetIDStr,
			sourceTargetVersion: defaultSourceTargetVersion,
			mockSetup: func() {
				mockPrompt := &rpc.LoopPrompt{
					ID:        defaultSourceTargetIDInt,
					PromptKey: "test_prompt_key",
					PromptCommit: &rpc.PromptCommit{
						Detail: &rpc.PromptDetail{
							PromptTemplate: &rpc.PromptTemplate{
								VariableDefs: []*rpc.VariableDef{},
							},
						},
						CommitInfo: &rpc.CommitInfo{Version: gptr.Of(defaultSourceTargetVersion)},
					},
				}
				mockPromptRPCAdapter.EXPECT().GetPrompt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockPrompt, nil)
			},
			wantEvalTargetCheck: func(t *testing.T, evalTarget *entity.EvalTarget) {
				assert.NotNil(t, evalTarget)
				assert.Len(t, evalTarget.EvalTargetVersion.InputSchema, 0)
			},
			wantErr: false,
		},
		{
			name:                "success scenario - PromptCommit.Detail.PromptTemplate is nil",
			sourceTargetID:      defaultSourceTargetIDStr,
			sourceTargetVersion: defaultSourceTargetVersion,
			mockSetup: func() {
				mockPrompt := &rpc.LoopPrompt{
					ID:        defaultSourceTargetIDInt,
					PromptKey: "test_prompt_key",
					PromptCommit: &rpc.PromptCommit{
						Detail: &rpc.PromptDetail{
							PromptTemplate: nil,
						},
						CommitInfo: &rpc.CommitInfo{Version: gptr.Of(defaultSourceTargetVersion)},
					},
				}
				mockPromptRPCAdapter.EXPECT().GetPrompt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockPrompt, nil)
			},
			wantEvalTargetCheck: func(t *testing.T, evalTarget *entity.EvalTarget) {
				assert.NotNil(t, evalTarget)
				assert.Nil(t, evalTarget.EvalTargetVersion.InputSchema)
			},
			wantErr: false,
		},
		{
			name:                "success scenario - PromptCommit.Detail is nil",
			sourceTargetID:      defaultSourceTargetIDStr,
			sourceTargetVersion: defaultSourceTargetVersion,
			mockSetup: func() {
				mockPrompt := &rpc.LoopPrompt{
					ID:        defaultSourceTargetIDInt,
					PromptKey: "test_prompt_key",
					PromptCommit: &rpc.PromptCommit{
						Detail:     nil,
						CommitInfo: &rpc.CommitInfo{Version: gptr.Of(defaultSourceTargetVersion)},
					},
				}
				mockPromptRPCAdapter.EXPECT().GetPrompt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockPrompt, nil)
			},
			wantEvalTargetCheck: func(t *testing.T, evalTarget *entity.EvalTarget) {
				assert.NotNil(t, evalTarget)
				assert.Nil(t, evalTarget.EvalTargetVersion.InputSchema)
			},
			wantErr: false,
		},
		{
			name:                "success scenario - PromptCommit is nil",
			sourceTargetID:      defaultSourceTargetIDStr,
			sourceTargetVersion: defaultSourceTargetVersion,
			mockSetup: func() {
				mockPrompt := &rpc.LoopPrompt{
					ID:           defaultSourceTargetIDInt,
					PromptKey:    "test_prompt_key",
					PromptCommit: nil,
				}
				mockPromptRPCAdapter.EXPECT().GetPrompt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockPrompt, nil)
			},
			wantEvalTargetCheck: func(t *testing.T, evalTarget *entity.EvalTarget) {
				assert.NotNil(t, evalTarget)
				assert.Nil(t, evalTarget.EvalTargetVersion.InputSchema)
			},
			wantErr: false,
		},
		{
			name:                "failure scenario - strconv.ParseInt failed",
			sourceTargetID:      "not-an-int",
			sourceTargetVersion: defaultSourceTargetVersion,
			mockSetup:           func() {},
			wantEvalTargetCheck: func(t *testing.T, evalTarget *entity.EvalTarget) {
				assert.Nil(t, evalTarget)
			},
			wantErr: true,
			wantErrCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				numErr, ok := err.(*strconv.NumError)
				assert.True(t, ok)
				assert.Equal(t, "ParseInt", numErr.Func)
			},
		},
		{
			name:                "failure scenario - promptRPCAdapter.GetPrompt returns error",
			sourceTargetID:      defaultSourceTargetIDStr,
			sourceTargetVersion: defaultSourceTargetVersion,
			mockSetup: func() {
				expectedErr := errors.New("RPC GetPrompt error")
				mockPromptRPCAdapter.EXPECT().GetPrompt(
					ctx,
					defaultSpaceID,
					defaultSourceTargetIDInt,
					rpc.GetPromptParams{CommitVersion: &defaultSourceTargetVersion},
				).Return(nil, expectedErr)
			},
			wantEvalTargetCheck: func(t *testing.T, evalTarget *entity.EvalTarget) {
				assert.Nil(t, evalTarget)
			},
			wantErr: true,
			wantErrCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Equal(t, "RPC GetPrompt error", err.Error())
			},
		},
		{
			name:                "failure scenario - promptRPCAdapter.GetPrompt returns nil prompt",
			sourceTargetID:      defaultSourceTargetIDStr,
			sourceTargetVersion: defaultSourceTargetVersion,
			mockSetup: func() {
				mockPromptRPCAdapter.EXPECT().GetPrompt(
					ctx,
					defaultSpaceID,
					defaultSourceTargetIDInt,
					rpc.GetPromptParams{CommitVersion: &defaultSourceTargetVersion},
				).Return(nil, nil)
			},
			wantEvalTargetCheck: func(t *testing.T, evalTarget *entity.EvalTarget) {
				assert.Nil(t, evalTarget)
			},
			wantErr: true,
			wantErrCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.ResourceNotFoundCode), statusErr.Code())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			evalTarget, err := service.BuildBySource(ctx, defaultSpaceID, tt.sourceTargetID, tt.sourceTargetVersion)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCheck != nil {
					tt.wantErrCheck(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.wantEvalTargetCheck != nil {
				tt.wantEvalTargetCheck(t, evalTarget)
			}
		})
	}
}

func TestPromptSourceEvalTargetServiceImpl_ListSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := &PromptSourceEvalTargetServiceImpl{
		promptRPCAdapter: mockPromptRPCAdapter,
	}

	tests := []struct {
		name             string
		param            *entity.ListSourceParam
		setupMocks       func(adapter *mocks.MockIPromptRPCAdapter)
		wantTargets      []*entity.EvalTarget
		wantNextCursor   string
		wantHasMore      bool
		wantErr          bool
		expectedErrorMsg string
	}{
		{
			name: "successfully get list - cursor is nil, has more data",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](1),
				PageSize: gptr.Of[int32](1),
				Cursor:   nil, // page will be 1
				KeyWord:  gptr.Of("test"),
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().ListPrompt(gomock.Any(), &rpc.ListPromptParam{
					SpaceID:  gptr.Of[int64](1),
					PageSize: gptr.Of[int32](1),
					Page:     gptr.Of[int32](1),
					KeyWord:  gptr.Of("test"),
				}).Return([]*rpc.LoopPrompt{
					{
						ID:        101,
						PromptKey: "key1",
						PromptBasic: &rpc.PromptBasic{
							DisplayName:   gptr.Of("Prompt 1"),
							Description:   gptr.Of("Desc 1"),
							LatestVersion: gptr.Of("v1.0"), // Submitted
						},
					},
				}, gptr.Of[int32](1), nil) // total field is not used in ListSource, can be any value
			},
			wantTargets: []*entity.EvalTarget{
				{
					SpaceID:        gptr.Indirect(gptr.Of[int64](1)),
					SourceTargetID: "101",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						SpaceID: gptr.Indirect(gptr.Of[int64](1)),
						Prompt: &entity.LoopPrompt{
							PromptID:     101,
							PromptKey:    "key1",
							Name:         "Prompt 1",
							Description:  "Desc 1",
							SubmitStatus: entity.SubmitStatus_Submitted,
						},
						RuntimeParamDemo: gptr.Of("{\"model_config\":{\"model_id\":\"0\",\"model_name\":\"\",\"max_tokens\":0,\"temperature\":0,\"top_p\":0,\"tool_choice\":\"\",\"json_ext\":\"{}\"}}"),
					},
				},
			},
			wantNextCursor: "2",  // page (1) + 1
			wantHasMore:    true, // len(prompts) (1) == PageSize (1)
			wantErr:        false,
		},
		{
			name: "successfully get list - cursor is valid, no more data",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](2),
				PageSize: gptr.Of[int32](2),
				Cursor:   gptr.Of("2"), // page will be 2
				KeyWord:  nil,
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().ListPrompt(gomock.Any(), &rpc.ListPromptParam{
					SpaceID:  gptr.Of[int64](2),
					PageSize: gptr.Of[int32](2),
					Page:     gptr.Of[int32](2),
					KeyWord:  nil,
				}).Return([]*rpc.LoopPrompt{
					{
						ID:        201,
						PromptKey: "key2",
						PromptBasic: &rpc.PromptBasic{
							DisplayName:   gptr.Of("Prompt 2"),
							Description:   gptr.Of("Desc 2"),
							LatestVersion: nil, // UnSubmit
						},
					},
				}, gptr.Of[int32](1), nil)
			},
			wantTargets: []*entity.EvalTarget{
				{
					SpaceID:        gptr.Indirect(gptr.Of[int64](2)),
					SourceTargetID: "201",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						SpaceID: gptr.Indirect(gptr.Of[int64](2)),
						Prompt: &entity.LoopPrompt{
							PromptID:     201,
							PromptKey:    "key2",
							Name:         "Prompt 2",
							Description:  "Desc 2",
							SubmitStatus: entity.SubmitStatus_UnSubmit,
						},
						RuntimeParamDemo: gptr.Of("{\"model_config\":{\"model_id\":\"0\",\"model_name\":\"\",\"max_tokens\":0,\"temperature\":0,\"top_p\":0,\"tool_choice\":\"\",\"json_ext\":\"{}\"}}"),
					},
				},
			},
			wantNextCursor: "3",   // page (2) + 1
			wantHasMore:    false, // len(prompts) (1) != PageSize (2)
			wantErr:        false,
		},
		{
			name: "successfully get list - PromptBasic is nil",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](3),
				PageSize: gptr.Of[int32](1),
				Cursor:   gptr.Of("1"),
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().ListPrompt(gomock.Any(), gomock.Any()).Return([]*rpc.LoopPrompt{
					{
						ID:          301,
						PromptKey:   "key3",
						PromptBasic: nil, // PromptBasic is nil
					},
				}, gptr.Of[int32](1), nil)
			},
			wantTargets: []*entity.EvalTarget{
				{
					SpaceID:        gptr.Indirect(gptr.Of[int64](3)),
					SourceTargetID: "301",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						SpaceID: gptr.Indirect(gptr.Of[int64](3)),
						Prompt: &entity.LoopPrompt{
							PromptID:    301,
							PromptKey:   "key3",
							Name:        "", // Default from gptr.From
							Description: "", // Default from gptr.From
						},
						RuntimeParamDemo: gptr.Of("{\"model_config\":{\"model_id\":\"0\",\"model_name\":\"\",\"max_tokens\":0,\"temperature\":0,\"top_p\":0,\"tool_choice\":\"\",\"json_ext\":\"{}\"}}"),
					},
				},
			},
			wantNextCursor: "2",
			wantHasMore:    true,
			wantErr:        false,
		},
		{
			name: "successfully get list - return empty list",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](4),
				PageSize: gptr.Of[int32](5),
				Cursor:   gptr.Of("1"),
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().ListPrompt(gomock.Any(), gomock.Any()).Return([]*rpc.LoopPrompt{}, gptr.Of[int32](0), nil)
			},
			wantTargets:    []*entity.EvalTarget{},
			wantNextCursor: "2",
			wantHasMore:    false, // len(prompts) (0) != PageSize (5)
			wantErr:        false,
		},
		{
			name: "failure - buildPageByCursor returns error (invalid cursor)",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](5),
				PageSize: gptr.Of[int32](1),
				Cursor:   gptr.Of("abc"), // Invalid cursor
			},
			setupMocks:       func(adapter *mocks.MockIPromptRPCAdapter) {}, // No RPC call expected
			wantTargets:      nil,
			wantNextCursor:   "",
			wantHasMore:      false,
			wantErr:          true,
			expectedErrorMsg: "strconv.ParseInt: parsing \"abc\": invalid syntax",
		},
		{
			name: "failure - promptRPCAdapter.ListPrompt returns error",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](6),
				PageSize: gptr.Of[int32](1),
				Cursor:   gptr.Of("1"),
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().ListPrompt(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("RPC error"))
			},
			wantTargets:      nil,
			wantNextCursor:   "",
			wantHasMore:      false,
			wantErr:          true,
			expectedErrorMsg: "RPC error",
		},
		{
			name: "boundary case - PageSize is nil (gptr.From will handle as 0)",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](7),
				PageSize: nil, // gptr.Indirect(param.PageSize) will be 0
				Cursor:   gptr.Of("1"),
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().ListPrompt(gomock.Any(), &rpc.ListPromptParam{
					SpaceID:  gptr.Of[int64](7),
					PageSize: nil,
					Page:     gptr.Of[int32](1),
					KeyWord:  nil,
				}).Return([]*rpc.LoopPrompt{
					{ID: 701, PromptKey: "key7", PromptBasic: &rpc.PromptBasic{DisplayName: gptr.Of("P7")}},
				}, gptr.Of[int32](1), nil)
			},
			wantTargets: []*entity.EvalTarget{
				{
					SpaceID:        gptr.Indirect(gptr.Of[int64](7)),
					SourceTargetID: "701",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						SpaceID: gptr.Indirect(gptr.Of[int64](7)),
						Prompt: &entity.LoopPrompt{
							PromptID:     701,
							PromptKey:    "key7",
							Name:         "P7",
							Description:  "",
							SubmitStatus: entity.SubmitStatus_UnSubmit,
						},
						RuntimeParamDemo: gptr.Of("{\"model_config\":{\"model_id\":\"0\",\"model_name\":\"\",\"max_tokens\":0,\"temperature\":0,\"top_p\":0,\"tool_choice\":\"\",\"json_ext\":\"{}\"}}"),
					},
				},
			},
			wantNextCursor: "2",
			wantHasMore:    false, // len(prompts) (1) != PageSize (0) -> false. Note: if PageSize is 0, this logic might be tricky.
			// Actually, len(prompts) == int(gptr.Indirect(nil)) -> len(prompts) == 0.
			// So if prompts is not empty, hasMore will be false. If prompts is empty, hasMore will be true.
			// Let's assume the test case returns 1 prompt, so 1 != 0, hasMore = false.
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks(mockPromptRPCAdapter)

			targets, nextCursor, hasMore, err := service.ListSource(context.Background(), tt.param)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantTargets, targets)
			assert.Equal(t, tt.wantNextCursor, nextCursor)
			assert.Equal(t, tt.wantHasMore, hasMore)
		})
	}
}

func TestPromptSourceEvalTargetServiceImpl_ListSourceVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := &PromptSourceEvalTargetServiceImpl{
		promptRPCAdapter: mockPromptRPCAdapter,
	}

	ctx := context.Background()
	defaultSpaceID := int64(123)
	defaultPromptIDStr := "456"
	defaultPromptIDInt, _ := strconv.ParseInt(defaultPromptIDStr, 10, 64)

	tests := []struct {
		name             string
		param            *entity.ListSourceVersionParam
		mockSetup        func(adapter *mocks.MockIPromptRPCAdapter)
		wantVersions     []*entity.EvalTargetVersion
		wantNextCursor   string
		wantHasMore      bool
		wantErr          bool
		expectedErrorMsg string
		expectedErrCode  int32
	}{
		{
			name: "successfully get version list - has data, has next page",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
				PageSize:       gptr.Of[int32](1),
				Cursor:         gptr.Of("cursor_prev"),
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(&rpc.LoopPrompt{
						ID:        defaultPromptIDInt,
						PromptKey: "test_key",
						PromptBasic: &rpc.PromptBasic{
							DisplayName:   gptr.Of("Test Prompt"),
							LatestVersion: gptr.Of("v2.0"), // Submitted
						},
					}, nil)
				adapter.EXPECT().ListPromptVersion(ctx, &rpc.ListPromptVersionParam{
					PromptID: defaultPromptIDInt,
					SpaceID:  gptr.Of(defaultSpaceID),
					PageSize: gptr.Of[int32](1),
					Cursor:   gptr.Of("cursor_prev"),
				}).Return([]*rpc.CommitInfo{
					{Version: gptr.Of("v1.0"), Description: gptr.Of("Version 1.0 desc")},
				}, "cursor_next", nil)
			},
			wantVersions: []*entity.EvalTargetVersion{
				{
					SpaceID:             defaultSpaceID,
					SourceTargetVersion: "v1.0",
					EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
					Prompt: &entity.LoopPrompt{
						PromptID:     defaultPromptIDInt,
						Version:      "v1.0",
						Name:         "Test Prompt",
						PromptKey:    "test_key",
						SubmitStatus: entity.SubmitStatus_Submitted,
						Description:  "Version 1.0 desc",
					},
					RuntimeParamDemo: gptr.Of("{\"model_config\":{\"model_id\":\"0\",\"model_name\":\"\",\"max_tokens\":0,\"temperature\":0,\"top_p\":0,\"tool_choice\":\"\",\"json_ext\":\"{}\"}}"),
				},
			},
			wantNextCursor: "cursor_next",
			wantHasMore:    true, // len(info) (1) == PageSize (1)
			wantErr:        false,
		},
		{
			name: "successfully get version list - has data, no next page",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
				PageSize:       gptr.Of[int32](2),
				Cursor:         nil,
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(&rpc.LoopPrompt{
						ID:        defaultPromptIDInt,
						PromptKey: "test_key_unsubmit",
						PromptBasic: &rpc.PromptBasic{
							DisplayName:   gptr.Of("Unsubmitted Prompt"),
							LatestVersion: nil, // Unsubmitted
						},
					}, nil)
				adapter.EXPECT().ListPromptVersion(ctx, &rpc.ListPromptVersionParam{
					PromptID: defaultPromptIDInt,
					SpaceID:  gptr.Of(defaultSpaceID),
					PageSize: gptr.Of[int32](2),
					Cursor:   nil,
				}).Return([]*rpc.CommitInfo{
					{Version: gptr.Of("v0.1"), Description: gptr.Of("Version 0.1 desc")},
				}, "cursor_final", nil)
			},
			wantVersions: []*entity.EvalTargetVersion{
				{
					SpaceID:             defaultSpaceID,
					SourceTargetVersion: "v0.1",
					EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
					Prompt: &entity.LoopPrompt{
						PromptID:     defaultPromptIDInt,
						Version:      "v0.1",
						Name:         "Unsubmitted Prompt",
						PromptKey:    "test_key_unsubmit",
						SubmitStatus: entity.SubmitStatus_UnSubmit,
						Description:  "Version 0.1 desc",
					},
					RuntimeParamDemo: gptr.Of("{\"model_config\":{\"model_id\":\"0\",\"model_name\":\"\",\"max_tokens\":0,\"temperature\":0,\"top_p\":0,\"tool_choice\":\"\",\"json_ext\":\"{}\"}}"),
				},
			},
			wantNextCursor: "cursor_final",
			wantHasMore:    false, // len(info) (1) != PageSize (2)
			wantErr:        false,
		},
		{
			name: "successfully get version list - PromptBasic is nil",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
				PageSize:       gptr.Of[int32](1),
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(&rpc.LoopPrompt{
						ID:          defaultPromptIDInt,
						PromptKey:   "key_no_basic",
						PromptBasic: nil, // PromptBasic is nil
					}, nil)
				adapter.EXPECT().ListPromptVersion(ctx, gomock.Any()).
					Return([]*rpc.CommitInfo{
						{Version: gptr.Of("v0.0.1"), Description: gptr.Of("Desc")},
					}, "next", nil)
			},
			wantVersions: []*entity.EvalTargetVersion{
				{
					SpaceID:             defaultSpaceID,
					SourceTargetVersion: "v0.0.1",
					EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
					Prompt: &entity.LoopPrompt{
						PromptID:    defaultPromptIDInt,
						Version:     "v0.0.1",
						Name:        "", // Default from gptr.Indirect(nil)
						PromptKey:   "key_no_basic",
						Description: "Desc",
					},
					RuntimeParamDemo: gptr.Of("{\"model_config\":{\"model_id\":\"0\",\"model_name\":\"\",\"max_tokens\":0,\"temperature\":0,\"top_p\":0,\"tool_choice\":\"\",\"json_ext\":\"{}\"}}"),
				},
			},
			wantNextCursor: "next",
			wantHasMore:    true,
			wantErr:        false,
		},
		{
			name: "successfully get version list - ListPromptVersion returns empty list",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
				PageSize:       gptr.Of[int32](5),
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(&rpc.LoopPrompt{ID: defaultPromptIDInt, PromptKey: "empty_versions"}, nil)
				adapter.EXPECT().ListPromptVersion(ctx, gomock.Any()).
					Return([]*rpc.CommitInfo{}, "no_more", nil)
			},
			wantVersions:   []*entity.EvalTargetVersion{},
			wantNextCursor: "no_more",
			wantHasMore:    false, // len(info) (0) != PageSize (5)
			wantErr:        false,
		},
		{
			name: "failure - invalid SourceTargetID (strconv.ParseInt failed)",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: "not-an-int",
				SpaceID:        gptr.Of(defaultSpaceID),
			},
			mockSetup:        func(adapter *mocks.MockIPromptRPCAdapter) {}, // No RPC call expected
			wantVersions:     nil,
			wantNextCursor:   "",
			wantHasMore:      false,
			wantErr:          true,
			expectedErrorMsg: "strconv.ParseInt: parsing \"not-an-int\": invalid syntax",
		},
		{
			name: "failure - GetPrompt returns error",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(nil, errors.New("GetPrompt RPC error"))
			},
			wantVersions:     nil,
			wantNextCursor:   "",
			wantHasMore:      false,
			wantErr:          true,
			expectedErrorMsg: "GetPrompt RPC error",
		},
		{
			name: "failure - GetPrompt returns nil prompt (ResourceNotFound)",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(nil, nil) // prompt is nil
			},
			wantVersions:     nil,
			wantNextCursor:   "",
			wantHasMore:      false,
			wantErr:          true,
			expectedErrorMsg: errorx.NewByCode(errno.ResourceNotFoundCode).Error(), // compare specific error message
			expectedErrCode:  errno.ResourceNotFoundCode,
		},
		{
			name: "failure - ListPromptVersion returns error",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(&rpc.LoopPrompt{ID: defaultPromptIDInt}, nil)
				adapter.EXPECT().ListPromptVersion(ctx, gomock.Any()).
					Return(nil, "", errors.New("ListPromptVersion RPC error"))
			},
			wantVersions:     nil,
			wantNextCursor:   "",
			wantHasMore:      false,
			wantErr:          true,
			expectedErrorMsg: "ListPromptVersion RPC error",
		},
		{
			name: "boundary case - PageSize is nil (gptr.From will handle as 0), hasMore depends on RPC return count",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
				PageSize:       nil, // gptr.Indirect(param.PageSize) will be 0
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(&rpc.LoopPrompt{
						ID:        defaultPromptIDInt,
						PromptKey: "test_key_pagesize_nil",
						PromptBasic: &rpc.PromptBasic{
							DisplayName:   gptr.Of("PageSize Nil Prompt"),
							LatestVersion: gptr.Of("v1"),
						},
					}, nil)
				// ListPromptVersionParam.PageSize will be nil, which is fine for the mock.
				// The hasMore logic is len(info) == int(gptr.Indirect(param.PageSize)), so len(info) == 0.
				// If ListPromptVersion returns 1 item, hasMore will be 1 == 0 -> false.
				// If ListPromptVersion returns 0 items, hasMore will be 0 == 0 -> true.
				// Let's assume the test case returns 1 prompt, so 1 != 0, hasMore = false.
				adapter.EXPECT().ListPromptVersion(ctx, &rpc.ListPromptVersionParam{
					PromptID: defaultPromptIDInt,
					SpaceID:  gptr.Of(defaultSpaceID),
					PageSize: nil, // PageSize is nil
					Cursor:   nil,
				}).Return([]*rpc.CommitInfo{
					{Version: gptr.Of("vA.1"), Description: gptr.Of("Desc A.1")},
				}, "cursor_pagesize_nil", nil) // Returns 1 item
			},
			wantVersions: []*entity.EvalTargetVersion{
				{
					SpaceID:             defaultSpaceID,
					SourceTargetVersion: "vA.1",
					EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
					Prompt: &entity.LoopPrompt{
						PromptID:     defaultPromptIDInt,
						Version:      "vA.1",
						Name:         "PageSize Nil Prompt",
						PromptKey:    "test_key_pagesize_nil",
						SubmitStatus: entity.SubmitStatus_Submitted,
						Description:  "Desc A.1",
					},
					RuntimeParamDemo: gptr.Of("{\"model_config\":{\"model_id\":\"0\",\"model_name\":\"\",\"max_tokens\":0,\"temperature\":0,\"top_p\":0,\"tool_choice\":\"\",\"json_ext\":\"{}\"}}"),
				},
			},
			wantNextCursor: "cursor_pagesize_nil",
			wantHasMore:    false, // len(info) is 1, gptr.Indirect(nil PageSize) is 0. 1 == 0 is false.
			wantErr:        false,
		},
		{
			name: "boundary case - PageSize is nil, ListPromptVersion returns empty list, hasMore is true",
			param: &entity.ListSourceVersionParam{
				SourceTargetID: defaultPromptIDStr,
				SpaceID:        gptr.Of(defaultSpaceID),
				PageSize:       nil, // gptr.Indirect(param.PageSize) will be 0
			},
			mockSetup: func(adapter *mocks.MockIPromptRPCAdapter) {
				adapter.EXPECT().GetPrompt(ctx, defaultSpaceID, defaultPromptIDInt, rpc.GetPromptParams{}).
					Return(&rpc.LoopPrompt{
						ID: defaultPromptIDInt,
					}, nil)
				adapter.EXPECT().ListPromptVersion(ctx, &rpc.ListPromptVersionParam{
					PromptID: defaultPromptIDInt,
					SpaceID:  gptr.Of(defaultSpaceID),
					PageSize: nil,
					Cursor:   nil,
				}).Return([]*rpc.CommitInfo{}, "cursor_empty_pagesize_nil", nil) // Returns 0 items
			},
			wantVersions:   []*entity.EvalTargetVersion{},
			wantNextCursor: "cursor_empty_pagesize_nil",
			wantHasMore:    true, // len(info) is 0, gptr.Indirect(nil PageSize) is 0. 0 == 0 is true.
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(mockPromptRPCAdapter)

			versions, nextCursor, hasMore, err := service.ListSourceVersion(ctx, tt.param)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok, "Error should be a status error")
					if ok {
						assert.Equal(t, tt.expectedErrCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantVersions, versions)
			assert.Equal(t, tt.wantNextCursor, nextCursor)
			assert.Equal(t, tt.wantHasMore, hasMore)
		})
	}
}

func TestPromptSourceEvalTargetServiceImpl_PackSourceInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := &PromptSourceEvalTargetServiceImpl{
		promptRPCAdapter: mockPromptRPCAdapter,
	}
	ctx := context.Background()

	tests := []struct {
		name         string
		spaceID      int64
		dos          []*entity.EvalTarget // input dos, will be modified by method
		setupMocks   func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget)
		wantErr      bool // PackSourceInfo is designed not to return error, so usually false
		wantDosCheck func(t *testing.T, gotDos []*entity.EvalTarget)
	}{
		{
			name:    "success scenario - normal pack information",
			spaceID: 1,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "101", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
				{SourceTargetID: "102", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				id101, _ := strconv.ParseInt(dos[0].SourceTargetID, 10, 64)
				id102, _ := strconv.ParseInt(dos[1].SourceTargetID, 10, 64)
				adapter.EXPECT().MGetPrompt(gomock.Any(), int64(1), gomock.InAnyOrder(
					[]*rpc.MGetPromptQuery{
						{PromptID: id101, Version: nil},
						{PromptID: id102, Version: nil},
					},
				)).Return([]*rpc.LoopPrompt{
					{ID: id101, PromptBasic: &rpc.PromptBasic{DisplayName: gptr.Of("Prompt 101")}},
					{ID: id102, PromptBasic: &rpc.PromptBasic{DisplayName: gptr.Of("Prompt 102")}},
				}, nil)
			},
			wantErr: false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 2)
				assert.NotNil(t, gotDos[0].EvalTargetVersion)
				assert.NotNil(t, gotDos[0].EvalTargetVersion.Prompt)
				assert.Equal(t, "Prompt 101", gotDos[0].EvalTargetVersion.Prompt.Name)
				assert.NotNil(t, gotDos[1].EvalTargetVersion)
				assert.NotNil(t, gotDos[1].EvalTargetVersion.Prompt)
				assert.Equal(t, "Prompt 102", gotDos[1].EvalTargetVersion.Prompt.Name)
			},
		},
		{
			name:    "success scenario - contains non-LoopPrompt types and MGetPrompt partial match",
			spaceID: 2,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "201", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
				{SourceTargetID: "202", EvalTargetType: entity.EvalTargetTypeCozeBot}, // non-LoopPrompt
				{SourceTargetID: "203", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				id201, _ := strconv.ParseInt(dos[0].SourceTargetID, 10, 64)
				id203, _ := strconv.ParseInt(dos[2].SourceTargetID, 10, 64)
				adapter.EXPECT().MGetPrompt(gomock.Any(), int64(2), gomock.InAnyOrder(
					[]*rpc.MGetPromptQuery{
						{PromptID: id201, Version: nil},
						{PromptID: id203, Version: nil},
					},
				)).Return([]*rpc.LoopPrompt{
					{ID: id201, PromptBasic: &rpc.PromptBasic{DisplayName: gptr.Of("Prompt 201")}},
					// ID 203 is not in the return results
				}, nil)
			},
			wantErr: false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 3)
				assert.NotNil(t, gotDos[0].EvalTargetVersion)
				assert.NotNil(t, gotDos[0].EvalTargetVersion.Prompt)
				assert.Equal(t, "Prompt 201", gotDos[0].EvalTargetVersion.Prompt.Name)
				assert.Nil(t, gotDos[1].EvalTargetVersion) // non-LoopPrompt type, should not be processed
				assert.Nil(t, gotDos[2].EvalTargetVersion) // LoopPrompt type, but MGetPrompt didn't return, should not be processed
			},
		},
		{
			name:    "success scenario - MGetPrompt returns PromptBasic is nil or DisplayName is nil",
			spaceID: 3,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "301", EvalTargetType: entity.EvalTargetTypeLoopPrompt}, // PromptBasic is nil
				{SourceTargetID: "302", EvalTargetType: entity.EvalTargetTypeLoopPrompt}, // DisplayName is nil
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				id301, _ := strconv.ParseInt(dos[0].SourceTargetID, 10, 64)
				id302, _ := strconv.ParseInt(dos[1].SourceTargetID, 10, 64)
				adapter.EXPECT().MGetPrompt(gomock.Any(), int64(3), gomock.InAnyOrder(
					[]*rpc.MGetPromptQuery{
						{PromptID: id301, Version: nil},
						{PromptID: id302, Version: nil},
					},
				)).Return([]*rpc.LoopPrompt{
					{ID: id301, PromptBasic: nil},
					{ID: id302, PromptBasic: &rpc.PromptBasic{DisplayName: nil}},
				}, nil)
			},
			wantErr: false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 2)
				assert.NotNil(t, gotDos[0].EvalTargetVersion)
				assert.NotNil(t, gotDos[0].EvalTargetVersion.Prompt)
				assert.Equal(t, "", gotDos[0].EvalTargetVersion.Prompt.Name) // gptr.Indirect(nil) is ""
				assert.NotNil(t, gotDos[1].EvalTargetVersion)
				assert.NotNil(t, gotDos[1].EvalTargetVersion.Prompt)
				assert.Equal(t, "", gotDos[1].EvalTargetVersion.Prompt.Name) // gptr.Indirect(nil) is ""
			},
		},
		{
			name:       "boundary scenario - dos is empty",
			spaceID:    4,
			dos:        []*entity.EvalTarget{},
			setupMocks: nil, // MGetPrompt will not be called
			wantErr:    false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Empty(t, gotDos)
			},
		},
		{
			name:    "boundary scenario - no LoopPrompt type in dos",
			spaceID: 5,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "501", EvalTargetType: entity.EvalTargetTypeCozeBot},
			},
			setupMocks: nil, // MGetPrompt will not be called
			wantErr:    false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 1)
				assert.Nil(t, gotDos[0].EvalTargetVersion)
			},
		},
		{
			name:    "boundary scenario - MGetPrompt returns empty list",
			spaceID: 6,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "601", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				id601, _ := strconv.ParseInt(dos[0].SourceTargetID, 10, 64)
				adapter.EXPECT().MGetPrompt(gomock.Any(), int64(6), []*rpc.MGetPromptQuery{
					{PromptID: id601, Version: nil},
				}).Return([]*rpc.LoopPrompt{}, nil)
			},
			wantErr: false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 1)
				assert.Nil(t, gotDos[0].EvalTargetVersion) // MGetPrompt returns empty, no match found, should not be filled
			},
		},
		{
			name:    "abnormal scenario - strconv.ParseInt failed (handled internally, no error returned)",
			spaceID: 7,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "abc", EvalTargetType: entity.EvalTargetTypeLoopPrompt}, // invalid ID
				{SourceTargetID: "701", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				// "abc" will cause ParseInt to fail, so MGetPrompt will only query "701"
				id701, _ := strconv.ParseInt(dos[1].SourceTargetID, 10, 64)
				adapter.EXPECT().MGetPrompt(gomock.Any(), int64(7), []*rpc.MGetPromptQuery{
					{PromptID: id701, Version: nil},
				}).Return([]*rpc.LoopPrompt{
					{ID: id701, PromptBasic: &rpc.PromptBasic{DisplayName: gptr.Of("Prompt 701")}},
				}, nil)
				// Note: here we can mock logs.CtxError to verify if it's called, but usually we don't do this, instead check its side effects
			},
			wantErr: false, // PackSourceInfo handles ParseInt error internally, doesn't throw outward
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 2)
				assert.Nil(t, gotDos[0].EvalTargetVersion) // ParseInt failed, not processed
				assert.NotNil(t, gotDos[1].EvalTargetVersion)
				assert.NotNil(t, gotDos[1].EvalTargetVersion.Prompt)
				assert.Equal(t, "Prompt 701", gotDos[1].EvalTargetVersion.Prompt.Name)
			},
		},
		{
			name:    "abnormal scenario - MGetPrompt returns error (handled internally, no error returned)",
			spaceID: 8,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "801", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				id801, _ := strconv.ParseInt(dos[0].SourceTargetID, 10, 64)
				adapter.EXPECT().MGetPrompt(gomock.Any(), int64(8), []*rpc.MGetPromptQuery{
					{PromptID: id801, Version: nil},
				}).Return(nil, errors.New("RPC MGetPrompt error"))
			},
			wantErr: false, // PackSourceInfo handles MGetPrompt error internally, doesn't throw outward
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 1)
				assert.Nil(t, gotDos[0].EvalTargetVersion) // MGetPrompt failed, not processed
			},
		},
		{
			name:    "success scenario - dos is nil (function should handle)",
			spaceID: 9,
			dos:     nil,
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				// MGetPrompt will not be called
			},
			wantErr: false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Nil(t, gotDos)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy dos for each test case to avoid concurrent modification or cross-test case impact
			// For pointer slices, shallow copying element pointers is fine, because the method internally modifies struct fields pointed to by elements,
			// or replaces elements (if the method reallocates EvalTargetVersion).
			// In this specific PackSourceInfo method, it modifies dos[i].EvalTargetVersion, so the original dos will be modified.
			// If the test case's dos needs to be reused in multiple places and should not be modified, deep copy is needed.
			// Here we directly pass tt.dos because each t.Run is independent.
			currentDos := make([]*entity.EvalTarget, len(tt.dos))
			for i, d := range tt.dos { // Simple shallow copy, if EvalTarget has internal pointer fields that will be modified, deeper copy is needed
				currentDos[i] = &entity.EvalTarget{
					// Create a new EvalTarget copy to avoid modifying EvalTarget in original test data
					// This is important for ensuring isolation of each sub-test, especially if EvalTarget structure is complex and its fields will be modified
					SourceTargetID: d.SourceTargetID, EvalTargetType: d.EvalTargetType,
					// If EvalTargetVersion etc. are also pointers and will be modified, deep copy is also needed
					// In this case, EvalTargetVersion will be reassigned, so shallow copying EvalTarget itself and letting the method internally create new EvalTargetVersion is OK
				}
			}
			if tt.dos == nil { // Handle case where dos is nil
				currentDos = nil
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPromptRPCAdapter, currentDos) // Pass currentDos to mock setup
			}

			err := service.PackSourceInfo(ctx, tt.spaceID, currentDos)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantDosCheck != nil {
				tt.wantDosCheck(t, currentDos)
			}
		})
	}
}

func TestPromptSourceEvalTargetServiceImpl_PackSourceVersionInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := &PromptSourceEvalTargetServiceImpl{
		promptRPCAdapter: mockPromptRPCAdapter,
	}

	tests := []struct {
		name      string
		spaceID   int64
		dos       []*entity.EvalTarget
		mockSetup func()
		wantCheck func(t *testing.T, dos []*entity.EvalTarget)
		wantErr   bool
	}{
		{
			name:    "success scenario - normal get Prompt information",
			spaceID: 123,
			dos: []*entity.EvalTarget{
				{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					SourceTargetID: "456",
					EvalTargetVersion: &entity.EvalTargetVersion{
						SourceTargetVersion: "v1.0",
						Prompt: &entity.LoopPrompt{
							PromptID: 456,
						},
					},
				},
			},
			mockSetup: func() {
				mockPromptRPCAdapter.EXPECT().
					MGetPrompt(gomock.Any(), int64(123), []*rpc.MGetPromptQuery{
						{
							PromptID: int64(456),
							Version:  gptr.Of("v1.0"),
						},
					}).Return([]*rpc.LoopPrompt{
					{
						ID: 456,
						PromptBasic: &rpc.PromptBasic{
							DisplayName: gptr.Of("Test Prompt"),
						},
						PromptCommit: &rpc.PromptCommit{
							CommitInfo: &rpc.CommitInfo{
								Version:     gptr.Of("v1.0"),
								Description: gptr.Of("Test Description"),
							},
						},
					},
				}, nil)
			},
			wantCheck: func(t *testing.T, dos []*entity.EvalTarget) {
				assert.Equal(t, "Test Prompt", dos[0].EvalTargetVersion.Prompt.Name)
				assert.Equal(t, "Test Description", dos[0].EvalTargetVersion.Prompt.Description)
			},
			wantErr: false,
		},
		{
			name:    "success scenario - empty input slice",
			spaceID: 123,
			dos:     []*entity.EvalTarget{},
			mockSetup: func() {
				// Empty input should not call RPC
			},
			wantCheck: func(t *testing.T, dos []*entity.EvalTarget) {
				assert.Empty(t, dos)
			},
			wantErr: false,
		},
		{
			name:    "success scenario - Prompt deleted",
			spaceID: 123,
			dos: []*entity.EvalTarget{
				{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					SourceTargetID: "456",
					BaseInfo:       &entity.BaseInfo{},
					EvalTargetVersion: &entity.EvalTargetVersion{
						SourceTargetVersion: "v1.0",
						Prompt: &entity.LoopPrompt{
							PromptID: 456,
						},
					},
				},
			},
			mockSetup: func() {
				mockPromptRPCAdapter.EXPECT().
					MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*rpc.LoopPrompt{}, nil) // Return empty result indicates Prompt doesn't exist
			},
			wantCheck: func(t *testing.T, dos []*entity.EvalTarget) {
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := service.PackSourceVersionInfo(context.Background(), tt.spaceID, tt.dos)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantCheck != nil {
				tt.wantCheck(t, tt.dos)
			}
		})
	}
}

func TestPromptSourceEvalTargetServiceImpl_BatchGetSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := NewPromptSourceEvalTargetServiceImpl(mockAdapter)

	validSpaceID := int64(123)
	validIDs := []string{"1", "2"}

	prompt1 := &rpc.LoopPrompt{
		ID: 1,
		PromptBasic: &rpc.PromptBasic{
			DisplayName: gptr.Of("Prompt 1"),
			Description: gptr.Of("Desc 1"),
		},
	}
	prompt2 := &rpc.LoopPrompt{
		ID: 2,
		PromptBasic: &rpc.PromptBasic{
			DisplayName: gptr.Of("Prompt 2"),
			Description: gptr.Of("Desc 2"),
		},
	}

	tests := []struct {
		name        string
		spaceID     int64
		ids         []string
		mockSetup   func()
		wantTargets []*entity.EvalTarget
		wantErr     bool
	}{
		{
			name:    "success - get multiple prompts",
			spaceID: validSpaceID,
			ids:     validIDs,
			mockSetup: func() {
				mockAdapter.EXPECT().
					MGetPrompt(gomock.Any(), validSpaceID, gomock.Any()).
					Return([]*rpc.LoopPrompt{prompt1, prompt2}, nil)
			},
			wantTargets: []*entity.EvalTarget{
				{
					SpaceID:        validSpaceID,
					SourceTargetID: "1",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						Prompt: &entity.LoopPrompt{
							PromptID:    1,
							Name:        "Prompt 1",
							Description: "Desc 1",
						},
					},
				},
				{
					SpaceID:        validSpaceID,
					SourceTargetID: "2",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						Prompt: &entity.LoopPrompt{
							PromptID:    2,
							Name:        "Prompt 2",
							Description: "Desc 2",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "success - get partial prompts",
			spaceID: validSpaceID,
			ids:     validIDs,
			mockSetup: func() {
				mockAdapter.EXPECT().
					MGetPrompt(gomock.Any(), validSpaceID, gomock.Any()).
					Return([]*rpc.LoopPrompt{prompt1}, nil)
			},
			wantTargets: []*entity.EvalTarget{
				{
					SpaceID:        validSpaceID,
					SourceTargetID: "1",
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						Prompt: &entity.LoopPrompt{
							PromptID:    1,
							Name:        "Prompt 1",
							Description: "Desc 1",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "success - get no prompts",
			spaceID: validSpaceID,
			ids:     validIDs,
			mockSetup: func() {
				mockAdapter.EXPECT().
					MGetPrompt(gomock.Any(), validSpaceID, gomock.Any()).
					Return([]*rpc.LoopPrompt{}, nil)
			},
			wantTargets: []*entity.EvalTarget{},
			wantErr:     false,
		},
		{
			name:    "error - rpc call failed",
			spaceID: validSpaceID,
			ids:     validIDs,
			mockSetup: func() {
				mockAdapter.EXPECT().
					MGetPrompt(gomock.Any(), validSpaceID, gomock.Any()).
					Return(nil, assert.AnError)
			},
			wantTargets: nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			targets, err := service.BatchGetSource(context.Background(), tt.spaceID, tt.ids)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, targets)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantTargets), len(targets))
				for i, want := range tt.wantTargets {
					got := targets[i]
					assert.Equal(t, want.SpaceID, got.SpaceID)
					assert.Equal(t, want.SourceTargetID, got.SourceTargetID)
					assert.Equal(t, want.EvalTargetType, got.EvalTargetType)
					assert.NotNil(t, got.EvalTargetVersion)
					assert.NotNil(t, got.EvalTargetVersion.Prompt)
					assert.Equal(t, want.EvalTargetVersion.Prompt.PromptID, got.EvalTargetVersion.Prompt.PromptID)
					assert.Equal(t, want.EvalTargetVersion.Prompt.Name, got.EvalTargetVersion.Prompt.Name)
					assert.Equal(t, want.EvalTargetVersion.Prompt.Description, got.EvalTargetVersion.Prompt.Description)
				}
			}
		})
	}
}

func TestPromptSourceEvalTargetServiceImpl_RuntimeParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := NewPromptSourceEvalTargetServiceImpl(mockPromptRPCAdapter)

	// Test RuntimeParam method
	runtimeParam := service.RuntimeParam()

	// Verify that PromptRuntimeParam type is returned
	assert.NotNil(t, runtimeParam)
	promptParam, ok := runtimeParam.(*entity.PromptRuntimeParam)
	assert.True(t, ok, "RuntimeParam should return PromptRuntimeParam type")

	// Verify that initialized ModelConfig is nil (because nil was passed in)
	assert.Nil(t, promptParam.ModelConfig)

	// Verify that IRuntimeParam interface methods can be called normally
	demo := runtimeParam.GetJSONDemo()
	assert.NotEmpty(t, demo)
	assert.Contains(t, demo, "model_config")

	jsonValue := runtimeParam.GetJSONValue()
	assert.NotEmpty(t, jsonValue)
}

func TestPromptSourceEvalTargetServiceImpl_EvalType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := NewPromptSourceEvalTargetServiceImpl(mockPromptRPCAdapter)

	// Test EvalType method
	evalType := service.EvalType()
	assert.Equal(t, entity.EvalTargetTypeLoopPrompt, evalType)
}

func TestPromptSourceEvalTargetServiceImpl_Execute_WithRuntimeParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
	service := NewPromptSourceEvalTargetServiceImpl(mockPromptRPCAdapter)

	tests := []struct {
		name           string
		spaceID        int64
		param          *entity.ExecuteEvalTargetParam
		mockSetup      func()
		wantOutputData *entity.EvalTargetOutputData
		wantStatus     entity.EvalTargetRunStatus
		wantErr        bool
		wantErrCode    int32
	}{
		{
			name:    "successful execution with runtime param",
			spaceID: 123,
			param: &entity.ExecuteEvalTargetParam{
				TargetID:            1,
				SourceTargetID:      "456",
				SourceTargetVersion: "v1",
				Input: &entity.EvalTargetInputData{
					InputFields: map[string]*entity.Content{
						"var1": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("test input"),
						},
					},
					Ext: map[string]string{
						consts.TargetExecuteExtRuntimeParamKey: `{"model_config":{"model_id":"test_model","temperature":0.7}}`,
					},
				},
				TargetType: entity.EvalTargetTypeLoopPrompt,
			},
			mockSetup: func() {
				expectedParam := &rpc.ExecutePromptParam{
					PromptID:      456,
					PromptVersion: "v1",
					Variables: []*entity.VariableVal{
						{
							Key:   gptr.Of("var1"),
							Value: gptr.Of("test input"),
						},
					},
					History:      nil,
					RuntimeParam: gptr.Of(`{"model_config":{"model_id":"test_model","temperature":0.7}}`),
				}
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), expectedParam).
					Return(&rpc.ExecutePromptResult{
						Content: gptr.Of("test output with runtime param"),
						TokenUsage: &entity.TokenUsage{
							InputTokens:  120,
							OutputTokens: 60,
						},
					}, nil)
			},
			wantOutputData: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{
					consts.OutputSchemaKey: {
						ContentType: gptr.Of(entity.ContentTypeText),
						Format:      gptr.Of(entity.Markdown),
						Text:        gptr.Of("test output with runtime param"),
					},
				},
				EvalTargetUsage: &entity.EvalTargetUsage{
					InputTokens:  120,
					OutputTokens: 60,
				},
			},
			wantStatus: entity.EvalTargetRunStatusSuccess,
			wantErr:    false,
		},
		{
			name:    "successful execution without runtime param",
			spaceID: 123,
			param: &entity.ExecuteEvalTargetParam{
				TargetID:            1,
				SourceTargetID:      "456",
				SourceTargetVersion: "v1",
				Input: &entity.EvalTargetInputData{
					InputFields: map[string]*entity.Content{
						"var1": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("test input"),
						},
					},
					Ext: map[string]string{}, // No runtime param
				},
				TargetType: entity.EvalTargetTypeLoopPrompt,
			},
			mockSetup: func() {
				expectedParam := &rpc.ExecutePromptParam{
					PromptID:      456,
					PromptVersion: "v1",
					Variables: []*entity.VariableVal{
						{
							Key:   gptr.Of("var1"),
							Value: gptr.Of("test input"),
						},
					},
					History:      nil,
					RuntimeParam: nil, // No runtime param
				}
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), expectedParam).
					Return(&rpc.ExecutePromptResult{
						Content: gptr.Of("test output without runtime param"),
						TokenUsage: &entity.TokenUsage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					}, nil)
			},
			wantOutputData: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{
					consts.OutputSchemaKey: {
						ContentType: gptr.Of(entity.ContentTypeText),
						Format:      gptr.Of(entity.Markdown),
						Text:        gptr.Of("test output without runtime param"),
					},
				},
				EvalTargetUsage: &entity.EvalTargetUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
			wantStatus: entity.EvalTargetRunStatusSuccess,
			wantErr:    false,
		},
		{
			name:    "successful execution with empty runtime param",
			spaceID: 123,
			param: &entity.ExecuteEvalTargetParam{
				TargetID:            1,
				SourceTargetID:      "456",
				SourceTargetVersion: "v1",
				Input: &entity.EvalTargetInputData{
					InputFields: map[string]*entity.Content{},
					Ext: map[string]string{
						consts.TargetExecuteExtRuntimeParamKey: "", // Empty runtime param
					},
				},
				TargetType: entity.EvalTargetTypeLoopPrompt,
			},
			mockSetup: func() {
				expectedParam := &rpc.ExecutePromptParam{
					PromptID:      456,
					PromptVersion: "v1",
					Variables:     []*entity.VariableVal{},
					History:       nil,
					RuntimeParam:  nil, // Empty string should not set RuntimeParam
				}
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), expectedParam).
					Return(&rpc.ExecutePromptResult{
						Content: gptr.Of("test output"),
					}, nil)
			},
			wantOutputData: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{
					consts.OutputSchemaKey: {
						ContentType: gptr.Of(entity.ContentTypeText),
						Format:      gptr.Of(entity.Markdown),
						Text:        gptr.Of("test output"),
					},
				},
			},
			wantStatus: entity.EvalTargetRunStatusSuccess,
			wantErr:    false,
		},
		{
			name:    "successful execution with runtime param and other ext values",
			spaceID: 123,
			param: &entity.ExecuteEvalTargetParam{
				TargetID:            1,
				SourceTargetID:      "456",
				SourceTargetVersion: "v1",
				Input: &entity.EvalTargetInputData{
					InputFields: map[string]*entity.Content{},
					Ext: map[string]string{
						consts.TargetExecuteExtRuntimeParamKey: `{"model_config":{"model_id":"test_model"}}`,
						"other_ext_key":                        "other_ext_value",
					},
				},
				TargetType: entity.EvalTargetTypeLoopPrompt,
			},
			mockSetup: func() {
				expectedParam := &rpc.ExecutePromptParam{
					PromptID:      456,
					PromptVersion: "v1",
					Variables:     []*entity.VariableVal{},
					History:       nil,
					RuntimeParam:  gptr.Of(`{"model_config":{"model_id":"test_model"}}`),
				}
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), expectedParam).
					Return(&rpc.ExecutePromptResult{
						Content: gptr.Of("test output with mixed ext"),
					}, nil)
			},
			wantOutputData: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{
					consts.OutputSchemaKey: {
						ContentType: gptr.Of(entity.ContentTypeText),
						Format:      gptr.Of(entity.Markdown),
						Text:        gptr.Of("test output with mixed ext"),
					},
				},
			},
			wantStatus: entity.EvalTargetRunStatusSuccess,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			gotOutputData, gotStatus, err := service.Execute(context.Background(), tt.spaceID, tt.param)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantStatus, gotStatus)

			if tt.wantOutputData != nil {
				// Validate output fields
				assert.Equal(t, gptr.Indirect(tt.wantOutputData.OutputFields[consts.OutputSchemaKey].Text), gptr.Indirect(gotOutputData.OutputFields[consts.OutputSchemaKey].Text))
				// Validate usage
				if tt.wantOutputData.EvalTargetUsage != nil {
					assert.Equal(t, tt.wantOutputData.EvalTargetUsage.InputTokens, gotOutputData.EvalTargetUsage.InputTokens)
					assert.Equal(t, tt.wantOutputData.EvalTargetUsage.OutputTokens, gotOutputData.EvalTargetUsage.OutputTokens)
				}
				// Validate execution time
				assert.NotNil(t, gotOutputData.TimeConsumingMS)
				assert.GreaterOrEqual(t, *gotOutputData.TimeConsumingMS, int64(0))
			}
		})
	}
}
