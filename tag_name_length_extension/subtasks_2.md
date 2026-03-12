# Phase 2: User Story US-001 — 新建标签支持 100 字符

## 目标
将 `TagsForm` 组件中标签名称输入框的 `maxLength` 硬编码替换为引用 `MAX_TAG_NAME_LENGTH` 常量，使新建标签时名称可输入最多 100 个字符。

## 独立验证方式
- 编译通过：`pnpm build` 无报错
- 新建标签表单中标签名称输入框允许输入 100 个字符
- 键入第 101 个字符时被阻止
- 粘贴超长内容时截断至 100 字符

## Tasks

- [X] T002 [US1] 在 TagsForm 组件 import 中添加 `MAX_TAG_NAME_LENGTH` 常量引用，路径：frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx:30行（`import { TAG_TYPE_OPTIONS, MAX_TAG_LENGTH } from '@/const';`）
      2. frontend/packages/loop-components/tag-components/src/const/index.ts:14行（常量定义）
    - Restrictions: 保留原有 `MAX_TAG_LENGTH` 导入不变，仅追加 `MAX_TAG_NAME_LENGTH`

- [X] T003 [US1] 将标签名称输入框 `maxLength={50}` 替换为 `maxLength={MAX_TAG_NAME_LENGTH}`，路径：frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx:133行（`maxLength={50}`，标签名称 FormInput）
      2. plan.md:阶段3 标签名称输入框变更
    - Restrictions: 仅修改 `tag_key_name` 字段对应的 `maxLength`，不修改描述字段的 `maxLength={200}`
