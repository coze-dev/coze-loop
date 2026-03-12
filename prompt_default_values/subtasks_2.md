# Phase 2: Foundational — v1 PromptCreate initValues 默认值预填（同步修改）

## 前置依赖（Prerequisites）

- Phase 1 完成（v2 PromptCreateModal 默认值已添加，作为参考基准）
- 理解 plan.md TA-04 技术决策：同步修改 v1 和 v2，保持行为一致

## 目标

在 v1 版本 `PromptCreate` 组件的 `initValues` 中添加与 Phase 1 相同的默认值兜底逻辑，确保评测模块（evaluate-components）中通过全局配置注入的 PromptCreate 也具有默认值预填行为。

## 独立验证方式

1. 编译验证：`prompt-components` 包可正常通过 TypeScript 编译
2. 功能验证：
   - v1 新建模式（无 data，评测模块场景）：Prompt Key 显示 `prompt_key_0`，名称显示 `prompt_demo_name_0`
   - v1 编辑模式（isEdit=true）：显示已有数据
   - v1 复制模式（isCopy=true）：显示已有值 + `_copy`
3. 一致性验证：v1 与 v2 在相同入参下的 initValues 计算结果一致

## Tasks

- [X] T003 修改 PromptCreate 的 initValues，为 prompt_key 添加 `|| 'prompt_key_0'` 兜底，路径：frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx:143行-148行（当前 `data?.prompt_key` 赋值）
      2. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx（Phase 1 修改后的代码参考）
      3. plan.md:阶段 5 修改后代码示例
    - Restrictions: 使用 `||` 运算符；与 Phase 1 修改模式保持完全一致；注意 v1 组件使用硬编码 `95` 而非 `COPY_PROMPT_KEY_MAX_LEN` 常量

- [X] T004 修改 PromptCreate 的 initValues，为 prompt_name 添加 `|| 'prompt_demo_name_0'` 兜底，路径：frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx:149行-154行（当前 `data?.prompt_basic?.display_name` 赋值）
      2. plan.md:阶段 5 修改后代码示例
    - Restrictions: 使用 `||` 运算符；仅修改 else 分支

## Implementation Strategy For Each Phase

### 改动范围

仅修改 1 个文件中的 2 处：`prompt-create/index.tsx` 的 `initValues` 对象内两个字段的 else 分支。

### 具体修改

```diff
- : data?.prompt_key,
+ : data?.prompt_key || 'prompt_key_0',

- : data?.prompt_basic?.display_name,
+ : data?.prompt_basic?.display_name || 'prompt_demo_name_0',
```

### v1 与 v2 差异说明

| 差异点 | v1 (PromptCreate) | v2 (PromptCreateModal) |
|--------|-------------------|----------------------|
| Copy 长度常量 | 硬编码 `95` | `COPY_PROMPT_KEY_MAX_LEN`（值为 95） |
| isSnippet 支持 | 不支持 | 支持 |
| version 字段 | 无 | 有（Copy 模式下的版本选择） |
| 默认值修改方式 | **完全一致** | **完全一致** |

### 编译验证

T003 和 T004 修改完成后，在 `prompt-components` 目录下执行编译，确保无类型错误。
