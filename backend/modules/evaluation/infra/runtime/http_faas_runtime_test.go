// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestHTTPFaaSRuntimeAdapter(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 创建HTTP FaaS运行时适配器
	config := &HTTPFaaSRuntimeConfig{
		BaseURL:        "http://localhost:8890", // 使用测试端口
		Timeout:        30 * time.Second,
		MaxRetries:     1, // 减少重试次数以加快测试
		RetryInterval:  100 * time.Millisecond,
		EnableEnhanced: true,
	}

	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	t.Run("GetLanguageType", func(t *testing.T) {
		langType := adapter.GetLanguageType()
		assert.Equal(t, entity.LanguageTypeJS, langType)
	})


	t.Run("Cleanup", func(t *testing.T) {
		err := adapter.Cleanup()
		assert.NoError(t, err)
	})
}



func TestHTTPFaaSRuntimeConfig_Default(t *testing.T) {
	logger := logrus.New()

	// 测试默认配置
	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, nil, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, "http://coze-loop-js-faas:8000", adapter.config.BaseURL)
	assert.Equal(t, 30*time.Second, adapter.config.Timeout)
	assert.Equal(t, 3, adapter.config.MaxRetries)
	assert.Equal(t, 1*time.Second, adapter.config.RetryInterval)
	assert.True(t, adapter.config.EnableEnhanced)
}