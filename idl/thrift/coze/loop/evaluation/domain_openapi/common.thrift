namespace go coze.loop.evaluation.domain_openapi.common

// 内容类型枚举
typedef string ContentType(ts.enum="true")
const ContentType ContentType_Text = "text"
const ContentType ContentType_Image = "image" 
const ContentType ContentType_Audio = "audio"
const ContentType ContentType_MultiPart = "multi_part"

// 内容结构
struct Content {
    1: optional ContentType content_type
    2: optional string text
    3: optional Image image

    10: optional list<Content> multi_part
}

// 图片结构
struct Image {
    1: optional string name
    2: optional string url
    3: optional string thumb_url
}

// 音频结构  
struct Audio {
    1: optional string format
    2: optional string url
}

// 用户信息
struct UserInfo {
    1: optional string name
    2: optional string user_id
    3: optional string avatar_url
    4: optional string email
}

// 基础信息
struct BaseInfo {
    1: optional UserInfo created_by
    2: optional UserInfo updated_by
    3: optional i64 created_at (api.js_conv="true", go.tag = 'json:"created_at"')
    4: optional i64 updated_at (api.js_conv="true", go.tag = 'json:"updated_at"')
}

// 模型配置
struct ModelConfig {
    1: optional i64 model_id (api.js_conv="true", go.tag = 'json:"model_id"') // 模型id
    2: optional string model_name // 模型名称
    3: optional double temperature
    4: optional i32 max_tokens
    5: optional double top_p
}

// 参数Schema
struct ArgsSchema {
    1: optional string key
    2: optional list<ContentType> support_content_types
    3: optional string json_schema  // JSON Schema字符串
}

// 分页信息
struct PageInfo {
    1: optional i32 page_num
    2: optional i32 page_size
    3: optional bool has_more
    4: optional i64 total_count (api.js_conv="true", go.tag = 'json:"total_count"')
}

// 统一响应格式
struct OpenAPIResponse {
    1: optional i32 code
    2: optional string msg
}

struct OrderBy {
    1: optional string field,
    2: optional bool is_asc,
}

struct RuntimeParam {
    1: optional string json_value
    2: optional string json_demo
}