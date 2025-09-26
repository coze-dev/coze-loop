// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// MockCodeBuilderFactory 实现 CodeBuilderFactory 接口用于测试
type MockCodeBuilderFactory struct {
	ctrl     *gomock.Controller
	recorder *MockCodeBuilderFactoryMockRecorder
}

type MockCodeBuilderFactoryMockRecorder struct {
	mock *MockCodeBuilderFactory
}

func NewMockCodeBuilderFactory(ctrl *gomock.Controller) *MockCodeBuilderFactory {
	mock := &MockCodeBuilderFactory{ctrl: ctrl}
	mock.recorder = &MockCodeBuilderFactoryMockRecorder{mock}
	return mock
}

func (m *MockCodeBuilderFactory) EXPECT() *MockCodeBuilderFactoryMockRecorder {
	return m.recorder
}

func (m *MockCodeBuilderFactory) CreateBuilder(languageType entity.LanguageType) (UserCodeBuilder, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateBuilder", languageType)
	ret0, _ := ret[0].(UserCodeBuilder)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockCodeBuilderFactoryMockRecorder) CreateBuilder(languageType interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateBuilder", reflect.TypeOf((*MockCodeBuilderFactory)(nil).CreateBuilder), languageType)
}

func (m *MockCodeBuilderFactory) GetSupportedLanguages() []entity.LanguageType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSupportedLanguages")
	ret0, _ := ret[0].([]entity.LanguageType)
	return ret0
}

func (mr *MockCodeBuilderFactoryMockRecorder) GetSupportedLanguages() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSupportedLanguages", reflect.TypeOf((*MockCodeBuilderFactory)(nil).GetSupportedLanguages))
}

func (m *MockCodeBuilderFactory) SetRuntimeManager(manager component.IRuntimeManager) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetRuntimeManager", manager)
}

func (mr *MockCodeBuilderFactoryMockRecorder) SetRuntimeManager(manager interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetRuntimeManager", reflect.TypeOf((*MockCodeBuilderFactory)(nil).SetRuntimeManager), manager)
}

// MockUserCodeBuilder 实现 UserCodeBuilder 接口用于测试
type MockUserCodeBuilder struct {
	ctrl     *gomock.Controller
	recorder *MockUserCodeBuilderMockRecorder
}

type MockUserCodeBuilderMockRecorder struct {
	mock *MockUserCodeBuilder
}

func NewMockUserCodeBuilder(ctrl *gomock.Controller) *MockUserCodeBuilder {
	mock := &MockUserCodeBuilder{ctrl: ctrl}
	mock.recorder = &MockUserCodeBuilderMockRecorder{mock}
	return mock
}

func (m *MockUserCodeBuilder) EXPECT() *MockUserCodeBuilderMockRecorder {
	return m.recorder
}

func (m *MockUserCodeBuilder) BuildCode(input *entity.EvaluatorInputData, codeVersion *entity.CodeEvaluatorVersion) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BuildCode", input, codeVersion)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (mr *MockUserCodeBuilderMockRecorder) BuildCode(input, codeVersion interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BuildCode", reflect.TypeOf((*MockUserCodeBuilder)(nil).BuildCode), input, codeVersion)
}

func (m *MockUserCodeBuilder) BuildSyntaxCheckCode(userCode string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BuildSyntaxCheckCode", userCode)
	ret0, _ := ret[0].(string)
	return ret0
}

func (mr *MockUserCodeBuilderMockRecorder) BuildSyntaxCheckCode(userCode interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BuildSyntaxCheckCode", reflect.TypeOf((*MockUserCodeBuilder)(nil).BuildSyntaxCheckCode), userCode)
}

func (m *MockUserCodeBuilder) GetLanguageType() entity.LanguageType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLanguageType")
	ret0, _ := ret[0].(entity.LanguageType)
	return ret0
}

func (mr *MockUserCodeBuilderMockRecorder) GetLanguageType() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLanguageType", reflect.TypeOf((*MockUserCodeBuilder)(nil).GetLanguageType))
}

func (m *MockUserCodeBuilder) SetRuntime(runtime component.IRuntime) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetRuntime", runtime)
}

func (mr *MockUserCodeBuilderMockRecorder) SetRuntime(runtime interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetRuntime", reflect.TypeOf((*MockUserCodeBuilder)(nil).SetRuntime), runtime)
}

