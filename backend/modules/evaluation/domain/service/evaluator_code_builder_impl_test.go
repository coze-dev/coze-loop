// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// MockRuntimeManager 手动定义的RuntimeManager Mock，避免循环导入
type MockRuntimeManager struct {
	ctrl     *gomock.Controller
	recorder *MockRuntimeManagerMockRecorder
}

type MockRuntimeManagerMockRecorder struct {
	mock *MockRuntimeManager
}

func NewMockRuntimeManager(ctrl *gomock.Controller) *MockRuntimeManager {
	mock := &MockRuntimeManager{ctrl: ctrl}
	mock.recorder = &MockRuntimeManagerMockRecorder{mock}
	return mock
}

func (m *MockRuntimeManager) EXPECT() *MockRuntimeManagerMockRecorder {
	return m.recorder
}

func (m *MockRuntimeManager) GetRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRuntime", languageType)
	ret0, _ := ret[0].(component.IRuntime)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *MockRuntimeManager) GetSupportedLanguages() []entity.LanguageType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSupportedLanguages")
	ret0, _ := ret[0].([]entity.LanguageType)
	return ret0
}

func (m *MockRuntimeManager) ClearCache() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ClearCache")
}

func (mr *MockRuntimeManagerMockRecorder) GetRuntime(languageType interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRuntime", reflect.TypeOf((*MockRuntimeManager)(nil).GetRuntime), languageType)
}

func (mr *MockRuntimeManagerMockRecorder) GetSupportedLanguages() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSupportedLanguages", reflect.TypeOf((*MockRuntimeManager)(nil).GetSupportedLanguages))
}

func (mr *MockRuntimeManagerMockRecorder) ClearCache() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClearCache", reflect.TypeOf((*MockRuntimeManager)(nil).ClearCache))
}

// TestNewCodeBuilderFactory 测试构造函数
func TestNewCodeBuilderFactory(t *testing.T) {
	factory := NewCodeBuilderFactory()
	assert.NotNil(t, factory)
	assert.IsType(t, &CodeBuilderFactoryImpl{}, factory)
}

// TestCodeBuilderFactoryImpl_SetRuntimeManager 测试设置运行时管理器
func TestCodeBuilderFactoryImpl_SetRuntimeManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	factory := &CodeBuilderFactoryImpl{}
	mockRuntimeManager := NewMockRuntimeManager(ctrl)

	factory.SetRuntimeManager(mockRuntimeManager)
	assert.Equal(t, mockRuntimeManager, factory.runtimeManager)
}

// TestCodeBuilderFactoryImpl_GetSupportedLanguages 测试获取支持的语言类型列表
func TestCodeBuilderFactoryImpl_GetSupportedLanguages(t *testing.T) {
	factory := &CodeBuilderFactoryImpl{}
	languages := factory.GetSupportedLanguages()

	expectedLanguages := []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}

	assert.Equal(t, expectedLanguages, languages)
	assert.Len(t, languages, 2)
}

