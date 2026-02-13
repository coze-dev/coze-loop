namespace go coze.loop.evaluation.eval_set

include "../../../base.thrift"
include "domain/eval_set.thrift"
include "domain/common.thrift"
include "../data/domain/dataset.thrift"
include "../data/domain/dataset_job.thrift"
include "../data/domain/filter.thrift"

struct CreateEvaluationSetRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),

    2: optional string name (vt.min_size = "1", vt.max_size = "255"),
    3: optional string description (vt.max_size = "2048"),
    4: optional eval_set.EvaluationSetSchema evaluation_set_schema,
    5: optional eval_set.BizCategory biz_category (vt.max_size = "128") // 业务分类

    200: optional common.Session session (api.none = 'true')
    255: optional base.Base Base
}

struct CreateEvaluationSetResponse {
    1: optional i64 evaluation_set_id (api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    255: base.BaseResp BaseResp
}

struct CreateEvaluationSetWithImportRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),

    2: optional string name (vt.min_size = "1", vt.max_size = "255"),
    3: optional string description (vt.max_size = "2048"),
    4: optional eval_set.EvaluationSetSchema evaluation_set_schema,
    5: optional eval_set.BizCategory biz_category (vt.max_size = "128") // 业务分类

    6: optional dataset_job.SourceType source_type (vt.defined_only = "true")
    7: required dataset_job.DatasetIOEndpoint source
    8: optional list<dataset_job.FieldMapping> fieldMappings (vt.min_size = "1", vt.elem.skip = "false")

    200: optional common.Session session (api.none = 'true')
    255: optional base.Base Base
}

struct CreateEvaluationSetWithImportResponse {
    1: optional i64 evaluation_set_id (api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    2: optional i64 job_id (api.js_conv="true", go.tag='json:"job_id"')

    255: base.BaseResp BaseResp
}

struct ParseImportSourceFileRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: optional dataset_job.DatasetIOFile file (vt.not_nil = "true")                // 如果 path 为文件夹，此处只默认解析当前路径级别下所有指定类型的文件，不嵌套解析

    255: optional base.Base base
}

struct ParseImportSourceFileResponse {
    1: optional i64 bytes (api.js_conv="true", go.tag='json:"bytes"')       // 文件大小，单位为 byte
    10: optional list<eval_set.FieldSchema> field_schemas,        // 数据集字段约束
    3: optional list<ConflictField> conflicts         // 冲突详情。key: 列名，val：冲突详情
    4: optional list<string> files_with_ambiguous_column // 存在列定义不明确的文件（即一个列被定义为多个类型），当前仅 jsonl 文件会出现该状况

    /*base*/
    255: optional base.BaseResp baseResp
}

struct ConflictField {
    1: optional string field_name                           // 存在冲突的列名
    2: optional map<string, eval_set.FieldSchema> detail_m // 冲突详情。key: 文件名，val：该文件中包含的类型
}

struct UpdateEvaluationSetRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    3: optional string name (vt.min_size = "1", vt.max_size = "255"),
    4: optional string description (vt.max_size = "2048"),

    255: optional base.Base Base
}

struct UpdateEvaluationSetResponse {

    255: base.BaseResp BaseResp
}

struct DeleteEvaluationSetRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    255: optional base.Base Base
}

struct DeleteEvaluationSetResponse {

    255: base.BaseResp BaseResp
}

struct GetEvaluationSetRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    3: optional bool deleted_at (api.query='deleted_at'),

    255: optional base.Base Base
}

struct GetEvaluationSetResponse {
    1: optional eval_set.EvaluationSet evaluation_set,

    255: base.BaseResp BaseResp
}

struct ListEvaluationSetsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),

    2: optional string name (vt.max_size = "100"), // 支持模糊搜索
    3: optional list<string> creators,
    4: optional list<i64> evaluation_set_ids (api.js_conv="true", go.tag='json:"evaluation_set_ids"'),

    100: optional i32 page_number (vt.gt = "0"),
    101: optional i32 page_size (vt.gt = "0", vt.le = "200"),    // 分页大小 (0, 200]，默认为 20
    102: optional string page_token
    103: optional list<common.OrderBy> order_bys,           // 排列顺序，默认按照 createdAt 顺序排列，目前仅支持按照 createdAt 和 UpdatedAt 排序

    255: optional base.Base Base
}

