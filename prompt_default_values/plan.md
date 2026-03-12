# 技术实现方案：Prompt 创建表单预填默认值

## 文档信息

| 属性 | 值 |
|------|------|
| 版本 | 1.0 |
| 状态 | 草稿 |
| 来源 | feature_spec.md / mining-result.md / prd.md |

---

## 技术背景

### 项目概况

- **项目名称**：CozeLoop（coze-loop）
- **技术栈**：React + TypeScript + Rush.js (monorepo) + pnpm
- **UI 框架**：`@coze-arch/coze-design`（基于 Semi Design 封装）
- **国际化**：`@cozeloop/i18n-adapter`（基于 `intlClient` 封装的 `I18n.t()` 调用）
- **状态管理**：Zustand（`useShallow`）+ ahooks（`useRequest`）
- **表单组件**：`@coze-arch/coze-design` 的 `Form`、`FormInput`、`FormTextArea`、`withField`
- **API 层**：IDL 自动生成的 TypeScript 类型 + `createAPI` 封装

### 涉及的核心包与组件

| 包/模块 | 路径 | 说明 |
|---------|------|------|
| `prompt-components-v2` | `frontend/packages/loop-components/prompt-components-v2` | v2 版本 Prompt 组件库，主要修改目标 |
| `prompt-components` | `frontend/packages/loop-components/prompt-components` | v1 版本 Prompt 组件库，evaluate 模块使用 |
| `prompt-pages` | `frontend/packages/loop-pages/prompt-pages` | Prompt 列表页，消费 `PromptCreateModal` |
| `base-hooks` | `frontend/packages/loop-base/base-hooks` | 基础 Hooks（`useModalData`） |
| `api-schema` | `frontend/packages/loop-base/api-schema` | API 类型定义 |
| `i18n` | `frontend/packages/loop-base/i18n` | 国际化资源 |
| `evaluate-components` | `frontend/packages/loop-components/evaluate-components` | 评测组件（通过全局配置注入 v1 `PromptCreate`） |

### 核心组件关系

```
PromptCreateModal (v2)
├── 消费方 1: prompt-pages/list（列表页 → 新建/编辑/复制）
└── 消费方 2: prompt-develop/prompt-header（开发页 → 编辑/复制/快速创建）

PromptCreate (v1)
└── 消费方: evaluate-components/eval-global-config（全局配置注入）
```

### Modal 生命周期

`useModalData` hook 使用 `useState` 管理 `visible` 和 `data`：
- `open(data?)` → `setVisible(true)` + `setData(data)`
- `close()` → `setVisible(false)` + `setData(undefined)`

Semi Design `Modal` 在 `visible=false` 时**默认不销毁子组件 DOM**（`keepDOM` 默认 true），但 `Form` 的 `initValues` 仅在首次挂载时生效。不过，由于 `useModalData.close()` 会将 `data` 设为 `undefined`，当传入新的 `data`（或无 `data`）重新打开时，`initValues` 的计算值会变化，Semi Form 会在 `initValues` 引用变化时重新设置表单值。

---

## 章程检查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| PRD 需求覆盖 | ✅ | 所有 PRD 需求已覆盖，详见 feature_spec.md |
| 隐性需求覆盖 | ✅ | mining-result.md 中 16 项隐性需求已逐一分析并给出技术决策 |
| 组件规模控制 | ✅ | 改动量极小（< 10 行代码），不涉及组件拆分 |
| 向后兼容 | ✅ | 不修改组件 Props 接口，不影响已有消费方 |

---

## 技术分析

### TA-01：`initValues` 在 Modal 重新打开时是否重新生效

**问题**：Semi Design 的 `Form` 组件 `initValues` 仅在首次挂载时应用。如果 Modal 关闭后不卸载 DOM，再次打开时 `initValues` 不会重新应用。

**分析**：
1. 当前 `PromptCreateModal` 的 `initValues` 对象在每次渲染时重新计算（非 `useMemo` 缓存），其值依赖 `data`、`isCopy` 等 Props
2. Semi Design 的 `Form` 组件在 `initValues` **引用变化**时会调用 `formApi.setValues()` 重新应用值
3. 由于每次 `open()` 调用 `setData(val)` 会触发 `data` 变化，`initValues` 对象引用随之变化
4. 实际测试确认：即使 Modal DOM 不卸载，`initValues` 引用变化后表单值会正确重置

