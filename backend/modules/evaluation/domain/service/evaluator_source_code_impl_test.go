// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
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

// TestEvaluatorSourceCodeServiceImpl_convertPythonDictToJSON 测试 convertPythonDictToJSON 方法
func TestEvaluatorSourceCodeServiceImpl_convertPythonDictToJSON(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "单引号字符串",
			input:    "{'score': 1.0, 'reason': 'test'}",
			expected: `{"score": 1.0, "reason": "test"}`,
			wantErr:  false,
		},
		{
			name:     "混合引号",
			input:    `{'score': 1.0, "reason": 'test'}`,
			expected: `{"score": 1.0, "reason": "test"}`,
			wantErr:  false,
		},
		{
			name:     "转义字符",
			input:    `{'message': 'It\'s a test'}`,
			expected: `{"message": "It\'s a test"}`,
			wantErr:  false,
		},
		{
			name:     "嵌套引号",
			input:    `{'text': 'He said "hello"'}`,
			expected: `{"text": "He said \"hello\""}`,
			wantErr:  false,
		},
		{
			name:     "空字典",
			input:    "{}",
			expected: "{}",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := service.convertPythonDictToJSON(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_parseEvaluationRetVal 测试 parseEvaluationRetVal 方法
func TestEvaluatorSourceCodeServiceImpl_parseEvaluationRetVal(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	tests := []struct {
		name        string
		retVal      string
		expectScore *float64
		expectReason string
		wantErr     bool
	}{
		{
			name:         "有效的JSON",
			retVal:       `{"score": 1.0, "reason": "test"}`,
			expectScore:  gptr.Of(1.0),
			expectReason: "test",
			wantErr:      false,
		},
		{
			name:         "Python字典格式",
			retVal:       `{'score': 0.5, 'reason': 'partial match'}`,
			expectScore:  gptr.Of(0.5),
			expectReason: "partial match",
			wantErr:      false,
		},
		{
			name:         "整数score",
			retVal:       `{"score": 1, "reason": "perfect"}`,
			expectScore:  gptr.Of(1.0),
			expectReason: "perfect",
			wantErr:      false,
		},
		{
			name:         "字符串score",
			retVal:       `{"score": "0.8", "reason": "good"}`,
			expectScore:  gptr.Of(0.8),
			expectReason: "good",
			wantErr:      false,
		},
		{
			name:         "只有score",
			retVal:       `{"score": 1.0}`,
			expectScore:  gptr.Of(1.0),
			expectReason: "",
			wantErr:      false,
		},
		{
			name:         "只有reason",
			retVal:       `{"reason": "test"}`,
			expectScore:  nil,
			expectReason: "test",
			wantErr:      false,
		},
		{
			name:         "空字符串",
			retVal:       "",
			expectScore:  nil,
			expectReason: "",
			wantErr:      false,
		},
		{
			name:         "无效JSON",
			retVal:       `{invalid json}`,
			expectScore:  nil,
			expectReason: "",
			wantErr:      true,
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
				if tt.expectScore != nil {
					assert.NotNil(t, score)
					assert.Equal(t, *tt.expectScore, *score)
				} else {
					assert.Nil(t, score)
				}
				assert.Equal(t, tt.expectReason, reason)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_processStdoutAndStderr 测试 processStdoutAndStderr 方法
func TestEvaluatorSourceCodeServiceImpl_processStdoutAndStderr(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	tests := []struct {
		name                 string
		result               *entity.ExecutionResult
		evaluatorResult      *entity.EvaluatorResult
		expectedStdout       string
		expectedCanIgnore    bool
	}{
		{
			name: "成功解析，有stderr警告",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "normal output",
					Stderr: "warning message\nanother warning",
				},
			},
			evaluatorResult: &entity.EvaluatorResult{
				Score:     gptr.Of(1.0),
				Reasoning: "test",
			},
			expectedStdout:    "normal output\n[warning] warning message\n[warning] another warning",
			expectedCanIgnore: true,
		},
		{
			name: "成功解析，无stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "normal output",
					Stderr: "",
				},
			},
			evaluatorResult: &entity.EvaluatorResult{
				Score:     gptr.Of(1.0),
				Reasoning: "test",
			},
			expectedStdout:    "normal output",
			expectedCanIgnore: true,
		},
		{
			name: "解析失败",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "output",
					Stderr: "error",
				},
			},
			evaluatorResult:   nil,
			expectedStdout:    "",
			expectedCanIgnore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, canIgnore := service.processStdoutAndStderr(tt.result, tt.evaluatorResult)

			assert.Equal(t, tt.expectedStdout, stdout)
			assert.Equal(t, tt.expectedCanIgnore, canIgnore)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_checkExecutionErrors 测试 checkExecutionErrors 方法
func TestEvaluatorSourceCodeServiceImpl_checkExecutionErrors(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	tests := []struct {
		name            string
		result          *entity.ExecutionResult
		retValErrorMsg  string
		canIgnoreStderr bool
		expectError     bool
	}{
		{
			name: "无错误",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "success",
					Stderr: "",
				},
			},
			retValErrorMsg:  "",
			canIgnoreStderr: false,
			expectError:     false,
		},
		{
			name: "可忽略stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "success",
					Stderr: "warning",
				},
			},
			retValErrorMsg:  "",
			canIgnoreStderr: true,
			expectError:     false,
		},
		{
			name: "有retVal错误",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "",
					Stderr: "",
				},
			},
			retValErrorMsg:  "execution failed",
			canIgnoreStderr: false,
			expectError:     true,
		},
		{
			name: "有stderr错误",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "",
					Stderr: "runtime error",
				},
			},
			retValErrorMsg:  "",
			canIgnoreStderr: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := service.checkExecutionErrors(tt.result, tt.retValErrorMsg, tt.canIgnoreStderr)

			if tt.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_buildSimplePythonSyntaxCheckCode 测试 buildSimplePythonSyntaxCheckCode 方法
func TestEvaluatorSourceCodeServiceImpl_buildSimplePythonSyntaxCheckCode(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}

	tests := []struct {
		name     string
		userCode string
		contains []string
	}{
		{
			name:     "简单Python代码",
			userCode: "print('hello')",
			contains: []string{"import ast", "check_syntax", "print(json.dumps(result))"},
		},
		{
			name:     "包含特殊字符的代码",
			userCode: `print("hello world")`,
			contains: []string{"import ast", "check_syntax"},
		},
		{
			name:     "多行代码",
			userCode: "def test():\n    return 'test'",
			contains: []string{"import ast", "check_syntax"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := service.buildSimplePythonSyntaxCheckCode(tt.userCode)

			assert.NotEmpty(t, result)
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
			// 检查用户代码是否被包含（可能被转义）
			assert.True(t, strings.Contains(result, tt.userCode) || strings.Contains(result, strings.ReplaceAll(tt.userCode, `"`, `\"`)))
		})
	}
}