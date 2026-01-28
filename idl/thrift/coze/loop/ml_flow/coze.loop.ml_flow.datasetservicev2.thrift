namespace go stone.fornax.ml_flow.datasetservicev2

include "../../../base.thrift"
include "../ml_flow/data/datasetv2.thrift"
include "../ml_flow/data/datasetv2job.thrift"
include "../ml_flow/data/filter.thrift"
include "../ml_flow/data/datasetv2similarity.thrift"

typedef ListDatasetItemsReq SearchDatasetItemsReq
typedef ListDatasetItemsResp SearchDatasetItemsResp
typedef ListDatasetVersionsReq SearchDatasetVersionsReq
typedef ListDatasetVersionsResp SearchDatasetVersionsResp
typedef ListDatasetItemsByVersionReq SearchDatasetItemsByVersionReq
typedef ListDatasetItemsByVersionResp SearchDatasetItemsByVersionResp
typedef ListDatasetIOJobsOfDatasetReq SearchDatasetIOJobsOfDatasetReq
typedef ListDatasetIOJobsOfDatasetResp SearchDatasetIOJobsOfDatasetResp

struct CreateDatasetReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: optional i32 appID (api.js_conv = "str")
    3: required string name (vt.min_size = "1", vt.max_size = "255")
    4: optional string description (vt.max_size = "2048")
    5: optional datasetv2.DatasetCategory category (vt.defined_only = "true")
    6: optional string bizCategory (vt.max_size = "128")
    7: optional list<datasetv2.FieldSchema> fields (vt.min_size = "1", vt.elem.skip = "false")
    15: optional datasetv2.SecurityLevel securityLevel (vt.defined_only = "true")
    16: optional datasetv2.DatasetVisibility visibility (vt.defined_only = "true")
    17: optional datasetv2.DatasetSpec spec
    18: optional datasetv2.DatasetFeatures features
    19: optional string userID
    20: optional i64 createdAt
    255: optional base.Base base
}

struct CreateDatasetResp {
    1: optional i64 datasetID (api.js_conv = "str")
    255: optional base.BaseResp baseResp
}

struct UpdateDatasetReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional string name (vt.max_size = "255")
    4: optional string description (vt.max_size = "2048")
    255: optional base.Base base
}

struct UpdateDatasetResp {
    255: optional base.BaseResp baseResp
}

struct DeleteDatasetReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.gt = "0", vt.not_nil = "true")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    255: optional base.Base base
}

struct DeleteDatasetResp {
    255: optional base.BaseResp baseResp
}

struct GetDatasetReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    10: optional bool withDeleted                                                        // 数据集已删除时是否返回
    255: optional base.Base base
}

struct GetDatasetResp {
    1: optional datasetv2.Dataset dataset
    255: optional base.BaseResp baseResp
}

struct BatchGetDatasetsReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: required list<i64> datasetIDs (api.js_conv = "str", vt.max_size = "100")
    10: optional bool withDeleted
    255: optional base.Base base
}

struct BatchGetDatasetsResp {
    1: optional list<datasetv2.Dataset> datasets
    255: optional base.BaseResp baseResp
}

struct SearchDatasetsReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: optional list<i64> datasetIDs (api.js_conv = "str")
    3: optional datasetv2.DatasetCategory category
    4: optional string name (vt.max_size = "255")             // 支持模糊搜索
    5: optional list<string> createdBys
    6: optional list<string> bizCategories
    7: optional list<string> exactNames (vt.max_size = "255") // 精确匹配名称，可以指定多个

    /* pagination */
    100: optional i32 page (vt.gt = "0")
    101: optional i32 pageSize (vt.gt = "0", vt.le = "200")                          // 分页大小(0, 200]，默认为 20
    102: optional string cursor                                                      // 与 page 同时提供时，优先使用 cursor
    103: optional datasetv2.OrderBy orderBy
    255: optional base.Base base
}

struct SearchDatasetsResp {
    1: optional list<datasetv2.Dataset> datasets

    /* pagination */
    100: optional string nextCursor
    101: optional i64 total (api.js_conv = "str")
    255: optional base.BaseResp baseResp
}

struct SignUploadFileTokenReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: optional datasetv2.StorageProvider storage (vt.not_nil = "true", vt.defined_only = "true") // 支持 ImageX, TOS
    3: optional string fileName
    10: optional string imageXServiceID                                                           // 本次需要上传到的 serviceID

    /*base*/
    255: optional base.Base base
}

struct SignUploadFileTokenResp {
    1: optional string url
    2: optional datasetv2.FileUploadToken token

    /*base*/
    255: optional base.BaseResp baseResp
}

struct ImportDatasetReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional datasetv2job.DatasetIOFile file (vt.not_nil = "true")
    4: optional list<datasetv2job.FieldMapping> fieldMappings (vt.min_size = "1")        // 待外场前端修复后再加上 vt.elem.skip = "false"
    5: optional datasetv2job.DatasetIOJobOption option

    /*base*/
    255: optional base.Base base
}

