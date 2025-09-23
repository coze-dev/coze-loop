# @cozeloop/tag-components

> 标签管理组件库，提供完整的标签创建、编辑、管理和选择功能。

## ✨ 特性

- 🏷️ **完整的标签管理功能**：创建、编辑、删除、启用/禁用标签
- 🔍 **智能搜索和筛选**：支持按名称、类型、创建人等条件筛选
- 📱 **响应式设计**：适配不同屏幕尺寸和设备
- 🌍 **国际化支持**：内置中英文支持，可扩展其他语言
- 🎨 **可定制样式**：基于 Coze Design 设计系统，支持主题定制
- 📚 **Storybook 支持**：完整的组件文档和交互示例
- 🔧 **TypeScript 支持**：完整的类型定义和类型安全

## 📦 安装

```bash
# 使用 pnpm
pnpm add @cozeloop/tag-components

# 使用 npm
npm install @cozeloop/tag-components

# 使用 yarn
yarn add @cozeloop/tag-components
```

## 🚀 快速开始

```tsx
import { TagsList, TagsForm, TagSelect } from '@cozeloop/tag-components';
import { I18n } from '@cozeloop/i18n-adapter';

function App() {
  return (
    <div>
      <h1>{I18n.t('tag_management')}</h1>
      <TagsList tagListPagePath="/tags" />
    </div>
  );
}
```

## 🧩 组件列表

### 核心组件

- **`TagsList`** - 标签列表页面，包含搜索、筛选、分页等功能
- **`TagsForm`** - 标签创建/编辑表单，支持多种标签类型
- **`TagSelect`** - 标签选择器，支持搜索和新建标签
- **`TagsDetail`** - 标签详情页面，支持编辑和查看历史

### 功能组件

- **`AnnotationPanel`** - 标注面板，用于数据标注场景
- **`EditHistoryList`** - 编辑历史列表，显示标签变更记录
- **`TagStatusSwitch`** - 标签状态切换开关

### 工具组件

- **`TagsItem`** - 标签项展示组件
- **`TagTable`** - 标签表格组件

## 🛠️ 开发

### 环境要求

- Node.js >= 16
- pnpm >= 7

### 开发命令

```bash
# 安装依赖
rush update

# 启动开发服务器
npm run dev

# 构建生产版本
npm run build

# 启动 Storybook
npm run storybook

# 运行测试
npm run test

# 代码检查
npm run lint
```

### 项目结构

```
src/
├── components/          # 组件源码
│   ├── annotation-panel/    # 标注面板
│   ├── edit-history-list/   # 编辑历史
│   ├── tags-detail/         # 标签详情
│   ├── tags-form/           # 标签表单
│   ├── tags-item/           # 标签项
│   ├── tags-list/           # 标签列表
│   └── tags-select/         # 标签选择器
├── hooks/               # 自定义 Hooks
├── pages/               # 页面组件
├── utils/               # 工具函数
└── const/               # 常量定义
```

## 📖 API 文档

详细的 API 文档请查看 [Storybook](http://localhost:6006) 或组件源码中的 JSDoc 注释。

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

### 开发规范

- 遵循项目的 ESLint 和 TypeScript 配置
- 新组件需要添加 Storybook 故事
- 所有文案都需要支持国际化
- 提交前请运行测试和代码检查

## 📄 许可证

Apache-2.0

## 🔗 相关链接

- [Coze Loop 官网](https://coze.com)
- [Coze Design 设计系统](https://design.coze.com)
- [项目 Issues](https://github.com/coze/coze-loop/issues)
