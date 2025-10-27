namespace go coze.loop.evaluation.openapi

include "../../../base.thrift"
include "domain_openapi/common.thrift"
include "domain_openapi/eval_set.thrift"
include "coze.loop.evaluation.spi.thrift"

// ===============================
// 评测集相关接口 (9个接口)
// ===============================

// 1.1 创建评测集
struct CreateEvaluationSetOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional string name (api.body="name", vt.min_size = "1", vt.max_size = "255")
    3: optional string description (api.body="description", vt.max_size = "2048")
    4: optional eval_set.EvaluationSetSchema evaluation_set_schema (api.body="evaluation_set_schema")

    255: optional base.Base Base
}

struct CreateEvaluationSetOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateEvaluationSetOpenAPIData data

    255: base.BaseResp BaseResp
}

struct CreateEvaluationSetOpenAPIData {
    1: optional i64 evaluation_set_id (api.js_conv="true", go.tag='json:"evaluation_set_id"'),
}

// 1.2 获取评测集详情
struct GetEvaluationSetOApiRequest {
    1: optional i64 workspace_id (api.query="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    255: optional base.Base Base
}

struct GetEvaluationSetOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetEvaluationSetOpenAPIData data

    255: base.BaseResp BaseResp
}

struct GetEvaluationSetOpenAPIData {
    1: optional eval_set.EvaluationSet evaluation_set
}

// 1.3 查询评测集列表
struct ListEvaluationSetsOApiRequest {
    1: optional i64 workspace_id (api.query="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional string name (api.query="name")
    3: optional list<string> creators (api.query="creators")
    4: optional list<i64> evaluation_set_ids (api.query="evaluation_set_ids", api.js_conv="true", go.tag='json:"evaluation_set_ids"'),

    100: optional string page_token (api.query="page_token")
    101: optional i32 page_size (api.query="page_size", vt.gt = "0", vt.le = "200")

    255: optional base.Base Base
}

struct ListEvaluationSetsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListEvaluationSetsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct ListEvaluationSetsOpenAPIData {
    1: optional list<eval_set.EvaluationSet> sets // 列表

    100: optional bool has_more
    101: optional string next_page_token
    102: optional i64 total
}

// 1.4 创建评测集版本
struct CreateEvaluationSetVersionOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: optional string version (api.body="version", vt.min_size = "1", vt.max_size="50")
    4: optional string description (api.body="description", vt.max_size = "400")

    255: optional base.Base Base
}

struct CreateEvaluationSetVersionOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateEvaluationSetVersionOpenAPIData data

    255: base.BaseResp BaseResp
}

struct CreateEvaluationSetVersionOpenAPIData {
    1: optional i64 version_id (api.js_conv="true", go.tag='json:"version_id"')
}

struct ListEvaluationSetVersionsOApiRequest {
    1: optional i64 workspace_id (api.query="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"'),
    2: optional i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),
    3: optional string version_like (api.query="version_like") // 根据版本号模糊匹配

    100: optional i32 page_size (api.query="page_size", vt.gt = "0", vt.le = "200"),    // 分页大小 (0, 200]，默认为 20
    101: optional string page_token (api.query="page_token")

    255: optional base.Base Base
}

struct ListEvaluationSetVersionsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListEvaluationSetVersionsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct ListEvaluationSetVersionsOpenAPIData {
    1: optional list<eval_set.EvaluationSetVersion> versions,

    100: optional i64 total (api.js_conv="true", go.tag='json:"total"'),
    101: optional string next_page_token
}

// 1.5 批量添加评测集数据
struct BatchCreateEvaluationSetItemsOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path='evaluation_set_id',api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: optional list<eval_set.EvaluationSetItem> items (api.body="items", vt.min_size='1',vt.max_size='100')
    4: optional bool is_skip_invalid_items (api.body="is_skip_invalid_items")// items 中存在非法数据时，默认所有数据写入失败；设置 skipInvalidItems=true 则会跳过无效数据，写入有效数据
    5: optional bool is_allow_partial_add (api.body="is_allow_partial_add")// 批量写入 items 如果超出数据集容量限制，默认所有数据写入失败；设置 partialAdd=true 会写入不超出容量限制的前 N 条

    255: optional base.Base Base
}

struct BatchCreateEvaluationSetItemsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchCreateEvaluationSetItemsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct BatchCreateEvaluationSetItemsOpenAPIData {
    1: optional list<eval_set.DatasetItemOutput> itemOutputs
    2: optional list<eval_set.ItemErrorGroup> errors
}


// 1.6 批量更新评测集数据
struct BatchUpdateEvaluationSetItemsOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path='evaluation_set_id', api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: optional list<eval_set.EvaluationSetItem> items (api.body="items", vt.min_size='1',vt.max_size='100')
    4: optional bool is_skip_invalid_items (api.body="is_skip_invalid_items")

    255: optional base.Base Base
}

struct BatchUpdateEvaluationSetItemsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchUpdateEvaluationSetItemsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct BatchUpdateEvaluationSetItemsOpenAPIData {
    1: optional list<eval_set.DatasetItemOutput> itemOutputs
    2: optional list<eval_set.ItemErrorGroup> errors
}