**决策**：依赖现有的 `initValues` 动态计算机制，**不需要额外的 `useEffect` + `formApi.setValues()`**。只需在 `initValues` 中添加默认值兜底逻辑即可。

---

### TA-02：空字符串 vs `undefined` 的默认值兜底运算符选择

**问题**：Playground「快速创建」场景中，传入的 `data` 对象 `prompt_key` 为空字符串 `""`。使用 `??`（空值合并）还是 `||`（逻辑或）决定了空字符串是否触发默认值填充。

**分析**：
- `??` 仅在 `null` / `undefined` 时返回右侧值，空字符串 `""` 不触发
- `||` 在所有 falsy 值（`null`、`undefined`、`""`、`0`、`false`）时返回右侧值
- Playground 快速创建时 `data.prompt_key` 为 `""`，期望填充默认值

**决策**：使用 `||` 运算符。`data?.prompt_key || 'prompt_key_0'` 确保 `undefined`、`null`、`""` 三种情况均填充默认值。

---

### TA-03：固定默认 Key 的唯一性冲突处理

**问题**：固定默认值 `prompt_key_0` 在多用户或同一用户第二次使用时几乎必然冲突（mining-result M-07）。

**分析**：
- PRD 明确标注"待澄清"，但未给出答案
- 当前已有的错误处理机制：`handleOk` 中 `runAsync` reject 后异常未被显式 catch
- 后端 API 在 Key 重复时返回错误

**决策**：**方案 D（保持当前行为）**——依赖后端返回重复错误提示。理由：
1. PRD 仅要求"预填默认值"，未要求自动递增或前端唯一性校验
2. 默认值的核心目的是降低空表单的认知负担，不是生成唯一 Key
3. 后续如需增强，可作为独立需求迭代（添加异步校验或自动递增）
4. 关于 `handleOk` 缺少错误处理（M-10），该问题是既有问题，不在本需求范围内修复

---

### TA-04：v1 与 v2 组件同步策略

**问题**：v1 `PromptCreate` 和 v2 `PromptCreateModal` 独立维护，是否需要同步修改（mining-result M-06、M-13）。

**分析**：
- v1 `PromptCreate` 被 `evaluate-components` 通过全局配置注入使用
- 评测模块调用时不传 `data`，相当于新建模式
- v1 组件的 `initValues` 结构与 v2 基本一致

**决策**：**同步修改 v1 和 v2**。改动量极小（仅 `initValues` 中加一个 `||` 兜底），保持两个版本行为一致，避免用户体验不一致。

---

### TA-05：Snippet 模式下的默认值处理

**问题**：`isSnippet=true` 时 Prompt Key 由 `nanoid` 自动生成，且输入框被隐藏。是否需要跳过默认值逻辑（mining-result M-09）。

**分析**：
- `isSnippet=true` 时 `FormInput[field="prompt_key"]` 不渲染（`isSnippet ? null : <FormInput ... />`）
- `handleOk` 中会用 `fornax.segment.{nanoid}` 覆盖 `prompt_key`
- `initValues` 中设置的值不会对用户产生影响

**决策**：**不做特殊处理**。`initValues` 中的默认值会被设置到 Form 状态中，但由于 Snippet 模式下 `handleOk` 会覆盖 `prompt_key`，默认值不会影响最终结果。添加额外的条件判断反而增加复杂度，与「最小改动」原则不符。

---

### TA-06：默认值实现方式选择

**问题**：默认值硬编码在组件内部，还是通过 Props 传入（mining-result M-15）。

**决策**：**硬编码方案**。理由：
1. PRD 明确固定值 `prompt_key_0` 和 `prompt_demo_name_0`
2. 当前无多消费方自定义默认值的需求
3. 硬编码实现最简洁，改动量最小
4. 若将来需要自定义，再扩展 Props 接口即可

---

### TA-07：`isCopyPrompt` 状态残留风险

**问题**：列表页「复制→取消→创建」操作序列中 `isCopyPrompt` 是否正确重置（mining-result M-05）。

