namespace go coze.loop.llm.domain.runtime

include "common.thrift"

struct ModelConfig {
    1: required i64 model_id  (api.js_conv='true', go.tag='json:"model_id"')// 模型id
    2: optional double temperature
    3: optional i64 max_tokens (api.js_conv='true', go.tag='json:"max_tokens"')
    4: optional double top_p
    5: optional list<string> stop
    6: optional ToolChoice tool_choice
    7: optional ResponseFormat response_format // support json
    8: optional i32 top_k
    9: optional double presence_penalty
    10: optional double frequency_penalty
}

struct Message {
    1: required Role role
    2: optional string content
    3: optional list<ChatMessagePart> multimodal_contents
    4: optional list<ToolCall> tool_calls // only for AssistantMessage
    5: optional string tool_call_id // only for ToolMessage
    6: optional ResponseMeta response_meta // collects meta information about a chat response
    7: optional string reasoning_content // only for AssistantMessage, And when reasoning_content is not empty, content must be empty
    // 8: optional map<string,string> extra
}

struct ChatMessagePart {
    1: optional ChatMessagePartType type
    2: optional string text
    3: optional ChatMessageImageURL image_url
//    4: optional ChatMessageAudioURL audio_url 占位,暂不支持
//    5: optional ChatMessageVideoURL video_url 占位,暂不支持
//    6: optional ChatMessageFileURL file_url 占位,暂不支持
}

struct ChatMessageImageURL {
    1: optional string url
    2: optional ImageURLDetail detail
    3: optional string mime_type
}

struct ToolCall {
    1: optional i64 index (api.js_conv='true', go.tag='json:"index"')
    2: optional string id
    3: optional ToolType type
    4: optional FunctionCall function_call
}

struct FunctionCall {
    1: optional string name
    2: optional string arguments
}

struct ResponseMeta {
    1: optional string finish_reason
    2: optional TokenUsage usage
    // 3: optional LogProbs log_probs
}

struct TokenUsage {
    1: optional i64 prompt_tokens (api.js_conv='true', go.tag='json:"prompt_tokens"')
    2: optional i64 completion_tokens (api.js_conv='true', go.tag='json:"completion_tokens"')
    3: optional i64 total_tokens (api.js_conv='true', go.tag='json:"total_tokens"')
}

struct Tool {
    1: optional string name
    2: optional string desc
    3: optional ToolDefType def_type
    4: optional string def // 必须使用openapi3.Schema序列化后的json
}


struct BizParam {
    1: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional string user_id
    3: optional common.Scenario scenario // 使用场景
    4: optional string scenario_entity_id // 场景实体id(非必填)
    5: optional string scenario_entity_version // 场景实体version(非必填)
    6: optional string scenario_entity_key // 场景实体key(非必填), prompt场景需要传prompt key
}

struct ResponseFormat {
    1: optional ResponseFormatType type
}

typedef string ResponseFormatType
const ResponseFormatType response_format_json_object  = "json_object"
const ResponseFormatType response_format_text  = "text"

typedef string ToolChoice (ts.enum="true")
const ToolChoice tool_choice_auto = "auto"
const ToolChoice tool_choice_required = "required"
const ToolChoice tool_choice_none = "none"

typedef string ToolDefType (ts.enum="true")
const ToolDefType tool_def_type_open_api_v3 = "open_api_v3"

typedef string Role (ts.enum="true")
const Role role_system = "system"
const Role role_assistant = "assistant"
const Role role_user = "user"
const Role role_tool = "tool"

typedef string ToolType (ts.enum="true")
const ToolType tool_type_function = "function"

typedef string ChatMessagePartType (ts.enum="true")
const ChatMessagePartType chat_message_part_type_text = "text"
const ChatMessagePartType chat_message_part_type_image_url = "image_url"
// const ChatMessagePartType chat_message_part_type_audio_url = "audio_url"
// const ChatMessagePartType chat_message_part_type_video_url = "video_url"
// const ChatMessagePartType chat_message_part_type_file_url = "file_url"

typedef string ImageURLDetail (ts.enum="true")
const ImageURLDetail image_url_detail_auto = "auto"
const ImageURLDetail image_url_detail_low = "low"
const ImageURLDetail image_url_detail_high = "high"
