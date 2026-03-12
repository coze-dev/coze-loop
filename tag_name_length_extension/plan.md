# 技术实现方案：标签名称字符长度扩展

## 文档信息

| 项目 | 内容 |
|------|------|
| 需求来源 | feature_spec.md |
| 隐性需求参考 | mining-result.md |
| 目标 | 将标签名称长度上限从 50 扩展到 100 |
| 涉及包 | `@cozeloop/tag-components`、`loop-lng` |

---

## 技术分析

### 决策 1：仅修改 `MAX_TAG_NAME_LENGTH`，不修改 `MAX_TAG_LENGTH`

**背景**：`const/index.ts` 中存在两个值均为 50 的常量：
- `MAX_TAG_LENGTH = 50`：标签选项数量上限，被 `AnnotationContent` 用于限制标注面板中标签数量
- `MAX_TAG_NAME_LENGTH = 50`：标签名称字符长度上限，被 `tagNameValidate` 引用

**决策**：仅将 `MAX_TAG_NAME_LENGTH` 修改为 100，`MAX_TAG_LENGTH` 保持 50 不变。在代码审查中特别标注此区分。

---

### 决策 2：`maxLength` 硬编码改为引用常量

**背景**：`TagsForm` 组件中存在 4 处 `maxLength={50}` 硬编码（标签名称 1 处、分类型选项值 1 处、布尔型选项值 2 处），与 `MAX_TAG_NAME_LENGTH` 常量脱节。

**决策**：将 4 处 `maxLength={50}` 全部替换为 `maxLength={MAX_TAG_NAME_LENGTH}`，统一由常量驱动，消除未来修改遗漏风险。

---

### 决策 3：i18n 文案直接修改数字，不做参数化

**背景**：`tag_name_length_limit` 的中文文案为 `"标签名称必须为 1～50 字符长度"`，英文为 `"Tag name must be 1-50 characters long"`，均硬编码了 `50`。mining-result 建议改为参数化（`{max}`）。

**决策**：本次需求仅将文案中的 `50` 替换为 `100`，不做参数化改造。原因：
1. 参数化需要确认 `@cozeloop/i18n-adapter` 的插值语法支持情况
2. 该文案变更频率极低，参数化收益不大
3. 保持最小变更范围，降低风险

---

### 决策 4：不引入防抖/竞态控制

**背景**：mining-result（M-004）指出唯一性校验（`useTagNameValidateUniqBySpace`）在每次 blur 时发起 API 请求，无防抖和竞态控制。

**决策**：本次需求不改动唯一性校验逻辑。原因：
1. 长度扩展不改变 blur 触发频率
2. 防抖/竞态控制属于优化类需求，超出本次 PRD 范围
3. 记录为后续技术债务

---

### 决策 5：不新增 `maxNameLength` prop

**背景**：mining-result（M-009）建议将标签名称最大长度外部化为 `TagsForm` 的 prop。

**决策**：本次不新增 prop。当前所有消费者（`TagsCreatePage`、`TagsDetail`、`useTagFormModal`）使用统一的 100 字符限制，无差异化需求。通过常量统一控制即可。

---

### 决策 6：长名称 UI 展示无需额外适配

**背景**：mining-result（M-005、M-006）指出标签名扩展到 100 字符后，列表/选择器/面包屑/弹窗中长名称展示需验证。

**决策**：现有组件已具备溢出处理能力，无需额外代码变更：
- `TagsItem`：使用 `Typography.Text` 的 `ellipsis={{ rows: 1 }}` 单行省略
- `TagsSelect`：继承 `TagsItem` 的溢出处理
- 面包屑：`useBreadcrumb` 由框架控制，有内置截断
- `useTagFormModal` 弹窗：`width={600}` 足以容纳 100 字符的输入框，输入框自动横向滚动

验收时需人工确认长名称在以上场景的视觉表现，但不需要代码变更。

---

## 组件分析

### 组件清单

本次需求涉及的组件和文件变更：

