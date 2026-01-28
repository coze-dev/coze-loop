namespace go stone.fornax.ml_flow.domain.datasetv2t

include "datasetv2similarity.thrift"
include "../domain/tag.thrift"

enum StorageProvider {
    TOS = 1
    VETOS = 2
    HDFS = 3
    ImageX = 4

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
    General = 1    // 数据集
    Training = 2   // 训练集 (暂无)
    Validation = 3 // 验证集 (暂无)
    Evaluation = 4 // 评测集 (暂无)
    Result = 5     // 结果集 (暂无)
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
    SingleOption = 6
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
    SingleChoice = 6 // 单选
    Trajectory = 7   // 轨迹
}

struct DatasetFeatures {
    1: optional bool editSchema   // 变更 schema
    2: optional bool repeatedData // 多轮数据
    3: optional bool multiModal   // 多模态
}

struct DatasetBizProperties {
    1: optional TrainingSetProperties trainingSet
}

struct TrainingSetProperties {
    1: optional bool allowFunctionCalling
    2: optional bool allowImgUnderstanding
}

// Dataset 数据集实体
struct Dataset {
    1: required i64 id (api.js_conv = "str")
    2: optional i32 appID
    3: required i64 spaceID (api.js_conv = "str")
    4: required i64 schemaID (api.js_conv = "str")
    10: optional string name
    11: optional string description
    12: optional DatasetStatus status
    13: optional DatasetCategory category             // 业务场景分类
    14: optional string bizCategory                   // 提供给上层业务定义数据集类别
    15: optional DatasetSchema schema                 // 当前数据集结构
    16: optional SecurityLevel securityLevel          // 密级
    17: optional DatasetVisibility visibility         // 可见性
    18: optional DatasetSpec spec                     // 规格限制
    19: optional DatasetFeatures features             // 数据集功能开关
    20: optional string latestVersion                 // 最新的版本号
    21: optional i64 nextVersionNum                   // 下一个的版本号
    22: optional i64 itemCount (api.js_conv = "str")  // 数据条数
    23: optional DatasetBizProperties bizProperties   // 提供给上层业务定义的属性

    /* 通用信息 */
    100: optional string createdBy
    101: optional i64 createdAt (api.js_conv = "str")
    102: optional string updatedBy
    103: optional i64 updatedAt (api.js_conv = "str")
    104: optional i64 expiredAt (api.js_conv = "str")

    /* DTO 专用字段 */
    150: optional bool changeUncommitted              // 是否有未提交的修改
    151: optional list<DatasetLockInfo> lockInfo      // 数据集锁定信息
}

struct DatasetLockInfo {
    1: optional DatasetLockReason reason
    2: optional i64 crowdsourcingAnnotateJobID (api.js_conv = "str") // 众包标注任务ID
    3: optional i64 pipelineRunID (api.js_conv = "str")              // 工作流运行ID. 在 reason=PipelineTemporary 时有值
}

enum DatasetLockReason {
    Undefined = 0
    CrowdsourcingAnnotateJobRunning = 1 // 众包标注任务正在运行
    PipelineTemporary = 2 // 工作流过程处理中的临时数据集，有可能永远都不会解锁（除非被工作流转正）
}

struct DatasetSpec {
    1: optional i64 maxItemCount (api.js_conv = "str")           // 条数上限
    2: optional i32 maxFieldCount (api.js_conv = "str")          // 字段数量上限
    3: optional i64 maxItemSize (api.js_conv = "str")            // 单条数据字数上限
    4: optional i32 maxItemDataNestedDepth (api.js_conv = "str") // 单条 array/struct 数据嵌套上限
    5: optional MultiModalSpec multiModalSpec
}

// DatasetVersion 数据集版本元信息，不包含数据本身
struct DatasetVersion {
    1: required i64 id (api.js_conv = "str")
    2: optional i32 appID
    3: required i64 spaceID (api.js_conv = "str")
    4: required i64 datasetID (api.js_conv = "str")
    5: required i64 schemaID (api.js_conv = "str")
    10: optional string version                        // 展示的版本号，SemVer2 三段式
    11: optional i64 versionNum (api.js_conv = "str")  // 后端记录的数字版本号，从 1 开始递增
    12: optional string description                    // 版本描述
    13: optional string datasetBrief                   // marshal 后的版本保存时的数据集元信息，不包含 schema
    14: optional i64 itemCount (api.js_conv = "str")   // 数据条数
    15: optional SnapshotStatus snapshotStatus         // 当前版本的快照状态

