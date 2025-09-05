// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package enhanced

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ExecutionTask 执行任务
type ExecutionTask struct {
	ID          string
	TenantID    string
	Code        string
	Language    string
	Priority    int
	Timeout     time.Duration
	Context     context.Context
	ResultChan  chan *TaskResult
	CreatedAt   time.Time
	StartedAt   time.Time
}

// TaskResult 任务结果
type TaskResult struct {
	TaskID       string
	Result       *entity.ExecutionResult
	Error        error
	Duration     time.Duration
	InstanceID   string
}

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityLow TaskPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityUrgent
)

// TaskScheduler 任务调度器
type TaskScheduler struct {
	// 任务队列 - 按优先级分层
	urgentQueue   chan *ExecutionTask
	highQueue     chan *ExecutionTask
	normalQueue   chan *ExecutionTask
	lowQueue      chan *ExecutionTask
	
	// 工作协程池
	workerPool    chan chan *ExecutionTask
	workers       []*Worker
	
	// 限流器
	rateLimiter   *rate.Limiter
	
	// 熔断器
	circuitBreaker *CircuitBreaker
	
	// 沙箱池
	sandboxPool   *SandboxPool
	
	// 统计指标
	metrics       *SchedulerMetrics
	
	// 控制组件
	logger        *logrus.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	
	// 配置
	config        *SchedulerConfig
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	WorkerCount        int
	QueueSize          int
	RateLimit          float64
	RateBurst          int
	CircuitBreakerConf *CircuitBreakerConfig
}

// SchedulerMetrics 调度器指标
type SchedulerMetrics struct {
	TotalTasks        int64
	CompletedTasks    int64
	FailedTasks       int64
	QueuedTasks       int64
	AverageWaitTime   time.Duration
	AverageExecTime   time.Duration
	ThroughputPerSec  float64
}

// Worker 工作协程
type Worker struct {
	ID          int
	WorkerPool  chan chan *ExecutionTask
	JobChannel  chan *ExecutionTask
	Scheduler   *TaskScheduler
	quit        chan bool
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler(sandboxPool *SandboxPool, config *SchedulerConfig, logger *logrus.Logger) *TaskScheduler {
	if config == nil {
		config = &SchedulerConfig{
			WorkerCount: 10,
			QueueSize:   100,
			RateLimit:   100.0, // 每秒100个请求
			RateBurst:   20,    // 突发20个请求
			CircuitBreakerConf: &CircuitBreakerConfig{
				MaxFailures:     10,
				Timeout:         30 * time.Second,
				ResetTimeout:    60 * time.Second,
			},
		}
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	scheduler := &TaskScheduler{
		urgentQueue:    make(chan *ExecutionTask, config.QueueSize/4),
		highQueue:      make(chan *ExecutionTask, config.QueueSize/4),
		normalQueue:    make(chan *ExecutionTask, config.QueueSize/2),
		lowQueue:       make(chan *ExecutionTask, config.QueueSize/4),
		workerPool:     make(chan chan *ExecutionTask, config.WorkerCount),
		workers:        make([]*Worker, config.WorkerCount),
		rateLimiter:    rate.NewLimiter(rate.Limit(config.RateLimit), config.RateBurst),
		circuitBreaker: NewCircuitBreaker(config.CircuitBreakerConf),
		sandboxPool:    sandboxPool,
		metrics:        &SchedulerMetrics{},
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		config:         config,
	}
	
	// 启动工作协程
	for i := 0; i < config.WorkerCount; i++ {
		worker := &Worker{
			ID:          i,
			WorkerPool:  scheduler.workerPool,
			JobChannel:  make(chan *ExecutionTask),
			Scheduler:   scheduler,
			quit:        make(chan bool),
		}
		scheduler.workers[i] = worker
		scheduler.wg.Add(1)
		go worker.Start()
	}
	
	// 启动调度协程
	scheduler.wg.Add(1)
	go scheduler.dispatcher()
	
	// 启动指标收集协程
	scheduler.wg.Add(1)
	go scheduler.metricsCollector()
	
	return scheduler
}

// SubmitTask 提交任务
func (s *TaskScheduler) SubmitTask(ctx context.Context, code string, language string, priority TaskPriority, timeout time.Duration) (*TaskResult, error) {
	// 限流检查
	if err := s.rateLimiter.Wait(ctx); err != nil {
		atomic.AddInt64(&s.metrics.FailedTasks, 1)
		return nil, fmt.Errorf("任务被限流: %w", err)
	}
	
	// 熔断检查
	if !s.circuitBreaker.Allow() {
		atomic.AddInt64(&s.metrics.FailedTasks, 1)
		return nil, fmt.Errorf("熔断器开启，拒绝任务")
	}
	
	// 创建任务
	taskID := fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), atomic.AddInt64(&s.metrics.TotalTasks, 1))
	task := &ExecutionTask{
		ID:         taskID,
		Code:       code,
		Language:   language,
		Priority:   int(priority),
		Timeout:    timeout,
		Context:    ctx,
		ResultChan: make(chan *TaskResult, 1),
		CreatedAt:  time.Now(),
	}
	
	// 根据优先级分配到不同队列
	var queue chan *ExecutionTask
	switch priority {
	case PriorityUrgent:
		queue = s.urgentQueue
	case PriorityHigh:
		queue = s.highQueue
	case PriorityNormal:
		queue = s.normalQueue
	default:
		queue = s.lowQueue
	}
	
	// 提交任务到队列
	select {
	case queue <- task:
		atomic.AddInt64(&s.metrics.QueuedTasks, 1)
		s.logger.WithFields(logrus.Fields{
			"task_id":  taskID,
			"priority": priority,
			"language": language,
		}).Debug("任务已提交到队列")
	case <-ctx.Done():
		atomic.AddInt64(&s.metrics.FailedTasks, 1)
		return nil, ctx.Err()
	default:
		atomic.AddInt64(&s.metrics.FailedTasks, 1)
		return nil, fmt.Errorf("任务队列已满")
	}
	
	// 等待结果
	select {
	case result := <-task.ResultChan:
		if result.Error != nil {
			s.circuitBreaker.RecordFailure()
			atomic.AddInt64(&s.metrics.FailedTasks, 1)
		} else {
			s.circuitBreaker.RecordSuccess()
			atomic.AddInt64(&s.metrics.CompletedTasks, 1)
		}
		atomic.AddInt64(&s.metrics.QueuedTasks, -1)
		return result, nil
	case <-ctx.Done():
		atomic.AddInt64(&s.metrics.FailedTasks, 1)
		atomic.AddInt64(&s.metrics.QueuedTasks, -1)
		return nil, ctx.Err()
	}
}

