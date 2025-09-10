# Runtime 模块重构说明

## 概述

本次重构整合了 `backend/modules/evaluation/infra/runtime` 目录下的所有运行时代码，实现了统一的运行时架构，提供了更简洁、高效和易维护的代码执行解决方案。

## 架构设计1. **Runtime** (`runtime.go`)
- 统一的运行时实现，专注于HTTP FaaS模式
- 通过环境变量 `COZE_LOOP_PYTHON_FAAS_URL` 和 `COZE_LOOP_JS_FAAS_URL` 配置FaaS服务
- 根据语言类型自动路由到对应的FaaS服务

2. **RuntimeFactory** (`factory.go`)
- 统一的运行时工厂实现
- 使用单例模式管理运行时实例
- 支持多语言类型的运行时创建

3. **RuntimeManager** (`manager.go`)
- 统一的运行时管理器
- 提供线程安全的实例缓存和管理
- 支持运行时的生命周期管理
### 运行模式

#### 1. HTTP FaaS 模式
- 当设置环境变量 `COZE_LOOP_FAAS_URL` 时自动启用
- 通过HTTP调用远程FaaS服务执行代码
- 适用于生产环境和分布式部署

#### 2. 精简架构
- 移除了本地增强运行时模式
- 仅支持HTTP FaaS模式，简化了架构复杂度
- 专注于Python和JavaScript的FaaS执行

## 支持的语言

- **JavaScript/TypeScript**: `js`, `javascript`, `ts`, `typescript`
- **Python**: `python`, `py`

## 主要特性

### 1. 统一接口
- 完全实现 `IRuntime` 接口
- 统一的代码执行和验证接口
- 一致的错误处理和结果格式

### 2. FaaS服务配置
```go
// 设置环境变量配置FaaS服务
os.Setenv("COZE_LOOP_PYTHON_FAAS_URL", "http://python-faas:8000")
os.Setenv("COZE_LOOP_JS_FAAS_URL", "http://js-faas:8000")

// 创建运行时（自动路由到对应FaaS服务）
runtime, err := NewRuntime(config, logger)
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
factory := runtime.NewRuntimeFactory(logger, config)
manager := runtime.NewRuntimeManager(factory, logger)

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
factory := runtime.NewRuntimeFactory(logger, config)

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
os.Setenv("COZE_LOOP_PYTHON_FAAS_URL", "http://coze-loop-python-faas:8000")
os.Setenv("COZE_LOOP_JS_FAAS_URL", "http://coze-loop-js-faas:8000")
```

## 迁移指南

### 从旧版本迁移

1. **替换工厂创建**
```go
// 旧版本
factory := runtime.NewRuntimeFactory(logger, config)// 新版本
factory := runtime.NewRuntimeFactory(logger, config)
manager := runtime.NewRuntimeManager(factory, logger)
```

2. **替换管理器创建**
```go
// 旧版本
manager := runtime.NewRuntimeManager(factory)

// 新版本
factory := runtime.NewRuntimeFactory(logger, config)
manager := runtime.NewRuntimeManager(factory, logger)
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

## 精简后的架构

本次精简重构删除了以下文件和目录：

### 删除的本地执行相关代码
- `enhanced/` 目录 - 增强运行时实现（沙箱池、任务调度器等）
- `deno/` 目录 - Deno客户端实现
- `pyodide/` 目录 - Pyodide运行时实现
- `simple_faas_server.py` - 本地FaaS服务器
- `simple_runtime.go` - 简单运行时实现
- `factory.go` - 旧版运行时工厂
- `manager.go` - 旧版运行时管理器
- 相关测试文件

### 保留的核心文件
- `unified_runtime.go` - 统一运行时（仅支持HTTP FaaS）
- `unified_factory.go` - 统一运行时工厂
- `unified_manager.go` - 统一运行时管理器
- `http_faas_runtime.go` - HTTP FaaS适配器
- 相关测试文件

## 未来扩展

1. **新语言支持**: 可通过扩展统一运行时轻松添加新语言
2. **新运行模式**: 可添加新的运行时后端（如Docker、Kubernetes等）
3. **高级特性**: 可添加代码缓存、预编译、热重载等特性
4. **监控增强**: 可添加更详细的指标和追踪功能