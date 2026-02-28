namespace go coze.loop.evaluation.openapi

include "../../../base.thrift"
include "domain_openapi/common.thrift"
include "domain_openapi/eval_set.thrift"
include "coze.loop.evaluation.spi.thrift"
include "domain_openapi/experiment.thrift"
include "domain_openapi/eval_target.thrift"
include "domain_openapi/evaluator.thrift"

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

// 更新评测集详情
struct UpdateEvaluationSetOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    3: optional string name (api.body="name", vt.min_size = "1", vt.max_size = "255"),
    4: optional string description (api.body="description", vt.max_size = "2048"),

    255: optional base.Base Base
}

struct UpdateEvaluationSetOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional UpdateEvaluationSetOpenAPIData data

    255: base.BaseResp BaseResp
}

struct UpdateEvaluationSetOpenAPIData {
}

// 删除评测集
struct DeleteEvaluationSetOApiRequest {
    1: optional i64 workspace_id (api.query="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 evaluation_set_id (api.path = "evaluation_set_id", api.js_conv="true", go.tag='json:"evaluation_set_id"'),

    255: optional base.Base Base
}

struct DeleteEvaluationSetOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional DeleteEvaluationSetOpenAPIData data

    255: base.BaseResp BaseResp
}

struct DeleteEvaluationSetOpenAPIData {
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

struct GetEvaluationItemFieldOApiRequest {
    1: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"'),
    2: optional i64 evaluation_set_id (api.path='evaluation_set_id',api.js_conv='true', go.tag='json:"evaluation_set_id"'),
    3: optional i64 version_id (api.js_conv="true", go.tag='json:"version_id"'),
    4: optional i64 item_id (api.path='item_id',api.js_conv='true', go.tag='json:"item_id"'),
    5: optional string field_name // 列名
    6: optional i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"') // 当 item 为多轮时，必须提供

    255: optional base.Base Base
}

struct GetEvaluationItemFieldOApiResponse {
    1: optional eval_set.FieldData field_data

    255: optional base.BaseResp BaseResp
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
// 评测实验相关接口
// ===============================

// 3.1 创建评测实验
struct SubmitExperimentOApiRequest {
    // 基础信息
    1: optional i64 workspace_id (api.body = 'workspace_id', api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional string name (api.body = 'name')
    3: optional string description (api.body = 'description')

    // 三元组信息
    4: optional SubmitExperimentEvalSetParam eval_set_param (api.body = 'eval_set_param')
    5: optional list<SubmitExperimentEvaluatorParam> evaluator_params (api.body = 'evaluator_params')
    6: optional SubmitExperimentEvalTargetParam eval_target_param (api.body = 'eval_target_param')

    7: optional experiment.TargetFieldMapping target_field_mapping (api.body = 'target_field_mapping')
    8: optional list<experiment.EvaluatorFieldMapping> evaluator_field_mapping (api.body = 'evaluator_field_mapping')

    // 运行信息
    20: optional i32 item_concur_num (api.body = 'item_concur_num')
    22: optional common.RuntimeParam target_runtime_param (api.body = 'target_runtime_param')

    45: optional i32 item_retry_num (api.body = 'item_retry_num')

    255: optional base.Base Base
}

struct SubmitExperimentEvalSetParam {
    1: optional i64 eval_set_id (api.js_conv="true", go.tag='json:"eval_set_id"')
    2: optional string version
}

struct SubmitExperimentEvaluatorParam {
    1: optional i64 evaluator_id (api.js_conv="true", go.tag='json:"evaluator_id"')
    2: optional string version
    3: optional evaluator.EvaluatorRunConfig run_config
}

struct SubmitExperimentEvalTargetParam {
    1: optional string source_target_id
    2: optional string source_target_version
    3: optional eval_target.EvalTargetType eval_target_type
    4: optional eval_target.CozeBotInfoType bot_info_type
    5: optional string bot_publish_version // 如果是发布版本则需要填充这个字段
    6: optional eval_target.CustomEvalTarget custom_eval_target // type=6,并且有搜索对象，搜索结果信息通过这个字段透传
    7: optional eval_target.Region region   // 有区域限制需要填充这个字段
    8: optional string env  // 有环境限制需要填充这个字段
}


struct SubmitExperimentOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional SubmitExperimentOpenAPIData data

