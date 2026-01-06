// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
)

func TestNewPromptService(t *testing.T) {
	t.Run("creates service with all dependencies", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Create mock dependencies
		mockFormatter := service.NewPromptFormatter()
		mockToolConfigProvider := servicemocks.NewMockIToolConfigProvider(ctrl)
		mockToolResultsProcessor := servicemocks.NewMockIToolResultsProcessor(ctrl)
		mockIDGen := mocks.NewMockIIDGenerator(ctrl)
		mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
		mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
		mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
		mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
		mockConfigProvider := confmocks.NewMockIConfigProvider(ctrl)
		mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
		mockFile := rpcmocks.NewMockIFileProvider(ctrl)

		// Call constructor
		svc := service.NewPromptService(
			mockFormatter,
			mockToolConfigProvider,
			mockToolResultsProcessor,
			mockIDGen,
			mockDebugLogRepo,
			mockDebugContextRepo,
			mockManageRepo,
			mockLabelRepo,
			mockConfigProvider,
			mockLLM,
			mockFile,
			service.NewCozeLoopSnippetParser(),
		)

		// Verify
		assert.NotNil(t, svc)

		// Verify it returns the expected implementation type
		_, ok := svc.(*service.PromptServiceImpl)
		assert.True(t, ok, "should return *PromptServiceImpl")
	})

	t.Run("sets formatter correctly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockFormatter := service.NewPromptFormatter()
		mockToolConfigProvider := servicemocks.NewMockIToolConfigProvider(ctrl)
		mockToolResultsProcessor := servicemocks.NewMockIToolResultsProcessor(ctrl)
		mockIDGen := mocks.NewMockIIDGenerator(ctrl)
		mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
		mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
		mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
		mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
		mockConfigProvider := confmocks.NewMockIConfigProvider(ctrl)
		mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
		mockFile := rpcmocks.NewMockIFileProvider(ctrl)

		svc := service.NewPromptService(
			mockFormatter,
			mockToolConfigProvider,
			mockToolResultsProcessor,
			mockIDGen,
			mockDebugLogRepo,
			mockDebugContextRepo,
			mockManageRepo,
			mockLabelRepo,
			mockConfigProvider,
			mockLLM,
			mockFile,
			service.NewCozeLoopSnippetParser(),
		)

		_ = svc
	})
}
