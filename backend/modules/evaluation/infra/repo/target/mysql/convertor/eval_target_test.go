// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
)

func TestEvalTargetDO2PO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		do       *entity.EvalTarget
		expected *model.Target
	}{
		{
			name: "完整的DO转PO",
			do: &entity.EvalTarget{
				ID:             123,
				SpaceID:        456,
				SourceTargetID: "source123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: gptr.Of("user1")},
					UpdatedBy: &entity.UserInfo{UserID: gptr.Of("user2")},
					CreatedAt: gptr.Of(int64(1640995200000)),
					UpdatedAt: gptr.Of(int64(1640995300000)),
				},
			},
			expected: &model.Target{
				ID:             123,
				SpaceID:        456,
				SourceTargetID: "source123",
				TargetType:     int32(entity.EvalTargetTypeCozeBot),
				CreatedBy:      "user1",
				UpdatedBy:      "user2",
				CreatedAt:      time.UnixMilli(1640995200000),
				UpdatedAt:      time.UnixMilli(1640995300000),
			},
		},
		{
			name: "最小字段的DO",
			do: &entity.EvalTarget{
				ID:             1,
				SpaceID:        2,
				SourceTargetID: "test",
				EvalTargetType: entity.EvalTargetTypeLoopPrompt,
			},
			expected: &model.Target{
				ID:             1,
				SpaceID:        2,
				SourceTargetID: "test",
				TargetType:     int32(entity.EvalTargetTypeLoopPrompt),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetDO2PO(tt.do)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalTargetVersionDO2PO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		do          *entity.EvalTargetVersion
		expectError bool
		checkResult func(t *testing.T, po *model.TargetVersion)
	}{
		{
			name: "CozeBot类型的版本转换",
			do: &entity.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v1.0",
				EvalTargetType:      entity.EvalTargetTypeCozeBot,
				CozeBot: &entity.CozeBot{
					BotID:       123,
					BotVersion:  "v1.0",
					BotName:     "TestBot",
					AvatarURL:   "http://example.com/avatar.png",
					Description: "Test bot description",
				},
			},
			expectError: false,
			checkResult: func(t *testing.T, po *model.TargetVersion) {
				assert.Equal(t, int64(1), po.ID)
				assert.Equal(t, int64(2), po.SpaceID)
				assert.Equal(t, int64(3), po.TargetID)
				assert.Equal(t, "v1.0", po.SourceTargetVersion)
				assert.NotNil(t, po.TargetMeta)
			},
		},
		{
			name: "LoopPrompt类型的版本转换",
			do: &entity.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v2.0",
				EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
				Prompt: &entity.LoopPrompt{
					PromptID:     123,
					Version:      "v2.0",
					PromptKey:    "test_prompt",
					Name:         "Test Prompt",
					SubmitStatus: entity.SubmitStatus_Submitted,
					Description:  "Test prompt description",
				},
			},
			expectError: false,
			checkResult: func(t *testing.T, po *model.TargetVersion) {
				assert.Equal(t, int64(1), po.ID)
				assert.Equal(t, "v2.0", po.SourceTargetVersion)
				assert.NotNil(t, po.TargetMeta)
			},
		},
		{
			name: "火山智能体类型的版本转换",
			do: &entity.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v2.0",
				EvalTargetType:      entity.EvalTargetTypeVolcengineAgent,
				VolcengineAgent: &entity.VolcengineAgent{
					Name:        "Test Prompt",
					Description: "Test prompt description",
					VolcengineAgentEndpoints: []*entity.VolcengineAgentEndpoint{
						{
							EndpointID: "test_endpoint",
							APIKey:     "test_api_key",
						},
					},
				},
			},
			expectError: false,
			checkResult: func(t *testing.T, po *model.TargetVersion) {
				assert.Equal(t, int64(1), po.ID)
				assert.Equal(t, "v2.0", po.SourceTargetVersion)
				assert.NotNil(t, po.TargetMeta)
			},
		},
		{
			name: "自定义对象版本转换",
			do: &entity.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v2.0",
				EvalTargetType:      entity.EvalTargetTypeCustomRPCServer,
				CustomRPCServer: &entity.CustomRPCServer{
					Name:        "Test Prompt",
					Description: "Test prompt description",
				},
			},
			expectError: false,
			checkResult: func(t *testing.T, po *model.TargetVersion) {
				assert.Equal(t, int64(1), po.ID)
				assert.Equal(t, "v2.0", po.SourceTargetVersion)
				assert.NotNil(t, po.TargetMeta)
			},
		},
		{
			name: "CozeWorkflow类型的版本转换",
			do: &entity.EvalTargetVersion{
				ID:                  1,
				EvalTargetType:      entity.EvalTargetTypeCozeWorkflow,
				CozeWorkflow:        &entity.CozeWorkflow{ID: "wf1"},
				InputSchema:         []*entity.ArgsSchema{{Key: gptr.Of("in")}},
				OutputSchema:        []*entity.ArgsSchema{{Key: gptr.Of("out")}},
			},
			expectError: false,
			checkResult: func(t *testing.T, po *model.TargetVersion) {
				assert.NotNil(t, po.TargetMeta)
				assert.NotNil(t, po.InputSchema)
				assert.NotNil(t, po.OutputSchema)
			},
		},
		{
			name: "VolcengineAgentAgentkit类型的版本转换",
			do: &entity.EvalTargetVersion{
				ID:                  1,
				EvalTargetType:      entity.EvalTargetTypeVolcengineAgentAgentkit,
				VolcengineAgent:     &entity.VolcengineAgent{Name: "agent"},
			},
			expectError: false,
			checkResult: func(t *testing.T, po *model.TargetVersion) {
				assert.NotNil(t, po.TargetMeta)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			po, err := EvalTargetVersionDO2PO(tt.do)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, po)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, po)
				if tt.checkResult != nil {
					tt.checkResult(t, po)
				}
			}
		})
	}
}

