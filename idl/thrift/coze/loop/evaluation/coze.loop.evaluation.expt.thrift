namespace go coze.loop.evaluation.expt

include "../../../base.thrift"
include "../data/domain/dataset.thrift"
include "./domain/eval_set.thrift"
include "coze.loop.evaluation.eval_target.thrift"
include "./domain/common.thrift"
include "./domain/expt.thrift"

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
    1: optional list<expt.ExptAggregateResult> expt_aggregate_results (api.body = 'expt_aggregate_result')

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
    1: optional expt.ExptResultExportRecord expt_result_export_record (agw.key = "expt_result_export_records")

    255: base.BaseResp BaseResp
}



service ExperimentService {

    CheckExperimentNameResponse CheckExperimentName(1: CheckExperimentNameRequest req) (api.post = '/api/evaluation/v1/experiments/check_name')

    // CreateExperiment 只创建，不提交运行
    CreateExperimentResponse CreateExperiment(1: CreateExperimentRequest req)

    // SubmitExperiment 创建并提交运行
    SubmitExperimentResponse SubmitExperiment(1: SubmitExperimentRequest req) (api.post = '/api/evaluation/v1/experiments/submit')

    BatchGetExperimentsResponse BatchGetExperiments(1: BatchGetExperimentsRequest req) (api.post = '/api/evaluation/v1/experiments/batch_get')

    ListExperimentsResponse ListExperiments(1: ListExperimentsRequest req) (api.post = '/api/evaluation/v1/experiments/list')

    UpdateExperimentResponse UpdateExperiment(1: UpdateExperimentRequest req) (api.patch = '/api/evaluation/v1/experiments/:expt_id')

    DeleteExperimentResponse DeleteExperiment(1: DeleteExperimentRequest req) (api.delete = '/api/evaluation/v1/experiments/:expt_id')

    BatchDeleteExperimentsResponse BatchDeleteExperiments(1: BatchDeleteExperimentsRequest req) (api.delete = '/api/evaluation/v1/experiments/batch_delete')

    CloneExperimentResponse CloneExperiment(1: CloneExperimentRequest req) (api.post = '/api/evaluation/v1/experiments/:expt_id/clone')

    // RunExperiment 运行已创建的实验
    RunExperimentResponse RunExperiment(1: RunExperimentRequest req)

    RetryExperimentResponse RetryExperiment(1: RetryExperimentRequest req) (api.post = '/api/evaluation/v1/experiments/:expt_id/retry')

    KillExperimentResponse KillExperiment(1: KillExperimentRequest req) (api.post = '/api/evaluation/v1/experiments/:expt_id/kill')

    // MGetExperimentResult 获取实验结果
    BatchGetExperimentResultResponse BatchGetExperimentResult(1: BatchGetExperimentResultRequest req) (api.post = "/api/evaluation/v1/experiments/results/batch_get")

    BatchGetExperimentAggrResultResponse BatchGetExperimentAggrResult(1: BatchGetExperimentAggrResultRequest req) (api.post = "/api/evaluation/v1/experiments/aggr_results/batch_get")

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
    ExportExptResultResponse ExportExptResult(1: ExportExptResultRequest req) (api.post="/api/evaluation/v1/experiments/:expt_id/results/export")
    ListExptResultExportRecordResponse ListExptResultExportRecord(1: ListExptResultExportRecordRequest req) (api.post="/api/evaluation/v1/experiments/:expt_id/export_records/list")
    GetExptResultExportRecordResponse GetExptResultExportRecord(1: GetExptResultExportRecordRequest req) (api.post="/api/evaluation/v1/experiments/:expt_id/export_records/:export_id")
}

