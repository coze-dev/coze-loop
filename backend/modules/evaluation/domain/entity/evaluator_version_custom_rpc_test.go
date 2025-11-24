// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// TestCustomRPCEvaluatorVersion_ValidateInput 测试验证输入数据
func TestCustomRPCEvaluatorVersion_ValidateInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		evaluator   *CustomRPCEvaluatorVersion
		input       *EvaluatorInputData
		wantErr     bool
		errCode     int32
		description string
	}{
		{
			name: "成功 - 有效输入",
			evaluator: &CustomRPCEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{
						Key:                 gptr.Of("input1"),
						SupportContentTypes: []ContentType{ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "string"}`),
					},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"input1": {
						ContentType: gptr.Of(ContentTypeText),
						Text:        gptr.Of(`"test"`),
					},
				},
			},
			wantErr:     false,
			description: "有效输入应该通过验证",
		},
		{
			name: "成功 - 输入字段不在schema中（不验证）",
			evaluator: &CustomRPCEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{
						Key:                 gptr.Of("input1"),
						SupportContentTypes: []ContentType{ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "string"}`),
					},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"input2": {
						ContentType: gptr.Of(ContentTypeText),
						Text:        gptr.Of("test"),
					},
				},
			},
			wantErr:     false,
			description: "输入字段不在schema中应该通过验证",
		},
		{
			name: "成功 - 空输入字段（跳过）",
			evaluator: &CustomRPCEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{
						Key:                 gptr.Of("input1"),
						SupportContentTypes: []ContentType{ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "string"}`),
					},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"input1": nil,
				},
			},
			wantErr:     false,
			description: "空输入字段应该跳过验证",
		},
		{
			name: "失败 - 不支持的内容类型",
			evaluator: &CustomRPCEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{
						Key:                 gptr.Of("input1"),
						SupportContentTypes: []ContentType{ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "string"}`),
					},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"input1": {
						ContentType: gptr.Of(ContentTypeImage),
						Text:        gptr.Of("test"),
					},
				},
			},
			wantErr:     true,
			errCode:     errno.ContentTypeNotSupportedCode,
			description: "不支持的内容类型应该返回错误",
		},
		{
			name: "失败 - JSON Schema验证失败",
			evaluator: &CustomRPCEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{
						Key:                 gptr.Of("input1"),
						SupportContentTypes: []ContentType{ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "number"}`),
					},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"input1": {
						ContentType: gptr.Of(ContentTypeText),
						Text:        gptr.Of(`"not a number"`),
					},
				},
			},
			wantErr:     true,
			errCode:     errno.ContentSchemaInvalidCode,
			description: "JSON Schema验证失败应该返回错误",
		},
		{
			name: "成功 - 无InputSchemas",
			evaluator: &CustomRPCEvaluatorVersion{
				InputSchemas: []*ArgsSchema{},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"input1": {
						ContentType: gptr.Of(ContentTypeText),
						Text:        gptr.Of("test"),
					},
				},
			},
			wantErr:     false,
			description: "无InputSchemas时应该通过验证",
		},
		{
			name: "成功 - 多个输入字段",
			evaluator: &CustomRPCEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{
						Key:                 gptr.Of("input1"),
						SupportContentTypes: []ContentType{ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "string"}`),
					},
					{
						Key:                 gptr.Of("input2"),
						SupportContentTypes: []ContentType{ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "number"}`),
					},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"input1": {
						ContentType: gptr.Of(ContentTypeText),
						Text:        gptr.Of(`"test"`),
					},
					"input2": {
						ContentType: gptr.Of(ContentTypeText),
						Text:        gptr.Of("123"),
					},
				},
			},
			wantErr:     false,
			description: "多个有效输入字段应该通过验证",
		},
		{
			name: "成功 - 非Text类型（跳过JSON验证）",
			evaluator: &CustomRPCEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{
						Key:                 gptr.Of("input1"),
						SupportContentTypes: []ContentType{ContentTypeImage},
						JsonSchema:          gptr.Of(`{"type": "string"}`),
					},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"input1": {
						ContentType: gptr.Of(ContentTypeImage),
					},
				},
			},
			wantErr:     false,
			description: "非Text类型应该跳过JSON Schema验证",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.evaluator.ValidateInput(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.errCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCustomRPCEvaluatorVersion_ValidateBaseInfo 测试验证基础信息
func TestCustomRPCEvaluatorVersion_ValidateBaseInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		evaluator   *CustomRPCEvaluatorVersion
		wantErr     bool
		errCode     int32
		description string
	}{
		{
			name: "成功 - 有效的基础信息",
			evaluator: &CustomRPCEvaluatorVersion{
				ProviderEvaluatorCode: gptr.Of("PROVIDER_001"),
				AccessProtocol:        EvaluatorAccessProtocol("rpc"),
				ServiceName:           gptr.Of("test_service"),
			},
			wantErr:     false,
			description: "有效的基础信息应该通过验证",
		},
		{
			name:        "失败 - nil evaluator",
			evaluator:   nil,
			wantErr:     true,
			errCode:     errno.EvaluatorNotExistCode,
			description: "nil evaluator应该返回错误",
		},
		{
			name: "失败 - 空的ProviderEvaluatorCode",
			evaluator: &CustomRPCEvaluatorVersion{
				ProviderEvaluatorCode: gptr.Of(""),
				AccessProtocol:        EvaluatorAccessProtocol("rpc"),
				ServiceName:           gptr.Of("test_service"),
			},
			wantErr:     true,
			errCode:     errno.InvalidProviderEvaluatorCodeCode,
			description: "空的ProviderEvaluatorCode应该返回错误",
		},
		{
			name: "失败 - nil ProviderEvaluatorCode",
			evaluator: &CustomRPCEvaluatorVersion{
				ProviderEvaluatorCode: nil,
				AccessProtocol:        EvaluatorAccessProtocol("rpc"),
				ServiceName:           gptr.Of("test_service"),
			},
			wantErr:     true,
			errCode:     errno.InvalidProviderEvaluatorCodeCode,
			description: "nil ProviderEvaluatorCode应该返回错误",
		},
		{
			name: "失败 - 空的AccessProtocol",
			evaluator: &CustomRPCEvaluatorVersion{
				ProviderEvaluatorCode: gptr.Of("PROVIDER_001"),
				AccessProtocol:        EvaluatorAccessProtocol(""),
				ServiceName:           gptr.Of("test_service"),
			},
			wantErr:     true,
			errCode:     errno.InvalidAccessProtocolCode,
			description: "空的AccessProtocol应该返回错误",
		},
		{
			name: "失败 - 空的ServiceName",
			evaluator: &CustomRPCEvaluatorVersion{
				ProviderEvaluatorCode: gptr.Of("PROVIDER_001"),
				AccessProtocol:        EvaluatorAccessProtocol("rpc"),
				ServiceName:           gptr.Of(""),
			},
			wantErr:     true,
			errCode:     errno.InvalidServiceNameCode,
			description: "空的ServiceName应该返回错误",
		},
		{
			name: "失败 - nil ServiceName",
			evaluator: &CustomRPCEvaluatorVersion{
				ProviderEvaluatorCode: gptr.Of("PROVIDER_001"),
				AccessProtocol:        EvaluatorAccessProtocol("rpc"),
				ServiceName:           nil,
			},
			wantErr:     true,
			errCode:     errno.InvalidServiceNameCode,
			description: "nil ServiceName应该返回错误",
		},
		{
			name: "成功 - 所有可选字段都有值",
			evaluator: &CustomRPCEvaluatorVersion{
				ProviderEvaluatorCode: gptr.Of("PROVIDER_001"),
				AccessProtocol:        EvaluatorAccessProtocol("rpc"),
				ServiceName:           gptr.Of("test_service"),
				Cluster:               gptr.Of("test_cluster"),
				Timeout:               gptr.Of(int64(5000)),
			},
			wantErr:     false,
			description: "所有字段都有值应该通过验证",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.evaluator.ValidateBaseInfo()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.errCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCustomRPCEvaluatorVersion_GettersAndSetters 测试getter和setter方法
func TestCustomRPCEvaluatorVersion_GettersAndSetters(t *testing.T) {
	t.Parallel()

	evaluator := &CustomRPCEvaluatorVersion{}

	// 测试 ID
	evaluator.SetID(123)
	assert.Equal(t, int64(123), evaluator.GetID())

	// 测试 EvaluatorID
	evaluator.SetEvaluatorID(456)
	assert.Equal(t, int64(456), evaluator.GetEvaluatorID())

	// 测试 SpaceID
	evaluator.SetSpaceID(789)
	assert.Equal(t, int64(789), evaluator.GetSpaceID())

	// 测试 Version
	evaluator.SetVersion("1.0.0")
	assert.Equal(t, "1.0.0", evaluator.GetVersion())

	// 测试 Description
	evaluator.SetDescription("Test description")
	assert.Equal(t, "Test description", evaluator.GetDescription())

	// 测试 BaseInfo
	baseInfo := &BaseInfo{
		CreatedBy: &UserInfo{
			UserID: gptr.Of("user1"),
		},
	}
	evaluator.SetBaseInfo(baseInfo)
	assert.Equal(t, baseInfo, evaluator.GetBaseInfo())
}
