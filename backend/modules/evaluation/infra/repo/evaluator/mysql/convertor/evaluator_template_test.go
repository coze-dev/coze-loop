// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// TestConvertEvaluatorTemplateDO2PO 测试将 DO 对象转换为 PO 对象
func TestConvertEvaluatorTemplateDO2PO(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		do          *evaluatordo.EvaluatorTemplate
		wantErr     bool
		validate    func(t *testing.T, po *model.EvaluatorTemplate, err error)
		description string
	}{
		{
			name:    "nil输入",
			do:      nil,
			wantErr: false,
			validate: func(t *testing.T, po *model.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.Nil(t, po)
			},
			description: "nil输入应该返回nil",
		},
		{
			name: "成功 - Prompt类型模板（基础字段）",
			do: &evaluatordo.EvaluatorTemplate{
				ID:                 1,
				SpaceID:            123,
				Name:               "Test Template",
				Description:        "Test Description",
				EvaluatorType:      evaluatordo.EvaluatorTypePrompt,
				ReceiveChatHistory: gptr.Of(true),
				Popularity:         100,
				EvaluatorInfo:      &evaluatordo.EvaluatorInfo{Benchmark: "benchmark1", Vendor: "vendor1", VendorURL: "u1", UserManualURL: "m1"},
				BaseInfo: &evaluatordo.BaseInfo{
					CreatedBy: &evaluatordo.UserInfo{
						UserID: ptr.Of("user1"),
					},
					UpdatedBy: &evaluatordo.UserInfo{
						UserID: ptr.Of("user1"),
					},
					CreatedAt: ptr.Of(baseTime.UnixMilli()),
					UpdatedAt: ptr.Of(baseTime.UnixMilli()),
				},
			},
			wantErr: false,
			validate: func(t *testing.T, po *model.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, po)
				assert.Equal(t, int64(1), po.ID)
				assert.Equal(t, int64(123), gptr.Indirect(po.SpaceID))
				assert.Equal(t, "Test Template", gptr.Indirect(po.Name))
				assert.Equal(t, "Test Description", gptr.Indirect(po.Description))
				assert.Equal(t, int32(evaluatordo.EvaluatorTypePrompt), gptr.Indirect(po.EvaluatorType))
				assert.NotNil(t, po.ReceiveChatHistory)
				assert.True(t, gptr.Indirect(po.ReceiveChatHistory))
				assert.Equal(t, int64(100), po.Popularity)
				if assert.NotNil(t, po.EvaluatorInfo) {
					var info evaluatordo.EvaluatorInfo
					_ = json.Unmarshal(*po.EvaluatorInfo, &info)
					assert.Equal(t, "benchmark1", info.Benchmark)
					assert.Equal(t, "vendor1", info.Vendor)
					assert.Equal(t, "u1", info.VendorURL)
					assert.Equal(t, "m1", info.UserManualURL)
				}
				assert.Equal(t, "user1", po.CreatedBy)
				assert.Equal(t, "user1", po.UpdatedBy)
				assert.Equal(t, baseTime.UnixMilli(), po.CreatedAt.UnixMilli())
				assert.Equal(t, baseTime.UnixMilli(), po.UpdatedAt.UnixMilli())
			},
			description: "成功转换Prompt类型模板（基础字段）",
		},
		{
			name: "成功 - Code类型模板",
			do: &evaluatordo.EvaluatorTemplate{
				ID:            2,
				SpaceID:       123,
				Name:          "Code Template",
				EvaluatorType: evaluatordo.EvaluatorTypeCode,
				CodeEvaluatorContent: &evaluatordo.CodeEvaluatorContent{
					Lang2CodeContent: map[evaluatordo.LanguageType]string{
						evaluatordo.LanguageTypePython: "def evaluate(): pass",
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, po *model.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, po)
				assert.Equal(t, int32(evaluatordo.EvaluatorTypeCode), gptr.Indirect(po.EvaluatorType))
				assert.NotNil(t, po.Metainfo)
			},
			description: "成功转换Code类型模板",
		},
		{
			name: "成功 - 带InputSchemas和OutputSchemas",
			do: &evaluatordo.EvaluatorTemplate{
				ID:            3,
				SpaceID:       123,
				Name:          "Template with Schemas",
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				InputSchemas: []*evaluatordo.ArgsSchema{
					{
						Key:                 ptr.Of("input1"),
						SupportContentTypes: []evaluatordo.ContentType{evaluatordo.ContentTypeText},
						JsonSchema:          ptr.Of(`{"type": "string"}`),
					},
				},
				OutputSchemas: []*evaluatordo.ArgsSchema{
					{
						Key:                 ptr.Of("output1"),
						SupportContentTypes: []evaluatordo.ContentType{evaluatordo.ContentTypeText},
						JsonSchema:          ptr.Of(`{"type": "string"}`),
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, po *model.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, po)
				assert.NotNil(t, po.InputSchema)
				assert.NotNil(t, po.OutputSchema)
			},
			description: "成功转换带Schemas的模板",
		},
		{
			name: "成功 - 带删除时间",
			do: &evaluatordo.EvaluatorTemplate{
				ID:            4,
				SpaceID:       123,
				Name:          "Deleted Template",
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				BaseInfo: &evaluatordo.BaseInfo{
					DeletedAt: ptr.Of(baseTime.UnixMilli()),
				},
			},
			wantErr: false,
			validate: func(t *testing.T, po *model.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, po)
				assert.True(t, po.DeletedAt.Valid)
				assert.Equal(t, baseTime.UnixMilli(), po.DeletedAt.Time.UnixMilli())
			},
			description: "成功转换带删除时间的模板",
		},
		{
			name: "成功 - Prompt类型带PromptEvaluatorContent",
			do: &evaluatordo.EvaluatorTemplate{
				ID:            5,
				SpaceID:       123,
				Name:          "Prompt Template with Content",
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				PromptEvaluatorContent: &evaluatordo.PromptEvaluatorContent{
					MessageList: []*evaluatordo.Message{
						{
							Content: &evaluatordo.Content{
								Text: gptr.Of("test message"),
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, po *model.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, po)
				assert.NotNil(t, po.Metainfo)
			},
			description: "成功转换带PromptEvaluatorContent的模板",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			po, err := ConvertEvaluatorTemplateDO2PO(tt.do)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				tt.validate(t, po, err)
			}
		})
	}
}

