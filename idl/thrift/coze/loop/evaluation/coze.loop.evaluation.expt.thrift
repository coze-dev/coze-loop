namespace go coze.loop.evaluation.expt

include "../../../base.thrift"
include "../data/domain/dataset.thrift"
include "./domain/eval_set.thrift"
include "coze.loop.evaluation.eval_target.thrift"
include "./domain/common.thrift"
include "./domain/expt.thrift"
include "./domain/evaluator.thrift"

struct CreateExperimentRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i64 eval_set_version_id (api.body='eval_set_version_id', api.js_conv='true', go.tag='json:"eval_set_version_id"')
    3: optional i64 target_version_id (api.body='target_version_id', api.js_conv='true', go.tag='json:"target_version_id"')
    4: optional list<i64> evaluator_version_ids (api.body='evaluator_version_ids', api.js_conv='true', go.tag='json:"evaluator_version_ids"')
    5: optional string name (api.body='name')
    6: optional string desc (api.body='desc')
    7: optional i64 eval_set_id (api.body='eval_set_id', api.js_conv='true', go.tag='json:"eval_set_id"')
    8: optional i64 target_id (api.body='target_id', api.js_conv='true', go.tag='json:"target_id"')

    20: optional expt.TargetFieldMapping target_field_mapping (api.body = 'target_field_mapping')
    21: optional list<expt.EvaluatorFieldMapping> evaluator_field_mapping (api.body = 'evaluator_field_mapping')
    22: optional i32 item_concur_num (api.body = 'item_concur_num')
    23: optional i32 evaluators_concur_num (api.body = 'evaluators_concur_num')
    24: optional coze.loop.evaluation.eval_target.CreateEvalTargetParam create_eval_target_param (api.body = 'create_eval_target_param')
    25: optional common.RuntimeParam target_runtime_param (api.body = 'target_runtime_param')

    30: optional expt.ExptType expt_type (api.body = 'expt_type')
    31: optional i64 max_alive_time (api.body = 'max_alive_time')
    32: optional expt.SourceType source_type (api.body = 'source_type')
    33: optional string source_id (api.body = 'source_id')

    40: optional list<evaluator.EvaluatorIDVersionItem> evaluator_id_version_list (api.body = 'evaluator_id_version_list') // 补充的评估器id+version关联评估器方式，和evaluator_version_ids共同使用，兼容老逻辑

    // 是否启用评估器得分加权汇总，以及各评估器的权重配置（key 为 evaluator_version_id，value 为权重）
    41: optional bool enable_weighted_score (api.body = 'enable_weighted_score', go.tag='json:"enable_weighted_score"')
    42: optional map<i64, double> evaluator_score_weights (api.body = 'evaluator_score_weights', go.tag='json:"evaluator_score_weights"')
    43: optional i64 expt_template_id (api.body='expt_template_id',api.js_conv='true', go.tag='json:"expt_template_id"')

    200: optional common.Session session

    255: optional base.Base Base
}

struct CreateExperimentResponse {
    1: optional expt.Experiment experiment

    255: base.BaseResp BaseResp
}

struct SubmitExperimentRequest {
    1: required i64 workspace_id (api.body='workspace_id',api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i64 eval_set_version_id (api.body='eval_set_version_id',api.js_conv='true', go.tag='json:"eval_set_version_id"')
    3: optional i64 target_version_id (api.body='target_version_id',api.js_conv='true', go.tag='json:"target_version_id"')
    4: optional list<i64> evaluator_version_ids (api.body='evaluator_version_ids',api.js_conv='true', go.tag='json:"evaluator_version_ids"')
    5: optional string name (api.body='name')
    6: optional string desc (api.body='desc')
    7: optional i64 eval_set_id (api.body='eval_set_id',api.js_conv='true', go.tag='json:"eval_set_id"')
    8: optional i64 target_id (api.body='target_id',api.js_conv='true', go.tag='json:"target_id"')

    20: optional expt.TargetFieldMapping target_field_mapping (api.body = 'target_field_mapping')
    21: optional list<expt.EvaluatorFieldMapping> evaluator_field_mapping (api.body = 'evaluator_field_mapping')
    22: optional i32 item_concur_num (api.body = 'item_concur_num')
    23: optional i32 evaluators_concur_num (api.body = 'evaluators_concur_num')
    24: optional coze.loop.evaluation.eval_target.CreateEvalTargetParam create_eval_target_param (api.body = 'create_eval_target_param')
    25: optional common.RuntimeParam target_runtime_param (api.body = 'target_runtime_param')

    30: optional expt.ExptType expt_type (api.body = 'expt_type')
    31: optional i64 max_alive_time (api.body = 'max_alive_time')
    32: optional expt.SourceType source_type (api.body = 'source_type')
    33: optional string source_id (api.body = 'source_id')

    40: optional list<evaluator.EvaluatorIDVersionItem> evaluator_id_version_list (api.body = 'evaluator_id_version_list') // 补充的评估器id+version关联评估器方式，和evaluator_version_ids共同使用，兼容老逻辑
    // 是否启用评估器得分加权汇总，以及各评估器的权重配置（key 为 evaluator_version_id，value 为权重）
    41: optional bool enable_weighted_score (api.body = 'enable_weighted_score', go.tag='json:"enable_weighted_score"')
    42: optional i64 expt_template_id (api.body='expt_template_id',api.js_conv='true', go.tag='json:"expt_template_id"')

    100: optional map<string, string> ext (api.body = 'ext')

    200: optional common.Session session

    255: optional base.Base Base
}

struct SubmitExperimentResponse {
    1: optional expt.Experiment experiment (api.body = 'experiment')
    2: optional i64 run_id (api.body = 'run_id', api.js_conv = 'true', go.tag = 'json:"run_id"')

