package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeValidator_Validate(t *testing.T) {
	validator := NewCodeValidator()

	tests := []struct {
		name     string
		code     string
		language string
		wantErr  bool
	}{
		{
			name:     "安全的Python代码",
			code:     "x = 1 + 1\nprint(x)",
			language: "python",
			wantErr:  false,
		},
		{
			name:     "安全的JavaScript代码",
			code:     "const x = 1 + 1; console.log(x);",
			language: "javascript",
			wantErr:  false,
		},
		{
			name:     "Python危险导入 - os",
			code:     "import os\nos.system('ls')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险导入 - subprocess",
			code:     "import subprocess\nsubprocess.call(['ls'])",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险函数 - exec",
			code:     "exec('print(\"hello\")')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险函数 - eval",
			code:     "eval('1+1')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "JavaScript危险函数 - eval",
			code:     "eval('console.log(\"test\")')",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript危险导入 - fs",
			code:     "import fs from 'fs'; fs.readFileSync('/etc/passwd');",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript require危险模块",
			code:     "const fs = require('fs');",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "空代码",
			code:     "",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "只有空白字符",
			code:     "   \n\t  ",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python无限循环",
			code:     "while True:\n    pass",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "JavaScript无限循环",
			code:     "while(true) { console.log('loop'); }",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "Python from import危险模块",
			code:     "from os import system\nsystem('ls')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "TypeScript危险函数",
			code:     "eval('console.log(\"test\")');",
			language: "typescript",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.code, tt.language)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCodeValidator_ValidateCodeLength(t *testing.T) {
	validator := NewCodeValidator()

	tests := []struct {
		name      string
		code      string
		maxLength int
		wantErr   bool
	}{
		{
			name:      "正常长度",
			code:      "print('hello')",
			maxLength: 100,
			wantErr:   false,
		},
		{
			name:      "超过长度限制",
			code:      "print('hello world')",
			maxLength: 10,
			wantErr:   true,
		},
		{
			name:      "边界情况 - 等于最大长度",
			code:      "hello",
			maxLength: 5,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateCodeLength(tt.code, tt.maxLength)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCodeValidator_SanitizeCode(t *testing.T) {
	validator := NewCodeValidator()

	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "移除Python注释",
			code:     "print('hello')  # 这是注释",
			expected: "print('hello')",
		},
		{
			name:     "移除JavaScript注释",
			code:     "console.log('hello'); // 这是注释",
			expected: "console.log('hello');",
		},
		{
			name:     "移除多行注释",
			code:     "console.log('hello'); /* 这是多行注释 */",
			expected: "console.log('hello');",
		},
		{
			name:     "移除前后空白",
			code:     "  \n  print('hello')  \n  ",
			expected: "print('hello')",
		},
		{
			name:     "复合情况",
			code:     "  print('hello')  # 注释\n  ",
			expected: "print('hello')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.SanitizeCode(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}
