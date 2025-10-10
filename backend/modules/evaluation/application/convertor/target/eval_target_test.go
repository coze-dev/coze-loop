// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	dto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	do "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestEvalTargetDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		targetDTO *dto.EvalTarget
		expected  *do.EvalTarget
	}{
		{
			name:      "nil输入",
			targetDTO: nil,
			expected:  nil,
		},
		{
			name: "完整的EvalTarget转换",
			targetDTO: &dto.EvalTarget{
				ID:             gptr.Of(int64(123)),
				WorkspaceID:    gptr.Of(int64(456)),
				SourceTargetID: gptr.Of("source123"),
				EvalTargetType: gptr.Of(dto.EvalTargetType_CozeBot),
				BaseInfo: &commondto.BaseInfo{
					CreatedAt: gptr.Of(int64(1640995200000)),
					UpdatedAt: gptr.Of(int64(1640995200000)),
				},
				EvalTargetVersion: &dto.EvalTargetVersion{
					ID:                  gptr.Of(int64(789)),
					WorkspaceID:         gptr.Of(int64(456)),
					TargetID:            gptr.Of(int64(123)),
					SourceTargetVersion: gptr.Of("v1.0"),
				},
			},
			expected: &do.EvalTarget{
				ID:             123,
				SpaceID:        456,
				SourceTargetID: "source123",
				EvalTargetType: do.EvalTargetType(dto.EvalTargetType_CozeBot),
				BaseInfo: &do.BaseInfo{
					CreatedAt: gptr.Of(int64(1640995200000)),
					UpdatedAt: gptr.Of(int64(1640995200000)),
				},
				EvalTargetVersion: &do.EvalTargetVersion{
					ID:                  789,
					SpaceID:             456,
					TargetID:            123,
					SourceTargetVersion: "v1.0",
				},
			},
		},
		{
			name: "最小字段的EvalTarget",
			targetDTO: &dto.EvalTarget{
				ID:             gptr.Of(int64(1)),
				WorkspaceID:    gptr.Of(int64(2)),
				SourceTargetID: gptr.Of("test"),
				EvalTargetType: gptr.Of(dto.EvalTargetType_CozeLoopPrompt),
			},
			expected: &do.EvalTarget{
				ID:                1,
				SpaceID:           2,
				SourceTargetID:    "test",
				EvalTargetType:    do.EvalTargetType(dto.EvalTargetType_CozeLoopPrompt),
				EvalTargetVersion: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetDTO2DO(tt.targetDTO)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalTargetVersionDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		targetVersionDTO *dto.EvalTargetVersion
		expected         *do.EvalTargetVersion
	}{
		{
			name:             "nil输入",
			targetVersionDTO: nil,
			expected:         nil,
		},
		{
			name: "基本版本转换",
			targetVersionDTO: &dto.EvalTargetVersion{
				ID:                  gptr.Of(int64(1)),
				WorkspaceID:         gptr.Of(int64(2)),
				TargetID:            gptr.Of(int64(3)),
				SourceTargetVersion: gptr.Of("v1.0"),
			},
			expected: &do.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v1.0",
			},
		},
		{
			name: "自定义对象转换",
			targetVersionDTO: &dto.EvalTargetVersion{
				ID:                  gptr.Of(int64(1)),
				WorkspaceID:         gptr.Of(int64(2)),
				TargetID:            gptr.Of(int64(3)),
				SourceTargetVersion: gptr.Of("v1.0"),
				EvalTargetContent: &dto.EvalTargetContent{
					CustomRPCServer: &dto.CustomRPCServer{
						ID:   gptr.Of(int64(4)),
						Name: gptr.Of("test"),
						InvokeHTTPInfo: &dto.HTTPInfo{
							Method: gptr.Of(""),
							Path:   gptr.Of(""),
						},
						CustomEvalTarget: &dto.CustomEvalTarget{
							ID:        gptr.Of(""),
							Name:      gptr.Of(""),
							AvatarURL: gptr.Of(""),
						},
					},
				},
			},
			expected: &do.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v1.0",
				CustomRPCServer: &do.CustomRPCServer{
					ID:   4,
					Name: "test",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetVersionDTO2DO(tt.targetVersionDTO)
			if tt.name == "自定义对象转换" {
				assert.Equal(t, result.CustomRPCServer.Name, tt.expected.CustomRPCServer.Name)
				return
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalTargetListDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		targetDOList  []*do.EvalTarget
		expectedCount int
	}{
		{
			name:          "空列表",
			targetDOList:  []*do.EvalTarget{},
			expectedCount: 0,
		},
		{
			name: "单个元素列表",
			targetDOList: []*do.EvalTarget{
				{
					ID:             1,
					SpaceID:        2,
					SourceTargetID: "test",
					EvalTargetType: do.EvalTargetTypeLoopPrompt,
				},
			},
			expectedCount: 1,
		},
		{
			name: "多个元素列表",
			targetDOList: []*do.EvalTarget{
				{
					ID:             1,
					SpaceID:        2,
					SourceTargetID: "test1",
					EvalTargetType: do.EvalTargetTypeLoopPrompt,
				},
				{
					ID:             2,
					SpaceID:        3,
					SourceTargetID: "test2",
					EvalTargetType: do.EvalTargetTypeCozeBot,
				},
			},
			expectedCount: 2,
		},
		{
			name:          "nil列表",
			targetDOList:  nil,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetListDO2DTO(tt.targetDOList)
			assert.Len(t, result, tt.expectedCount)

			// 验证每个元素都正确转换
			for i, targetDO := range tt.targetDOList {
				expectedDTO := EvalTargetDO2DTO(targetDO)
				assert.Equal(t, expectedDTO, result[i])
			}
		})
	}
}

func TestEvalTargetDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		targetDO *do.EvalTarget
		expected *dto.EvalTarget
	}{
		{
			name:     "nil输入",
			targetDO: nil,
			expected: nil,
		},
		{
			name: "基本EvalTarget转换",
			targetDO: &do.EvalTarget{
				ID:             123,
				SpaceID:        456,
				SourceTargetID: "source123",
				EvalTargetType: do.EvalTargetTypeCozeBot,
				BaseInfo: &do.BaseInfo{
					CreatedAt: gptr.Of(int64(1640995200000)),
					UpdatedAt: gptr.Of(int64(1640995200000)),
				},
			},
			expected: &dto.EvalTarget{
				ID:             gptr.Of(int64(123)),
				WorkspaceID:    gptr.Of(int64(456)),
				SourceTargetID: gptr.Of("source123"),
				EvalTargetType: gptr.Of(dto.EvalTargetType(do.EvalTargetTypeCozeBot)),
				BaseInfo: &commondto.BaseInfo{
					CreatedAt: gptr.Of(int64(1640995200000)),
					UpdatedAt: gptr.Of(int64(1640995200000)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetDO2DTO(tt.targetDO)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalTargetVersionDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		targetVersionDO *do.EvalTargetVersion
		expected        *dto.EvalTargetVersion
	}{
		{
			name:            "nil输入",
			targetVersionDO: nil,
			expected:        nil,
		},
		{
			name: "基本版本转换",
			targetVersionDO: &do.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v1.0",
				EvalTargetType:      do.EvalTargetTypeCozeBot,
			},
			expected: &dto.EvalTargetVersion{
				ID:                  gptr.Of(int64(1)),
				WorkspaceID:         gptr.Of(int64(2)),
				TargetID:            gptr.Of(int64(3)),
				SourceTargetVersion: gptr.Of("v1.0"),
				EvalTargetContent: &dto.EvalTargetContent{
					InputSchemas:  []*commondto.ArgsSchema{},
					OutputSchemas: []*commondto.ArgsSchema{},
				},
			},
		},
		{
			name: "火山转换",
			targetVersionDO: &do.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v1.0",
				EvalTargetType:      do.EvalTargetTypeVolcengineAgent,
				VolcengineAgent: &do.VolcengineAgent{
					VolcengineAgentEndpoints: []*do.VolcengineAgentEndpoint{
						{
							EndpointID: "test",
							APIKey:     "test",
						},
					},
				},
			},
			expected: &dto.EvalTargetVersion{
				ID:                  gptr.Of(int64(1)),
				WorkspaceID:         gptr.Of(int64(2)),
				TargetID:            gptr.Of(int64(3)),
				SourceTargetVersion: gptr.Of("v1.0"),
				EvalTargetContent: &dto.EvalTargetContent{
					InputSchemas:  []*commondto.ArgsSchema{},
					OutputSchemas: []*commondto.ArgsSchema{},
					VolcengineAgent: &dto.VolcengineAgent{
						Name:        gptr.Of("agent"),
						Description: gptr.Of("test"),
						VolcengineAgentEndpoints: []*dto.VolcengineAgentEndpoint{
							{
								EndpointID: gptr.Of("test"),
								APIKey:     gptr.Of("test"),
							},
						},
					},
				},
			},
		},
		{
			name: "自定义对象转换",
			targetVersionDO: &do.EvalTargetVersion{
				ID:                  1,
				SpaceID:             2,
				TargetID:            3,
				SourceTargetVersion: "v1.0",
				EvalTargetType:      do.EvalTargetTypeCustomRPCServer,
				CustomRPCServer: &do.CustomRPCServer{
					Name:        "test",
					Description: "test",
					InvokeHTTPInfo: &do.HTTPInfo{
						Method: "GET",
						Path:   "/test",
					},
					AsyncInvokeHTTPInfo: &do.HTTPInfo{
						Method: "GET",
						Path:   "/test",
					},
					SearchHTTPInfo: &do.HTTPInfo{
						Method: "GET",
						Path:   "/test",
					},
					CustomEvalTarget: &do.CustomEvalTarget{
						Name: gptr.Of("test"),
					},
					IsAsync: gptr.Of(true),
					Ext: map[string]string{
						"test": "test",
					},
				},
			},
			expected: &dto.EvalTargetVersion{
				ID:                  gptr.Of(int64(1)),
				WorkspaceID:         gptr.Of(int64(2)),
				TargetID:            gptr.Of(int64(3)),
				SourceTargetVersion: gptr.Of("v1.0"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := EvalTargetVersionDO2DTO(tt.targetVersionDO)
			if result == nil {
				assert.Equal(t, tt.expected, result)
				return
			}
			assert.Equal(t, tt.expected.TargetID, result.TargetID)
			assert.Equal(t, tt.expected.ID, result.ID)
			assert.Equal(t, tt.expected.SourceTargetVersion, result.SourceTargetVersion)
		})
	}
}
