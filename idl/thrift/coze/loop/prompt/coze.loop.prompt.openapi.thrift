namespace go coze.loop.prompt.openapi

include "../../../base.thrift"
include "./domain/prompt.thrift"

service PromptOpenAPIService {
    BatchGetPromptByPromptKeyResponse BatchGetPromptByPromptKey(1: BatchGetPromptByPromptKeyRequest req) (api.tag="openapi", api.post='/v1/loop/prompts/mget')
}

struct BatchGetPromptByPromptKeyRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional list<PromptQuery> queries (api.body="queries")

    255: optional base.Base Base
}

struct BatchGetPromptByPromptKeyResponse {
    1: optional i32                 code
    2: optional string              msg
    3: optional PromptResultData data

    255: optional base.BaseResp  BaseResp
}

struct PromptResultData {
    1: optional list<PromptResult> items
}

struct PromptQuery {
    1: optional string prompt_key
    2: optional string version
}

struct PromptResult {
    1: optional PromptQuery query
    2: optional Prompt prompt
}

struct Prompt {
    1: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"') // 空间ID
    2: optional string prompt_key // 唯一标识
    3: optional string version // 版本
    4: optional PromptTemplate prompt_template // Prompt模板
    5: optional list<Tool> tools // tool定义
    6: optional ToolCallConfig tool_call_config // tool调用配置
    7: optional LLMConfig llm_config // 模型配置
}

struct PromptTemplate {
    1: optional TemplateType template_type // 模板类型
    2: optional list<Message> messages // 只支持message list形式托管
    3: optional list<VariableDef> variable_defs // 变量定义
}

typedef string TemplateType
const TemplateType TemplateType_Normal = "normal"

typedef string ToolChoiceType
const ToolChoiceType ToolChoiceType_Auto = "auto"
const ToolChoiceType ToolChoiceType_None = "none"

struct ToolCallConfig {
    1: optional ToolChoiceType tool_choice
}

struct Message {
    1: optional Role role
    2: optional string content
}

struct VariableDef {
     1: optional string key // 变量名字
     2: optional string desc // 变量描述
     3: optional VariableType type // 变量类型
}

typedef string VariableType
const VariableType VariableType_String = "string"
const VariableType VariableType_Placeholder = "placeholder"

typedef string Role
const Role Role_System = "system"
const Role Role_User = "user"
const Role Role_Assistant = "assistant"
const Role Role_Tool = "tool"
const Role Role_Placeholder = "placeholder"

struct Tool {
    1: optional ToolType type
    2: optional Function function
}

typedef string ToolType
const ToolType ToolType_Function = "function"

struct Function {
    1: optional string name
    2: optional string description
    3: optional string parameters
}

struct LLMConfig {
    1: optional double temperature
    2: optional i32 max_tokens
    3: optional i32 top_k
    4: optional double top_p
    5: optional double presence_penalty
    6: optional double frequency_penalty
    7: optional bool json_mode
}