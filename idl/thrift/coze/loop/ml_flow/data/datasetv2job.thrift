namespace go stone.fornax.ml_flow.domain.datasetv2jobt

include "datasetv2.thrift"
include "datasetv2similarity.thrift"
include "trainingdatasetv2.thrift"
include "../../../../coze/loop/data/domain/common.thrift"

// 通用任务类型
enum JobType {
    ImportFromFile = 1
    ExportToFile = 2
    ExportToDataset = 3
}

// 通用任务状态
enum JobStatus {
    Undefined = 0
    Pending = 1   // 待处理
    Running = 2   // 处理中
    Completed = 3 // 已完成
    Failed = 4    // 失败
    Cancelled = 5 // 已取消
}

const string LogLevelInfo = "info"
const string LogLevelError = "error"
const string LogLevelWarning = "warning"

// 通用任务日志
struct JobLog {
    1: required string content
    2: required string level
    3: required i64 timestamp (agw.js_conv = "str")
    10: required bool hidden
}

enum FileFormat {
    JSONL = 1
    Parquet = 2
    CSV = 3
    XLSX = 4

    /*[100, 200) 压缩格式*/
    ZIP = 100
}

struct DatasetIOFile {
    1: required datasetv2.StorageProvider provider (vt.defined_only = "true")
    2: optional string path (vt.min_size = "1")
    3: optional FileFormat format                                             // 数据文件的格式
    4: optional FileFormat compressFormat                                     // 压缩包格式
    5: optional list<string> files                                            // path 为文件夹或压缩包时，数据文件列表, 服务端设置
    6: optional string originalFileName                                       // 原始的文件名，创建文件时由前端写入。为空则与 path 保持一致
    7: optional string downloadURL                                            // 文件下载地址
    100: optional string providerID                                           // 存储提供方ID，目前主要在 provider==imagex 时生效
    101: optional ProviderAuth providerAuth                                   // 存储提供方鉴权信息，目前主要在 provider==imagex 时生效
}

enum IODatasetProvider {
    Undefined = 0
    Evaluation = 1
    Data = 2 // 新版数据集（数据集引擎）
}

struct DatasetIODataset {
    1: optional i64 spaceID (agw.js_conv = "str")
    2: optional i64 datasetID (agw.js_conv = "str")
    3: optional i64 versionID (agw.js_conv = "str")
    4: optional datasetv2.Dataset dataset                       // 数据集详情，在接口上返回，不写入
    5: optional datasetv2.DatasetVersion version                // 版本详情，在接口上返回，不写入
    6: optional string newDatasetName
    7: optional string newDatasetDesc
    8: optional datasetv2.DatasetCategory datasetCategory
    9: optional string bizCategory
    10: optional IODatasetProvider provider                     // 数据集来源，如果不填写则会默认填充为旧版数据集。notice:目前仅在导出任务的目标数据集上生效
    11: optional list<i64> itemIDs                              // 需要导出的 item ID
    12: optional trainingdatasetv2.UsageScene usageScene        // for 训练集, 使用场景
    13: optional list<trainingdatasetv2.DataFormat> dataFormats // for 训练集, 数据格式
}

struct DatasetIOEndpoint {
    1: optional DatasetIOFile file
    2: optional DatasetIODataset dataset
}

// DatasetIOJob 数据集导入导出任务
struct DatasetIOJob {
    1: required i64 id (agw.js_conv = "str")
    2: optional i32 appID
    3: required i64 spaceID (agw.js_conv = "str")
    4: required i64 datasetID (agw.js_conv = "str")   // 导入导出到文件时，为数据集 ID；数据集间转移时，为目标数据集 ID
    5: required JobType jobType
    6: required DatasetIOEndpoint source
    7: required DatasetIOEndpoint target
    8: optional list<FieldMapping> fieldMappings      // 字段映射
    9: optional DatasetIOJobOption option
    10: optional Visibility visibility                // 任务可见性

    /* 运行数据, [20, 100) */
    20: optional JobStatus status
    21: optional DatasetIOJobProgress progress
    22: optional list<datasetv2.ItemErrorGroup> errors

    /* 通用信息 */
    100: optional string createdBy
    101: optional i64 createdAt (agw.js_conv = "str")
    102: optional string updatedBy
    103: optional i64 updatedAt (agw.js_conv = "str")
    104: optional i64 startedAt (agw.js_conv = "str")
    105: optional i64 endedAt (agw.js_conv = "str")
    106: optional common.UserInfo createdByDetail
    107: optional common.UserInfo updatedByDetail
}

struct DatasetIOJobOption {
    1: optional bool overwriteDataset                                     // 覆盖数据集，仅在导入任务中生效
    2: optional i64 jobID                                                 // 需要按照手动打标的taskID结果导入，被确认无需导入的不会被导入，仅在导入任务中生效
    3: optional WriteMode writeMode                                       // 覆盖还是追加
    4: optional ExportETLStrategy etlStrategy                             // 导出时的数据清洗策略
    5: optional list<CallbackOption> callbackOptions
    6: optional list<InternalFieldExportConfig> internalFieldExportConfig // 包含的系统内置信息，目前仅在导出到文件时生效
    7: optional UpdateMatchConfig updateMatchConfig                       // 更新匹配方式配置，目前仅在从文件导入时生效
}

