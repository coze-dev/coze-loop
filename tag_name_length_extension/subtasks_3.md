# Phase 3: User Story US-002 — 编辑标签及选项值名称支持 100 字符

## 目标
将 `TagsForm` 组件中分类型选项值输入框（1 处）和布尔型选项值输入框（2 处）的 `maxLength={50}` 硬编码替换为 `maxLength={MAX_TAG_NAME_LENGTH}`，确保编辑标签时标签名称和所有选项值名称均支持最多 100 个字符，与新建规则保持一致。

## 独立验证方式
- 编译通过：`pnpm build` 无报错
- 编辑标签表单中标签名称输入框允许输入 100 个字符（已由 Phase 2 完成）
- 分类型标签选项值输入框允许输入 100 个字符
- 布尔型标签选项一/选项二输入框允许输入 100 个字符
- 所有 `maxLength` 均由常量驱动，无残留硬编码 `50`

## Tasks

- [X] T004 [US2] 将分类型选项值输入框 `maxLength={50}` 替换为 `maxLength={MAX_TAG_NAME_LENGTH}`，路径：frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx:220行（分类型 ArrayField 内的 FormInput `maxLength={50}`）
      2. plan.md:阶段3 分类型选项值输入框变更
    - Restrictions: 仅修改 `tag_values[n].tag_value_name` 对应的 FormInput，不影响其他字段

- [X] T005 [US2] 将布尔型选项一输入框 `maxLength={50}` 替换为 `maxLength={MAX_TAG_NAME_LENGTH}`，路径：frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx:310行（布尔型选项一 FormInput `maxLength={50}`）
      2. plan.md:阶段3 布尔型选项一输入框变更
    - Restrictions: 仅修改 `tag_values.0.tag_value_name` 对应的 FormInput

- [X] T006 [US2] 将布尔型选项二输入框 `maxLength={50}` 替换为 `maxLength={MAX_TAG_NAME_LENGTH}`，路径：frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx:334行（布尔型选项二 FormInput `maxLength={50}`）
      2. plan.md:阶段3 布尔型选项二输入框变更
    - Restrictions: 仅修改 `tag_values.1.tag_value_name` 对应的 FormInput
