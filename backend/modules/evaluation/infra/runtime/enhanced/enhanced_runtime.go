// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package enhanced

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// EnhancedRuntime 增强版运行时，集成沙箱池和任务调度
type EnhancedRuntime struct {
	// 增强组件
	sandboxPool     *SandboxPool
	taskScheduler   *TaskScheduler
	processManager  *DenoProcessManager
	
	// 配置和日志
	config          *entity.SandboxConfig
	logger          *logrus.Logger
	
	// 支持的语言类型
	supportedLanguages []entity.LanguageType
}

// NewEnhancedRuntime 创建增强版运行时
func NewEnhancedRuntime(config *entity.SandboxConfig, logger *logrus.Logger) (*EnhancedRuntime, error) {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}
	
	// 创建Deno进程管理器
	processManager := NewDenoProcessManager(config, logger)
	
	// 创建沙箱池
	sandboxPool := NewSandboxPool(config, logger)
	
	// 创建任务调度器
	schedulerConfig := &SchedulerConfig{
		WorkerCount: 10,
		QueueSize:   100,
		RateLimit:   100.0,
		RateBurst:   20,
	}
	taskScheduler := NewTaskScheduler(sandboxPool, schedulerConfig, logger)
	
	return &EnhancedRuntime{
		sandboxPool:        sandboxPool,
		taskScheduler:      taskScheduler,
		processManager:     processManager,
		config:             config,
		logger:             logger,
		supportedLanguages: []entity.LanguageType{entity.LanguageTypeJS, entity.LanguageTypePython},
	}, nil
}

// GetLanguageType 获取支持的语言类型（返回第一个作为主要类型）
func (er *EnhancedRuntime) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypeJS
}

// RunCode 在增强的沙箱环境中执行代码
func (er *EnhancedRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("代码不能为空")
	}
	
	// 设置超时
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = er.config.TimeoutLimit
	}
	
	// 确定任务优先级（可以根据租户、代码复杂度等因素动态调整）
	priority := PriorityNormal
	
	// 通过任务调度器提交任务
	result, err := er.taskScheduler.SubmitTask(ctx, code, language, priority, timeout)
	if err != nil {
		er.logger.WithError(err).WithFields(logrus.Fields{
			"language": language,
			"timeout":  timeout,
		}).Error("任务调度失败")
		return nil, fmt.Errorf("任务调度失败: %w", err)
	}
	
	// 检查任务执行结果
	if result.Error != nil {
		er.logger.WithError(result.Error).WithFields(logrus.Fields{
			"task_id":     result.TaskID,
			"instance_id": result.InstanceID,
			"duration":    result.Duration,
		}).Error("任务执行失败")
		return nil, result.Error
	}
	
	er.logger.WithFields(logrus.Fields{
		"task_id":     result.TaskID,
		"instance_id": result.InstanceID,
		"duration":    result.Duration,
		"language":    language,
	}).Debug("任务执行成功")
	
	return result.Result, nil
}

// ValidateCode 验证代码（简化实现，主要检查基本语法）
func (er *EnhancedRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	if code == "" {
		return false
	}
	
	// 根据语言类型进行基本验证
	switch language {
	case "javascript", "js", "typescript", "ts":
		// 简单的JavaScript/TypeScript语法检查
		return len(code) > 0 && !containsObviousErrors(code)
	case "python", "py":
		// 简单的Python语法检查
		return len(code) > 0 && !containsObviousErrors(code)
	default:
		er.logger.WithField("language", language).Warn("不支持的语言类型")
		return false
	}
}

// containsObviousErrors 检查代码是否包含明显的语法错误
func containsObviousErrors(code string) bool {
	// 简单的语法错误检查
	// 检查未闭合的引号
	singleQuotes := 0
	doubleQuotes := 0
	for _, char := range code {
		switch char {
		case '\'':
			singleQuotes++
		case '"':
			doubleQuotes++
		}
	}
	return singleQuotes%2 != 0 || doubleQuotes%2 != 0
}

// Cleanup 清理资源
func (er *EnhancedRuntime) Cleanup() error {
	er.logger.Info("开始清理增强运行时资源...")
	
	var errors []error
	
	// 关闭任务调度器
	if err := er.taskScheduler.Shutdown(); err != nil {
		errors = append(errors, fmt.Errorf("关闭任务调度器失败: %w", err))
	}
	
	// 关闭沙箱池
	if err := er.sandboxPool.Shutdown(); err != nil {
		errors = append(errors, fmt.Errorf("关闭沙箱池失败: %w", err))
	}
	
	// 关闭Deno进程管理器
	if err := er.processManager.StopAllProcesses(); err != nil {
		errors = append(errors, fmt.Errorf("关闭Deno进程管理器失败: %w", err))
	}
	
	if len(errors) > 0 {
		er.logger.WithField("errors", errors).Error("清理过程中出现错误")
		return fmt.Errorf("清理过程中出现 %d 个错误: %v", len(errors), errors)
	}
	
	er.logger.Info("增强运行时资源清理完成")
	return nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (er *EnhancedRuntime) GetSupportedLanguages() []entity.LanguageType {
	return er.supportedLanguages
}

// GetPoolMetrics 获取沙箱池指标
func (er *EnhancedRuntime) GetPoolMetrics() *PoolMetrics {
	return er.sandboxPool.GetMetrics()
}

// GetSchedulerMetrics 获取调度器指标
func (er *EnhancedRuntime) GetSchedulerMetrics() *SchedulerMetrics {
	return er.taskScheduler.GetMetrics()
}

// GetHealthStatus 获取健康状态
func (er *EnhancedRuntime) GetHealthStatus() map[string]interface{} {
	poolMetrics := er.GetPoolMetrics()
	schedulerMetrics := er.GetSchedulerMetrics()
	
	return map[string]interface{}{
		"status": "healthy",
		"pool": map[string]interface{}{
			"total_instances":   poolMetrics.TotalInstances,
			"active_instances":  poolMetrics.ActiveInstances,
			"idle_instances":    poolMetrics.IdleInstances,
			"pool_hit_rate":     fmt.Sprintf("%.2f%%", poolMetrics.PoolHitRate*100),
		},
		"scheduler": map[string]interface{}{
			"total_tasks":       schedulerMetrics.TotalTasks,
			"completed_tasks":   schedulerMetrics.CompletedTasks,
			"failed_tasks":      schedulerMetrics.FailedTasks,
			"queued_tasks":      schedulerMetrics.QueuedTasks,
			"throughput_per_sec": fmt.Sprintf("%.2f", schedulerMetrics.ThroughputPerSec),
		},
		"process_manager": map[string]interface{}{
			"process_count": er.processManager.GetProcessCount(),
		},
		"supported_languages": er.supportedLanguages,
	}
}

// 确保EnhancedRuntime实现IRuntime接口
var _ component.IRuntime = (*EnhancedRuntime)(nil)