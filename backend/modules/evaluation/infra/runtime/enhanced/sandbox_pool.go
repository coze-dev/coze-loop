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

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// SandboxInstance 沙箱实例
type SandboxInstance struct {
	ID           string
	Status       InstanceStatus
	CreatedAt    time.Time
	LastUsed     time.Time
	ExecuteCount int64
	Language     entity.LanguageType
	Config       *entity.SandboxConfig
	logger       *logrus.Logger
}

// InstanceStatus 实例状态
type InstanceStatus int

const (
	StatusIdle InstanceStatus = iota
	StatusBusy
	StatusError
	StatusShutdown
)

// SandboxPool 沙箱池管理器
type SandboxPool struct {
	// 配置参数
	minInstances int
	maxInstances int
	idleTimeout  time.Duration
	
	// 实例池
	warmPool    chan *SandboxInstance
	activePool  sync.Map // map[string]*SandboxInstance
	
	// 统计指标
	metrics     *PoolMetrics
	
	// 控制组件
	logger      *logrus.Logger
	config      *entity.SandboxConfig
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	
	// 原子计数器
	instanceCounter int64
}

// PoolMetrics 池指标
type PoolMetrics struct {
	TotalInstances    int64
	ActiveInstances   int64
	IdleInstances     int64
	TotalExecutions   int64
	FailedExecutions  int64
	AverageExecTime   time.Duration
	PoolHitRate       float64
}

// NewSandboxPool 创建沙箱池
func NewSandboxPool(config *entity.SandboxConfig, logger *logrus.Logger) *SandboxPool {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &SandboxPool{
		minInstances: 5,  // 最小预热实例数
		maxInstances: 50, // 最大实例数
		idleTimeout:  5 * time.Minute,
		warmPool:     make(chan *SandboxInstance, 10),
		metrics:      &PoolMetrics{},
		logger:       logger,
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
	}
	
	// 启动池管理协程
	pool.wg.Add(1)
	go pool.poolManager()
	
	// 启动指标收集协程
	pool.wg.Add(1)
	go pool.metricsCollector()
	
	return pool
}

// GetInstance 获取可用实例
func (p *SandboxPool) GetInstance(languageType entity.LanguageType) (*SandboxInstance, error) {
	// 尝试从预热池获取
	select {
	case instance := <-p.warmPool:
		if instance.Status == StatusIdle && instance.Language == languageType {
			instance.Status = StatusBusy
			instance.LastUsed = time.Now()
			p.activePool.Store(instance.ID, instance)
			atomic.AddInt64(&p.metrics.ActiveInstances, 1)
			atomic.AddInt64(&p.metrics.IdleInstances, -1)
			return instance, nil
		}
		// 语言类型不匹配，放回池中
		p.warmPool <- instance
	default:
		// 预热池为空，创建新实例
	}
	
	// 检查是否超过最大实例数
	if atomic.LoadInt64(&p.metrics.TotalInstances) >= int64(p.maxInstances) {
		return nil, fmt.Errorf("沙箱池已达到最大实例数限制: %d", p.maxInstances)
	}
	
	// 创建新实例
	instance, err := p.createInstance(languageType)
	if err != nil {
		return nil, fmt.Errorf("创建沙箱实例失败: %w", err)
	}
	
	instance.Status = StatusBusy
	instance.LastUsed = time.Now()
	p.activePool.Store(instance.ID, instance)
	atomic.AddInt64(&p.metrics.ActiveInstances, 1)
	
	return instance, nil
}

// ReturnInstance 归还实例到池中
func (p *SandboxPool) ReturnInstance(instance *SandboxInstance) {
	if instance == nil {
		return
	}
	
	// 从活跃池中移除
	p.activePool.Delete(instance.ID)
	atomic.AddInt64(&p.metrics.ActiveInstances, -1)
	
	// 检查实例状态
	if instance.Status == StatusError || instance.ExecuteCount > 100 {
		// 实例有错误或执行次数过多，销毁实例
		p.destroyInstance(instance)
		return
	}
	
	// 重置实例状态
	instance.Status = StatusIdle
	instance.LastUsed = time.Now()
	
	// 尝试放回预热池
	select {
	case p.warmPool <- instance:
		atomic.AddInt64(&p.metrics.IdleInstances, 1)
	default:
		// 预热池已满，销毁实例
		p.destroyInstance(instance)
	}
}

// createInstance 创建新实例
func (p *SandboxPool) createInstance(languageType entity.LanguageType) (*SandboxInstance, error) {
	instanceID := fmt.Sprintf("sandbox-%d-%d", 
		time.Now().UnixNano(), 
		atomic.AddInt64(&p.instanceCounter, 1))
	
	instance := &SandboxInstance{
		ID:        instanceID,
		Status:    StatusIdle,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Language:  languageType,
		Config:    p.config,
		logger:    p.logger,
	}
	
	atomic.AddInt64(&p.metrics.TotalInstances, 1)
	
	p.logger.WithFields(logrus.Fields{
		"instance_id": instanceID,
		"language":    languageType,
	}).Info("创建新沙箱实例")
	
	return instance, nil
}

// destroyInstance 销毁实例
func (p *SandboxPool) destroyInstance(instance *SandboxInstance) {
	if instance == nil {
		return
	}
	
	instance.Status = StatusShutdown
	atomic.AddInt64(&p.metrics.TotalInstances, -1)
	
	p.logger.WithFields(logrus.Fields{
		"instance_id":    instance.ID,
		"execute_count":  instance.ExecuteCount,
		"lifetime":       time.Since(instance.CreatedAt),
	}).Info("销毁沙箱实例")
}