**分析**：
- 当前代码在 `onCancel` 回调中已调用 `setIsCopyPrompt(false)`
- Semi Modal 的 `onCancel` 在点击遮罩层关闭时也会触发
- 因此 `isCopyPrompt` 不会出现状态残留

**决策**：**不需要额外处理**。现有逻辑已覆盖此场景。

---

## 组件分析

### 涉及组件清单

本需求改动范围极小，仅涉及两个文件中 `initValues` 的修改：

| 组件 | 文件路径 | 修改类型 | 说明 |
|------|----------|----------|------|
| `PromptCreateModal` (v2) | `prompt-components-v2/src/components/prompt-create-modal/index.tsx` | 修改 `initValues` | 主要修改目标 |
| `PromptCreate` (v1) | `prompt-components/src/prompt-create/index.tsx` | 修改 `initValues` | 同步修改保持一致 |

### 页面入口与动线

```
Prompt 列表页 → 点击「创建空白 Prompt」→ PromptCreateModal (v2) [新建模式]
Prompt 列表页 → 点击「编辑」→ PromptCreateModal (v2) [编辑模式，不预填默认值]
Prompt 列表页 → 点击「复制」→ PromptCreateModal (v2) [复制模式，不预填默认值]
Prompt 开发页 → Playground「快速创建」→ PromptCreateModal (v2) [新建模式，data 中 prompt_key 为空]
Prompt 开发页 → 点击编辑按钮 → PromptCreateModal (v2) [编辑模式]
评测模块 → 全局配置注入 → PromptCreate (v1) [新建模式]
```

### 不涉及的组件

以下组件和模块**不需要修改**：

- `prompt-pages/list/index.tsx`（消费方，无需改动）
- `prompt-develop/prompt-header/index.tsx`（消费方，无需改动）
- `evaluate-components/eval-global-config.ts`（全局配置注入，无需改动）
- `base-hooks/use-modal-data.ts`（Modal 状态管理，无需改动）
- `api-schema`（API 类型定义，无需改动）
- `i18n`（国际化资源，无需新增 key，默认值为硬编码常量非 i18n 文案）

---

## 实现阶段

### 阶段 1：视觉效果实现（组件预览）

**目标**：在 PromptCreateModal 组件预览页面中验证默认值预填效果。

**改动说明**：
本需求的 UI 变化仅为表单字段的初始值变化（从空变为预填值），不涉及新的 UI 组件或样式。组件预览页面需展示以下三种模式对比：

1. **新建模式**：Prompt Key 显示 `prompt_key_0`，Prompt 名称显示 `prompt_demo_name_0`
2. **编辑模式**：Prompt Key 显示已有值（如 `existing_key`），Prompt 名称显示已有值
3. **复制模式**：Prompt Key 显示已有值 + `_copy`，Prompt 名称显示已有值 + `_copy`

**预览页面结构**（Storybook 或独立预览页）：

```tsx
// 新建模式预览
<PromptCreateModal
  spaceID="preview"
  visible={true}
  onOk={() => {}}
  onCancel={() => {}}
/>
// → Prompt Key: "prompt_key_0", Prompt 名称: "prompt_demo_name_0"

// 编辑模式预览
<PromptCreateModal
  spaceID="preview"
  visible={true}
  isEdit={true}
  data={{ id: '1', prompt_key: 'existing_key', prompt_basic: { display_name: 'Existing Name' } }}
  onOk={() => {}}
  onCancel={() => {}}
/>
// → Prompt Key: "existing_key" (disabled), Prompt 名称: "Existing Name"

// 复制模式预览
<PromptCreateModal
  spaceID="preview"
  visible={true}
  isCopy={true}
  data={{ id: '1', prompt_key: 'existing_key', prompt_basic: { display_name: 'Existing Name' } }}
  onOk={() => {}}
  onCancel={() => {}}
/>
// → Prompt Key: "existing_key_copy", Prompt 名称: "Existing Name_copy"
```

**验收场景**：