| 编号 | 组件/文件 | 路径 | 变更类型 |
|------|-----------|------|----------|
| C-1 | 常量定义 | `tag-components/src/const/index.ts` | 修改常量值 |
| C-2 | 校验函数 | `tag-components/src/utils/validate.ts` | 无需修改（引用常量） |
| C-3 | TagsForm | `tag-components/src/components/tags-form/index.tsx` | 修改 maxLength 硬编码 |
| C-4 | i18n 中文 | `loop-lng/src/locales/tag/zh-CN.json` | 修改文案数字 |
| C-5 | i18n 英文 | `loop-lng/src/locales/tag/en-US.json` | 修改文案数字 |

### 组件依赖关系

```
const/index.ts (MAX_TAG_NAME_LENGTH = 100)
  └── utils/validate.ts (tagNameValidate 引用常量)
       └── components/tags-form/index.tsx (调用 tagNameValidate + maxLength)
            ├── pages/tags-create-page.tsx (新建标签)
            ├── components/tags-detail/content/index.tsx (编辑标签)
            └── hooks/use-tag-form-modal.tsx (弹窗模式新建/编辑)

loop-lng/src/locales/tag/zh-CN.json (tag_name_length_limit 文案)
loop-lng/src/locales/tag/en-US.json (tag_name_length_limit 文案)
  └── utils/validate.ts (I18n.t('tag_name_length_limit') 引用)
```

### 不变更文件（需明确排除）

| 文件 | 原因 |
|------|------|
| `annotation-content.tsx` | 使用 `MAX_TAG_LENGTH`（标签选项数量限制），与名称长度无关 |
| `tags-select/index.tsx` | 无 maxLength 逻辑，仅展示标签列表 |
| `tags-item/index.tsx` | 已有 `ellipsis` 溢出处理，无需修改 |
| `tags-detail/index.tsx` | 容器组件，无校验逻辑 |
| `tags-create-page.tsx` | 仅引用 TagsForm，无独立校验逻辑 |

---

## 实现阶段

---

### 阶段 1：常量与校验层变更

**目标**：修改标签名称长度上限常量，校验函数自动适配。

**涉及文件**：
- `frontend/packages/loop-components/tag-components/src/const/index.ts`

**变更内容**：

```typescript
// 修改前
export const MAX_TAG_NAME_LENGTH = 50;

// 修改后
export const MAX_TAG_NAME_LENGTH = 100;
```

**⚠️ 注意**：`MAX_TAG_LENGTH = 50` 保持不变，此常量控制标签选项数量上限。

**校验函数无需修改**：`tagNameValidate`（`utils/validate.ts`）已通过引用 `MAX_TAG_NAME_LENGTH` 进行长度判断，常量变更后自动生效。

**验收场景**：

```gherkin
Given 常量 MAX_TAG_NAME_LENGTH 已修改为 100
When tagNameValidate 接收 100 字符的合法名称
Then 返回空字符串（校验通过）

Given 常量 MAX_TAG_NAME_LENGTH 已修改为 100
When tagNameValidate 接收 101 字符的合法名称
Then 返回 I18n.t('tag_name_length_limit') 错误信息

Given 常量 MAX_TAG_LENGTH 保持为 50
When AnnotationContent 检查标签数量限制
Then 标签数量上限仍为 50（不受影响）
```

---

### 阶段 2：i18n 国际化文案更新

**目标**：将校验错误提示文案中的长度数字从 50 更新为 100。

**涉及文件**：
- `frontend/packages/loop-base/loop-lng/src/locales/tag/zh-CN.json`
- `frontend/packages/loop-base/loop-lng/src/locales/tag/en-US.json`

**变更内容**：

zh-CN.json：
```json
// 修改前
"tag_name_length_limit": "标签名称必须为 1～50 字符长度"

// 修改后
"tag_name_length_limit": "标签名称必须为 1～100 字符长度"
```

en-US.json：
```json
// 修改前
"tag_name_length_limit": "Tag name must be 1-50 characters long"

// 修改后
"tag_name_length_limit": "Tag name must be 1-100 characters long"
```

**验收场景**：

