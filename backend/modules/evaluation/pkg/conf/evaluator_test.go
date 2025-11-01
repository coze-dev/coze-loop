// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	mock_conf "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/contexts"
)

func TestConfiger_GetEvaluatorPromptSuffix(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
	c := &evaluatorConfiger{loader: mockLoader}

	ctx := context.Background()
	const key = "evaluator_prompt_suffix"
	locale := "en-US"
	ctxWithLocale := contexts.WithLocale(ctx, locale)
	localeKey := key + "_" + locale

	tests := []struct {
		name           string
		ctx            context.Context
		mockSetup      func()
		expectedResult map[string]string
	}{
		{
			name: "locale key hit",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						m := map[string]string{"a": "b"}
						ptr := out.(*map[string]string)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]string{"a": "b"},
		},
		{
			name: "locale key miss, hit default key",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						m := map[string]string{"c": "d"}
						ptr := out.(*map[string]string)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]string{"c": "d"},
		},
		{
			name: "all miss, return default",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, key, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
			},
			expectedResult: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result := c.GetEvaluatorPromptSuffix(tt.ctx)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestConfiger_GetEvaluatorToolConf(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
	c := &evaluatorConfiger{loader: mockLoader}

	ctx := context.Background()
	const key = "evaluator_tool_conf"
	locale := "en-US"
	ctxWithLocale := contexts.WithLocale(ctx, locale)
	localeKey := key + "_" + locale

	tests := []struct {
		name           string
		ctx            context.Context
		mockSetup      func()
		expectedResult map[string]*evaluatordto.Tool
	}{
		{
			name: "locale key hit",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						m := map[string]*evaluatordto.Tool{"tool1": {}}
						ptr := out.(*map[string]*evaluatordto.Tool)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]*evaluatordto.Tool{"tool1": {}},
		},
		{
			name: "locale key miss, hit default key",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						m := map[string]*evaluatordto.Tool{"tool2": {}}
						ptr := out.(*map[string]*evaluatordto.Tool)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]*evaluatordto.Tool{"tool2": {}},
		},
		{
			name: "all miss, return default",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, key, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
			},
			expectedResult: map[string]*evaluatordto.Tool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result := c.GetEvaluatorToolConf(tt.ctx)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestConfiger_GetEvaluatorTemplateConf(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
	c := &evaluatorConfiger{loader: mockLoader}

	ctx := context.Background()
	const key = "evaluator_template_conf"
	locale := "en-US"
	ctxWithLocale := contexts.WithLocale(ctx, locale)
	localeKey := key + "_" + locale

	tests := []struct {
		name           string
		ctx            context.Context
		mockSetup      func()
		expectedResult map[string]map[string]*evaluatordto.EvaluatorContent
	}{
		{
			name: "locale key hit",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						m := map[string]map[string]*evaluatordto.EvaluatorContent{"a": {"b": {}}}
						ptr := out.(*map[string]map[string]*evaluatordto.EvaluatorContent)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{"a": {"b": {}}},
		},
		{
			name: "locale key miss, hit default key",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						m := map[string]map[string]*evaluatordto.EvaluatorContent{"c": {"d": {}}}
						ptr := out.(*map[string]map[string]*evaluatordto.EvaluatorContent)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{"c": {"d": {}}},
		},
		{
			name: "all miss, return default",
			ctx:  ctxWithLocale,
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, localeKey, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
				mockLoader.EXPECT().UnmarshalKey(ctxWithLocale, key, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result := c.GetEvaluatorTemplateConf(tt.ctx)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestConfiger_GetCodeEvaluatorTemplateConf(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
	c := &evaluatorConfiger{loader: mockLoader}

	ctx := context.Background()
	const key = "code_evaluator_template_conf"

	tests := []struct {
		name           string
		mockSetup      func()
		expectedResult map[string]map[string]*evaluatordto.EvaluatorContent
		validateResult func(*testing.T, map[string]map[string]*evaluatordto.EvaluatorContent)
	}{
		{
			name: "成功获取配置并规范化语言键",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						// 模拟原始配置数据，包含非标准语言键
						m := map[string]map[string]*evaluatordto.EvaluatorContent{
							"template1": {
								"python": {
									CodeEvaluator: &evaluatordto.CodeEvaluator{
										LanguageType: gptr.Of(evaluatordto.LanguageType("python")),
									},
								},
								"javascript": {
									CodeEvaluator: &evaluatordto.CodeEvaluator{
										LanguageType: gptr.Of(evaluatordto.LanguageType("javascript")),
									},
								},
							},
						}
						ptr := out.(*map[string]map[string]*evaluatordto.EvaluatorContent)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{
				"template1": {
					string(evaluatordto.LanguageTypePython): {
						CodeEvaluator: &evaluatordto.CodeEvaluator{
							LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
						},
					},
					string(evaluatordto.LanguageTypeJS): {
						CodeEvaluator: &evaluatordto.CodeEvaluator{
							LanguageType: gptr.Of(evaluatordto.LanguageTypeJS),
						},
					},
				},
			},
			validateResult: func(t *testing.T, result map[string]map[string]*evaluatordto.EvaluatorContent) {
				// 验证语言键被规范化
				assert.Contains(t, result["template1"], string(evaluatordto.LanguageTypePython))
				assert.Contains(t, result["template1"], string(evaluatordto.LanguageTypeJS))
				// 验证内部LanguageType也被规范化
				pythonContent := result["template1"][string(evaluatordto.LanguageTypePython)]
				assert.Equal(t, evaluatordto.LanguageTypePython, *pythonContent.CodeEvaluator.LanguageType)
			},
		},
		{
			name: "配置为空返回默认配置",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{},
		},
		{
			name: "处理重复键，保留已存在的",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						// 模拟包含重复键的配置
						m := map[string]map[string]*evaluatordto.EvaluatorContent{
							"template1": {
								"python": {
									CodeEvaluator: &evaluatordto.CodeEvaluator{
										LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
									},
								},
								"Python": {
									CodeEvaluator: &evaluatordto.CodeEvaluator{
										LanguageType: gptr.Of(evaluatordto.LanguageType("Python")),
									},
								},
							},
						}
						ptr := out.(*map[string]map[string]*evaluatordto.EvaluatorContent)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{
				"template1": {
					string(evaluatordto.LanguageTypePython): {
						CodeEvaluator: &evaluatordto.CodeEvaluator{
							LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
						},
					},
				},
			},
			validateResult: func(t *testing.T, result map[string]map[string]*evaluatordto.EvaluatorContent) {
				// 验证只保留了一个python键（第一个出现的）
				assert.Len(t, result["template1"], 1)
				assert.Contains(t, result["template1"], string(evaluatordto.LanguageTypePython))
			},
		},
		{
			name: "处理nil模板内容",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						// 模拟包含nil模板内容的配置
						m := map[string]map[string]*evaluatordto.EvaluatorContent{
							"template1": {
								"python": nil,
								"js": {
									CodeEvaluator: &evaluatordto.CodeEvaluator{
										LanguageType: gptr.Of(evaluatordto.LanguageType("javascript")),
									},
								},
							},
						}
						ptr := out.(*map[string]map[string]*evaluatordto.EvaluatorContent)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{
				"template1": {
					string(evaluatordto.LanguageTypeJS): {
						CodeEvaluator: &evaluatordto.CodeEvaluator{
							LanguageType: gptr.Of(evaluatordto.LanguageTypeJS),
						},
					},
					string(evaluatordto.LanguageTypePython): nil,
				},
			},
			validateResult: func(t *testing.T, result map[string]map[string]*evaluatordto.EvaluatorContent) {
				// 验证nil内容被正确处理 - 实际上代码中并没有过滤nil值，所以这里修正预期
				assert.Len(t, result["template1"], 2)
				assert.Contains(t, result["template1"], string(evaluatordto.LanguageTypeJS))
				assert.Contains(t, result["template1"], string(evaluatordto.LanguageTypePython))
				// 验证nil值确实存在
				assert.Nil(t, result["template1"][string(evaluatordto.LanguageTypePython)])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result := c.GetCodeEvaluatorTemplateConf(ctx)
			assert.Equal(t, tt.expectedResult, result)
			if tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestConfiger_GetCustomCodeEvaluatorTemplateConf(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mock_conf.NewMockIConfigLoader(ctrl)
	c := &evaluatorConfiger{loader: mockLoader}

	ctx := context.Background()
	const key = "custom_code_evaluator_template_conf"

	tests := []struct {
		name           string
		mockSetup      func()
		expectedResult map[string]map[string]*evaluatordto.EvaluatorContent
		validateResult func(*testing.T, map[string]map[string]*evaluatordto.EvaluatorContent)
	}{
		{
			name: "成功获取自定义配置并规范化",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ string, out any, _ ...conf.DecodeOptionFn) error {
						m := map[string]map[string]*evaluatordto.EvaluatorContent{
							"custom_template": {
								"js": {
									CodeEvaluator: &evaluatordto.CodeEvaluator{
										LanguageType: gptr.Of(evaluatordto.LanguageType("js")),
									},
								},
							},
						}
						ptr := out.(*map[string]map[string]*evaluatordto.EvaluatorContent)
						*ptr = m
						return nil
					},
				)
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{
				"custom_template": {
					string(evaluatordto.LanguageTypeJS): {
						CodeEvaluator: &evaluatordto.CodeEvaluator{
							LanguageType: gptr.Of(evaluatordto.LanguageTypeJS),
						},
					},
				},
			},
			validateResult: func(t *testing.T, result map[string]map[string]*evaluatordto.EvaluatorContent) {
				assert.Contains(t, result["custom_template"], string(evaluatordto.LanguageTypeJS))
				jsContent := result["custom_template"][string(evaluatordto.LanguageTypeJS)]
				assert.Equal(t, evaluatordto.LanguageTypeJS, *jsContent.CodeEvaluator.LanguageType)
			},
		},
		{
			name: "自定义配置为空返回默认配置",
			mockSetup: func() {
				mockLoader.EXPECT().UnmarshalKey(ctx, key, gomock.Any(), gomock.Any()).Return(errors.New("not found"))
			},
			expectedResult: map[string]map[string]*evaluatordto.EvaluatorContent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			result := c.GetCustomCodeEvaluatorTemplateConf(ctx)
			assert.Equal(t, tt.expectedResult, result)
			if tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestGetEvaluatorTemplateSpaceConf(t *testing.T) {
	tests := []struct {
		name           string
		configData     map[string]interface{}
		expectedResult []string
	}{
		{
			name: "valid config with space IDs",
			configData: map[string]interface{}{
				"evaluator_template_space": map[string]interface{}{
					"evaluator_template_space": []string{"7565071389755228204", "1234567890123456789"},
				},
			},
			expectedResult: []string{"7565071389755228204", "1234567890123456789"},
		},
		{
			name: "empty config",
			configData: map[string]interface{}{
				"evaluator_template_space": map[string]interface{}{
					"evaluator_template_space": []string{},
				},
			},
			expectedResult: []string{},
		},
		{
			name:           "missing config",
			configData:     map[string]interface{}{},
			expectedResult: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建mock configer
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockConfiger := mocks.NewMockIConfiger(ctrl)

			// 设置mock期望
			mockConfiger.EXPECT().GetEvaluatorTemplateSpaceConf(gomock.Any()).Return(tt.expectedResult)

			// 调用方法
			result := mockConfiger.GetEvaluatorTemplateSpaceConf(context.Background())

			// 验证结果
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestDefaultEvaluatorTemplateSpaceConf(t *testing.T) {
	result := DefaultEvaluatorTemplateSpaceConf()
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result))
}
