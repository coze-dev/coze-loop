# PRD：Prompt 创建表单预填默认值

## 需求背景

在 Prompt 开发页面，用户点击「创建空白 Prompt」后会弹出创建表单。当前表单中的 Prompt Key 和 Prompt 名称字段为空，用户需要手动填写。

## 需求描述

在「创建空白 Prompt」的表单中，为以下字段预先填入默认值：

| 字段 | 默认值 |
|------|--------|
| Prompt Key | `prompt_key_0` |
| Prompt 名称 | `prompt_demo_name_0` |

## 用户场景

1. 用户进入 Prompt 开发页面
2. 用户点击「创建空白 Prompt」按钮
3. 弹出创建表单
4. 表单中 Prompt Key 字段已预填 `prompt_key_0`
5. 表单中 Prompt 名称字段已预填 `prompt_demo_name_0`
6. 用户可以直接使用默认值，也可以修改后提交

## 验收标准

- 点击创建空白 Prompt 后，表单中 Prompt Key 默认显示 `prompt_key_0`
- 点击创建空白 Prompt 后，表单中 Prompt 名称默认显示 `prompt_demo_name_0`
- 用户可以修改预填的默认值
- 默认值不影响表单验证逻辑