```gherkin
Given 语言设置为中文
When 用户输入空标签名称并触发 blur 校验
Then 错误提示显示 "标签名称必须为 1～100 字符长度"

Given 语言设置为英文
When 用户输入超过 100 字符的标签名称并触发 blur 校验
Then 错误提示显示 "Tag name must be 1-100 characters long"
```

---

### 阶段 3：TagsForm 组件 maxLength 硬编码修正

**目标**：将 `TagsForm` 组件中 4 处 `maxLength={50}` 硬编码替换为引用 `MAX_TAG_NAME_LENGTH` 常量。

**涉及文件**：
- `frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx`

**变更内容**：

1. **导入常量**：在现有 import 中添加 `MAX_TAG_NAME_LENGTH`：
```typescript
// 修改前
import { TAG_TYPE_OPTIONS, MAX_TAG_LENGTH } from '@/const';

// 修改后
import { TAG_TYPE_OPTIONS, MAX_TAG_LENGTH, MAX_TAG_NAME_LENGTH } from '@/const';
```

2. **标签名称输入框**（第 133 行）：
```tsx
// 修改前
maxLength={50}

// 修改后
maxLength={MAX_TAG_NAME_LENGTH}
```

3. **分类型选项值输入框**（第 220 行）：
```tsx
// 修改前
maxLength={50}

// 修改后
maxLength={MAX_TAG_NAME_LENGTH}
```

4. **布尔型选项一输入框**（第 310 行）：
```tsx
// 修改前
maxLength={50}

// 修改后
maxLength={MAX_TAG_NAME_LENGTH}
```

5. **布尔型选项二输入框**（第 334 行）：
```tsx
// 修改前
maxLength={50}

// 修改后
maxLength={MAX_TAG_NAME_LENGTH}
```

**⚠️ 注意**：描述字段的 `maxLength={200}` 和 `maxCount={200}` 不修改（描述长度限制不变）。

**验收场景**：

```gherkin
Given 用户打开新建标签表单
When 在标签名称输入框键入第 101 个字符
Then 输入被阻止，输入框最多显示 100 个字符

Given 用户打开新建标签表单
When 在标签名称输入框粘贴 150 个字符的内容
Then 浏览器原生 maxLength 截断至 100 个字符

Given 用户创建分类型标签
When 在选项值输入框键入第 101 个字符
Then 输入被阻止，输入框最多显示 100 个字符

Given 用户创建布尔型标签
When 在选项一/选项二输入框键入第 101 个字符
Then 输入被阻止，输入框最多显示 100 个字符
```

---

### 阶段 4：User Story US-001 — 新建标签支持 100 字符

**目标**：验证新建标签流程中标签名称可输入最多 100 个字符。

**涉及组件**：
- `TagsCreatePage`（引用 `TagsForm`，entry="crete-tag"）
- `useTagFormModal`（弹窗模式新建，entry="crete-tag"）

**实现说明**：
- 本阶段无额外代码变更，所有变更已在阶段 1～3 完成
- `TagsCreatePage` 和 `useTagFormModal` 通过 `TagsForm` 间接获得新的长度限制
- 校验逻辑通过 `tagNameValidate` → `MAX_TAG_NAME_LENGTH` 链路自动生效
- 输入截断通过 `maxLength={MAX_TAG_NAME_LENGTH}` 自动生效

**验收场景**（复制自 feature_spec.md + 扩充）：

