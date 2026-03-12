# Phase 1 实现总结：Prompt 创建表单预填默认值

## 概要

在 Prompt 创建表单的 `initValues` 中为 `prompt_key` 和 `prompt_name` 字段添加了默认值兜底逻辑，实现新建模式下自动预填 `prompt_key_0` 和 `prompt_demo_name_0`。

## 修改文件

### 1. PromptCreateModal (v2)
**文件**：`frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx`

**修改内容**：`initValues` 中两处 else 分支添加 `||` 默认值兜底

| 字段 | 修改前 | 修改后 |
|------|--------|--------|
| `prompt_key` | `data?.prompt_key` | `data?.prompt_key \|\| 'prompt_key_0'` |
| `prompt_name` | `data?.prompt_basic?.display_name` | `data?.prompt_basic?.display_name \|\| 'prompt_demo_name_0'` |

### 2. PromptCreate (v1)
**文件**：`frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx`

**修改内容**：与 v2 完全一致的修改模式，保持 v1/v2 行为一致

| 字段 | 修改前 | 修改后 |
|------|--------|--------|
| `prompt_key` | `data?.prompt_key` | `data?.prompt_key \|\| 'prompt_key_0'` |
| `prompt_name` | `data?.prompt_basic?.display_name` | `data?.prompt_basic?.display_name \|\| 'prompt_demo_name_0'` |

## 技术决策

- **使用 `||` 而非 `??`**（TA-02）：确保空字符串 `""` 也触发默认值填充，覆盖 Playground 快速创建场景（`data.prompt_key` 为 `""`）
- **硬编码默认值**（TA-06）：PRD 明确固定值，无需 Props 传入或 i18n 处理
- **不做 Snippet 特殊处理**（TA-05）：Snippet 模式下 `handleOk` 会用 `nanoid` 覆盖 `prompt_key`，默认值不影响结果
- **v1/v2 同步修改**（TA-04）：保持评测模块（v1）和主流程（v2）行为一致

## 覆盖场景验证

| 场景 | `data?.prompt_key` 值 | `||` 结果 | 状态 |
|------|----------------------|-----------|------|
| 新建（列表页） | `undefined` | `'prompt_key_0'` | ✅ |
| 新建（Playground 快速创建） | `''` | `'prompt_key_0'` | ✅ |
| 编辑模式 | `'existing_key'`（truthy） | `'existing_key'` | ✅ 不受影响 |
| 复制模式 | N/A（走 `isCopy` 分支） | N/A | ✅ 不受影响 |
| 评测模块（v1，无 data） | `undefined` | `'prompt_key_0'` | ✅ |

## 未修改的部分

- `prompt_description`：保持原样，不预填默认值
- Props 接口：未新增任何 Props
- API 类型定义：无变更
- i18n 资源：无新增 key（默认值为技术标识符，非展示文案）
- 消费方代码：`prompt-pages`、`prompt-develop`、`evaluate-components` 均无需修改