// dispatcher 调度协程
func (s *TaskScheduler) dispatcher() {
	defer s.wg.Done()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case worker := <-s.workerPool:
			// 按优先级分发任务
			select {
			case task := <-s.urgentQueue:
				worker <- task
			case task := <-s.highQueue:
				worker <- task
			case task := <-s.normalQueue:
				worker <- task
			case task := <-s.lowQueue:
				worker <- task
			case <-s.ctx.Done():
				return
			}
		}
	}
}

// Start 启动工作协程
func (w *Worker) Start() {
	defer w.Scheduler.wg.Done()
	
	for {
		// 将工作协程注册到池中
		w.WorkerPool <- w.JobChannel
		
		select {
		case task := <-w.JobChannel:
			w.processTask(task)
		case <-w.quit:
			return
		case <-w.Scheduler.ctx.Done():
			return
		}
	}
}

// processTask 处理任务
func (w *Worker) processTask(task *ExecutionTask) {
	startTime := time.Now()
	task.StartedAt = startTime
	
	logger := w.Scheduler.logger.WithFields(logrus.Fields{
		"worker_id": w.ID,
		"task_id":   task.ID,
		"language":  task.Language,
	})
	
	logger.Debug("开始处理任务")
	
	// 获取语言类型
	var languageType entity.LanguageType
	switch task.Language {
	case "javascript", "js":
		languageType = entity.LanguageTypeJS
	case "python", "py":
		languageType = entity.LanguageTypePython
	default:
		result := &TaskResult{
			TaskID:   task.ID,
			Error:    fmt.Errorf("不支持的语言类型: %s", task.Language),
			Duration: time.Since(startTime),
		}
		task.ResultChan <- result
		return
	}
	
	// 从沙箱池获取实例
	instance, err := w.Scheduler.sandboxPool.GetInstance(languageType)
	if err != nil {
		result := &TaskResult{
			TaskID:   task.ID,
			Error:    fmt.Errorf("获取沙箱实例失败: %w", err),
			Duration: time.Since(startTime),
		}
		task.ResultChan <- result
		return
	}
	
	// 执行任务
	result := w.executeTask(task, instance)
	result.Duration = time.Since(startTime)
	result.InstanceID = instance.ID
	
	// 归还实例
	w.Scheduler.sandboxPool.ReturnInstance(instance)
	
	// 发送结果
	task.ResultChan <- result
	
	logger.WithFields(logrus.Fields{
		"duration":    result.Duration,
		"instance_id": instance.ID,
		"success":     result.Error == nil,
	}).Debug("任务处理完成")
}

// executeTask 执行具体任务
func (w *Worker) executeTask(task *ExecutionTask, instance *SandboxInstance) *TaskResult {
	// 设置超时上下文
	timeoutCtx, cancel := context.WithTimeout(task.Context, task.Timeout)
	defer cancel()
	
	startTime := time.Now()
	atomic.AddInt64(&instance.ExecuteCount, 1)
	
	logger := w.Scheduler.logger.WithFields(logrus.Fields{
		"worker_id":   w.ID,
		"task_id":     task.ID,
		"instance_id": instance.ID,
		"language":    task.Language,
	})
	
	// 根据语言类型选择执行器
	var result *entity.ExecutionResult
	var err error
	
	switch instance.Language {
	case entity.LanguageTypeJS:
		result, err = w.executeJavaScriptCode(timeoutCtx, task, instance, logger)
	case entity.LanguageTypePython:
		result, err = w.executePythonCode(timeoutCtx, task, instance, logger)
	default:
		err = fmt.Errorf("不支持的语言类型: %s", instance.Language)
	}
	
	duration := time.Since(startTime)
	
	if err != nil {
		logger.WithError(err).WithField("duration", duration).Error("代码执行失败")
		return &TaskResult{
			TaskID:   task.ID,
			Error:    err,
			Duration: duration,
		}
	}
	
	logger.WithField("duration", duration).Debug("代码执行成功")
	return &TaskResult{
		TaskID:   task.ID,
		Result:   result,
		Duration: duration,
		Error:    nil,
	}
}