// TestCodeBuilderFactoryImpl_CreateBuilder 测试创建代码构建器
func TestCodeBuilderFactoryImpl_CreateBuilder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 定义测试用例结构体
	type fields struct {
		runtimeManager *MockRuntimeManager
	}
	type args struct {
		languageType entity.LanguageType
	}
	tests := []struct {
		name           string
		args           args
		prepareMock    func(t *testing.T, f *fields, args args)
		wantBuilderNil bool
		wantErr        bool
		expectedErr    error
		checkBuilder   func(t *testing.T, builder UserCodeBuilder, args args)
	}{
		{
			name: "成功创建Python代码构建器 - 有runtime管理器且获取runtime成功",
			args: args{
				languageType: entity.LanguageTypePython,
			},
			prepareMock: func(t *testing.T, f *fields, args args) {
				mockRuntime := componentMocks.NewMockIRuntime(ctrl)
				f.runtimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil).Times(1)
			},
			wantBuilderNil: false,
			wantErr:        false,
			checkBuilder: func(t *testing.T, builder UserCodeBuilder, args args) {
				assert.Equal(t, entity.LanguageTypePython, builder.GetLanguageType())
				assert.IsType(t, &PythonCodeBuilder{}, builder)
			},
		},
		{
			name: "成功创建JavaScript代码构建器 - 有runtime管理器且获取runtime成功",
			args: args{
				languageType: entity.LanguageTypeJS,
			},
			prepareMock: func(t *testing.T, f *fields, args args) {
				mockRuntime := componentMocks.NewMockIRuntime(ctrl)
				f.runtimeManager.EXPECT().GetRuntime(entity.LanguageTypeJS).Return(mockRuntime, nil).Times(1)
			},
			wantBuilderNil: false,
			wantErr:        false,
			checkBuilder: func(t *testing.T, builder UserCodeBuilder, args args) {
				assert.Equal(t, entity.LanguageTypeJS, builder.GetLanguageType())
				assert.IsType(t, &JavaScriptCodeBuilder{}, builder)
			},
		},
		{
			name: "成功创建Python代码构建器 - 有runtime管理器但获取runtime失败",
			args: args{
				languageType: entity.LanguageTypePython,
			},
			prepareMock: func(t *testing.T, f *fields, args args) {
				f.runtimeManager.EXPECT().GetRuntime(entity.LanguageTypePython).Return(nil, errors.New("runtime not found")).Times(1)
			},
			wantBuilderNil: false,
			wantErr:        false,
			checkBuilder: func(t *testing.T, builder UserCodeBuilder, args args) {
				assert.Equal(t, entity.LanguageTypePython, builder.GetLanguageType())
				assert.IsType(t, &PythonCodeBuilder{}, builder)
			},
		},
		{
			name: "成功创建Python代码构建器 - 没有runtime管理器",
			args: args{
				languageType: entity.LanguageTypePython,
			},
			prepareMock:    nil, // 不设置runtime管理器
			wantBuilderNil: false,
			wantErr:        false,
			checkBuilder: func(t *testing.T, builder UserCodeBuilder, args args) {
				assert.Equal(t, entity.LanguageTypePython, builder.GetLanguageType())
				assert.IsType(t, &PythonCodeBuilder{}, builder)
			},
		},
		{
			name: "失败 - 不支持的语言类型",
			args: args{
				languageType: entity.LanguageType("unsupported"),
			},
			prepareMock:    nil,
			wantBuilderNil: true,
			wantErr:        true,
			expectedErr:    errors.New("unsupported language type: unsupported"),
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			f := fields{}

			// 根据测试用例决定是否创建runtime管理器
			if tt.prepareMock != nil {
				f.runtimeManager = NewMockRuntimeManager(ctrl)
			}

			factory := &CodeBuilderFactoryImpl{}
			if f.runtimeManager != nil {
				factory.SetRuntimeManager(f.runtimeManager)
			}

			if tt.prepareMock != nil {
				tt.prepareMock(t, &f, tt.args)
			}

			builder, err := factory.CreateBuilder(tt.args.languageType)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.EqualError(t, err, tt.expectedErr.Error())
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.wantBuilderNil {
				assert.Nil(t, builder)
			} else {
				assert.NotNil(t, builder)
			}

			if tt.checkBuilder != nil && builder != nil {
				tt.checkBuilder(t, builder, tt.args)
			}
		})
	}
}

// TestCodeBuilderFactoryImpl_CreateBuilder_Integration 集成测试 - 测试工厂创建的构建器能正常工作
func TestCodeBuilderFactoryImpl_CreateBuilder_Integration(t *testing.T) {
	factory := NewCodeBuilderFactory()

	t.Run("Python构建器集成测试", func(t *testing.T) {
		builder, err := factory.CreateBuilder(entity.LanguageTypePython)
		assert.NoError(t, err)
		assert.NotNil(t, builder)

		// 测试构建器的基本功能
		assert.Equal(t, entity.LanguageTypePython, builder.GetLanguageType())

		// 测试语法检查代码构建
		syntaxCheckCode := builder.BuildSyntaxCheckCode("def test(): pass")
		assert.NotEmpty(t, syntaxCheckCode)
		assert.Contains(t, syntaxCheckCode, "def test(): pass")
	})

	t.Run("JavaScript构建器集成测试", func(t *testing.T) {
		builder, err := factory.CreateBuilder(entity.LanguageTypeJS)
		assert.NoError(t, err)
		assert.NotNil(t, builder)

		// 测试构建器的基本功能
		assert.Equal(t, entity.LanguageTypeJS, builder.GetLanguageType())

		// 测试语法检查代码构建
		syntaxCheckCode := builder.BuildSyntaxCheckCode("function test() {}")
		assert.NotEmpty(t, syntaxCheckCode)
		assert.Contains(t, syntaxCheckCode, "function test() {}")
	})
}