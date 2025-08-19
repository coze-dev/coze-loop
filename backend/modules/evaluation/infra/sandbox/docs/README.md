# Cozeloop Sandbox 代码执行环境

基于Deno + Pyodide技术栈的安全代码执行环境，为Cozeloop平台提供Python和JavaScript/TypeScript代码评估能力。

## 🚀 快速开始

### 一键安装
```bash
# 安装Deno
curl -fsSL https://deno.land/install.sh | sh

# 安装Go依赖
go mod tidy

# 启动服务
go run cmd/demo/main.go
```

服务将在 `http://localhost:8080` 启动。

### 手动安装

#### 其他平台
- **macOS**: `brew install deno`
- **Windows**: `iwr https://deno.land/install.ps1 -useb | iex`
- **Linux**: 使用上述curl命令

## 🌟 支持的语言

- **Python**: 通过Pyodide (WebAssembly)
- **JavaScript**: 通过Deno V8引擎
- **TypeScript**: 通过Deno原生支持

## 📋 基本使用

### 执行Python代码
```bash
curl -X POST http://localhost:8080/api/v1/sandbox/execute \
  -H "Content-Type: application/json" \
  -d '{
    "code": "score = 1.0; reason = \"测试成功\"",
    "language": "python"
  }'
```

### 执行JavaScript代码
```bash
curl -X POST http://localhost:8080/api/v1/sandbox/execute \
  -H "Content-Type: application/json" \
  -d '{
    "code": "const score = 1.0; const reason = \"测试成功\";",
    "language": "javascript"
  }'
```

### 健康检查
```bash
curl http://localhost:8080/api/v1/sandbox/health
```

## 📊 数据格式

### 输入格式
```json
{
  "code": "代码内容",
  "language": "python|javascript|typescript",
  "eval_input": {
    "run": {
      "input": {"content_type": "text", "text": "输入内容"},
      "output": {"content_type": "text", "text": "用户输出"},
      "reference_output": {"content_type": "text", "text": "参考答案"}
    }
  }
}
```

### 输出格式
```json
{
  "success": true,
  "result": {
    "output": {
      "score": 1.0,
      "reason": "评估说明"
    },
    "success": true,
    "duration": 850000000
  }
}
```

## 🔒 安全特性

- **沙箱隔离**: Deno安全沙箱 + Pyodide WASM隔离
- **资源限制**: 内存、时间、输出大小限制
- **代码验证**: 检测危险函数和模块导入
- **网络隔离**: 默认禁止网络访问

## 🧪 测试

```bash
# 运行测试
go test ./...
make test
```

## 🛠️ 项目结构

- **docs/**: 文档目录
- **application/**: 应用服务层
- **domain/**: 领域层
- **infra/**: 基础设施层 (deno, pyodide)
- **pkg/**: 工具包
- **cmd/demo/**: Demo服务器

## 📚 相关文档

- [API接口文档](./API.md) - 详细的API接口说明
- [架构设计文档](./ARCHITECTURE.md) - 系统架构和技术选型
- [Deno官方文档](https://deno.land/manual)
- [Pyodide官方文档](https://pyodide.org/en/stable/)

## 🔧 故障排除

### 常见问题

- **Deno命令未找到**: 确保Deno已安装并在PATH中
- **Python代码执行失败**: 检查Pyodide初始化是否完成
- **端口被占用**: 使用环境变量 `PORT=8081` 修改端口

### 性能优化

- **首次执行较慢**: Pyodide初始化需要800-1000ms，后续执行会更快
- **内存限制**: 默认128MB，可通过配置调整
- **并发执行**: 支持多个请求并发处理

## 📄 许可证

本项目遵循 [MIT许可证](../../../LICENSE)。