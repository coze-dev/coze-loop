namespace go coze.loop.llm.domain.manage

include "common.thrift"

struct Model {
    1: optional i64 model_id (api.js_conv='true', go.tag='json:"model_id"')
    2: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"')
    3: optional string name
    4: optional string desc
    5: optional Ability ability
    6: optional Protocol protocol
    7: optional ProtocolConfig protocol_config
    8: optional map<common.Scenario, ScenarioConfig> scenario_configs
    9: optional ParamConfig param_config
}

struct Ability {
    1: optional i64 max_context_tokens (api.js_conv='true', go.tag='json:"max_context_tokens"')
    2: optional i64 max_input_tokens (api.js_conv='true', go.tag='json:"max_input_tokens"')
    3: optional i64 max_output_tokens (api.js_conv='true', go.tag='json:"max_output_tokens"')
    4: optional bool function_call
    5: optional bool json_mode
    6: optional bool multi_modal
    7: optional AbilityMultiModal ability_multi_modal
}

struct AbilityMultiModal {
    1: optional bool image
    2: optional AbilityImage ability_image
    3: optional bool video
    4: optional AbilityVideo ability_video
}

struct AbilityImage {
    1: optional bool url_enabled
    2: optional bool binary_enabled
    3: optional i64 max_image_size (api.js_conv='true', go.tag='json:"max_image_size"')
    4: optional i64 max_image_count (api.js_conv='true', go.tag='json:"max_image_count"')
    5: optional bool image_gen_enabled
}

struct AbilityVideo {
    1: optional i32 max_video_size_in_mb // the size limit of single video
    2: optional list<VideoFormat> supported_video_formats
}

struct ProtocolConfig {
    1: optional string base_url
    2: optional string api_key
    3: optional string model
    4: optional ProtocolConfigArk protocol_config_ark
    5: optional ProtocolConfigOpenAI protocol_config_openai
    6: optional ProtocolConfigClaude protocol_config_claude
    7: optional ProtocolConfigDeepSeek protocol_config_deepseek
    8: optional ProtocolConfigOllama protocol_config_ollama
    9: optional ProtocolConfigQwen protocol_config_qwen
    10: optional ProtocolConfigQianfan protocol_config_qianfan
    11: optional ProtocolConfigGemini protocol_config_gemini
    12: optional ProtocolConfigArkbot protocol_config_arkbot
}

struct ProtocolConfigArk {
    1: optional string region // Default: "cn-beijing"
    2: optional string access_key
    3: optional string secret_key
    4: optional i64 retry_times (api.js_conv='true', go.tag='json:"retry_times"')
    5: optional map<string,string> custom_headers
}

struct ProtocolConfigOpenAI {
    1: optional bool by_azure
    2: optional string api_version
    3: optional string response_format_type
    4: optional string response_format_json_schema
}
struct ProtocolConfigClaude {
    1: optional bool by_bedrock
    // bedrock config
    2: optional string access_key
    3: optional string secret_access_key
    4: optional string session_token
    5: optional string region

}
struct ProtocolConfigDeepSeek {
    1: optional string response_format_type
}

struct ProtocolConfigGemini {
    1: optional string response_schema
    2: optional bool enable_code_execution
    3: optional list<ProtocolConfigGeminiSafetySetting> safety_settings
}

struct ProtocolConfigGeminiSafetySetting {
    1: optional i32 category
    2: optional i32 threshold
}

struct ProtocolConfigOllama {
    1: optional string format
    2: optional i64 keep_alive_ms (api.js_conv='true', go.tag='json:"keep_alive_ms"')
}

struct ProtocolConfigQwen {
    1: optional string response_format_type
    2: optional string response_format_json_schema
}

struct ProtocolConfigQianfan {
    1: optional i32 llm_retry_count
    2: optional double llm_retry_timeout
    3: optional double llm_retry_backoff_factor
    4: optional bool parallel_tool_calls
    5: optional string response_format_type
    6: optional string response_format_json_schema
}

struct ProtocolConfigArkbot {
    1: optional string region // Default: "cn-beijing"
    2: optional string access_key
    3: optional string secret_key
    4: optional i64 retry_times (api.js_conv='true', go.tag='json:"retry_times"')
    5: optional map<string,string> custom_headers
}

struct ScenarioConfig {
    1: optional common.Scenario scenario
    3: optional Quota quota
    4: optional bool unavailable
}

struct ParamConfig {
    1: optional list<ParamSchema> param_schemas
}

struct ParamSchema {
    1: optional string name // 实际名称
    2: optional string label // 展示名称
    3: optional string desc
    4: optional ParamType type
    5: optional string min
    6: optional string max
    7: optional string default_value
    8: optional list<ParamOption> options
}

struct ParamOption {
    1: optional string value // 实际值
    2: optional string label // 展示值
}

struct Quota {
    1: optional i64 qpm (api.js_conv='true', go.tag='json:"qpm"')
    2: optional i64 tpm (api.js_conv='true', go.tag='json:"tpm"')
}

typedef string Protocol (ts.enum="true")
const Protocol protocol_ark = "ark"
const Protocol protocol_openai = "openai"
const Protocol protocol_claude = "claude"
const Protocol protocol_deepseek = "deepseek"
const Protocol protocol_ollama = "ollama"
const Protocol protocol_gemini = "gemini"
const Protocol protocol_qwen = "qwen"
const Protocol protocol_qianfan = "qianfan"
const Protocol protocol_arkbot = "arkbot"

typedef string ParamType (ts.enum="true")
const ParamType param_type_float = "float"
const ParamType param_type_int = "int"
const ParamType param_type_boolean = "boolean"
const ParamType param_type_string = "string"

typedef string VideoFormat (ts.enum="true")
const VideoFormat video_format_undefined = "undefined"
const    VideoFormat video_format_mp4 = "mp4"
const    VideoFormat video_format_avi = "avi"
const    VideoFormat video_format_mov = "mov"
const    VideoFormat video_format_mpg = "mpg"
const    VideoFormat video_format_webm = "webm"
const   VideoFormat video_format_rvmb = "rvmb"
const    VideoFormat video_format_wmv = "wmv"
const    VideoFormat video_format_mkv = "mkv"
const    VideoFormat video_format_t3gp = "t3gp"
const    VideoFormat video_format_flv = "flv"
const    VideoFormat video_format_mpeg = "mpeg"
const    VideoFormat video_format_ts = "ts"
const    VideoFormat video_format_rm = "rm"
const    VideoFormat video_format_m4v = "m4v"