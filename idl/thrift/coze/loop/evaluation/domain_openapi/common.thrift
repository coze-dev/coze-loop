namespace go coze.loop.evaluation.domain_openapi.common

// 内容类型枚举
typedef string ContentType(ts.enum="true")
const ContentType ContentType_Text = "text"
const ContentType ContentType_Image = "image" 
const ContentType ContentType_Audio = "audio"
const ContentType ContentType_Video = "video"
const ContentType ContentType_MultiPart = "multi_part"

// 内容结构
struct Content {
    1: optional ContentType content_type
    2: optional string text
    3: optional Image image
    4: optional Video video
    5: optional Audio audio
    10: optional list<Content> multi_part

    // 超大文本相关字段
    30: optional bool content_omitted       // 当前列的数据是否省略, 如果此处返回 true, 需要通过 GetDatasetItemField 获取当前列的具体内容, 或者是通过 omittedDataStorage.url 下载
    31: optional ObjectStorage full_content // 被省略数据的完整信息，批量返回时会签发相应的 url，用户可以点击下载. 同时支持通过该字段传入已经上传好的超长数据(dataOmitted 为 true 时生效)
    32: optional i32 full_content_bytes      // 超长数据完整内容的大小，单位 byte
}

struct ObjectStorage {
    1: optional string url
}

// 图片结构
struct Image {
    1: optional string name
    2: optional string url
    3: optional string thumb_url
}

// 视频结构
struct Video {
    1: optional string name
    2: optional string url
    3: optional string uri
    4: optional string thumb_url
}

// 音频结构
struct Audio {
    1: optional string format
    2: optional string url
    3: optional string name
    4: optional string uri
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
    1: optional string field
    2: optional bool is_asc
}

struct RuntimeParam {
    1: optional string json_value
}

// 消息角色
typedef string Role(ts.enum="true")
const Role Role_System = "system"
const Role Role_User = "user"
const Role Role_Assistant = "assistant"

// 消息结构
struct Message {
    1: optional Role role
    2: optional Content content
    3: optional map<string, string> ext
}