struct ImportDatasetResp {
    1: optional i64 jobID (api.js_conv = "str")
    255: optional base.BaseResp baseResp
}

struct CreateDatasetWithImportReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: optional i32 appID (api.js_conv = "str")
    3: optional datasetv2job.SourceType sourceType (vt.defined_only = "true")
    4: required datasetv2job.DatasetIOEndpoint source
    5: optional list<datasetv2job.FieldMapping> fieldMappings (vt.min_size = "1", vt.elem.skip = "false")
    6: optional datasetv2job.DatasetIOJobOption option
    21: required string targetDatasetName (vt.min_size = "1")                                             // 新建数据集名称
    22: optional string targetDatasetDesc                                                                 // 新建数据集描述
    23: optional datasetv2.DatasetCategory category (vt.defined_only = "true")
    24: optional list<datasetv2.FieldSchema> fields (vt.min_size = "1", vt.elem.skip = "false")

    /*base*/
    255: optional base.Base base
}

struct CreateDatasetWithImportResp {
    1: optional i64 datasetID (api.js_conv = "str")
    2: optional i64 jobID (api.js_conv = "str")
    255: optional base.BaseResp baseResp
}

struct ExportDatasetReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional i64 versionID (api.js_conv = "str")                                      // 需要导出的数据集版本 id，为 0 表示导出草稿版本
    4: required datasetv2job.SourceType targetType (vt.defined_only = "true")
    5: required datasetv2job.DatasetIOEndpoint target                                    // 此处填写一个文件夹，会将对应的文件生成到该文件夹下
    6: optional datasetv2job.WriteMode writeMode                                         // 覆盖还是追加
    7: optional list<datasetv2job.FieldMapping> fieldMappings                            // 字段映射
    8: optional list<i64> itemIDs                                                        // 待导出的 item ID 列表，不指定时默认导出整个数据集
    9: optional datasetv2job.ExportETLStrategy etlStrategy                               // 导出时的数据清洗策略
    10: optional list<datasetv2job.InternalFieldExportConfig> internalFieldExportConfig  // 包含的系统内置信息，目前仅在导出到文件时生效
    11: optional datasetv2job.Visibility visibility (vt.defined_only = "true")           // 导出数据集的可见性，默认是用户可见

    /*base*/
    255: optional base.Base base
}

struct ExportDatasetResp {
    1: optional i64 jobID (api.js_conv = "str")

    /*base*/
    255: optional base.BaseResp baseResp
}

struct ParseImportSourceFileReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: optional datasetv2job.DatasetIOFile file (vt.not_nil = "true")                // 如果 path 为文件夹，此处只默认解析当前路径级别下所有指定类型的文件，不嵌套解析

    /*base*/
    255: optional base.Base base
}

struct ConflictField {
    1: optional string fieldName                           // 存在冲突的列名
    2: optional map<string, datasetv2.FieldSchema> detailM // 冲突详情。key: 文件名，val：该文件中包含的类型
}

struct ParseImportSourceFileResp {
    1: optional i64 bytes (api.js_conv = "str")       // 文件大小，单位为 byte
    2: optional list<datasetv2.FieldSchema> fields    // 列名和类型，有多文件的话会取并集返回。如果文件中的列定义存在冲突，此处不返回解析结果，具体冲突详情通过 conflicts 返回
    3: optional list<ConflictField> conflicts         // 冲突详情。key: 列名，val：冲突详情
    4: optional list<string> filesWithAmbiguousColumn // 存在列定义不明确的文件（即一个列被定义为多个类型），当前仅 jsonl 文件会出现该状况

    /*base*/
    255: optional base.BaseResp baseResp
}

struct CreateItemDeduplicateJobReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (vt.gt = "0")
    3: optional datasetv2job.DatasetIOFile file (vt.not_nil = "true")
    4: optional list<datasetv2job.FieldMapping> fieldMappings (vt.min_size = "1", vt.elem.skip = "false")
    5: optional datasetv2job.DatasetIOJobOption option
    6: optional i64 jobID (api.js_conv = "str")                                                           // 任务id，重入时用
    7: optional string fieldKey                                                                           // 根据哪一列去重
    8: optional datasetv2similarity.SimilarityAlgorithm similarityAlgorithm                               // 去重算法
    9: optional i64 threshold                                                                             // 阈值

    /*base*/
    255: optional base.Base base
}

struct CreateItemDeduplicateJobResp {
    1: required i64 jobID (api.js_conv = "str"), // 任务id，前端后续用这个id去获取 待确认列表
    255: optional base.BaseResp baseResp
}

struct GetItemDeduplicateJobReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 jobID (api.js_conv = "str", api.path = "jobID", vt.gt = "0")
    3: optional datasetv2job.ImportConfirmType confirmType
    100: optional i32 page (vt.gt = "0")
    101: optional i32 pageSize (vt.gt = "0", vt.le = "200")
    255: optional base.Base base
}

