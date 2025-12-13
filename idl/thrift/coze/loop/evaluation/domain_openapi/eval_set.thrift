namespace go coze.loop.evaluation.domain_openapi.eval_set

include "common.thrift"

// 评测集状态
typedef string EvaluationSetStatus(ts.enum="true")
const EvaluationSetStatus EvaluationSetStatus_Active = "active"
const EvaluationSetStatus EvaluationSetStatus_Archived = "archived"

typedef string FieldDisplayFormat(ts.enum="true")
const FieldDisplayFormat FieldDisplayFormat_PlainText = "plain_text"
const FieldDisplayFormat FieldDisplayFormat_Markdown = "markdown"
const FieldDisplayFormat FieldDisplayFormat_JSON = "json"
const FieldDisplayFormat FieldDisplayFormate_YAML = "yaml"
const FieldDisplayFormat FieldDisplayFormate_Code = "code"

typedef string SchemaKey(ts.enum="true")
const SchemaKey SchemaKey_String = "string"
const SchemaKey SchemaKey_Integer = "integer"
const SchemaKey SchemaKey_Float = "float"
const SchemaKey SchemaKey_Bool = "bool"
const SchemaKey SchemaKey_Trajectory = "trajectory"

// 字段Schema
struct FieldSchema {
    1: optional string name
    2: optional string description
    3: optional common.ContentType content_type
    4: optional FieldDisplayFormat default_display_format // 默认渲染格式，如 code, json, etc.mai
    5: optional bool is_required
    6: optional string text_schema  // JSON Schema字符串
    7: optional SchemaKey schema_key                    // 对应的内置 schema

    10: optional string key    // 唯一键，创建列时无需关注，更新列的时候携带即可
}

// 评测集Schema
struct EvaluationSetSchema {
    1: optional list<FieldSchema> field_schemas
}

// 评测集版本
struct EvaluationSetVersion {
    1: optional i64 id (api.js_conv="true", go.tag = 'json:"id"')
    2: optional string version
    3: optional string description
    4: optional EvaluationSetSchema evaluation_set_schema
    5: optional i64 item_count

    100: optional common.BaseInfo base_info
}

// 评测集
struct EvaluationSet {
    1: optional i64 id (api.js_conv="true", go.tag = 'json:"id"')
    2: optional string name
    3: optional string description
    4: optional EvaluationSetStatus status
    5: optional i64 item_count
    6: optional string latest_version
    7: optional bool is_change_uncommitted

    20: optional EvaluationSetVersion current_version

    100: optional common.BaseInfo base_info
}

// 字段数据
struct FieldData {
    1: optional string name
    2: optional common.Content content
}

// 轮次数据
struct Turn {
    1: optional i64 id (api.js_conv="true", go.tag = 'json:"id"')
    2: optional list<FieldData> field_datas
}

// 评测集数据项
struct EvaluationSetItem {
    1: optional i64 id (api.js_conv="true", go.tag = 'json:"id"')
    2: optional string item_key
    3: optional list<Turn> turns
    100: optional common.BaseInfo base_info
}

// 数据项错误信息
struct ItemError {
    1: optional string item_key
    2: optional string error_code
    3: optional string error_message
}

// 数据项错误分组信息
struct ItemErrorGroup {
    1: optional i32 error_code
    2: optional string error_message
    3: optional i32 error_count                // 错误条数
    4: optional list<ItemErrorDetail> details // 错误详情
}

struct ItemErrorDetail {
    1: optional string message     // 错误信息
    2: optional i32 index      // 单条错误数据在输入数据中的索引。从 0 开始，下同
    3: optional i32 start_index // [startIndex, endIndex] 表示区间错误范围, 如 ExceedDatasetCapacity 错误时
    4: optional i32 end_index
}

struct DatasetItemOutput {
    1: optional i32 item_index                    // item 在 入参 中的索引
    2: optional string item_key
    3: optional i64 item_id (api.js_conv="true", go.tag = 'json:"item_id"')
    4: optional bool is_new_item                   // 是否是新的 Item。提供 itemKey 时，如果 itemKey 在数据集中已存在数据，则不算做「新 Item」，该字段为 false。
}