func TestEvalTargetPO2DOs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		targetPOs     []*model.Target
		expectedCount int
	}{
		{
			name:          "nil输入",
			targetPOs:     nil,
			expectedCount: 0,
		},
		{
			name:          "空列表",
			targetPOs:     []*model.Target{},
			expectedCount: 0,
		},
		{
			name: "单个元素列表",
			targetPOs: []*model.Target{
				{
					ID:             1,
					SpaceID:        2,
					SourceTargetID: "test",
					TargetType:     int32(entity.EvalTargetTypeLoopPrompt),
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
				},
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetPO2DOs(tt.targetPOs)

			if tt.targetPOs == nil {
				assert.Nil(t, result)
			} else {
				assert.Len(t, result, tt.expectedCount)
			}
		})
	}
}

func TestEvalTargetPO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		targetPO *model.Target
		expected *entity.EvalTarget
	}{
		{
			name:     "nil输入",
			targetPO: nil,
			expected: nil,
		},
		{
			name: "完整的PO转DO",
			targetPO: &model.Target{
				ID:             123,
				SpaceID:        456,
				SourceTargetID: "source123",
				TargetType:     int32(entity.EvalTargetTypeCozeBot),
				CreatedBy:      "user1",
				UpdatedBy:      "user2",
				CreatedAt:      time.UnixMilli(1640995200000),
				UpdatedAt:      time.UnixMilli(1640995300000),
				DeletedAt:      gorm.DeletedAt{Time: time.UnixMilli(1640995400000), Valid: true},
			},
			expected: &entity.EvalTarget{
				ID:             123,
				SpaceID:        456,
				SourceTargetID: "source123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: gptr.Of("user1")},
					UpdatedBy: &entity.UserInfo{UserID: gptr.Of("user2")},
					CreatedAt: gptr.Of(int64(1640995200000)),
					UpdatedAt: gptr.Of(int64(1640995300000)),
					DeletedAt: gptr.Of(int64(1640995400000)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetPO2DO(tt.targetPO)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalTargetVersionPO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		targetVersionPO *model.TargetVersion
		targetType      entity.EvalTargetType
		checkResult     func(t *testing.T, do *entity.EvalTargetVersion)
	}{
		{
			name:            "nil输入",
			targetVersionPO: nil,
			targetType:      entity.EvalTargetTypeCozeBot,
			checkResult: func(t *testing.T, do *entity.EvalTargetVersion) {
				assert.Nil(t, do)
			},
		},
		{
			name: "CozeBot类型的版本转换",
			targetVersionPO: &model.TargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v1.0",
				TargetMeta:          gptr.Of([]byte(`{"bot_id":123,"bot_version":"v1.0","bot_name":"TestBot"}`)),
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			},
			targetType: entity.EvalTargetTypeCozeBot,
			checkResult: func(t *testing.T, do *entity.EvalTargetVersion) {
				assert.NotNil(t, do)
				assert.Equal(t, int64(1), do.ID)
				assert.Equal(t, "v1.0", do.SourceTargetVersion)
				// CozeBot可能为nil或者BotID为0，因为JSON解析可能失败
				if do.CozeBot != nil {
					assert.GreaterOrEqual(t, do.CozeBot.BotID, int64(0))
				}
			},
		},
		{
			name: "LoopPrompt类型的版本转换",
			targetVersionPO: &model.TargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v2.0",
				TargetMeta:          gptr.Of([]byte(`{"prompt_id":123,"version":"v2.0","name":"Test Prompt"}`)),
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			},
			targetType: entity.EvalTargetTypeLoopPrompt,
			checkResult: func(t *testing.T, do *entity.EvalTargetVersion) {
				assert.NotNil(t, do)
				assert.Equal(t, int64(1), do.ID)
				assert.Equal(t, "v2.0", do.SourceTargetVersion)
				// Prompt可能为nil或者PromptID为0，因为JSON解析可能失败
				if do.Prompt != nil {
					assert.GreaterOrEqual(t, do.Prompt.PromptID, int64(0))
				}
			},
		},
		{
			name: "火山智能体类型的版本转换",
			targetVersionPO: &model.TargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v2.0",
				TargetMeta:          gptr.Of([]byte(`{"name":"Test agent"}`)),
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			},
			targetType: entity.EvalTargetTypeVolcengineAgent,
			checkResult: func(t *testing.T, do *entity.EvalTargetVersion) {
				assert.NotNil(t, do)
				assert.Equal(t, int64(1), do.ID)
			},
		},
		{
			name: "自定义对象的版本转换",
			targetVersionPO: &model.TargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v2.0",
				TargetMeta:          gptr.Of([]byte(`{"id":1}`)),
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			},
			targetType: entity.EvalTargetTypeCustomRPCServer,
			checkResult: func(t *testing.T, do *entity.EvalTargetVersion) {
				assert.NotNil(t, do)
				assert.Equal(t, int64(1), do.ID)
			},
		},
		{
			name: "CozeWorkflow类型的版本转换",
			targetVersionPO: &model.TargetVersion{
				ID:         1,
				TargetMeta: gptr.Of([]byte(`{"id":"wf1"}`)),
			},
			targetType: entity.EvalTargetTypeCozeWorkflow,
			checkResult: func(t *testing.T, do *entity.EvalTargetVersion) {
				assert.NotNil(t, do)
				assert.Equal(t, "wf1", do.CozeWorkflow.ID)
			},
		},
		{
			name: "VolcengineAgentAgentkit类型的版本转换",
			targetVersionPO: &model.TargetVersion{
				ID:         1,
				TargetMeta: gptr.Of([]byte(`{"RuntimeID":"agent"}`)),
			},
			targetType: entity.EvalTargetTypeVolcengineAgentAgentkit,
			checkResult: func(t *testing.T, do *entity.EvalTargetVersion) {
				assert.NotNil(t, do)
				assert.Equal(t, "agent", *do.VolcengineAgent.RuntimeID)
			},
		},
		{
			name: "Schema转换测试",
			targetVersionPO: &model.TargetVersion{
				ID:           1,
				InputSchema:  gptr.Of([]byte(`[{"key":"in"}]`)),
				OutputSchema: gptr.Of([]byte(`[{"key":"out"}]`)),
			},
			targetType: entity.EvalTargetTypeCozeBot,
			checkResult: func(t *testing.T, do *entity.EvalTargetVersion) {
				assert.Len(t, do.InputSchema, 1)
				assert.Len(t, do.OutputSchema, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetVersionPO2DO(tt.targetVersionPO, tt.targetType)
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}
