# @coze-arch/monorepo-kits

> 一个用于管理 monorepo 项目的工具包，提供了基于 Rush 框架的项目查找、依赖分析和配置管理功能。

## 🚀 功能概述

`@coze-arch/monorepo-kits` 是专为 monorepo 项目设计的工具库，提供了一套完整的包管理、依赖分析和配置查询解决方案。通过统一的 API 接口，帮助开发者更高效地管理复杂的 monorepo 项目结构。

## ✨ 主要功能

### 📦 子包管理

- **依赖递归查找**: 自动查找包的所有子依赖关系
- **路径解析**: 获取包在文件系统中的实际位置
- **配置查询**: 读取包的 package.json 配置信息
- **缓存优化**: 智能缓存机制提升查找性能

### ⚙️ Rush 配置管理

- **配置加载**: 自动加载和管理 Rush 配置
- **单例模式**: 避免重复加载配置文件
- **类型安全**: 完整的 TypeScript 类型支持

### 🔍 项目查找

- **依赖分析**: 查找包的直接和间接依赖关系
- **项目定位**: 快速定位指定包的项目配置
- **关系查询**: 分析包之间的依赖关系图

## 📚 API 文档

### 子包管理

#### `lookupSubPackages(packageName: string): string[]`

递归查找指定包的所有子依赖包。

```typescript
import { lookupSubPackages } from '@coze-arch/monorepo-kits';

const deps = lookupSubPackages('@coze/ui-components');
console.log(deps); // ['@coze/icons', '@coze/themes', ...]
```

#### `getPackageLocation(packageName: string): string`

获取指定包的文件系统路径。

```typescript
import { getPackageLocation } from '@coze-arch/monorepo-kits';

const location = getPackageLocation('@coze/ui-components');
console.log(location); // '/path/to/packages/ui-components'
```

#### `getPackageJson(packageName: string): RushConfigurationProject['packageJson']`

获取指定包的 package.json 配置信息。

```typescript
import { getPackageJson } from '@coze-arch/monorepo-kits';

const pkg = getPackageJson('@coze/ui-components');
console.log(pkg.version); // '1.0.0'
```

### Rush 配置管理

#### `getRushConfiguration(): RushConfiguration`

获取 Rush 配置实例（单例模式）。

```typescript
import { getRushConfiguration } from '@coze-arch/monorepo-kits';

const rushConfig = getRushConfiguration();
console.log(rushConfig.projects.length); // 项目总数
```

### 项目查找

#### `lookupTo(to: string): string[]`

查找指定包的直接依赖项。

```typescript
import { lookupTo } from '@coze-arch/monorepo-kits';

const dependencies = lookupTo('@coze/ui-components');
console.log(dependencies); // 依赖的包名数组
```

#### `lookupOnly(packageName: string): RushConfigurationProject`

查找并返回指定包的完整项目配置对象。

```typescript
import { lookupOnly } from '@coze-arch/monorepo-kits';

const project = lookupOnly('@coze/ui-components');
console.log(project.projectFolder); // 项目文件夹路径
```

## 🛠 安装使用

### 安装

```bash
# 在 monorepo 内部作为工具库使用
npm install @coze-arch/monorepo-kits
```

### 基本使用

```typescript
import {
  lookupSubPackages,
  getPackageLocation,
  getRushConfiguration,
  lookupOnly
} from '@coze-arch/monorepo-kits';

// 查找包的所有依赖
const allDeps = lookupSubPackages('your-package');

// 获取包的位置
const location = getPackageLocation('your-package');

// 获取项目配置
const project = lookupOnly('your-package');

// 获取 Rush 配置
const rushConfig = getRushConfiguration();
```

## 🏗 项目结构

```
src/
├── index.ts          # 主入口文件，导出所有公共 API
├── sub-packages.ts   # 子包管理和依赖查找功能
├── rush-config.ts    # Rush 配置管理
└── lookup.ts         # 项目查找相关功能
```

## 🎯 使用场景

### 1. 依赖分析工具

```typescript
// 分析包的依赖关系
const analyzeDependencies = (packageName: string) => {
  const allDeps = lookupSubPackages(packageName);
  const directDeps = lookupTo(packageName);

  return {
    total: allDeps.length,
    direct: directDeps.length,
    indirect: allDeps.length - directDeps.length
  };
};
```

### 2. 构建脚本集成

```typescript
// 在构建脚本中获取包信息
const buildPackage = (packageName: string) => {
  const location = getPackageLocation(packageName);
  const pkg = getPackageJson(packageName);

  console.log(`Building ${pkg.name}@${pkg.version} at ${location}`);
  // ... 执行构建逻辑
};
```

### 3. 自动化工具开发

```typescript
// 为自动化工具提供项目信息
const getProjectInfo = (packageName: string) => {
  const project = lookupOnly(packageName);
  const dependencies = lookupSubPackages(packageName);

  return {
    name: project.packageName,
    path: project.projectFolder,
    dependencies,
    config: project.packageJson
  };
};
```

## 🚀 特性优势

- **🎯 专为 Monorepo 设计**: 针对大型 monorepo 项目优化
- **⚡ 高性能**: 智能缓存机制，避免重复计算
- **🛡 类型安全**: 完整的 TypeScript 类型定义
- **🔧 易于集成**: 简洁的 API 设计，易于集成到现有工具链
- **📦 轻量级**: 最小化依赖，专注核心功能

## 📄 依赖信息

- **主要依赖**: `@rushstack/rush-sdk@5.172.1`
- **开发依赖**: ESLint、TypeScript、Vitest 等工具链
- **运行环境**: Node.js 18+

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request 来改进这个项目。

## 📝 许可证

Apache-2.0 License

---

**作者**: <fanwenjie.fe@bytedance.com>

如需了解更多信息，请查看项目文档或联系维护团队。