// TestConvertEvaluatorTemplatePO2DO 测试将 PO 对象转换为 DO 对象
func TestConvertEvaluatorTemplatePO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		po          *model.EvaluatorTemplate
		wantErr     bool
		validate    func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error)
		description string
	}{
		{
			name:    "nil输入",
			po:      nil,
			wantErr: false,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.Nil(t, do)
			},
			description: "nil输入应该返回nil",
		},
		{
			name: "成功 - Prompt类型模板",
			po: &model.EvaluatorTemplate{
				ID:                 1,
				SpaceID:            gptr.Of(int64(123)),
				Name:               gptr.Of("Test Template"),
				Description:        gptr.Of("Test Description"),
				EvaluatorType:      gptr.Of(int32(evaluatordo.EvaluatorTypePrompt)),
				ReceiveChatHistory: gptr.Of(true),
				Popularity:         100,
				EvaluatorInfo:      gptr.Of([]byte(`{"benchmark":"benchmark1","vendor":"vendor1","vendor_url":"u1","user_manual_url":"m1"}`)),
			},
			wantErr: false,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, do)
				assert.Equal(t, int64(1), do.ID)
				assert.Equal(t, int64(123), do.SpaceID)
				assert.Equal(t, "Test Template", do.Name)
				assert.Equal(t, "Test Description", do.Description)
				assert.Equal(t, evaluatordo.EvaluatorTypePrompt, do.EvaluatorType)
				assert.NotNil(t, do.ReceiveChatHistory)
				assert.True(t, gptr.Indirect(do.ReceiveChatHistory))
				assert.Equal(t, int64(100), do.Popularity)
				if assert.NotNil(t, do.EvaluatorInfo) {
					assert.Equal(t, "benchmark1", do.EvaluatorInfo.Benchmark)
					assert.Equal(t, "vendor1", do.EvaluatorInfo.Vendor)
					assert.Equal(t, "u1", do.EvaluatorInfo.VendorURL)
					assert.Equal(t, "m1", do.EvaluatorInfo.UserManualURL)
				}
				assert.NotNil(t, do.Tags)
			},
			description: "成功转换Prompt类型模板",
		},
		{
			name: "成功 - Code类型模板",
			po: &model.EvaluatorTemplate{
				ID:            2,
				SpaceID:       gptr.Of(int64(123)),
				Name:          gptr.Of("Code Template"),
				EvaluatorType: gptr.Of(int32(evaluatordo.EvaluatorTypeCode)),
				Metainfo:      gptr.Of([]byte(`{"code_content":"def evaluate(): pass"}`)),
			},
			wantErr: false,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, do)
				assert.Equal(t, evaluatordo.EvaluatorTypeCode, do.EvaluatorType)
				assert.NotNil(t, do.CodeEvaluatorContent)
			},
			description: "成功转换Code类型模板",
		},
		{
			name: "成功 - 带InputSchema和OutputSchema",
			po: &model.EvaluatorTemplate{
				ID:            3,
				SpaceID:       gptr.Of(int64(123)),
				Name:          gptr.Of("Template with Schemas"),
				EvaluatorType: gptr.Of(int32(evaluatordo.EvaluatorTypePrompt)),
				InputSchema:   gptr.Of([]byte(`[{"key":"input1","support_content_types":["Text"],"json_schema":"{\"type\": \"string\"}"}]`)),
				OutputSchema:  gptr.Of([]byte(`[{"key":"output1","support_content_types":["Text"],"json_schema":"{\"type\": \"string\"}"}]`)),
			},
			wantErr: false,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, do)
				assert.NotNil(t, do.InputSchemas)
				assert.Len(t, do.InputSchemas, 1)
				assert.NotNil(t, do.OutputSchemas)
				assert.Len(t, do.OutputSchemas, 1)
			},
			description: "成功转换带Schemas的模板",
		},
		{
			name: "失败 - 无效的Metainfo JSON",
			po: &model.EvaluatorTemplate{
				ID:            4,
				SpaceID:       gptr.Of(int64(123)),
				Name:          gptr.Of("Invalid Template"),
				EvaluatorType: gptr.Of(int32(evaluatordo.EvaluatorTypePrompt)),
				Metainfo:      gptr.Of([]byte(`invalid json`)),
			},
			wantErr: true,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.Error(t, err)
			},
			description: "无效的Metainfo JSON应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			do, err := ConvertEvaluatorTemplatePO2DO(tt.po)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				tt.validate(t, do, err)
			}
		})
	}
}

