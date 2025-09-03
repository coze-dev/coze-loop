// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestNewRuntimeFactory(t *testing.T) {
	logger := logrus.New()
	config := DefaultSandboxConfig()

	factory := NewRuntimeFactory(logger, config)
	assert.NotNil(t, factory)

	// 测试使用nil配置
	factoryWithNilConfig := NewRuntimeFactory(logger, nil)
	assert.NotNil(t, factoryWithNilConfig)
}

func TestRuntimeFactoryImpl_CreateRuntime(t *testing.T) {
	logger := logrus.New()
	config := DefaultSandboxConfig()
	factory := NewRuntimeFactory(logger, config)

	tests := []struct {
		name         string
		languageType entity.LanguageType
		expectError  bool
	}{
		{
			name:         "创建Python运行时",
			languageType: entity.LanguageTypePython,
			expectError:  false,
		},
		{
			name:         "创建JavaScript运行时",
			languageType: entity.LanguageTypeJS,
			expectError:  false,
		},
		{
			name:         "不支持的语言类型",
			languageType: entity.LanguageType("unsupported"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime, err := factory.CreateRuntime(tt.languageType)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, runtime)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, runtime)
				assert.Equal(t, tt.languageType, runtime.GetLanguageType())
			}
		})
	}
}

func TestRuntimeFactoryImpl_GetSupportedLanguages(t *testing.T) {
	logger := logrus.New()
	config := DefaultSandboxConfig()
	factory := NewRuntimeFactory(logger, config)

	languages := factory.GetSupportedLanguages()
	assert.Len(t, languages, 2)
	assert.Contains(t, languages, entity.LanguageTypePython)
	assert.Contains(t, languages, entity.LanguageTypeJS)
}