// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeValidator_Validate(t *testing.T) {
	t.Parallel()

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
		// 新增测试用例
		{
			name:     "Python危险函数 - __import__",
			code:     "__import__('os').system('rm -rf /')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险函数 - open",
			code:     "open('/etc/passwd', 'r').read()",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "JavaScript危险函数 - Function",
			code:     "new Function('return process.env')",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript危险函数 - setTimeout",
			code:     "setTimeout('malicious code', 1000)",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "Python复杂危险模式 - 嵌套导入",
			code:     "import sys\nfrom os import *\nsystem('malicious')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "JavaScript复杂危险模式 - 动态导入",
			code:     "const mod = require('child_process'); mod.exec('rm -rf /')",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "Python网络相关危险导入 - urllib",
			code:     "import urllib.request\nurllib.request.urlopen('http://evil.com')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python网络相关危险导入 - socket",
			code:     "import socket\ns = socket.socket()",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "JavaScript网络相关危险导入 - http",
			code:     "const http = require('http'); http.get('http://evil.com')",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "不支持的语言类型",
			code:     "some code",
			language: "unsupported",
			wantErr:  false, // 不支持的语言不进行检查，返回成功
		},
		{
			name:     "大小写混合的语言名称",
			code:     "eval('test')",
			language: "Python", // 大写P
			wantErr:  true,
		},
		{
			name:     "带空格的语言名称",
			code:     "eval('test')",
			language: " python ",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validator.Validate(tt.code, tt.language)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCodeValidator_checkDangerousFunctions 测试危险函数检查
func TestCodeValidator_checkDangerousFunctions(t *testing.T) {
	t.Parallel()

	validator := NewCodeValidator()

	tests := []struct {
		name     string
		code     string
		language string
		wantErr  bool
	}{
		{
			name:     "Python安全函数调用",
			code:     "print('hello')\nlen([1,2,3])",
			language: "python",
			wantErr:  false,
		},
		{
			name:     "Python危险函数 - exec",
			code:     "exec('malicious code')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险函数 - eval",
			code:     "result = eval('1+1')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险函数 - __import__",
			code:     "__import__('os')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险函数 - open",
			code:     "f = open('/etc/passwd')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "JavaScript安全函数调用",
			code:     "console.log('hello'); Math.max(1,2,3)",
			language: "javascript",
			wantErr:  false,
		},
		{
			name:     "JavaScript危险函数 - eval",
			code:     "eval('malicious code')",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript危险函数 - Function",
			code:     "new Function('return process')",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript危险函数 - setTimeout",
			code:     "setTimeout('code', 1000)",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "TypeScript危险函数",
			code:     "eval('test code')",
			language: "typescript",
			wantErr:  true,
		},
		{
			name:     "不支持的语言",
			code:     "eval('test')",
			language: "go",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validator.checkDangerousFunctions(tt.code, tt.language)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCodeValidator_checkDangerousImports 测试危险导入检查
func TestCodeValidator_checkDangerousImports(t *testing.T) {
	t.Parallel()

	validator := NewCodeValidator()

	tests := []struct {
		name     string
		code     string
		language string
		wantErr  bool
	}{
		{
			name:     "Python安全导入",
			code:     "import json\nfrom datetime import datetime",
			language: "python",
			wantErr:  false,
		},
		{
			name:     "Python危险导入 - os",
			code:     "import os",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险导入 - subprocess",
			code:     "import subprocess",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险from导入 - os",
			code:     "from os import system",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险from导入 - sys",
			code:     "from sys import exit",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python危险__import__调用",
			code:     "__import__('os')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "JavaScript安全导入",
			code:     "import { Component } from 'react'",
			language: "javascript",
			wantErr:  false,
		},
		{
			name:     "JavaScript危险导入 - fs",
			code:     "import fs from 'fs'",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript危险require - child_process",
			code:     "const cp = require('child_process')",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript危险require - os",
			code:     "require('os')",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "TypeScript危险导入",
			code:     "import * as fs from 'fs'",
			language: "typescript",
			wantErr:  true,
		},
		{
			name:     "不支持的语言",
			code:     "import os",
			language: "go",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validator.checkDangerousImports(tt.code, tt.language)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCodeValidator_checkMaliciousPatterns 测试恶意模式检查
func TestCodeValidator_checkMaliciousPatterns(t *testing.T) {
	t.Parallel()

	validator := NewCodeValidator()

	tests := []struct {
		name     string
		code     string
		language string
		wantErr  bool
	}{
		{
			name:     "Python安全循环",
			code:     "for i in range(10):\n    print(i)",
			language: "python",
			wantErr:  false,
		},
		{
			name:     "Python有限while循环",
			code:     "i = 0\nwhile i < 10:\n    i += 1",
			language: "python",
			wantErr:  false,
		},
		{
			name:     "Python无限循环 - while True",
			code:     "while True:\n    pass",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "Python无限循环 - while 1",
			code:     "while 1:\n    print('loop')",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "JavaScript安全循环",
			code:     "for(let i = 0; i < 10; i++) { console.log(i); }",
			language: "javascript",
			wantErr:  false,
		},
		{
			name:     "JavaScript无限循环 - while(true)",
			code:     "while(true) { console.log('loop'); }",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript无限循环 - while(1)",
			code:     "while(1) { doSomething(); }",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "JavaScript无限循环 - for(;;)",
			code:     "for(;;) { console.log('infinite'); }",
			language: "javascript",
			wantErr:  true,
		},
		{
			name:     "不支持的语言",
			code:     "while(true) {}",
			language: "go",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validator.checkMaliciousPatterns(tt.code, tt.language)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCodeValidator_EdgeCases 测试边界情况
func TestCodeValidator_EdgeCases(t *testing.T) {
	t.Parallel()

	validator := NewCodeValidator()

	tests := []struct {
		name     string
		code     string
		language string
		wantErr  bool
	}{
		{
			name:     "空字符串代码",
			code:     "",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "只有空白字符",
			code:     "   \n\t  \r\n  ",
			language: "python",
			wantErr:  true,
		},
		{
			name:     "空语言类型",
			code:     "print('hello')",
			language: "",
			wantErr:  false,
		},
		{
			name:     "语言类型大小写混合",
			code:     "eval('test')",
			language: "Python",
			wantErr:  true,
		},
		{
			name:     "语言类型带空格",
			code:     "eval('test')",
			language: " python ",
			wantErr:  true,
		},
		{
			name:     "代码中包含注释的危险函数",
			code:     "# exec('malicious')\nprint('safe')",
			language: "python",
			wantErr:  true, // 当前实现会检测注释中的危险函数
		},
		{
			name:     "字符串中包含危险函数名",
			code:     "message = 'do not exec this'",
			language: "python",
			wantErr:  false, // 字符串中的函数名不应该被检测
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validator.Validate(tt.code, tt.language)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