struct GetItemDeduplicateJobResp {
    1: optional datasetv2job.ItemDeduplicateJob job
    255: optional base.BaseResp baseResp
}

struct ConfirmItemDeduplicateReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 jobID (api.js_conv = "str", api.path = "jobID", vt.gt = "0")
    3: required list<ConfirmItemPair> pairs,                                        // 批量确认

    /*base*/
    255: optional base.Base base
}

struct ConfirmItemPair {
    1: required string newItemsUniqKey,                           // 新导入的条目主键
    2: required datasetv2job.ImportConfirmType importConfirmType,
}

struct ConfirmItemDeduplicateResp {
    255: optional base.BaseResp baseResp
}

struct GetDatasetIOJobReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 jobID (api.js_conv = "str", api.path = "jobID", vt.gt = "0")
    255: optional base.Base base
}

struct GetDatasetIOJobResp {
    1: optional datasetv2job.DatasetIOJob job
    255: optional base.BaseResp baseResp
}

struct ListDatasetIOJobsOfDatasetReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional list<datasetv2job.JobType> types
    4: optional list<datasetv2job.JobStatus> statuses
    255: optional base.Base base
}

struct ListDatasetIOJobsOfDatasetResp {
    1: optional list<datasetv2job.DatasetIOJob> jobs
    255: optional base.BaseResp baseResp
}

struct CancelDatasetIOJobReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: required i64 jobID (api.js_conv = "str", api.path = "jobID", vt.gt = "0")
    255: optional base.Base base
}

struct CancelDatasetIOJobResp {
    255: optional base.BaseResp baseResp
}

struct ListDatasetVersionsReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional string versionLike    // 根据版本号模糊匹配
    4: optional list<string> versions // 根据版本号精确匹配

    /* pagination */
    100: optional i32 page (vt.gt = "0")
    101: optional i32 pageSize (vt.gt = "0", vt.le = "200")                              // 分页大小(0, 200]，默认为 20
    102: optional string cursor                                                          // 与 page 同时提供时，优先使用 cursor
    103: optional datasetv2.OrderBy orderBy
    255: optional base.Base base
}

struct ListDatasetVersionsResp {
    1: optional list<datasetv2.DatasetVersion> versions

    /* pagination */
    100: optional string nextCursor
    101: optional i64 total (api.js_conv = "str")
    255: optional base.BaseResp baseResp
}

struct GetDatasetVersionReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 versionID (api.js_conv = "str", api.path = "versionID", vt.gt = "0")
    10: optional bool withDeleted                                                        // 是否返回已删除的数据，默认不返回
    255: optional base.Base base
}

struct GetDatasetVersionResp {
    1: optional datasetv2.DatasetVersion version
    2: optional datasetv2.Dataset dataset
    255: optional base.BaseResp baseResp
}

struct VersionedDataset {
    1: optional datasetv2.DatasetVersion version
    2: optional datasetv2.Dataset dataset
}

struct BatchGetVersionedDatasetsReq {
    1: optional i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: required list<i64> versionIDs (api.js_conv = "str", vt.max_size = "100")
    10: optional bool withDeleted                                                    // 是否返回已删除的数据，默认不返回
    255: optional base.Base base
}

struct BatchGetVersionedDatasetsResp {
    1: optional list<VersionedDataset> versionedDataset
    255: optional base.BaseResp baseResp
}

struct CreateDatasetVersionReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required string version (vt.min_size = "1", vt.max_size = "128")                  // 展示的版本号，SemVer2 三段式，需要大于上一版本
    4: optional string desc (vt.max_size = "2048")
    255: optional base.Base base
}

struct CreateDatasetVersionResp {
    1: optional i64 id (api.js_conv = "str")
    255: optional base.BaseResp baseResp
}

struct UpdateDatasetVersionReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: required i64 versionID (api.js_conv = "str", api.path = "versionID", vt.gt = "0")
    10: optional string desc (vt.max_size = "2048")
    255: optional base.Base base
}

struct UpdateDatasetVersionResp {
    255: optional base.BaseResp baseResp
}

struct UpdateDatasetSchemaReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    // fieldSchema.key 为空时：插入新的一列
    // fieldSchema.key 不为空时：更新对应的列
    // 使用示例参考：https://bytedance.larkoffice.com/wiki/BEbMwdYDQinYFckYbHVcW3DfnZx#doxcnCEi007nKCLwZ4o84nVivle
    3: optional list<datasetv2.FieldSchema> fields (vt.min_size = "1", vt.elem.skip = "false")
    255: optional base.Base base
}

struct UpdateDatasetSchemaResp {
    255: optional base.BaseResp baseResp
}

struct GetDatasetSchemaReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    10: optional bool withDeleted                                                        // 是否获取已经删除的列，默认不返回
    255: optional base.Base base
}

struct GetDatasetSchemaResp {
    1: optional list<datasetv2.FieldSchema> fields
    255: optional base.BaseResp baseResp
}

struct BatchCreateDatasetItemsReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional list<datasetv2.DatasetItem> items (vt.min_size = "1", vt.max_size = "100", vt.elem.skip = "false")
    10: optional bool skipInvalidItems                                                                             // items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
    11: optional bool allowPartialAdd                                                                              // 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条
    255: optional base.Base base
}

struct BatchCreateDatasetItemsResp {
    1: optional map<i32, i64> addedItems (api.js_conv = "str") // key: item 在 items 中的索引; Deprecated 使用 itemOutputs，信息更全面
    2: optional list<datasetv2.ItemErrorGroup> errors
    3: optional list<datasetv2.CreateDatasetItemOutput> itemOutputs
    /* base */
    255: optional base.BaseResp baseResp
}

struct ValidateDatasetItemsReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: optional list<datasetv2.DatasetItem> items (vt.min_size = "1", vt.max_size = "500", vt.elem.skip = "false")
    3: optional i64 datasetID (api.js_conv = "str")                                                                // 添加到已有数据集时提供
    4: optional datasetv2.DatasetCategory datasetCategory (vt.defined_only = "true")                               // 新建数据集并添加数据时提供
    5: optional list<datasetv2.FieldSchema> datasetFields (vt.elem.skip = "false")                                 // 新建数据集并添加数据时，必须提供；添加到已有数据集时，如非空，则覆盖已有 schema 用于校验
    10: optional bool ignoreCurrentItemCount                                                                       // 添加到已有数据集时，现有数据条数，做容量校验时不做考虑，仅考虑提供 items 数量是否超限
}

struct ValidateDatasetItemsResp {
    1: optional list<i32> validItemIndices            // 合法的 item 索引，与 ValidateCreateDatasetItemsReq.items 中的索引对应
    2: optional list<datasetv2.ItemErrorGroup> errors

    /* base */
    255: optional base.BaseResp baseResp
}

struct UpdateDatasetItemReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required i64 itemID (api.js_conv = "str", api.path = "itemID", vt.gt = "0")
    4: optional list<datasetv2.FieldData> data (vt.elem.skip = "false")                  // 单轮数据内容，当数据集为单轮时，写入此处的值
    5: optional list<datasetv2.ItemData> repeatedData (vt.elem.skip = "false")           // 多轮对话数据内容，当数据集为多轮对话时，写入此处的值
    255: optional base.Base base
}

struct UpdateDatasetItemResp {
    255: optional base.BaseResp baseResp
}

struct DeleteDatasetItemReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required i64 itemID (api.js_conv = "str", api.path = "itemID", vt.gt = "0")
    255: optional base.Base base
}

struct DeleteDatasetItemResp {
    255: optional base.BaseResp baseResp
}

struct BatchDeleteDatasetItemsReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional list<i64> itemIDs (api.js_conv = "str", vt.min_size = "1") // 要删除的 item 列表，最多 100 个，在后端接口中校验
    255: optional base.Base base
}

struct BatchDeleteDatasetItemsResp {
    255: optional base.BaseResp baseResp
}

struct ListDatasetItemsReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")

    /* pagination */
    100: optional i32 page (vt.gt = "0")
    101: optional i32 pageSize (vt.gt = "0", vt.le = "200")                              // 分页大小(0, 200]，默认为 20
    102: optional string cursor                                                          // 与 page 同时提供时，优先使用 cursor
    103: optional datasetv2.OrderBy orderBy
    200: optional filter.Filter filter
    255: optional base.Base base
}

struct ListDatasetItemsResp {
    1: optional list<datasetv2.DatasetItem> items

    /* pagination */
    100: optional string nextCursor
    101: optional i64 total (api.js_conv = "str")
    102: optional i64 filterTotal (api.js_conv = "str")
    255: optional base.BaseResp baseResp
}

struct ListDatasetItemsByVersionReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required i64 versionID (api.js_conv = "str", api.path = "versionID", vt.gt = "0")

    /* pagination */
    100: optional i32 page (vt.gt = "0")
    101: optional i32 pageSize (vt.gt = "0", vt.le = "200")                              // 分页大小(0, 200]，默认为 20
    102: optional string cursor                                                          // 与 page 同时提供时，优先使用 cursor
    103: optional datasetv2.OrderBy orderBy
    200: optional filter.Filter filter
    255: optional base.Base base
}

struct ListDatasetItemsByVersionResp {
    1: optional list<datasetv2.DatasetItem> items

    /* pagination */
    100: optional string nextCursor (api.js_conv = "str"),
    101: optional i64 total
    102: optional i64 filterTotal
    255: optional base.BaseResp baseResp
}

struct GetDatasetItemReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.gt = "0", vt.not_nil = "true")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required i64 itemID (api.js_conv = "str", api.path = "itemID", vt.gt = "0")
    255: optional base.Base base
}

struct GetDatasetItemResp {
    1: optional datasetv2.DatasetItem item
    255: optional base.BaseResp baseResp
}

struct BatchGetDatasetItemsReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.gt = "0", vt.not_nil = "true")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required list<i64> itemIDs (api.js_conv = "str", vt.max_size = "100")
    255: optional base.Base base
}

