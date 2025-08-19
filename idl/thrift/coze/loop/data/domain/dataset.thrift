namespace go coze.loop.data.domain.dataset

enum StorageProvider {
    TOS = 1
    VETOS = 2
    HDFS = 3
    ImageX = 4
    S3 = 5

    /* 后端内部使用 */
    Abase = 100
    RDS = 101
    LocalFS = 102
}

enum DatasetVisibility {
    Public = 1 // 所有空间可见
    Space = 2  // 当前空间可见
    System = 3 // 用户不可见
}

enum SecurityLevel {
    L1 = 1
    L2 = 2
    L3 = 3
    L4 = 4
}

enum DatasetCategory {
    General = 1
    Training = 2
    Validation = 3
    Evaluation = 4
}

enum DatasetStatus {
    Available = 1
    Deleted = 2
    Expired = 3
    Importing = 4
    Exporting = 5
    Indexing = 6
}

enum ContentType {

    /* 基础类型 */
    Text = 1
    Image = 2
    Audio = 3
    Video = 4
    MultiPart = 100 // 图文混排
}

enum FieldDisplayFormat {
    PlainText = 1
    Markdown = 2
    JSON = 3
    YAML = 4
    Code = 5
}

enum SnapshotStatus {
    Unstarted = 1
    InProgress = 2
    Completed = 3
    Failed = 4
}

enum SchemaKey {
    String = 1
    Integer = 2
    Float = 3
    Bool = 4
    Message = 5
}

struct DatasetFeatures {
    1: optional bool editSchema   // 变更 schema
    2: optional bool repeatedData // 多轮数据
    3: optional bool multiModal   // 多模态
}

// Dataset 数据集实体
struct Dataset {
    1: required i64 id (api.js_conv="true", go.tag='json:"id"')
    2: optional i32 app_id
    3: required i64 space_id (api.js_conv="true", go.tag='json:"space_id"')
    4: required i64 schema_id (api.js_conv="true", go.tag='json:"schema_id"')
    10: optional string name
    11: optional string description
    12: optional DatasetStatus status
    13: optional DatasetCategory category             // 业务场景分类
    14: optional string biz_category                   // 提供给上层业务定义数据集类别
    15: optional DatasetSchema schema                 // 当前数据集结构
    16: optional SecurityLevel security_level          // 密级
    17: optional DatasetVisibility visibility         // 可见性
    18: optional DatasetSpec spec                     // 规格限制
    19: optional DatasetFeatures features             // 数据集功能开关
    20: optional string latest_version                 // 最新的版本号
    21: optional i64 next_version_num (api.js_conv="true", go.tag='json:"next_version_num"')                   // 下一个的版本号
    22: optional i64 item_count (api.js_conv="true", go.tag='json:"item_count"')    // 数据条数

    /* 通用信息 */
    100: optional string created_by
    101: optional i64 created_at (api.js_conv="true", go.tag='json:"created_at"')
    102: optional string updated_by
    103: optional i64 updated_at (api.js_conv="true", go.tag='json:"updated_at"')
    104: optional i64 expired_at (api.js_conv="true", go.tag='json:"expired_at"')

    /* DTO 专用字段 */
    150: optional bool change_uncommitted              // 是否有未提交的修改
}

struct DatasetSpec {
    1: optional i64 max_item_count (api.js_conv="true", go.tag='json:"max_item_count"')  // 条数上限
    2: optional i32 max_field_count // 字段数量上限
    3: optional i64 max_item_size (api.js_conv="true", go.tag='json:"max_item_size"')   // 单条数据字数上限
    4: optional i32 max_item_data_nested_depth
}

// DatasetVersion 数据集版本元信息，不包含数据本身
struct DatasetVersion {
    1: required i64 id (api.js_conv="true", go.tag='json:"id"')
    2: optional i32 app_id
    3: required i64 space_id (api.js_conv="true", go.tag='json:"space_id"')
    4: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"')
    5: required i64 schema_id (api.js_conv="true", go.tag='json:"schema_id"')
    10: optional string version                        // 展示的版本号，SemVer2 三段式
    11: optional i64 version_num (api.js_conv="true", go.tag='json:"version_num"')  // 后端记录的数字版本号，从 1 开始递增
    12: optional string description                    // 版本描述
    13: optional string dataset_brief                   // marshal 后的版本保存时的数据集元信息，不包含 schema
    14: optional i64 item_count (api.js_conv="true", go.tag='json:"item_count"')   // 数据条数
    15: optional SnapshotStatus snapshot_status         // 当前版本的快照状态

    /* 通用信息 */
    100: optional string created_by
    101: optional i64 created_at (api.js_conv="true", go.tag='json:"created_at"')
    102: optional i64 disabled_at (api.js_conv="true", go.tag='json:"disabled_at"') // 版本禁用的时间
}

// DatasetSchema 数据集 Schema，包含数据集列的类型限制等信息
struct DatasetSchema {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')              // 主键 ID，创建时可以不传
    2: optional i32 app_id                                 // schema 所在的空间 ID，创建时可以不传
    3: optional i64 space_id (api.js_conv="true", go.tag='json:"space_id"')         // schema 所在的空间 ID，创建时可以不传
    4: optional i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"')       // 数据集 ID，创建时可以不传
    10: optional list<FieldSchema> fields                 // 数据集列约束
    11: optional bool immutable                           // 是否不允许编辑

    /* 通用信息 */
    100: optional string created_by
    101: optional i64 created_at (api.js_conv="true", go.tag='json:"created_at"')
    102: optional string updated_by
    103: optional i64 updated_at (api.js_conv="true", go.tag='json:"updated_at"')
    104: optional i64 update_version (api.js_conv="true", go.tag='json:"update_version"')
}