// TestConvertEvaluatorTemplatePO2DOWithBaseInfo 测试将 PO 对象转换为 DO 对象（包含基础信息）
func TestConvertEvaluatorTemplatePO2DOWithBaseInfo(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		po          *model.EvaluatorTemplate
		wantErr     bool
		validate    func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error)
		description string
	}{
		{
			name:    "nil输入",
			po:      nil,
			wantErr: false,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.Nil(t, do)
			},
			description: "nil输入应该返回nil",
		},
		{
			name: "成功 - 带基础信息",
			po: &model.EvaluatorTemplate{
				ID:            1,
				SpaceID:       gptr.Of(int64(123)),
				Name:          gptr.Of("Test Template"),
				EvaluatorType: gptr.Of(int32(evaluatordo.EvaluatorTypePrompt)),
				CreatedBy:     "user1",
				UpdatedBy:     "user1",
				CreatedAt:     baseTime,
				UpdatedAt:     baseTime,
			},
			wantErr: false,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, do)
				assert.NotNil(t, do.GetBaseInfo())
				assert.NotNil(t, do.GetBaseInfo().CreatedBy)
				assert.Equal(t, "user1", gptr.Indirect(do.GetBaseInfo().CreatedBy.UserID))
				assert.NotNil(t, do.GetBaseInfo().UpdatedBy)
				assert.Equal(t, "user1", gptr.Indirect(do.GetBaseInfo().UpdatedBy.UserID))
				assert.Equal(t, baseTime.UnixMilli(), gptr.Indirect(do.GetBaseInfo().CreatedAt))
				assert.Equal(t, baseTime.UnixMilli(), gptr.Indirect(do.GetBaseInfo().UpdatedAt))
			},
			description: "成功转换带基础信息的模板",
		},
		{
			name: "成功 - 带删除时间",
			po: &model.EvaluatorTemplate{
				ID:            2,
				SpaceID:       gptr.Of(int64(123)),
				Name:          gptr.Of("Deleted Template"),
				EvaluatorType: gptr.Of(int32(evaluatordo.EvaluatorTypePrompt)),
				CreatedBy:     "user1",
				UpdatedBy:     "user1",
				CreatedAt:     baseTime,
				UpdatedAt:     baseTime,
				DeletedAt: gorm.DeletedAt{
					Time:  baseTime,
					Valid: true,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, do)
				assert.NotNil(t, do.GetBaseInfo())
				assert.NotNil(t, do.GetBaseInfo().DeletedAt)
				assert.Equal(t, baseTime.UnixMilli(), gptr.Indirect(do.GetBaseInfo().DeletedAt))
			},
			description: "成功转换带删除时间的模板",
		},
		{
			name: "成功 - 无删除时间",
			po: &model.EvaluatorTemplate{
				ID:            3,
				SpaceID:       gptr.Of(int64(123)),
				Name:          gptr.Of("Active Template"),
				EvaluatorType: gptr.Of(int32(evaluatordo.EvaluatorTypePrompt)),
				CreatedBy:     "user1",
				UpdatedBy:     "user1",
				CreatedAt:     baseTime,
				UpdatedAt:     baseTime,
				DeletedAt: gorm.DeletedAt{
					Valid: false,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, do *evaluatordo.EvaluatorTemplate, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, do)
				assert.NotNil(t, do.GetBaseInfo())
				assert.Nil(t, do.GetBaseInfo().DeletedAt)
			},
			description: "成功转换无删除时间的模板",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			do, err := ConvertEvaluatorTemplatePO2DOWithBaseInfo(tt.po)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				tt.validate(t, do, err)
			}
		})
	}
}
