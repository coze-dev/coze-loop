# CozeLoop

<div align="center">

![CozeLoop Logo](src/assets/images/logo.svg)

🧭 **CozeLoop** - 智能化的 AI 提示工程与评估平台

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../../LICENSE)
[![Version](https://img.shields.io/badge/version-1.4.1-green.svg)](package.json)
[![React](https://img.shields.io/badge/React-18.2.0-61dafb.svg)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.8.2-blue.svg)](https://www.typescriptlang.org/)

</div>

## 📖 项目简介

CozeLoop 是一个专业的 AI 提示工程与评估平台，为开发者和研究人员提供完整的 AI 应用开发工具链。平台集成了提示开发、模型评估、实验管理、可观测性监控等核心功能，帮助用户高效构建和优化 AI 应用。

## ✨ 核心功能

### 🎯 提示工程 (Prompt Engineering)

- **提示开发**: 可视化的提示编辑器，支持多种模板和变量
- **Playground**: 实时测试和调试提示效果
- **版本管理**: 提示版本控制和历史记录追踪

### 📊 评估系统 (Evaluation)

- **评估数据集**: 创建和管理测试数据集
- **评估器**: 自定义评估指标和规则
- **实验管理**: 批量实验执行和结果分析

### 🔍 可观测性 (Observability)

- **链路追踪**: 完整的请求链路监控
- **性能分析**: 实时性能指标和报告
- **日志管理**: 结构化日志收集和查询

### 🏷️ 标签管理 (Tag Management)

- **标签系统**: 灵活的资源分类和组织
- **权限控制**: 基于标签的访问控制
- **批量操作**: 高效的标签批量管理

## 🏗️ 技术架构

### 前端技术栈

- **框架**: React 18.2 + TypeScript 5.8
- **构建工具**: Rsbuild 1.1
- **路由**: React Router 6.22
- **状态管理**: Zustand 4.4
- **样式**: Tailwind CSS 3.3 + Less
- **组件库**: @coze-arch/coze-design
- **国际化**: 内置 i18n 支持

### 后端技术栈

- **语言**: Go
- **框架**: Hertz (CloudWeGo)
- **数据库**: MySQL + ClickHouse
- **缓存**: Redis
- **消息队列**: RocketMQ
- **微服务**: Kitex (CloudWeGo)

### 项目结构

```
frontend/apps/cozeloop/
├── src/
│   ├── components/          # 通用组件
│   │   ├── basic-layout/    # 基础布局
│   │   ├── navbar/          # 导航栏
│   │   └── user-info-section/ # 用户信息
│   ├── routes/              # 路由配置
│   ├── hooks/               # 自定义 Hooks
│   ├── constants/           # 常量定义
│   └── assets/              # 静态资源
├── config/                  # 配置文件
└── package.json
```

## 🚀 快速开始

### 环境要求

- Node.js >= 18
- pnpm >= 8
- Go >= 1.19 (后端开发)

### 安装依赖

```bash
# 安装前端依赖
pnpm install

# 安装后端依赖 (如需本地开发)
cd ../../backend && go mod download
```

### 开发模式

```bash
# 启动开发服务器 (中国区 BOE 环境)
npm run dev

# 启动开发服务器 (中国区)
npm run dev:cn

# 启动开发服务器 (中国区生产环境)
npm run dev:cn-release
```

### 构建部署

```bash
# 构建生产版本
npm run build

# 预览构建结果
npm run preview

# 分析构建包大小
npm run analyze
```

## 🧪 测试与质量

### 代码检查

```bash
# ESLint 检查
npm run lint

# 运行测试
npm run test

# TypeScript 类型检查
npm run build:ts
```

### 代码规范

- 遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范
- 使用 ESLint + Prettier 进行代码格式化
- 支持 Stylelint 样式检查
- 集成 Vitest 单元测试框架

## 📦 依赖说明

### 核心依赖

- `react` & `react-dom`: React 框架
- `react-router-dom`: 路由管理
- `zustand`: 轻量级状态管理
- `ahooks`: React Hooks 工具库
- `classnames`: CSS 类名工具
- `immer`: 不可变数据处理

### 内部包依赖

- `@cozeloop/*`: CozeLoop 业务模块包
- `@coze-arch/*`: Coze 架构基础包

## 🌍 国际化

项目支持多语言国际化，通过 `@cozeloop/i18n-adapter` 实现：

- 中文 (简体)
- 英文
- 其他语言扩展支持

## 🔧 配置说明

### 环境变量

- `REGION`: 部署区域 (cn/us/eu)
- `CUSTOM_VERSION`: 自定义版本 (inhouse/release)
- `BUILD_TYPE`: 构建类型 (online/offline)
- `BUNDLE_ANALYZE`: 是否启用构建分析

### 构建配置

- `rsbuild.config.ts`: Rsbuild 构建配置
- `tailwind.config.ts`: Tailwind CSS 配置
- `tsconfig.json`: TypeScript 配置

## 🤝 贡献指南

我们欢迎社区贡献！请查看 [CONTRIBUTING.md](../../CONTRIBUTING.md) 了解详细的贡献指南。

### 开发流程

1. Fork 项目仓库
2. 创建功能分支: `git checkout -b feat/your-feature`
3. 提交更改: `git commit -m 'feat: add some feature'`
4. 推送分支: `git push origin feat/your-feature`
5. 创建 Pull Request

## 📄 许可证

本项目采用 [Apache 2.0](../../LICENSE) 许可证。

## 🔗 相关链接

- [项目主页](https://github.com/coze-dev/coze-loop)
- [文档中心](https://docs.coze.com)
- [问题反馈](https://github.com/coze-dev/coze-loop/issues)
- [讨论区](https://github.com/coze-dev/coze-loop/discussions)

## 👥 维护团队

- **作者**: <qihai@bytedance.com>
- **维护者**: CozeLoop 开发团队

---

<div align="center">

**[⬆ 回到顶部](#cozeloop)**

Made with ❤️ by the CozeLoop Team

</div>
