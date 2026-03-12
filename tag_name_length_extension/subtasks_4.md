# Phase 4: User Story US-003 — 错误提示文案准确性

## 目标
更新 i18n 国际化文案中 `tag_name_length_limit` 的长度数字从 50 到 100，确保校验失败时的错误提示准确反映新的 100 字符限制。

## 独立验证方式
- 编译通过：`pnpm build` 无报错
- 中文环境下标签名称校验失败提示为「标签名称必须为 1～100 字符长度」
- 英文环境下标签名称校验失败提示为「Tag name must be 1-100 characters long」
- 其他校验提示文案（字符合法性、唯一性）不受影响

## Tasks

- [X] T007 [US3] 更新中文 i18n 文案 `tag_name_length_limit` 中的 50 为 100，路径：frontend/packages/loop-base/loop-lng/src/locales/tag/zh-CN.json
    - Leverage:
      1. frontend/packages/loop-base/loop-lng/src/locales/tag/zh-CN.json:17行（`"tag_name_length_limit": "标签名称必须为 1～50 字符长度"`）
      2. plan.md:阶段2 i18n 国际化文案更新
    - Restrictions: 仅修改 `tag_name_length_limit` 字段值中的数字 50→100，不修改其他任何 i18n 条目

- [X] T008 [US3] 更新英文 i18n 文案 `tag_name_length_limit` 中的 50 为 100，路径：frontend/packages/loop-base/loop-lng/src/locales/tag/en-US.json
    - Leverage:
      1. frontend/packages/loop-base/loop-lng/src/locales/tag/en-US.json:17行（`"tag_name_length_limit": "Tag name must be 1-50 characters long"`）
      2. plan.md:阶段2 i18n 国际化文案更新
    - Restrictions: 仅修改 `tag_name_length_limit` 字段值中的数字 50→100，不修改其他任何 i18n 条目
