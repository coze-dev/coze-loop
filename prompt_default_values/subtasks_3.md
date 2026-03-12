# Phase 3: User Story US-001 — 新建模式预填默认值端到端集成

## 前置依赖（Prerequisites）

- Phase 1 完成（v2 PromptCreateModal initValues 已修改）
- Phase 2 完成（v1 PromptCreate initValues 已修改）

## 目标

端到端验证 US-001「点击创建空白 Prompt 后表单预填默认值」在所有消费方入口（列表页、Playground 快速创建、评测模块）下的完整功能表现。本 Phase 不涉及代码修改，仅为集成验证。

## 独立验证方式

1. 列表页入口验证：在 Prompt 列表页点击「创建空白 Prompt」，表单 Prompt Key 显示 `prompt_key_0`，名称显示 `prompt_demo_name_0`，描述为空
2. Playground 入口验证：在 Playground 页面点击快速创建，表单预填默认值（`||` 运算符正确处理空字符串 `""` 场景）
3. 评测模块入口验证：评测模块中通过全局配置注入的 v1 PromptCreate 渲染新建模式，预填默认值
4. Modal 重开验证：关闭 Modal 后重新打开，默认值正确恢复（非残留上次编辑值）

## Tasks

- [X] T005 [US1] 验证列表页新建入口的默认值预填，路径：frontend/packages/loop-pages/prompt-pages/src/pages/list/index.tsx
    - Leverage:
      1. frontend/packages/loop-pages/prompt-pages/src/pages/list/index.tsx:102行（`onCreatePromptClick={() => createModal.open()}`，无参数调用，data=undefined）
      2. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx:170行-185行（initValues 计算逻辑）
      3. feature_spec.md:US-001 验收标准
    - Restrictions: 不修改任何代码；仅做端到端功能走查确认默认值在列表页新建模式下正确展示

- [X] T006 [US1] 验证 Playground 快速创建入口的默认值预填，路径：frontend/packages/loop-components/prompt-components-v2/src/components/prompt-develop/components/prompt-header/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-develop/components/prompt-header/index.tsx:373行-376行（`promptInfoModal.open({ prompt: newPromptInfo })`，传入 prompt 对象但 prompt_key 为空）
      2. mining-result.md:M-04（Playground 快速创建场景下 data.prompt_key 为空字符串）
      3. plan.md:TA-02（`||` 运算符处理空字符串决策）
    - Restrictions: 不修改代码；确认 `newPromptInfo` 中 `prompt_key` 为空字符串时，`|| 'prompt_key_0'` 生效

- [X] T007 [US1] 验证评测模块 v1 PromptCreate 新建入口的默认值预填，路径：frontend/packages/loop-components/evaluate-components/src/stores/eval-global-config.ts
    - Leverage:
      1. frontend/packages/loop-components/evaluate-components/src/stores/eval-global-config.ts:67行（`PromptCreate` 作为默认组件被注入）
      2. frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx（v1 组件 initValues）
      3. plan.md:阶段 5 验收场景
    - Restrictions: 不修改代码；确认评测模块调用 v1 PromptCreate 时不传 data，新建模式默认值生效

- [X] T008 [US1] 验证 Modal 关闭后重新打开时默认值恢复，路径：frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx
    - Leverage:
      1. mining-result.md:M-01、M-03（Modal 关闭时 initValues 重新生效机制）
      2. plan.md:TA-01（initValues 在 Modal 重新打开时的行为分析）
    - Restrictions: 不修改代码；确认 Semi Design Form 在 initValues 引用变化时会重新 setValues

## Implementation Strategy For Each Phase

### 本 Phase 特殊说明

本 Phase **不涉及代码修改**，是纯验证集成 Phase。Phase 1 和 Phase 2 的代码修改已覆盖全部 US-001 的实现工作。

### 验证策略

按照 T005→T008 的顺序逐一走查，每个 Task 对应一个消费方入口或边界场景。验证方式：

1. **代码走读**：追踪从消费方调用 → Modal 组件 → initValues 计算的完整数据流，确认默认值在各场景下正确生效
2. **运行时验证**（如开发环境可用）：在浏览器中实际操作各入口，确认表单预填值

### 验证通过标准

- 所有 4 个入口/场景下，新建模式的 Prompt Key 均为 `prompt_key_0`
- 所有 4 个入口/场景下，新建模式的 Prompt 名称均为 `prompt_demo_name_0`
- 编辑/复制模式不受影响
