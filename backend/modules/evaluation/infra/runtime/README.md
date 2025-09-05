# Runtime 模块重构说明

## 概述

本次重构整合了 `backend/modules/evaluation/infra/runtime` 目录下的所有运行时代码，实现了统一的运行时架构，提供了更简洁、高效和易维护的代码执行解决方案。

## 架构设计

### 核心组件

1. **UnifiedRuntime** (`unified_runtime.go`)
   - 统一的运行时实现，整合了所有运行时功能
   - 支持自动切换本地增强运行时和HTTP FaaS模式
   - 通过环境变量 `COZE_LOOP_FAAS_URL` 控制运行模式

2. **UnifiedRuntimeFactory** (`unified_factory.go`)
   - 统一的运行时工厂实现
   - 使用单例模式管理运行时实例
   - 支持多语言类型的运行时创建

3. **UnifiedRuntimeManager** (`unified_manager.go`)
   - 统一的运行时管理器
   - 提供线程安全的实例缓存和管理
   - 支持健康状态监控和指标收集

### 运行模式

#### 1. HTTP FaaS 模式
- 当设置环境变量 `COZE_LOOP_FAAS_URL` 时自动启用
- 通过HTTP调用远程FaaS服务执行代码
- 适用于生产环境和分布式部署

#### 2. 本地增强模式
- 当未设置 `COZE_LOOP_FAAS_URL` 时使用
- 使用本地增强运行时（沙箱池 + 任务调度器）
- 适用于开发环境和单机部署

## 支持的语言

- **JavaScript/TypeScript**: `js`, `javascript`, `ts`, `typescript`
- **Python**: `python`, `py`

## 主要特性

### 1. 统一接口
- 完全实现 `IRuntime` 接口
- 统一的代码执行和验证接口
- 一致的错误处理和结果格式

### 2. 自动模式切换
```go
// 设置环境变量启用HTTP FaaS模式
os.Setenv("COZE_LOOP_FAAS_URL", "http://faas-service:8000")

// 创建运行时（自动选择模式）
runtime, err := NewUnifiedRuntime(config, logger)
```

### 3. 资源管理
- 自动资源清理
- 线程安全的实例管理
- 优雅的错误处理

### 4. 监控和指标
- 健康状态检查
- 运行时指标收集
- 详细的执行日志

## 使用方式

### 基本使用

```go
import (
    "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/runtime"
    "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// 创建运行时管理器
logger := logrus.New()
config := entity.DefaultSandboxConfig()
manager := runtime.NewDefaultRuntimeManager(logger, config)

// 获取JavaScript运行时
jsRuntime, err := manager.GetRuntime(entity.LanguageTypeJS)
if err != nil {
    return err
}

// 执行代码
result, err := jsRuntime.RunCode(ctx, "console.log('Hello World')", "javascript", 5000)
if err != nil {
    return err
}

// 验证代码
isValid := jsRuntime.ValidateCode(ctx, "function test() {}", "javascript")
```

### 工厂模式使用

```go
// 创建工厂
factory := runtime.NewUnifiedRuntimeFactory(logger, config)

// 创建运行时
pythonRuntime, err := factory.CreateRuntime(entity.LanguageTypePython)
if err != nil {
    return err
}

// 执行Python代码
result, err := pythonRuntime.RunCode(ctx, "print('Hello Python')", "python", 5000)
```

## 配置选项

### 沙箱配置

```go
config := &entity.SandboxConfig{
    MemoryLimit:    256,              // 内存限制 (MB)
    TimeoutLimit:   30 * time.Second, // 执行超时
    MaxOutputSize:  2 * 1024 * 1024,  // 最大输出 (2MB)
    NetworkEnabled: false,            // 网络访问
}
```

### HTTP FaaS 配置

```go
// 通过环境变量配置
os.Setenv("COZE_LOOP_FAAS_URL", "http://coze-loop-faas-enhanced:8000")
```

## 迁移指南

### 从旧版本迁移

1. **替换工厂创建**
```go
// 旧版本
factory := runtime.NewRuntimeFactory(logger, config)

// 新版本（自动使用统一工厂）
factory := runtime.NewRuntimeFactory(logger, config)
// 或者直接使用
manager := runtime.NewDefaultRuntimeManager(logger, config)
```

2. **替换管理器创建**
```go
// 旧版本
manager := runtime.NewRuntimeManager(factory)

// 新版本
manager := runtime.NewDefaultRuntimeManager(logger, config)
```

3. **接口保持兼容**
- 所有 `IRuntime` 接口方法保持不变
- 所有 `IRuntimeFactory` 接口方法保持不变
- 所有 `IRuntimeManager` 接口方法保持不变

## 性能优化

1. **单例模式**: 统一运行时使用单例模式，减少资源消耗
2. **实例缓存**: 管理器缓存运行时实例，避免重复创建
3. **资源复用**: 内部组件支持资源复用和连接池
4. **异步处理**: 支持异步任务调度和并发执行

## 测试

运行测试：
```bash
cd backend/modules/evaluation/infra/runtime
go test -v ./...
```

测试覆盖：
- 基本功能测试
- 模式切换测试
- 并发安全测试
- 错误处理测试
- 资源清理测试

## 清理的文件

以下文件已被删除，功能已整合到统一运行时中：

- `deno_javascript_runtime.go` - JavaScript运行时适配器
- `deno_python_runtime.go` - Python运行时适配器  
- `enhanced_factory.go` - 增强运行时工厂
- `enhanced_manager.go` - 增强运行时管理器
- 相关测试文件

## 保留的文件

以下文件保留用于特定场景：

- `http_faas_runtime.go` - HTTP FaaS适配器（被统一运行时使用）
- `enhanced/` 目录 - 增强运行时实现（被统一运行时使用）
- `deno/` 目录 - Deno客户端实现
- `pyodide/` 目录 - Pyodide运行时实现

## 未来扩展

1. **新语言支持**: 可通过扩展统一运行时轻松添加新语言
2. **新运行模式**: 可添加新的运行时后端（如Docker、Kubernetes等）
3. **高级特性**: 可添加代码缓存、预编译、热重载等特性
4. **监控增强**: 可添加更详细的指标和追踪功能