```gherkin
# --- 来自 feature_spec.md 验收场景 ---

Given 用户进入新建标签页面
When 输入 1 个字符的标签名称
Then 校验通过，允许提交

Given 用户进入新建标签页面
When 输入 50 个字符的标签名称
Then 校验通过，允许提交

Given 用户进入新建标签页面
When 输入 100 个字符的标签名称
Then 校验通过，允许提交

Given 用户进入新建标签页面
When 输入中文、英文、数字、下划线混合的标签名称（≤100字符）
Then 校验通过，允许提交

Given 用户进入新建标签页面
When 不输入任何字符直接提交
Then 提示长度错误："标签名称必须为 1～100 字符长度"

Given 用户进入新建标签页面
When 输入 101 个字符的标签名称
Then 输入框阻止输入（maxLength 限制）

Given 用户进入新建标签页面
When 粘贴超过 100 字符的内容到标签名称输入框
Then 截断至 100 字符

Given 用户进入新建标签页面
When 输入特殊字符（@#$%）
Then 提示字符不合法："标签名称仅支持输入中文、英文、数字和下划线"

Given 用户进入新建标签页面
When 输入与空间内已有标签相同的名称
Then 提示名称重复："同空间内标签名称不允许重复"

# --- 扩充场景 ---

Given 用户通过弹窗模式（useTagFormModal）新建标签
When 输入 100 个字符的标签名称并提交
Then 标签创建成功，弹窗关闭，显示成功提示

Given 用户通过新建标签页面提交
When 标签名称为 100 个字符的合法名称
Then API 请求成功，页面跳转至标签列表
```

---

### 阶段 5：User Story US-002 — 编辑标签支持 100 字符

**目标**：验证编辑标签流程中标签名称同样支持最多 100 个字符。

**涉及组件**：
- `TagsDetail` → `TagDetailContent`（引用 `TagsForm`，entry="edit-tag"）
- `useTagFormModal`（弹窗模式编辑，entry="edit-tag"）

**实现说明**：
- 本阶段无额外代码变更，所有变更已在阶段 1～3 完成
- 编辑模式与新建模式共用 `TagsForm` 组件，校验和截断逻辑一致
- `formatTagDetailToFormValues` 将后端数据转为表单初始值，不涉及长度处理
- `isEqual` 变更检测在长名称场景下工作正常（字符串比较与长度无关）

**验收场景**（复制自 feature_spec.md + 扩充）：

```gherkin
# --- 来自 feature_spec.md 验收场景 ---

Given 用户进入编辑标签页面（TagsDetail）
When 将已有标签名称修改为 100 字符以内
Then 校验通过，允许保存

Given 用户进入编辑标签页面
When 保持原名称不变
Then 校验通过（且变更检测判定为"无变更"，保存按钮不高亮）

Given 用户进入编辑标签页面
When 清空名称
Then 提示长度错误："标签名称必须为 1～100 字符长度"

Given 用户进入编辑标签页面
When 修改为 101 字符
Then 输入框阻止输入

# --- 扩充场景 ---

Given 用户通过弹窗模式（useTagFormModal）编辑标签
When 修改标签名称为 100 个字符的合法名称并保存
Then 更新成功，弹窗关闭，显示成功提示

Given 已有标签名称为 30 个字符
When 用户修改为 80 个字符后未保存，尝试离开页面
Then 弹出离开拦截弹窗："修改还未提交，退出后将不会保存此次修改。"

Given 已有标签名称为 50 个字符（旧上限）
When 用户在编辑模式下查看
Then 名称正常回显，可继续编辑至 100 字符
```

---

### 阶段 6：User Story US-003 — 错误提示准确性

**目标**：确保所有校验场景的错误提示文案准确反映新的 100 字符限制。

**涉及组件**：
- `tagNameValidate`（校验函数）
- i18n 文案：`tag_name_length_limit`、`tag_name_valid_chars`、`tag_name_no_duplicate_space`

**实现说明**：
- 本阶段无额外代码变更，阶段 1～2 已完成所有相关修改
- 需逐一验证以下校验场景的提示文案：

**验收场景**（复制自 feature_spec.md FR-005 + 扩充）：

```gherkin
# --- 来自 feature_spec.md 验收场景 ---

Given 用户输入的标签名称为空
When 触发 blur 校验
Then 提示："标签名称必须为 1～100 字符长度"

Given 用户输入的标签名称超过 100 字符（理论上被 maxLength 阻止，但校验函数仍覆盖）
When 触发 blur 校验
Then 提示："标签名称必须为 1～100 字符长度"

Given 用户输入包含特殊字符（@#$%空格）的标签名称
When 触发 blur 校验
Then 提示："标签名称仅支持输入中文、英文、数字和下划线"

Given 用户输入的标签名称与空间内已有标签重名
When 触发 blur 校验（异步唯一性校验）
Then 提示："同空间内标签名称不允许重复"

# --- 扩充场景 ---

Given 语言切换为英文（en-US）
When 输入空标签名称并触发 blur 校验
Then 提示："Tag name must be 1-100 characters long"

Given 分类型标签选项值为空
When 触发校验
Then 提示："标签值不能为空"（不受本次变更影响）

Given 分类型标签两个选项值名称相同
When 触发校验
Then 提示："一个标签内的标签值不允许重复"（不受本次变更影响）
```

