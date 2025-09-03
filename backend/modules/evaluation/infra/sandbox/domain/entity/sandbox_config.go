package entity

import "time"

// SandboxConfig 沙箱配置
type SandboxConfig struct {
	MemoryLimit   int64         `json:"memory_limit"`   // 内存限制 (MB)
	TimeoutLimit  time.Duration `json:"timeout_limit"`  // 执行超时时间
	MaxOutputSize int64         `json:"max_output_size"` // 最大输出大小 (bytes)
	NetworkEnabled bool         `json:"network_enabled"` // 是否允许网络访问
}

// DefaultSandboxConfig 默认沙箱配置
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		MemoryLimit:    128,              // 128MB
		TimeoutLimit:   30 * time.Second, // 30秒
		MaxOutputSize:  1024 * 1024,      // 1MB
		NetworkEnabled: false,            // 默认禁止网络
	}
}

// ExecutionLog 执行日志
type ExecutionLog struct {
	ID          string    `json:"id"`
	Language    string    `json:"language"`
	CodeHash    string    `json:"code_hash"`
	Duration    int64     `json:"duration_ms"`
	MemoryUsage int64     `json:"memory_usage"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}