// poolManager 池管理协程
func (p *SandboxPool) poolManager() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.maintainPool()
		}
	}
}

// maintainPool 维护池状态
func (p *SandboxPool) maintainPool() {
	// 清理超时的空闲实例
	p.cleanupIdleInstances()
	
	// 确保最小实例数
	p.ensureMinInstances()
	
	// 记录池状态
	p.logPoolStatus()
}

// cleanupIdleInstances 清理超时的空闲实例
func (p *SandboxPool) cleanupIdleInstances() {
	now := time.Now()
	var instancesToDestroy []*SandboxInstance
	
	// 检查预热池中的实例
	poolSize := len(p.warmPool)
	for i := 0; i < poolSize; i++ {
		select {
		case instance := <-p.warmPool:
			if now.Sub(instance.LastUsed) > p.idleTimeout {
				instancesToDestroy = append(instancesToDestroy, instance)
				atomic.AddInt64(&p.metrics.IdleInstances, -1)
			} else {
				// 放回池中
				p.warmPool <- instance
			}
		default:
			break
		}
	}
	
	// 销毁超时实例
	for _, instance := range instancesToDestroy {
		p.destroyInstance(instance)
	}
}

// ensureMinInstances 确保最小实例数
func (p *SandboxPool) ensureMinInstances() {
	currentIdle := int(atomic.LoadInt64(&p.metrics.IdleInstances))
	if currentIdle < p.minInstances {
		needed := p.minInstances - currentIdle
		for i := 0; i < needed; i++ {
			// 为JavaScript和Python各创建一些实例
			var languageType entity.LanguageType
			if i%2 == 0 {
				languageType = entity.LanguageTypeJS
			} else {
				languageType = entity.LanguageTypePython
			}
			
			instance, err := p.createInstance(languageType)
			if err != nil {
				p.logger.WithError(err).Error("预热实例创建失败")
				continue
			}
			
			select {
			case p.warmPool <- instance:
				atomic.AddInt64(&p.metrics.IdleInstances, 1)
			default:
				p.destroyInstance(instance)
				break
			}
		}
	}
}

// metricsCollector 指标收集协程
func (p *SandboxPool) metricsCollector() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.updateMetrics()
		}
	}
}

// updateMetrics 更新指标
func (p *SandboxPool) updateMetrics() {
	// 计算池命中率
	totalExec := atomic.LoadInt64(&p.metrics.TotalExecutions)
	if totalExec > 0 {
		poolHits := totalExec - atomic.LoadInt64(&p.metrics.FailedExecutions)
		p.metrics.PoolHitRate = float64(poolHits) / float64(totalExec)
	}
}

// logPoolStatus 记录池状态
func (p *SandboxPool) logPoolStatus() {
	p.logger.WithFields(logrus.Fields{
		"total_instances":    atomic.LoadInt64(&p.metrics.TotalInstances),
		"active_instances":   atomic.LoadInt64(&p.metrics.ActiveInstances),
		"idle_instances":     atomic.LoadInt64(&p.metrics.IdleInstances),
		"total_executions":   atomic.LoadInt64(&p.metrics.TotalExecutions),
		"failed_executions":  atomic.LoadInt64(&p.metrics.FailedExecutions),
		"pool_hit_rate":      fmt.Sprintf("%.2f%%", p.metrics.PoolHitRate*100),
	}).Debug("沙箱池状态")
}

// GetMetrics 获取池指标
func (p *SandboxPool) GetMetrics() *PoolMetrics {
	return &PoolMetrics{
		TotalInstances:    atomic.LoadInt64(&p.metrics.TotalInstances),
		ActiveInstances:   atomic.LoadInt64(&p.metrics.ActiveInstances),
		IdleInstances:     atomic.LoadInt64(&p.metrics.IdleInstances),
		TotalExecutions:   atomic.LoadInt64(&p.metrics.TotalExecutions),
		FailedExecutions:  atomic.LoadInt64(&p.metrics.FailedExecutions),
		AverageExecTime:   p.metrics.AverageExecTime,
		PoolHitRate:       p.metrics.PoolHitRate,
	}
}

// Shutdown 关闭池
func (p *SandboxPool) Shutdown() error {
	p.logger.Info("开始关闭沙箱池...")
	
	// 取消上下文
	p.cancel()
	
	// 等待协程结束
	p.wg.Wait()
	
	// 安全清理所有实例
	defer func() {
		if r := recover(); r != nil {
			// Channel已经关闭，忽略panic
			p.logger.Debug("沙箱池channel已关闭")
		}
	}()
	
	// 先清理warmPool中的实例（非阻塞方式）
	done := false
	for !done {
		select {
		case instance := <-p.warmPool:
			if instance != nil {
				go p.destroyInstance(instance) // 异步清理避免阻塞
			}
		default:
			// 没有更多实例
			done = true
		}
	}
	
	// 关闭channel
	close(p.warmPool)
	
	// 清理活跃实例（异步方式）
	p.activePool.Range(func(key, value interface{}) bool {
		if instance, ok := value.(*SandboxInstance); ok {
			go p.destroyInstance(instance) // 异步清理
		}
		p.activePool.Delete(key)
		return true
	})
	
	p.logger.Info("沙箱池已关闭")
	return nil
}