    255: base.BaseResp BaseResp
}

struct SubmitExperimentOpenAPIData {
    1: optional experiment.Experiment experiment
}

// 3.2 获取评测实验详情
struct GetExperimentsOApiRequest {
    1: optional i64 workspace_id (api.query='workspace_id',api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i64 experiment_id (api.path='experiment_id',api.js_conv='true', go.tag='json:"experiment_id"')

    255: optional base.Base Base
}

struct GetExperimentsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetExperimentsOpenAPIDataData data

    255: base.BaseResp BaseResp
}

struct GetExperimentsOpenAPIDataData {
    1: optional experiment.Experiment experiment

    255: base.BaseResp BaseResp
}

// 3.3 获取评测实验结果
struct ListExperimentResultOApiRequest {
    1: optional i64 workspace_id (api.body = 'workspace_id', api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 experiment_id (api.path = "experiment_id", api.js_conv="true", go.tag='json:"experiment_id"')

    100: optional i32 page_num (api.body = 'page_num')
    101: optional i32 page_size (api.body = 'page_size')

    255: optional base.Base Base
}

struct ListExperimentResultOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListExperimentResultOpenAPIData data

    255: base.BaseResp BaseResp
}

struct ListExperimentResultOpenAPIData {
    1: optional list<experiment.ColumnEvalSetField> column_eval_set_fields  // 评测集列
    2: optional list<experiment.ColumnEvaluator> column_evaluators  // 评估器列
    3: optional list<experiment.ItemResult> item_results    // 评测行级结果
    4: optional list<experiment.ColumnEvalTarget> column_eval_targets

    100: optional i64 total
}

// 3.4 获取聚合结果
struct GetExperimentAggrResultOApiRequest {
    1: optional i64 workspace_id (api.body = 'workspace_id', api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 experiment_id (api.path = "experiment_id", api.js_conv="true", go.tag='json:"experiment_id"')

    255: optional base.Base Base
}

struct GetExperimentAggrResultOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetExperimentAggrResultOpenAPIData data

    255: base.BaseResp BaseResp
}

struct GetExperimentAggrResultOpenAPIData {
    1: optional list<experiment.EvaluatorAggregateResult> evaluator_results (go.tag = 'json:"evaluator_results"')
    2: optional experiment.EvalTargetAggregateResult eval_target_aggr_result
}

// ===============================
// 评估器 (Evaluator) 接口
// ===============================

// 3.1 查询评估器列表
struct ListEvaluatorsOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional string search_name (api.body="search_name")
    3: optional list<i64> creator_ids (api.body="creator_ids", api.js_conv="true", go.tag='json:"creator_ids"')
    4: optional list<evaluator.EvaluatorType> evaluator_type (api.body="evaluator_type")
    5: optional bool with_version (api.body="with_version")
    6: optional bool builtin (api.body="builtin")
    7: optional evaluator.EvaluatorFilterOption filter_option (api.body="filter_option")
    100: optional i32 page_size (api.body="page_size", vt.gt = "0", vt.le = "200")
    101: optional i32 page_number (api.body="page_number", vt.gt = "0")
    102: optional list<common.OrderBy> order_bys (api.body="order_bys")

    255: optional base.Base Base
}

struct ListEvaluatorsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListEvaluatorsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct ListEvaluatorsOpenAPIData {
    1: optional list<evaluator.Evaluator> evaluators (api.body="evaluators")
    2: optional i64 total (api.body="total", api.js_conv="true", go.tag='json:"total"')
}

// 3.2 批量查询评估器
struct BatchGetEvaluatorsOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional list<i64> evaluator_ids (api.body="evaluator_ids", api.js_conv="true", go.tag='json:"evaluator_ids"')
    3: optional bool include_deleted (api.body="include_deleted")