    /* 通用信息 */
    100: optional string createdBy
    101: optional i64 createdAt (api.js_conv = "str")
    102: optional i64 disabledAt (api.js_conv = "str") // 版本禁用的时间
    103: optional string updatedBy
    104: optional i64 updatedAt (api.js_conv = "str")
}

// DatasetSchema 数据集 Schema，包含数据集列的类型限制等信息
struct DatasetSchema {
    1: optional i64 id (api.js_conv = "str")              // 主键 ID，创建时可以不传
    2: optional i32 appID                                 // schema 所在的空间 ID，创建时可以不传
    3: optional i64 spaceID (api.js_conv = "str")         // schema 所在的空间 ID，创建时可以不传
    4: optional i64 datasetID (api.js_conv = "str")       // 数据集 ID，创建时可以不传
    10: optional list<FieldSchema> fields                 // 数据集列约束
    11: optional bool immutable                           // 是否不允许编辑

    /* 通用信息 */
    100: optional string createdBy
    101: optional i64 createdAt (api.js_conv = "str")
    102: optional string updatedBy
    103: optional i64 updatedAt (api.js_conv = "str")
    104: optional i64 updateVersion (api.js_conv = "str")
}

enum FieldStatus {
    Available = 1
    Deleted = 2
}

struct FieldSchema {
    1: optional string key                                                              // 数据集 schema 版本变化中 key 唯一，新建时自动生成，不需传入
    2: optional string name (vt.min_size = "1", vt.max_size = "256")                    // 展示名称
    3: optional string description (vt.max_size = "1024")                               // 描述
    4: optional ContentType contentType (vt.not_nil = "true", vt.defined_only = "true") // 类型，如 文本，图片，etc.
    5: optional FieldDisplayFormat defaultFormat (vt.defined_only = "true")             // 默认渲染格式，如 code, json, etc.
    6: optional SchemaKey schemaKey                                                     // 对应的内置 schema

    /* [20,50) 内容格式限制相关 */
    20: optional string textSchema                                                      // 文本内容格式限制，格式为 JSON schema，协议参考 https://json-schema.org/specification
    21: optional MultiModalSpec multiModelSpec                                          // 多模态规格限制
    22: optional bool isRequired                                                        // 当前列的数据是否必填，不填则会报错
    50: optional bool hidden                                                            // 用户是否不可见
    51: optional FieldStatus status                                                     // 当前列的状态，创建/更新时可以不传
    52: optional SimilaritySearchConfig similaritySearchConfig                          // 是否开启相似度索引
    53: optional QualityScoreConfig qualityScoreConfig                                  // 质量分配置
    54: optional TagFieldConfig tagFieldConfig                                          // 标签字段配置
    55: optional list<FieldTransformationConfig> defaultTransformations                 // 默认的预置转换配置，目前在数据校验后执行
}

enum FieldTransformationType {
    RemoveExtraFields = 1 // 移除未在当前列的 jsonSchema 中定义的字段（包括 properties 和 patternProperties），仅在列类型为 struct 时有效
}

struct FieldTransformationConfig {
    1: optional FieldTransformationType transType // 预置的转换类型
    2: optional bool global                       // 当前转换配置在这一列上的数据及其嵌套的子结构上均生效
}

// 质量分配置
struct QualityScoreConfig {
    1: optional bool enabled // 列是否为质量分
}

// 相似度算法的配置
struct SimilaritySearchConfig {
    1: optional bool enabled                                                // 是否开启相似度索引
    2: optional datasetv2similarity.SimilarityAlgorithm similarityAlgorithm // 配置了哪个相似度算法
    3: optional datasetv2similarity.EmbeddingModel embeddingType            // 所使用的相似度模型
}

struct TagFieldConfig {
    1: optional tag.TagInfo tagInfo // tag配置
}

struct MultiModalSpec {
    1: optional i64 maxFileCount (api.js_conv = "str") // 文件数量上限
    2: optional i64 maxFileSize (api.js_conv = "str")  // 文件大小上限
    3: optional list<string> supportedFormats          // 文件格式
    4: optional i32 maxPartCount (api.js_conv = "str") // 多模态节点总数上限
}

