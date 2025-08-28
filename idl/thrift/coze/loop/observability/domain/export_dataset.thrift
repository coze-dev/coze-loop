namespace go coze.loop.observability.domain.dataset

include "../../data/domain/dataset.thrift"
include "../domain/common.thrift"

typedef string ExportType (ts.enum="true")
const ExportType ExportType_Append = "append"
const ExportType ExportType_Overwrite = "overwrite"

typedef string ItemStatus (ts.enum="true")
const ItemStatus ItemStatus_Success = "success"
const ItemStatus ItemStatus_Error = "error"


// DatasetSchema 数据集 Schema，包含字段的类型限制等信息
struct DatasetSchema {
    10: optional list<FieldSchema> field_schemas,        // 数据集字段约束
}

struct FieldSchema {
    1: optional string key                                                              // 数据集 schema 版本变化中 key 唯一，新建时自动生成，不需传入
    2: optional string name (vt.min_size = "1", vt.max_size = "128")                    // 展示名称
    3: optional string description (vt.max_size = "1024")                               // 描述
    4: optional common.ContentType content_type (vt.not_nil = "true") // 类型，如 文本，图片，etc.
    5: optional dataset.FieldDisplayFormat default_format (vt.defined_only = "true")             // 默认渲染格式，如 code, json, etc.

    /* [20,50) 内容格式限制相关 */
    20: optional string text_schema                                  // 文本内容格式限制，格式为 JSON schema，协议参考 https://json-schema.org/specification
}

struct Item {
    1: required ItemStatus status
    2: optional list<FieldData>  field_list // todo 多模态需要修改
    3: optional list<ItemError> errors     // 错误信息
}

struct FieldData {
    1: optional string key,
    2: optional string name,
    3: optional Content content,
}

struct Content {
    1: optional common.ContentType contentType (agw.key = "content_type"  go.tag = "json:\"content_type\""),
    10: optional string text (agw.key = "text" go.tag = "json:\"text\""),
    11: optional Image image (agw.key = "image" go.tag = "json:\"image\""),               // 图片内容
    12: optional list<Content> multiPart (agw.key = "multi_part" go.tag = "json:\"multi_part\""),          // 图文混排时，图文内容
}

struct Image {
    1: optional string name (agw.key = "name" go.tag = "json:\"name\"")
    2: optional string url  (agw.key = "url" go.tag = "json:\"url\"")
}
struct ItemError {
    1: optional dataset.ItemErrorType type
    2: optional list<string> field_names       // 有错误的字段名，非必填
}

struct FieldMapping {
    1: required FieldSchema field_schema   // 数据集字段约束
    2: required string trace_field_key
    3: required string trace_field_jsonpath
}
