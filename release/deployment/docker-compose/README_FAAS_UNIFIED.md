# 统一FaaS服务部署指南

## 概述

本项目已将原有的基础版FaaS和增强版FaaS服务整合为统一的FaaS服务，通过配置参数控制运行模式，简化部署和维护。

## 架构变更

### 整合前
- `coze-loop-faas`: 基础版，简单代码执行
- `coze-loop-faas-enhanced`: 增强版，沙箱池+任务调度

### 整合后
- `coze-loop-faas`: 统一服务，支持两种运行模式
  - **基础模式**: 轻量级代码执行，无池管理
  - **增强模式**: 沙箱池+任务调度，高并发优化

## 配置说明

### 环境变量配置

```bash
# 运行模式选择
FAAS_MODE=enhanced              # basic 或 enhanced

# 基础配置
FAAS_WORKSPACE=/tmp/faas-workspace
FAAS_TIMEOUT=30000
COZE_LOOP_FAAS_PORT=8889

# 增强模式配置 (仅在enhanced模式下生效)
FAAS_POOL_SIZE=10               # 预热实例数
FAAS_MAX_INSTANCES=50           # 最大实例数
FAAS_WORKER_COUNT=10            # 工作协程数

# 功能开关
FAAS_ENABLE_POOL=true           # 沙箱池
FAAS_ENABLE_SCHEDULER=true      # 任务调度
FAAS_ENABLE_METRICS=true        # 指标监控

# 资源限制
FAAS_MEMORY_LIMIT=2G
FAAS_CPU_LIMIT=1.0
```

### 运行模式对比

| 特性 | 基础模式 (basic) | 增强模式 (enhanced) |
|------|------------------|---------------------|
| 代码执行 | ✅ | ✅ |
| 沙箱池管理 | ❌ | ✅ |
| 任务调度 | ❌ | ✅ |
| 指标监控 | ❌ | ✅ |
| 内存占用 | 低 (256M-1G) | 中 (512M-2G) |
| 并发性能 | 基础 | 高 |
| 适用场景 | 轻量使用 | 高并发生产 |

## 部署方式

### 1. 基础模式部署
适用于开发环境或轻量级使用场景：

```bash
# 设置环境变量
export FAAS_MODE=basic
export FAAS_MEMORY_LIMIT=1G
export FAAS_CPU_LIMIT=0.5

# 启动服务
docker-compose --profile faas up -d
```

### 2. 增强模式部署
适用于生产环境或高并发场景：

```bash
# 设置环境变量
export FAAS_MODE=enhanced
export FAAS_POOL_SIZE=10
export FAAS_MAX_INSTANCES=50
export FAAS_MEMORY_LIMIT=2G
export FAAS_CPU_LIMIT=1.0

# 启动服务
docker-compose --profile faas up -d
```

### 3. 使用配置文件
复制并修改配置文件：

```bash
# 复制配置模板
cp .env.example .env

# 编辑配置文件
vim .env

# 启动服务
docker-compose --profile faas up -d
```

## API接口

### 代码执行接口
```http
POST /run_code
Content-Type: application/json

{
  "language": "python",
  "code": "result = 1 + 1\nprint(f'Result: {result}')",
  "timeout": 30000,
  "priority": "normal"
}
```

### 响应格式

**基础模式响应**:
```json
{
  "output": {
    "stdout": "Result: 2\n",
    "stderr": "",
    "ret_val": "2"
  }
}
```

**增强模式响应**:
```json
{
  "output": {
    "stdout": "Result: 2\n", 
    "stderr": "",
    "ret_val": "2"
  },
  "metadata": {
    "task_id": "task-1234567890-abc123",
    "instance_id": "sandbox-1-1234567890",
    "duration": 125,
    "pool_stats": {
      "mode": "enhanced",
      "totalInstances": 10,
      "idleInstances": 8,
      "activeInstances": 2
    },
    "mode": "enhanced"
  }
}
```

### 健康检查接口
```http
GET /health
```

**基础模式响应**:
```json
{
  "status": "healthy",
  "timestamp": "2025-01-01T12:00:00.000Z",
  "mode": "basic",
  "version": "unified-v1.0.0"
}
```

**增强模式响应**:
```json
{
  "status": "healthy",
  "timestamp": "2025-01-01T12:00:00.000Z", 
  "mode": "enhanced",
  "version": "unified-v1.0.0",
  "pool": {
    "mode": "enhanced",
    "totalInstances": 10,
    "idleInstances": 8,
    "activeInstances": 2
  },
  "scheduler": {
    "totalTasks": 150,
    "completedTasks": 148,
    "failedTasks": 2,
    "queuedTasks": 0,
    "averageExecutionTime": 95.5
  }
}
```

### 指标接口 (仅增强模式)
```http
GET /metrics
```

## 监控和运维

### 日志查看
```bash
# 查看服务日志
docker-compose logs coze-loop-faas

# 实时跟踪日志
docker-compose logs -f coze-loop-faas
```

### 性能监控
```bash
# 查看容器资源使用
docker stats coze-loop-faas

# 查看服务健康状态
curl http://localhost:8889/health

# 查看指标信息 (增强模式)
curl http://localhost:8889/metrics
```

### 故障排查

1. **服务启动失败**
   ```bash
   # 检查配置
   docker-compose config
   
   # 查看启动日志
   docker-compose logs coze-loop-faas
   ```

2. **内存不足**
   ```bash
   # 调整内存限制
   export FAAS_MEMORY_LIMIT=4G
   docker-compose up -d coze-loop-faas
   ```

3. **性能问题**
   ```bash
   # 切换到增强模式
   export FAAS_MODE=enhanced
   export FAAS_POOL_SIZE=20
   docker-compose up -d coze-loop-faas
   ```

## 迁移指南

### 从旧版本迁移

1. **备份现有配置**
   ```bash
   cp docker-compose.yml docker-compose.yml.backup
   ```

2. **更新配置文件**
   - 删除 `coze-loop-faas-enhanced` 服务定义
   - 更新 `coze-loop-faas` 服务配置
   - 添加新的环境变量

3. **重新部署**
   ```bash
   docker-compose down
   docker-compose --profile faas up -d
   ```

### Go后端适配

统一FaaS服务完全兼容现有的Go后端接口，无需修改业务代码：

```go
// 现有代码无需修改
runtime, err := NewHTTPFaaSRuntimeAdapter(languageType, config, logger)
result, err := runtime.RunCode(ctx, code, language, timeoutMS)
```

## 最佳实践

### 1. 环境选择
- **开发环境**: 使用基础模式，节省资源
- **测试环境**: 使用增强模式，验证性能
- **生产环境**: 使用增强模式，确保高可用

### 2. 资源配置
- **基础模式**: 1G内存，0.5CPU
- **增强模式**: 2G内存，1.0CPU
- **高并发场景**: 4G内存，2.0CPU

### 3. 池大小调优
- **轻量负载**: FAAS_POOL_SIZE=5, FAAS_MAX_INSTANCES=20
- **中等负载**: FAAS_POOL_SIZE=10, FAAS_MAX_INSTANCES=50
- **重载场景**: FAAS_POOL_SIZE=20, FAAS_MAX_INSTANCES=100

### 4. 监控告警
- 监控 `/health` 接口状态
- 关注内存和CPU使用率
- 设置任务失败率告警 (增强模式)

## 支持的语言

- **JavaScript/TypeScript**: 基于Deno运行时
- **Python**: 基于Pyodide沙箱 (WebAssembly)

## 安全特性

- 沙箱隔离执行
- 资源限制控制
- 网络访问限制
- 文件系统隔离
- 执行超时保护