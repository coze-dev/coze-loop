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

// TestCodeValidator_checkDangerousFunctions 测试危险函数检查
