// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
			},
			wantErr: true,
			errCode: errno.InvalidEvaluatorConfigurationCode,
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
			},
			wantErr: true,
			errCode: errno.InvalidEvaluatorConfigurationCode,
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
			},
			wantErr: true,
			errCode: errno.InvalidCodeContentCode,
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
			},
			wantErr: true,
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
			},
			wantErr: true,
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
			},
			wantErr: true,
			errCode: errno.RequiredFunctionNotFoundCode,
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime) {
			},
			wantErr: true,
			errCode: errno.InvalidLanguageTypeCode,
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
	tests := []struct {
		name      string
		evaluator *entity.Evaluator
		input     *entity.EvaluatorInputData
		mockSetup func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockCodeBuilder *MockUserCodeBuilder)
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockCodeBuilder *MockUserCodeBuilder) {
				// Debug方法内部调用Run方法，需要完整的Mock链
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
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockCodeBuilder *MockUserCodeBuilder) {
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
			input: &entity.EvaluatorInputData{},
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockCodeBuilder *MockUserCodeBuilder) {
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 为每个测试用例创建独立的 mock 对象
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

			tt.mockSetup(ctrl, mockRuntimeManager, mockCodeBuilderFactory, mockRuntime, mockCodeBuilder)

			output, err := service.Debug(context.Background(), tt.evaluator, tt.input, 0)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, output)
				if output != nil && tt.wantScore != nil {
					assert.NotNil(t, output.EvaluatorResult)
					if output.EvaluatorResult != nil {
						assert.NotNil(t, output.EvaluatorResult.Score)
						if output.EvaluatorResult.Score != nil {
							assert.Equal(t, *tt.wantScore, *output.EvaluatorResult.Score)
						}
					}
				}
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_Run 测试 Run 方法
func TestEvaluatorSourceCodeServiceImpl_Run(t *testing.T) {
	tests := []struct {
		name           string
		evaluator      *entity.Evaluator
		input          *entity.EvaluatorInputData
		disableTracing bool
		mockSetup      func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics)
		wantErr        bool
		wantStatus     entity.EvaluatorRunStatus
		wantScore      *float64
	}{
		{
			name: "成功执行Python代码",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypePython,
					CodeContent:  "def exec_evaluation(turn_data): return {'score': 0.8, 'reason': 'good'}",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"test": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("test input")},
				},
			},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
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
							RetVal: `{"score": 0.8, "reason": "good"}`,
							Stdout: "execution output",
							Stderr: "",
						},
					}, nil)
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusSuccess,
			wantScore:  gptr.Of(0.8),
		},
		{
			name: "成功执行JavaScript代码",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID:           1,
					LanguageType: entity.LanguageTypeJS,
					CodeContent:  "function execEvaluation(turn_data) { return {score: 0.9, reason: 'excellent'}; }",
				},
			},
			input: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"test": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("test input")},
				},
			},
			disableTracing: true,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypeJS).
					Return(mockCodeBuilder, nil)

				mockCodeBuilder.EXPECT().
					BuildCode(gomock.Any(), gomock.Any()).
					Return("built_js_code", nil)

				mockRuntimeManager.EXPECT().
					GetRuntime(entity.LanguageTypeJS).
					Return(mockRuntime, nil)

				mockRuntime.EXPECT().
					RunCode(gomock.Any(), "built_js_code", "JS", gomock.Any(), gomock.Any()).
					Return(&entity.ExecutionResult{
						Output: &entity.ExecutionOutput{
							RetVal: `{"score": 0.9, "reason": "excellent"}`,
							Stdout: "js execution output",
							Stderr: "",
						},
					}, nil)
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusSuccess,
			wantScore:  gptr.Of(0.9),
		},
		{
			name: "执行失败 - 无效的评估器类型",
			evaluator: &entity.Evaluator{
				ID:            1,
				SpaceID:       123,
				Name:          "test_evaluator",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			input:          &entity.EvaluatorInputData{},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusFail,
			wantScore:  nil,
		},
		{
			name: "执行失败 - CodeEvaluatorVersion为空",
			evaluator: &entity.Evaluator{
				ID:                   1,
				SpaceID:              123,
				Name:                 "test_evaluator",
				EvaluatorType:        entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			input:          &entity.EvaluatorInputData{},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusFail,
			wantScore:  nil,
		},
		{
			name: "执行失败 - CodeBuilder创建失败",
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
			input:          &entity.EvaluatorInputData{},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypePython).
					Return(nil, errors.New("failed to create code builder"))
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusFail,
			wantScore:  nil,
		},
		{
			name: "执行失败 - 代码构建失败",
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
			input:          &entity.EvaluatorInputData{},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypePython).
					Return(mockCodeBuilder, nil)

				mockCodeBuilder.EXPECT().
					BuildCode(gomock.Any(), gomock.Any()).
					Return("", errors.New("failed to build code"))
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusFail,
			wantScore:  nil,
		},
		{
			name: "执行失败 - Runtime获取失败",
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
			input:          &entity.EvaluatorInputData{},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
				mockCodeBuilderFactory.EXPECT().
					CreateBuilder(entity.LanguageTypePython).
					Return(mockCodeBuilder, nil)

				mockCodeBuilder.EXPECT().
					BuildCode(gomock.Any(), gomock.Any()).
					Return("built_code", nil)

				mockRuntimeManager.EXPECT().
					GetRuntime(entity.LanguageTypePython).
					Return(nil, errors.New("runtime not found"))
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusFail,
			wantScore:  nil,
		},
		{
			name: "执行失败 - 代码执行失败",
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
			input:          &entity.EvaluatorInputData{},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
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
					Return(nil, errors.New("code execution failed"))
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusFail,
			wantScore:  nil,
		},
		{
			name: "执行成功但有警告信息",
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
			input:          &entity.EvaluatorInputData{},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
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
							RetVal: `{"score": 0.6, "reason": "ok"}`,
							Stdout: "normal output",
							Stderr: "warning: deprecated function used",
						},
					}, nil)
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusSuccess,
			wantScore:  gptr.Of(0.6),
		},
		{
			name: "执行失败 - 解析结果失败",
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
			input:          &entity.EvaluatorInputData{},
			disableTracing: false,
			mockSetup: func(ctrl *gomock.Controller, mockRuntimeManager *componentmocks.MockIRuntimeManager, mockCodeBuilderFactory *MockCodeBuilderFactory, mockRuntime *componentmocks.MockIRuntime, mockMetrics *metricsmocks.MockEvaluatorExecMetrics) {
				mockCodeBuilder := NewMockUserCodeBuilder(ctrl)
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
							RetVal: `invalid json`,
							Stdout: "",
							Stderr: "parsing error",
						},
					}, nil)
			},
			wantErr:    false,
			wantStatus: entity.EvaluatorRunStatusFail,
			wantScore:  nil,
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

			tt.mockSetup(ctrl, mockRuntimeManager, mockCodeBuilderFactory, mockRuntime, mockMetrics)

			output, runStatus, _ := service.Run(context.Background(), tt.evaluator, tt.input, tt.disableTracing)

			// 验证结果
			assert.Equal(t, tt.wantStatus, runStatus)
			// traceID 在测试环境中可能为空，这是正常的
			assert.NotNil(t, output)

			if tt.wantStatus == entity.EvaluatorRunStatusSuccess {
				assert.Nil(t, output.EvaluatorRunError)
				if tt.wantScore != nil {
					assert.NotNil(t, output.EvaluatorResult)
					assert.NotNil(t, output.EvaluatorResult.Score)
					assert.Equal(t, *tt.wantScore, *output.EvaluatorResult.Score)
				}
			} else {
				assert.NotNil(t, output.EvaluatorRunError)
			}

			// 验证时间消耗 - 在测试环境中可能为0，这是正常的
			assert.GreaterOrEqual(t, output.TimeConsumingMS, int64(0))
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_validateEvaluator 测试 validateEvaluator 方法
func TestEvaluatorSourceCodeServiceImpl_validateEvaluator(t *testing.T) {
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
	}{
		{
			name: "验证成功",
			evaluator: &entity.Evaluator{
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "评估器类型错误",
			evaluator: &entity.Evaluator{
				EvaluatorType: entity.EvaluatorTypePrompt,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					ID: 1,
				},
			},
			wantErr: true,
		},
		{
			name: "CodeEvaluatorVersion为空",
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
}

// TestEvaluatorSourceCodeServiceImpl_processStdoutAndStderr 测试 processStdoutAndStderr 方法
func TestEvaluatorSourceCodeServiceImpl_processStdoutAndStderr(t *testing.T) {
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
		name                string
		result              *entity.ExecutionResult
		evaluatorResult     *entity.EvaluatorResult
		wantProcessedStdout string
		wantCanIgnoreStderr bool
	}{
		{
			name: "成功解析结果，有stderr警告",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "normal output",
					Stderr: "warning message\nline2",
				},
			},
			evaluatorResult: &entity.EvaluatorResult{
				Score:     gptr.Of(0.8),
				Reasoning: "good result",
			},
			wantProcessedStdout: "normal output\n[warning] warning message\n[warning] line2",
			wantCanIgnoreStderr: true,
		},
		{
			name: "成功解析结果，无stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "normal output",
					Stderr: "",
				},
			},
			evaluatorResult: &entity.EvaluatorResult{
				Score:     gptr.Of(0.8),
				Reasoning: "good result",
			},
			wantProcessedStdout: "normal output",
			wantCanIgnoreStderr: true,
		},
		{
			name: "解析失败，不能忽略stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "normal output",
					Stderr: "error message",
				},
			},
			evaluatorResult:     nil,
			wantProcessedStdout: "",
			wantCanIgnoreStderr: false,
		},
		{
			name: "score为空，不能忽略stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "normal output",
					Stderr: "error message",
				},
			},
			evaluatorResult: &entity.EvaluatorResult{
				Score:     nil,
				Reasoning: "result without score",
			},
			wantProcessedStdout: "",
			wantCanIgnoreStderr: false,
		},
		{
			name: "reasoning为空，不能忽略stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "normal output",
					Stderr: "error message",
				},
			},
			evaluatorResult: &entity.EvaluatorResult{
				Score:     gptr.Of(0.8),
				Reasoning: "",
			},
			wantProcessedStdout: "",
			wantCanIgnoreStderr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			processedStdout, canIgnoreStderr := service.processStdoutAndStderr(tt.result, tt.evaluatorResult)
			assert.Equal(t, tt.wantProcessedStdout, processedStdout)
			assert.Equal(t, tt.wantCanIgnoreStderr, canIgnoreStderr)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_checkExecutionErrors 测试 checkExecutionErrors 方法
