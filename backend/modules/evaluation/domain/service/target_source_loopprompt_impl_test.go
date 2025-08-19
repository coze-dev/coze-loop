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
			name:    "成功执行 - 返回文本内容",
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
					ExecutePrompt(gomock.Any(), int64(123), &rpc.ExecutePromptParam{
						PromptID:      456,
						PromptVersion: "v1",
						Variables: []*entity.VariableVal{
							{
								Key:   gptr.Of("var1"),
								Value: gptr.Of("test input"),
							},
						},
						History: []*entity.Message{
							{
								Role: entity.RoleUser,
								Content: &entity.Content{
									ContentType: gptr.Of(entity.ContentTypeText),
									Text:        gptr.Of("test message"),
								},
							},
						},
					}).
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
			name:    "执行失败 - 无效的 SourceTargetID",
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
			name:    "执行失败 - RPC 调用错误",
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
			name:    "成功执行 - 返回工具调用结果",
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
				// 验证输出字段
				assert.Equal(t, gptr.Indirect(tt.wantOutputData.OutputFields[consts.OutputSchemaKey].Text), gptr.Indirect(gotOutputData.OutputFields[consts.OutputSchemaKey].Text))
				// 验证使用情况
				if tt.wantOutputData.EvalTargetUsage != nil {
					assert.Equal(t, tt.wantOutputData.EvalTargetUsage.InputTokens, gotOutputData.EvalTargetUsage.InputTokens)
					assert.Equal(t, tt.wantOutputData.EvalTargetUsage.OutputTokens, gotOutputData.EvalTargetUsage.OutputTokens)
				}
				// 验证执行时间
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
			name:                "成功场景 - 完整数据",
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
									{Key: gptr.Of("var1")},
									{Key: gptr.Of("var2")},
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

				assert.Len(t, evalTarget.EvalTargetVersion.InputSchema, 2)
				if len(evalTarget.EvalTargetVersion.InputSchema) == 2 {
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
			name:                "成功场景 - PromptCommit.Detail.PromptTemplate.VariableDefs 为空",
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
			name:                "成功场景 - PromptCommit.Detail.PromptTemplate 为 nil",
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
			name:                "成功场景 - PromptCommit.Detail 为 nil",
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
			name:                "成功场景 - PromptCommit 为 nil",
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
			name:                "失败场景 - strconv.ParseInt 失败",
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
			name:                "失败场景 - promptRPCAdapter.GetPrompt 返回错误",
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
			name:                "失败场景 - promptRPCAdapter.GetPrompt 返回 nil prompt",
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
			name: "成功获取列表 - cursor为nil, 有更多数据",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](1),
				PageSize: gptr.Of[int32](1),
				Cursor:   nil, // page 将为 1
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
				}, gptr.Of[int32](1), nil) // total 字段在 ListSource 中未使用，可以为任意值
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
					},
				},
			},
			wantNextCursor: "2",  // page (1) + 1
			wantHasMore:    true, // len(prompts) (1) == PageSize (1)
			wantErr:        false,
		},
		{
			name: "成功获取列表 - cursor有效, 没有更多数据",
			param: &entity.ListSourceParam{
				SpaceID:  gptr.Of[int64](2),
				PageSize: gptr.Of[int32](2),
				Cursor:   gptr.Of("2"), // page 将为 2
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
					},
				},
			},
			wantNextCursor: "3",   // page (2) + 1
			wantHasMore:    false, // len(prompts) (1) != PageSize (2)
			wantErr:        false,
		},
		{
			name: "成功获取列表 - PromptBasic为nil",
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
					},
				},
			},
			wantNextCursor: "2",
			wantHasMore:    true,
			wantErr:        false,
		},
		{
			name: "成功获取列表 - 返回空列表",
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
			name: "失败 - buildPageByCursor返回错误 (无效cursor)",
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
			name: "失败 - promptRPCAdapter.ListPrompt返回错误",
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
			name: "边界情况 - PageSize为nil (gptr.From会处理为0)",
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
			name: "成功获取版本列表 - 有数据，有下一页",
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
				},
			},
			wantNextCursor: "cursor_next",
			wantHasMore:    true, // len(info) (1) == PageSize (1)
			wantErr:        false,
		},
		{
			name: "成功获取版本列表 - 有数据，没有下一页",
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
				},
			},
			wantNextCursor: "cursor_final",
			wantHasMore:    false, // len(info) (1) != PageSize (2)
			wantErr:        false,
		},
		{
			name: "成功获取版本列表 - PromptBasic 为 nil",
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
				},
			},
			wantNextCursor: "next",
			wantHasMore:    true,
			wantErr:        false,
		},
		{
			name: "成功获取版本列表 - ListPromptVersion 返回空列表",
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
			name: "失败 - 无效的 SourceTargetID (strconv.ParseInt 失败)",
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
			name: "失败 - GetPrompt 返回错误",
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
			name: "失败 - GetPrompt 返回 nil prompt (ResourceNotFound)",
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
			expectedErrorMsg: errorx.NewByCode(errno.ResourceNotFoundCode).Error(), // 比较具体的错误信息
			expectedErrCode:  errno.ResourceNotFoundCode,
		},
		{
			name: "失败 - ListPromptVersion 返回错误",
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
			name: "边界情况 - PageSize为nil (gptr.From会处理为0), hasMore判断依赖RPC返回数量",
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
				},
			},
			wantNextCursor: "cursor_pagesize_nil",
			wantHasMore:    false, // len(info) is 1, gptr.Indirect(nil PageSize) is 0. 1 == 0 is false.
			wantErr:        false,
		},
		{
			name: "边界情况 - PageSize为nil, ListPromptVersion返回空列表, hasMore为true",
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
		dos          []*entity.EvalTarget // 输入的 dos，会被方法修改
		setupMocks   func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget)
		wantErr      bool // PackSourceInfo 设计上不返回 error，所以通常为 false
		wantDosCheck func(t *testing.T, gotDos []*entity.EvalTarget)
	}{
		{
			name:    "成功场景 - 正常打包信息",
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
			name:    "成功场景 - 包含非LoopPrompt类型及MGetPrompt部分匹配",
			spaceID: 2,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "201", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
				{SourceTargetID: "202", EvalTargetType: entity.EvalTargetTypeCozeBot}, // 非LoopPrompt
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
					// ID 203 不在返回结果中
				}, nil)
			},
			wantErr: false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 3)
				assert.NotNil(t, gotDos[0].EvalTargetVersion)
				assert.NotNil(t, gotDos[0].EvalTargetVersion.Prompt)
				assert.Equal(t, "Prompt 201", gotDos[0].EvalTargetVersion.Prompt.Name)
				assert.Nil(t, gotDos[1].EvalTargetVersion) // 非LoopPrompt类型，应未被处理
				assert.Nil(t, gotDos[2].EvalTargetVersion) // LoopPrompt类型，但MGetPrompt未返回，应未被处理
			},
		},
		{
			name:    "成功场景 - MGetPrompt返回的PromptBasic为nil或DisplayName为nil",
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
			name:       "边界场景 - dos 为空",
			spaceID:    4,
			dos:        []*entity.EvalTarget{},
			setupMocks: nil, // MGetPrompt 不会被调用
			wantErr:    false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Empty(t, gotDos)
			},
		},
		{
			name:    "边界场景 - dos 中无 LoopPrompt 类型",
			spaceID: 5,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "501", EvalTargetType: entity.EvalTargetTypeCozeBot},
			},
			setupMocks: nil, // MGetPrompt 不会被调用
			wantErr:    false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 1)
				assert.Nil(t, gotDos[0].EvalTargetVersion)
			},
		},
		{
			name:    "边界场景 - MGetPrompt 返回空列表",
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
				assert.Nil(t, gotDos[0].EvalTargetVersion) // MGetPrompt返回空，未找到匹配，不应填充
			},
		},
		{
			name:    "异常场景 - strconv.ParseInt 失败 (函数内部处理，不返回error)",
			spaceID: 7,
			dos: []*entity.EvalTarget{
				{SourceTargetID: "abc", EvalTargetType: entity.EvalTargetTypeLoopPrompt}, // 无效ID
				{SourceTargetID: "701", EvalTargetType: entity.EvalTargetTypeLoopPrompt},
			},
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				// "abc" 会导致 ParseInt 失败，所以 MGetPrompt 只会查询 "701"
				id701, _ := strconv.ParseInt(dos[1].SourceTargetID, 10, 64)
				adapter.EXPECT().MGetPrompt(gomock.Any(), int64(7), []*rpc.MGetPromptQuery{
					{PromptID: id701, Version: nil},
				}).Return([]*rpc.LoopPrompt{
					{ID: id701, PromptBasic: &rpc.PromptBasic{DisplayName: gptr.Of("Prompt 701")}},
				}, nil)
				// 注意：这里可以mock logs.CtxError 来验证它是否被调用，但通常不这么做，而是检查其副作用
			},
			wantErr: false, // PackSourceInfo 内部处理了ParseInt错误，不向外抛出
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 2)
				assert.Nil(t, gotDos[0].EvalTargetVersion) // ParseInt 失败，未处理
				assert.NotNil(t, gotDos[1].EvalTargetVersion)
				assert.NotNil(t, gotDos[1].EvalTargetVersion.Prompt)
				assert.Equal(t, "Prompt 701", gotDos[1].EvalTargetVersion.Prompt.Name)
			},
		},
		{
			name:    "异常场景 - MGetPrompt 返回错误 (函数内部处理，不返回error)",
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
			wantErr: false, // PackSourceInfo 内部处理了MGetPrompt错误，不向外抛出
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Len(t, gotDos, 1)
				assert.Nil(t, gotDos[0].EvalTargetVersion) // MGetPrompt 失败，未处理
			},
		},
		{
			name:    "成功场景 - dos 为 nil (函数应能处理)",
			spaceID: 9,
			dos:     nil,
			setupMocks: func(adapter *mocks.MockIPromptRPCAdapter, dos []*entity.EvalTarget) {
				// MGetPrompt 不会被调用
			},
			wantErr: false,
			wantDosCheck: func(t *testing.T, gotDos []*entity.EvalTarget) {
				assert.Nil(t, gotDos)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个测试用例复制一份dos，以避免并发修改或跨测试用例影响
			// 对于指针切片，浅拷贝元素指针是可以的，因为方法内部是修改元素指向的结构体字段，
			// 或者替换元素（如果方法会重新分配EvalTargetVersion）。
			// 在这个特定的PackSourceInfo方法中，它修改的是dos[i].EvalTargetVersion，所以原始dos会被修改。
			// 如果测试用例的dos需要在多个地方复用且不想被修改，则需要深拷贝。
			// 这里我们直接传入tt.dos，因为每个t.Run是独立的。
			currentDos := make([]*entity.EvalTarget, len(tt.dos))
			for i, d := range tt.dos { // 简单的浅拷贝，如果EvalTarget内部有指针字段且会被修改，则需要更深的拷贝
				if d != nil {
					// 创建一个新的EvalTarget副本，以避免修改原始测试数据中的EvalTarget
					// 这对于确保每个子测试的隔离性很重要，特别是如果EvalTarget结构复杂且其字段会被修改
					copiedTarget := *d
					// 如果EvalTargetVersion等也是指针，并且会被修改，也需要深拷贝
					// 在此例中，EvalTargetVersion会被重新赋值，所以浅拷贝EvalTarget本身，然后让方法内部创建新的EvalTargetVersion是OK的
					currentDos[i] = &copiedTarget
				}
			}
			if tt.dos == nil { // 处理dos为nil的情况
				currentDos = nil
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockPromptRPCAdapter, currentDos) // 传递 currentDos 给 mock 设置
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
			name:    "成功场景 - 正常获取Prompt信息",
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
			name:    "成功场景 - 空输入切片",
			spaceID: 123,
			dos:     []*entity.EvalTarget{},
			mockSetup: func() {
				// 空输入不应该调用RPC
			},
			wantCheck: func(t *testing.T, dos []*entity.EvalTarget) {
				assert.Empty(t, dos)
			},
			wantErr: false,
		},
		{
			name:    "成功场景 - Prompt已删除",
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
					Return([]*rpc.LoopPrompt{}, nil) // 返回空结果表示Prompt不存在
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
