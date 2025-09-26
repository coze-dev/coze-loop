namespace go coze.loop.evaluation.domain_openapi.eval_set

include "common.thrift"

// 评测集状态
typedef string EvaluationSetStatus(ts.enum="true")
const EvaluationSetStatus EvaluationSetStatus_Active = "active"
const EvaluationSetStatus EvaluationSetStatus_Archived = "archived"

// 字段Schema
struct FieldSchema {
    1: optional string name
    2: optional string description
    3: optional common.ContentType content_type
    4: optional bool is_required
    5: optional string text_schema  // JSON Schema字符串
}

// 评测集Schema
struct EvaluationSetSchema {
    1: optional list<FieldSchema> field_schemas
}

// 评测集版本
struct EvaluationSetVersion {
    1: optional string version_id
    2: optional string version
    3: optional string description
    4: optional EvaluationSetSchema evaluation_set_schema
    5: optional string item_count
    6: optional common.BaseInfo base_info
}

// 评测集
struct EvaluationSet {
    1: optional string evaluation_set_id
    2: optional string name
    3: optional string description
    4: optional EvaluationSetStatus status
    5: optional string item_count
    6: optional string latest_version
    7: optional bool change_uncommitted
    8: optional string biz_category
    9: optional EvaluationSetVersion current_version
    10: optional common.BaseInfo base_info
}

// 字段数据
struct FieldData {
    1: optional string name
    2: optional common.Content content
}

// 轮次数据
struct Turn {
    1: optional string turn_id
    2: optional list<FieldData> field_data_list
}

// 评测集数据项
struct EvaluationSetItem {
    1: optional string item_id
    2: optional string item_key
    3: optional list<Turn> turns
    4: optional common.BaseInfo base_info
}

// 数据项错误信息
struct ItemError {
    1: optional string item_key
    2: optional string error_code
    3: optional string error_message
}

struct ItemErrorGroup {
    1: optional string error_code
    2: optional string error_message
    3: optional list<string> item_keys
}