// DatasetItem 数据内容
struct DatasetItem {
    1: optional i64 id (api.js_conv = "str")                            // 主键 ID，创建时可以不传
    2: optional i32 appID                                               // 冗余 app ID，创建时可以不传
    3: optional i64 spaceID (api.js_conv = "str")                       // 冗余 space ID，创建时可以不传
    4: optional i64 datasetID (api.js_conv = "str")                     // 所属的 data ID，创建时可以不传
    5: optional i64 schemaID (api.js_conv = "str")                      // 插入时对应的 schema ID，后端根据 req 参数中的 datasetID 自动填充
    6: optional i64 itemID (api.js_conv = "str")                        // 数据在当前数据集内的唯一 ID，不随版本发生改变
    10: optional string itemKey (vt.max_size = "255")                   // 数据插入的幂等 key
    11: optional list<FieldData> data (vt.elem.not_nil = "true")        // 数据内容
    12: optional list<ItemData> repeatedData (vt.elem.not_nil = "true") // 多轮数据内容，与 data 互斥
    13: optional ItemSource source                                      // item 的来源信息，批量返回时不填充任务的详细信息。创建时不填充则视为为手动添加

    /* 通用信息 */
    100: optional string createdBy
    101: optional i64 createdAt (api.js_conv = "str")
    102: optional string updatedBy
    103: optional i64 updatedAt (api.js_conv = "str")

    /* DTO 专用字段 */
    150: optional bool dataOmitted                                      // 数据（data 或 repeatedData）是否省略。列表查询 item 时，特长的数据内容不予返回，可通过单独 Item 接口获取内容
}

struct ItemData {
    1: optional i64 id (api.js_conv = "str")
    2: optional list<FieldData> data
}

struct FieldData {
    1: optional string key
    2: optional string name                     // 字段名，写入 Item 时 key 与 name 提供其一即可，同时提供时以 key 为准
    3: optional ContentType contentType
    4: optional string content
    5: optional list<ObjectStorage> attachments // 外部存储信息  deprecated, use image/audio/... instead
    6: optional FieldDisplayFormat format       // 数据的渲染格式
    7: optional list<FieldData> parts           // 图文混排时，图文内容
    8: optional string traceID                  // 这条数据生成traceID
    9: optional bool genFail                    // 是否生成失败
    10: optional string fallbackDisplayName     // 标签回流失败后的展示名称
    11: optional bool contentOmitted       // 当前列的数据是否省略, 如果此处返回 true, 需要通过 GetDatasetItemField 获取当前列的具体内容, 或者是通过 omittedDataStorage.url 下载
    12: optional ObjectStorage fullContent // 被省略数据的完整信息，批量返回时会签发相应的 url，用户可以点击下载. 同时支持通过该字段传入已经上传好的超长数据(dataOmitted 为 true 时生效)
    13: optional i32 fullContentBytes      // 超长数据完整内容的大小，单位 byte
    // [30, 50)平铺的 MultiModal 结构，替代上面的 attachments. 如果指定了这边的 image, 则 attachments 会被忽略
    30: optional ImageField image
}

struct ImageField {
    1: optional StorageProvider storageProvider (vt.defined_only = "true") // 创建时如果为空，则会从对应的 url 下载文件并上传到默认的存储中
    2: optional string name,
    3: optional string url,
    4: optional string uri,
    5: optional string thumb_url,
}

struct ObjectStorage {
    1: optional StorageProvider provider (vt.defined_only = "true")
    2: optional string name
    3: optional string uri (vt.min_size = "1")
    4: optional string url
    5: optional string thumbURL
}

struct OrderBy {
    1: optional string field // 排序字段，默认是updated_at和created_at，在基于数据行int/float排序的场景，会传入fieldKey
    2: optional bool isAsc   // 升序，默认倒序
    3: optional bool isFieldKey // 是否为数据集fieldKey，默认false，向前兼容
}

struct FileUploadToken {
    1: optional string accessKeyID
    2: optional string secretAccessKey
    3: optional string sessionToken
    4: optional string expiredTime
    5: optional string currentTime
}

struct CreateDatasetItemOutput {
    1: optional i32 itemIndex                    // item 在 BatchCreateDatasetItemsReq.items 中的索引
    2: optional string itemKey
    3: optional i64 itemID (api.js_conv = "str")
    4: optional bool isNewItem                   // 是否是新的 Item。提供 itemKey 时，如果 itemKey 在数据集中已存在数据，则不算做「新 Item」，该字段为 false。
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
    ExceedMaxImageCount = 10  // 图片数量超限
    ExceedMaxImageSize = 11   // 图片大小超限
    GetImageFailed = 12       // 图片获取失败（例如图片不存在/访问不在白名单内的内网链接）
    IllegalExtension = 13     // 文件扩展名不合法
    ExceedMaxPartCount = 14   // 多模态节点数量超限
    ItemNotFound = 15         // Item 不存在（仅更新的场景使用）

