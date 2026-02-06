# @cozeloop/guard

Cozeloop 权限守卫组件，用于管理和控制用户界面中的权限点。

## 安装

```bash
pnpm add @cozeloop/guard
```

## 特性

- 支持多种权限控制方式（隐藏、只读、拦截等）
- 提供上下文管理权限状态
- 支持路由级别的权限控制
- 可扩展的权限点定义

## 使用方法

### 基础使用

```tsx
import { Guard, GuardPoint } from '@cozeloop/guard';

function MyComponent() {
  return (
    <Guard point={GuardPoint['pe.prompts.create']}>
      <button onClick={() => console.log('创建提示词')}>
        创建提示词
      </button>
    </Guard>
  );
}
```

### 路由权限控制

```tsx
import { GuardRoute, GuardPoint } from '@cozeloop/guard';

function ProtectedRoute() {
  return (
    <GuardRoute point={GuardPoint['pe.prompts.create']}>
      <div>受保护的内容</div>
    </GuardRoute>
  );
}
```

### 权限上下文

```tsx
import { GuardProvider } from '@cozeloop/guard';

function App() {
  // 自定义权限策略
  const customStrategy = {
    // 实现 GuardStrategy 接口
  };

  return (
    <GuardProvider strategy={customStrategy}>
      <YourApp />
    </GuardProvider>
  );
}
```

## Features

- [x] eslint & ts
- [x] esm bundle
- [x] umd bundle
- [x] storybook

## Commands

- init: `rush update`
- dev: `npm run dev`
- build: `npm run build`