    255: optional base.Base Base
}

struct BatchGetEvaluatorsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchGetEvaluatorsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct BatchGetEvaluatorsOpenAPIData {
    1: optional list<evaluator.Evaluator> evaluators (api.body="evaluators")
}

// 3.3 创建评估器
struct CreateEvaluatorOApiRequest {
    1: optional evaluator.Evaluator evaluator (api.body="evaluator")
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')

    255: optional base.Base Base
}

struct CreateEvaluatorOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateEvaluatorOpenAPIData data

    255: base.BaseResp BaseResp
}

struct CreateEvaluatorOpenAPIData {
    1: optional i64 evaluator_id (api.body="evaluator_id", api.js_conv="true", go.tag='json:"evaluator_id"')
}

// 3.4 更新评估器
struct UpdateEvaluatorOApiRequest {
    1: optional i64 evaluator_id (api.path="evaluator_id", api.js_conv="true", go.tag='json:"evaluator_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional string name (api.body="name")
    4: optional string description (api.body="description")

    255: optional base.Base Base
}

struct UpdateEvaluatorOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional UpdateEvaluatorOpenAPIData data

    255: base.BaseResp BaseResp
}

struct UpdateEvaluatorOpenAPIData {
}

// 3.5 更新评估器草稿
struct UpdateEvaluatorDraftOApiRequest {
    1: optional i64 evaluator_id (api.path="evaluator_id", api.js_conv="true", go.tag='json:"evaluator_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional evaluator.EvaluatorContent evaluator_content (api.body="evaluator_content")
    4: optional evaluator.EvaluatorType evaluator_type (api.body="evaluator_type")

    255: optional base.Base Base
}

struct UpdateEvaluatorDraftOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional UpdateEvaluatorDraftOpenAPIData data

    255: base.BaseResp BaseResp
}

struct UpdateEvaluatorDraftOpenAPIData {
    1: optional evaluator.Evaluator evaluator (api.body="evaluator")
}

// 3.6 删除评估器
struct DeleteEvaluatorOApiRequest {
    1: optional i64 evaluator_id (api.path="evaluator_id", api.js_conv="true", go.tag='json:"evaluator_id"')
    2: optional i64 workspace_id (api.query="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')

    255: optional base.Base Base
}

struct DeleteEvaluatorOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional DeleteEvaluatorOpenAPIData data

    255: base.BaseResp BaseResp
}

struct DeleteEvaluatorOpenAPIData {
}

// 3.7 查询评估器版本列表
struct ListEvaluatorVersionsOApiRequest {
    1: optional i64 evaluator_id (api.path="evaluator_id", api.js_conv="true", go.tag='json:"evaluator_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional list<string> query_versions (api.body="query_versions")
    100: optional i32 page_size (api.body="page_size", vt.gt="0")
    101: optional i32 page_number (api.body="page_number")
    102: optional list<common.OrderBy> order_bys (api.body="order_bys")

    255: optional base.Base Base
}

struct ListEvaluatorVersionsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListEvaluatorVersionsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct ListEvaluatorVersionsOpenAPIData {
    1: optional list<evaluator.EvaluatorVersion> evaluator_versions (api.body="evaluator_versions")
    2: optional i64 total (api.body="total", api.js_conv="true", go.tag='json:"total"')
}

// 3.8 批量查询评估器版本
struct BatchGetEvaluatorVersionsOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional list<i64> evaluator_version_ids (api.body="evaluator_version_ids", api.js_conv="true", go.tag='json:"evaluator_version_ids"')
    3: optional bool include_deleted (api.body="include_deleted")

    255: optional base.Base Base
}

struct BatchGetEvaluatorVersionsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchGetEvaluatorVersionsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct BatchGetEvaluatorVersionsOpenAPIData {
    1: optional list<evaluator.Evaluator> evaluators (api.body="evaluators")
}

