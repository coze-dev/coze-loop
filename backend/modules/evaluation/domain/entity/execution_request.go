// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// SandboxExecutionRequest 沙箱代码执行请求
type SandboxExecutionRequest struct {
	Code        string            `json:"code"`
	Language    string            `json:"language"`
	Input       *EvalInput        `json:"input,omitempty"`
	Config      *SandboxConfig    `json:"config,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

// SandboxExecutionResult 沙箱代码执行结果
type SandboxExecutionResult struct {
	Output      *EvalOutput   `json:"output"`
	Error       string        `json:"error,omitempty"`
	ExitCode    int           `json:"exit_code"`
	Duration    time.Duration `json:"duration"`
	MemoryUsage int64         `json:"memory_usage"`
	Success     bool          `json:"success"`
	Stdout      string        `json:"stdout,omitempty"`
	Stderr      string        `json:"stderr,omitempty"`
}

// RuntimeCapabilities 运行时能力
type RuntimeCapabilities struct {
	SupportedLanguages []string `json:"supported_languages"`
	MaxMemoryMB        int64    `json:"max_memory_mb"`
	MaxExecutionTime   int64    `json:"max_execution_time_seconds"`
	NetworkAccess      bool     `json:"network_access"`
	FileSystemAccess   bool     `json:"file_system_access"`
}