struct BatchGetDatasetItemsResp {
    1: optional list<datasetv2.DatasetItem> items
    255: optional base.BaseResp baseResp
}

struct BatchGetDatasetItemsByVersionReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.gt = "0", vt.not_nil = "true")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required i64 versionID (api.js_conv = "str", api.path = "versionID", vt.gt = "0")
    4: required list<i64> itemIDs (api.js_conv = "str", vt.max_size = "100")
    255: optional base.Base base
}

struct BatchGetDatasetItemsByVersionResp {
    1: optional list<datasetv2.DatasetItem> items
    255: optional base.BaseResp baseResp
}

struct CreateDatasetItemReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional string itemKey (vt.max_size = "255")                                                     // 数据插入的幂等 key，前端创建时可以不传
    4: optional list<datasetv2.FieldData> data (vt.elem.not_nil = "true", vt.elem.skip = "false")        // 数据内容
    5: optional list<datasetv2.ItemData> repeatedData (vt.elem.not_nil = "true", vt.elem.skip = "false") // 多轮数据内容，与 data 互斥
    10: optional bool keepLineage                                                                        // 如果有来源 item，可以通过该字段指定是否保留与克隆的源 item 的血缘关系
    11: optional i64 sourceItemID (api.js_conv = "str", vt.gt = "0")                                     // 源 item id，在 keepLineage 为 true 时必填
    12: optional i64 sourceDatasetID (api.js_conv = "str", vt.gt = "0")                                  // 源 item id，在 keepLineage 为 true 时填写，如果为 0 默认与当前 dataset 一致。
    13: optional i64 sourceDatasetVersionID (api.js_conv = "str", vt.gt = "0")                           // 源 item 版本，在 keepLineage 为 true 时填写，如果为 0 默认为源数据集的草稿版本。
    255: optional base.Base base
}

struct CreateDatasetItemResp {
    1: optional i64 itemID (api.js_conv = "str")
    2: optional datasetv2.ItemErrorGroup error
    3: optional string itemKey
    4: optional bool isNewItem                   // 是否是新的 Item。提供 itemKey 时，如果 itemKey 在数据集中已存在数据，则不算做「新 Item」，该字段为 false。
    255: optional base.BaseResp baseResp
}

struct GetDatasetItemSourceReq {
    1: required i64 spaceID (api.js_conv = "str", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required i64 id (api.js_conv = "str", api.path = "id", vt.gt = "0")               // item 的主键 id
    255: optional base.Base base
}

struct GetDatasetItemSourceResp {
    1: optional datasetv2.ItemSource source
    255: optional base.BaseResp baseResp
}

struct GetDatasetItemDeepSourcesReq {
    1: required i64 spaceID (api.js_conv = "str", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required i64 id (api.js_conv = "str", api.path = "id", vt.gt = "0")               // item 的主键 id
    255: optional base.Base base
}

struct GetDatasetItemDeepSourcesResp {
    1: optional list<datasetv2.ItemSource> deepSources // 按照从 root 到当前 item 的顺序返回
    255: optional base.BaseResp baseResp
}

struct FieldOptions {
    1: optional list<i32> i32FieldOption (agw.key = "i32")
    2: optional list<i64> i64FieldOption (api.js_conv = "str" agw.key = "i64")
    3: optional list<double> f64FieldOption (agw.key = "f64")
    4: optional list<string> stringFieldOption (agw.key = "string")
    5: optional list<ObjectFieldOption> objFieldOption (agw.key = "obj")
}

struct ObjectFieldOption {
    1: required i64 id
    2: required string displayName
}

struct BatchUpdateDatasetItemsReq {
    1: optional i64 spaceID (api.js_conv = "str", vt.not_nil = "true", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional list<datasetv2.DatasetItem> items (vt.min_size = "1", vt.max_size = "100", vt.elem.skip = "false") // 通过 item ID 或 itemKey 指定需要更新的 item
    10: optional bool skipInvalidItems // items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
    255: optional base.Base base
}

struct BatchUpdateDatasetItemsResp {
    1: optional list<datasetv2.UpdateDatasetItemOutput> itemOutputs // 更新成功的 item 信息
    2: optional list<datasetv2.ItemErrorGroup> errors