struct ListEvaluationSetsResponse {
    1: optional list<eval_set.EvaluationSet> evaluation_sets,

    100: optional i64 total (api.js_conv="true", go.tag='json:"total"'),
    101: optional string next_page_token

    255: base.BaseResp BaseResp
}

struct CreateEvaluationSetVersionRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id" , api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    3: optional string version (vt.min_size = "1", vt.max_size="50"), // 展示的版本号，SemVer2 三段式，需要大于上一版本
    4: optional string desc (vt.max_size = "400"),

    255: optional base.Base Base
}

struct CreateEvaluationSetVersionResponse {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"'),

    255: base.BaseResp BaseResp
}

struct GetEvaluationSetVersionRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 version_id (api.path = "version_id", api.js_conv="true", go.tag='json:"version_id"'),
    3: optional i64 evaluation_set_id (api.path='evaluation_set_id', api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    4: optional bool deleted_at (api.query='deleted_at'),

    255: optional base.Base Base
}

struct GetEvaluationSetVersionResponse {
    1: optional eval_set.EvaluationSetVersion version,
    2: optional eval_set.EvaluationSet evaluation_set,

    255: base.BaseResp BaseResp
}

struct BatchGetEvaluationSetVersionsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required list<i64> version_ids (vt.max_size = "100", api.js_conv="true", go.tag='json:"version_ids"'),
    3: optional bool deleted_at,


    255: optional base.Base Base
}

struct BatchGetEvaluationSetVersionsResponse {
    1: optional list<VersionedEvaluationSet> versioned_evaluation_sets,

    255: base.BaseResp BaseResp
}

struct VersionedEvaluationSet {
    1: optional eval_set.EvaluationSetVersion version,
    2: optional eval_set.EvaluationSet evaluation_set,
}

struct ListEvaluationSetVersionsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    3: optional string version_like// 根据版本号模糊匹配

    100: optional i32 page_number (vt.gt = "0"),
    101: optional i32 page_size (vt.gt = "0", vt.le = "200"),    // 分页大小 (0, 200]，默认为 20
    102: optional string page_token

    255: optional base.Base Base
}

struct ListEvaluationSetVersionsResponse {
    1: optional list<eval_set.EvaluationSetVersion> versions,

    100: optional i64 total (api.js_conv="true", go.tag='json:"total"'),
    101: optional string next_page_token
    255: base.BaseResp BaseResp
}

struct UpdateEvaluationSetSchemaRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    // fieldSchema.key 为空时：插入新的一列
    // fieldSchema.key 不为空时：更新对应的列
    // 硬删除（不支持恢复数据）的情况下，不需要写入入参的 field list；
    // 软删（支持恢复数据）的情况下，入参的 field list 中仍需保留该字段，并且需要把该字段的 deleted 置为 true
    10: optional list<eval_set.FieldSchema> fields,

    255: optional base.Base Base
}

struct UpdateEvaluationSetSchemaResponse {

    255: base.BaseResp BaseResp
}

struct BatchCreateEvaluationSetItemsRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path='evaluation_set_id',api.js_conv='true', go.tag='json:"evaluation_set_id"'),
    3: optional list<eval_set.EvaluationSetItem> items (vt.min_size='1',vt.max_size='100'),

    10: optional bool skip_invalid_items, // items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据                                                    // items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
    11: optional bool allow_partial_add  // 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条

    255: optional base.Base Base
}

struct BatchCreateEvaluationSetItemsResponse {
    1: optional map<i64, i64> added_items (api.js_conv='true', go.tag='json:"added_items"') // key: item 在 items 中的索引
    2: optional list<dataset.ItemErrorGroup> errors

    3: optional list<dataset.CreateDatasetItemOutput> item_outputs

    255: base.BaseResp BaseResp
}

struct UpdateEvaluationSetItemRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path='evaluation_set_id',api.js_conv='true', go.tag='json:"evaluation_set_id"'),
    3: required i64 item_id (api.path='item_id',api.js_conv='true', go.tag='json:"item_id"'),
    5: optional list<eval_set.Turn> turns,  // 每轮对话

    255: optional base.Base Base
}

struct UpdateEvaluationSetItemResponse {

    255: base.BaseResp BaseResp
}

struct DeleteEvaluationSetItemRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    3: required i64 item_id (api.path = "item_id", api.js_conv="true", go.tag='json:"item_id"'),

    255: optional base.Base Base
}

struct DeleteEvaluationSetItemResponse {
    255: base.BaseResp BaseResp
}

struct BatchDeleteEvaluationSetItemsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    3: optional list<i64> item_ids (api.js_conv="true", go.tag='json:"item_ids"'),

    255: optional base.Base Base
}

struct BatchDeleteEvaluationSetItemsResponse {
    255: base.BaseResp BaseResp
}

struct ListEvaluationSetItemsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    3: optional i64 version_id (api.js_conv="true", go.tag='json:"version_id"'),

    100: optional i32 page_number,
    101: optional i32 page_size,    // 分页大小 (0, 200]，默认为 20
    102: optional string page_token
    103: optional list<common.OrderBy> order_bys, // 排列顺序，默认按照 updated_at 顺序排列，目前仅支持按照一个字段排序，该字段必须是 field key 或 item 元信息中的 created_at 或 updated_at

    200: optional list<i64> item_id_not_in (api.js_conv="true", go.tag='json:"item_id_not_in"')
    201: optional filter.Filter filter // item 过滤条件

    255: optional base.Base Base
}

struct ListEvaluationSetItemsResponse {
    1: optional list<eval_set.EvaluationSetItem> items,

    100: optional i64 total (api.js_conv="true", go.tag='json:"total"'),
    101: optional string next_page_token
    102: optional i64 filterTotal (agw.js_conv = "str", go.tag='json:"filter_total"')

    255: base.BaseResp BaseResp
}

struct GetEvaluationSetItemRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    3: required i64 item_id (api.path = "item_id", api.js_conv="true", go.tag='json:"item_id"'),

    255: optional base.Base Base
}

struct GetEvaluationSetItemResponse {
    1: optional eval_set.EvaluationSetItem item,

    255: base.BaseResp BaseResp
}


struct BatchGetEvaluationSetItemsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    3: optional i64 version_id (api.js_conv="true", go.tag='json:"version_id"'),
    4: optional list<i64> item_ids (api.js_conv = 'true', go.tag='json:"item_ids"'),

    255: optional base.Base Base
}

struct BatchGetEvaluationSetItemsResponse {
    1: optional list<eval_set.EvaluationSetItem> items,

    255: base.BaseResp BaseResp
}

struct ClearEvaluationSetDraftItemRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    255: optional base.Base Base
}

struct ClearEvaluationSetDraftItemResponse {
    255: base.BaseResp BaseResp
}

struct GetEvaluationSetItemFieldRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"'),
    2: required i64 evaluation_set_id (api.path='evaluation_set_id',api.js_conv='true', go.tag='json:"evaluation_set_id"'),
    3: required i64 item_pk (api.path='item_pk',api.js_conv='true', go.tag='json:"item_pk"'), // item 的主键ID，即 item.ID 这一字段
    5: required string field_name // 列名
    6: optional i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"') // 当 item 为多轮时，必须提供

    255: optional base.Base Base
}

struct GetEvaluationSetItemFieldResponse {
    1: optional eval_set.FieldData field_data

    255: optional base.BaseResp BaseResp
}

