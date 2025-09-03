namespace go coze.loop.data.dataset

include "../../../base.thrift"
include "domain/dataset.thrift"
include "domain/dataset_job.thrift"

struct CreateDatasetRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt = "0")
    2: optional i32 app_id (api.js_conv="true")
    3: required string name (vt.min_size = "1", vt.max_size = "255")
    4: optional string description (vt.max_size = "2048")
    5: optional dataset.DatasetCategory category (vt.defined_only = "true")
    6: optional string biz_category (vt.max_size = "128")
    7: optional list<dataset.FieldSchema> fields (vt.min_size = "1", vt.elem.skip = "false")
    15: optional dataset.SecurityLevel security_level (vt.defined_only = "true")
    16: optional dataset.DatasetVisibility visibility (vt.defined_only = "true")
    17: optional dataset.DatasetSpec spec
    18: optional dataset.DatasetFeatures features
    255: optional base.Base Base
}

struct CreateDatasetResponse {
    1: optional i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"')
    255: base.BaseResp BaseResp
}

struct UpdateDatasetRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: optional string name (vt.max_size = "255")
    4: optional string description (vt.max_size = "2048")
    255: optional base.Base Base
}

struct UpdateDatasetResponse {
    255: base.BaseResp BaseResp
}

struct DeleteDatasetRequest {
    1: optional i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"', vt.gt = "0", vt.not_nil = "true")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    255: optional base.Base Base
}

struct DeleteDatasetResponse {
    255: base.BaseResp BaseResp
}

struct GetDatasetRequest {
    1: optional i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    10: optional bool with_deleted (api.query='with_deleted')                                                    // 数据集已删除时是否返回
    255: optional base.Base Base
}

struct GetDatasetResponse {
    1: optional dataset.Dataset dataset
        255: base.BaseResp BaseResp
}

struct BatchGetDatasetsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt = "0")
    2: required list<i64> dataset_ids (api.js_conv="true", go.tag='json:"dataset_ids"', vt.max_size = "100")
    10: optional bool with_deleted
    255: optional base.Base Base
}

struct BatchGetDatasetsResponse {
    1: optional list<dataset.Dataset> datasets
    255: base.BaseResp BaseResp
}

struct ListDatasetsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', api.path = "workspace_id", vt.gt = "0")
    2: optional list<i64> dataset_ids (api.js_conv="true", go.tag='json:"dataset_ids"')
    3: optional dataset.DatasetCategory category
    4: optional string name (vt.max_size = "255")                                    // 支持模糊搜索
    5: optional list<string> created_bys
    6: optional list<string> biz_categorys

    /* pagination */
    100: optional i32 page_number (vt.gt = "0")
    101: optional i32 page_size (vt.gt = "0", vt.le = "200")                          // 分页大小(0, 200]，默认为 20
    102: optional string page_token                                                      // 与 page 同时提供时，优先使用 cursor
    103: optional list<dataset.OrderBy> order_bys
    255: optional base.Base Base
}

struct ListDatasetsResponse {
    1: optional list<dataset.Dataset> datasets

    /* pagination */
    100: optional string next_page_token
    101: optional i64 total (api.js_conv="true", go.tag='json:"total"')
    255: base.BaseResp BaseResp
}

struct SignUploadFileTokenRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: optional dataset.StorageProvider storage (vt.not_nil = "true", vt.defined_only = "true") // 支持 ImageX, TOS
    3: optional string file_name

    /*base*/
    255: optional base.Base Base
}

struct SignUploadFileTokenResponse {
    1: optional string url
    2: optional dataset.FileUploadToken token
    3: optional string image_x_service_id

    /*base*/
    255: base.BaseResp BaseResp
}

struct ImportDatasetRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: optional dataset_job.DatasetIOFile file (vt.not_nil = "true")
    4: optional list<dataset_job.FieldMapping> field_mappings (vt.min_size = "1")
    5: optional dataset_job.DatasetIOJobOption option

    /*base*/
    255: optional base.Base Base
}

struct ImportDatasetResponse {
    1: optional i64 job_id (api.js_conv="true", go.tag='json:"job_id"')

    255: base.BaseResp BaseResp
}

struct GetDatasetIOJobRequest {
    1: optional i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 job_id (api.js_conv="true", go.tag='json:"job_id"', api.path = "job_id", vt.gt = "0")

    255: optional base.Base Base
}

struct GetDatasetIOJobResponse {
    1: optional dataset_job.DatasetIOJob job

    255: base.BaseResp BaseResp
}

struct ListDatasetIOJobsRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: optional list<dataset_job.JobType> types
    4: optional list<dataset_job.JobStatus> statuses

    255: optional base.Base Base
}

