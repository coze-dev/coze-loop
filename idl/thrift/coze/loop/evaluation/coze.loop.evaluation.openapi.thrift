namespace go coze.loop.evaluation.openapi

include "../../../base.thrift"
include "domain_openapi/common.thrift"
include "domain_openapi/eval_set.thrift"
include "domain_openapi/evaluator.thrift"
include "domain_openapi/experiment.thrift"

// ===============================
// 评测集相关接口 (9个接口)
// ===============================

// 1.1 创建评测集
struct CreateEvaluationSetOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string name (vt.min_size = "1", vt.max_size = "255")
    3: optional string description (vt.max_size = "2048")
    4: optional eval_set.EvaluationSetSchema evaluation_set_schema

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
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

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
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional string name
    3: optional list<string> creators
    4: optional list<i64> evaluation_set_ids (api.js_conv="true", go.tag='json:"evaluation_set_ids"'),

    100: optional string page_token
    101: optional i32 page_size (vt.gt = "0", vt.le = "200")
    103: optional list<common.OrderBy> order_bys,

    255: optional base.Base Base
}

struct ListEvaluationSetsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListEvaluationSetsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct ListEvaluationSetsOpenAPIData {
    1: optional list<eval_set.EvaluationSet> sets   // 列表

    100: optional bool has_more
    101: optional string next_page_token
    102: optional i64 total (api.js_conv="true", go.tag='json:"total"'),
}

// 1.4 创建评测集版本
struct CreateEvaluationSetVersionOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: optional string version (vt.min_size = "1", vt.max_size="50")
    4: optional string description (vt.max_size = "400")

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

// 1.5 批量添加评测集数据
struct BatchCreateEvaluationSetItemsOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required i64 evaluation_set_id (api.path='evaluation_set_id',api.js_conv='true', go.tag='json:"evaluation_set_id"')
    3: optional list<eval_set.EvaluationSetItem> items (vt.min_size='1',vt.max_size='100')
    4: optional bool skip_invalid_items // items 中存在非法数据时，默认所有数据写入失败；设置 skipInvalidItems=true 则会跳过无效数据，写入有效数据
    5: optional bool allow_partial_add // 批量写入 items 如果超出数据集容量限制，默认所有数据写入失败；设置 partialAdd=true 会写入不超出容量限制的前 N 条

    255: optional base.Base Base
}

struct BatchCreateEvaluationSetItemsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchCreateEvaluationSetItemsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct BatchCreateEvaluationSetItemsOpenAPIData {
    1: optional map<i64, i64> added_items (api.js_conv='true', go.tag='json:"added_items"') // key: item 在 items 中的索引，value: item_id
    2: optional list<eval_set.ItemErrorGroup> errors
}

// 1.6 批量更新评测集数据
struct BatchUpdateEvaluationSetItemsOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required i64 evaluation_set_id (api.path='evaluation_set_id', api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: optional list<eval_set.EvaluationSetItem> items (vt.min_size='1',vt.max_size='100')
    4: optional bool skip_invalid_items

    255: optional base.Base Base
}

struct BatchUpdateEvaluationSetItemsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchUpdateEvaluationSetItemsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct BatchUpdateEvaluationSetItemsOpenAPIData {
    1: optional map<i64, i64> updated_items (api.js_conv="true", go.tag='json:"updated_items"')  // key: item 在 items 中的索引，value: item_id
    2: optional list<eval_set.ItemErrorGroup> errors
}

// 1.7 批量删除评测集数据
struct BatchDeleteEvaluationSetItemsOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: optional list<i64> item_ids (api.js_conv="true", go.tag='json:"item_ids"')

    255: optional base.Base Base
}

struct BatchDeleteEvaluationSetItemsOApiResponse {
    1: optional i32 code
    2: optional string msg

    255: base.BaseResp BaseResp
}


// 1.8 清空评测集草稿数据
struct ClearEvaluationSetDraftItemsOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"')

    255: optional base.Base Base
}