```gherkin
Scenario: 新建模式下预览默认值
  Given 组件以新建模式渲染（无 data、isEdit=false、isCopy=false）
  When 组件加载完成
  Then Prompt Key 输入框显示 "prompt_key_0"
  And Prompt 名称输入框显示 "prompt_demo_name_0"
  And Prompt 描述输入框为空

Scenario: 编辑模式下预览已有数据
  Given 组件以编辑模式渲染（isEdit=true, data 包含已有值）
  When 组件加载完成
  Then Prompt Key 显示已有值 "existing_key"
  And Prompt Key 输入框为禁用状态
  And Prompt 名称显示已有值 "Existing Name"

Scenario: 复制模式下预览复制数据
  Given 组件以复制模式渲染（isCopy=true, data 包含已有值）
  When 组件加载完成
  Then Prompt Key 显示 "existing_key_copy"
  And Prompt 名称显示 "Existing Name_copy"
```

---

### 阶段 2：i18n 国际化文案

**目标**：确认本需求的国际化文案需求。

**分析**：
预填默认值 `prompt_key_0` 和 `prompt_demo_name_0` 是**技术标识符/常量**，不是面向用户的展示文案。它们：
- 不需要根据语言环境变化
- 是 Prompt Key 的合法值（满足正则 `^[a-zA-Z][a-zA-Z0-9_.]*$`）
- 用户预期它们是可直接用于创建的有效 Key

**决策**：**不需要新增 i18n key**。默认值作为硬编码常量直接写入 `initValues` 计算逻辑中。

**已有 i18n key 引用**（不修改，仅列出相关引用）：

| i18n key | 用途 | 备注 |
|----------|------|------|
| `prompt_please_input_prompt_key` | Prompt Key placeholder | 保持不变 |
| `prompt_please_input_prompt_key_caps` | Prompt Key 必填提示 | 保持不变 |
| `prompt_key_format` | Prompt Key 格式错误提示 | 保持不变 |
| `prompt_name` | Prompt 名称 label | 保持不变 |
| `prompt_name_format` | Prompt 名称格式错误提示 | 保持不变 |

**验收场景**：

```gherkin
Scenario: 默认值不受语言切换影响
  Given 系统语言为英文
  When 用户以新建模式打开创建表单
  Then Prompt Key 显示 "prompt_key_0"（与中文环境一致）
  And Prompt 名称显示 "prompt_demo_name_0"（与中文环境一致）
```

---

### 阶段 3：API 类型定义

**目标**：确认本需求的 API 类型定义需求。

**分析**：
本需求不涉及 API 接口变更：
- `CreatePromptRequest` 类型不变：`prompt_key`、`prompt_name` 等字段保持原有类型
- `CreatePromptResponse` 类型不变
- 默认值仅影响前端表单的 `initValues`，不影响 API 调用参数的类型定义

**已有 API 类型引用**（不修改，仅列出相关引用）：

| 类型 | 路径 | 说明 |
|------|------|------|
| `CreatePromptRequest` | `api-schema/src/api/idl/prompt/coze.loop.prompt.manage.ts` | 创建 Prompt 请求 |
| `CreatePromptResponse` | 同上 | 创建 Prompt 响应 |
| `Prompt` | `api-schema/src/api/idl/prompt/domain/prompt.ts` | Prompt 实体类型 |
| `PromptBasic` | 同上 | Prompt 基本信息（含 `display_name`） |

**决策**：**不需要新增或修改 API 类型定义**。

**验收场景**：

```gherkin
Scenario: 使用默认值创建 Prompt 的 API 调用参数正确
  Given 用户以新建模式打开表单且未修改默认值
  When 用户点击确认提交
  Then API 调用 CreatePrompt 的 prompt_key 参数为 "prompt_key_0"
  And API 调用 CreatePrompt 的 prompt_name 参数为 "prompt_demo_name_0"
```

---

### 阶段 4：PromptCreateModal (v2) — 默认值预填逻辑

**目标**：在 v2 版本 `PromptCreateModal` 组件的 `initValues` 中添加默认值兜底逻辑。

**修改文件**：`frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx`

**修改点**：仅修改 `<Form>` 组件的 `initValues` 属性中 `prompt_key` 和 `prompt_name` 的计算逻辑。

**当前代码**（第 140-157 行附近）：