// TestEvaluatorSourceCodeServiceImpl_Run 测试 Run 方法
func TestEvaluatorSourceCodeServiceImpl_Run(t *testing.T) {
	t.Parallel()

	t.Run("成功执行Python代码评估器", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
		mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
		mockRuntime := componentmocks.NewMockIRuntime(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            100,
			SpaceID:       1,
			Name:          "Test Python Evaluator",
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:           100,
				EvaluatorID:  100,
				SpaceID:      1,
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation(turn, user_input, model_output, model_config, evaluator_config):\n    return {'score': 1.0, 'reasoning': 'test'}",
			},
		}

		input := &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"user_input": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test input"),
				},
			},
		}

		// Mock setup
		mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
		mockCodeBuilder.EXPECT().BuildCode(input, evaluator.CodeEvaluatorVersion).Return("built_code", nil)
		mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
		mockRuntime.EXPECT().RunCode(gomock.Any(), "built_code", "Python", gomock.Any(), gomock.Any()).Return(
			&entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: `{"score": 1.0, "reason": "test"}`,
					Stdout: "execution output",
					Stderr: "",
				},
			}, nil)

		output, status, _ := service.Run(ctx, evaluator, input, false)

		assert.Equal(t, entity.EvaluatorRunStatusSuccess, status)
		assert.NotNil(t, output.EvaluatorResult)
		assert.Equal(t, 1.0, *output.EvaluatorResult.Score)
		assert.Equal(t, "test", output.EvaluatorResult.Reasoning)
		assert.Nil(t, output.EvaluatorRunError)
	})

	t.Run("评估器类型验证失败", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            102,
			SpaceID:       1,
			Name:          "Invalid Evaluator",
			EvaluatorType: entity.EvaluatorTypePrompt, // 错误的类型
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:          102,
				EvaluatorID: 102,
				SpaceID:     1,
			},
		}

		input := &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"user_input": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test input"),
				},
			},
		}

		output, status, _ := service.Run(ctx, evaluator, input, false)

		assert.Equal(t, entity.EvaluatorRunStatusFail, status)
		assert.NotNil(t, output.EvaluatorRunError)
		assert.Equal(t, int32(errno.InvalidEvaluatorTypeCode), output.EvaluatorRunError.Code)
		assert.Contains(t, output.EvaluatorRunError.Message, "invalid evaluator type")
	})

	t.Run("代码构建失败", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
		mockCodeBuilder := NewMockUserCodeBuilder(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            100,
			SpaceID:       1,
			Name:          "Test Python Evaluator",
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:           100,
				EvaluatorID:  100,
				SpaceID:      1,
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation():\n    pass",
			},
		}

		input := &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"user_input": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test input"),
				},
			},
		}

		// Mock setup
		mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
		mockCodeBuilder.EXPECT().BuildCode(input, evaluator.CodeEvaluatorVersion).Return("", errors.New("code build failed"))

		output, status, _ := service.Run(ctx, evaluator, input, false)

		assert.Equal(t, entity.EvaluatorRunStatusFail, status)
		assert.NotNil(t, output.EvaluatorRunError)
		assert.Equal(t, int32(errno.CodeBuildFailedCode), output.EvaluatorRunError.Code)
		assert.Contains(t, output.EvaluatorRunError.Message, "code build failed")
	})

	t.Run("Runtime获取失败", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
		mockCodeBuilder := NewMockUserCodeBuilder(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            100,
			SpaceID:       1,
			Name:          "Test Python Evaluator",
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:           100,
				EvaluatorID:  100,
				SpaceID:      1,
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation():\n    pass",
			},
		}

		input := &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"user_input": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test input"),
				},
			},
		}

		// Mock setup
		mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
		mockCodeBuilder.EXPECT().BuildCode(input, evaluator.CodeEvaluatorVersion).Return("built_code", nil)
		mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(nil, errors.New("runtime not found"))

		output, status, _ := service.Run(ctx, evaluator, input, false)

		assert.Equal(t, entity.EvaluatorRunStatusFail, status)
		assert.NotNil(t, output.EvaluatorRunError)
		assert.Equal(t, int32(errno.RuntimeGetFailedCode), output.EvaluatorRunError.Code)
		assert.Contains(t, output.EvaluatorRunError.Message, "runtime not found")
	})
}

// TestEvaluatorSourceCodeServiceImpl_Debug 测试 Debug 方法
func TestEvaluatorSourceCodeServiceImpl_Debug(t *testing.T) {
	t.Parallel()

	t.Run("成功调试代码评估器", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
		mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
		mockRuntime := componentmocks.NewMockIRuntime(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            100,
			SpaceID:       1,
			Name:          "Test Python Evaluator",
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:           100,
				EvaluatorID:  100,
				SpaceID:      1,
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation():\n    return {'score': 1.0, 'reasoning': 'test'}",
			},
		}

		input := &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"user_input": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test input"),
				},
			},
		}

		// Mock setup
		mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
		mockCodeBuilder.EXPECT().BuildCode(input, evaluator.CodeEvaluatorVersion).Return("built_code", nil)
		mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
		mockRuntime.EXPECT().RunCode(gomock.Any(), "built_code", "Python", gomock.Any(), gomock.Any()).Return(
			&entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: `{"score": 1.0, "reason": "test"}`,
					Stdout: "debug output",
					Stderr: "",
				},
			}, nil)

		output, err := service.Debug(ctx, evaluator, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.NotNil(t, output.EvaluatorResult)
		assert.Equal(t, 1.0, *output.EvaluatorResult.Score)
		assert.Equal(t, "test", output.EvaluatorResult.Reasoning)
	})

	t.Run("调试失败场景", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
		mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
		mockRuntime := componentmocks.NewMockIRuntime(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            100,
			SpaceID:       1,
			Name:          "Test Python Evaluator",
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:           100,
				EvaluatorID:  100,
				SpaceID:      1,
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation():\n    pass",
			},
		}

		input := &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"user_input": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test input"),
				},
			},
		}

		// Mock setup
		mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
		mockCodeBuilder.EXPECT().BuildCode(input, evaluator.CodeEvaluatorVersion).Return("built_code", nil)
		mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
		mockRuntime.EXPECT().RunCode(gomock.Any(), "built_code", "Python", gomock.Any(), gomock.Any()).Return(
			&entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: "",
					Stdout: "",
					Stderr: "SyntaxError: invalid syntax",
				},
			}, nil)

		output, err := service.Debug(ctx, evaluator, input)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SyntaxError: invalid syntax")
		assert.NotNil(t, output)
	})
}

// TestEvaluatorSourceCodeServiceImpl_PreHandle 测试 PreHandle 方法
func TestEvaluatorSourceCodeServiceImpl_PreHandle(t *testing.T) {
	t.Parallel()

	t.Run("成功预处理代码评估器", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            100,
			SpaceID:       1,
			Name:          "Test Evaluator",
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:           100,
				EvaluatorID:  100,
				SpaceID:      1,
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation():\n    pass",
			},
		}

		err := service.PreHandle(ctx, evaluator)

		assert.NoError(t, err)
	})

	t.Run("评估器类型错误", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            101,
			SpaceID:       1,
			Name:          "Invalid Evaluator",
			EvaluatorType: entity.EvaluatorTypePrompt, // 错误的类型
		}

		err := service.PreHandle(ctx, evaluator)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid evaluator type or code evaluator version is nil")
	})
}