service EvaluationSetService {
    // 基本信息管理
    CreateEvaluationSetResponse CreateEvaluationSet(1: CreateEvaluationSetRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets", api.op_type = 'create', api.tag = 'volc-agentkit,open'
    )
    UpdateEvaluationSetResponse UpdateEvaluationSet(1: UpdateEvaluationSetRequest req) (
        api.category="evaluation_set", api.patch = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id", api.op_type = 'update', api.tag = 'volc-agentkit,open'
    )
    DeleteEvaluationSetResponse DeleteEvaluationSet(1: DeleteEvaluationSetRequest req) (
        api.category="evaluation_set", api.delete = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id", api.op_type = 'delete', api.tag = 'volc-agentkit,open'
    )
    GetEvaluationSetResponse GetEvaluationSet(1: GetEvaluationSetRequest req) (
        api.category="evaluation_set", api.get = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id", api.op_type = 'query', api.tag = 'volc-agentkit,open'
    )
    ListEvaluationSetsResponse ListEvaluationSets(1: ListEvaluationSetsRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/list", api.op_type = 'list', api.tag = 'volc-agentkit,open'
    )
    CreateEvaluationSetWithImportResponse CreateEvaluationSetWithImport(1: CreateEvaluationSetWithImportRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/create_with_import", api.op_type = 'create', api.tag = 'volc-agentkit'
    )
    ParseImportSourceFileResponse ParseImportSourceFile(1: ParseImportSourceFileRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/parse_import_source_file", api.op_type = 'query', api.tag = 'volc-agentkit'
    )

    // 版本管理
    CreateEvaluationSetVersionResponse CreateEvaluationSetVersion(1: CreateEvaluationSetVersionRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions", api.op_type = 'create', api.tag = 'volc-agentkit,open'
    )
    GetEvaluationSetVersionResponse GetEvaluationSetVersion(1: GetEvaluationSetVersionRequest req) (
        api.category="evaluation_set", api.get = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions/:version_id", api.op_type = 'query', api.tag = 'volc-agentkit'
    )
    ListEvaluationSetVersionsResponse ListEvaluationSetVersions(1: ListEvaluationSetVersionsRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions/list", api.op_type = 'list', api.tag = 'volc-agentkit,open'
    )
    BatchGetEvaluationSetVersionsResponse BatchGetEvaluationSetVersions(1: BatchGetEvaluationSetVersionsRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_set_versions/batch_get", api.op_type = 'query', api.tag = 'volc-agentkit'
    )

    // 字段管理
    UpdateEvaluationSetSchemaResponse UpdateEvaluationSetSchema(1: UpdateEvaluationSetSchemaRequest req) (
        api.category="evaluation_set", api.put = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/schema", api.op_type = 'update', api.tag = 'volc-agentkit,open'
    )

    // 数据管理
    BatchCreateEvaluationSetItemsResponse BatchCreateEvaluationSetItems(1: BatchCreateEvaluationSetItemsRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_create", api.op_type = 'query', api.tag = 'volc-agentkit,open'
    )
    UpdateEvaluationSetItemResponse UpdateEvaluationSetItem(1: UpdateEvaluationSetItemRequest req) (
        api.category="evaluation_set", api.put = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/:item_id", api.op_type = 'update', api.tag = 'volc-agentkit,open'
    )
    BatchDeleteEvaluationSetItemsResponse BatchDeleteEvaluationSetItems(1: BatchDeleteEvaluationSetItemsRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_delete", api.op_type = 'delete', api.tag = 'volc-agentkit,open'
    )
    ListEvaluationSetItemsResponse ListEvaluationSetItems(1: ListEvaluationSetItemsRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/list", api.op_type = 'list', api.tag = 'volc-agentkit,open'
    )
    BatchGetEvaluationSetItemsResponse BatchGetEvaluationSetItems(1: BatchGetEvaluationSetItemsRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_get", api.op_type = 'query', api.tag = 'volc-agentkit'
    )
    ClearEvaluationSetDraftItemResponse ClearEvaluationSetDraftItem(1: ClearEvaluationSetDraftItemRequest req) (
        api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/clear", api.op_type = 'update', api.tag = 'volc-agentkit'
    )
    GetEvaluationSetItemFieldResponse GetEvaluationSetItemField(1: GetEvaluationSetItemFieldRequest req) (
        api.category="evaluation_set", api.get = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/:item_pk/field", api.op_type = 'query', api.tag = 'volc-agentkit,open'
    )
}

