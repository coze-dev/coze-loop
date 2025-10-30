// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
)

func TestNewPromptService(t *testing.T) {
	t.Run("creates service with all dependencies", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Create mock dependencies
		mockFormatter := NewPromptFormatter()
		mockIDGen := mocks.NewMockIIDGenerator(ctrl)
		mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
		mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
		mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
		mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
		mockConfigProvider := confmocks.NewMockIConfigProvider(ctrl)
		mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
		mockFile := rpcmocks.NewMockIFileProvider(ctrl)

		// Call constructor
		service := NewPromptService(
			mockFormatter,
			mockIDGen,
			mockDebugLogRepo,
			mockDebugContextRepo,
			mockManageRepo,
			mockLabelRepo,
			mockConfigProvider,
			mockLLM,
			mockFile,
		)

		// Verify
		assert.NotNil(t, service)

		// Verify it returns the interface type
		var _ IPromptService = service

		// Verify implementation has all fields set (by converting to concrete type for inspection)
		impl, ok := service.(*PromptServiceImpl)
		assert.True(t, ok, "should return *PromptServiceImpl")
		assert.NotNil(t, impl.formatter)
		assert.NotNil(t, impl.idgen)
		assert.NotNil(t, impl.debugLogRepo)
		assert.NotNil(t, impl.debugContextRepo)
		assert.NotNil(t, impl.manageRepo)
		assert.NotNil(t, impl.labelRepo)
		assert.NotNil(t, impl.configProvider)
		assert.NotNil(t, impl.llm)
		assert.NotNil(t, impl.file)
	})

	t.Run("sets formatter correctly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockFormatter := NewPromptFormatter()
		mockIDGen := mocks.NewMockIIDGenerator(ctrl)
		mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
		mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
		mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
		mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
		mockConfigProvider := confmocks.NewMockIConfigProvider(ctrl)
		mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
		mockFile := rpcmocks.NewMockIFileProvider(ctrl)

		service := NewPromptService(
			mockFormatter,
			mockIDGen,
			mockDebugLogRepo,
			mockDebugContextRepo,
			mockManageRepo,
			mockLabelRepo,
			mockConfigProvider,
			mockLLM,
			mockFile,
		)

		impl := service.(*PromptServiceImpl)
		assert.Equal(t, mockFormatter, impl.formatter, "formatter should be set correctly")
	})
}