struct ClearEvaluationSetDraftItemsOApiResponse {
    1: optional i32 code
    2: optional string msg

    255: base.BaseResp BaseResp
}

// 1.9 查询评测集特定版本数据
struct ListEvaluationSetVersionItemsOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"')
    3: required i64 version_id (api.js_conv="true", go.tag='json:"version_id"')

    100: optional string page_token
    101: optional i32 page_size (vt.gt = "0", vt.le = "200")
    102: optional list<common.OrderBy> order_bys,

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

// ===============================
// 评估器相关接口 (5个接口)
// ===============================

// 2.1 创建评估器
struct CreateEvaluatorOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required evaluator.Evaluator evaluator

    255: optional base.Base Base
}

struct CreateEvaluatorOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateEvaluatorOpenAPIData data

    255: base.BaseResp BaseResp
}

struct CreateEvaluatorOpenAPIData {
    1: optional string evaluator_id (api.js_conv='true')
}

// 2.2 提交评估器版本
struct SubmitEvaluatorVersionOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluator_id (api.path='evaluator_id')
    3: required string version
    4: optional string description

    255: optional base.Base Base
}

struct SubmitEvaluatorVersionOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional SubmitEvaluatorVersionOpenAPIData data

    255: base.BaseResp BaseResp
}

struct SubmitEvaluatorVersionOpenAPIData {
    1: optional evaluator.Evaluator evaluator
}

// 2.3 获取评估器版本详情
struct GetEvaluatorVersionOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluator_version_id (api.path='evaluator_version_id')
    3: optional bool include_deleted

    255: optional base.Base Base
}

struct GetEvaluatorVersionOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetEvaluatorVersionOpenAPIData data

    255: base.BaseResp BaseResp
}

struct GetEvaluatorVersionOpenAPIData {
    1: optional evaluator.Evaluator evaluator
}

// 2.4 执行评估器
struct RunEvaluatorOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluator_version_id (api.path='evaluator_version_id')
    3: required evaluator.EvaluatorInputData input_data
    4: optional map<string, string> ext

    255: optional base.Base Base
}

struct RunEvaluatorOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional RunEvaluatorOpenAPIData data

    255: base.BaseResp BaseResp
}

struct RunEvaluatorOpenAPIData {
    1: required evaluator.EvaluatorRecord record
}

// 2.5 获取评估器执行结果
struct GetEvaluatorRecordOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluator_record_id (api.path='evaluator_record_id')
    3: optional bool include_deleted

    255: optional base.Base Base
}

struct GetEvaluatorRecordOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetEvaluatorRecordOpenAPIData data

    255: base.BaseResp BaseResp
}

struct GetEvaluatorRecordOpenAPIData {
    1: required evaluator.EvaluatorRecord record
}

// ===============================
// 评测实验相关接口 (2个接口)
// ===============================

// 3.1 创建评测实验
struct CreateExperimentOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional string eval_set_version_id
    3: optional string target_version_id
    4: optional list<string> evaluator_version_ids
    5: optional string name
    6: optional string description
    7: optional experiment.TargetFieldMapping target_field_mapping
    8: optional list<experiment.EvaluatorFieldMapping> evaluator_field_mapping
    9: optional i32 item_concur_num
    10: optional i32 evaluators_concur_num

    255: optional base.Base Base
}

struct CreateExperimentOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateExperimentOpenAPIData data

    255: base.BaseResp BaseResp
}

struct CreateExperimentOpenAPIData {
    1: optional experiment.Experiment experiment
}

// 3.2 获取评测实验结果
struct GetExperimentResultOApiRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string experiment_id (api.path = "experiment_id")
    3: optional string page_token
    4: optional i32 page_size (vt.gt = "0", vt.le = "200")

    255: optional base.Base Base
}

struct GetExperimentResultOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetExperimentResultOpenAPIData data

    255: base.BaseResp BaseResp
}

