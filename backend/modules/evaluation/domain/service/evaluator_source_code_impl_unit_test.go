// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

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

func TestEvaluatorSourceCodeServiceImpl_EvaluatorTypeUnit(t *testing.T) {
	t.Parallel()

	service := &EvaluatorSourceCodeServiceImpl{}
	result := service.EvaluatorType()
	assert.Equal(t, entity.EvaluatorTypeCode, result)
}