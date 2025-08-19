namespace go coze.loop.evaluation.domain.eval_set

include "../../data/domain/dataset.thrift"
include "common.thrift"

struct EvaluationSet {
    // 主键&外键
    1: optional i64 id (api.js_conv="true", go.tag = 'json:"id"'),
    2: optional i32 app_id,
    3: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"'),

    // 基础信息
    10: optional string name,
    11: optional string description,
    12: optional dataset.DatasetStatus status,
    13: optional dataset.DatasetSpec spec,             // 规格限制
    14: optional dataset.DatasetFeatures features,     // 功能开关
    15: optional i64 item_count (api.js_conv="true", go.tag = 'json:"item_count"'),                        // 数据条数
    16: optional bool change_uncommitted           // 是否有未提交的修改
    17: optional BizCategory biz_category               // 业务分类

    // 版本信息
    30: optional EvaluationSetVersion evaluation_set_version,  // 版本详情信息
    31: optional string latest_version ,                      // 最新的版本号
    32: optional i64 next_version_num (api.js_conv="true", go.tag = 'json:"next_version_num"'),                   // 下一个的版本号

    // 系统信息
    100: optional common.BaseInfo base_info
}

struct EvaluationSetVersion {
    // 主键&外键
    1: optional i64 id (api.js_conv="true", go.tag = 'json:"id"'),
    2: optional i32 app_id,
    3: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"'),
    4: optional i64 evaluation_set_id (api.js_conv="true", go.tag = 'json:"evaluation_set_id"'),

    // 版本信息
    10: optional string version,                            // 展示的版本号，SemVer2 三段式
    11: optional i64 version_num (api.js_conv="true", go.tag = 'json:"version_num"'),                            // 后端记录的数字版本号，从 1 开始递增
    12: optional string description,                        // 版本描述
    13: optional EvaluationSetSchema evaluation_set_schema     // schema
    14: optional i64 item_count (api.js_conv="true", go.tag = 'json:"item_count"'),                             // 数据条数

    // 系统信息
    100: optional common.BaseInfo base_info                       (go.tag = 'json:"base_info"')
}

// EvaluationSetSchema 评测集 Schema，包含字段的类型限制等信息
struct EvaluationSetSchema {
    // 主键&外键
    1: optional i64 id (api.js_conv="true", go.tag = 'json:"id"'),
    2: optional i32 app_id,
    3: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"'),
    4: optional i64 evaluation_set_id (api.js_conv="true", go.tag = 'json:"evaluation_set_id"'),

    10: optional list<FieldSchema> field_schemas,        // 数据集字段约束

    // 系统信息
    100: optional common.BaseInfo base_info
}

struct FieldSchema {
    1: optional string key,                                     // 唯一键
    2: optional string name,                                    // 展示名称
    3: optional string description,                             // 描述
    4: optional common.ContentType content_type,                // 类型，如 文本，图片，etc.
    5: optional dataset.FieldDisplayFormat default_display_format, // 默认渲染格式，如 code, json, etc.mai
    6: optional dataset.FieldStatus status,                     // 当前列的状态
    7: optional bool isRequired                                 // 是否必填

    // [20,50) 内容格式限制相关
    20: optional string text_schema,                             // 文本内容格式限制，格式为 JSON schema，协议参考 https://json-schema.org/specification
    21: optional dataset.MultiModalSpec multi_model_spec,         // 多模态规格限制

    50: optional bool hidden,                                   // 用户是否不可见

    55: optional list<dataset.FieldTransformationConfig> default_transformations                 // 默认的预置转换配置，目前在数据校验后执行
}

struct EvaluationSetItem {
    // 主键&外键
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"'),                     // 主键，随版本变化
    2: optional i32 app_id,
    3: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"'),
    4: optional i64 evaluation_set_id (api.js_conv='true', go.tag='json:"evaluation_set_id"'),
    5: optional i64 schema_id (api.js_conv='true', go.tag='json:"schema_id"'),
    6: optional i64 item_id (api.js_conv='true', go.tag='json:"item_id"'),                 // 数据在当前数据集内的唯一 ID，不随版本发生改变

    10: optional string item_key,            // 数据插入的幂等 key
    11: optional list<Turn> turns,  // 轮次数据内容

    // 系统信息
    100: optional common.BaseInfo base_info
}

struct Turn {
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"'),                        // 轮次ID，如果是单轮评测集，id=0
    2: optional list<FieldData> field_data_list, // 字段数据
}

struct FieldData {
    1: optional string key,
    2: optional string name,
    3: optional common.Content content,
}


typedef string BizCategory(ts.enum="true")
const BizCategory BizCategory_FromOnlineTrace = "from_online_trace" // 标识来自于在线trace
