# Phase 1 — 核心代码实现：Prompt 创建表单预填默认值

## 任务列表

- [X] Task 1: 修改 PromptCreateModal (v2) 的 initValues，为 prompt_key 添加 `|| 'prompt_key_0'` 默认值兜底
  - 文件：`frontend/packages/loop-components/prompt-components-v2/src/components/prompt-create-modal/index.tsx`
  - 修改 `initValues` 中 `prompt_key` 的 else 分支：`data?.prompt_key` → `data?.prompt_key || 'prompt_key_0'`
  - 参考 plan.md 阶段 4，使用 `||` 运算符覆盖空字符串场景（TA-02）

- [X] Task 2: 修改 PromptCreateModal (v2) 的 initValues，为 prompt_name 添加 `|| 'prompt_demo_name_0'` 默认值兜底
  - 文件：同 Task 1
  - 修改 `initValues` 中 `prompt_name` 的 else 分支：`data?.prompt_basic?.display_name` → `data?.prompt_basic?.display_name || 'prompt_demo_name_0'`

- [X] Task 3: 修改 PromptCreate (v1) 的 initValues，为 prompt_key 添加 `|| 'prompt_key_0'` 默认值兜底
  - 文件：`frontend/packages/loop-components/prompt-components/src/prompt-create/index.tsx`
  - 修改 `initValues` 中 `prompt_key` 的 else 分支：`data?.prompt_key` → `data?.prompt_key || 'prompt_key_0'`
  - 参考 plan.md 阶段 5，保持 v1/v2 行为一致

- [X] Task 4: 修改 PromptCreate (v1) 的 initValues，为 prompt_name 添加 `|| 'prompt_demo_name_0'` 默认值兜底
  - 文件：同 Task 3
  - 修改 `initValues` 中 `prompt_name` 的 else 分支：`data?.prompt_basic?.display_name` → `data?.prompt_basic?.display_name || 'prompt_demo_name_0'`

- [X] Task 5: 验证代码修改正确性
  - 确认 v2 initValues 中 prompt_key 和 prompt_name 的 else 分支均已添加默认值
  - 确认 v1 initValues 中 prompt_key 和 prompt_name 的 else 分支均已添加默认值
  - 确认 isCopy 分支未受影响
  - 确认 prompt_description 未被修改