// 3.9 提交评估器版本
struct SubmitEvaluatorVersionOApiRequest {
    1: optional i64 evaluator_id (api.path="evaluator_id", api.js_conv="true", go.tag='json:"evaluator_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional string version (api.body="version")
    4: optional string description (api.body="description")

    255: optional base.Base Base
}

struct SubmitEvaluatorVersionOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional SubmitEvaluatorVersionOpenAPIData data

    255: base.BaseResp BaseResp
}

struct SubmitEvaluatorVersionOpenAPIData {
    1: optional evaluator.Evaluator evaluator (api.body="evaluator")
}

// 3.10 执行评估器
struct RunEvaluatorOApiRequest {
    1: optional i64 evaluator_version_id (api.path="evaluator_version_id", api.js_conv="true", go.tag='json:"evaluator_version_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional evaluator.EvaluatorInputData input_data (api.body="input_data")
    4: optional evaluator.EvaluatorRunConfig evaluator_run_conf (api.body="evaluator_run_conf")

    100: optional map<string, string> ext (api.body="ext")

    255: optional base.Base Base
}

struct RunEvaluatorOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional RunEvaluatorOpenAPIData data

    255: base.BaseResp BaseResp
}

struct RunEvaluatorOpenAPIData {
    1: optional evaluator.EvaluatorRecord record (api.body="record")
}

// 3.11 修正评估记录
struct CorrectEvaluatorRecordOApiRequest {
    1: optional i64 evaluator_record_id (api.path="evaluator_record_id", api.js_conv="true", go.tag='json:"evaluator_record_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional evaluator.Correction correction (api.body="correction")

    255: optional base.Base Base
}

struct CorrectEvaluatorRecordOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CorrectEvaluatorRecordOpenAPIData data

    255: base.BaseResp BaseResp
}

struct CorrectEvaluatorRecordOpenAPIData {
    1: optional evaluator.EvaluatorRecord record (api.body="record")
}

// 3.12 批量查询评估记录
struct BatchGetEvaluatorRecordsOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional list<i64> evaluator_record_ids (api.body="evaluator_record_ids", api.js_conv="true", go.tag='json:"evaluator_record_ids"')
    3: optional bool include_deleted (api.body="include_deleted")

    255: optional base.Base Base
}

struct BatchGetEvaluatorRecordsOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchGetEvaluatorRecordsOpenAPIData data

    255: base.BaseResp BaseResp
}

struct BatchGetEvaluatorRecordsOpenAPIData {
    1: optional list<evaluator.EvaluatorRecord> records (api.body="records")
}

struct ValidateEvaluatorOpenAPIData {
    1: optional bool valid (api.body="valid")
    2: optional string error_message (api.body="error_message")
    3: optional evaluator.EvaluatorOutputData evaluator_output_data (api.body="evaluator_output_data")
}

// ===============================
// 实验模板 (Experiment Template) 接口
// ===============================

// 4.1 创建实验模板
struct CreateExptTemplateOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional experiment.ExptTemplateMeta meta (api.body="meta")
    3: optional experiment.ExptTuple triple_config (api.body="triple_config")
    4: optional experiment.ExptFieldMapping field_mapping_config (api.body="field_mapping_config")
    20: optional SubmitExperimentEvalTargetParam create_eval_target_param (api.body="create_eval_target_param")
    21: optional i32 default_evaluators_concur_num (api.body="default_evaluators_concur_num")

    255: optional base.Base Base
}

struct CreateExptTemplateOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateExptTemplateOpenAPIData data

    255: base.BaseResp BaseResp
}

struct CreateExptTemplateOpenAPIData {
    1: optional experiment.ExptTemplate experiment_template (api.body="experiment_template")
}

// 4.2 批量查询实验模板
struct BatchGetExptTemplatesOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional list<i64> template_ids (api.body="template_ids", api.js_conv="true", go.tag='json:"template_ids"')

    255: optional base.Base Base
}

struct BatchGetExptTemplatesOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchGetExptTemplatesOpenAPIData data

    255: base.BaseResp BaseResp
}

struct BatchGetExptTemplatesOpenAPIData {
    1: optional list<experiment.ExptTemplate> experiment_templates (api.body="experiment_templates")
}

