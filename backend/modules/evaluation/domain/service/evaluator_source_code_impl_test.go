// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

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

// TestEvaluatorSourceCodeServiceImpl_EvaluatorType 测试 EvaluatorType 方法
func TestEvaluatorSourceCodeServiceImpl_EvaluatorType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
	mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
	mockMetrics := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

	// 创建被测服务
	service := NewEvaluatorSourceCodeServiceImpl(
		mockRuntimeManager,
		mockCodeBuilderFactory,
		mockMetrics,
	)

	result := service.EvaluatorType()
	assert.Equal(t, entity.EvaluatorTypeCode, result)
}

// TestEvaluatorSourceCodeServiceImpl_PreHandle 测试 PreHandle 方法
func TestEvaluatorSourceCodeServiceImpl_PreHandle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
	mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
	mockMetrics := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)

	// 创建被测服务
	service := NewEvaluatorSourceCodeServiceImpl(
		mockRuntimeManager,
		mockCodeBuilderFactory,
		mockMetrics,
	)

	tests := []struct {
		name      string
		evaluator *entity.Evaluator
		wantErr   bool
		errCode   int32
	}{
		{
			name: "预处理成功",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "test code",
				},
			},
			wantErr: false,
		},
		{
			name: "评估器类型无效",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			wantErr: true,
			errCode: errno.InvalidEvaluatorTypeCode,
		},
		{
			name: "CodeEvaluatorVersion为空",
			evaluator: &entity.Evaluator{
				ID:                   1,
				SpaceID:              123,
				Name:                 "test_evaluator",
				EvaluatorType:        entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			wantErr: true,
			errCode: errno.InvalidEvaluatorTypeCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := service.PreHandle(context.Background(), tt.evaluator)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.errCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_Validate 测试 Validate 方法
func TestEvaluatorSourceCodeServiceImpl_Validate(t *testing.T) {
	tests := []struct {
		name      string
		evaluator *entity.Evaluator
		mockSetup func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime)
		wantErr   bool
		errCode   int32
	}{
		{
			name: "Python代码验证成功",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation(turn, user_input, model_output, model_config, evaluator_config):\n    return {'score': 1.0, 'reason': 'test'}",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypePython).
					Return(mockCodeBuilder, nil)

				mockCodeBuilder.EXPECT().
					BuildSyntaxCheckCode(gomock.Any()).
					Return("syntax_check_code")

				mockRuntimeManager.EXPECT().
					GetRuntime(entity.LanguageTypePython).
					Return(mockRuntime, nil)

				mockRuntime.EXPECT().
					RunCode(gomock.Any(), "syntax_check_code", "python", int64(10000), gomock.Any()).
					Return(&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"valid": true}`,
							Stdout: "",
							Stderr: "",
						},
					}, nil)
			},
			wantErr: false,
		},
		{
			name: "JavaScript代码验证成功",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypeJS,
					CodeContent:  "function execEvaluation(turn, userInput, modelOutput, modelConfig, evaluatorConfig) {\n    return {score: 1.0, reason: 'test'};\n}",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypeJS).
					Return(mockCodeBuilder, nil)

				mockCodeBuilder.EXPECT().
					BuildSyntaxCheckCode(gomock.Any()).
					Return("syntax_check_code")

				mockRuntimeManager.EXPECT().
					GetRuntime(entity.LanguageTypeJS).
					Return(mockRuntime, nil)

				mockRuntime.EXPECT().
					RunCode(gomock.Any(), "syntax_check_code", "js", int64(10000), gomock.Any()).
					Return(&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"valid": true}`,
							Stdout: "",
							Stderr: "",
						},
					}, nil)
			},
			wantErr: false,
		},
		{
			name: "评估器类型无效",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {},
			wantErr:   true,
			errCode:   errno.InvalidEvaluatorConfigurationCode,
		},
		{
			name: "CodeEvaluatorVersion为空",
			evaluator: &entity.Evaluator{
				ID:                   1,
				SpaceID:              123,
				Name:                 "test_evaluator",
				EvaluatorType:        entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {},
			wantErr:   true,
			errCode:   errno.InvalidEvaluatorConfigurationCode,
		},
		{
			name: "代码为空",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {},
			wantErr:   true,
			errCode:   errno.InvalidCodeContentCode,
		},
		{
			name: "包含恶意模式 - Python while True",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation(turn, user_input, model_output, model_config, evaluator_config):\n    while True:\n        pass\n    return {'score': 1.0, 'reason': 'test'}",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {},
			wantErr:   true,
		},
		{
			name: "包含恶意模式 - JavaScript while(true)",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypeJS,
					CodeContent:  "function execEvaluation(turn, userInput, modelOutput, modelConfig, evaluatorConfig) {\n    while(true) {}\n    return {score: 1.0, reason: 'test'};\n}",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {},
			wantErr:   true,
		},
		{
			name: "缺少exec_evaluation函数",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def other_function():\n    return 1",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {},
			wantErr:   true,
			errCode:   errno.RequiredFunctionNotFoundCode,
		},
		{
			name: "语法验证失败",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation(turn, user_input, model_output, model_config, evaluator_config):\n    return {'score': 1.0, 'reason': 'test'",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypePython).
					Return(mockCodeBuilder, nil)

				mockCodeBuilder.EXPECT().
					BuildSyntaxCheckCode(gomock.Any()).
					Return("syntax_check_code")

				mockRuntimeManager.EXPECT().
					GetRuntime(entity.LanguageTypePython).
					Return(mockRuntime, nil)

				mockRuntime.EXPECT().
					RunCode(gomock.Any(), "syntax_check_code", "python", int64(10000), gomock.Any()).
					Return(&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"valid": false, "error": "SyntaxError: invalid syntax"}`,
							Stdout: "",
							Stderr: "",
						},
					}, nil)
			},
			wantErr: true,
			errCode: errno.SyntaxValidationFailedCode,
		},
		{
			name: "获取Runtime失败",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation(turn, user_input, model_output, model_config, evaluator_config):\n    return {'score': 1.0, 'reason': 'test'}",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
				mockRuntimeManager.EXPECT().
					GetRuntime(entity.LanguageTypePython).
					Return(nil, errors.New("runtime not found"))
			},
			wantErr: true,
			errCode: errno.RuntimeGetFailedCode,
		},
		{
			name: "不支持的语言类型",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageType("unsupported"),
					CodeContent:  "some code",
				},
			},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {},
			wantErr:   true,
			errCode:   errno.InvalidLanguageTypeCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 创建 mock 对象
			mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
			mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
			mockMetrics := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
			mockRuntime := componentmocks.NewMockIRuntime(ctrl)

			// 创建被测服务
			service := NewEvaluatorSourceCodeServiceImpl(
				mockRuntimeManager,
				mockCodeBuilderFactory,
				mockMetrics,
			)

			tt.mockSetup(ctrl, mockRuntimeManager, mockCodeBuilderFactory, mockRuntime)

			err := service.Validate(context.Background(), tt.evaluator)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.errCode, statusErr.Code())
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_Debug 测试 Debug 方法
func TestEvaluatorSourceCodeServiceImpl_Debug(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockRuntimeManager := componentmocks.NewMockIRuntimeManager(ctrl)
	mockCodeBuilderFactory := NewMockCodeBuilderFactory(ctrl)
	mockMetrics := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
	mockRuntime := componentmocks.NewMockIRuntime(ctrl)
	mockCodeBuilder := NewMockUserCodeBuilder(ctrl)

	// 创建被测服务
	service := NewEvaluatorSourceCodeServiceImpl(
		mockRuntimeManager,
		mockCodeBuilderFactory,
		mockMetrics,
	)

	tests := []struct {
		name      string
		evaluator *entity.Evaluator
		input     *entity.EvaluatorInputData
		mockSetup func()
		wantErr   bool
		wantScore *float64
	}{
		{
			name: "成功调试",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "test code",
				},
			},
			input: &entity.EvaluatorInputData{},
			mockSetup: func() {
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypePython).
					Return(mockCodeBuilder, nil)

				mockCodeBuilder.EXPECT().
					BuildCode(gomock.Any(), gomock.Any()).
					Return("built_code", nil)

				mockRuntimeManager.EXPECT().
					GetRuntime(entity.LanguageTypePython).
					Return(mockRuntime, nil)

				mockRuntime.EXPECT().
					RunCode(gomock.Any(), "built_code", "Python", gomock.Any(), gomock.Any()).
					Return(&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"score": 0.7, "reason": "Debug result"}`,
							Stdout: "Debug output",
							Stderr: "",
						},
					}, nil)
			},
			wantErr:   false,
			wantScore: gptr.Of(0.7),
		},
		{
			name: "调试失败 - CodeBuilder创建失败",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "invalid code",
				},
			},
			input: &entity.EvaluatorInputData{},
			mockSetup: func() {
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypePython).
					Return(nil, errors.New("create builder failed"))
			},
			wantErr: true,
		},
		{
			name: "评估器类型错误",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			input:     &entity.EvaluatorInputData{},
			mockSetup: func() {},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			output, err := service.Debug(context.Background(), tt.evaluator, tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if output != nil && tt.wantScore != nil {
					assert.NotNil(t, output.EvaluatorResult)
					if output.EvaluatorResult != nil {
						assert.NotNil(t, output.EvaluatorResult.Score)
						assert.Equal(t, *tt.wantScore, *output.EvaluatorResult.Score)
					}
				}
			}
		})
	}
}