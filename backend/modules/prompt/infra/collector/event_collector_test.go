// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

// TestNewEventCollectorProvider 测试实例创建
func TestNewEventCollectorProvider(t *testing.T) {
	// 执行测试函数
	provider := NewEventCollectorProvider()

	// 验证结果
	assert.NotNil(t, provider, "实例创建不应为nil")
	assert.IsType(t, &EventCollectorProviderImpl{}, provider, "应返回EventCollectorProviderImpl类型实例")
}

// TestCollectPromptHubEvent 测试事件收集方法
func TestCollectPromptHubEvent(t *testing.T) {
	// 准备测试数据
	testCases := []struct {
		name    string
		ctx     context.Context
		spaceID int64
		prompts []*entity.Prompt
	}{
		{
			name:    "正常参数",
			ctx:     context.Background(),
			spaceID: 123,
			prompts: []*entity.Prompt{},
		},
		{
			name:    "空上下文",
			ctx:     nil,
			spaceID: 456,
			prompts: []*entity.Prompt{},
		},
		{
			name:    "nil切片",
			ctx:     context.Background(),
			spaceID: 789,
			prompts: nil,
		},
	}

	// 创建测试实例
	collector := &EventCollectorProviderImpl{}

	// 执行测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 执行测试方法
			// 由于当前是空实现，主要测试是否会panic
			assert.NotPanics(t, func() {
				collector.CollectPromptHubEvent(tc.ctx, tc.spaceID, tc.prompts)
			}, "方法不应panic")
		})
	}
}
