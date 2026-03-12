# Phase 2 实现总结：v1 PromptCreate initValues 默认值预填

## 概述

在 v1 版本 `PromptCreate` 组件的 `initValues` 中添加了与 Phase 1（v2 PromptCreateModal）相同的默认值兜底逻辑，确保评测模块（evaluate-components）中通过全局配置注入的 PromptCreate 也具有默认值预填行为。

## 完成任务

| 任务 | 状态 | 说明 |
|------|------|------|
| T003 | ✅ | `prompt_key` 添加 `\|\| 'prompt_key_0'` 兜底 |
| T004 | ✅ | `prompt_name` 添加 `\|\| 'prompt_demo_name_0'` 兜底 |

## 修改文件

### `frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx`

**修改点 1（T003）— prompt_key 默认值：**

```diff
- : data?.prompt_key,
+ : data?.prompt_key || 'prompt_key_0',
```

**修改点 2（T004）— prompt_name 默认值：**

```diff
- : data?.prompt_basic?.display_name,
+ : data?.prompt_basic?.display_name || 'prompt_demo_name_0',
```

## 技术决策一致性

| 决策点 | v1 (PromptCreate) | v2 (PromptCreateModal) | 一致性 |
|--------|-------------------|----------------------|--------|
| 运算符 | `\|\|` | `\|\|` | ✅ |
| prompt_key 默认值 | `'prompt_key_0'` | `'prompt_key_0'` | ✅ |
| prompt_name 默认值 | `'prompt_demo_name_0'` | `'prompt_demo_name_0'` | ✅ |
| Copy 长度常量 | 硬编码 `95` | `COPY_PROMPT_KEY_MAX_LEN`（值为 95） | ✅ 保持原有差异 |

## 影响分析

| 场景 | 预期行为 | 是否正确 |
|------|----------|----------|
| 新建模式（无 data，评测模块场景） | Key=`prompt_key_0`, Name=`prompt_demo_name_0` | ✅ |
| 编辑模式（isEdit=true） | 显示已有数据 | ✅ |
| 复制模式（isCopy=true） | 显示已有值 + `_copy` | ✅ |
| data 中字段为空字符串 | 触发默认值填充（`\|\|` 运算符处理） | ✅ |

## 改动统计

- 新增文件：0 个
- 修改文件：1 个（`prompt-components/src/prompt-create/index.tsx`）
- 代码改动量：2 行（仅 else 分支追加 `|| '默认值'`）
