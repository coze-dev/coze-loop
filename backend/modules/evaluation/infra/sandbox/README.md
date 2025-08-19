# Cozeloop Sandbox 代码执行环境

基于Deno + Pyodide技术栈的安全代码执行环境，为Cozeloop平台提供Python和JavaScript/TypeScript代码评估能力。

## 🚀 快速开始

```bash
# 安装Deno
curl -fsSL https://deno.land/install.sh | sh

# 安装Go依赖并启动服务
go mod tidy
go run cmd/demo/main.go
```

服务将在 `http://localhost:8080` 启动。

## 🌟 支持的语言

- **Python**: 通过Pyodide (WebAssembly)
- **JavaScript**: 通过Deno V8引擎  
- **TypeScript**: 通过Deno原生支持

## 📋 基本使用

```bash
# 执行Python代码
curl -X POST http://localhost:8080/api/v1/sandbox/execute \
  -H "Content-Type: application/json" \
  -d '{"code": "score = 1.0; reason = \"测试成功\"", "language": "python"}'

# 执行JavaScript代码
curl -X POST http://localhost:8080/api/v1/sandbox/execute \
  -H "Content-Type: application/json" \
  -d '{"code": "const score = 1.0; const reason = \"测试成功\";", "language": "javascript"}'
```

## 🔒 安全特性

- **沙箱隔离**: Deno安全沙箱 + Pyodide WASM隔离
- **资源限制**: 内存、时间、输出大小限制
- **代码验证**: 检测危险函数和模块导入
- **网络隔离**: 默认禁止网络访问

## 🧪 测试

```bash
# 运行所有测试
go test ./...

# 构建和运行
make build
make run
```

## 📚 详细文档

- **[完整使用指南](./docs/README.md)** - 详细的安装、配置和使用说明
- **[API接口文档](./docs/API.md)** - 完整的API接口规范
- **[架构设计文档](./docs/ARCHITECTURE.md)** - 系统架构和技术选型

## 🛠️ 项目结构

```
sandbox/
├── docs/                   # 文档目录
├── application/           # 应用服务层
├── domain/               # 领域层  
├── infra/               # 基础设施层
├── pkg/                # 工具包
├── cmd/demo/          # Demo服务器
├── Dockerfile         # 容器化配置
├── Makefile          # 构建脚本
├── go.mod            # Go模块定义
└── go.sum            # Go依赖锁定
```

## 📄 许可证

本项目遵循 [MIT许可证](../../../LICENSE)。