// TestEvaluatorSourceCodeServiceImpl_Validate 测试 Validate 方法
func TestEvaluatorSourceCodeServiceImpl_Validate(t *testing.T) {
	t.Parallel()

	t.Run("Python代码验证成功", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
		mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
		mockRuntime := componentmocks.NewMockIRuntime(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            100,
			SpaceID:       1,
			Name:          "Valid Python Evaluator",
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:           100,
				EvaluatorID:  100,
				SpaceID:      1,
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation(turn, user_input, model_output, model_config, evaluator_config):\n    return {'score': 1.0, 'reasoning': 'test'}",
			},
		}

		// Mock setup
		mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
		mockCodeBuilder.EXPECT().BuildSyntaxCheckCode(gomock.Any()).Return("syntax_check_code")
		mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
		mockRuntime.EXPECT().RunCode(gomock.Any(), "syntax_check_code", "python", int64(10000), gomock.Any()).Return(
			&entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: `{"valid": true}`,
					Stdout: "",
					Stderr: "",
				},
			}, nil)

		err := service.Validate(ctx, evaluator)

		assert.NoError(t, err)
	})

	t.Run("评估器类型错误", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            102,
			SpaceID:       1,
			Name:          "Invalid Type Evaluator",
			EvaluatorType: entity.EvaluatorTypePrompt, // 错误的类型
		}

		err := service.Validate(ctx, evaluator)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid evaluator type or code evaluator version is nil")
	})

	t.Run("不支持的语言类型", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
		mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
		mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

		service := &EvaluatorSourceCodeServiceImpl{
			runtimeManager:     mockRuntimeManager,
			codeBuilderFactory: mockCodeBuilderFactory,
			metric:             mockMetric,
		}

		ctx := context.Background()
		evaluator := &entity.Evaluator{
			ID:            105,
			SpaceID:       1,
			Name:          "Unsupported Language Evaluator",
			EvaluatorType: entity.EvaluatorTypeCode,
			CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
				ID:           105,
				EvaluatorID:  105,
				SpaceID:      1,
				LanguageType: "unsupported", // 不支持的语言
				CodeContent:  "some code",
			},
		}

		err := service.Validate(ctx, evaluator)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "language type: unsupported")
	})
}

// TestEvaluatorSourceCodeServiceImpl_EvaluatorType 测试 EvaluatorType 方法
func TestEvaluatorSourceCodeServiceImpl_EvaluatorType(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
	mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
	mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

	service := &EvaluatorSourceCodeServiceImpl{
		runtimeManager:     mockRuntimeManager,
		codeBuilderFactory: mockCodeBuilderFactory,
		metric:             mockMetric,
	}

	evaluatorType := service.EvaluatorType()
	assert.Equal(t, entity.EvaluatorTypeCode, evaluatorType)
}

// TestNewEvaluatorSourceCodeServiceImpl 测试构造函数
func TestNewEvaluatorSourceCodeServiceImpl(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
	mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
	mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

	service := NewEvaluatorSourceCodeServiceImpl(
		mockRuntimeManager,
		mockCodeBuilderFactory,
		mockMetric,
	)

	assert.NotNil(t, service)
	assert.Implements(t, (*EvaluatorSourceService)(nil), service)
	assert.Equal(t, mockRuntimeManager, service.runtimeManager)
	assert.Equal(t, mockCodeBuilderFactory, service.codeBuilderFactory)
	assert.Equal(t, mockMetric, service.metric)
}

