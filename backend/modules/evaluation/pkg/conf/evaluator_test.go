// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
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