    255: optional base.BaseResp baseResp
}

struct FieldMeta {
    // 字段类型
    1: required filter.FieldType fieldType (agw.key = "field_type")
    // 当前字段支持的操作类型
    2: required list<filter.QueryType> queryTypes (agw.key = "query_types")
    3: required string displayName (agw.key = "display_name")
    // 支持的可选项
    4: optional FieldOptions fieldOptions (agw.key = "field_options")
    5: optional bool exist                                                  // 当前字段在schema中是否存在
}

struct FieldMetaInfoData {
    // 字段元信息
    1: required map<string, FieldMeta> fieldMetas (agw.key = "field_metas")
}

struct GetFieldsMetaInfoRequest {
    1: required i64 spaceID (api.path = "spaceID")
    2: required i64 datasetID (api.path = "datasetID")
    3: optional i64 versionID
    255: optional base.Base Base (api.none = "true")
}

struct GetFieldsMetaInfoResponse {
    1: required FieldMetaInfoData data (agw.key = "data")
    255: optional base.BaseResp baseResp (api.none = "true")
}

struct ClearDatasetItemRequest {
    1: optional i64 spaceID (api.js_conv = "str", vt.gt = "0", vt.not_nil = "true")
    2: required i64 datasetID (api.js_conv = "str", api.path = "dataset_id", vt.gt = "0")
    255: optional base.Base Base
}

struct ClearDatasetItemResponse {
    255: optional base.BaseResp BaseResp
}

struct GetDatasetItemFieldRequest {
    1: optional i64 spaceID (api.js_conv = "str", vt.gt = "0", vt.not_nil = "true")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: required i64 itemPK (api.js_conv = "str", api.path = "itemPK", vt.gt = "0") // item 的主键ID，即 item.ID 这一字段
    4: required string fieldName // 列名
    5: optional i64 turnID (api.js_conv = "str") // 当 item 为多轮时，必须提供
    255: optional base.Base Base
}

struct GetDatasetItemFieldResponse {
    1: optional datasetv2.FieldData field
    255: optional base.BaseResp BaseResp
}

struct DatasetItemWithSource {
    1: optional datasetv2.DatasetItem item
    2: optional datasetv2.ItemSource source
}

struct QueryFieldDistributeRequest {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: required i64 datasetID (api.js_conv = "str", api.path = "datasetID", vt.gt = "0")
    3: optional i64 datasetVersion (api.js_conv = "str"),
    4: optional list<string> fieldKeys                                                   // 按照列洞察的时候，列的字段
    255: optional base.Base base
}

// 每一个元素代表柱状图中的一个柱
struct DistributeBucket {
    1: required bool isEmpty        // 代表没打标签的对象，若为true则只需要看count，代表没被打标签的个数
    2: required string tagValueID
    3: required string tagKeyName
    4: required string tagValueName
    5: required i64 count           // 数量
}

struct QueryFieldDistributeResponse {
    1: required map<string, list<DistributeBucket>> fieldsDistributeMap
    255: optional base.BaseResp baseResp
}

struct ListDatasetImportTemplateReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: optional i64 datasetID (api.js_conv = "str", vt.gt = "0")
    3: optional datasetv2.DatasetCategory category (vt.defined_only = "true")
    255: optional base.Base base
}

struct ImportTemplate {
    1: optional datasetv2job.FileFormat format
    2: optional string url
}

struct ListDatasetImportTemplateResp {
    1: optional list<ImportTemplate> templates
    255: optional base.BaseResp baseResp
}

struct UploadAttachmentDetail {
    1: optional datasetv2.ContentType contentType
    // [20,50) 多模态信息. 根据 contentType 获取对应内容
    20: optional datasetv2.ImageField originImage   // contentType=Image，原始图片
    21: optional datasetv2.ImageField image         // contentType=Image，上传后的图片
    // 错误信息
    101: optional datasetv2.ItemErrorType errorType // notice: 只返回图片相关的错误类型
    102: optional string errMsg
}

struct BatchUploadDatasetAttachmentsReq {
    1: required i64 spaceID (api.js_conv = "str", api.path = "spaceID", vt.gt = "0")
    2: optional i64 datasetID (api.js_conv = "str", vt.gt = "0")                             // 对应的数据集 id，用于获取多模态配置
    3: optional datasetv2.ContentType contentType                                            // 上传的附件类型