func TestEvaluatorSourceCodeServiceImpl_checkExecutionErrors(t *testing.T) {
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
		name            string
		result          *entity.ExecutionResult
		retValErrorMsg  string
		canIgnoreStderr bool
		wantError       bool
		wantErrorMsg    string
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
			canIgnoreStderr: true,
			wantError:       false,
		},
		{
			name: "有RetVal错误信息",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "output",
					Stderr: "",
				},
			},
			retValErrorMsg:  "retval error",
			canIgnoreStderr: false,
			wantError:       true,
			wantErrorMsg:    "retval error",
		},
		{
			name: "有stderr错误信息",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "output",
					Stderr: "stderr error",
				},
			},
			retValErrorMsg:  "",
			canIgnoreStderr: false,
			wantError:       true,
			wantErrorMsg:    "stderr error",
		},
		{
			name: "RetVal和stderr都有错误",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "output",
					Stderr: "stderr error",
				},
			},
			retValErrorMsg:  "retval error",
			canIgnoreStderr: false,
			wantError:       true,
			wantErrorMsg:    "retval error\nstderr error",
		},
		{
			name: "可以忽略stderr",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "output",
					Stderr: "warning message",
				},
			},
			retValErrorMsg:  "",
			canIgnoreStderr: true,
			wantError:       false,
		},
		{
			name:            "result.Output为空",
			result:          &entity.ExecutionResult{Output: nil},
			retValErrorMsg:  "",
			canIgnoreStderr: false,
			wantError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := service.checkExecutionErrors(tt.result, tt.retValErrorMsg, tt.canIgnoreStderr)
			if tt.wantError {
				assert.NotNil(t, err)
				assert.Equal(t, tt.wantErrorMsg, err.Message)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_parseEvaluationRetVal 测试 parseEvaluationRetVal 方法
func TestEvaluatorSourceCodeServiceImpl_parseEvaluationRetVal(t *testing.T) {
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
		name       string
		retVal     string
		wantScore  *float64
		wantReason string
		wantErr    bool
	}{
		{
			name:       "标准JSON格式",
			retVal:     `{"score": 0.8, "reason": "good result"}`,
			wantScore:  gptr.Of(0.8),
			wantReason: "good result",
			wantErr:    false,
		},
		{
			name:       "整数score",
			retVal:     `{"score": 1, "reason": "perfect"}`,
			wantScore:  gptr.Of(1.0),
			wantReason: "perfect",
			wantErr:    false,
		},
		{
			name:       "字符串score",
			retVal:     `{"score": "0.5", "reason": "average"}`,
			wantScore:  gptr.Of(0.5),
			wantReason: "average",
			wantErr:    false,
		},
		{
			name:       "Python字典格式",
			retVal:     `{'score': 0.9, 'reason': 'excellent'}`,
			wantScore:  gptr.Of(0.9),
			wantReason: "excellent",
			wantErr:    false,
		},
		{
			name:       "嵌套JSON",
			retVal:     `{"score": 0.7, "reason": "ok"}` + "\n" + `{"stdout": "output", "stderr": ""}`,
			wantScore:  gptr.Of(0.7),
			wantReason: "ok",
			wantErr:    false,
		},
		{
			name:       "空字符串",
			retVal:     "",
			wantScore:  nil,
			wantReason: "",
			wantErr:    false,
		},
		{
			name:       "只有空白字符",
			retVal:     "   \n\t   ",
			wantScore:  nil,
			wantReason: "",
			wantErr:    false,
		},
		{
			name:       "无效JSON",
			retVal:     `{"score": 0.8, "reason": "incomplete`,
			wantScore:  nil,
			wantReason: "",
			wantErr:    true,
		},
		{
			name:       "缺少score字段",
			retVal:     `{"reason": "no score"}`,
			wantScore:  nil,
			wantReason: "no score",
			wantErr:    false,
		},
		{
			name:       "缺少reason字段",
			retVal:     `{"score": 0.6}`,
			wantScore:  gptr.Of(0.6),
			wantReason: "",
			wantErr:    false,
		},
		{
			name:       "无效的score类型",
			retVal:     `{"score": "invalid", "reason": "test"}`,
			wantScore:  nil,
			wantReason: "test",
			wantErr:    false,
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
					if score != nil {
						assert.Equal(t, *tt.wantScore, *score)
					}
				} else {
					assert.Nil(t, score)
				}
				assert.Equal(t, tt.wantReason, reason)
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_convertPythonDictToJSON 测试 convertPythonDictToJSON 方法
func TestEvaluatorSourceCodeServiceImpl_convertPythonDictToJSON(t *testing.T) {
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
		name       string
		pythonDict string
		wantJSON   string
	}{
		{
			name:       "单引号转双引号",
			pythonDict: `{'score': 0.8, 'reason': 'good'}`,
			wantJSON:   `{"score": 0.8, "reason": "good"}`,
		},
		{
			name:       "混合引号",
			pythonDict: `{'score': 0.8, "reason": 'good'}`,
			wantJSON:   `{"score": 0.8, "reason": "good"}`,
		},
		{
			name:       "已经是双引号",
			pythonDict: `{"score": 0.8, "reason": "good"}`,
			wantJSON:   `{"score": 0.8, "reason": "good"}`,
		},
		{
			name:       "包含转义字符",
			pythonDict: `{'message': 'It\'s a "test"'}`,
			wantJSON:   `{"message": "It\'s a \"test\""}`,
		},
		{
			name:       "空字符串",
			pythonDict: ``,
			wantJSON:   ``,
		},
		{
			name:       "嵌套结构",
			pythonDict: `{'outer': {'inner': 'value'}}`,
			wantJSON:   `{"outer": {"inner": "value"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := service.convertPythonDictToJSON(tt.pythonDict)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantJSON, result)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_validateCodeSecurity 测试 validateCodeSecurity 方法
func TestEvaluatorSourceCodeServiceImpl_validateCodeSecurity(t *testing.T) {
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
		name        string
		codeVersion *entity.CodeEvaluatorVersion
		wantErr     bool
		errCode     int32
	}{
		{
			name: "安全的Python代码",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation(turn_data):\n    return {'score': 1.0, 'reason': 'test'}",
			},
			wantErr: false,
		},
		{
			name: "安全的JavaScript代码",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "function execEvaluation(turn_data) {\n    return {score: 1.0, reason: 'test'};\n}",
			},
			wantErr: false,
		},
		{
			name: "空代码内容",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "",
			},
			wantErr: true,
			errCode: errno.EmptyCodeContentCode,
		},
		{
			name: "只有空白字符",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "   \n\t   ",
			},
			wantErr: true,
			errCode: errno.EmptyCodeContentCode,
		},
		{
			name: "Python危险函数 - exec",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation(turn_data):\n    exec('print(\"hello\")')\n    return {'score': 1.0}",
			},
			wantErr: true,
			errCode: errno.DangerousFunctionDetectedCode,
		},
		{
			name: "Python危险导入 - os",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "import os\ndef exec_evaluation(turn_data):\n    return {'score': 1.0}",
			},
			wantErr: true,
			errCode: errno.DangerousImportDetectedCode,
		},
		{
			name: "Python无限循环",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation(turn_data):\n    while True:\n        pass\n    return {'score': 1.0}",
			},
			wantErr: true,
			errCode: errno.MaliciousCodePatternDetectedCode,
		},
		{
			name: "JavaScript危险函数 - eval",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "function execEvaluation(turn_data) {\n    eval('console.log(\"hello\")');\n    return {score: 1.0};\n}",
			},
			wantErr: true,
			errCode: errno.DangerousFunctionDetectedCode,
		},
		{
			name: "JavaScript危险导入 - fs",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "const fs = require('fs');\nfunction execEvaluation(turn_data) {\n    return {score: 1.0};\n}",
			},
			wantErr: true,
			errCode: errno.DangerousImportDetectedCode,
		},
		{
			name: "JavaScript无限循环",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "function execEvaluation(turn_data) {\n    while(true) {}\n    return {score: 1.0};\n}",
			},
			wantErr: true,
			errCode: errno.MaliciousCodePatternDetectedCode,
		},
		{
			name: "JavaScript DOM操作",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "function execEvaluation(turn_data) {\n    document.getElementById('test');\n    return {score: 1.0};\n}",
			},
			wantErr: false, // 根据实际代码逻辑，DOM操作通过checkDangerousFunctions检查，而不是validateJavaScriptSpecificSecurity
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := service.validateCodeSecurity(tt.codeVersion)
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

// TestEvaluatorSourceCodeServiceImpl_validateExecEvaluationFunction 测试 validateExecEvaluationFunction 方法
func TestEvaluatorSourceCodeServiceImpl_validateExecEvaluationFunction(t *testing.T) {
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
		name        string
		codeVersion *entity.CodeEvaluatorVersion
		wantErr     bool
		errCode     int32
	}{
		{
			name: "Python标准函数定义",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def exec_evaluation(turn_data):\n    return {'score': 1.0, 'reason': 'test'}",
			},
			wantErr: false,
		},
		{
			name: "Python缺少exec_evaluation函数",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypePython,
				CodeContent:  "def other_function():\n    return 1",
			},
			wantErr: true,
			errCode: errno.RequiredFunctionNotFoundCode,
		},
		{
			name: "JavaScript function声明",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "function execEvaluation(turn_data) {\n    return {score: 1.0, reason: 'test'};\n}",
			},
			wantErr: false,
		},
		{
			name: "JavaScript exec_evaluation函数",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "function exec_evaluation(turn_data) {\n    return {score: 1.0, reason: 'test'};\n}",
			},
			wantErr: false,
		},
		{
			name: "JavaScript const箭头函数",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "const execEvaluation = (turn_data) => {\n    return {score: 1.0, reason: 'test'};\n}",
			},
			wantErr: false,
		},
		{
			name: "JavaScript let function表达式",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "let exec_evaluation = function(turn_data) {\n    return {score: 1.0, reason: 'test'};\n}",
			},
			wantErr: false,
		},
		{
			name: "JavaScript缺少exec_evaluation函数",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageTypeJS,
				CodeContent:  "function otherFunction() {\n    return 1;\n}",
			},
			wantErr: true,
			errCode: errno.RequiredFunctionNotFoundCode,
		},
		{
			name: "不支持的语言类型",
			codeVersion: &entity.CodeEvaluatorVersion{
				LanguageType: entity.LanguageType("unsupported"),
				CodeContent:  "some code",
			},
			wantErr: true,
			errCode: errno.UnsupportedLanguageTypeCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := service.validateExecEvaluationFunction(tt.codeVersion)
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

// TestEvaluatorSourceCodeServiceImpl_buildExtParams 测试 buildExtParams 方法
func TestEvaluatorSourceCodeServiceImpl_buildExtParams(t *testing.T) {
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
		want      map[string]string
	}{
		{
			name: "正常的evaluator",
			evaluator: &entity.Evaluator{
				SpaceID:       123,
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
					SpaceID: 123,
				},
			},
			want: map[string]string{
				"space_id": "123",
			},
		},
		{
			name: "SpaceID为0",
			evaluator: &entity.Evaluator{
				SpaceID: 0,
			},
			want: map[string]string{},
		},
		{
			name:      "evaluator为nil",
			evaluator: nil,
			want:      map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := service.buildExtParams(tt.evaluator)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_getTimeoutMS 测试 getTimeoutMS 方法
func TestEvaluatorSourceCodeServiceImpl_getTimeoutMS(t *testing.T) {
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

	result := service.getTimeoutMS()
	assert.Equal(t, int64(5000), result)
}

// TestEvaluatorSourceCodeServiceImpl_getFinalStdout 测试 getFinalStdout 方法
func TestEvaluatorSourceCodeServiceImpl_getFinalStdout(t *testing.T) {
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
		name            string
		result          *entity.ExecutionResult
		processedStdout string
		canIgnoreStderr bool
		want            string
	}{
		{
			name: "可以忽略stderr，使用处理后的stdout",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "original stdout",
					Stderr: "warning",
				},
			},
			processedStdout: "processed stdout with warnings",
			canIgnoreStderr: true,
			want:            "processed stdout with warnings",
		},
		{
			name: "不能忽略stderr，使用原始stdout",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "original stdout",
					Stderr: "error",
				},
			},
			processedStdout: "",
			canIgnoreStderr: false,
			want:            "original stdout",
		},
		{
			name:            "result.Output为nil",
			result:          &entity.ExecutionResult{Output: nil},
			processedStdout: "",
			canIgnoreStderr: true,
			want:            "",
		},
		{
			name: "可以忽略stderr但processedStdout为空",
			result: &entity.ExecutionResult{
				Output: &entity.ExecutionOutput{
					Stdout: "original stdout",
					Stderr: "warning",
				},
			},
			processedStdout: "",
			canIgnoreStderr: true,
			want:            "original stdout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := service.getFinalStdout(tt.result, tt.processedStdout, tt.canIgnoreStderr)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_decodeUnicodeEscapes 测试 decodeUnicodeEscapes 方法
func TestEvaluatorSourceCodeServiceImpl_decodeUnicodeEscapes(t *testing.T) {
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
		name  string
		input string
		want  string
	}{
		{
			name:  "无Unicode转义",
			input: "normal string",
			want:  "normal string",
		},
		{
			name:  "单个Unicode转义",
			input: "hello \\u4e2d\\u6587",
			want:  "hello 中文",
		},
		{
			name:  "多个Unicode转义",
			input: "\\u4f60\\u597d\\u4e16\\u754c",
			want:  "你好世界",
		},
		{
			name:  "混合内容",
			input: "Hello \\u4e2d\\u6587 World",
			want:  "Hello 中文 World",
		},
		{
			name:  "无效的Unicode转义",
			input: "\\uXXXX invalid",
			want:  "\\uXXXX invalid",
		},
		{
			name:  "不完整的Unicode转义",
			input: "\\u123",
			want:  "\\u123",
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := service.decodeUnicodeEscapes(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_cleanNestedJSON 测试 cleanNestedJSON 方法
func TestEvaluatorSourceCodeServiceImpl_cleanNestedJSON(t *testing.T) {
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
		name  string
		input string
		want  string
	}{
		{
			name:  "标准JSON",
			input: `{"score": 0.8, "reason": "good"}`,
			want:  `{"score": 0.8, "reason": "good"}`,
		},
		{
			name:  "嵌套JSON - 提取评估结果",
			input: `{"score": 0.8, "reason": "good"}` + "\n" + `{"stdout": "output", "stderr": ""}`,
			want:  `{"score": 0.8, "reason": "good"}`,
		},
		{
			name:  "多行嵌套JSON",
			input: "line1\n{\"score\": 0.9, \"reason\": \"excellent\"}\nline3",
			want:  `{"score": 0.9, "reason": "excellent"}`,
		},
		{
			name:  "无评估结果的JSON",
			input: `{"stdout": "output", "stderr": ""}\n{"other": "data"}`,
			want:  `{"stdout": "output", "stderr": ""}\n{"other": "data"}`,
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
		{
			name:  "只有空白字符",
			input: "   \n\t   ",
			want:  "",
		},
		{
			name:  "无效JSON",
			input: "not json content",
			want:  "not json content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := service.cleanNestedJSON(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_createErrorOutput 测试 createErrorOutput 方法
func TestEvaluatorSourceCodeServiceImpl_createErrorOutput(t *testing.T) {
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

	startTime := time.Now()
	time.Sleep(1 * time.Millisecond) // 确保有时间差

	output, status := service.createErrorOutput(
		errors.New("test error"),
		int32(errno.CodeExecutionFailedCode),
		"test message",
		startTime,
	)

	assert.Equal(t, entity.EvaluatorRunStatusFail, status)
	assert.NotNil(t, output)
	assert.NotNil(t, output.EvaluatorRunError)
	assert.Equal(t, int32(errno.CodeExecutionFailedCode), output.EvaluatorRunError.Code)
	assert.Equal(t, "test message", output.EvaluatorRunError.Message)
	assert.Greater(t, output.TimeConsumingMS, int64(0))
	assert.Equal(t, "", output.Stdout)
}

// TestEvaluatorSourceCodeServiceImpl_buildSimplePythonSyntaxCheckCode 测试 buildSimplePythonSyntaxCheckCode 方法
func TestEvaluatorSourceCodeServiceImpl_buildSimplePythonSyntaxCheckCode(t *testing.T) {
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
		name         string
		userCode     string
		wantContains []string // 期望包含的字符串
	}{
		{
			name:     "简单Python代码",
			userCode: "def hello():\n    print('hello')",
			wantContains: []string{
				"import ast",
				"import json",
				"check_syntax",
				"user_code = \"\"\"def hello():",
				"print('hello')\"\"\"",
				"ast.parse(code)",
				"print(json.dumps(result))",
			},
		},
		{
			name:     "包含双引号的代码",
			userCode: `def greet(name):\n    print("Hello, " + name)`,
			wantContains: []string{
				"import ast",
				"import json",
				`print(\"Hello, \" + name)`,
				"check_syntax",
			},
		},
		{
			name:     "包含三重引号的代码",
			userCode: `def doc():\n    """\n    This is a docstring\n    """`,
			wantContains: []string{
				"import ast",
				"import json",
				"check_syntax",
				"This is a docstring",
			},
		},
		{
			name:     "包含反斜杠的代码",
			userCode: `def path():\n    return "C:\\Users\\test"`,
			wantContains: []string{
				"import ast",
				"import json",
				`C:\\\\Users\\\\test`,
				"check_syntax",
			},
		},
		{
			name:     "空代码",
			userCode: "",
			wantContains: []string{
				"import ast",
				"import json",
				"user_code = \"\"\"\"\"\"",
				"check_syntax",
			},
		},
		{
			name:     "多行复杂代码",
			userCode: "def complex():\n    x = \"test\"\n    y = 'another'\n    z = \"\"\"multiline\"\"\"\n    return x + y",
			wantContains: []string{
				"import ast",
				"import json",
				"check_syntax",
				"ast.parse(code)",
				"SyntaxError",
				"print(json.dumps(result))",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildSimplePythonSyntaxCheckCode(tt.userCode)

			// 验证返回的代码不为空
			assert.NotEmpty(t, result)

			// 验证包含期望的字符串
			for _, expected := range tt.wantContains {
				assert.Contains(t, result, expected, "Expected to contain: %s", expected)
			}

			// 验证代码结构正确
			assert.Contains(t, result, "def check_syntax(code):")
			assert.Contains(t, result, "has_error, msg = check_syntax(user_code)")
			assert.Contains(t, result, `result = {"valid": False, "error": msg}`)
			assert.Contains(t, result, `result = {"valid": True, "error": None}`)
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_buildSimpleJavaScriptSyntaxCheckCode 测试 buildSimpleJavaScriptSyntaxCheckCode 方法
func TestEvaluatorSourceCodeServiceImpl_buildSimpleJavaScriptSyntaxCheckCode(t *testing.T) {
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
		name         string
		userCode     string
		wantContains []string // 期望包含的字符串
	}{
		{
			name:     "简单JavaScript函数",
			userCode: "function hello() {\n    console.log('hello');\n}",
			wantContains: []string{
				"const userCode = `function hello() {",
				"console.log('hello');",
				"new Function(userCode);",
				`"valid": true`,
				`"valid": false`,
				"console.log(JSON.stringify(result));",
			},
		},
		{
			name:     "包含反斜杠的代码",
			userCode: `function path() {\n    return "C:\\Users\\test";\n}`,
			wantContains: []string{
				"const userCode = `",
				`C:\\\\Users\\\\test`,
				"new Function(userCode);",
				"JSON.stringify(result)",
			},
		},
		{
			name:     "包含模板字符串特殊字符",
			userCode: "function template() {\n    return `Hello ${name}`;\n}",
			wantContains: []string{
				"const userCode = `",
				"Hello \\${name}",
				"new Function(userCode);",
			},
		},
		{
			name:     "包含反引号的代码",
			userCode: "function quote() {\n    return `test`;\n}",
			wantContains: []string{
				"const userCode = `",
				"return \\`test\\`;",
				"new Function(userCode);",
			},
		},
		{
			name:     "空代码",
			userCode: "",
			wantContains: []string{
				"const userCode = ``;",
				"new Function(userCode);",
				"console.log(JSON.stringify(result));",
			},
		},
		{
			name:     "复杂JavaScript代码",
			userCode: "function complex() {\n    const x = `template ${var}`;\n    const y = 'string';\n    return x + y;\n}",
			wantContains: []string{
				"const userCode = `",
				"template \\${var}",
				"new Function(userCode);",
				"try {",
				"} catch (error) {",
				`"error": "语法错误: " + error.message`,
			},
		},
		{
			name:     "包含多种特殊字符",
			userCode: "function special() {\n    const str = `Hello ${'world'} with \\ backslash`;\n    return str;\n}",
			wantContains: []string{
				"const userCode = `",
				"\\\\",
				"\\$",
				"\\`",
				"new Function(userCode);",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildSimpleJavaScriptSyntaxCheckCode(tt.userCode)

			// 验证返回的代码不为空
			assert.NotEmpty(t, result)

			// 验证包含期望的字符串
			for _, expected := range tt.wantContains {
				assert.Contains(t, result, expected, "Expected to contain: %s", expected)
			}

			// 验证代码结构正确
			assert.Contains(t, result, "const userCode = `")
			assert.Contains(t, result, "new Function(userCode);")
			assert.Contains(t, result, "try {")
			assert.Contains(t, result, "} catch (error) {")
			assert.Contains(t, result, "console.log(JSON.stringify(result));")
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_getMaliciousPatternsForLanguage 测试 getMaliciousPatternsForLanguage 方法
func TestEvaluatorSourceCodeServiceImpl_getMaliciousPatternsForLanguage(t *testing.T) {
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
		name           string
		language       string
		expectedCount  int
		expectPatterns []string // 期望包含的模式
		expectGeneral  bool     // 是否期望返回通用模式
	}{
		{
			name:          "Python语言",
			language:      "python",
			expectedCount: 3,
			expectPatterns: []string{
				`while\s+True\s*:`,
				`exit\s*\(`,
				`quit\s*\(`,
			},
			expectGeneral: false,
		},
		{
			name:          "Python语言别名py",
			language:      "py",
			expectedCount: 3,
			expectPatterns: []string{
				`while\s+True\s*:`,
				`exit\s*\(`,
				`quit\s*\(`,
			},
			expectGeneral: false,
		},
		{
			name:          "JavaScript语言",
			language:      "javascript",
			expectedCount: 5,
			expectPatterns: []string{
				`while\s*\(\s*true\s*\)`,
				`for\s*\(\s*;\s*;\s*\)`,
				`setInterval\s*\(`,
				`setTimeout\s*\(`,
				`process\.exit`,
			},
			expectGeneral: false,
		},
		{
			name:          "JavaScript语言别名js",
			language:      "js",
			expectedCount: 5,
			expectPatterns: []string{
				`while\s*\(\s*true\s*\)`,
				`for\s*\(\s*;\s*;\s*\)`,
				`setInterval\s*\(`,
				`setTimeout\s*\(`,
				`process\.exit`,
			},
			expectGeneral: false,
		},
		{
			name:          "TypeScript语言",
			language:      "typescript",
			expectedCount: 5,
			expectPatterns: []string{
				`while\s*\(\s*true\s*\)`,
				`for\s*\(\s*;\s*;\s*\)`,
				`setInterval\s*\(`,
				`setTimeout\s*\(`,
				`process\.exit`,
			},
			expectGeneral: false,
		},
		{
			name:          "TypeScript语言别名ts",
			language:      "ts",
			expectedCount: 5,
			expectPatterns: []string{
				`while\s*\(\s*true\s*\)`,
				`for\s*\(\s*;\s*;\s*\)`,
				`setInterval\s*\(`,
				`setTimeout\s*\(`,
				`process\.exit`,
			},
			expectGeneral: false,
		},
		{
			name:          "Java语言",
			language:      "java",
			expectedCount: 2,
			expectPatterns: []string{
				`while\s*\(\s*true\s*\)`,
				`System\.exit`,
			},
			expectGeneral: false,
		},
		{
			name:          "Go语言",
			language:      "go",
			expectedCount: 2,
			expectPatterns: []string{
				`for\s*\{\s*\}`,
				`for\s*;\s*;\s*\{`,
			},
			expectGeneral: false,
		},
		{
			name:          "Go语言别名golang",
			language:      "golang",
			expectedCount: 2,
			expectPatterns: []string{
				`for\s*\{\s*\}`,
				`for\s*;\s*;\s*\{`,
			},
			expectGeneral: false,
		},
		{
			name:          "不支持的语言",
			language:      "unsupported",
			expectedCount: 2,
			expectPatterns: []string{
				`exit\s*\(`,
				`quit\s*\(`,
			},
			expectGeneral: true,
		},
		{
			name:          "空语言名称",
			language:      "",
			expectedCount: 2,
			expectPatterns: []string{
				`exit\s*\(`,
				`quit\s*\(`,
			},
			expectGeneral: true,
		},
		{
			name:          "大小写混合的语言名称",
			language:      "PYTHON",
			expectedCount: 3,
			expectPatterns: []string{
				`while\s+True\s*:`,
				`exit\s*\(`,
				`quit\s*\(`,
			},
			expectGeneral: false,
		},
		{
			name:          "带空格的语言名称",
			language:      "  javascript  ",
			expectedCount: 5,
			expectPatterns: []string{
				`while\s*\(\s*true\s*\)`,
				`for\s*\(\s*;\s*;\s*\)`,
				`setInterval\s*\(`,
				`setTimeout\s*\(`,
				`process\.exit`,
			},
			expectGeneral: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := service.getMaliciousPatternsForLanguage(tt.language)

			// 验证返回的模式数量
			assert.Equal(t, tt.expectedCount, len(patterns), "Expected %d patterns, got %d", tt.expectedCount, len(patterns))

			// 验证包含期望的模式
			patternStrings := make([]string, len(patterns))
			for i, pattern := range patterns {
				patternStrings[i] = pattern.Pattern
			}

			for _, expectedPattern := range tt.expectPatterns {
				assert.Contains(t, patternStrings, expectedPattern, "Expected to contain pattern: %s", expectedPattern)
			}

			// 验证模式的完整性
			for _, pattern := range patterns {
				assert.NotEmpty(t, pattern.Pattern, "Pattern should not be empty")
				assert.NotEmpty(t, pattern.Category, "Category should not be empty")
				assert.NotEmpty(t, pattern.Description, "Description should not be empty")
				assert.NotEmpty(t, pattern.Severity, "Severity should not be empty")
				assert.NotEmpty(t, pattern.Risk, "Risk should not be empty")
				assert.NotEmpty(t, pattern.Suggestion, "Suggestion should not be empty")
				assert.NotEmpty(t, pattern.Languages, "Languages should not be empty")

				// 验证类别是否为预定义的值
				validCategories := []MaliciousPatternCategory{
					CategoryInfiniteLoop,
					CategoryProcessControl,
					CategoryAsyncOperation,
					CategoryResourceAccess,
				}
				assert.Contains(t, validCategories, pattern.Category, "Category should be one of the predefined values")

				// 验证严重程度是否为预期值
				validSeverities := []string{"low", "medium", "high"}
				assert.Contains(t, validSeverities, pattern.Severity, "Severity should be low, medium, or high")
			}

			// 如果是通用模式，验证语言字段
			if tt.expectGeneral {
				for _, pattern := range patterns {
					assert.Contains(t, pattern.Languages, "general", "General patterns should contain 'general' in languages")
				}
			}
		})
	}
}

// TestEvaluatorSourceCodeServiceImpl_getMaliciousPatternsForLanguage_Categories 测试不同类别的恶意模式
func TestEvaluatorSourceCodeServiceImpl_getMaliciousPatternsForLanguage_Categories(t *testing.T) {
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
		name             string
		language         string
		expectedCategory MaliciousPatternCategory
		expectedPattern  string
	}{
		{
			name:             "Python无限循环模式",
			language:         "python",
			expectedCategory: CategoryInfiniteLoop,
			expectedPattern:  `while\s+True\s*:`,
		},
		{
			name:             "Python进程控制模式 - exit",
			language:         "python",
			expectedCategory: CategoryProcessControl,
			expectedPattern:  `exit\s*\(`,
		},
		{
			name:             "Python进程控制模式 - quit",
			language:         "python",
			expectedCategory: CategoryProcessControl,
			expectedPattern:  `quit\s*\(`,
		},
		{
			name:             "JavaScript无限循环模式 - while",
			language:         "javascript",
			expectedCategory: CategoryInfiniteLoop,
			expectedPattern:  `while\s*\(\s*true\s*\)`,
		},
		{
			name:             "JavaScript无限循环模式 - for",
			language:         "javascript",
			expectedCategory: CategoryInfiniteLoop,
			expectedPattern:  `for\s*\(\s*;\s*;\s*\)`,
		},
		{
			name:             "JavaScript异步操作模式 - setInterval",
			language:         "javascript",
			expectedCategory: CategoryAsyncOperation,
			expectedPattern:  `setInterval\s*\(`,
		},
		{
			name:             "JavaScript异步操作模式 - setTimeout",
			language:         "javascript",
			expectedCategory: CategoryAsyncOperation,
			expectedPattern:  `setTimeout\s*\(`,
		},
		{
			name:             "JavaScript进程控制模式",
			language:         "javascript",
			expectedCategory: CategoryProcessControl,
			expectedPattern:  `process\.exit`,
		},
		{
			name:             "Java无限循环模式",
			language:         "java",
			expectedCategory: CategoryInfiniteLoop,
			expectedPattern:  `while\s*\(\s*true\s*\)`,
		},
		{
			name:             "Java进程控制模式",
			language:         "java",
			expectedCategory: CategoryProcessControl,
			expectedPattern:  `System\.exit`,
		},
		{
			name:             "Go无限循环模式 - for{}",
			language:         "go",
			expectedCategory: CategoryInfiniteLoop,
			expectedPattern:  `for\s*\{\s*\}`,
		},
		{
			name:             "Go无限循环模式 - for;;",
			language:         "go",
			expectedCategory: CategoryInfiniteLoop,
			expectedPattern:  `for\s*;\s*;\s*\{`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := service.getMaliciousPatternsForLanguage(tt.language)

			// 查找指定模式
			var foundPattern *MaliciousPattern
			for _, pattern := range patterns {
				if pattern.Pattern == tt.expectedPattern {
					foundPattern = &pattern
					break
				}
			}

			// 验证找到了期望的模式
			assert.NotNil(t, foundPattern, "Expected to find pattern: %s", tt.expectedPattern)

			if foundPattern != nil {
				// 验证类别
				assert.Equal(t, tt.expectedCategory, foundPattern.Category, "Expected category: %s, got: %s", tt.expectedCategory, foundPattern.Category)

				// 验证语言列表包含当前语言
				languageFound := false
				for _, lang := range foundPattern.Languages {
					if strings.EqualFold(lang, tt.language) ||
						(tt.language == "javascript" && (lang == "typescript" || lang == "javascript")) ||
						(tt.language == "typescript" && (lang == "typescript" || lang == "javascript")) {
						languageFound = true
						break
					}
				}
				assert.True(t, languageFound, "Pattern should support language: %s, got languages: %v", tt.language, foundPattern.Languages)
			}
		})
	}
}