    255: base.BaseResp BaseResp
}

struct ListExperimentsRequest {
    1: required i64 workspace_id (api.body='workspace_id',api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i32 page_number (api.body='page_number')
    3: optional i32 page_size (api.body='page_size')

    20: optional expt.ExptFilterOption filter_option (api.body = 'filter_option')
    21: optional list<common.OrderBy> order_bys (api.body = 'order_bys')

    255: optional base.Base Base
}

struct ListExperimentsResponse {
    1: optional list<expt.Experiment> experiments (api.body = 'experiments')
    2: optional i32 total (api.body = 'total')

    255: base.BaseResp BaseResp
}

struct BatchGetExperimentsRequest {
    1: required i64 workspace_id (api.body='workspace_id',api.js_conv='true', go.tag='json:"workspace_id"')
    2: required list<i64> expt_ids (api.body='expt_ids',api.js_conv='true', go.tag='json:"expt_ids"')

    255: optional base.Base Base
}

struct BatchGetExperimentsResponse {
    1: optional list<expt.Experiment> experiments (api.body = 'experiments')

    255: base.BaseResp BaseResp
}

struct UpdateExperimentRequest {
    1: required i64 workspace_id (api.body='workspace_id',api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 expt_id (api.path='expt_id',api.js_conv='true', go.tag='json:"expt_id"')
    3: optional string name (api.body='name')
    4: optional string desc (api.body='desc')

    255: optional base.Base Base
}

struct UpdateExperimentResponse {
    1: optional expt.Experiment experiment (api.body = 'experiment')

    255: base.BaseResp BaseResp
}

struct DeleteExperimentRequest {
    1: required i64 workspace_id (api.body='workspace_id',api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 expt_id (api.path='expt_id',api.js_conv='true', go.tag='json:"expt_id"')

    255: optional base.Base Base
}

struct DeleteExperimentResponse {
    255: base.BaseResp BaseResp
}

struct BatchDeleteExperimentsRequest {
    1: required i64 workspace_id (api.body='workspace_id',api.js_conv='true', go.tag='json:"workspace_id"')
    2: required list<i64> expt_ids (api.body='expt_ids',api.js_conv='true', go.tag='json:"expt_ids"')

    255: optional base.Base Base
}

struct BatchDeleteExperimentsResponse {
    255: base.BaseResp BaseResp
}

struct RunExperimentRequest {
    1: optional i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: optional i64 expt_id (api.body = 'expt_id', api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: optional list<i64> item_ids (api.body = 'item_ids', api.js_conv = 'true', go.tag = 'json:"item_ids"')
    10: optional expt.ExptType expt_type (api.body = 'expt_type')

    100: optional map<string, string> ext (api.body = 'ext')

    200: optional common.Session session

    255: optional base.Base Base
}

struct RunExperimentResponse {
    1: optional i64 run_id (api.body = 'run_id', api.js_conv = 'true', go.tag = 'json:"run_id"')

    255: base.BaseResp BaseResp
}

struct RetryExperimentRequest {
    1: optional expt.ExptRetryMode retry_mode (api.body = 'retry_mode')
    2: optional i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: optional i64 expt_id (api.path = 'expt_id', api.js_conv = 'true', go.tag = 'json:"expt_id"')
    4: optional list<i64> item_ids (api.body = 'item_ids', api.js_conv = 'true', go.tag = 'json:"item_ids"')

