# Enhanced Runtime 增强版运行时

## 概述

Enhanced Runtime 是基于 Deno 沙箱的高并发短代码执行方案的核心实现，提供了沙箱池管理、任务调度、熔断保护等企业级特性。

## 核心组件

### 1. SandboxPool (沙箱池管理器)

负责管理 Deno 沙箱实例的生命周期：

- **预热机制**：维护最小数量的预热实例，减少冷启动时间
- **实例复用**：安全前提下复用沙箱实例，提升性能
- **资源管理**：自动清理超时和过度使用的实例
- **指标监控**：实时监控池状态和性能指标

**主要特性：**
- 支持最小/最大实例数配置
- 自动清理空闲超时实例
- 实例执行计数和生命周期管理
- 线程安全的实例获取和归还

### 2. TaskScheduler (任务调度器)

提供高并发任务调度和执行：

- **优先级队列**：支持 4 级优先级任务调度（紧急、高、普通、低）
- **工作协程池**：可配置的工作协程数量
- **限流保护**：基于令牌桶的请求限流
- **熔断机制**：自动熔断和恢复机制
- **指标收集**：任务执行统计和性能监控

**主要特性：**
- 支持优先级任务调度
- 可配置的工作协程池大小
- 集成限流和熔断保护
- 实时任务执行指标

### 3. CircuitBreaker (熔断器)

提供服务保护和故障恢复：

- **三状态管理**：关闭、开启、半开状态
- **失败计数**：基于失败次数的熔断触发
- **自动恢复**：支持自动状态转换和恢复
- **配置灵活**：可配置失败阈值和超时时间

### 4. EnhancedRuntime (增强运行时)

集成所有组件的统一运行时接口：

- **统一接口**：实现标准 IRuntime 接口
- **多语言支持**：支持 JavaScript/TypeScript 和 Python
- **资源管理**：统一的资源创建、使用和清理
- **健康检查**：提供详细的健康状态和指标信息

## 使用方式

### 基本使用

```go
// 创建增强运行时
logger := logrus.New()
config := entity.DefaultSandboxConfig()
runtime, err := enhanced.NewEnhancedRuntime(config, logger)
if err != nil {
    log.Fatal(err)
}
defer runtime.Cleanup()

// 执行代码
ctx := context.Background()
result, err := runtime.RunCode(ctx, "console.log('Hello World');", "javascript", 5000)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("执行结果: %s\n", result.Output.Stdout)
```

### 工厂模式使用

```go
// 创建增强工厂
factory := runtime.NewEnhancedRuntimeFactory(logger, config)

// 创建管理器
manager := runtime.NewEnhancedRuntimeManager(factory, logger)

// 获取运行时实例
jsRuntime, err := manager.GetRuntime(entity.LanguageTypeJS)
if err != nil {
    log.Fatal(err)
}

// 执行代码
result, err := jsRuntime.RunCode(ctx, code, "javascript", 5000)
```

## 配置参数

### 沙箱池配置

- `minInstances`: 最小实例数 (默认: 5)
- `maxInstances`: 最大实例数 (默认: 50)
- `idleTimeout`: 空闲超时时间 (默认: 5分钟)

### 任务调度器配置

- `WorkerCount`: 工作协程数 (默认: 10)
- `QueueSize`: 队列大小 (默认: 100)
- `RateLimit`: 限流速率 (默认: 100 QPS)
- `RateBurst`: 突发请求数 (默认: 20)

### 熔断器配置

- `MaxFailures`: 最大失败次数 (默认: 10)
- `Timeout`: 熔断超时时间 (默认: 30秒)
- `ResetTimeout`: 重置超时时间 (默认: 60秒)

## 监控指标

### 沙箱池指标

- 总实例数
- 活跃实例数
- 空闲实例数
- 总执行次数
- 失败执行次数
- 池命中率

### 调度器指标

- 总任务数
- 完成任务数
- 失败任务数
- 排队任务数
- 平均等待时间
- 平均执行时间
- 每秒吞吐量

## 性能特性

- **高并发**：支持数千并发请求
- **低延迟**：预热机制实现毫秒级响应
- **高可用**：熔断保护和故障恢复
- **可扩展**：支持水平扩展和负载均衡

## 安全特性

- **沙箱隔离**：每个实例独立沙箱环境
- **资源限制**：内存、CPU、执行时间限制
- **权限控制**：最小权限原则
- **审计日志**：完整的执行日志记录

## 测试

运行单元测试：

```bash
cd backend
go test ./modules/evaluation/infra/runtime/enhanced/... -v
```

运行集成测试：

```bash
go test ./modules/evaluation/infra/runtime/... -v -run=Integration
```

## 注意事项

1. **资源管理**：确保正确调用 `Cleanup()` 方法清理资源
2. **并发安全**：所有组件都是线程安全的
3. **配置调优**：根据实际负载调整池大小和队列配置
4. **监控告警**：建议监控关键指标并设置告警
5. **日志级别**：生产环境建议使用 WARN 级别以上日志