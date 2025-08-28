namespace go coze.loop.prompt.domain.prompt

struct Prompt {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')
    2: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional string prompt_key
    4: optional PromptBasic prompt_basic
    5: optional PromptDraft prompt_draft
    6: optional PromptCommit prompt_commit
}

struct PromptBasic {
    1: optional string display_name
    2: optional string description
    3: optional string latest_version
    4: optional string created_by
    5: optional string updated_by
    6: optional i64 created_at (api.js_conv="true", go.tag='json:"created_at"')
    7: optional i64 updated_at (api.js_conv="true", go.tag='json:"updated_at"')
    8: optional i64 latest_committed_at (api.js_conv="true", go.tag='json:"latest_committed_at"')

}

struct PromptCommit {
    1: optional PromptDetail detail
    2: optional CommitInfo commit_info
}

struct CommitInfo {
    1: optional string version
    2: optional string base_version
    3: optional string description
    4: optional string committed_by
    5: optional i64 committed_at (api.js_conv="true", go.tag='json:"committed_at"')
}

struct PromptDraft {
    1: optional PromptDetail detail
    2: optional DraftInfo draft_info
}

struct DraftInfo {
    1: optional string user_id
    2: optional string base_version
    3: optional bool is_modified

    11: optional i64 created_at (api.js_conv="true", go.tag='json:"created_at"')
    12: optional i64 updated_at (api.js_conv="true", go.tag='json:"updated_at"')
}

struct PromptDetail {
    1: optional PromptTemplate prompt_template
    2: optional list<Tool> tools
    3: optional ToolCallConfig tool_call_config
    4: optional ModelConfig model_config

    255: optional map<string, string> ext_infos
}

struct PromptTemplate {
    1: optional TemplateType template_type
    2: optional list<Message> messages
    3: optional list<VariableDef> variable_defs
}

typedef string TemplateType (ts.enum="true")
const TemplateType TemplateType_Normal = "normal"
const TemplateType TemplateType_Jinja2 = "jinja2"

struct Tool {
    1: optional ToolType type
    2: optional Function function
}

typedef string ToolType (ts.enum="true")
const ToolType ToolType_Function = "function"

struct Function {
    1: optional string name
    2: optional string description
    3: optional string parameters
}

struct ToolCallConfig {
    1: optional ToolChoiceType tool_choice
}

typedef string ToolChoiceType (ts.enum="true")
const ToolChoiceType ToolChoiceType_None = "none"
const ToolChoiceType ToolChoiceType_Auto = "auto"

struct ModelConfig {
    1: optional i64 model_id (api.js_conv="true", go.tag='json:"model_id"')
    2: optional i32 max_tokens
    3: optional double temperature
    4: optional i32 top_k
    5: optional double top_p
    6: optional double presence_penalty
    7: optional double frequency_penalty
    8: optional bool json_mode
}

struct Message {
    1: optional Role role
    2: optional string reasoning_content
    3: optional string content
    4: optional list<ContentPart> parts
    5: optional string tool_call_id
    6: optional list<ToolCall> tool_calls
}

typedef string Role (ts.enum="true")
const Role Role_System = "system"
const Role Role_User = "user"
const Role Role_Assistant = "assistant"
const Role Role_Tool = "tool"
const Role Role_Placeholder = "placeholder"

struct ContentPart {
    1: optional ContentType type
    2: optional string text
    3: optional ImageURL image_url
}

typedef string ContentType (ts.enum="true")
const ContentType ContentType_Text = "text"
const ContentType ContentType_ImageURL = "image_url"

struct ImageURL {
    1: optional string uri
    2: optional string url
}

struct ToolCall {
    1: optional i64 index (api.js_conv="true", go.tag='json:"index"')
    2: optional string id
    3: optional ToolType type
    4: optional FunctionCall function_call
}

struct FunctionCall {
    1: optional string name
    2: optional string arguments
}

struct VariableDef {
    1: optional string key
    2: optional string desc
    3: optional VariableType type
    4: optional list<string> type_tags
}

struct VariableVal {
    1: optional string key
    2: optional string value
    3: optional list<Message> placeholder_messages
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

struct TokenUsage {
    1: optional i64 input_tokens (api.js_conv="true", go.tag='json:"input_tokens"')
    2: optional i64 output_tokens (api.js_conv="true", go.tag='json:"output_tokens"')
}

struct DebugContext {
    1: optional DebugCore debug_core
    2: optional DebugConfig debug_config

    101: optional CompareConfig compare_config
}

struct DebugCore {
    1: optional list<DebugMessage> mock_contexts
    2: optional list<VariableVal> mock_variables
    3: optional list<MockTool> mock_tools
}

struct CompareConfig {
    1: optional list<CompareGroup> groups
}

struct CompareGroup {
    1: optional PromptDetail prompt_detail
    2: optional DebugCore debug_core
}

struct DebugMessage {
    1: optional Role role
    2: optional string content
    3: optional string reasoning_content
    4: optional list<ContentPart> parts
    5: optional string tool_call_id
    6: optional list<DebugToolCall> tool_calls

    101: optional string debug_id
    102: optional i64 input_tokens (api.js_conv="true", go.tag='json:"input_tokens"')
    103: optional i64 output_tokens (api.js_conv="true", go.tag='json:"output_tokens"')
    104: optional i64 cost_ms (api.js_conv="true", go.tag='json:"cost_ms"')
}

struct DebugToolCall {
    1: optional ToolCall tool_call
    2: optional string mock_response
    3: optional string debug_trace_key
}

struct MockTool {
    1: optional string name
    2: optional string mock_response
}

struct DebugConfig {
    1: optional bool single_step_debug
}

struct DebugLog {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')
    2: optional i64 prompt_id (api.js_conv="true", go.tag='json:"prompt_id"')
    3: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    4: optional string prompt_key
    5: optional string version
    6: optional i64 input_tokens (api.js_conv="true", go.tag='json:"input_tokens"')
    7: optional i64 output_tokens (api.js_conv="true", go.tag='json:"output_tokens"')
    8: optional i64 cost_ms (api.js_conv="true", go.tag='json:"cost_ms"')
    9: optional i32 status_code
    10: optional string debugged_by
    11: optional i64 debug_id (api.js_conv="true", go.tag='json:"debug_id"')
    12: optional i32 debug_step
    13: optional i64 started_at (api.js_conv="true", go.tag='json:"started_at"')
    14: optional i64 ended_at (api.js_conv="true", go.tag='json:"ended_at"')
}

typedef string Scenario (ts.enum="true")
const Scenario Scenario_Default = "default"
const Scenario Scenario_EvalTarget = "eval_target"

struct OverridePromptParams {
    1: optional ModelConfig model_config
}