```tsx
initValues={{
  prompt_key: isCopy
    ? `${
        (data?.prompt_key?.length || 0) < COPY_PROMPT_KEY_MAX_LEN
          ? `${data?.prompt_key}_copy`
          : data?.prompt_key
      }`
    : data?.prompt_key,
  prompt_name: isCopy
    ? `${
        (data?.prompt_basic?.display_name?.length || 0) <
        COPY_PROMPT_KEY_MAX_LEN
          ? `${data?.prompt_basic?.display_name}_copy`
          : data?.prompt_basic?.display_name
      }`
    : data?.prompt_basic?.display_name,
  prompt_description: data?.prompt_basic?.description,
  // ...
}}
```

**修改后代码**：

```tsx
initValues={{
  prompt_key: isCopy
    ? `${
        (data?.prompt_key?.length || 0) < COPY_PROMPT_KEY_MAX_LEN
          ? `${data?.prompt_key}_copy`
          : data?.prompt_key
      }`
    : data?.prompt_key || 'prompt_key_0',
  prompt_name: isCopy
    ? `${
        (data?.prompt_basic?.display_name?.length || 0) <
        COPY_PROMPT_KEY_MAX_LEN
          ? `${data?.prompt_basic?.display_name}_copy`
          : data?.prompt_basic?.display_name
      }`
    : data?.prompt_basic?.display_name || 'prompt_demo_name_0',
  prompt_description: data?.prompt_basic?.description,
  // ...
}}
```

**改动说明**：
- `data?.prompt_key` → `data?.prompt_key || 'prompt_key_0'`
- `data?.prompt_basic?.display_name` → `data?.prompt_basic?.display_name || 'prompt_demo_name_0'`
- 使用 `||` 运算符（非 `??`），确保空字符串 `""` 也触发默认值（覆盖 Playground 快速创建场景，技术决策 TA-02）
- `isCopy` 为 `true` 时走原有复制逻辑，不受影响
- `isEdit` 为 `true` 时 `data` 必定包含有效值，`||` 不会触发默认值

**影响分析**：

| 模式 | `data?.prompt_key` 值 | 使用 `\|\|` 后的结果 | 是否正确 |
|------|----------------------|---------------------|----------|
| 新建（列表页） | `undefined` | `'prompt_key_0'` | ✅ |
| 新建（Playground 快速创建） | `''` | `'prompt_key_0'` | ✅ |
| 编辑 | `'existing_key'`（truthy） | `'existing_key'` | ✅ |
| 复制 | N/A（走 `isCopy` 分支） | N/A | ✅ |

**Props 接口**：不变，不新增任何 Props。

**验收场景**：

```gherkin
Scenario: 新建模式预填默认值（spec AC 复制）
  Given 用户在 Prompt 列表页
  When 用户点击「创建空白 Prompt」按钮
  Then 弹出创建表单 Modal
  And Prompt Key 输入框显示 "prompt_key_0"
  And Prompt 名称输入框显示 "prompt_demo_name_0"
  And Prompt 描述输入框为空

Scenario: 默认值可修改（spec AC 复制）
  Given 创建表单已预填默认值
  When 用户清除 Prompt Key 并输入 "my_custom_key"
  And 用户清除 Prompt 名称并输入 "My Custom Name"
  Then 表单验证通过
  And 提交时使用修改后的值

Scenario: 默认值可直接提交（spec AC 复制）
  Given 创建表单已预填默认值
  When 用户不修改任何字段直接点击确认
  Then 表单验证通过
  And 调用 CreatePrompt API，prompt_key 为 "prompt_key_0"，prompt_name 为 "prompt_demo_name_0"

Scenario: 编辑模式不受默认值影响（spec AC 复制）
  Given 用户以编辑模式打开表单（isEdit=true, data 包含 prompt_key="existing_key"）
  When 表单加载完成
  Then Prompt Key 显示 "existing_key"（非默认值）
  And Prompt Key 输入框为禁用状态
  And Prompt 名称显示已有值（非默认值）

Scenario: 复制模式不受默认值影响（spec AC 复制）
  Given 用户以复制模式打开表单（isCopy=true, data 包含 prompt_key="existing_key"）
  When 表单加载完成
  Then Prompt Key 显示 "existing_key_copy"（非默认值）
  And Prompt 名称显示已有值 + "_copy"（非默认值）

Scenario: Playground 快速创建场景预填默认值（扩充）
  Given 用户在 Playground 页面
  When 用户点击「快速创建」按钮
  Then 弹出创建表单，data.prompt_key 为空字符串
  And Prompt Key 输入框显示 "prompt_key_0"（|| 运算符正确处理空字符串）
  And Prompt 名称输入框显示 "prompt_demo_name_0"

Scenario: Snippet 模式下默认值不影响结果（扩充）
  Given 用户以 Snippet 模式创建（isSnippet=true）
  When 表单提交
  Then Prompt Key 被覆盖为 "fornax.segment.{nanoid}" 格式
  And initValues 中的默认值不影响最终结果

Scenario: Modal 关闭后重新打开恢复默认值（扩充）
  Given 用户打开创建 Modal 并修改了 Prompt Key 为 "modified_key"
  When 用户取消关闭 Modal
  And 用户再次点击「创建空白 Prompt」
  Then Prompt Key 显示 "prompt_key_0"（而非上次修改的 "modified_key"）

Scenario: 默认值符合表单验证规则（spec AC 复制）
  Given 默认值 prompt_key_0
  When 验证正则 ^[a-zA-Z][a-zA-Z0-9_.]*$
  Then 验证通过
  Given 默认值 prompt_demo_name_0
  When 验证正则 ^[\u4e00-\u9fa5a-zA-Z0-9_.-]+$ 且不以 _.- 开头
  Then 验证通过
```