---

### 阶段 7：User Story 补充 — 标签选项值名称长度同步扩展

**目标**：确保分类型和布尔型标签的选项值名称同样支持最多 100 个字符。

**涉及组件**：
- `TagsForm`（分类型选项值输入框、布尔型选项一/选项二输入框）

**实现说明**：
- 本阶段无额外代码变更，阶段 3 已将所有选项值输入框的 `maxLength` 统一为 `MAX_TAG_NAME_LENGTH`
- 选项值名称的校验函数复用 `tagNameValidate`，长度规则通过 `MAX_TAG_NAME_LENGTH` 自动同步
- 选项值的唯一性校验（`tagValidateNameUniqByOptions`）不涉及长度限制，无需修改

**验收场景**（复制自 feature_spec.md + 扩充）：

```gherkin
# --- 来自 feature_spec.md 验收场景 ---

Given 用户创建分类型标签
When 选项值名称输入 1～100 字符
Then 校验通过

Given 用户创建分类型标签
When 选项值名称超过 100 字符
Then 输入框阻止输入

Given 用户创建布尔型标签
When 选项值名称输入 1～100 字符
Then 校验通过

Given 用户创建布尔型标签
When 选项值名称超过 100 字符
Then 输入框阻止输入

# --- 扩充场景 ---

Given 用户编辑已有分类型标签
When 修改某选项值名称为 100 个字符
Then 校验通过，允许保存

Given 用户编辑已有分类型标签
When 新增选项值，名称输入 100 个字符
Then 校验通过，选项成功添加

Given 用户创建分类型标签并添加多个选项
When 两个选项值名称相同（均为 80 个字符）
Then 提示："一个标签内的标签值不允许重复"
```

---

## 变更文件汇总

| 序号 | 文件路径 | 变更描述 | 对应阶段 |
|------|----------|----------|----------|
| 1 | `frontend/packages/loop-components/tag-components/src/const/index.ts` | `MAX_TAG_NAME_LENGTH` 从 50 改为 100 | 阶段 1 |
| 2 | `frontend/packages/loop-base/loop-lng/src/locales/tag/zh-CN.json` | `tag_name_length_limit` 文案中 50 改为 100 | 阶段 2 |
| 3 | `frontend/packages/loop-base/loop-lng/src/locales/tag/en-US.json` | `tag_name_length_limit` 文案中 50 改为 100 | 阶段 2 |
| 4 | `frontend/packages/loop-components/tag-components/src/components/tags-form/index.tsx` | 导入 `MAX_TAG_NAME_LENGTH`；4 处 `maxLength={50}` 改为 `maxLength={MAX_TAG_NAME_LENGTH}` | 阶段 3 |

**总计：4 个文件，6 处变更点**

---

## 风险项与注意事项

| 编号 | 风险 | 应对措施 | 严重程度 |
|------|------|----------|----------|
| R-1 | 误改 `MAX_TAG_LENGTH` 导致标签选项数量限制变化 | Code Review 时明确检查，只改 `MAX_TAG_NAME_LENGTH` | 🔴 高 |
| R-2 | 后端 API 未同步放宽到 100，前端提交 51-100 字符被后端拒绝 | 需与后端确认同步上线时间点 | 🟡 中 |
| R-3 | 长名称在列表/选择器/面包屑中的视觉表现 | 验收时人工检查，现有 ellipsis 机制应能处理 | 🟢 低 |
| R-4 | i18n 文案遗漏其他语言包 | 确认仅 zh-CN 和 en-US 两个语言包需修改 | 🟢 低 |