// 4.3 更新实验模板元信息
struct UpdateExptTemplateMetaOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 template_id (api.body="template_id", api.js_conv="true", go.tag='json:"template_id"')
    3: optional experiment.ExptTemplateMeta meta (api.body="meta")

    255: optional base.Base Base
}

struct UpdateExptTemplateMetaOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional UpdateExptTemplateMetaOpenAPIData data

    255: base.BaseResp BaseResp
}

struct UpdateExptTemplateMetaOpenAPIData {
    1: optional experiment.ExptTemplateMeta meta (api.body="meta")
}

// 4.4 更新实验模板
struct UpdateExptTemplateOApiRequest {
    1: optional i64 template_id (api.path="template_id", api.js_conv="true", go.tag='json:"template_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional experiment.ExptTemplateMeta meta (api.body="meta")
    4: optional experiment.ExptTuple triple_config (api.body="triple_config")
    5: optional experiment.ExptFieldMapping field_mapping_config (api.body="field_mapping_config")
    20: optional SubmitExperimentEvalTargetParam create_eval_target_param (api.body="create_eval_target_param")
    21: optional i32 default_evaluators_concur_num (api.body="default_evaluators_concur_num")

    255: optional base.Base Base
}

struct UpdateExptTemplateOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional UpdateExptTemplateOpenAPIData data

    255: base.BaseResp BaseResp
}

struct UpdateExptTemplateOpenAPIData {
    1: optional experiment.ExptTemplate experiment_template (api.body="experiment_template")
}

// 4.5 删除实验模板
struct DeleteExptTemplateOApiRequest {
    1: optional i64 template_id (api.path="template_id", api.js_conv="true", go.tag='json:"template_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')

    255: optional base.Base Base
}

struct DeleteExptTemplateOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional DeleteExptTemplateOpenAPIData data

    255: base.BaseResp BaseResp
}

struct DeleteExptTemplateOpenAPIData {
}

// 4.6 查询实验模板列表
struct ListExptTemplatesOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i32 page_number (api.body="page_number")
    3: optional i32 page_size (api.body="page_size")
    4: optional experiment.ExperimentTemplateFilter filter_option (api.body="filter_option")
    5: optional list<common.OrderBy> order_bys (api.body="order_bys")

    255: optional base.Base Base
}

struct ListExptTemplatesOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListExptTemplatesOpenAPIData data

    255: base.BaseResp BaseResp
}

struct ListExptTemplatesOpenAPIData {
    1: optional list<experiment.ExptTemplate> experiment_templates (api.body="experiment_templates")
    2: optional i32 total (api.body="total")
}

// 4.7 根据实验模板提交新实验
struct SubmitExptFromTemplateOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional i64 template_id (api.body="template_id", api.js_conv="true", go.tag='json:"template_id"')
    3: optional string name (api.body="name")

    255: optional base.Base Base
}

struct SubmitExptFromTemplateOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional SubmitExptFromTemplateOpenAPIData data

    255: base.BaseResp BaseResp
}

struct SubmitExptFromTemplateOpenAPIData {
    1: optional experiment.Experiment experiment (api.body="experiment")
}
struct ReportEvaluatorInvokeResultRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional i64 invoke_id (api.js_conv="true", go.tag = 'json:"invoke_id"')
    3: optional coze.loop.evaluation.spi.InvokeEvaluatorRunStatus status

    // set output if status=SUCCESS
    10: optional coze.loop.evaluation.spi.InvokeEvaluatorOutputData output

    255: optional base.Base Base
}

struct ReportEvaluatorInvokeResultResponse {
    255: base.BaseResp BaseResp
}