---

### 阶段 5：PromptCreate (v1) — 默认值预填逻辑

**目标**：在 v1 版本 `PromptCreate` 组件的 `initValues` 中添加相同的默认值兜底逻辑，保持 v1/v2 行为一致。

**修改文件**：`frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx`

**修改点**：与阶段 4 相同，仅修改 `<Form>` 组件的 `initValues` 属性。

**当前代码**（第 90-105 行附近）：

```tsx
initValues={{
  prompt_key: isCopy
    ? `${
        (data?.prompt_key?.length || 0) < 95
          ? `${data?.prompt_key}_copy`
          : data?.prompt_key
      }`
    : data?.prompt_key,
  prompt_name: isCopy
    ? `${
        (data?.prompt_basic?.display_name?.length || 0) < 95
          ? `${data?.prompt_basic?.display_name}_copy`
          : data?.prompt_basic?.display_name
      }`
    : data?.prompt_basic?.display_name,
  prompt_description: data?.prompt_basic?.description,
}}
```

**修改后代码**：

```tsx
initValues={{
  prompt_key: isCopy
    ? `${
        (data?.prompt_key?.length || 0) < 95
          ? `${data?.prompt_key}_copy`
          : data?.prompt_key
      }`
    : data?.prompt_key || 'prompt_key_0',
  prompt_name: isCopy
    ? `${
        (data?.prompt_basic?.display_name?.length || 0) < 95
          ? `${data?.prompt_basic?.display_name}_copy`
          : data?.prompt_basic?.display_name
      }`
    : data?.prompt_basic?.display_name || 'prompt_demo_name_0',
  prompt_description: data?.prompt_basic?.description,
}}
```

**改动说明**：
- 与阶段 4 完全一致的修改模式
- v1 组件不支持 `isSnippet`，无需考虑 Snippet 场景
- v1 组件被 `evaluate-components` 通过全局配置注入使用，接口定义为 `{ visible, onCancel, onOk }`，不传 `data`，因此始终走新建模式 → 默认值生效

**Props 接口**：不变。

**验收场景**：

```gherkin
Scenario: v1 组件新建模式预填默认值
  Given 评测模块通过全局配置调用 PromptCreate（v1）
  When 组件以 visible=true 渲染，无 data 传入
  Then Prompt Key 输入框显示 "prompt_key_0"
  And Prompt 名称输入框显示 "prompt_demo_name_0"

Scenario: v1 与 v2 行为一致
  Given v1 PromptCreate 和 v2 PromptCreateModal 均以新建模式打开
  When 两者均无 data 传入
  Then 两者的 Prompt Key 默认值相同（"prompt_key_0"）
  And 两者的 Prompt 名称默认值相同（"prompt_demo_name_0"）

Scenario: v1 编辑模式不受默认值影响
  Given v1 PromptCreate 以编辑模式打开（isEdit=true, data 包含有效值）
  When 组件加载完成
  Then Prompt Key 显示已有值（非默认值）

Scenario: v1 复制模式不受默认值影响
  Given v1 PromptCreate 以复制模式打开（isCopy=true, data 包含有效值）
  When 组件加载完成
  Then Prompt Key 显示已有值 + "_copy"（非默认值）
```