    /* 待上传的附件，根据 mime 自动解析文件后缀 */
    50: optional list<datasetv2.ImageField> images (vt.min_size = "1", vt.max_size = "100"), // contentType=Image. 目前仅支持 url, 其他字段不识别
    255: optional base.Base base
}

struct BatchUploadDatasetAttachmentsResp {
    1: optional list<UploadAttachmentDetail> details // 成功上传的附件
    255: optional base.Base base
}

service DatasetServiceV2 {

    /* Dataset */

    // 新增数据集
    CreateDatasetResp CreateDataset(1: CreateDatasetReq req) (api.post = "/api/ml_flow/v2/spaces/:spaceID/datasets")
    // 修改数据集
    UpdateDatasetResp UpdateDataset(1: UpdateDatasetReq req) (api.put = "/api/ml_flow/v2/datasets/:datasetID")
    // 删除数据集
    DeleteDatasetResp DeleteDataset(1: DeleteDatasetReq req) (api.delete = "/api/ml_flow/v2/datasets/:datasetID")
    // 获取数据集列表
    SearchDatasetsResp SearchDatasets(1: SearchDatasetsReq req) (api.post = "/api/ml_flow/v2/spaces/:spaceID/datasets/search")
    // 数据集当前信息（不包括数据）
    GetDatasetResp GetDataset(1: GetDatasetReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID")
    // 批量获取数据集
    BatchGetDatasetsResp BatchGetDatasets(1: BatchGetDatasetsReq req) (api.post = "/api/ml_flow/v2/spaces/:spaceID/datasets/batch_get")
    // 将外部 url 上传到内部存储
    BatchUploadDatasetAttachmentsResp BatchUploadDatasetAttachments(1: BatchUploadDatasetAttachmentsReq req) (api.post = "/api/ml_flow/v2/spaces/:spaceID/datasets/attachements/batch_upload", api.tag = 'volc-agentkit', api.category = 'mlflow', api.top_timeout = '30000')
    // 获取数据集上传模板（目前仅返回固定内容，不根据列配置生成）
    ListDatasetImportTemplateResp ListDatasetImportTemplate(1: ListDatasetImportTemplateReq req) (api.get = "/api/ml_flow/v2/spaces/:spaceID/import_templates", api.tag = 'volc-agentkit', api.top_timeout = '3000', api.category = 'mlflow')

    /* Dataset IO Job */
    SignUploadFileTokenResp SignUploadFileToken(1: SignUploadFileTokenReq req) (api.get = "/api/ml_flow/v2/files/upload_token", api.tag = 'volc-agentkit', api.top_timeout = '3000', api.category = 'mlflow')
    // 导入数据
    ImportDatasetResp ImportDataset(1: ImportDatasetReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/import", api.tag = 'volc-agentkit', api.top_timeout = '3000', api.category = 'mlflow')
    // 从数据集导入数据
    CreateDatasetWithImportResp CreateDatasetWithImport(1: CreateDatasetWithImportReq req) (api.post = "/api/ml_flow/v2/datasets/create_with_import")
    // 任务(导入、导出、转换)详情
    GetDatasetIOJobResp GetDatasetIOJob(1: GetDatasetIOJobReq req) (api.get = "/api/ml_flow/v2/dataset_io_jobs/:jobID", api.tag = 'volc-agentkit', api.top_timeout = '3000', api.category = 'mlflow')
    // 数据集任务列表，用于获取当前数据集的导入任务
    ListDatasetIOJobsOfDatasetResp ListDatasetIOJobsOfDataset(1: ListDatasetIOJobsOfDatasetReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/io_jobs")
    // 数据集任务列表，用于获取当前数据集的导入任务(POST 方法，便于传参)
    SearchDatasetIOJobsOfDatasetResp SearchDatasetIOJobsOfDataset(1: SearchDatasetIOJobsOfDatasetReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/io_jobs/search", api.tag = 'volc-agentkit', api.top_timeout = '3000', api.category = 'mlflow')
    // 取消一个任务
    CancelDatasetIOJobResp CancelDatasetIOJob(1: CancelDatasetIOJobReq req) (api.put = "/api/ml_flow/v2/spaces/:spaceID/dataset_io_jobs/:jobID/cancel", api.tag = 'volc-agentkit', api.top_timeout = '3000', api.category = 'mlflow')
    // 导出数据
    ExportDatasetResp ExportDataset(1: ExportDatasetReq req) (api.post = "/api/ml_flow/v2/spaces/:spaceID/datasets/:datasetID/export", api.tag = 'volc-agentkit', api.top_timeout = '3000', api.category = 'mlflow')
    // 解析源文件
    ParseImportSourceFileResp ParseImportSourceFile(1: ParseImportSourceFileReq req) (api.post = "/api/ml_flow/v2/spaces/:spaceID/parse_import_source_file", api.tag = 'volc-agentkit', api.category = 'mlflow', api.top_timeout = '30000')

    /* Dataset Version */

    // 生成一个新版本
    CreateDatasetVersionResp CreateDatasetVersion(1: CreateDatasetVersionReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/versions")
    // 更新一个版本
    UpdateDatasetVersionResp UpdateDatasetVersion(1: UpdateDatasetVersionReq req) (api.put = "/api/ml_flow/v2/spaces/:spaceID/dataset_versions/:versionID")
    // 版本列表
    ListDatasetVersionsResp ListDatasetVersions(1: ListDatasetVersionsReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/versions")
    // 版本列表(POST 方法，便于传参)
    SearchDatasetVersionsResp SearchDatasetVersions(1: SearchDatasetVersionsReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/versions/search")
    // 获取指定版本的数据集详情
    GetDatasetVersionResp GetDatasetVersion(1: GetDatasetVersionReq req) (api.get = "/api/ml_flow/v2/dataset_versions/:versionID")
    // 批量获取指定版本的数据集详情
    BatchGetVersionedDatasetsResp BatchGetVersionedDatasets(1: BatchGetVersionedDatasetsReq req) (api.post = "/api/ml_flow/v2/spaces/:spaceID/versioned_datasets/batch_get")

    /* Dataset Schema */

    // 获取数据集当前的 schema
    GetDatasetSchemaResp GetDatasetSchema(1: GetDatasetSchemaReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/schema")
    // 覆盖更新 schema
    UpdateDatasetSchemaResp UpdateDatasetSchema(1: UpdateDatasetSchemaReq req) (api.put = "/api/ml_flow/v2/datasets/:datasetID/schema")

    /* Dataset Item */

    // 校验数据
    ValidateDatasetItemsResp ValidateDatasetItems(1: ValidateDatasetItemsReq req) (api.post = "/api/ml_flow/v2/dataset_items/validate")
    // 批量新增数据
    BatchCreateDatasetItemsResp BatchCreateDatasetItems(1: BatchCreateDatasetItemsReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/items/batch")
    // 更新数据
    UpdateDatasetItemResp UpdateDatasetItem(1: UpdateDatasetItemReq req) (api.put = "/api/ml_flow/v2/datasets/:datasetID/items/:itemID")
    // 删除数据
    DeleteDatasetItemResp DeleteDatasetItem(1: DeleteDatasetItemReq req) (api.delete = "/api/ml_flow/v2/datasets/:datasetID/items/:itemID")
    // 批量删除数据
    BatchDeleteDatasetItemsResp BatchDeleteDatasetItems(1: BatchDeleteDatasetItemsReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/items/batch_delete")
    // 分页查询当前数据
    ListDatasetItemsResp ListDatasetItems(1: ListDatasetItemsReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/items")
    // 分页查询当前数据(POST 方法，便于传参)
    SearchDatasetItemsResp SearchDatasetItems(1: SearchDatasetItemsReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/items/search")
    // 分页查询指定版本的数据
    ListDatasetItemsByVersionResp ListDatasetItemsByVersion(1: ListDatasetItemsByVersionReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/versions/:versionID/items")
    // 分页查询指定版本的数据(POST 方法，便于传参)
    SearchDatasetItemsByVersionResp SearchDatasetItemsByVersion(1: SearchDatasetItemsByVersionReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/versions/:versionID/items/search")
    // 获取一行数据
    GetDatasetItemResp GetDatasetItem(1: GetDatasetItemReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/items/:itemID")
    // 批量获取数据
    BatchGetDatasetItemsResp BatchGetDatasetItems(1: BatchGetDatasetItemsReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/items/batch_get")
    // 批量获取指定版本的数据
    BatchGetDatasetItemsByVersionResp BatchGetDatasetItemsByVersion(1: BatchGetDatasetItemsByVersionReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/versions/:versionID/items/batch_get")
    // 创建数据行
    CreateDatasetItemResp CreateDatasetItem(1: CreateDatasetItemReq req) (api.post = "/api/ml_flow/v2/spaces/:spaceID/datasets/:datasetID/items")
    // 获取筛选元数据
    GetFieldsMetaInfoResponse GetFieldsMetaInfo(1: GetFieldsMetaInfoRequest Req) (api.get = "/api/ml_flow/v2/spaces/:spaceID/datasets/:datasetID/fields_meta_info")
    // 清除(草稿)数据项
    ClearDatasetItemResponse ClearDatasetItem(1: ClearDatasetItemRequest req) (api.post = "/api/ml_flow/v2/datasets/:dataset_id/items/clear")
    // 获取单个 field 的数据内容, 用于获取超长文本的内容
    GetDatasetItemFieldResponse GetDatasetItemField(1: GetDatasetItemFieldRequest req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/items/:itemPK/field")
    // 批量更新 item 的数据
    BatchUpdateDatasetItemsResp BatchUpdateDatasetItems(1: BatchUpdateDatasetItemsReq req) (api.post = "/api/ml_flow/v2/datasets/:datasetID/items/batch_update")

    /* Dataset Lineage */

    // 查询 item 的来源信息
    GetDatasetItemSourceResp GetDatasetItemSource(1: GetDatasetItemSourceReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/items/:id/source")
    // 查询 item 的溯源信息
    GetDatasetItemDeepSourcesResp GetDatasetItemDeepSources(1: GetDatasetItemDeepSourcesReq req) (api.get = "/api/ml_flow/v2/datasets/:datasetID/items/:id/deep_sources")

    /* Dataset Similarity */

    // 创建判重任务
    CreateItemDeduplicateJobResp CreateItemDeduplicateJob(1: CreateItemDeduplicateJobReq req) (api.post = "/api/ml_flow/v2/deduplicate/dedup_jobs")
    // 获取判重任务
    GetItemDeduplicateJobResp GetItemDeduplicateJob(1: GetItemDeduplicateJobReq req) (api.get = "/api/ml_flow/v2/deduplicate/dedup_jobs/:jobID")
    // 确认疑似重复任务
    ConfirmItemDeduplicateResp ConfirmItemDeduplicate(1: ConfirmItemDeduplicateReq req) (api.post = "/api/ml_flow/v2/deduplicate/dedup_jobs/:jobID/confirm")
    // 列洞察分布
    QueryFieldDistributeResponse QueryFieldDistribute(1: QueryFieldDistributeRequest request) (api.get = "/api/ml_flow/v2/spaces/:spaceID/datasets/:datasetID/insight/field_distribute")
}