struct ListDatasetIOJobsResponse {
    1: optional list<dataset_job.DatasetIOJob> jobs

    255: base.BaseResp BaseResp
}

struct ListDatasetVersionsRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: optional string version_like // 根据版本号模糊匹配

    /* pagination */
    100: optional i32 page_number (vt.gt = "0")
    101: optional i32 page_size (vt.gt = "0", vt.le = "200")                              // 分页大小(0, 200]，默认为 20
    102: optional string page_token                                                          // 与 page 同时提供时，优先使用 cursor
    103: optional list<dataset.OrderBy> order_bys

    255: optional base.Base Base
}

struct ListDatasetVersionsResponse {
    1: optional list<dataset.DatasetVersion> versions

    /* pagination */
    100: optional string next_page_token
    101: optional i64 total (api.js_conv="true", go.tag='json:"total"')

    255: base.BaseResp BaseResp
}

struct GetDatasetVersionRequest {
    1: optional i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 version_id (api.js_conv="true", go.tag='json:"version_id"', api.path = "version_id", vt.gt = "0")
    10: optional bool with_deleted  (api.query='with_deleted')                                                      // 是否返回已删除的数据，默认不返回
    255: optional base.Base Base
}

struct GetDatasetVersionResponse {
    1: optional dataset.DatasetVersion version
    2: optional dataset.Dataset dataset

    255: base.BaseResp BaseResp
}

struct VersionedDataset {
    1: optional dataset.DatasetVersion version
    2: optional dataset.Dataset dataset
}

struct BatchGetDatasetVersionsRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', api.path = "workspace_id", vt.gt = "0")
    2: required list<i64> version_ids (api.js_conv="true", go.tag='json:"version_ids"', vt.max_size = "100")
    10: optional bool with_deleted                                                    // 是否返回已删除的数据，默认不返回

    255: optional base.Base Base
}

struct BatchGetDatasetVersionsResponse {
    1: optional list<VersionedDataset> versioned_dataset

    255: base.BaseResp BaseResp
}

struct CreateDatasetVersionRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: required string version (vt.min_size = "1", vt.max_size = "128")                  // 展示的版本号，SemVer2 三段式，需要大于上一版本
    4: optional string desc (vt.max_size = "2048")

    255: optional base.Base Base
}

struct CreateDatasetVersionResponse {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')

    255: base.BaseResp BaseResp
}

struct UpdateDatasetSchemaRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    // fieldSchema.key 为空时：插入新的一列
    // fieldSchema.key 不为空时：更新对应的列
    3: optional list<dataset.FieldSchema> fields (vt.min_size = "1", vt.elem.skip = "false")

    255: optional base.Base Base
}

struct UpdateDatasetSchemaResponse {
    255: base.BaseResp BaseResp
}

struct GetDatasetSchemaRequest {
    1: optional i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    10: optional bool with_deleted (api.query='with_deleted')                                                       // 是否获取已经删除的列，默认不返回

    255: optional base.Base Base
}

struct GetDatasetSchemaResponse {
    1: optional list<dataset.FieldSchema> fields

    255: base.BaseResp BaseResp
}

struct ValidateDatasetItemsReq {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: optional list<dataset.DatasetItem> items (vt.min_size = "1", vt.max_size = "500", vt.elem.skip = "false")
    3: optional i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0") // 添加到已有数据集时提供
    4: optional dataset.DatasetCategory dataset_category (vt.defined_only = "true")                               // 新建数据集并添加数据时提供
    5: optional list<dataset.FieldSchema> dataset_fields (vt.elem.skip = "false")                                 // 新建数据集并添加数据时，必须提供；添加到已有数据集时，如非空，则覆盖已有 schema 用于校验
    10: optional bool ignore_current_item_count                                                                     // 添加到已有数据集时，现有数据条数，做容量校验时不做考虑，仅考虑提供 items 数量是否超限
    
    255: optional base.Base Base
}

struct ValidateDatasetItemsResp {
    1: optional list<i32> valid_item_indices          // 合法的 item 索引，与 ValidateCreateDatasetItemsReq.items 中的索引对应
    2: optional list<dataset.ItemErrorGroup> errors
    255: optional base.BaseResp baseResp
}

struct BatchCreateDatasetItemsRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: optional list<dataset.DatasetItem> items (vt.min_size = "1", vt.max_size = "100", vt.elem.skip = "false")
    10: optional bool skip_invalid_items                                                     // items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
    11: optional bool allow_partial_add                                                      // 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条

    255: optional base.Base Base
}

struct BatchCreateDatasetItemsResponse {
    1: optional map<i64, i64> added_items (api.js_conv="true", go.tag='json:"added_items"') // key: item 在 items 中的索引
    2: optional list<dataset.ItemErrorGroup> errors