enum FieldStatus {
    Available = 1
    Deleted = 2
}

struct FieldSchema {
    1: optional string key                                                              // 数据集 schema 版本变化中 key 唯一，新建时自动生成，不需传入
    2: optional string name (vt.min_size = "1", vt.max_size = "128")                    // 展示名称
    3: optional string description (vt.max_size = "1024")                               // 描述
    4: optional ContentType content_type (vt.not_nil = "true", vt.defined_only = "true") // 类型，如 文本，图片，etc.
    5: optional FieldDisplayFormat default_format (vt.defined_only = "true")             // 默认渲染格式，如 code, json, etc.
    6: optional SchemaKey schemaKey                                                     // 对应的内置 schema

    /* [20,50) 内容格式限制相关 */
    20: optional string text_schema                                   // 文本内容格式限制，格式为 JSON schema，协议参考 https://json-schema.org/specification
    21: optional MultiModalSpec multi_model_spec                       // 多模态规格限制
    50: optional bool hidden                                         // 用户是否不可见
    51: optional FieldStatus status                                  // 当前列的状态，创建/更新时可以不传

    55: optional list<FieldTransformationConfig> default_transformations                 // 默认的预置转换配置，目前在数据校验后执行
}

enum FieldTransformationType {
    RemoveExtraFields = 1 // 移除未在当前列的 jsonSchema 中定义的字段（包括 properties 和 patternProperties），仅在列类型为 struct 时有效
}
struct FieldTransformationConfig {
    1: optional FieldTransformationType transType // 预置的转换类型
    2: optional bool global                       // 当前转换配置在这一列上的数据及其嵌套的子结构上均生效
}

struct MultiModalSpec {
    1: optional i64 max_file_count (api.js_conv="true", go.tag='json:"max_file_count"')               // 文件数量上限
    2: optional i64 max_file_size (api.js_conv="true", go.tag='json:"max_file_size"')                // 文件大小上限
    3: optional list<string> supported_formats // 文件格式
}

// DatasetItem 数据内容
struct DatasetItem {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')                            // 主键 ID，创建时可以不传
    2: optional i32 app_id                                               // 冗余 app ID，创建时可以不传
    3: optional i64 space_id (api.js_conv="true", go.tag='json:"space_id"')                       // 冗余 space ID，创建时可以不传
    4: optional i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"')                     // 所属的 data ID，创建时可以不传
    5: optional i64 schema_id (api.js_conv="true", go.tag='json:"schema_id"')                      // 插入时对应的 schema ID，后端根据 req 参数中的 datasetID 自动填充
    6: optional i64 item_id (api.js_conv="true", go.tag='json:"item_id"')                        // 数据在当前数据集内的唯一 ID，不随版本发生改变
    10: optional string item_key (vt.max_size = "255")                   // 数据插入的幂等 key
    11: optional list<FieldData> data (vt.elem.not_nil = "true")        // 数据内容
    12: optional list<ItemData> repeated_data (vt.elem.not_nil = "true") // 多轮数据内容，与 data 互斥

    /* 通用信息 */
    100: optional string created_by
    101: optional i64 created_at (api.js_conv="true", go.tag='json:"created_at"')
    102: optional string updated_by
    103: optional i64 updated_at (api.js_conv="true", go.tag='json:"updated_at"')

    /* DTO 专用字段 */
    150: optional bool data_omitted                                      // 数据（data 或 repeatedData）是否省略。列表查询 item 时，特长的数据内容不予返回，可通过单独 Item 接口获取内容
}

struct ItemData {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')
    2: optional list<FieldData> data
}

struct FieldData {
    1: optional string key
    2: optional string name                     // 字段名，写入 Item 时 key 与 name 提供其一即可，同时提供时以 key 为准
    3: optional ContentType content_type
    4: optional string content
    5: optional list<ObjectStorage> attachments // 外部存储信息
    6: optional FieldDisplayFormat format       // 数据的渲染格式
    7: optional list<FieldData> parts           // 图文混排时，图文内容
}

struct ObjectStorage {
    1: optional StorageProvider provider (vt.defined_only = "true")
    2: optional string name
    3: optional string uri (vt.min_size = "1")
    4: optional string url
    5: optional string thumb_url
}

struct OrderBy {
    1: optional string field // 排序字段
    2: optional bool is_asc   // 升序，默认倒序
}

struct FileUploadToken {
    1: optional string access_key_id
    2: optional string secret_access_key
    3: optional string session_token
    4: optional string expired_time
    5: optional string current_time
}

enum ItemErrorType {
    MismatchSchema = 1        // schema 不匹配
    EmptyData = 2             // 空数据
    ExceedMaxItemSize = 3     // 单条数据大小超限
    ExceedDatasetCapacity = 4 // 数据集容量超限
    MalformedFile = 5         // 文件格式错误
    IllegalContent = 6        // 包含非法内容
    MissingRequiredField = 7  // 缺少必填字段
    ExceedMaxNestedDepth = 8  // 数据嵌套层数超限
    TransformItemFailed = 9   // 数据转换失败

    /* system error*/
    InternalError = 100
}

struct ItemErrorDetail {
    1: optional string message
    2: optional i32 index      // 单条错误数据在输入数据中的索引。从 0 开始，下同
    3: optional i32 start_index // [startIndex, endIndex] 表示区间错误范围, 如 ExceedDatasetCapacity 错误时
    4: optional i32 end_index
}

struct ItemErrorGroup {
    1: optional ItemErrorType type
    2: optional string summary
    3: optional i32 error_count                // 错误条数
    4: optional list<ItemErrorDetail> details // 批量写入时，每类错误至多提供 5 个错误详情；导入任务，至多提供 10 个错误详情
}