struct InternalFieldExportConfig {
    1: optional string internalField   // 内部字段名。只支持 item_id 和 dataset_id。
    2: optional string targetFieldName // 导出后的目标列名。本期不填写 item_id 默认为 __system_internal_id__，dataset_id 默认为 __system_dataset_id__
}

struct UpdateMatchConfig {
    1: optional OnNotFoundAction onNotFoundAction // 当匹配列未找到对应数据时的处理策略
    2: optional string mappingFieldName           // 原地更新时，导入文件中对应的列名，不传默认为 __system_internal_id__
}

struct DatasetIOJobProgress {
    2: optional i64 total                                 // 总量
    3: optional i64 processed                             // 已处理数量
    4: optional i64 added                                 // 已成功处理的数量
    5: optional i64 skipped                               // 已跳过的数量
    6: optional string cursor                             // 下一个扫描的游标，在数据源为数据集时生效
    7: optional i64 updated                               // 已更新的数量

    /*子任务*/
    10: optional string name                              // 可空, 表示子任务的名称
    11: optional list<DatasetIOJobProgress> subProgresses // 子任务的进度
}

struct FieldMapping {
    1: required string source (vt.min_size = "1")
    2: required string target (vt.min_size = "1")
    3: optional bool isNewField                   // 在文件导入的场景下，目标 field 是否为新字段，创建任务时由后端计算。如果对应 target field 不存在并且未指定 field schema，则会返回报错。
    4: optional datasetv2.FieldSchema fieldSchema // 新字段的 schema，在文件导入的场景下，目标 field 为新字段时必填
}

enum SourceType {
    File = 1
    Dataset = 2
}

struct ItemDeduplicateJob {
    1: required i64 id (agw.js_conv = "str"),
    2: required i64 spaceID (agw.js_conv = "str")
    3: required i64 datasetID (agw.js_conv = "str")

    /* 导入文件需要的数据 */
    10: optional JobType jobType
    11: optional DatasetIOEndpoint source
    12: optional DatasetIOEndpoint target
    13: optional list<FieldMapping> fieldMappings      // 字段映射
    14: optional DatasetIOJobOption option

     /* job信息 */
    20: optional JobStatus status // 如果status=Completed,则表明已经处理完成
    22: optional string itemDedupJobBrief // 任务当时的简要信息，冗余存储
    23: optional string fieldKey // 根据哪一列去重
    24: optional datasetv2similarity.SimilarityAlgorithm similarityAlgorithm    // 去重算法
    25: optional i64 threshold       // 阈值
    26: optional i64 jobTotal  // job中需要处理的总数据，不跟随筛选条件变动
    27: optional i64 confirmedDedupItemsCount      // 已确认的重复条数，不跟随筛选条件变动
    28: optional i64 confirmedNotDedupItemsCount   // 已确认的不重复条数，不跟随筛选条件变动
    29: optional string error // 错误信息，当 JobStatus=Failed时使用

    /* 去重列表信息 */
    30: optional list<ItemDeduplicatePair> pairs,   // 去重列表的内容
    31: optional i64 total  // pairs总条数，跟随筛选条件变动

    /* 通用信息 */
    103: optional string createdBy
    104: optional i64 createdAt (agw.js_conv = "str")
    105: optional string updatedBy
    106: optional i64 updatedAt (agw.js_conv = "str")
    107: optional i64 startedAt (agw.js_conv = "str")
    108: optional i64 endedAt (agw.js_conv = "str")
}

struct ItemDeduplicatePair {
    1: required i64 id (agw.js_conv = "str"),
    2: required string uniqKey,  // 本条主键
    3: optional datasetv2.DatasetItem newItem,  // 新导入的内容
    4: optional list<SuspectedDupItemInfo> items, // 可能重复的内容
    5: optional ImportConfirmType importConfirmType,    // 是否确认

    103: optional string createdBy
    104: optional i64 createdAt (agw.js_conv = "str")
    105: optional string updatedBy
    106: optional i64 updatedAt (agw.js_conv = "str")
}

struct SuspectedDupItemInfo {
    1: datasetv2.DatasetItem item,  // 行内容
    2: i64 score,    // 相似度评分
}

enum ImportConfirmType {
    NotConfirmed = 0
    ConfirmedDuplicated = 1
    ConfirmedNotDuplicated = 2
}

enum WriteMode {
    OverWrite = 1
    Append = 2
}

struct ExportETLStrategy {
    1: optional ExportETLStrategyType type
    2: optional string etlConfig // 不同type有不同的配置

    50: optional list<datasetv2.FieldSchema> targetSchema // 设置后可以覆盖原数据集的 schema 配置
}

enum ExportETLStrategyType {
    None = 0
    TrainingDatasetThinkingForArkSft = 1
    TrainingDatasetExtraAndThinkingForArkGRPO = 2
}

struct ProviderAuth {
    1: optional i64 providerAccountID (agw.js_conv = "str") // provider == VETOS 时，此处存储的是用户在 fornax 上托管的方舟账号的ID
}

enum CallbackType {
    Workflow = 1
}

struct CallbackOption {
    1: optional CallbackType callbackType
}

enum Visibility {
    System = 1 // 用户不可见
}

enum OnNotFoundAction {
    None   = 0
    Failed = 1
    Append = 2
}
