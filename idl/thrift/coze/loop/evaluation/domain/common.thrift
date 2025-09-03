namespace go coze.loop.evaluation.domain.common

include "../../data/domain/dataset.thrift"

typedef string ContentType(ts.enum="true")

const ContentType ContentType_Text = "Text" // 空间
const ContentType ContentType_Image = "Image"
const ContentType ContentType_Audio = "Audio"
const ContentType ContentType_MultiPart = "MultiPart"
const ContentType ContentType_MultiPartVariable = "multi_part_variable"

struct Content {
    1: optional ContentType content_type (go.tag='mapstructure:"content_type"'),
    2: optional dataset.FieldDisplayFormat format (go.tag='mapstructure:"format"'),
    10: optional string text (go.tag='mapstructure:"text"'),
    11: optional Image image (go.tag='mapstructure:"image"'),
    12: optional list<Content> multi_part (go.tag='mapstructure:"multi_part"'),
    13: optional Audio audio (go.tag='mapstructure:"audio"'),
}

struct AudioContent {
    1: optional list<Audio> audios,
}

struct Audio {
    1: optional string format,
    2: optional string url,
}

struct Image {
    1: optional string name,
    2: optional string url,
    3: optional string uri,
    4: optional string thumb_url,

    10: optional dataset.StorageProvider storage_provider (vt.defined_only = "true") // 当前多模态附件存储的 provider. 如果为空，则会从对应的 url 下载文件并上传到默认的存储中，并填充uri
}

struct OrderBy {
    1: optional string field,
    2: optional bool is_asc,
}

enum Role {
    System = 1
    User = 2
    Assistant = 3
    Tool = 4
}

struct Message {
    1: optional Role role (go.tag='mapstructure:"role"'),
    2: optional Content content (go.tag='mapstructure:"content"'),
    3: optional map<string, string> ext (go.tag='mapstructure:"ext"'),
}

struct ArgsSchema {
    1: optional string key (go.tag='mapstructure:"key"'),
    2: optional list<ContentType> support_content_types (go.tag='mapstructure:"support_content_types"'),
    // 	序列化后的jsonSchema字符串，例如："{\"type\": \"object\", \"properties\": {\"name\": {\"type\": \"string\"}, \"age\": {\"type\": \"integer\"}, \"isStudent\": {\"type\": \"boolean\"}}, \"required\": [\"name\", \"age\", \"isStudent\"]}"
    3: optional string json_schema (go.tag='mapstructure:"json_schema"'),
}

struct UserInfo {
	1: optional string name // 姓名
	2: optional string en_name // 英文名称
	3: optional string avatar_url // 用户头像url
	4: optional string avatar_thumb // 72 * 72 头像
	5: optional string open_id // 用户应用内唯一标识
	6: optional string union_id // 用户应用开发商内唯一标识
    8: optional string user_id // 用户在租户内的唯一标识
    9: optional string email // 用户邮箱
}

struct BaseInfo {
    1: optional UserInfo created_by                       
    2: optional UserInfo updated_by                     
    3: optional i64 created_at      (api.js_conv="true", go.tag = 'json:"created_at"')
    4: optional i64 updated_at      (api.js_conv="true", go.tag = 'json:"updated_at"')
    5: optional i64 deleted_at      (api.js_conv="true", go.tag = 'json:"deleted_at"')
}

// 评测模型配置
struct ModelConfig {
    1: optional i64 model_id (api.js_conv="true", go.tag = 'json:"model_id"') // 模型id
    2: optional string model_name // 模型名称
    3: optional double temperature
    4: optional i32 max_tokens
    5: optional double top_p

    50: optional string json_ext
}

struct Session {
    1: optional i64 user_id
    2: optional i32 app_id
}

struct RuntimeParam {
    1: optional string json_value
    2: optional string json_demo
}