    100: optional map<string, string> ext (api.body = 'ext')

    255: optional base.Base Base
}

struct RetryExperimentResponse {
    1: optional i64 run_id (api.body = 'run_id', api.js_conv = 'true', go.tag = 'json:"run_id"')

    255: base.BaseResp BaseResp
}

struct KillExperimentRequest {
    1: optional i64 expt_id (api.path = 'expt_id', api.js_conv = 'true', go.tag = 'json:"expt_id"')
    2: optional i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')

    255: optional base.Base Base
}

struct KillExperimentResponse {
    255: base.BaseResp BaseResp
}

struct CloneExperimentRequest {
    1: optional i64 expt_id (api.path = 'expt_id', api.js_conv = 'true', go.tag = 'json:"expt_id"')
    2: optional i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')

    255: optional base.Base Base
}

struct CloneExperimentResponse {
    1: optional expt.Experiment experiment (api.body = 'experiment')

    255: base.BaseResp BaseResp
}

struct BatchGetExperimentResultRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required list<i64> experiment_ids (api.body='experiment_ids', api.js_conv='true', go.tag='json:"experiment_ids"')
    3: optional i64 baseline_experiment_id (api.body='baseline_experiment_id', api.js_conv='true', go.tag='json:"baseline_experiment_id"')  // Baseline experiment ID for experiment comparison

    10: optional map<i64, expt.ExperimentFilter> filters (api.body = 'filters', go.tag = 'json:"filters"') // key: experiment_id

    20: optional i32 page_number (api.query="page_number", go.tag='json:"page_number"')
    21: optional i32 page_size (api.query="page_size", go.tag='json:"page_size"')

    30: optional bool use_accelerator (api.query="use_accelerator", go.tag='json:"use_accelerator"')

    40: optional bool full_trajectory (api.query="full_trajectory", go.tag='json:"full_trajectory"') // 是否包含轨迹

    255: optional base.Base Base
}

struct BatchGetExperimentResultResponse {
    // 数据集表头信息
    1: required list<expt.ColumnEvalSetField> column_eval_set_fields (api.body = "column_eval_set_fields")
    // 评估器表头信息
    2: optional list<expt.ColumnEvaluator> column_evaluators (api.body = "column_evaluators")
    3: optional list<expt.ExptColumnEvaluator> expt_column_evaluators (api.body = "expt_column_evaluators")
    // 人工标注标签表头信息
    4: optional list<expt.ExptColumnAnnotation> expt_column_annotations (api.body = "expt_column_annotations")
    5: optional list<expt.ExptColumnEvalTarget> expt_column_eval_target (api.body = "expt_column_eval_target")

    // item粒度实验结果详情
    10: optional list<expt.ItemResult> item_results (api.body = "item_results")

    20: optional i64 total (api.body = "total", go.tag = 'json:"total"')

    255: base.BaseResp BaseResp
}

struct BatchGetExperimentAggrResultRequest {
    1: required i64 workspace_id (api.query = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required list<i64> experiment_ids (api.body = 'experiment_ids', api.js_conv = 'true', go.tag = 'json:"experiment_ids"')

    255: optional base.Base Base
}

struct BatchGetExperimentAggrResultResponse {
    1: optional list<expt.ExptAggregateResult> expt_aggregate_result (api.body = 'expt_aggregate_result')

    255: base.BaseResp BaseResp
}

struct CalculateExperimentAggrResultRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true')
    2: required i64 expt_id (api.path = 'expt_id', api.js_conv = 'true')

    255: optional base.Base Base
}

struct CalculateExperimentAggrResultResponse {

    255: base.BaseResp BaseResp
}

struct CheckExperimentNameRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional string name (api.body='name')

    255: optional base.Base Base
}

struct CheckExperimentNameResponse {
    1: optional bool pass (api.body = 'pass')
    2: optional string message (api.body = 'message')

    255: base.BaseResp BaseResp
}

struct InvokeExperimentRequest {
    1: required i64 workspace_id
    2: required i64 evaluation_set_id
    3: optional list<eval_set.EvaluationSetItem> items (vt.min_size = "1", vt.max_size = "100")

    10: optional bool skip_invalid_items // items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
    11: optional bool allow_partial_add // 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条

    20: optional i64 experiment_id
    21: optional i64 experiment_run_id

    100: optional map<string, string> ext

    200: optional common.Session session

