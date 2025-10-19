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

func TestCustomRPCServerConversions(t *testing.T) {
	t.Parallel()

	trueVal := true
	timeout := int64(1000)
	asyncTimeout := int64(2000)
	execEnv := "prod"
	doValue := &do.CustomRPCServer{
		ID:                  123,
		Name:                "custom",
		Description:         "desc",
		ServerName:          "svc",
		AccessProtocol:      do.AccessProtocolFaasHTTP,
		Regions:             []do.Region{"cn"},
		Cluster:             "default",
		InvokeHTTPInfo:      &do.HTTPInfo{Method: do.HTTPMethodPost, Path: "/invoke"},
		AsyncInvokeHTTPInfo: &do.HTTPInfo{Method: do.HTTPMethodGet, Path: "/async"},
		NeedSearchTarget:    &trueVal,
		SearchHTTPInfo:      &do.HTTPInfo{Method: do.HTTPMethodGet, Path: "/search"},
		CustomEvalTarget:    &do.CustomEvalTarget{ID: gptr.Of("id"), Name: gptr.Of("target"), AvatarURL: gptr.Of("avatar"), Ext: map[string]string{"k": "v"}},
		IsAsync:             &trueVal,
		ExecRegion:          do.RegionCN,
		ExecEnv:             &execEnv,
		Timeout:             &timeout,
		AsyncTimeout:        &asyncTimeout,
		Ext:                 map[string]string{"extra": "value"},
	}

	dtoValue := CustomRPCServerDO2DTO(doValue)
	assert.NotNil(t, dtoValue)
	assert.Equal(t, doValue.ID, gptr.Indirect(dtoValue.ID))
	assert.Equal(t, doValue.Name, gptr.Indirect(dtoValue.Name))
	assert.Equal(t, doValue.Description, gptr.Indirect(dtoValue.Description))
	assert.Equal(t, doValue.ServerName, gptr.Indirect(dtoValue.ServerName))
	assert.Equal(t, doValue.AccessProtocol, gptr.Indirect(dtoValue.AccessProtocol))
	assert.Equal(t, []string(doValue.Regions), dtoValue.Regions)
	assert.Equal(t, doValue.Cluster, gptr.Indirect(dtoValue.Cluster))
	assert.Equal(t, doValue.InvokeHTTPInfo.Path, gptr.Indirect(dtoValue.InvokeHTTPInfo.Path))
	assert.Equal(t, doValue.AsyncInvokeHTTPInfo.Method, gptr.Indirect(dtoValue.AsyncInvokeHTTPInfo.Method))
	assert.Equal(t, doValue.NeedSearchTarget, dtoValue.NeedSearchTarget)
	assert.Equal(t, doValue.SearchHTTPInfo.Path, gptr.Indirect(dtoValue.SearchHTTPInfo.Path))
	assert.Equal(t, doValue.CustomEvalTarget.Name, dtoValue.CustomEvalTarget.Name)
	assert.Equal(t, doValue.IsAsync, dtoValue.IsAsync)
	assert.Equal(t, do.Region(gptr.Indirect(dtoValue.ExecRegion)), doValue.ExecRegion)
	assert.Equal(t, doValue.ExecEnv, dtoValue.ExecEnv)
	assert.Equal(t, doValue.Timeout, dtoValue.Timeout)
	assert.Equal(t, doValue.AsyncTimeout, dtoValue.AsyncTimeout)
	assert.Equal(t, doValue.Ext, dtoValue.Ext)

	roundtrip := CustomRPCServerDTO2DO(dtoValue)
	assert.Equal(t, doValue.ID, roundtrip.ID)
	assert.Equal(t, doValue.Name, roundtrip.Name)
	assert.Equal(t, doValue.Description, roundtrip.Description)
	assert.Equal(t, doValue.ServerName, roundtrip.ServerName)
	assert.Equal(t, doValue.AccessProtocol, roundtrip.AccessProtocol)
	assert.Equal(t, doValue.Regions, roundtrip.Regions)
	assert.Equal(t, doValue.Cluster, roundtrip.Cluster)
	assert.Equal(t, doValue.InvokeHTTPInfo.Method, roundtrip.InvokeHTTPInfo.Method)
	assert.Equal(t, doValue.AsyncInvokeHTTPInfo.Path, roundtrip.AsyncInvokeHTTPInfo.Path)
	assert.Equal(t, doValue.NeedSearchTarget, roundtrip.NeedSearchTarget)
	assert.Equal(t, doValue.SearchHTTPInfo.Method, roundtrip.SearchHTTPInfo.Method)
	assert.Equal(t, doValue.CustomEvalTarget.Ext, roundtrip.CustomEvalTarget.Ext)
	assert.Equal(t, doValue.IsAsync, roundtrip.IsAsync)
	assert.Equal(t, doValue.ExecRegion, roundtrip.ExecRegion)
	assert.Equal(t, doValue.ExecEnv, roundtrip.ExecEnv)
	assert.Equal(t, doValue.Timeout, roundtrip.Timeout)
	assert.Equal(t, doValue.AsyncTimeout, roundtrip.AsyncTimeout)
	assert.Equal(t, doValue.Ext, roundtrip.Ext)

	assert.Nil(t, CustomRPCServerDTO2DO(nil))
}

func TestCustomEvalTargetConversions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dos  []*do.CustomEvalTarget
	}{
		{
			name: "包含nil元素",
			dos: []*do.CustomEvalTarget{
				{ID: gptr.Of("1"), Name: gptr.Of("a")},
				nil,
			},
		},
		{
			name: "nil输入",
			dos:  nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dtos := CustomEvalTargetDO2DTOs(tt.dos)
			if tt.dos == nil {
				assert.Nil(t, dtos)
				return
			}
			assert.Len(t, dtos, 1)
			assert.Equal(t, gptr.Indirect(tt.dos[0].ID), gptr.Indirect(dtos[0].ID))
		})
	}

	dtoValue := &dto.CustomEvalTarget{ID: gptr.Of("id"), Name: gptr.Of("name"), AvatarURL: gptr.Of("avatar")}
	doValue := CustomEvalTargetDTO2DO(dtoValue)
	assert.Equal(t, gptr.Indirect(dtoValue.ID), gptr.Indirect(doValue.ID))
	assert.Equal(t, gptr.Indirect(dtoValue.Name), gptr.Indirect(doValue.Name))
	assert.Equal(t, gptr.Indirect(dtoValue.AvatarURL), gptr.Indirect(doValue.AvatarURL))
	assert.Nil(t, CustomEvalTargetDTO2DO(nil))
	assert.Nil(t, CustomEvalTargetDO2DTO(nil))
}