    /* system error*/
    InternalError = 100
    ClearDatasetFailed = 101  // 清空数据集失败
    RWFileFailed = 102        // 读写文件失败
    UploadImageFailed = 103   // 上传图片失败
}

struct ItemErrorDetail {
    1: optional string message
    2: optional i32 index                           // 单条错误数据在输入数据中的索引。从 0 开始，下同
    3: optional i32 startIndex                      // [startIndex, endIndex] 表示区间错误范围, 如 ExceedDatasetCapacity 错误时
    4: optional i32 endIndex
    5: optional map<string, string> messagesByField // ItemErrorType=MismatchSchema, key 为 FieldSchema.name, value 为错误信息
}

struct ItemErrorGroup {
    1: optional ItemErrorType type
    2: optional string summary
    3: optional i32 errorCount                // 错误条数
    4: optional list<ItemErrorDetail> details // 批量写入时，每类错误至多提供 5 个错误详情；导入任务，至多提供 10 个错误详情
}

enum LineageSourceType {
    Manual = 1
    Dataset = 2                 // 需要根据 ItemSource.dataset.category 字段区分评测集/数据集/...
    FileStorage = 3             // 需要根据 ItemSource.file.storage 字段区分HDFS/本地上传/...
    DataReflow = 4              // 数据回流，需要根据 ItemSource.span.isManual 是否是手动回流。如果是自动回流，则 ItemSource.jobID 中会包含对应的任务 ID
    DataAnnotation = 5          // 暂无
    DataProcessing = 6          // 暂无
    DataGenerate = 7            // 暂无
    OpenAPI = 8
    CrowdsourcingAnnotation = 9 // 众包标注
}

enum TrackedJobType {
    DatasetIOJob = 1            // 数据导入任务
    DataReflow = 2              // 数据回流任务
    DataAnnotation = 3          // 标注任务
    DataProcessing = 4          // 数据处理任务
    DataGenerate = 5            // 数据生成任务
    CrowdsourcingAnnotation = 6 // 众包标注任务
}

struct ItemSource {
    1: required LineageSourceType type
    2: optional TrackedItem trackedItem              // 源 item 信息，可以为空
    3: optional TrackedJobType jobType               // 任务类型，根据该字段区分数据导入任务/数据回流任务/...
    4: optional i64 jobID (api.js_conv = "str")      // item 关联的任务 id，为 0 表示无相应任务(例如数据是通过克隆另一数据行产生的)
    5: optional TrackedDataset dataset               // type = Dataset 时，从该字段获取数据集具体信息
    6: optional TrackedFile file                     // type = FileStorage 时，从该字段获取文件信息
    7: optional TrackedTraceSpan span                // type = DataReflow 时，从该字段获取 span 信息
    51: optional i64 createdAt (api.js_conv = "str")
}

struct TrackedItem {
    1: optional i64 spaceID (api.js_conv = "str")
    2: optional i64 datasetID (api.js_conv = "str")
    3: optional i64 itemID (api.js_conv = "str")
    4: optional i64 versionID (api.js_conv = "str") // 版本号提交后的版本 id，为 0 表示为草稿版本
    5: optional string version                      // 版本号（三段式）
}

struct TrackedFile {
    1: optional StorageProvider storage // 存储介质，根据该字段区分 hdfs/本地文件(即 ImageX)/...
    2: optional string originalFileName // 用户上传文件的原始文件名
}

// 源数据集信息（数据集 id 从 TrackedItem 中获取，此处不额外返回）
struct TrackedDataset {
    1: optional DatasetCategory category // 数据集类别，根据该字段区分评测集/数据集/...
    2: optional string datasetName
}

struct TrackedTraceSpan {
    1: optional string traceID
    2: optional string spanID
    3: optional string spanName
    4: optional string spanType
    5: optional bool isManual   // 是否手工回流
}

struct UpdateDatasetItemOutput {
    1: optional i64 itemID (api.js_conv = "str")
    2: optional string itemKey
}