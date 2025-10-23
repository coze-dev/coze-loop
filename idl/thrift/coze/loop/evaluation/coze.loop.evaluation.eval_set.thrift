namespace go coze.loop.evaluation.eval_set

include "../../../base.thrift"
include "domain/eval_set.thrift"
include "domain/common.thrift"
include "../data/domain/dataset.thrift"

struct CreateEvaluationSetRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"'),

    2: optional string name (vt.not_nil="true",vt.min_size = "1", vt.max_size = "255"),
    3: optional string description (vt.max_size = "2048",vt.not_nil="true"),
    4: optional eval_set.EvaluationSetSchema evaluation_set_schema (vt.not_nil="true"),
    5: optional eval_set.BizCategory biz_category (vt.max_size = "128") // 业务分类

    200: optional common.Session session (api.none = 'true')
    255: optional base.Base Base
}

struct CreateEvaluationSetResponse {
    1: optional i64 evaluation_set_id (api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    255: base.BaseResp BaseResp
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
    103: optional list<common.OrderBy> order_bys,

    200: optional list<i64> item_id_not_in (api.js_conv="true", go.tag='json:"item_id_not_in"')

    255: optional base.Base Base
}

struct ListEvaluationSetItemsResponse {
    1: optional list<eval_set.EvaluationSetItem> items,

    100: optional i64 total (api.js_conv="true", go.tag='json:"total"'),
    101: optional string next_page_token

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

service EvaluationSetService {
    // 基本信息管理
    CreateEvaluationSetResponse CreateEvaluationSet(1: CreateEvaluationSetRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets")
    UpdateEvaluationSetResponse UpdateEvaluationSet(1: UpdateEvaluationSetRequest req) (api.category="evaluation_set", api.patch = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id")
    DeleteEvaluationSetResponse DeleteEvaluationSet(1: DeleteEvaluationSetRequest req) (api.category="evaluation_set", api.delete = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id"),
    GetEvaluationSetResponse GetEvaluationSet(1: GetEvaluationSetRequest req) (api.category="evaluation_set", api.get = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id"),
    ListEvaluationSetsResponse ListEvaluationSets(1: ListEvaluationSetsRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/list"),

    // 版本管理
    CreateEvaluationSetVersionResponse CreateEvaluationSetVersion(1: CreateEvaluationSetVersionRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions"),
    GetEvaluationSetVersionResponse GetEvaluationSetVersion(1: GetEvaluationSetVersionRequest req) (api.category="evaluation_set", api.get = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions/:version_id"),
    ListEvaluationSetVersionsResponse ListEvaluationSetVersions(1: ListEvaluationSetVersionsRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions/list"),
    BatchGetEvaluationSetVersionsResponse BatchGetEvaluationSetVersions(1: BatchGetEvaluationSetVersionsRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_set_versions/batch_get"),

    // 字段管理
    UpdateEvaluationSetSchemaResponse UpdateEvaluationSetSchema(1: UpdateEvaluationSetSchemaRequest req) (api.category="evaluation_set", api.put = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/schema"),

    // 数据管理
    BatchCreateEvaluationSetItemsResponse BatchCreateEvaluationSetItems(1: BatchCreateEvaluationSetItemsRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_create")
    UpdateEvaluationSetItemResponse UpdateEvaluationSetItem(1: UpdateEvaluationSetItemRequest req) (api.category="evaluation_set", api.put = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/:item_id")
    BatchDeleteEvaluationSetItemsResponse BatchDeleteEvaluationSetItems(1: BatchDeleteEvaluationSetItemsRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_delete")
    ListEvaluationSetItemsResponse ListEvaluationSetItems(1: ListEvaluationSetItemsRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/list")
    BatchGetEvaluationSetItemsResponse BatchGetEvaluationSetItems(1: BatchGetEvaluationSetItemsRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_get")
    ClearEvaluationSetDraftItemResponse ClearEvaluationSetDraftItem(1: ClearEvaluationSetDraftItemRequest req) (api.category="evaluation_set", api.post = "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/clear")
}