// 1.7 批量删除评测集数据
struct BatchDeleteEvaluationSetItemsOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: optional list<i64> item_ids (api.body="item_ids", api.js_conv="true", go.tag='json:"item_ids"')
    4: optional bool is_delete_all (api.body="is_delete_all")

    255: optional base.Base Base
}

struct BatchDeleteEvaluationSetItemsOApiResponse {
    1: optional i32 code
    2: optional string msg

    255: base.BaseResp BaseResp
}

// 1.9 查询评测集特定版本数据
struct ListEvaluationSetVersionItemsOApiRequest {
    1: optional i64 workspace_id (api.query="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: optional i64 version_id (api.query="version_id", api.js_conv="true", go.tag='json:"version_id"')

    100: optional string page_token (api.query="page_token")
    101: optional i32 page_size (api.query="page_size", vt.gt = "0", vt.le = "200")

    255: optional base.Base Base
}

struct ListEvaluationSetVersionItemsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListEvaluationSetVersionItemsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct ListEvaluationSetVersionItemsOpenAPIData {
    1: optional list<eval_set.EvaluationSetItem> items

    100: optional bool has_more
    101: optional string next_page_token
    102: optional i64 total (api.js_conv="true", go.tag='json:"total"')
}


struct UpdateEvaluationSetSchemaOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    // fieldSchema.key 为空时：插入新的一列
    // fieldSchema.key 不为空时：更新对应的列
    // 删除（不支持恢复数据）的情况下，不需要写入入参的 field list；
    10: optional list<eval_set.FieldSchema> fields (api.body="fields"),

    255: optional base.Base Base
}

struct UpdateEvaluationSetSchemaOApiResponse {
    1: optional i32 code
    2: optional string msg

    255: base.BaseResp BaseResp
}

struct ReportEvalTargetInvokeResultRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional i64 invoke_id (api.js_conv="true", go.tag = 'json:"invoke_id"')
    3: optional coze.loop.evaluation.spi.InvokeEvalTargetStatus status
    4: optional string callee

    // set output if status=SUCCESS
    10: optional coze.loop.evaluation.spi.InvokeEvalTargetOutput output
    // set output if status=SUCCESS
    11: optional coze.loop.evaluation.spi.InvokeEvalTargetUsage usage
    // set error_message if status=FAILED
    20: optional string error_message

    255: optional base.Base Base
}

struct ReportEvalTargetInvokeResultResponse {
    255: base.BaseResp BaseResp
}

// ===============================
// 服务定义
// ===============================
service EvaluationOpenAPIService {
    // 评测集接口
    // 创建评测集
    CreateEvaluationSetOApiResponse CreateEvaluationSetOApi(1: CreateEvaluationSetOApiRequest req) (api.tag="openapi", api.post = "/v1/loop/evaluation/evaluation_sets")
    // 获取评测集详情
    GetEvaluationSetOApiResponse GetEvaluationSetOApi(1: GetEvaluationSetOApiRequest req) (api.tag="openapi", api.get = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id")
    // 查询评测集列表
    ListEvaluationSetsOApiResponse ListEvaluationSetsOApi(1: ListEvaluationSetsOApiRequest req) (api.tag="openapi", api.get = "/v1/loop/evaluation/evaluation_sets")
    // 创建评测集版本
    CreateEvaluationSetVersionOApiResponse CreateEvaluationSetVersionOApi(1: CreateEvaluationSetVersionOApiRequest req) (api.tag="openapi", api.post = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/versions")
    // 获取评测集版本列表
    ListEvaluationSetVersionsOApiResponse ListEvaluationSetVersionsOApi(1: ListEvaluationSetVersionsOApiRequest req) (api.category="evaluation_set", api.get = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/versions")
    // 批量添加评测集数据
    BatchCreateEvaluationSetItemsOApiResponse BatchCreateEvaluationSetItemsOApi(1: BatchCreateEvaluationSetItemsOApiRequest req) (api.tag="openapi", api.post = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items")
    // 批量更新评测集数据
    BatchUpdateEvaluationSetItemsOApiResponse BatchUpdateEvaluationSetItemsOApi(1: BatchUpdateEvaluationSetItemsOApiRequest req) (api.tag="openapi", api.put = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items")
    // 批量删除评测集数据
    BatchDeleteEvaluationSetItemsOApiResponse BatchDeleteEvaluationSetItemsOApi(1: BatchDeleteEvaluationSetItemsOApiRequest req) (api.tag="openapi", api.delete = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items")
    // 查询评测集特定版本数据
    ListEvaluationSetVersionItemsOApiResponse ListEvaluationSetVersionItemsOApi(1: ListEvaluationSetVersionItemsOApiRequest req) (api.tag="openapi", api.get = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items")
    // 更新评测集字段信息
    UpdateEvaluationSetSchemaOApiResponse UpdateEvaluationSetSchemaOApi(1: UpdateEvaluationSetSchemaOApiRequest req) (api.tag="openapi", api.put = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/schema"),

    // 评测目标调用结果上报接口
    ReportEvalTargetInvokeResultResponse ReportEvalTargetInvokeResult(1: ReportEvalTargetInvokeResultRequest req) (api.category="openapi", api.post = "/v1/loop/eval_targets/result")
}