    255: optional base.Base Base
}

struct InvokeExperimentResponse {
    1: optional map<i64, i64> added_items // key: item 在 items 中的索引
    2: optional list<dataset.ItemErrorGroup> errors

    3: optional list<dataset.CreateDatasetItemOutput> item_outputs

    255: base.BaseResp BaseResp
}

struct FinishExperimentRequest {
    1: optional i64 workspace_id
    2: optional i64 experiment_id
    3: optional i64 experiment_run_id

    100: optional string cid

    200: optional common.Session session

    255: optional base.Base Base
}

struct FinishExperimentResponse {
    255: base.BaseResp BaseResp
}

struct ListExperimentStatsRequest {
    1: required i64 workspace_id
    2: optional i32 page_number
    3: optional i32 page_size

    20: optional expt.ExptFilterOption filter_option

    300: optional common.Session session

    255: optional base.Base Base
}

struct ListExperimentStatsResponse {
    1: optional list<expt.ExptStatsInfo> expt_stats_infos
    2: optional i32 total

    255: base.BaseResp BaseResp
}

// =========================
// 实验模板相关接口
// =========================

struct CreateExperimentTemplateRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')

    // 模板结构，与 ExptTemplate 保持一致
    10: optional expt.ExptTemplateMeta meta (api.body = 'meta')
    11: optional expt.ExptTuple triple_config (api.body = 'triple_config')
    12: optional expt.ExptFieldMapping field_mapping_config (api.body = 'field_mapping_config')

    // 创建评估对象参数（不在 ExptTemplate 结构中，保留在顶层）
    20: optional coze.loop.evaluation.eval_target.CreateEvalTargetParam create_eval_target_param (api.body = 'create_eval_target_param')

    // 默认评估器并发数（不在 ExptTemplate 结构中，保留在顶层）
    21: optional i32 default_evaluators_concur_num (api.body = 'default_evaluators_concur_num')
    // 调度配置（不在 ExptTemplate 结构中，保留在顶层）
    22: optional string schedule_cron (api.body = 'schedule_cron')

    30: optional expt.ExptSource expt_source (api.body = 'expt_source')

    200: optional common.Session session
    255: optional base.Base Base
}

struct CreateExperimentTemplateResponse {
    1: optional expt.ExptTemplate experiment_template

    255: base.BaseResp BaseResp
}

struct BatchGetExperimentTemplateRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required list<i64> template_ids (api.body='template_ids', api.js_conv='true', go.tag='json:"template_ids"')

    255: optional base.Base Base
}

struct BatchGetExperimentTemplateResponse {
    1: optional list<expt.ExptTemplate> experiment_templates

    255: base.BaseResp BaseResp
}

struct UpdateExperimentTemplateMetaRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 template_id (api.body='template_id', api.js_conv='true', go.tag='json:"template_id"')

    10: optional expt.ExptTemplateMeta meta (api.body = 'meta')

    255: optional base.Base Base
}

struct UpdateExperimentTemplateMetaResponse {
    1: optional expt.ExptTemplateMeta meta

    255: base.BaseResp BaseResp
}


struct UpdateExperimentTemplateRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 template_id (api.path='template_id', api.js_conv='true', go.tag='json:"template_id"')

    // 模板结构，与 ExptTemplate 保持一致
    // 注意：eval_set_id / target_id 不允许修改，仅允许调整版本与配置
    10: optional expt.ExptTemplateMeta meta (api.body = 'meta')
    11: optional expt.ExptTuple triple_config (api.body = 'triple_config')
    12: optional expt.ExptFieldMapping field_mapping_config (api.body = 'field_mapping_config')

    // 创建评估对象参数（不在 ExptTemplate 结构中，保留在顶层）
    20: optional coze.loop.evaluation.eval_target.CreateEvalTargetParam create_eval_target_param (api.body = 'create_eval_target_param')

    // 默认评估器并发数（不在 ExptTemplate 结构中，保留在顶层）
    21: optional i32 default_evaluators_concur_num (api.body = 'default_evaluators_concur_num')
    // 调度配置（不在 ExptTemplate 结构中，保留在顶层）
    22: optional string schedule_cron (api.body = 'schedule_cron')

    255: optional base.Base Base
}

struct UpdateExperimentTemplateResponse {
    1: optional expt.ExptTemplate experiment_template

    255: base.BaseResp BaseResp
}

struct DeleteExperimentTemplateRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 template_id (api.path='template_id', api.js_conv='true', go.tag='json:"template_id"')

    255: optional base.Base Base
}

struct DeleteExperimentTemplateResponse {
    255: base.BaseResp BaseResp
}

