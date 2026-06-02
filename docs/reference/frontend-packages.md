# 前端 Rush.js 包结构与分层参考

> 本文档汇总前端 59 个包的分层结构、Adapter 模式和关键包职责，供快速查阅。
> 详细模块导航见 [`../../frontend/AGENTS.md`](../../frontend/AGENTS.md)。

## 6 层依赖结构

```
Level-6  apps/cozeloop/                    主 SPA 应用（React 18, Rsbuild）
   ↑
Level-5  packages/loop-pages/              5 个页面模块
   ↑       auth-pages, evaluate-pages, observation-pages, prompt-pages, tag-pages
Level-4  packages/loop-modules/            高阶业务模块
   ↑       evaluate/
Level-3  packages/loop-components/         13 个 UI 组件包 + adapter
   ↑       biz-components, shared-components, *-components, *-adapter
Level-2  packages/loop-base/               20 个基础库
   ↑       account, api-schema, hooks, components, env, i18n, stores, route...
Level-1  config/ (6) + infra/ (4+)         工具链配置 + 基础设施
           eslint-config, ts-config, vitest-config, eslint-plugin, idl...
```

**依赖不变量**: 高层级只能依赖低层级，禁止反向依赖。

## Level-5: 页面模块 (`packages/loop-pages/`)

| 包 | 职责 |
|---|---|
| `auth-pages` | 登录 / 认证相关页面 |
| `evaluate-pages` | 评测相关页面（评测集、评估器、实验） |
| `observation-pages` | 可观测性页面（Trace 查询、分析） |
| `prompt-pages` | Prompt 管理页面（编辑、Playground、版本） |
| `tag-pages` | 标签管理页面 |

## Level-3: 组件包 (`packages/loop-components/`)

| 包 | 职责 |
|---|---|
| `shared-components` | 通用 UI 组件 |
| `biz-components` | 业务通用组件 |
| `biz-config` | 业务配置组件 |
| `biz-hooks` | 业务通用 hooks |
| `evaluate-components` | 评测专用组件 |
| `observation-components` | 可观测性专用组件 |
| `prompt-components` | Prompt 专用组件 |
| `prompt-components-v2` | Prompt 组件 v2 版 |
| `tag-components` | 标签专用组件 |
| `adapter-interfaces` | Adapter 接口定义 |
| `evaluate-adapter` | 评测 Adapter 实现 |
| `observation-adapter` | 可观测性 Adapter 实现 |
| `components-with-adapter` | Adapter 消费层 |

## Level-2: 基础库 (`packages/loop-base/`)

| 包 | 职责 |
|---|---|
| `account` | 用户账户管理 |
| `api-schema` | API 类型定义（从 IDL 生成） |
| `base-hooks` | 通用 React hooks |
| `bot-env` / `bot-env-adapter` | 运行环境抽象 |
| `bot-flags` | 功能开关 |
| `bot-typings` | 全局类型定义 |
| `components` | 基础 UI 组件 |
| `env` | 环境变量管理 |
| `fetch-stream` | 流式请求 |
| `guard` | 权限守卫 |
| `i18n` / `intl` | 国际化 |
| `logger` | 日志 |
| `loop-lng` | 语言包 |
| `route` | 路由管理 |
| `stores` | 状态管理（Zustand） |
| `tea` | 数据埋点 |
| `toolkit` | 工具函数 |

## Level-1: 工具链配置

| 包/目录 | 职责 |
|---|---|
| `config/eslint-config` | ESLint 共享配置 |
| `config/ts-config` | TypeScript 共享配置 |
| `config/vitest-config` | Vitest 共享配置 |
| `config/postcss-config` | PostCSS 配置 |
| `config/stylelint-config` | Stylelint 配置 |
| `config/tailwind-config` | Tailwind CSS 配置 |
| `infra/eslint-plugin` | 自定义 ESLint 插件 |
| `infra/idl` | Thrift IDL -> TypeScript 转换 |
| `infra/plugins` | 构建插件 |
| `infra/utils` | 构建工具函数 |

## Adapter 模式

用于解耦商业版与开源版差异:

```
adapter-interfaces/           # 接口定义
    ↑
*-adapter/                    # 接口实现（evaluate-adapter, observation-adapter）
    ↑
components-with-adapter/      # 消费者
```

新增商业版/开源版差异必须走 adapter 接口，不可在组件中硬编码条件分支。