struct GetExperimentResultOpenAPIData {
    1: required list<experiment.ColumnEvalSetField> column_eval_set_fields
    2: optional list<experiment.ColumnEvaluator> column_evaluators
    3: optional list<experiment.ItemResult> item_results
    4: optional bool has_more
    5: optional string next_page_token
    6: optional i64 total
}

// ===============================
// 服务定义 (16个接口总计)
// ===============================

service EvaluationOpenAPIService {
    // 评测集接口 (9个)
    // 1.1 创建评测集
    CreateEvaluationSetOApiResponse CreateEvaluationSetOApi(1: CreateEvaluationSetOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets")
    // 1.2 获取评测集详情
    GetEvaluationSetOApiResponse GetEvaluationSetOApi(1: GetEvaluationSetOApiRequest req) (api.get = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id")
    // 1.3 查询评测集列表
    ListEvaluationSetsOApiResponse ListEvaluationSetsOApi(1: ListEvaluationSetsOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/list")
    // 1.4 创建评测集版本
    CreateEvaluationSetVersionOApiResponse CreateEvaluationSetVersionOApi(1: CreateEvaluationSetVersionOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/versions")
    // 1.5 批量添加评测集数据
    BatchCreateEvaluationSetItemsOApiResponse BatchCreateEvaluationSetItemsOApi(1: BatchCreateEvaluationSetItemsOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_create")
    // 1.6 批量更新评测集数据
    BatchUpdateEvaluationSetItemsOApiResponse BatchUpdateEvaluationSetItemsOApi(1: BatchUpdateEvaluationSetItemsOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_update")
    // 1.7 批量删除评测集数据
    BatchDeleteEvaluationSetItemsOApiResponse BatchDeleteEvaluationSetItemsOApi(1: BatchDeleteEvaluationSetItemsOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_delete")
    // 1.8 清空评测集草稿数据
    ClearEvaluationSetDraftItemsOApiResponse ClearEvaluationSetDraftItemsOApi(1: ClearEvaluationSetDraftItemsOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items/clear")
    // 1.9 查询评测集特定版本数据
    ListEvaluationSetVersionItemsOApiResponse ListEvaluationSetVersionItemsOApi(1: ListEvaluationSetVersionItemsOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items/list")

    // 评估器接口 (5个)
    // 2.1 创建评估器
    CreateEvaluatorOApiResponse CreateEvaluatorOApi(1: CreateEvaluatorOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluators")
    // 2.2 提交评估器版本
    SubmitEvaluatorVersionOApiResponse SubmitEvaluatorVersionOApi(1: SubmitEvaluatorVersionOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluators/:evaluator_id/versions")
    // 2.3 获取评估器版本详情
    GetEvaluatorVersionOApiResponse GetEvaluatorVersionOApi(1: GetEvaluatorVersionOApiRequest req) (api.get = "/open-apis/evaluation/v1/evaluators/versions/:evaluator_version_id")
    // 2.4 执行评估器
    RunEvaluatorOApiResponse RunEvaluatorOApi(1: RunEvaluatorOApiRequest req) (api.post = "/open-apis/evaluation/v1/evaluators/versions/:evaluator_version_id/run")
    // 2.5 获取评估器执行结果
    GetEvaluatorRecordOApiResponse GetEvaluatorRecordOApi(1: GetEvaluatorRecordOApiRequest req) (api.get = "/open-apis/evaluation/v1/evaluator_records/:evaluator_record_id")

    // 评测实验接口 (2个)
    // 3.1 创建评测实验
    CreateExperimentOApiResponse CreateExperimentOApi(1: CreateExperimentOApiRequest req) (api.post = "/open-apis/evaluation/v1/experiments")
    // 3.2 获取评测实验结果
    GetExperimentResultOApiResponse GetExperimentResultOApi(1: GetExperimentResultOApiRequest req) (api.get = "/open-apis/evaluation/v1/experiments/:experiment_id/results")
}