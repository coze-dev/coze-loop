// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package enhanced

import (
	"sync"
	"time"
)

// CircuitBreakerState 熔断器状态
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	MaxFailures  int           // 最大失败次数
	Timeout      time.Duration // 熔断超时时间
	ResetTimeout time.Duration // 重置超时时间
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config       *CircuitBreakerConfig
	state        CircuitBreakerState
	failures     int
	lastFailTime time.Time
	mutex        sync.RWMutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = &CircuitBreakerConfig{
			MaxFailures:  10,
			Timeout:      30 * time.Second,
			ResetTimeout: 60 * time.Second,
		}
	}
	
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Allow 检查是否允许请求通过
func (cb *CircuitBreaker) Allow() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否应该转换到半开状态
		if time.Since(cb.lastFailTime) > cb.config.Timeout {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			// 双重检查
			if cb.state == StateOpen && time.Since(cb.lastFailTime) > cb.config.Timeout {
				cb.state = StateHalfOpen
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return cb.state == StateHalfOpen
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess 记录成功请求
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	switch cb.state {
	case StateHalfOpen:
		// 半开状态下成功，转换到关闭状态
		cb.state = StateClosed
		cb.failures = 0
	case StateClosed:
		// 关闭状态下成功，重置失败计数
		cb.failures = 0
	}
}

// RecordFailure 记录失败请求
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.failures++
	cb.lastFailTime = time.Now()
	
	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.MaxFailures {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		// 半开状态下失败，立即转换到开启状态
		cb.state = StateOpen
	}
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetFailures 获取失败次数
func (cb *CircuitBreaker) GetFailures() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failures
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.state = StateClosed
	cb.failures = 0
	cb.lastFailTime = time.Time{}
}