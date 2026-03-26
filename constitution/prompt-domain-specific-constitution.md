# Prompt Domain Specific Constitution

## I. 命名规范

1. Prompt Commit 可以被叫做 Prompt版本(version) / Prompt 提交版本 / Prompt 提交，凡是出现 Prompt版本的地方都指的是 Prompt Commit
    - 因此跟跟Prompt Version相关模型的命名，均要用 PromptCommitXxx，而不能是 VersionXxx / PromptVersionXxx
2. Tool（公共函数）相关的 Domain 实体统一以 CommonTool 为前缀（如 CommonTool、CommonToolBasic、CommonToolCommit、CommonToolDetail、CommonToolCommitInfo），以避免与 Prompt 中已有的 Tool（tool_call 配置）命名冲突
    - Tool Commit 的命名约定与 Prompt Commit 类似：CommonToolCommitXxx
    - Tool 的公共草稿使用 `$PublicDraft` 版本号存储在 tool_commit 表中，常量为 `entity.ToolPublicDraftVersion`