---

### 阶段 6：US-001 集成 — 新建模式预填默认值

**目标**：集成 User Story US-001「点击创建空白 Prompt 后表单预填默认值」的完整端到端流程。

**引用前序阶段**：
- 阶段 4（PromptCreateModal v2 `initValues` 修改）
- 阶段 5（PromptCreate v1 `initValues` 修改）

**集成流程**：

1. 用户在 Prompt 列表页点击「创建空白 Prompt」
2. `prompt-pages/list` 中调用 `createModal.open()`（无参数）
3. `useModalData.open()` 设置 `visible=true`、`data=undefined`
4. `PromptCreateModal` 渲染，`data` 为 `undefined`
5. `initValues` 计算：
   - `isCopy=false` → 走 else 分支
   - `data?.prompt_key` → `undefined`
   - `undefined || 'prompt_key_0'` → `'prompt_key_0'` ✅
   - `data?.prompt_basic?.display_name` → `undefined`
   - `undefined || 'prompt_demo_name_0'` → `'prompt_demo_name_0'` ✅
6. 表单显示预填默认值

**状态管理**：不涉及额外状态管理。默认值完全通过 `initValues` 静态计算得出。

**API 调用**：使用已有的 `CreatePrompt` API，参数中 `prompt_key` 和 `prompt_name` 使用表单值（默认值或用户修改后的值）。

**验收场景**：

```gherkin
Scenario: US-001 端到端验证（spec AC 完整复制）
  Given 用户在 Prompt 开发页面
  When 用户点击「创建空白 Prompt」按钮
  Then 系统弹出创建表单 Modal
  And Prompt Key 输入框显示 "prompt_key_0"
  And Prompt 名称输入框显示 "prompt_demo_name_0"
  And Prompt 描述为空

Scenario: US-001 从开发页 Header 创建（扩充）
  Given 用户在 Prompt 开发页面（非 Playground 模式）
  When prompt-header 中无直接创建按钮（仅编辑/复制/删除）
  Then 不影响 US-001（创建入口在列表页）

Scenario: US-001 从 Playground 快速创建（扩充）
  Given 用户在 Playground 页面
  When 用户点击「快速创建」按钮
  Then 弹出创建表单 Modal
  And data.prompt_key 为空字符串
  And Prompt Key 输入框显示 "prompt_key_0"（|| 运算符处理空字符串）
  And Prompt 名称输入框显示 "prompt_demo_name_0"

Scenario: US-001 从评测模块创建（扩充）
  Given 评测模块通过全局配置注入 PromptCreate (v1)
  When 组件以 visible=true 渲染（无 data）
  Then Prompt Key 输入框显示 "prompt_key_0"
  And Prompt 名称输入框显示 "prompt_demo_name_0"
```

---

### 阶段 7：US-002 集成 — 默认值可修改

**目标**：集成 User Story US-002「预填默认值可以自由修改」。

**引用前序阶段**：
- 阶段 4（PromptCreateModal v2 `initValues` 修改）

**集成说明**：
此 User Story **无需额外代码改动**。Semi Design 的 `FormInput` 默认为可编辑状态。`initValues` 设置的值用户可以自由修改。修改后的值通过 `formApi.validate()` 获取，并传给 `createService.runAsync(formData)` 进行 API 调用。

**交互流程**：

1. 用户在表单中看到预填的 `prompt_key_0`
2. 用户选中并清除，输入 `my_custom_key`
3. 用户点击确认
4. `formApi.validate()` 返回 `{ prompt_key: 'my_custom_key', ... }`
5. API 调用使用修改后的值

**验收场景**：

