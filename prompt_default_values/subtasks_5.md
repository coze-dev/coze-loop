# Phase 5: User Story US-003 — 编辑/复制模式不受默认值影响验证

## 前置依赖（Prerequisites）

- Phase 1 完成（v2 initValues 默认值已添加）
- Phase 2 完成（v1 initValues 默认值已添加）

## 目标

验证 US-003「预填默认值不影响已有的编辑、复制等表单行为」——编辑模式显示已有数据而非默认值，复制模式显示带 `_copy` 后缀的已有数据而非默认值。本 Phase 不涉及代码修改。

## 独立验证方式

1. 编辑模式验证（v2）：isEdit=true，data 包含有效 prompt_key，表单显示已有值且 Prompt Key 为禁用状态
2. 复制模式验证（v2）：isCopy=true，表单显示已有值 + `_copy` 后缀
3. 复制模式长 Key 验证：data.prompt_key 长度 ≥ 95 时，不添加 `_copy` 后缀
4. 编辑→取消→新建序列验证：编辑 Modal 取消后重新创建，应显示默认值而非编辑数据
5. 复制→取消→新建序列验证：复制 Modal 取消后重新创建，isCopyPrompt 正确重置为 false
6. v1 编辑/复制模式验证：与 v2 行为一致

## Tasks

- [X] T011 [US3] 验证 v2 编辑模式下 initValues 不触发默认值，路径：frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx:170行-185行（initValues 计算逻辑）
      2. plan.md:阶段 8 编辑模式分析（data.prompt_key 为 truthy，`||` 不触发）
      3. feature_spec.md:FR-004
    - Restrictions: 不修改代码；确认编辑模式下 `data?.prompt_key` 为非空 truthy 值，`|| 'prompt_key_0'` 不会覆盖已有数据

- [X] T012 [US3] 验证 v2 复制模式下 initValues 走 isCopy 分支，路径：frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx:171行-175行（isCopy 三元运算 truthy 分支）
      2. feature_spec.md:FR-005
      3. plan.md:阶段 8 复制模式分析
    - Restrictions: 不修改代码；确认 isCopy=true 时走三元运算 truthy 分支，完全绕过 `||` 默认值逻辑

- [X] T013 [US3] 验证列表页「复制→取消→新建」操作序列中 isCopyPrompt 状态正确重置，路径：frontend/packages/loop-pages/prompt-pages/src/pages/list/index.tsx
    - Leverage:
      1. frontend/packages/loop-pages/prompt-pages/src/pages/list/index.tsx:109行-113行（onCancel 回调中 `setIsCopyPrompt(false)` + `createModal.close()`）
      2. mining-result.md:M-05（isCopyPrompt 状态残留风险分析）
      3. plan.md:TA-07（isCopyPrompt 重置分析）
    - Restrictions: 不修改代码；确认 onCancel 和 Modal 遮罩层关闭均触发 `setIsCopyPrompt(false)`

- [X] T014 [US3] 验证 v1 PromptCreate 编辑/复制模式不受默认值影响，路径：frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx:140行-157行（v1 initValues 计算逻辑）
      2. plan.md:阶段 5 验收场景
    - Restrictions: 不修改代码；确认 v1 的编辑/复制行为与 v2 一致

## Implementation Strategy For Each Phase

### 本 Phase 特殊说明

本 Phase **不涉及代码修改**。US-003 的功能由 `initValues` 中的条件逻辑天然保证：
- 编辑模式：`data?.prompt_key` 为 truthy → `||` 不触发默认值
- 复制模式：`isCopy ? ... : ...` → 走 truthy 分支，完全绕过 else 分支的默认值逻辑

### 验证策略

1. **条件分支追踪**：逐场景分析 `initValues` 的三元运算和 `||` 运算结果
2. **状态流转确认**：确认列表页的 `isCopyPrompt` 在取消操作后正确重置
3. **跨版本一致性**：对比 v1 和 v2 在编辑/复制模式下的行为

### 验证通过标准

- 编辑模式：Prompt Key 显示 `data.prompt_key`（非 `prompt_key_0`），且为 disabled 状态
- 复制模式：Prompt Key 显示 `data.prompt_key + '_copy'`（非 `prompt_key_0`）
- 「复制→取消→新建」序列：最终显示 `prompt_key_0`（默认值）
- v1 与 v2 在相同输入下行为一致