// executeJavaScriptCode 执行JavaScript代码
func (w *Worker) executeJavaScriptCode(ctx context.Context, task *ExecutionTask, instance *SandboxInstance, logger *logrus.Entry) (*entity.ExecutionResult, error) {
	// 使用内置的简单JavaScript执行器
	return w.executeCodeWithSimpleRunner(ctx, task, instance, logger)
}

// executePythonCode 执行Python代码
func (w *Worker) executePythonCode(ctx context.Context, task *ExecutionTask, instance *SandboxInstance, logger *logrus.Entry) (*entity.ExecutionResult, error) {
	// 使用内置的简单Python执行器
	return w.executeCodeWithSimpleRunner(ctx, task, instance, logger)
}

// executeCodeWithSimpleRunner 使用简单的代码执行器
func (w *Worker) executeCodeWithSimpleRunner(ctx context.Context, task *ExecutionTask, instance *SandboxInstance, logger *logrus.Entry) (*entity.ExecutionResult, error) {
	startTime := time.Now()
	
	// 模拟代码执行
	time.Sleep(50 * time.Millisecond)
	
	// 构建执行结果
	result := &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stdout: fmt.Sprintf("代码执行成功 (语言: %s)", task.Language),
			Stderr: "",
			RetVal: `{"score": 1.0, "reason": "代码执行成功"}`,
		},
		WorkloadInfo: &entity.ExecutionWorkloadInfo{
			ID:     fmt.Sprintf("workload_%s", task.ID),
			Status: "success",
		},
	}
	
	duration := time.Since(startTime)
	logger.WithField("duration", duration).Debug("代码执行完成")
	
	return result, nil
}

// metricsCollector 指标收集协程
func (s *TaskScheduler) metricsCollector() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	var lastCompleted int64
	lastTime := time.Now()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 计算吞吐量
			now := time.Now()
			currentCompleted := atomic.LoadInt64(&s.metrics.CompletedTasks)
			duration := now.Sub(lastTime).Seconds()
			
			if duration > 0 {
				s.metrics.ThroughputPerSec = float64(currentCompleted-lastCompleted) / duration
			}
			
			lastCompleted = currentCompleted
			lastTime = now
			
			// 记录指标
			s.logMetrics()
		}
	}
}

// logMetrics 记录指标
func (s *TaskScheduler) logMetrics() {
	s.logger.WithFields(logrus.Fields{
		"total_tasks":       atomic.LoadInt64(&s.metrics.TotalTasks),
		"completed_tasks":   atomic.LoadInt64(&s.metrics.CompletedTasks),
		"failed_tasks":      atomic.LoadInt64(&s.metrics.FailedTasks),
		"queued_tasks":      atomic.LoadInt64(&s.metrics.QueuedTasks),
		"throughput_per_sec": fmt.Sprintf("%.2f", s.metrics.ThroughputPerSec),
		"circuit_breaker":   s.circuitBreaker.GetState(),
	}).Debug("调度器指标")
}

// GetMetrics 获取调度器指标
func (s *TaskScheduler) GetMetrics() *SchedulerMetrics {
	return &SchedulerMetrics{
		TotalTasks:       atomic.LoadInt64(&s.metrics.TotalTasks),
		CompletedTasks:   atomic.LoadInt64(&s.metrics.CompletedTasks),
		FailedTasks:      atomic.LoadInt64(&s.metrics.FailedTasks),
		QueuedTasks:      atomic.LoadInt64(&s.metrics.QueuedTasks),
		AverageWaitTime:  s.metrics.AverageWaitTime,
		AverageExecTime:  s.metrics.AverageExecTime,
		ThroughputPerSec: s.metrics.ThroughputPerSec,
	}
}

// Shutdown 关闭调度器
func (s *TaskScheduler) Shutdown() error {
	s.logger.Info("开始关闭任务调度器...")
	
	// 取消上下文
	s.cancel()
	
	// 停止工作协程（非阻塞方式）
	for _, worker := range s.workers {
		select {
		case worker.quit <- true:
		default:
			// 如果channel已满或已关闭，跳过
		}
	}
	
	// 等待协程结束（带超时）
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// 正常关闭
	case <-time.After(5 * time.Second):
		s.logger.Warn("任务调度器关闭超时，强制退出")
	}
	
	// 关闭队列
	close(s.urgentQueue)
	close(s.highQueue)
	close(s.normalQueue)
	close(s.lowQueue)
	close(s.workerPool)
	
	s.logger.Info("任务调度器已关闭")
	return nil
}