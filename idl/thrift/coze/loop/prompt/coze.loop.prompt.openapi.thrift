namespace go coze.loop.prompt.openapi

include "../../../base.thrift"
include "./domain/prompt.thrift"

service PromptOpenAPIService {
    BatchGetPromptByPromptKeyResponse BatchGetPromptByPromptKey(1: BatchGetPromptByPromptKeyRequest req) (api.tag="openapi", api.post='/v1/loop/prompts/mget')
    ExecuteResponse Execute(1: ExecuteRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/execute")
    ExecuteStreamingResponse ExecuteStreaming(1: ExecuteRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/execute_streaming", streaming.mode='server')
    ListPromptBasicResponse ListPromptBasic(1: ListPromptBasicRequest req) (api.tag="openapi", api.post='/v1/loop/prompts/list')
}

struct BatchGetPromptByPromptKeyRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional list<PromptQuery> queries (api.body="queries")

    255: optional base.Base Base
}

struct BatchGetPromptByPromptKeyResponse {
    1: optional i32 code
    2: optional string msg
    3: optional PromptResultData data

    255: optional base.BaseResp BaseResp
}

struct PromptResultData {
    1: optional list<PromptResult> items
}

struct ExecuteRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"') // 工作空间ID
    2: optional PromptQuery prompt_identifier (api.body="prompt_identifier") // Prompt 标识

    10: optional list<VariableVal> variable_vals (api.body="variable_vals") // 变量值
    11: optional list<Message> messages (api.body="messages") // 消息

    20: optional list<Tool> custom_tools (api.body="custom_tools") // 自定义工具
    21: optional ToolCallConfig custom_tool_call_config (api.body="custom_tool_call_config") // 自定义工具调用配置
    22: optional ModelConfig custom_model_config (api.body="custom_model_config") // 自定义模型配置

    255: optional base.Base Base
}

struct ExecuteResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ExecuteData data

    255: optional base.BaseResp BaseResp
}

struct ExecuteData {
    1: optional Message message // 消息
    2: optional string finish_reason // 结束原因
    3: optional TokenUsage usage //  token消耗
}

struct ExecuteStreamingResponse {
    1: optional string id
    2: optional string event
    3: optional i64 retry
    4: optional ExecuteStreamingData data

    255: optional base.BaseResp BaseResp
}

struct ExecuteStreamingData {
    1: optional i32 code
    2: optional string msg
    3: optional Message message // 消息
    4: optional string finish_reason // 结束原因
    5: optional TokenUsage usage // token消耗
}

struct PromptQuery {
    1: optional string prompt_key // prompt_key
    2: optional string version // prompt版本
    3: optional string label // prompt版本标识（如果version不为空，该字段会被忽略）
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
    8: optional i64 id (api.js_conv='true', go.tag='json:"id"') // promptId
    9: optional string display_name // Prompt名称
    10: optional string description // Prompt描述
    11: optional PromptType prompt_type // Prompt类型
    12: optional string created_by
    14: optional i64 created_at (api.js_conv="true", go.tag='json:"created_at"')
    15: optional i64 updated_at (api.js_conv="true", go.tag='json:"updated_at"')
    16: optional PublishStatus status // 发布状态
    17: optional PromptPublishInfo PublishInfo // 发布信息
    18: optional SecurityLevel security_level // 密级标签
}

typedef string PromptType (ts.enum="true")
const PromptType PromptType_Normal = "normal"
const PromptType PromptType_Snippet = "snippet"

struct PromptTemplate {
    1: optional TemplateType template_type // 模板类型
    2: optional list<Message> messages // 只支持message list形式托管
    3: optional list<VariableDef> variable_defs // 变量定义

    100: optional map<string, string> metadata // 模板级元信息
}

typedef string TemplateType
const TemplateType TemplateType_Normal = "normal"
const TemplateType TemplateType_Jinja2 = "jinja2"
const TemplateType TemplateType_GoTemplate = "go_template"
const TemplateType TemplateType_CustomTemplate_M = "custom_template_m"


typedef string ToolChoiceType
const ToolChoiceType ToolChoiceType_Auto = "auto"
const ToolChoiceType ToolChoiceType_None = "none"
const ToolChoiceType ToolChoiceType_Specific = "specific"

struct ToolCallConfig {
    1: optional ToolChoiceType tool_choice
    2: optional ToolChoiceSpecification tool_choice_specification
}

struct ToolChoiceSpecification {
    1: optional ToolType type
    2: optional string name
}

struct Message {
    1: optional Role role // 角色
    2: optional string content // 消息内容
    3: optional list<ContentPart> parts // 多模态内容
    4: optional string reasoning_content // 推理思考内容
    5: optional string tool_call_id // tool调用ID（role为tool时有效）
    6: optional list<ToolCall> tool_calls // tool调用（role为assistant时有效）
    7: optional bool skip_render // 是否跳过需要渲染
    8: optional string signature // gemini的签名

    100: optional map<string, string> metadata // 消息元信息
}

struct ContentPart {
    1: optional ContentType type
    2: optional string text
    3: optional string image_url
    4: optional string base64_data
    5: optional string video_url
    6: optional MediaConfig config
    7: optional string signature
    8: optional string image_uri
    9: optional string video_uri
}

struct MediaConfig {
    1: optional double fps (vt.ge="0.2", vt.le="5")
    2: optional string image_resolution
}

typedef string ContentType (ts.enum="true")
const ContentType ContentType_Text = "text"
const ContentType ContentType_ImageURL = "image_url"
const ContentType ContentType_VideoURL = "video_url"
const ContentType ContentType_Base64Data = "base64_data"
const ContentType ContentType_MultiPartVariable = "multi_part_variable"