    /* base */
    255: base.BaseResp BaseResp
}

struct UpdateDatasetItemRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: required i64 item_id (api.js_conv="true", go.tag='json:"item_id"', api.path = "item_id", vt.gt = "0")
    4: optional list<dataset.FieldData> data     (vt.elem.skip = "false")                                      // 单轮数据内容，当数据集为单轮时，写入此处的值
    5: optional list<dataset.ItemData> repeated_data      (vt.elem.skip = "false")                                 // 多轮对话数据内容，当数据集为多轮对话时，写入此处的值

    255: optional base.Base Base
}

struct UpdateDatasetItemResponse {
    255: base.BaseResp BaseResp
}

struct DeleteDatasetItemRequest {
    1: optional i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: required i64 item_id (api.js_conv="true", go.tag='json:"item_id"', api.path = "item_id", vt.gt = "0")

    255: optional base.Base Base
}

struct DeleteDatasetItemResponse {
    255: base.BaseResp BaseResp
}

struct BatchDeleteDatasetItemsRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: optional list<i64> item_ids (api.js_conv="true", go.tag='json:"item_ids"', vt.min_size = "1", vt.max_size = "100")

    255: optional base.Base Base
}

struct BatchDeleteDatasetItemsResponse {
    255: base.BaseResp BaseResp
}

struct ListDatasetItemsRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")

    /* pagination */
    100: optional i32 page_number (vt.gt = "0")
    101: optional i32 page_size (vt.gt = "0", vt.le = "200")                              // 分页大小(0, 200]，默认为 20
    102: optional string page_token                                                          // 与 page 同时提供时，优先使用 cursor
    103: optional list<dataset.OrderBy> order_bys

    255: optional base.Base Base
}

struct ListDatasetItemsResponse {
    1: optional list<dataset.DatasetItem> items

    /* pagination */
    100: optional string next_page_token
    101: optional i64 total (api.js_conv="true", go.tag='json:"total"')

    255: base.BaseResp BaseResp
}

struct ListDatasetItemsByVersionRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.not_nil = "true", vt.gt = "0")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: required i64 version_id (api.js_conv="true", go.tag='json:"version_id"', api.path = "version_id", vt.gt = "0")

    /* pagination */
    100: optional i32 page_number (vt.gt = "0")
    101: optional i32 page_size (vt.gt = "0", vt.le = "200")                              // 分页大小(0, 200]，默认为 20
    102: optional string page_token                                                          // 与 page 同时提供时，优先使用 cursor
    103: optional list<dataset.OrderBy> order_bys

    255: optional base.Base Base
}

struct ListDatasetItemsByVersionResponse {
    1: optional list<dataset.DatasetItem> items

    /* pagination */
    100: optional string next_page_token (api.js_conv="true", go.tag='json:"next_page_token"'),
    101: optional i64 total (api.js_conv="true", go.tag='json:"total"')

    255: base.BaseResp BaseResp
}

struct GetDatasetItemRequest {
    1: optional i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"', vt.gt = "0", vt.not_nil = "true")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: required i64 item_id (api.js_conv="true", go.tag='json:"item_id"', api.path = "item_id", vt.gt = "0")
    255: optional base.Base Base
}

struct GetDatasetItemResponse {
    1: optional dataset.DatasetItem item

    255: base.BaseResp BaseResp
}

struct BatchGetDatasetItemsRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt = "0", vt.not_nil = "true")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: required list<i64> item_ids (api.js_conv="true", go.tag='json:"item_ids"', vt.max_size = "100")
    255: optional base.Base Base
}

struct BatchGetDatasetItemsResponse {
    1: optional list<dataset.DatasetItem> items

    255: base.BaseResp BaseResp
}

struct BatchGetDatasetItemsByVersionRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt = "0", vt.not_nil = "true")
    2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")
    3: required i64 version_id (api.js_conv="true", go.tag='json:"version_id"', api.path = "version_id", vt.gt = "0")
    4: required list<i64> item_ids (api.js_conv="true", go.tag='json:"item_ids"', vt.max_size = "100")
    255: optional base.Base Base
}

struct BatchGetDatasetItemsByVersionResponse {
    1: optional list<dataset.DatasetItem> items

    255: base.BaseResp BaseResp
}

struct ClearDatasetItemRequest {
   1: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', vt.gt = "0", vt.not_nil = "true")
   2: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"', api.path = "dataset_id", vt.gt = "0")

    255: optional base.Base Base
}

struct ClearDatasetItemResponse {
    255: base.BaseResp BaseResp
}

