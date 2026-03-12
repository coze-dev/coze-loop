# Phase 1: Foundational — 常量层变更

## 目标
将标签名称长度上限常量 `MAX_TAG_NAME_LENGTH` 从 50 修改为 100，校验函数 `tagNameValidate` 自动适配新值。

## 独立验证方式
- 编译通过：`pnpm build` 无报错
- 确认 `MAX_TAG_NAME_LENGTH` 值为 100，`MAX_TAG_LENGTH` 仍为 50（不得误改）
- `tagNameValidate` 对 100 字符合法名称返回空字符串，对 101 字符返回错误信息

## Tasks

- [X] T001 修改常量 `MAX_TAG_NAME_LENGTH` 值从 50 到 100，路径：frontend/packages/loop-components/tag-components/src/const/index.ts
    - Leverage:
      1. frontend/packages/loop-components/tag-components/src/const/index.ts:14行（`export const MAX_TAG_NAME_LENGTH = 50;`）
      2. plan.md:阶段1 常量与校验层变更
    - Restrictions: 仅修改 `MAX_TAG_NAME_LENGTH`，严禁修改 `MAX_TAG_LENGTH`（第13行，控制标签选项数量上限），二者当前值均为 50 极易混淆