struct VariableDef {
     1: optional string key // 变量名字
     2: optional string desc // 变量描述
     3: optional VariableType type // 变量类型
}

typedef string VariableType (ts.enum="true")
const VariableType VariableType_String = "string"
const VariableType VariableType_Boolean = "boolean"
const VariableType VariableType_Integer = "integer"
const VariableType VariableType_Float = "float"
const VariableType VariableType_Object = "object"
const VariableType VariableType_Array_String = "array<string>"
const VariableType VariableType_Array_Boolean = "array<boolean>"
const VariableType VariableType_Array_Integer = "array<integer>"
const VariableType VariableType_Array_Float = "array<float>"
const VariableType VariableType_Array_Object = "array<object>"
const VariableType VariableType_Placeholder = "placeholder"
const VariableType VariableType_MultiPart = "multi_part"

typedef string Role (ts.enum="true")
const Role Role_System = "system"
const Role Role_User = "user"
const Role Role_Assistant = "assistant"
const Role Role_Tool = "tool"
const Role Role_Placeholder = "placeholder"

struct Tool {
    1: optional ToolType type
    2: optional Function function
    3: optional bool disable // 时候禁用
}

typedef string ToolType (ts.enum="true")
const ToolType ToolType_Function = "function"
const ToolType ToolType_GoogleSearch = "google_search"

struct Function {
    1: optional string name
    2: optional string description
    3: optional string parameters
}

struct ToolCall {
    1: optional i32 index
    2: optional string id
    3: optional ToolType type
    4: optional FunctionCall function_call
    5: optional string output_id
    6: optional string signature
}

struct FunctionCall {
    1: optional string name
    2: optional string arguments
}

struct LLMConfig {
    1: optional double temperature
    2: optional i32 max_tokens
    3: optional i32 top_k
    4: optional double top_p
    5: optional double presence_penalty
    6: optional double frequency_penalty
    7: optional bool json_mode
    8: optional i64 id (api.js_conv='true', go.tag='json:"id"')
    9: optional string name
    10: optional ThinkingConfig thinking
    11: optional string extra
}

struct VariableVal {
    1: optional string key // 变量key
    2: optional string value // 普通变量值（非string类型，如boolean、integer、float、object等，序列化后传入）
    3: optional list<Message> placeholder_messages // placeholder变量值
    4: optional list<ContentPart> multi_part_values // 多模态变量值
}

struct TokenUsage {
    1: optional i32 input_tokens // 输入消耗
    2: optional i32 output_tokens // 输出消耗
}

struct ListPromptBasicRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i32 page_number (api.body="page_number", vt.gt = "0")
    3: optional i32 page_size (api.body="page_size", vt.gt = "0", vt.le = "200")
    4: optional string key_word (api.body="key_word") // name/key前缀匹配
    5: optional string creator (api.body="creator") // 创建人
    6: optional map<string, string> extra (api.body="extra") // 额外查询条件

    255: optional base.Base Base
}

struct ListPromptBasicResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListPromptBasicData data

    255: optional base.BaseResp BaseResp
}

struct PromptBasic {
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"') // Prompt ID
    2: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"') // 工作空间ID
    3: optional string prompt_key // 唯一标识
    4: optional string display_name // Prompt名称
    5: optional string description // Prompt描述
    6: optional string latest_version // 最新版本
    7: optional string created_by // 创建者
    8: optional string updated_by // 更新者
    9: optional i64 created_at (api.js_conv='true', go.tag='json:"created_at"') // 创建时间
    10: optional i64 updated_at (api.js_conv='true', go.tag='json:"updated_at"') // 更新时间
    11: optional i64 latest_committed_at (api.js_conv='true', go.tag='json:"latest_committed_at"') // 最后提交时间
}

struct ListPromptBasicData {
    1: optional list<PromptBasic> prompts // Prompt列表
    2: optional i32 total
}

struct PromptPublishInfo {
    1: string publisher // 发布者
    2: string publish_description // 发布描述
    3: optional i64 publish_at // 发布时间
}

enum PublishStatus {
    Undefined = 0
    UnPublish // 未发布
    Published // 已发布
}

enum SecurityLevel {
    Undefined = 0
    L1 = 1
    L2 = 2
    L3 = 3
    L4 = 4
}

struct ThinkingConfig {
     1: optional i64 budget_tokens (agw.key="budget_tokens") // thinking内容的最大输出token
     2: optional ThinkingOption thinking_option (agw.key="thinking_option")
     3: optional ReasoningEffort reasoning_effort (agw.key="reasoning_effort") // 思考长度
}

enum ReasoningEffort {
    Minimal = 1
    Low = 2
    Medium = 3
    High = 4
}

enum ThinkingOption {
    Disabled = 1
    Enabled = 2
    Auto = 3
}

struct ModelConfig {
    1: optional i64 model_id (api.js_conv="true", go.tag='json:"model_id"')
    2: optional i32 max_tokens
    3: optional double temperature
    4: optional i32 top_k
    5: optional double top_p
    6: optional double presence_penalty
    7: optional double frequency_penalty
    8: optional bool json_mode
    9: optional string extra
    10: optional ThinkingConfig thinking

    100: optional list<ParamConfigValue> param_config_values
}

struct ParamConfigValue {
    1: optional string name // 传给下游模型的key，与ParamSchema.name对齐
    2: optional string label // 展示名称
    3: optional ParamOption value // 传给下游模型的value，与ParamSchema.options对齐
}

struct ParamOption {
    1: optional string value // 实际值
    2: optional string label // 展示值
}