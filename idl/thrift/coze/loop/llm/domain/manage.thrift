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
    10: optional string identification // 模型表示 (name, endpoint)
    11: optional Family family // 模型家族
    14: optional bool is_public
    15: optional Visibility visibility
    16: optional string icon // 模型图标
    17: optional list<string> tags //模型标签
    18: optional ModelStatus status // 模型状态

    100: optional string created_by
    101: optional i64 created_at
    102: optional string updated_by
    103: optional i64 updated_at
}

struct Series {
    1: optional string name // series name
    2: optional string icon // series icon url
    3: optional Family family // family name
}

struct Visibility {
    1: optional VisibleMode mode
    2: optional list<i64> spaceIDs // Mode为Specified有效，配置为除模型所属空间外的其他空间
}

struct ProviderInfo {
    1: optional MaaSInfo maas_info
}

struct MaaSInfo {
    1: optional string host
    2: optional string region
    3: optional string baseURL // v3 sdk
    4: optional string customizationJobsID // 精调模型任务的 ID
}


struct Ability {
    1: optional i64 max_context_tokens (api.js_conv='true', go.tag='json:"max_context_tokens"')
    2: optional i64 max_input_tokens (api.js_conv='true', go.tag='json:"max_input_tokens"')
    3: optional i64 max_output_tokens (api.js_conv='true', go.tag='json:"max_output_tokens"')
    4: optional bool function_call
    5: optional bool json_mode
    6: optional bool multi_modal
    7: optional AbilityMultiModal ability_multi_modal
    8: optional InterfaceCategory interface_category
}

struct AbilityMultiModal {
    // 图片
    1: optional bool image
    2: optional AbilityImage ability_image
    // 视频
    3: optional bool video
    4: optional AbilityVideo ability_video
}

struct AbilityImage {
    1: optional bool url_enabled
    2: optional bool binary_enabled
    3: optional i64 max_image_size (api.js_conv='true', go.tag='json:"max_image_size"')
    4: optional i64 max_image_count (api.js_conv='true', go.tag='json:"max_image_count"')
}

struct AbilityVideo {
    1: optional i64 max_video_size (api.js_conv='true', go.tag='json:"max_video_size"') // MB
    2: optional list<VideoFormat> supported_formats (api.js_conv='true', go.tag='json:"supported_formats"')
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
    9: optional list<ParamSchema> properties
    10: optional Dependency dependency      // 依赖参数
    11: optional string jsonpath // 赋值路径
}

struct Dependency {
    1: optional string key
    2: optional string condition
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
const ParamType param_type_void = "void"
const ParamType param_type_object = "object"

typedef string Family (ts.enum="true")
const Family family_undefined = "undefined"
const Family family_gpt = "gpt"
const Family family_seed = "seed"
const Family family_gemini = "gemini"
const Family family_claude = "claude"
const Family family_ernie = "ernie"
const Family family_baichuan = "baichuan"
const Family family_qwen = "qwen"
const Family family_glm = "glm"
const Family family_skylark = "skylark"
const Family family_moonshot = "moonshot"
const Family family_minimax = "minimax"
const Family family_doubao = "doubao"
const Family family_baichuan2 = "baichuan2"
const Family family_deepseekv2 = "deepseekv2"
const Family family_deepseek_coder_v2 = "deepseek_coder_v2"
const Family family_deepseek_coder = "deepseek_coder"
const Family family_internalm2_5 = "internalm2_5"
const Family family_qwen2 = "qwen2"
const Family family_qwen2_5 = "qwen2.5"
const Family family_qwen2_5_coder = "qwen2.5_coder"
const Family family_mini_cpm = "mini_cpm"
const Family family_mini_cpm_3 = "mini_cpm_3"
const Family family_chat_glm_3 = "chat_glm_3"
const Family family_mistra = "mistral"
const Family family_gemma = "gemma"
const Family family_gemma_2 = "gemma_2"
const Family family_intern_vl2 = "intern_vl2"
const Family family_intern_vl2_5 = "intern_vl2.5"
const Family family_deepseek_v3 = "deepseek_v3"
const Family family_deepseek_r1 = "deepseek_r1"
const Family family_kimi = "kimi"
const Family family_seedream = "seedream"
const Family family_intern_vl3 = "intern_vl3"


typedef string Provider (ts.enum="true")
const Provider provider_undefined = "undefined"
const Provider provider_maas = "maas"

typedef string VisibleMode (ts.enum="true")
const VisibleMode visible_mode_default = "default"
const VisibleMode visible_mode_specified = "specified"
const VisibleMode visible_mode_undefined = "undefined"
const VisibleMode visible_mode_all = "all"

typedef string ModelStatus (ts.enum="true")
const ModelStatus model_status_undefined = "undefined"
const ModelStatus model_status_available = "available"      //可用
const ModelStatus model_status_unavailable = "unavailable"    //不可用

typedef string VideoFormat (ts.enum="true")
const VideoFormat video_format_undefined = "undefined"
const VideoFormat video_format_mp4  = "mp4"
const VideoFormat video_format_avi  = "avi"
const VideoFormat video_format_mov = "mov"
const VideoFormat video_format_mpg  = "mpg"

typedef string InterfaceCategory (ts.enum="true")
const InterfaceCategory interface_category_undefined = "undefined"
const InterfaceCategory interface_category_chat_completion_api = "chat_completion_api"
const InterfaceCategory interface_category_response_api = "response_api"

typedef string AbilityEnum (ts.enum="true")
const AbilityEnum ability_undefined = "undefined"
const AbilityEnum ability_json_mode = "json_mode"
const AbilityEnum ability_function_call = "function_call"
const AbilityEnum ability_multi_modal = "multi_modal"