struct ListExperimentTemplatesRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i32 page_number (api.body='page_number')
    3: optional i32 page_size (api.body='page_size')

    20: optional expt.ExperimentTemplateFilter filter_option (api.body = 'filter_option')
    21: optional list<common.OrderBy> order_bys (api.body = 'order_bys')

    255: optional base.Base Base
}

struct ListExperimentTemplatesResponse {
    1: optional list<expt.ExptTemplate> experiment_templates (api.body = 'experiment_templates')
    2: optional i32 total (api.body = 'total')

    255: base.BaseResp BaseResp
}

struct CheckExperimentTemplateNameRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required string name (api.body='name')
    3: optional i64 template_id (api.body='template_id', api.js_conv='true', go.tag='json:"template_id"')

    255: optional base.Base Base
}

struct CheckExperimentTemplateNameResponse {
    1: optional bool is_available (api.body = 'is_available')

    255: base.BaseResp BaseResp
}

typedef string UpsertExptTurnResultFilterType (ts.enum="true")           // 标签状态
const UpsertExptTurnResultFilterType UpsertExptTurnResultFilterType_MANUAL = "manual"         // 启用
const UpsertExptTurnResultFilterType UpsertExptTurnResultFilterType_AUTO = "auto"     // 禁用
const UpsertExptTurnResultFilterType UpsertExptTurnResultFilterType_CHECK = "check" // 旧版本状态


struct UpsertExptTurnResultFilterRequest {
    1: optional i64 workspace_id
    2: optional i64 experiment_id
    3: optional list<i64> item_ids
    4: optional UpsertExptTurnResultFilterType filter_type
    5: optional i32 retry_times
}

struct UpsertExptTurnResultFilterResponse {
    255: base.BaseResp BaseResp
}

struct AssociateAnnotationTagReq {
     1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
     2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
     3: optional i64 tag_key_id (api.body = 'tag_key_id', api.js_conv = 'true', go.tag = 'json:"tag_key_id"')

     200: optional common.Session session
     255: optional base.Base Base
}

struct AssociateAnnotationTagResp {

    255: base.BaseResp BaseResp
}

struct DeleteAnnotationTagReq {
     1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
     2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
     3: optional i64 tag_key_id (api.body = 'tag_key_id', api.js_conv = 'true', go.tag = 'json:"tag_key_id"')

     200: optional common.Session session
     255: optional base.Base Base
}

struct DeleteAnnotationTagResp {

    255: base.BaseResp BaseResp
}

struct CreateAnnotateRecordReq {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: required expt.AnnotateRecord annotate_record (api.body = 'annotate_record')
    4: required i64 item_id (api.body = 'item_id', api.js_conv='true', go.tag='json:"item_id"')
    5: required i64 turn_id (api.body = 'turn_id', api.js_conv='true', go.tag='json:"turn_id"')
    200: optional common.Session session
    255: optional base.Base Base
}

struct CreateAnnotateRecordResp {
    1: required i64 annotate_record_id (api.body = "annotate_record_id", api.js_conv = 'true', go.tag = 'json:"annotate_record_id"')

    255: base.BaseResp BaseResp
}

struct UpdateAnnotateRecordReq {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: required expt.AnnotateRecord annotate_records (api.body = 'annotate_records')
    4: required i64 annotate_record_id (api.body = 'annotate_record_id', api.js_conv='true', go.tag='json:"annotate_record_id"')
    5: required i64 item_id (api.body = 'item_id', api.js_conv='true', go.tag='json:"item_id"')
    6: required i64 turn_id (api.body = 'turn_id', api.js_conv='true', go.tag='json:"turn_id"')

    200: optional common.Session session
    255: optional base.Base Base
}

struct UpdateAnnotateRecordResp {

    255: base.BaseResp BaseResp
}


struct ExportExptResultRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    4: optional expt.ExptResultExportType export_type (api.body = "export_type")

    200: optional common.Session session
    255: optional base.Base Base
}

struct ExportExptResultResponse {
    1: required i64 export_id (api.body = "export_id", api.js_conv = 'true', go.tag = 'json:"export_id"')

    255: base.BaseResp BaseResp
}

struct ListExptResultExportRecordRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: optional i32 page_number (api.body='page_number')
    4: optional i32 page_size (api.body='page_size')

    200: optional common.Session session
    255: optional base.Base Base
}

struct ListExptResultExportRecordResponse {
    1: required list<expt.ExptResultExportRecord> expt_result_export_records (agw.key = "expt_result_export_records")
    20: optional i64 total (api.body = "total", go.tag = 'json:"total"')
    255: base.BaseResp BaseResp
}