```gherkin
Scenario: US-002 用户修改 Prompt Key（spec AC 完整复制）
  Given 创建表单已预填 Prompt Key 为 "prompt_key_0"
  When 用户清除 Prompt Key 并输入 "my_custom_key"
  Then Prompt Key 输入框显示 "my_custom_key"
  And 表单验证通过

Scenario: US-002 用户修改 Prompt 名称（spec AC 完整复制）
  Given 创建表单已预填 Prompt 名称为 "prompt_demo_name_0"
  When 用户清除 Prompt 名称并输入 "My Custom Name"
  Then Prompt 名称输入框显示 "My Custom Name"
  And 表单验证通过

Scenario: US-002 修改后的值用于 API 调用（spec AC 完整复制）
  Given 用户已修改 Prompt Key 为 "my_custom_key"，Prompt 名称为 "My Custom Name"
  When 用户点击确认提交
  Then CreatePrompt API 的 prompt_key 参数为 "my_custom_key"
  And CreatePrompt API 的 prompt_name 参数为 "My Custom Name"

Scenario: US-002 修改后的值仍需通过表单验证（扩充）
  Given 用户清除 Prompt Key 并输入 "123_invalid"
  When 用户点击确认
  Then 表单验证失败
  And 显示格式错误提示（prompt_key_format）

Scenario: US-002 部分修改场景（扩充）
  Given 用户仅修改 Prompt Key 为 "custom_key"，保留 Prompt 名称默认值
  When 用户点击确认
  Then CreatePrompt API 的 prompt_key 参数为 "custom_key"
  And CreatePrompt API 的 prompt_name 参数为 "prompt_demo_name_0"
```

---

### 阶段 8：US-003 集成 — 编辑/复制模式不受影响

**目标**：集成 User Story US-003「预填默认值不影响已有的编辑、复制等表单行为」。

**引用前序阶段**：
- 阶段 4（PromptCreateModal v2 `initValues` 修改）
- 阶段 5（PromptCreate v1 `initValues` 修改）

**集成说明**：
此 User Story 通过 `initValues` 的条件逻辑天然保证：

1. **编辑模式**（`isEdit=true`）：
   - 必传 `data`，且 `data.prompt_key` 为有效非空值（truthy）
   - `data?.prompt_key || 'prompt_key_0'` → `data.prompt_key`（truthy，不触发 `||`）
   - 结果：显示已有值 ✅

2. **复制模式**（`isCopy=true`）：
   - 走 `isCopy` 三元运算的 truthy 分支，直接返回 `data.prompt_key + '_copy'`
   - 不走 `||` 兜底逻辑
   - 结果：显示已有值 + `_copy` ✅

**验收场景**：

```gherkin
Scenario: US-003 编辑模式显示已有值（spec AC 完整复制）
  Given 用户以编辑模式打开表单，data.prompt_key="existing_key"，data.prompt_basic.display_name="Existing Name"
  When 表单加载完成
  Then Prompt Key 显示 "existing_key"（非 "prompt_key_0"）
  And Prompt Key 为禁用状态
  And Prompt 名称显示 "Existing Name"（非 "prompt_demo_name_0"）

Scenario: US-003 复制模式显示带后缀的值（spec AC 完整复制）
  Given 用户以复制模式打开表单，data.prompt_key="existing_key"
  When 表单加载完成
  Then Prompt Key 显示 "existing_key_copy"（非 "prompt_key_0"）
  And Prompt 名称显示 "Existing Name_copy"（非 "prompt_demo_name_0"）

Scenario: US-003 复制模式长 Key 不添加后缀（spec AC 扩充）
  Given 用户以复制模式打开表单，data.prompt_key 长度 >= 95
  When 表单加载完成
  Then Prompt Key 显示原 Key（不添加 "_copy" 后缀）

Scenario: US-003 编辑→取消→新建序列（扩充）
  Given 用户先以编辑模式打开表单（显示 "existing_key"）
  When 用户取消关闭 Modal
  And 用户点击「创建空白 Prompt」（新建模式）
  Then Prompt Key 显示 "prompt_key_0"（默认值，非上次编辑的 "existing_key"）
  And Prompt 名称显示 "prompt_demo_name_0"

Scenario: US-003 复制→取消→新建序列（扩充）
  Given 用户先以复制模式打开表单（显示 "existing_key_copy"）
  When 用户取消关闭 Modal（isCopyPrompt 被重置为 false）
  And 用户点击「创建空白 Prompt」（新建模式）
  Then Prompt Key 显示 "prompt_key_0"（默认值）
  And isCopyPrompt 状态已正确重置
```

---