// TestEvaluatorSourceCodeServiceImpl_decodeUnicodeEscapes 测试 decodeUnicodeEscapes 方法
func TestEvaluatorSourceCodeServiceImpl_decodeUnicodeEscapes(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "无Unicode转义字符",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "单个Unicode转义字符",
			input:    "hello \\u4e2d world",
			expected: "hello 中 world",
		},
		{
			name:     "多个Unicode转义字符",
			input:    "\\u4f60\\u597d\\u4e16\\u754c",
			expected: "你好世界",
		},
		{
			name:     "混合Unicode和普通字符",
			input:    "Hello \\u4e2d\\u6587 World",
			expected: "Hello 中文 World",
		},
		{
			name:     "无效的Unicode转义",
			input:    "\\uXXXX",
			expected: "\\uXXXX",
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := service.decodeUnicodeEscapes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
func TestEvaluatorSourceCodeServiceImpl_Run_MoreCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMocks     func(*gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime)
		evaluator      *entity.Evaluator
		input          *entity.EvaluatorInputData
		expectStatus   entity.EvaluatorRunStatus
		expectError    bool
		validateOutput func(*testing.T, *entity.EvaluatorOutputData)
	}{
		{
			name: "代码执行成功但解析失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockRuntime := componentmocks.NewMockIRuntime(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildCode(gomock.Any(), gomock.Any()).Return("built_code", nil)
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
				mockRuntime.EXPECT().RunCode(gomock.Any(), "built_code", "Python", gomock.Any(), gomock.Any()).Return(
					&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: "invalid json",
							Stdout: "",
							Stderr: "parse error",
						},
					}, nil)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, mockRuntime
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Python Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    return 'invalid'",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			expectStatus: entity.EvaluatorRunStatusFail,
			expectError:  false,
			validateOutput: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Contains(t, output.EvaluatorRunError.Message, "parse error")
			},
		},
		{
			name: "代码构建器创建失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(nil, errors.New("unsupported language"))

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Python Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    pass",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			expectStatus: entity.EvaluatorRunStatusFail,
			expectError:  false,
			validateOutput: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Equal(t, int32(errno.CodeBuilderGetFailedCode), output.EvaluatorRunError.Code)
			},
		},
		{
			name: "代码执行失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockRuntime := componentmocks.NewMockIRuntime(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildCode(gomock.Any(), gomock.Any()).Return("built_code", nil)
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
				mockRuntime.EXPECT().RunCode(gomock.Any(), "built_code", "Python", gomock.Any(), gomock.Any()).Return(nil, errors.New("execution timeout"))

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, mockRuntime
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Python Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    pass",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			expectStatus: entity.EvaluatorRunStatusFail,
			expectError:  false,
			validateOutput: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Equal(t, int32(errno.CodeExecutionFailedCode), output.EvaluatorRunError.Code)
			},
		},
		{
			name: "CodeEvaluatorVersion为nil",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:                   100,
				SpaceID:              1,
				Name:                 "Invalid Evaluator",
				EvaluatorType:        entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: nil, // nil版本
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			expectStatus: entity.EvaluatorRunStatusFail,
			expectError:  false,
			validateOutput: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Equal(t, int32(errno.InvalidEvaluatorTypeCode), output.EvaluatorRunError.Code)
			},
		},
		{
			name: "成功执行JavaScript代码",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockRuntime := componentmocks.NewMockIRuntime(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypeJS).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildCode(gomock.Any(), gomock.Any()).Return("built_js_code", nil)
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypeJS).Return(mockRuntime, nil)
				mockRuntime.EXPECT().RunCode(gomock.Any(), "built_js_code", "JS", gomock.Any(), gomock.Any()).Return(
					&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"score": 0.8, "reason": "js test"}`,
							Stdout: "",
							Stderr: "",
						},
					}, nil)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, mockRuntime
			},
			evaluator: &entity.Evaluator{
				ID:            101,
				SpaceID:       1,
				Name:          "Test JS Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           101,
					EvaluatorID:  101,
					SpaceID:      1,
					LanguageType: entity.LanguageTypeJS,
					CodeContent:  "function execEvaluation() { return {score: 0.8, reason: 'good match'}; }",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test js input"),
					},
				},
			},
			expectStatus: entity.EvaluatorRunStatusSuccess,
			expectError:  false,
			validateOutput: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorResult)
				assert.Equal(t, 0.8, *output.EvaluatorResult.Score)
				assert.Equal(t, "js test", output.EvaluatorResult.Reasoning)
				assert.Nil(t, output.EvaluatorRunError)
				assert.Equal(t, "", output.Stdout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRuntimeManager, mockCodeBuilderFactory, mockMetric, _, _ := tt.setupMocks(ctrl)

			service := &EvaluatorSourceCodeServiceImpl{
				runtimeManager:     mockRuntimeManager,
				codeBuilderFactory: mockCodeBuilderFactory,
				metric:             mockMetric,
			}

			ctx := context.Background()
			output, status, _ := service.Run(ctx, tt.evaluator, tt.input, false)

			assert.Equal(t, tt.expectStatus, status)
			if tt.validateOutput != nil {
				tt.validateOutput(t, output)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_Validate_MoreCases 测试Validate方法的更多场景
func TestEvaluatorSourceCodeServiceImpl_Validate_MoreCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupMocks func(*gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime)
		evaluator  *entity.Evaluator
		wantErr    bool
		errMsg     string
	}{
		{
			name: "JavaScript代码验证成功",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockRuntime := componentmocks.NewMockIRuntime(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypeJS).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildSyntaxCheckCode(gomock.Any()).Return("js_syntax_check_code")
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypeJS).Return(mockRuntime, nil)
				mockRuntime.EXPECT().RunCode(gomock.Any(), "js_syntax_check_code", "js", int64(10000), gomock.Any()).Return(
					&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"valid": true}`,
							Stdout: "",
							Stderr: "",
						},
					}, nil)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, mockRuntime
			},
			evaluator: &entity.Evaluator{
				ID:            103,
				SpaceID:       1,
				Name:          "Valid JS Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           103,
					EvaluatorID:  103,
					SpaceID:      1,
					LanguageType: entity.LanguageTypeJS,
					CodeContent:  "function execEvaluation(turn, userInput, modelOutput, modelConfig, evaluatorConfig) { return {score: 1.0, reasoning: 'test'}; }",
				},
			},
			wantErr: false,
		},
		{
			name: "Python语法验证失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockRuntime := componentmocks.NewMockIRuntime(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildSyntaxCheckCode(gomock.Any()).Return("python_syntax_check_code")
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
				mockRuntime.EXPECT().RunCode(gomock.Any(), "python_syntax_check_code", "python", int64(10000), gomock.Any()).Return(
					&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"valid": false, "error": "SyntaxError: invalid syntax"}`,
							Stdout: "",
							Stderr: "",
						},
					}, nil)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, mockRuntime
			},
			evaluator: &entity.Evaluator{
				ID:            104,
				SpaceID:       1,
				Name:          "Invalid Python Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           104,
					EvaluatorID:  104,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation(turn user_input, model_output, model_config, evaluator_config):\n    return {'score': 1.0, 'reasoning': 'test'}", // 语法错误
				},
			},
			wantErr: true,
			errMsg:  "SyntaxError: invalid syntax",
		},
		{
			name: "Runtime获取失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(nil, errors.New("runtime unavailable"))

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            106,
				SpaceID:       1,
				Name:          "Runtime Error Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           106,
					EvaluatorID:  106,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation(turn, user_input, model_output, model_config, evaluator_config):\n    return {'score': 1.0, 'reasoning': 'test'}",
				},
			},
			wantErr: true,
			errMsg:  "runtime unavailable",
		},
		{
			name: "空代码内容",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            107,
				SpaceID:       1,
				Name:          "Empty Code Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           107,
					EvaluatorID:  107,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "", // 空代码
				},
			},
			wantErr: true,
			errMsg:  "code content is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRuntimeManager, mockCodeBuilderFactory, mockMetric, _, _ := tt.setupMocks(ctrl)

			service := &EvaluatorSourceCodeServiceImpl{
				runtimeManager:     mockRuntimeManager,
				codeBuilderFactory: mockCodeBuilderFactory,
				metric:             mockMetric,
			}

			ctx := context.Background()
			err := service.Validate(ctx, tt.evaluator)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_HelperMethods 测试辅助方法
func TestEvaluatorSourceCodeServiceImpl_HelperMethods(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	t.Run("validateEvaluator", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			evaluator *entity.Evaluator
			wantErr   bool
		}{
			{
				name: "有效评估器",
				evaluator: &entity.Evaluator{
					EvaluatorType: entity.EvaluatorTypeCode,
					CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
						ID: 1,
					},
				},
				wantErr: false,
			},
			{
				name: "错误的评估器类型",
				evaluator: &entity.Evaluator{
					EvaluatorType: entity.EvaluatorTypePrompt,
					CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
						ID: 1,
					},
				},
				wantErr: true,
			},
			{
				name: "CodeEvaluatorVersion为nil",
				evaluator: &entity.Evaluator{
					EvaluatorType:        entity.EvaluatorTypeCode,
					CodeEvaluatorVersion: nil,
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := service.validateEvaluator(tt.evaluator, time.Now())
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("getFinalStdout", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name             string
			result           *entity.ExecutionResult
			processedStdout  string
			canIgnoreStderr  bool
			expectedStdout   string
		}{
			{
				name: "使用处理后的stdout",
				result: &entity.ExecutionResult{
					Output: &entity.ExecutionOutput{
						Stdout: "original",
					},
				},
				processedStdout: "processed output",
				canIgnoreStderr: true,
				expectedStdout:  "processed output",
			},
			{
				name: "使用原始stdout",
				result: &entity.ExecutionResult{
					Output: &entity.ExecutionOutput{
						Stdout: "original output",
					},
				},
				processedStdout: "",
				canIgnoreStderr: false,
				expectedStdout:  "original output",
			},
			{
				name:             "空结果",
				result:           &entity.ExecutionResult{},
				processedStdout:  "",
				canIgnoreStderr:  false,
				expectedStdout:   "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				result := service.getFinalStdout(tt.result, tt.processedStdout, tt.canIgnoreStderr)
				assert.Equal(t, tt.expectedStdout, result)
			})
		}
	})
}

// TestEvaluatorSourceCodeServiceImpl_cleanNestedJSON 测试 cleanNestedJSON 方法
func TestEvaluatorSourceCodeServiceImpl_cleanNestedJSON(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "简单JSON",
			input:    `{"score": 1.0, "reason": "test"}`,
			expected: `{"score": 1.0, "reason": "test"}`,
		},
		{
			name: "嵌套JSON - 找到评估结果",
			input: `{"score": 1.0, "reason": "test"}
{"stdout": "output", "stderr": ""}`,
			expected: `{"score": 1.0, "reason": "test"}`,
		},
		{
			name: "多行无评估结果",
			input: `{"stdout": "output"}
{"stderr": "error"}`,
			expected: `{"stdout": "output"}
{"stderr": "error"}`,
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
		{
			name:     "只有空白字符",
			input:    "   \n\t  ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := service.cleanNestedJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_prepareAndExecuteCode 测试 prepareAndExecuteCode 方法
func TestEvaluatorSourceCodeServiceImpl_prepareAndExecuteCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMocks  func(*gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime)
		evaluator   *entity.Evaluator
		input       *entity.EvaluatorInputData
		wantCode    string
		wantResult  *entity.ExecutionResult
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "成功准备和执行代码",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockRuntime := componentmocks.NewMockIRuntime(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildCode(gomock.Any(), gomock.Any()).Return("built_code", nil)
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
				mockRuntime.EXPECT().RunCode(gomock.Any(), "built_code", "Python", gomock.Any(), gomock.Any()).Return(
					&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"score": 1.0, "reason": "test"}`,
							Stdout: "success",
							Stderr: "",
						},
					}, nil)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, mockRuntime
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    pass",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			wantCode: "built_code",
			wantResult: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: `{"score": 1.0, "reason": "test"}`,
					Stdout: "success",
					Stderr: "",
				},
			},
			wantErr: false,
		},
		{
			name: "代码构建器创建失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(nil, errors.New("unsupported language"))

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    pass",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			wantCode:    "",
			wantResult:  nil,
			wantErr:     true,
			wantErrCode: int32(errno.CodeBuilderGetFailedCode),
		},
		{
			name: "代码构建失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildCode(gomock.Any(), gomock.Any()).Return("", errors.New("build failed"))

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, nil
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    pass",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			wantCode:    "",
			wantResult:  nil,
			wantErr:     true,
			wantErrCode: int32(errno.CodeBuildFailedCode),
		},
		{
			name: "运行时获取失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildCode(gomock.Any(), gomock.Any()).Return("built_code", nil)
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(nil, errors.New("runtime not found"))

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, nil
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    pass",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			wantCode:    "built_code",
			wantResult:  nil,
			wantErr:     true,
			wantErrCode: int32(errno.RuntimeGetFailedCode),
		},
		{
			name: "代码执行失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockRuntime := componentmocks.NewMockIRuntime(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildCode(gomock.Any(), gomock.Any()).Return("built_code", nil)
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
				mockRuntime.EXPECT().RunCode(gomock.Any(), "built_code", "Python", gomock.Any(), gomock.Any()).Return(nil, errors.New("execution timeout"))

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, mockRuntime
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    pass",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			wantCode:    "built_code",
			wantResult:  nil,
			wantErr:     true,
			wantErrCode: int32(errno.CodeExecutionFailedCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRuntimeManager, mockCodeBuilderFactory, mockMetric, _, _ := tt.setupMocks(ctrl)

			service := &EvaluatorSourceCodeServiceImpl{
				runtimeManager:     mockRuntimeManager,
				codeBuilderFactory: mockCodeBuilderFactory,
				metric:             mockMetric,
			}

			ctx := context.Background()
			startTime := time.Now()

			code, result, err := service.prepareAndExecuteCode(ctx, tt.evaluator, tt.input, startTime)

			if tt.wantErr {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantErrCode, statusErr.Code())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCode, code)
				assert.Equal(t, tt.wantResult, result)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_processCodeExecutionResult 测试 processCodeExecutionResult 方法
func TestEvaluatorSourceCodeServiceImpl_processCodeExecutionResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		result       *entity.ExecutionResult
		wantStatus   entity.EvaluatorRunStatus
		wantErr      bool
		validateFunc func(*testing.T, *entity.EvaluatorOutputData)
	}{
		{
			name: "成功处理执行结果",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: `{"score": 1.0, "reason": "test success"}`,
					Stdout: "execution output",
					Stderr: "",
				},
			},
			wantStatus: entity.EvaluatorRunStatusSuccess,
			wantErr:    false,
			validateFunc: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorResult)
				assert.Equal(t, 1.0, *output.EvaluatorResult.Score)
				assert.Equal(t, "test success", output.EvaluatorResult.Reasoning)
				assert.Nil(t, output.EvaluatorRunError)
				assert.Equal(t, "execution output", output.Stdout)
			},
		},
		{
			name: "解析失败但有stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: "invalid json",
					Stdout: "",
					Stderr: "SyntaxError: invalid syntax",
				},
			},
			wantStatus: entity.EvaluatorRunStatusFail,
			wantErr:    true,
			validateFunc: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Equal(t, int32(errno.CodeExecutionFailedCode), output.EvaluatorRunError.Code)
				assert.Contains(t, output.EvaluatorRunError.Message, "SyntaxError: invalid syntax")
			},
		},
		{
			name: "成功解析结果但有stderr警告",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: `{"score": 0.8, "reason": "good but with warning"}`,
					Stdout: "normal output",
					Stderr: "warning: deprecated function used",
				},
			},
			wantStatus: entity.EvaluatorRunStatusSuccess,
			wantErr:    false,
			validateFunc: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorResult)
				assert.Equal(t, 0.8, *output.EvaluatorResult.Score)
				assert.Equal(t, "good but with warning", output.EvaluatorResult.Reasoning)
				assert.Nil(t, output.EvaluatorRunError)
				assert.Contains(t, output.Stdout, "normal output")
				assert.Contains(t, output.Stdout, "[warning] warning: deprecated function used")
			},
		},
		{
			name: "RetVal解析失败且有错误信息",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: "parse error: invalid format",
					Stdout: "",
					Stderr: "RuntimeError: execution failed",
				},
			},
			wantStatus: entity.EvaluatorRunStatusFail,
			wantErr:    true,
			validateFunc: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Equal(t, int32(errno.CodeExecutionFailedCode), output.EvaluatorRunError.Code)
				assert.Contains(t, output.EvaluatorRunError.Message, "parse error: invalid format")
				assert.Contains(t, output.EvaluatorRunError.Message, "RuntimeError: execution failed")
			},
		},
		{
			name: "空RetVal但有stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					RetVal: "",
					Stdout: "",
					Stderr: "ImportError: module not found",
				},
			},
			wantStatus: entity.EvaluatorRunStatusFail,
			wantErr:    true,
			validateFunc: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Equal(t, int32(errno.CodeExecutionFailedCode), output.EvaluatorRunError.Code)
				assert.Contains(t, output.EvaluatorRunError.Message, "ImportError: module not found")
			},
		},
		{
			name: "Output为nil",
			result: &entity.ExecutionResult{
				Output: nil,
			},
			wantStatus: entity.EvaluatorRunStatusSuccess,
			wantErr:    false,
			validateFunc: func(t *testing.T, output *entity.EvaluatorOutputData) {
				assert.Nil(t, output.EvaluatorResult)
				assert.Nil(t, output.EvaluatorRunError)
				assert.Equal(t, "", output.Stdout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &EvaluatorSourceCodeServiceImpl{}
			startTime := time.Now()

			output, status, err := service.processCodeExecutionResult(tt.result, startTime)

			assert.Equal(t, tt.wantStatus, status)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.NotNil(t, output)
			assert.GreaterOrEqual(t, output.TimeConsumingMS, int64(0))

			if tt.validateFunc != nil {
				tt.validateFunc(t, output)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_Validate_Extended 扩展Validate方法测试
func TestEvaluatorSourceCodeServiceImpl_Validate_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMocks  func(*gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime)
		evaluator   *entity.Evaluator
		wantErr     bool
		wantErrCode int32
		errContains string
	}{
		{
			name: "空代码内容",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Empty Code Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           100,
					EvaluatorID:  100,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "", // 空代码
				},
			},
			wantErr:     true,
			wantErrCode: int32(errno.InvalidCodeContentCode),
			errContains: "code content is empty",
		},
		{
			name: "Python危险函数检测",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            101,
				SpaceID:       1,
				Name:          "Dangerous Python Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           101,
					EvaluatorID:  101,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "import os\ndef exec_evaluation():\n    os.system('rm -rf /')",
				},
			},
			wantErr:     true,
			wantErrCode: int32(errno.DangerousImportDetectedCode),
			errContains: "detected import: os",
		},
		{
			name: "Python危险模式检测 - 无限循环",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            102,
				SpaceID:       1,
				Name:          "Malicious Loop Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           102,
					EvaluatorID:  102,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    while True:\n        pass",
				},
			},			wantErr:     true,
			wantErrCode: int32(errno.MaliciousCodePatternDetectedCode),
			errContains: "安全违规",
		},
		{
			name: "JavaScript危险函数检测",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            103,
				SpaceID:       1,
				Name:          "Dangerous JS Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           103,
					EvaluatorID:  103,
					SpaceID:      1,
					LanguageType: entity.LanguageTypeJS,
					CodeContent:  "function execEvaluation() { eval('malicious code'); }",
				},
			},
			wantErr:     true,
			wantErrCode: int32(errno.DangerousFunctionDetectedCode),
			errContains: "detected function: eval",
		},
		{
			name: "JavaScript无限循环检测",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            104,
				SpaceID:       1,
				Name:          "JS Infinite Loop Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           104,
					EvaluatorID:  104,
					SpaceID:      1,
					LanguageType: entity.LanguageTypeJS,
					CodeContent:  "function execEvaluation() { while(true) { } }",
				},
			},
			wantErr:     true,
			wantErrCode: int32(errno.MaliciousCodePatternDetectedCode),
			errContains: "安全违规",
		},
		{
			name: "缺少exec_evaluation函数",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, nil, nil
			},
			evaluator: &entity.Evaluator{
				ID:            105,
				SpaceID:       1,
				Name:          "No Function Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           105,
					EvaluatorID:  105,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def other_function():\n    pass", // 没有exec_evaluation函数
				},
			},
			wantErr:     true,
			wantErrCode: int32(errno.RequiredFunctionNotFoundCode),
			errContains: "代码中必须定义 exec_evaluation",
		},
		{
			name: "语法检查失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockRuntime := componentmocks.NewMockIRuntime(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildSyntaxCheckCode(gomock.Any()).Return("syntax_check_code")
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
				mockRuntime.EXPECT().RunCode(gomock.Any(), "syntax_check_code", "python", int64(10000), gomock.Any()).Return(
					&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"valid": false, "error": "SyntaxError: invalid syntax"}`,
							Stdout: "",
							Stderr: "",
						},
					}, nil)

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, mockRuntime
			},
			evaluator: &entity.Evaluator{
				ID:            106,
				SpaceID:       1,
				Name:          "Syntax Error Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           106,
					EvaluatorID:  106,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation(\n    # 语法错误：缺少右括号",
				},
			},
			wantErr:     true,
			wantErrCode: int32(errno.SyntaxValidationFailedCode),
			errContains: "SyntaxError: invalid syntax",
		},
		{
			name: "运行时获取失败",
			setupMocks: func(ctrl *gomock.Controller) (*componentmocks.MockIRuntimeManager, *MockCodeBuilderFactory, *metricsmocks.MockEvaluatorExecMetrics, *MockUserCodeBuilder, *componentmocks.MockIRuntime) {
				mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
				mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
				mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)

				mockCodeBuilderFactory.EXPECT().CreateBuilder(entity.LanguageTypePython).Return(mockCodeBuilder, nil)
				mockCodeBuilder.EXPECT().BuildSyntaxCheckCode(gomock.Any()).Return("syntax_check_code")
				mockRuntimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(nil, errors.New("runtime not available"))

				return mockRuntimeManager, mockCodeBuilderFactory, mockMetric, mockCodeBuilder, nil
			},
			evaluator: &entity.Evaluator{
				ID:            107,
				SpaceID:       1,
				Name:          "Runtime Error Evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           107,
					EvaluatorID:  107,
					SpaceID:      1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation():\n    pass",
				},
			},
			wantErr:     true,
			wantErrCode: int32(errno.RuntimeGetFailedCode),
			errContains: "runtime not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRuntimeManager, mockCodeBuilderFactory, mockMetric, _, _ := tt.setupMocks(ctrl)

			service := &EvaluatorSourceCodeServiceImpl{
				runtimeManager:     mockRuntimeManager,
				codeBuilderFactory: mockCodeBuilderFactory,
				metric:             mockMetric,
			}

			ctx := context.Background()
			err := service.Validate(ctx, tt.evaluator)

			if tt.wantErr {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.wantErrCode, statusErr.Code())
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_SecurityValidation 测试安全验证相关方法
func TestEvaluatorSourceCodeServiceImpl_SecurityValidation(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	t.Run("测试危险函数检测", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			code     string
			language string
			wantErr  bool
		}{
			{
				name:     "Python exec函数",
				code:     "exec('malicious code')",
				language: "python",
				wantErr:  true,
			},
			{
				name:     "Python eval函数",
				code:     "result = eval('1+1')",
				language: "python",
				wantErr:  true,
			},
			{
				name:     "JavaScript eval函数",
				code:     "eval('malicious code')",
				language: "javascript",
				wantErr:  true,
			},
			{
				name:     "JavaScript Function构造器",
				code:     "new Function('return 1')",
				language: "javascript",
				wantErr:  true,
			},
			{
				name:     "安全的Python代码",
				code:     "def exec_evaluation():\n    return {'score': 1.0}",
				language: "python",
				wantErr:  false,
			},
			{
				name:     "安全的JavaScript代码",
				code:     "function execEvaluation() { return {score: 1.0}; }",
				language: "javascript",
				wantErr:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := service.checkDangerousFunctions(tt.code, tt.language)
				if tt.wantErr {
					assert.Error(t, err)
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, int32(errno.DangerousFunctionDetectedCode), statusErr.Code())
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("测试危险导入检测", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			code     string
			language string
			wantErr  bool
		}{
			{
				name:     "Python os模块导入",
				code:     "import os",
				language: "python",
				wantErr:  true,
			},
			{
				name:     "Python subprocess模块导入",
				code:     "from subprocess import call",
				language: "python",
				wantErr:  true,
			},
			{
				name:     "JavaScript fs模块导入",
				code:     "const fs = require('fs')",
				language: "javascript",
				wantErr:  true,
			},
			{
				name:     "JavaScript child_process模块导入",
				code:     "import { exec } from 'child_process'",
				language: "javascript",
				wantErr:  true,
			},
			{
				name:     "安全的Python导入",
				code:     "import json\nimport math",
				language: "python",
				wantErr:  false,
			},
			{
				name:     "安全的JavaScript导入",
				code:     "import lodash from 'lodash'",
				language: "javascript",
				wantErr:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := service.checkDangerousImports(tt.code, tt.language)
				if tt.wantErr {
					assert.Error(t, err)
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, int32(errno.DangerousImportDetectedCode), statusErr.Code())
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("测试恶意模式检测", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			code     string
			language string
			wantErr  bool
		}{
			{
				name:     "Python无限循环",
				code:     "while True:\n    pass",
				language: "python",
				wantErr:  true,
			},
			{
				name:     "JavaScript无限循环",
				code:     "while(true) { }",
				language: "javascript",
				wantErr:  true,
			},
			{
				name:     "JavaScript for无限循环",
				code:     "for(;;) { }",
				language: "javascript",
				wantErr:  true,
			},
			{
				name:     "Python exit调用",
				code:     "exit(0)",
				language: "python",
				wantErr:  true,
			},
			{
				name:     "JavaScript process.exit调用",
				code:     "process.exit(0)",
				language: "javascript",
				wantErr:  true,
			},
			{
				name:     "安全的循环",
				code:     "for i in range(10):\n    print(i)",
				language: "python",
				wantErr:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := service.checkMaliciousPatterns(tt.code, tt.language)
				if tt.wantErr {
					assert.Error(t, err)
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, int32(errno.MaliciousCodePatternDetectedCode), statusErr.Code())
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

// TestEvaluatorSourceCodeServiceImpl_UtilityMethods 测试工具方法
func TestEvaluatorSourceCodeServiceImpl_UtilityMethods(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	t.Run("测试convertPythonDictToJSON", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			pythonDict string
			expected   string
		}{
			{
				name:       "简单字典",
				pythonDict: "{'score': 1.0, 'reason': 'test'}",
				expected:   `{"score": 1.0, "reason": "test"}`,
			},
			{
				name:       "嵌套引号",
				pythonDict: `{'message': "He said 'hello'"}`,
				expected:   `{"message": "He said 'hello'"}`,
			},
			{
				name:       "转义字符",
				pythonDict: `{'path': 'C:\\Users\\test'}`,
				expected:   `{"path": "C:\\Users\\test"}`,
			},
			{
				name:       "空字符串",
				pythonDict: "",
				expected:   "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				result, err := service.convertPythonDictToJSON(tt.pythonDict)
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("测试parseEvaluationRetVal", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			retVal      string
			wantScore   *float64
			wantReason  string
			wantErr     bool
		}{
			{
				name:        "标准JSON格式",
				retVal:      `{"score": 1.0, "reason": "excellent"}`,
				wantScore:   gptr.Of(1.0),
				wantReason:  "excellent",
				wantErr:     false,
			},
			{
				name:        "Python字典格式",
				retVal:      "{'score': 0.8, 'reason': 'good'}",
				wantScore:   gptr.Of(0.8),
				wantReason:  "good",
				wantErr:     false,
			},
			{
				name:        "整数分数",
				retVal:      `{"score": 1, "reason": "perfect"}`,
				wantScore:   gptr.Of(1.0),
				wantReason:  "perfect",
				wantErr:     false,
			},
			{
				name:        "字符串分数",
				retVal:      `{"score": "0.9", "reason": "very good"}`,
				wantScore:   gptr.Of(0.9),
				wantReason:  "very good",
				wantErr:     false,
			},
			{
				name:        "空retVal",
				retVal:      "",
				wantScore:   nil,
				wantReason:  "",
				wantErr:     false,
			},
			{
				name:        "无效JSON",
				retVal:      "invalid json",
				wantScore:   nil,
				wantReason:  "",
				wantErr:     true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				score, reason, err := service.parseEvaluationRetVal(tt.retVal)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					if tt.wantScore != nil {
						assert.NotNil(t, score)
						assert.Equal(t, *tt.wantScore, *score)
					} else {
						assert.Nil(t, score)
					}
					assert.Equal(t, tt.wantReason, reason)
				}
			})
		}
	})

	t.Run("测试cleanNestedJSON", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "嵌套JSON结构",
				input:    "{\"score\":1,\"reason\":\"test\"}\\n{\"stdout\":\"output\",\"stderr\":\"\"}",
				expected: "{\"score\":1,\"reason\":\"test\"}\\n{\"stdout\":\"output\",\"stderr\":\"\"}",
			},
			{
				name:     "单行JSON",
				input:    `{"score":0.8,"reason":"good"}`,
				expected: `{"score":0.8,"reason":"good"}`,
			},
			{
				name:     "多行但无有效JSON",
				input:    "line1\nline2\nline3",
				expected: "line1\nline2\nline3",
			},
			{
				name:     "空输入",
				input:    "",
				expected: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				result := service.cleanNestedJSON(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}