struct GetExptResultExportRecordRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    4: required i64 export_id (api.path = 'export_id', api.js_conv = 'true', go.tag = 'json:"export_id"')


    200: optional common.Session session
    255: optional base.Base Base
}

struct GetExptResultExportRecordResponse {
    1: optional expt.ExptResultExportRecord expt_result_export_records (api.body = "expt_result_export_records")

    255: base.BaseResp BaseResp
}


struct GetExptInsightAnalysisRecordRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: required i64 insight_analysis_record_id (api.path = 'insight_analysis_record_id', api.js_conv = 'true', go.tag = 'json:"insight_analysis_record_id"')


    200: optional common.Session session
    255: optional base.Base Base
}

struct GetExptInsightAnalysisRecordResponse {
    1: optional expt.ExptInsightAnalysisRecord expt_insight_analysis_record

    255: base.BaseResp BaseResp
}

struct InsightAnalysisExperimentRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')

    200: optional common.Session session
    255: optional base.Base Base
}

struct InsightAnalysisExperimentResponse {
    1: required i64 insight_analysis_record_id (api.body = "insight_analysis_record_id", api.js_conv = 'true', go.tag = 'json:"insight_analysis_record_id"')

    255: base.BaseResp BaseResp
}

struct ListExptInsightAnalysisRecordRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: optional i32 page_number (api.body='page_number')
    4: optional i32 page_size (api.body='page_size')

    200: optional common.Session session
    255: optional base.Base Base
}

struct ListExptInsightAnalysisRecordResponse {
    1: required list<expt.ExptInsightAnalysisRecord> expt_insight_analysis_records
    20: optional i64 total (api.body = "total", go.tag = 'json:"total"')
    255: base.BaseResp BaseResp
}

struct DeleteExptInsightAnalysisRecordRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: required i64 insight_analysis_record_id (api.path = 'insight_analysis_record_id', api.js_conv = 'true', go.tag = 'json:"insight_analysis_record_id"')


    200: optional common.Session session
    255: optional base.Base Base
}

struct DeleteExptInsightAnalysisRecordResponse {

    255: base.BaseResp BaseResp
}

struct FeedbackExptInsightAnalysisReportRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: required i64 insight_analysis_record_id (api.path = 'insight_analysis_record_id', api.js_conv = 'true', go.tag = 'json:"insight_analysis_record_id"')
    4: required expt.FeedbackActionType feedback_action_type
    5: optional string comment
    6: optional i64 comment_id (api.body = 'comment_id', api.js_conv = 'true', go.tag = 'json:"comment_id"')    // 用于更新comment


    200: optional common.Session session
    255: optional base.Base Base
}

struct FeedbackExptInsightAnalysisReportResponse {

    255: base.BaseResp BaseResp
}

