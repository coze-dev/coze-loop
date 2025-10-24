// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestDebugContextDO2PO(t *testing.T) {
	tests := []struct {
		name     string
		do       *entity.DebugContext
		expected *model.PromptDebugContext
		wantErr  bool
	}{
		{
			name:     "nil input",
			do:       nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name: "empty debug context",
			do: &entity.DebugContext{
				PromptID: 1,
				UserID:   "test_user",
			},
			expected: &model.PromptDebugContext{
				PromptID: 1,
				UserID:   "test_user",
			},
			wantErr: false,
		},
		{
			name: "debug context with debug core",
			do: &entity.DebugContext{
				PromptID: 1,
				UserID:   "test_user",
				DebugCore: &entity.DebugCore{
					MockContexts: []*entity.DebugMessage{
						{
							Role:    entity.RoleSystem,
							Content: ptr.Of("test content"),
						},
					},
					MockVariables: []*entity.VariableVal{
						{
							Key:   "test_key",
							Value: ptr.Of("test_value"),
						},
					},
					MockTools: []*entity.MockTool{
						{
							Name:         "test_tool",
							MockResponse: "test_response",
						},
					},
				},
			},
			expected: &model.PromptDebugContext{
				PromptID:      1,
				UserID:        "test_user",
				MockContexts:  ptr.Of(`[{"role":"system","content":"test content"}]`),
				MockVariables: ptr.Of(`[{"key":"test_key","value":"test_value"}]`),
				MockTools:     ptr.Of(`[{"name":"test_tool","mock_response":"test_response"}]`),
			},
			wantErr: false,
		},
		{
			name: "debug context with debug config",
			do: &entity.DebugContext{
				PromptID: 1,
				UserID:   "test_user",
				DebugConfig: &entity.DebugConfig{
					SingleStepDebug: ptr.Of(true),
				},
			},
			expected: &model.PromptDebugContext{
				PromptID:    1,
				UserID:      "test_user",
				DebugConfig: ptr.Of(`{"single_step_debug":true}`),
			},
			wantErr: false,
		},
		{
			name: "debug context with compare config",
			do: &entity.DebugContext{
				PromptID: 1,
				UserID:   "test_user",
				CompareConfig: &entity.CompareConfig{
					Groups: []*entity.CompareGroup{
						{
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{
									TemplateType: entity.TemplateTypeNormal,
									HasSnippets:  false,
									Messages: []*entity.Message{
										{
											Role:    entity.RoleSystem,
											Content: ptr.Of("test content"),
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &model.PromptDebugContext{
				PromptID:      1,
				UserID:        "test_user",
				CompareConfig: ptr.Of(`{"groups":[{"prompt_detail":{"prompt_template":{"template_type":"normal","messages":[{"role":"system","content":"test content"}],"has_snippets":false}}}]}`),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DebugContextDO2PO(tt.do)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestDebugContextPO2DO(t *testing.T) {
	tests := []struct {
		name     string
		po       *model.PromptDebugContext
		expected *entity.DebugContext
		wantErr  bool
	}{
		{
			name:     "nil input",
			po:       nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name: "empty debug context",
			po: &model.PromptDebugContext{
				PromptID: 1,
				UserID:   "test_user",
			},
			expected: &entity.DebugContext{
				PromptID:  1,
				UserID:    "test_user",
				DebugCore: &entity.DebugCore{},
			},
			wantErr: false,
		},
		{
			name: "debug context with mock contexts",
			po: &model.PromptDebugContext{
				PromptID:     1,
				UserID:       "test_user",
				MockContexts: ptr.Of(`[{"role":"system","content":"test content"}]`),
			},
			expected: &entity.DebugContext{
				PromptID: 1,
				UserID:   "test_user",
				DebugCore: &entity.DebugCore{
					MockContexts: []*entity.DebugMessage{
						{
							Role:    entity.RoleSystem,
							Content: ptr.Of("test content"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "debug context with mock variables",
			po: &model.PromptDebugContext{
				PromptID:      1,
				UserID:        "test_user",
				MockVariables: ptr.Of(`[{"key":"test_key","value":"test_value"}]`),
			},
			expected: &entity.DebugContext{
				PromptID: 1,
				UserID:   "test_user",
				DebugCore: &entity.DebugCore{
					MockVariables: []*entity.VariableVal{
						{
							Key:   "test_key",
							Value: ptr.Of("test_value"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "debug context with mock tools",
			po: &model.PromptDebugContext{
				PromptID:  1,
				UserID:    "test_user",
				MockTools: ptr.Of(`[{"name":"test_tool","mock_response":"test_response"}]`),
			},
			expected: &entity.DebugContext{
				PromptID: 1,
				UserID:   "test_user",
				DebugCore: &entity.DebugCore{
					MockTools: []*entity.MockTool{
						{
							Name:         "test_tool",
							MockResponse: "test_response",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "debug context with debug config",
			po: &model.PromptDebugContext{
				PromptID:    1,
				UserID:      "test_user",
				DebugConfig: ptr.Of(`{"single_step_debug":true}`),
			},
			expected: &entity.DebugContext{
				PromptID:  1,
				UserID:    "test_user",
				DebugCore: &entity.DebugCore{},
				DebugConfig: &entity.DebugConfig{
					SingleStepDebug: ptr.Of(true),
				},
			},
			wantErr: false,
		},
		{
			name: "debug context with compare config",
			po: &model.PromptDebugContext{
				PromptID:      1,
				UserID:        "test_user",
				CompareConfig: ptr.Of(`{"groups":[{"prompt_detail":{"prompt_template":{"template_type":"normal","messages":[{"role":"system","content":"test content"}],"has_snippets":false}}}]}`),
			},
			expected: &entity.DebugContext{
				PromptID:  1,
				UserID:    "test_user",
				DebugCore: &entity.DebugCore{},
				CompareConfig: &entity.CompareConfig{
					Groups: []*entity.CompareGroup{
						{
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{
									TemplateType: entity.TemplateTypeNormal,
									HasSnippets:  false,
									Messages: []*entity.Message{
										{
											Role:    entity.RoleSystem,
											Content: ptr.Of("test content"),
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid json in mock contexts",
			po: &model.PromptDebugContext{
				PromptID:     1,
				UserID:       "test_user",
				MockContexts: ptr.Of(`invalid json`),
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "invalid json in mock variables",
			po: &model.PromptDebugContext{
				PromptID:      1,
				UserID:        "test_user",
				MockVariables: ptr.Of(`invalid json`),
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "invalid json in mock tools",
			po: &model.PromptDebugContext{
				PromptID:  1,
				UserID:    "test_user",
				MockTools: ptr.Of(`invalid json`),
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "invalid json in debug config",
			po: &model.PromptDebugContext{
				PromptID:    1,
				UserID:      "test_user",
				DebugConfig: ptr.Of(`invalid json`),
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "invalid json in compare config",
			po: &model.PromptDebugContext{
				PromptID:      1,
				UserID:        "test_user",
				CompareConfig: ptr.Of(`invalid json`),
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DebugContextPO2DO(tt.po)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