// ===============================
// 服务定义
// ===============================
service EvaluationOpenAPIService {
    // 评测集接口
    // 创建评测集
    CreateEvaluationSetOApiResponse CreateEvaluationSetOApi(1: CreateEvaluationSetOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluation_sets")
    // 获取评测集详情
    GetEvaluationSetOApiResponse GetEvaluationSetOApi(1: GetEvaluationSetOApiRequest req) (api.category="openapi", api.get = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id")
    // 更新评测集详情
    UpdateEvaluationSetOApiResponse UpdateEvaluationSetOApi(1: UpdateEvaluationSetOApiRequest req) (api.category="openapi", api.put = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id")
    // 删除评测集
    DeleteEvaluationSetOApiResponse DeleteEvaluationSetOApi(1: DeleteEvaluationSetOApiRequest req) (api.category="openapi", api.delete = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id")

    // 查询评测集列表
    ListEvaluationSetsOApiResponse ListEvaluationSetsOApi(1: ListEvaluationSetsOApiRequest req) (api.category="openapi", api.get = "/v1/loop/evaluation/evaluation_sets")
    // 创建评测集版本
    CreateEvaluationSetVersionOApiResponse CreateEvaluationSetVersionOApi(1: CreateEvaluationSetVersionOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/versions")
    // 获取评测集版本列表
    ListEvaluationSetVersionsOApiResponse ListEvaluationSetVersionsOApi(1: ListEvaluationSetVersionsOApiRequest req) (api.category="openapi", api.get = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/versions")
    // 批量添加评测集数据
    BatchCreateEvaluationSetItemsOApiResponse BatchCreateEvaluationSetItemsOApi(1: BatchCreateEvaluationSetItemsOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items")
    // 批量更新评测集数据
    BatchUpdateEvaluationSetItemsOApiResponse BatchUpdateEvaluationSetItemsOApi(1: BatchUpdateEvaluationSetItemsOApiRequest req) (api.category="openapi", api.put = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items")
    // 批量删除评测集数据
    BatchDeleteEvaluationSetItemsOApiResponse BatchDeleteEvaluationSetItemsOApi(1: BatchDeleteEvaluationSetItemsOApiRequest req) (api.category="openapi", api.delete = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items")
    // 查询评测集特定版本数据
    ListEvaluationSetVersionItemsOApiResponse ListEvaluationSetVersionItemsOApi(1: ListEvaluationSetVersionItemsOApiRequest req) (api.category="openapi", api.get = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items")
    // 查询评测集某个filed值，用于获取超长文本的内容
    GetEvaluationItemFieldOApiResponse GetEvaluationItemFieldOApi(1: GetEvaluationItemFieldOApiRequest req) (api.category="openapi", api.get = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/items/:item_id/field")
    // 更新评测集字段信息
    UpdateEvaluationSetSchemaOApiResponse UpdateEvaluationSetSchemaOApi(1: UpdateEvaluationSetSchemaOApiRequest req) (api.category="openapi", api.put = "/v1/loop/evaluation/evaluation_sets/:evaluation_set_id/schema"),

    // 评测目标调用结果上报接口
    ReportEvalTargetInvokeResultResponse ReportEvalTargetInvokeResult(1: ReportEvalTargetInvokeResultRequest req) (api.category="openapi", api.post = "/v1/loop/eval_targets/result")

    // 评测实验接口
    // 创建评测实验
    SubmitExperimentOApiResponse SubmitExperimentOApi(1: SubmitExperimentOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/experiments")
    // 获取评测实验
    GetExperimentsOApiResponse GetExperimentsOApi(1: GetExperimentsOApiRequest req) (api.category="openapi", api.get = '/v1/loop/evaluation/experiments/:experiment_id')
    // 查询评测实验结果
    ListExperimentResultOApiResponse ListExperimentResultOApi(1: ListExperimentResultOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/experiments/:experiment_id/results")
    // 获取聚合结果
    GetExperimentAggrResultOApiResponse GetExperimentAggrResultOApi(1: GetExperimentAggrResultOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/experiments/:experiment_id/aggr_results")

    // 评估器接口
    // 查询评估器列表
    ListEvaluatorsOApiResponse ListEvaluatorsOApi(1: ListEvaluatorsOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluators/list")
    // 批量查询评估器
    BatchGetEvaluatorsOApiResponse BatchGetEvaluatorsOApi(1: BatchGetEvaluatorsOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluators/batch_get")
    // 创建评估器
    CreateEvaluatorOApiResponse CreateEvaluatorOApi(1: CreateEvaluatorOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluators")
    // 更新评估器
    UpdateEvaluatorOApiResponse UpdateEvaluatorOApi(1: UpdateEvaluatorOApiRequest req) (api.category="openapi", api.patch = "/v1/loop/evaluation/evaluators/:evaluator_id")
    // 更新评估器草稿
    UpdateEvaluatorDraftOApiResponse UpdateEvaluatorDraftOApi(1: UpdateEvaluatorDraftOApiRequest req) (api.category="openapi", api.patch = "/v1/loop/evaluation/evaluators/:evaluator_id/update_draft")
    // 删除评估器
    DeleteEvaluatorOApiResponse DeleteEvaluatorOApi(1: DeleteEvaluatorOApiRequest req) (api.category="openapi", api.delete = "/v1/loop/evaluation/evaluators/:evaluator_id")
    // 查询评估器版本列表
    ListEvaluatorVersionsOApiResponse ListEvaluatorVersionsOApi(1: ListEvaluatorVersionsOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluators/:evaluator_id/versions/list")
    // 批量查询评估器版本
    BatchGetEvaluatorVersionsOApiResponse BatchGetEvaluatorVersionsOApi(1: BatchGetEvaluatorVersionsOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluators_versions/batch_get")
    // 提交评估器版本
    SubmitEvaluatorVersionOApiResponse SubmitEvaluatorVersionOApi(1: SubmitEvaluatorVersionOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluators/:evaluator_id/submit_version")
    // 执行评估器
    RunEvaluatorOApiResponse RunEvaluatorOApi(1: RunEvaluatorOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluators_versions/:evaluator_version_id/run")
    // 修正评估记录
    CorrectEvaluatorRecordOApiResponse CorrectEvaluatorRecordOApi(1: CorrectEvaluatorRecordOApiRequest req) (api.category="openapi", api.patch = "/v1/loop/evaluation/evaluator_records/:evaluator_record_id")
    // 批量查询评估记录
    BatchGetEvaluatorRecordsOApiResponse BatchGetEvaluatorRecordsOApi(1: BatchGetEvaluatorRecordsOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluator_records/batch_get")

    // 实验模板接口
    // 创建实验模板
    CreateExptTemplateOApiResponse CreateExptTemplateOApi(1: CreateExptTemplateOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/experiment_templates")
    // 批量查询实验模板
    BatchGetExptTemplatesOApiResponse BatchGetExptTemplatesOApi(1: BatchGetExptTemplatesOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/experiment_templates/batch_get")
    // 更新实验模板元信息
    UpdateExptTemplateMetaOApiResponse UpdateExptTemplateMetaOApi(1: UpdateExptTemplateMetaOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/experiment_templates/update_meta")
    // 更新实验模板
    UpdateExptTemplateOApiResponse UpdateExptTemplateOApi(1: UpdateExptTemplateOApiRequest req) (api.category="openapi", api.patch = "/v1/loop/evaluation/experiment_templates/:template_id")
    // 删除实验模板
    DeleteExptTemplateOApiResponse DeleteExptTemplateOApi(1: DeleteExptTemplateOApiRequest req) (api.category="openapi", api.delete = "/v1/loop/evaluation/experiment_templates/:template_id")
    // 查询实验模板列表
    ListExptTemplatesOApiResponse ListExptTemplatesOApi(1: ListExptTemplatesOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/experiment_templates/list")
    // 根据实验模板提交新实验
    SubmitExptFromTemplateOApiResponse SubmitExptFromTemplateOApi(1: SubmitExptFromTemplateOApiRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/experiment_templates/submit_expt")


    // 评估器接口
    // 评估器调用结果上报接口
    ReportEvaluatorInvokeResultResponse ReportEvaluatorInvokeResult(1: ReportEvaluatorInvokeResultRequest req) (api.category="openapi", api.post = "/v1/loop/evaluation/evaluators/result")
}