service DatasetService {

    /* Dataset */

    // 新增数据集
    CreateDatasetResponse CreateDataset(1: CreateDatasetRequest req) (api.post = "/api/data/v1/datasets")
    // 修改数据集
    UpdateDatasetResponse UpdateDataset(1: UpdateDatasetRequest req) (api.patch = "/api/data/v1/datasets/:dataset_id")
    // 删除数据集
    DeleteDatasetResponse DeleteDataset(1: DeleteDatasetRequest req) (api.delete = "/api/data/v1/datasets/:dataset_id")
    // 获取数据集列表
    ListDatasetsResponse ListDatasets(1: ListDatasetsRequest req) (api.post = "/api/data/v1/datasets/list")
    // 数据集当前信息（不包括数据）
    GetDatasetResponse GetDataset(1: GetDatasetRequest req) (api.get = "/api/data/v1/datasets/:dataset_id")
    // 批量获取数据集
    BatchGetDatasetsResponse BatchGetDatasets(1: BatchGetDatasetsRequest req) (api.post = "/api/data/v1/datasets/batch_get")

    // 导入数据
    ImportDatasetResponse ImportDataset(1: ImportDatasetRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/import")
    // 任务(导入、导出、转换)详情
    GetDatasetIOJobResponse GetDatasetIOJob(1: GetDatasetIOJobRequest req) (api.get = "/api/data/v1/dataset_io_jobs/:job_id")
    // 数据集任务列表
    ListDatasetIOJobsResponse ListDatasetIOJobs(1: ListDatasetIOJobsRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/io_jobs")

    /* Dataset Version */

    // 生成一个新版本
    CreateDatasetVersionResponse CreateDatasetVersion(1: CreateDatasetVersionRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/versions")
    // 版本列表
    ListDatasetVersionsResponse ListDatasetVersions(1: ListDatasetVersionsRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/versions/list")
    // 获取指定版本的数据集详情
    GetDatasetVersionResponse GetDatasetVersion(1: GetDatasetVersionRequest req) (api.get = "/api/data/v1/dataset_versions/:version_id")
    // 批量获取指定版本的数据集详情
    BatchGetDatasetVersionsResponse BatchGetDatasetVersions(1: BatchGetDatasetVersionsRequest req) (api.post = "/api/data/v1/dataset_versions/batch_get")

    /* Dataset Schema */

    // 获取数据集当前的 schema
    GetDatasetSchemaResponse GetDatasetSchema(1: GetDatasetSchemaRequest req) (api.get = "/api/data/v1/datasets/:dataset_id/schema")
    // 覆盖更新 schema
    UpdateDatasetSchemaResponse UpdateDatasetSchema(1: UpdateDatasetSchemaRequest req) (api.put = "/api/data/v1/datasets/:dataset_id/schema")

    /* Dataset Item */
    // 校验数据
    ValidateDatasetItemsResp ValidateDatasetItems(1: ValidateDatasetItemsReq req) (api.post = "/api/data/v1/dataset_items/validate")
    // 批量新增数据
    BatchCreateDatasetItemsResponse BatchCreateDatasetItems(1: BatchCreateDatasetItemsRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/items/batch_create")
    // 更新数据
    UpdateDatasetItemResponse UpdateDatasetItem(1: UpdateDatasetItemRequest req) (api.put = "/api/data/v1/datasets/:dataset_id/items/:item_id")
    // 删除数据
    DeleteDatasetItemResponse DeleteDatasetItem(1: DeleteDatasetItemRequest req) (api.delete = "/api/data/v1/datasets/:dataset_id/items/:item_id")
    // 批量删除数据
    BatchDeleteDatasetItemsResponse BatchDeleteDatasetItems(1: BatchDeleteDatasetItemsRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/items/batch_delete")
    // 分页查询当前数据
    ListDatasetItemsResponse ListDatasetItems(1: ListDatasetItemsRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/items/list")
    // 分页查询指定版本的数据
    ListDatasetItemsByVersionResponse ListDatasetItemsByVersion(1: ListDatasetItemsByVersionRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/versions/:version_id/items/list")
    // 获取一行数据
    GetDatasetItemResponse GetDatasetItem(1: GetDatasetItemRequest req) (api.get = "/api/data/v1/datasets/:dataset_id/items/:item_id")
    // 批量获取数据
    BatchGetDatasetItemsResponse BatchGetDatasetItems(1: BatchGetDatasetItemsRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/items/batch_get")
    // 批量获取指定版本的数据
    BatchGetDatasetItemsByVersionResponse BatchGetDatasetItemsByVersion(1: BatchGetDatasetItemsByVersionRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/versions/:version_id/items/batch_get")
    // 清除(草稿)数据项
    ClearDatasetItemResponse ClearDatasetItem(1: ClearDatasetItemRequest req) (api.post = "/api/data/v1/datasets/:dataset_id/items/clear")
}