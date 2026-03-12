# Phase 4: User Story US-002 — 默认值可修改验证

## 前置依赖（Prerequisites）

- Phase 1 完成（v2 initValues 默认值已添加）
- Phase 3 完成（US-001 新建模式预填默认值已验证）

## 目标

验证 US-002「预填默认值可以自由修改」——用户可以清除预填的默认值并输入自定义值，修改后的值能通过表单验证并正确用于 API 调用。本 Phase 不涉及代码修改。

## 独立验证方式

1. 修改验证：用户可选中并清除 `prompt_key_0`，输入自定义 Key，表单验证通过
2. 修改验证：用户可选中并清除 `prompt_demo_name_0`，输入自定义名称，表单验证通过
3. 提交验证：修改后的值正确传入 `CreatePrompt` API
4. 部分修改验证：仅修改 Prompt Key、保留名称默认值，提交时各字段使用正确值
5. 校验失败验证：输入不合法值（如 `123_invalid`）时显示格式错误提示

## Tasks

- [X] T009 [US2] 验证预填默认值的可编辑性和表单验证兼容性，路径：frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx:193行-218行（Prompt Key 的 FormInput rules 定义）
      2. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx:220行-249行（Prompt Name 的 FormInput rules 定义）
      3. feature_spec.md:FR-003（默认值符合表单验证规则）
      4. feature_spec.md:FR-006（默认值可被用户修改）
    - Restrictions: 不修改代码；确认 `prompt_key_0` 满足正则 `^[a-zA-Z][a-zA-Z0-9_.]*$` 且长度 ≤ 100；确认 `prompt_demo_name_0` 满足正则 `^[\u4e00-\u9fa5a-zA-Z0-9_.-]+$` 且不以 `_.-` 开头

- [X] T010 [US2] 验证修改后的值正确传入 API 调用，路径：frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx
    - Leverage:
      1. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx:106行-114行（handleOk 中 formApi.validate → createService.runAsync 流程）
      2. frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx:68行-79行（createService 的 API 参数构造）
      3. plan.md:阶段 7 验收场景
    - Restrictions: 不修改代码；确认 handleOk → validate() → runAsync(formData) 流程中，formData 使用的是表单当前值（用户修改后的值或默认值）

## Implementation Strategy For Each Phase

### 本 Phase 特殊说明

本 Phase **不涉及代码修改**。US-002 的功能由 Semi Design `FormInput` 组件天然支持——`initValues` 设置的值用户可自由编辑，`formApi.validate()` 获取的是表单当前值。

### 验证策略

1. **正则验证**：手动验证 `prompt_key_0` 和 `prompt_demo_name_0` 是否满足现有校验规则
   - `prompt_key_0` → `/^[a-zA-Z][a-zA-Z0-9_.]*$/.test('prompt_key_0')` → `true` ✅
   - `prompt_demo_name_0` → `/^[\u4e00-\u9fa5a-zA-Z0-9_.-]+$/.test('prompt_demo_name_0')` → `true` ✅
   - `prompt_demo_name_0` → `/^[_.-]/.test('prompt_demo_name_0')` → `false`（不以 `_.-` 开头）✅

2. **数据流追踪**：确认 `handleOk` 中 `formApi.current?.validate()` 返回的 formData 包含用户修改后的值（而非初始 initValues 值），并正确传给 `createService.runAsync()`

### 验证通过标准

- 默认值 `prompt_key_0` 和 `prompt_demo_name_0` 通过全部前端校验规则
- 用户修改默认值后，提交时 API 参数使用修改后的值
- 用户不修改直接提交时，API 参数使用默认值