struct ListExptInsightAnalysisCommentRequest {
    1: required i64 workspace_id (api.body = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: required i64 expt_id (api.path = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: required i64 insight_analysis_record_id (api.path = 'insight_analysis_record_id', api.js_conv = 'true', go.tag = 'json:"insight_analysis_record_id"')
    4: optional i32 page_number (api.body='page_number')
    5: optional i32 page_size (api.body='page_size')

    200: optional common.Session session
    255: optional base.Base Base
}

struct ListExptInsightAnalysisCommentResponse {
    1: required list<expt.ExptInsightAnalysisFeedbackComment> expt_insight_analysis_feedback_comments
    20: optional i64 total (api.body = "total", go.tag = 'json:"total"')
    255: base.BaseResp BaseResp
}

struct GetAnalysisRecordFeedbackVoteRequest {
    1: optional i64 workspace_id (api.query = 'workspace_id', api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    2: optional i64 expt_id (api.query = 'expt_id' , api.js_conv = 'true', go.tag = 'json:"expt_id"')
    3: optional i64 insight_analysis_record_id (api.path = 'insight_analysis_record_id', api.js_conv = 'true', go.tag = 'json:"insight_analysis_record_id"')

    200: optional common.Session session
    255: optional base.Base Base
}

struct GetAnalysisRecordFeedbackVoteResponse {
    1: optional expt.ExptInsightAnalysisFeedbackVote vote
    255: base.BaseResp BaseResp
}

service ExperimentService {

    CheckExperimentNameResponse CheckExperimentName(1: CheckExperimentNameRequest req) (
        api.post = '/api/evaluation/v1/experiments/check_name', api.op_type = 'query', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    // CreateExperiment 只创建，不提交运行
    CreateExperimentResponse CreateExperiment(1: CreateExperimentRequest req)

    // SubmitExperiment 创建并提交运行
    SubmitExperimentResponse SubmitExperiment(1: SubmitExperimentRequest req) (
        api.post = '/api/evaluation/v1/experiments/submit', api.op_type = 'create', api.tag = 'volc-agentkit,open', api.category = 'experiment'
    )

    BatchGetExperimentsResponse BatchGetExperiments(1: BatchGetExperimentsRequest req) (
        api.post = '/api/evaluation/v1/experiments/batch_get', api.op_type = 'query', api.tag = 'volc-agentkit,open', api.category = 'experiment'
    )

    ListExperimentsResponse ListExperiments(1: ListExperimentsRequest req) (
        api.post = '/api/evaluation/v1/experiments/list', api.op_type = 'list', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    UpdateExperimentResponse UpdateExperiment(1: UpdateExperimentRequest req) (
        api.patch = '/api/evaluation/v1/experiments/:expt_id', api.op_type = 'update', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    DeleteExperimentResponse DeleteExperiment(1: DeleteExperimentRequest req) (
        api.delete = '/api/evaluation/v1/experiments/:expt_id', api.op_type = 'delete', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    BatchDeleteExperimentsResponse BatchDeleteExperiments(1: BatchDeleteExperimentsRequest req) (
        api.delete = '/api/evaluation/v1/experiments/batch_delete', api.op_type = 'delete', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    CloneExperimentResponse CloneExperiment(1: CloneExperimentRequest req) (
        api.post = '/api/evaluation/v1/experiments/:expt_id/clone', api.op_type = 'create', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    // RunExperiment 运行已创建的实验
    RunExperimentResponse RunExperiment(1: RunExperimentRequest req)

    RetryExperimentResponse RetryExperiment(1: RetryExperimentRequest req) (
        api.post = '/api/evaluation/v1/experiments/:expt_id/retry', api.op_type = 'update', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    KillExperimentResponse KillExperiment(1: KillExperimentRequest req) (
        api.post = '/api/evaluation/v1/experiments/:expt_id/kill', api.op_type = 'update', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    // MGetExperimentResult 获取实验结果
    BatchGetExperimentResultResponse BatchGetExperimentResult(1: BatchGetExperimentResultRequest req) (
        api.post = "/api/evaluation/v1/experiments/results/batch_get", api.op_type = 'query', api.tag = 'volc-agentkit,open', api.category = 'experiment'
    )

    CalculateExperimentAggrResultResponse CalculateExperimentAggrResult(1: CalculateExperimentAggrResultRequest req) (
        api.post = "/api/evaluation/v1/experiments/:expt_id/aggr_results", api.op_type = 'update', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    BatchGetExperimentAggrResultResponse BatchGetExperimentAggrResult(1: BatchGetExperimentAggrResultRequest req) (
        api.post = "/api/evaluation/v1/experiments/aggr_results/batch_get", api.op_type = 'query', api.tag = 'volc-agentkit,open', api.category = 'experiment'
    )

    // 在线实验
    InvokeExperimentResponse InvokeExperiment(1: InvokeExperimentRequest req)

    FinishExperimentResponse FinishExperiment(1: FinishExperimentRequest req)

    ListExperimentStatsResponse ListExperimentStats(1: ListExperimentStatsRequest req)

    // 更新报告ck
    UpsertExptTurnResultFilterResponse UpsertExptTurnResultFilter(1: UpsertExptTurnResultFilterRequest req)

    // 人工标注
    AssociateAnnotationTagResp AssociateAnnotationTag(1: AssociateAnnotationTagReq req) (api.post = "/api/evaluation/v1/experiments/:expt_id/associate_tag")
    DeleteAnnotationTagResp DeleteAnnotationTag(1: DeleteAnnotationTagReq req) (api.delete = "/api/evaluation/v1/experiments/:expt_id/delete_tag")
    CreateAnnotateRecordResp CreateAnnotateRecord(1: CreateAnnotateRecordReq req) (api.post = "/api/evaluation/v1/experiments/:expt_id/annotate_record/create")
    UpdateAnnotateRecordResp UpdateAnnotateRecord(1: UpdateAnnotateRecordReq req) (api.post = "/api/evaluation/v1/experiments/:expt_id/annotate_record/update")

    // 报告下载
    ExportExptResultResponse ExportExptResult(1: ExportExptResultRequest req) (
        api.post="/api/evaluation/v1/experiments/:expt_id/results/export", api.op_type = 'query', api.tag = 'volc-agentkit', api.category = 'experiment'
    )
    ListExptResultExportRecordResponse ListExptResultExportRecord(1: ListExptResultExportRecordRequest req) (
        api.post="/api/evaluation/v1/experiments/:expt_id/export_records/list", api.op_type = 'list', api.tag = 'volc-agentkit', api.category = 'experiment'
    )
    GetExptResultExportRecordResponse GetExptResultExportRecord(1: GetExptResultExportRecordRequest req) (
        api.post="/api/evaluation/v1/experiments/:expt_id/export_records/:export_id", api.op_type = 'query', api.tag = 'volc-agentkit', api.category = 'experiment'
    )

    // 报告分析
    InsightAnalysisExperimentResponse InsightAnalysisExperiment(1: InsightAnalysisExperimentRequest req) (api.post="/api/evaluation/v1/experiments/:expt_id/insight_analysis"    )
    ListExptInsightAnalysisRecordResponse ListExptInsightAnalysisRecord(1: ListExptInsightAnalysisRecordRequest req) (api.post="/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/list")
    DeleteExptInsightAnalysisRecordResponse DeleteExptInsightAnalysisRecord(1: DeleteExptInsightAnalysisRecordRequest req) (api.delete="/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/:insight_analysis_record_id")
    GetExptInsightAnalysisRecordResponse GetExptInsightAnalysisRecord(1: GetExptInsightAnalysisRecordRequest req) (api.post="/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/:insight_analysis_record_id")
    FeedbackExptInsightAnalysisReportResponse FeedbackExptInsightAnalysisReport(1: FeedbackExptInsightAnalysisReportRequest req) (api.post="/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/:insight_analysis_record_id/feedback")
    ListExptInsightAnalysisCommentResponse ListExptInsightAnalysisComment(1: ListExptInsightAnalysisCommentRequest req) (api.post="/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/:insight_analysis_record_id/comments/list")
    GetAnalysisRecordFeedbackVoteResponse GetAnalysisRecordFeedbackVote(1: GetAnalysisRecordFeedbackVoteRequest req) (api.get="/api/evaluation/v1/experiments/insight_analysis_records/:insight_analysis_record_id/feedback_vote")

    // 实验模板
    CreateExperimentTemplateResponse CreateExperimentTemplate(1: CreateExperimentTemplateRequest req) (
        api.post = '/api/evaluation/v1/experiment_templates', api.op_type = 'query', api.tag = 'volc-agentkit', api.category = 'experiment'
    )
    BatchGetExperimentTemplateResponse BatchGetExperimentTemplate(1: BatchGetExperimentTemplateRequest req) (
        api.post = '/api/evaluation/v1/experiment_templates/batch_get', api.op_type = 'query', api.tag = 'volc-agentkit', api.category = 'experiment'
    )
    UpdateExperimentTemplateMetaResponse UpdateExperimentTemplateMeta(1: UpdateExperimentTemplateMetaRequest req) (
        api.post = '/api/evaluation/v1/experiment_templates/update_meta', api.op_type = 'update', api.tag = 'volc-agentkit', api.category = 'experiment'
    )
    UpdateExperimentTemplateResponse UpdateExperimentTemplate(1: UpdateExperimentTemplateRequest req) (
        api.patch = '/api/evaluation/v1/experiment_templates/:template_id', api.op_type = 'update', api.tag = 'volc-agentkit', api.category = 'experiment'
    ) // 更新实验模板（不允许修改关联的评测对象 / 评测集，仅允许修改默认版本、映射、评估器与配置）
    DeleteExperimentTemplateResponse DeleteExperimentTemplate(1: DeleteExperimentTemplateRequest req) (
        api.delete = '/api/evaluation/v1/experiment_templates/:template_id', api.op_type = 'delete', api.tag = 'volc-agentkit', api.category = 'experiment'
    )
    ListExperimentTemplatesResponse ListExperimentTemplates(1: ListExperimentTemplatesRequest req) (
        api.post = '/api/evaluation/v1/experiment_templates/list', api.op_type = 'list', api.tag = 'volc-agentkit', api.category = 'experiment'
    )
    CheckExperimentTemplateNameResponse CheckExperimentTemplateName(1: CheckExperimentTemplateNameRequest req) (
        api.post = '/api/evaluation/v1/experiment_templates/check_name', api.op_type = 'query', api.tag = 'volc-agentkit', api.category = 'experiment'
    )
}

