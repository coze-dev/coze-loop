// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as evaluator from './domain/evaluator';
export { evaluator };
import * as expt from './domain/expt';
export { expt };
import * as common from './domain/common';
export { common };
import * as coze_loop_evaluation_eval_target from './coze.loop.evaluation.eval_target';
export { coze_loop_evaluation_eval_target };
import * as eval_set from './domain/eval_set';
export { eval_set };
import * as data_dataset from './../data/domain/dataset';
export { data_dataset };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface CreateExperimentRequest {
  workspace_id: string,
  eval_set_version_id?: string,
  target_version_id?: string,
  evaluator_version_ids?: string[],
  name?: string,
  desc?: string,
  eval_set_id?: string,
  target_id?: string,
  /** 实验模板可见性，默认为空，可见 */
  visibility?: expt.Visibility,
  target_field_mapping?: expt.TargetFieldMapping,
  evaluator_field_mapping?: expt.EvaluatorFieldMapping[],
  item_concur_num?: number,
  evaluators_concur_num?: number,
  create_eval_target_param?: coze_loop_evaluation_eval_target.CreateEvalTargetParam,
  target_runtime_param?: common.RuntimeParam,
  expt_type?: expt.ExptType,
  max_alive_time?: number,
  source_type?: expt.SourceType,
  source_id?: string,
  /** 补充的评估器id+version关联评估器方式，和evaluator_version_ids共同使用，兼容老逻辑 */
  evaluator_id_version_list?: evaluator.EvaluatorIDVersionItem[],
  /** 是否启用评估器得分加权汇总，以及各评估器的权重配置（key 为 evaluator_version_id，value 为权重） */
  enable_weighted_score?: boolean,
  evaluator_score_weights?: {
    [key: string | number]: number
  },
  expt_template_id?: string,
  item_retry_num?: number,
  /** 试运行行数 */
  trial_run_item_count?: number,
  /** 关联的智能评测会话ID */
  thread_id?: string,
  trigger_type?: expt.ExptTriggerType,
  session?: common.Session,
}
export interface CreateExperimentResponse {
  experiment?: expt.Experiment
}
export interface SubmitExperimentRequest {
  workspace_id: string,
  eval_set_version_id?: string,
  target_version_id?: string,
  evaluator_version_ids?: string[],
  name?: string,
  desc?: string,
  eval_set_id?: string,
  target_id?: string,
  /** 实验模板可见性，默认为空，可见 */
  visibility?: expt.Visibility,
  target_field_mapping?: expt.TargetFieldMapping,
  evaluator_field_mapping?: expt.EvaluatorFieldMapping[],
  item_concur_num?: number,
  evaluators_concur_num?: number,
  create_eval_target_param?: coze_loop_evaluation_eval_target.CreateEvalTargetParam,
  target_runtime_param?: common.RuntimeParam,
  expt_type?: expt.ExptType,
  max_alive_time?: number,
  source_type?: expt.SourceType,
  source_id?: string,
  /** 补充的评估器id+version关联评估器方式，和evaluator_version_ids共同使用，兼容老逻辑 */
  evaluator_id_version_list?: evaluator.EvaluatorIDVersionItem[],
  /** 是否启用评估器得分加权汇总，以及各评估器的权重配置（key 为 evaluator_version_id，value 为权重） */
  enable_weighted_score?: boolean,
  expt_template_id?: string,
  item_retry_num?: number,
  /** 试运行行数 */
  trial_run_item_count?: number,
  /**
   * 智能评测相关
   * 关联的智能评测会话ID
  */
  thread_id?: string,
  trigger_type?: expt.ExptTriggerType,
  time_range?: expt.TaskTimeRange,
  ext?: {
    [key: string | number]: string
  },
  session?: common.Session,
}
export interface SubmitExperimentResponse {
  experiment?: expt.Experiment,
  run_id?: string,
}
export interface ListExperimentsRequest {
  workspace_id: string,
  page_number?: number,
  page_size?: number,
  filter_option?: expt.ExptFilterOption,
  order_bys?: common.OrderBy[],
}
export interface ListExperimentsResponse {
  experiments?: expt.Experiment[],
  total?: number,
}
export interface BatchGetExperimentsRequest {
  workspace_id: string,
  expt_ids: string[],
}
export interface BatchGetExperimentsResponse {
  experiments?: expt.Experiment[]
}
export interface UpdateExperimentRequest {
  workspace_id: string,
  expt_id: string,
  name?: string,
  desc?: string,
}
export interface UpdateExperimentResponse {
  experiment?: expt.Experiment
}
export interface DeleteExperimentRequest {
  workspace_id: string,
  expt_id: string,
}
export interface DeleteExperimentResponse {}
export interface BatchDeleteExperimentsRequest {
  workspace_id: string,
  expt_ids: string[],
}
export interface BatchDeleteExperimentsResponse {}
export interface RunExperimentRequest {
  workspace_id?: string,
  expt_id?: string,
  item_ids?: string[],
  expt_type?: expt.ExptType,
  item_retry_num?: number,
  /** 试运行行数 */
  trial_run_item_count?: number,
  ext?: {
    [key: string | number]: string
  },
  session?: common.Session,
}
export interface RunExperimentResponse {
  run_id?: string
}
export interface RetryExperimentRequest {
  retry_mode?: expt.ExptRetryMode,
  workspace_id?: string,
  expt_id?: string,
  item_ids?: string[],
  ext?: {
    [key: string | number]: string
  },
}
export interface RetryExperimentResponse {
  run_id?: string
}
export interface KillExperimentRequest {
  expt_id?: string,
  workspace_id?: string,
}
export interface KillExperimentResponse {}
export interface CloneExperimentRequest {
  expt_id?: string,
  workspace_id?: string,
}
export interface CloneExperimentResponse {
  experiment?: expt.Experiment
}
export interface BatchGetExperimentResultRequest {
  workspace_id: string,
  experiment_ids: string[],
  /** Baseline experiment ID for experiment comparison */
  baseline_experiment_id?: string,
  /** key: experiment_id */
  filters?: {
    [key: string | number]: expt.ExperimentFilter
  },
  page_number?: number,
  page_size?: number,
  use_accelerator?: boolean,
  /** 是否包含轨迹 */
  full_trajectory?: boolean,
}
export interface BatchGetExperimentResultResponse {
  /** 数据集表头信息 */
  column_eval_set_fields: expt.ColumnEvalSetField[],
  /** 评估器表头信息 */
  column_evaluators?: expt.ColumnEvaluator[],
  expt_column_evaluators?: expt.ExptColumnEvaluator[],
  /** 人工标注标签表头信息 */
  expt_column_annotations?: expt.ExptColumnAnnotation[],
  expt_column_eval_target?: expt.ExptColumnEvalTarget[],
  /** item粒度实验结果详情 */
  item_results?: expt.ItemResult[],
  total?: number,
}
export interface BatchGetExperimentAggrResultRequest {
  workspace_id: string,
  experiment_ids: string[],
}
export interface BatchGetExperimentAggrResultResponse {
  expt_aggregate_result?: expt.ExptAggregateResult[]
}
export interface CalculateExperimentAggrResultRequest {
  workspace_id: string,
  expt_id: string,
}
export interface CalculateExperimentAggrResultResponse {}
export interface CheckExperimentNameRequest {
  workspace_id: string,
  name?: string,
}
export interface CheckExperimentNameResponse {
  pass?: boolean,
  message?: string,
}
export interface InvokeExperimentRequest {
  workspace_id: number,
  evaluation_set_id: number,
  items?: eval_set.EvaluationSetItem[],
  /** items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据 */
  skip_invalid_items?: boolean,
  /** 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条 */
  allow_partial_add?: boolean,
  experiment_id?: number,
  experiment_run_id?: number,
  ext?: {
    [key: string | number]: string
  },
  session?: common.Session,
}
export interface InvokeExperimentResponse {
  /** key: item 在 items 中的索引 */
  added_items?: {
    [key: string | number]: number
  },
  errors?: data_dataset.ItemErrorGroup[],
  item_outputs?: data_dataset.CreateDatasetItemOutput[],
}
export interface FinishExperimentRequest {
  workspace_id?: number,
  experiment_id?: number,
  experiment_run_id?: number,
  cid?: string,
  session?: common.Session,
}
export interface FinishExperimentResponse {}
export interface ListExperimentStatsRequest {
  workspace_id: number,
  page_number?: number,
  page_size?: number,
  filter_option?: expt.ExptFilterOption,
  session?: common.Session,
}
export interface ListExperimentStatsResponse {
  expt_stats_infos?: expt.ExptStatsInfo[],
  total?: number,
}
/**
 * =========================
 * 实验模板相关接口
 * =========================
*/
export interface CreateExperimentTemplateRequest {
  workspace_id: string,
  /** 模板结构，与 ExptTemplate 保持一致 */
  meta?: expt.ExptTemplateMeta,
  triple_config?: expt.ExptTuple,
  field_mapping_config?: expt.ExptFieldMapping,
  /** 创建评估对象参数（不在 ExptTemplate 结构中，保留在顶层） */
  create_eval_target_param?: coze_loop_evaluation_eval_target.CreateEvalTargetParam,
  /** 默认评估器并发数（不在 ExptTemplate 结构中，保留在顶层） */
  default_evaluators_concur_num?: number,
  /** 调度配置（不在 ExptTemplate 结构中，保留在顶层） */
  schedule_cron?: string,
  expt_source?: expt.ExptSource,
  /** 模板运行态信息（如是否开启定时触发）；创建时可只填 cron_activate */
  expt_info?: expt.ExptInfo,
  session?: common.Session,
}
export interface CreateExperimentTemplateResponse {
  experiment_template?: expt.ExptTemplate
}
export interface BatchGetExperimentTemplateRequest {
  workspace_id: string,
  template_ids: string[],
}
export interface BatchGetExperimentTemplateResponse {
  experiment_templates?: expt.ExptTemplate[]
}
export interface UpdateExperimentTemplateMetaRequest {
  workspace_id: string,
  template_id: string,
  meta?: expt.ExptTemplateMeta,
}
export interface UpdateExperimentTemplateMetaResponse {
  meta?: expt.ExptTemplateMeta
}
export interface UpdateExperimentTemplateRequest {
  workspace_id: string,
  template_id: string,
  /**
   * 模板结构，与 ExptTemplate 保持一致
   * 注意：eval_set_id / target_id 不允许修改，仅允许调整版本与配置
  */
  meta?: expt.ExptTemplateMeta,
  triple_config?: expt.ExptTuple,
  field_mapping_config?: expt.ExptFieldMapping,
  /** 创建评估对象参数（不在 ExptTemplate 结构中，保留在顶层） */
  create_eval_target_param?: coze_loop_evaluation_eval_target.CreateEvalTargetParam,
  /** 默认评估器并发数（不在 ExptTemplate 结构中，保留在顶层） */
  default_evaluators_concur_num?: number,
  /** 调度配置（不在 ExptTemplate 结构中，保留在顶层） */
  schedule_cron?: string,
  expt_info?: expt.ExptInfo,
}
export interface UpdateExperimentTemplateResponse {
  experiment_template?: expt.ExptTemplate
}
export interface DeleteExperimentTemplateRequest {
  workspace_id: string,
  template_id: string,
}
export interface DeleteExperimentTemplateResponse {}
export interface ListExperimentTemplatesRequest {
  workspace_id: string,
  page_number?: number,
  page_size?: number,
  filter_option?: expt.ExperimentTemplateFilter,
  order_bys?: common.OrderBy[],
}
export interface ListExperimentTemplatesResponse {
  experiment_templates?: expt.ExptTemplate[],
  total?: number,
}
export interface CheckExperimentTemplateNameRequest {
  workspace_id: string,
  name: string,
  template_id?: string,
}
export interface CheckExperimentTemplateNameResponse {
  is_available?: boolean
}
export enum UpsertExptTurnResultFilterType {
  /** 标签状态 */
  MANUAL = "manual",
  /** 启用 */
  AUTO = "auto",
  /** 禁用 */
  CHECK = "check",
}
/** 旧版本状态 */
export interface UpsertExptTurnResultFilterRequest {
  workspace_id?: number,
  experiment_id?: number,
  item_ids?: number[],
  filter_type?: UpsertExptTurnResultFilterType,
  retry_times?: number,
}
export interface UpsertExptTurnResultFilterResponse {}
export interface AssociateAnnotationTagReq {
  workspace_id: string,
  expt_id: string,
  tag_key_id?: string,
  session?: common.Session,
}
export interface AssociateAnnotationTagResp {}
export interface DeleteAnnotationTagReq {
  workspace_id: string,
  expt_id: string,
  tag_key_id?: string,
  session?: common.Session,
}
export interface DeleteAnnotationTagResp {}
export interface CreateAnnotateRecordReq {
  workspace_id: string,
  expt_id: string,
  annotate_record: expt.AnnotateRecord,
  item_id: string,
  turn_id: string,
  session?: common.Session,
}
export interface CreateAnnotateRecordResp {
  annotate_record_id: string
}
export interface UpdateAnnotateRecordReq {
  workspace_id: string,
  expt_id: string,
  annotate_records: expt.AnnotateRecord,
  annotate_record_id: string,
  item_id: string,
  turn_id: string,
  session?: common.Session,
}
export interface UpdateAnnotateRecordResp {}
/** 实验报告 CSV 导出列：多个一级分组，组内 list<string>。不传 export_columns：导出全部（含标注列等）。传 export_columns（含空 struct）：白名单模式，仅 item_id、status 等必填列 + 各分组非空 list 中的列；某一 list 未传（unset）与传 [] 对该组均表示不导出。人工标注列需在 tag_key_ids 中显式列出 TagKeyID（十进制字符串）才会在白名单导出中出现。 */
export interface ExptResultExportColumnSpec {
  /** 评测集字段：ColumnEvalSetField.Key */
  eval_set_fields?: string[],
  /** 评测对象输出（非性能指标）：ColumnEvalTarget.Name，如 actual_output、trajectory、自定义输出名 */
  eval_target_outputs?: string[],
  /** 性能指标：ColumnEvalTarget.Name（如 eval_target_total_latency、eval_target_input_tokens 等） */
  metrics?: string[],
  /** 评估器版本 ID 列表（字符串形式十进制）；每个 ID 导出该评估器的 score 与 reason 列 */
  evaluator_version_ids?: string[],
  /** 是否导出加权分数 */
  weighted_score?: boolean,
  /** 人工标注：每项为标注 TagKeyID（十进制字符串），与 ColumnAnnotation.TagKeyID 对应，导出该标注列 */
  tag_key_ids?: string[],
}
export interface ExportExptResultRequest {
  workspace_id: string,
  expt_id: string,
  export_columns?: ExptResultExportColumnSpec,
  export_type?: expt.ExptResultExportType,
  session?: common.Session,
}
export interface ExportExptResultResponse {
  export_id: string
}
export interface ListExptResultExportRecordRequest {
  workspace_id: string,
  expt_id: string,
  page_number?: number,
  page_size?: number,
  session?: common.Session,
}
export interface ListExptResultExportRecordResponse {
  expt_result_export_records: expt.ExptResultExportRecord[],
  total?: number,
}
export interface GetExptResultExportRecordRequest {
  workspace_id: string,
  expt_id: string,
  export_id: string,
  session?: common.Session,
}
export interface GetExptResultExportRecordResponse {
  expt_result_export_records?: expt.ExptResultExportRecord
}
export interface GetExptInsightAnalysisRecordRequest {
  workspace_id: string,
  expt_id: string,
  insight_analysis_record_id: string,
  session?: common.Session,
}
export interface GetExptInsightAnalysisRecordResponse {
  expt_insight_analysis_record?: expt.ExptInsightAnalysisRecord
}
export interface InsightAnalysisExperimentRequest {
  workspace_id: string,
  expt_id: string,
  session?: common.Session,
}
export interface InsightAnalysisExperimentResponse {
  insight_analysis_record_id: string
}
export interface ListExptInsightAnalysisRecordRequest {
  workspace_id: string,
  expt_id: string,
  page_number?: number,
  page_size?: number,
  session?: common.Session,
}
export interface ListExptInsightAnalysisRecordResponse {
  expt_insight_analysis_records: expt.ExptInsightAnalysisRecord[],
  total?: number,
}
export interface DeleteExptInsightAnalysisRecordRequest {
  workspace_id: string,
  expt_id: string,
  insight_analysis_record_id: string,
  session?: common.Session,
}
export interface DeleteExptInsightAnalysisRecordResponse {}
export interface FeedbackExptInsightAnalysisReportRequest {
  workspace_id: string,
  expt_id: string,
  insight_analysis_record_id: string,
  feedback_action_type: expt.FeedbackActionType,
  comment?: string,
  /** 用于更新comment */
  comment_id?: string,
  session?: common.Session,
}
export interface FeedbackExptInsightAnalysisReportResponse {}
export interface ListExptInsightAnalysisCommentRequest {
  workspace_id: string,
  expt_id: string,
  insight_analysis_record_id: string,
  page_number?: number,
  page_size?: number,
  session?: common.Session,
}
export interface ListExptInsightAnalysisCommentResponse {
  expt_insight_analysis_feedback_comments: expt.ExptInsightAnalysisFeedbackComment[],
  total?: number,
}
export interface GetAnalysisRecordFeedbackVoteRequest {
  workspace_id?: string,
  expt_id?: string,
  insight_analysis_record_id?: string,
  session?: common.Session,
}
export interface GetAnalysisRecordFeedbackVoteResponse {
  vote?: expt.ExptInsightAnalysisFeedbackVote
}
export const CheckExperimentName = /*#__PURE__*/createAPI<CheckExperimentNameRequest, CheckExperimentNameResponse>({
  "url": "/api/evaluation/v1/experiments/check_name",
  "method": "POST",
  "name": "CheckExperimentName",
  "reqType": "CheckExperimentNameRequest",
  "reqMapping": {
    "body": ["workspace_id", "name"]
  },
  "resType": "CheckExperimentNameResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
/** SubmitExperiment 创建并提交运行 */
export const SubmitExperiment = /*#__PURE__*/createAPI<SubmitExperimentRequest, SubmitExperimentResponse>({
  "url": "/api/evaluation/v1/experiments/submit",
  "method": "POST",
  "name": "SubmitExperiment",
  "reqType": "SubmitExperimentRequest",
  "reqMapping": {
    "body": ["workspace_id", "eval_set_version_id", "target_version_id", "evaluator_version_ids", "name", "desc", "eval_set_id", "target_id", "visibility", "target_field_mapping", "evaluator_field_mapping", "item_concur_num", "evaluators_concur_num", "create_eval_target_param", "target_runtime_param", "expt_type", "max_alive_time", "source_type", "source_id", "evaluator_id_version_list", "enable_weighted_score", "expt_template_id", "item_retry_num", "trial_run_item_count", "thread_id", "trigger_type", "time_range", "ext", "session"]
  },
  "resType": "SubmitExperimentResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const BatchGetExperiments = /*#__PURE__*/createAPI<BatchGetExperimentsRequest, BatchGetExperimentsResponse>({
  "url": "/api/evaluation/v1/experiments/batch_get",
  "method": "POST",
  "name": "BatchGetExperiments",
  "reqType": "BatchGetExperimentsRequest",
  "reqMapping": {
    "body": ["workspace_id", "expt_ids"]
  },
  "resType": "BatchGetExperimentsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const ListExperiments = /*#__PURE__*/createAPI<ListExperimentsRequest, ListExperimentsResponse>({
  "url": "/api/evaluation/v1/experiments/list",
  "method": "POST",
  "name": "ListExperiments",
  "reqType": "ListExperimentsRequest",
  "reqMapping": {
    "body": ["workspace_id", "page_number", "page_size", "filter_option", "order_bys"]
  },
  "resType": "ListExperimentsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const UpdateExperiment = /*#__PURE__*/createAPI<UpdateExperimentRequest, UpdateExperimentResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id",
  "method": "PATCH",
  "name": "UpdateExperiment",
  "reqType": "UpdateExperimentRequest",
  "reqMapping": {
    "body": ["workspace_id", "name", "desc"],
    "path": ["expt_id"]
  },
  "resType": "UpdateExperimentResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const DeleteExperiment = /*#__PURE__*/createAPI<DeleteExperimentRequest, DeleteExperimentResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id",
  "method": "DELETE",
  "name": "DeleteExperiment",
  "reqType": "DeleteExperimentRequest",
  "reqMapping": {
    "body": ["workspace_id"],
    "path": ["expt_id"]
  },
  "resType": "DeleteExperimentResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const BatchDeleteExperiments = /*#__PURE__*/createAPI<BatchDeleteExperimentsRequest, BatchDeleteExperimentsResponse>({
  "url": "/api/evaluation/v1/experiments/batch_delete",
  "method": "DELETE",
  "name": "BatchDeleteExperiments",
  "reqType": "BatchDeleteExperimentsRequest",
  "reqMapping": {
    "body": ["workspace_id", "expt_ids"]
  },
  "resType": "BatchDeleteExperimentsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const CloneExperiment = /*#__PURE__*/createAPI<CloneExperimentRequest, CloneExperimentResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/clone",
  "method": "POST",
  "name": "CloneExperiment",
  "reqType": "CloneExperimentRequest",
  "reqMapping": {
    "path": ["expt_id"],
    "body": ["workspace_id"]
  },
  "resType": "CloneExperimentResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const RetryExperiment = /*#__PURE__*/createAPI<RetryExperimentRequest, RetryExperimentResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/retry",
  "method": "POST",
  "name": "RetryExperiment",
  "reqType": "RetryExperimentRequest",
  "reqMapping": {
    "body": ["retry_mode", "workspace_id", "item_ids", "ext"],
    "path": ["expt_id"]
  },
  "resType": "RetryExperimentResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const KillExperiment = /*#__PURE__*/createAPI<KillExperimentRequest, KillExperimentResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/kill",
  "method": "POST",
  "name": "KillExperiment",
  "reqType": "KillExperimentRequest",
  "reqMapping": {
    "path": ["expt_id"],
    "body": ["workspace_id"]
  },
  "resType": "KillExperimentResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
/** MGetExperimentResult 获取实验结果 */
export const BatchGetExperimentResult = /*#__PURE__*/createAPI<BatchGetExperimentResultRequest, BatchGetExperimentResultResponse>({
  "url": "/api/evaluation/v1/experiments/results/batch_get",
  "method": "POST",
  "name": "BatchGetExperimentResult",
  "reqType": "BatchGetExperimentResultRequest",
  "reqMapping": {
    "query": ["workspace_id", "page_number", "page_size", "use_accelerator", "full_trajectory"],
    "body": ["experiment_ids", "baseline_experiment_id", "filters"]
  },
  "resType": "BatchGetExperimentResultResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const CalculateExperimentAggrResult = /*#__PURE__*/createAPI<CalculateExperimentAggrResultRequest, CalculateExperimentAggrResultResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/aggr_results",
  "method": "POST",
  "name": "CalculateExperimentAggrResult",
  "reqType": "CalculateExperimentAggrResultRequest",
  "reqMapping": {
    "body": ["workspace_id"],
    "path": ["expt_id"]
  },
  "resType": "CalculateExperimentAggrResultResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const BatchGetExperimentAggrResult = /*#__PURE__*/createAPI<BatchGetExperimentAggrResultRequest, BatchGetExperimentAggrResultResponse>({
  "url": "/api/evaluation/v1/experiments/aggr_results/batch_get",
  "method": "POST",
  "name": "BatchGetExperimentAggrResult",
  "reqType": "BatchGetExperimentAggrResultRequest",
  "reqMapping": {
    "query": ["workspace_id"],
    "body": ["experiment_ids"]
  },
  "resType": "BatchGetExperimentAggrResultResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
/** 人工标注 */
export const AssociateAnnotationTag = /*#__PURE__*/createAPI<AssociateAnnotationTagReq, AssociateAnnotationTagResp>({
  "url": "/api/evaluation/v1/experiments/:expt_id/associate_tag",
  "method": "POST",
  "name": "AssociateAnnotationTag",
  "reqType": "AssociateAnnotationTagReq",
  "reqMapping": {
    "body": ["workspace_id", "tag_key_id", "session"],
    "path": ["expt_id"]
  },
  "resType": "AssociateAnnotationTagResp",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const DeleteAnnotationTag = /*#__PURE__*/createAPI<DeleteAnnotationTagReq, DeleteAnnotationTagResp>({
  "url": "/api/evaluation/v1/experiments/:expt_id/delete_tag",
  "method": "DELETE",
  "name": "DeleteAnnotationTag",
  "reqType": "DeleteAnnotationTagReq",
  "reqMapping": {
    "body": ["workspace_id", "tag_key_id", "session"],
    "path": ["expt_id"]
  },
  "resType": "DeleteAnnotationTagResp",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const CreateAnnotateRecord = /*#__PURE__*/createAPI<CreateAnnotateRecordReq, CreateAnnotateRecordResp>({
  "url": "/api/evaluation/v1/experiments/:expt_id/annotate_record/create",
  "method": "POST",
  "name": "CreateAnnotateRecord",
  "reqType": "CreateAnnotateRecordReq",
  "reqMapping": {
    "body": ["workspace_id", "annotate_record", "item_id", "turn_id", "session"],
    "path": ["expt_id"]
  },
  "resType": "CreateAnnotateRecordResp",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const UpdateAnnotateRecord = /*#__PURE__*/createAPI<UpdateAnnotateRecordReq, UpdateAnnotateRecordResp>({
  "url": "/api/evaluation/v1/experiments/:expt_id/annotate_record/update",
  "method": "POST",
  "name": "UpdateAnnotateRecord",
  "reqType": "UpdateAnnotateRecordReq",
  "reqMapping": {
    "body": ["workspace_id", "annotate_records", "annotate_record_id", "item_id", "turn_id", "session"],
    "path": ["expt_id"]
  },
  "resType": "UpdateAnnotateRecordResp",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
/** 报告下载 */
export const ExportExptResult = /*#__PURE__*/createAPI<ExportExptResultRequest, ExportExptResultResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/results/export",
  "method": "POST",
  "name": "ExportExptResult",
  "reqType": "ExportExptResultRequest",
  "reqMapping": {
    "body": ["workspace_id", "export_columns", "export_type", "session"],
    "path": ["expt_id"]
  },
  "resType": "ExportExptResultResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const ListExptResultExportRecord = /*#__PURE__*/createAPI<ListExptResultExportRecordRequest, ListExptResultExportRecordResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/export_records/list",
  "method": "POST",
  "name": "ListExptResultExportRecord",
  "reqType": "ListExptResultExportRecordRequest",
  "reqMapping": {
    "body": ["workspace_id", "page_number", "page_size", "session"],
    "path": ["expt_id"]
  },
  "resType": "ListExptResultExportRecordResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const GetExptResultExportRecord = /*#__PURE__*/createAPI<GetExptResultExportRecordRequest, GetExptResultExportRecordResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/export_records/:export_id",
  "method": "POST",
  "name": "GetExptResultExportRecord",
  "reqType": "GetExptResultExportRecordRequest",
  "reqMapping": {
    "body": ["workspace_id", "session"],
    "path": ["expt_id", "export_id"]
  },
  "resType": "GetExptResultExportRecordResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
/** 报告分析 */
export const InsightAnalysisExperiment = /*#__PURE__*/createAPI<InsightAnalysisExperimentRequest, InsightAnalysisExperimentResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/insight_analysis",
  "method": "POST",
  "name": "InsightAnalysisExperiment",
  "reqType": "InsightAnalysisExperimentRequest",
  "reqMapping": {
    "body": ["workspace_id", "session"],
    "path": ["expt_id"]
  },
  "resType": "InsightAnalysisExperimentResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const ListExptInsightAnalysisRecord = /*#__PURE__*/createAPI<ListExptInsightAnalysisRecordRequest, ListExptInsightAnalysisRecordResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/list",
  "method": "POST",
  "name": "ListExptInsightAnalysisRecord",
  "reqType": "ListExptInsightAnalysisRecordRequest",
  "reqMapping": {
    "body": ["workspace_id", "page_number", "page_size", "session"],
    "path": ["expt_id"]
  },
  "resType": "ListExptInsightAnalysisRecordResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const DeleteExptInsightAnalysisRecord = /*#__PURE__*/createAPI<DeleteExptInsightAnalysisRecordRequest, DeleteExptInsightAnalysisRecordResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/:insight_analysis_record_id",
  "method": "DELETE",
  "name": "DeleteExptInsightAnalysisRecord",
  "reqType": "DeleteExptInsightAnalysisRecordRequest",
  "reqMapping": {
    "body": ["workspace_id", "session"],
    "path": ["expt_id", "insight_analysis_record_id"]
  },
  "resType": "DeleteExptInsightAnalysisRecordResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const GetExptInsightAnalysisRecord = /*#__PURE__*/createAPI<GetExptInsightAnalysisRecordRequest, GetExptInsightAnalysisRecordResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/:insight_analysis_record_id",
  "method": "POST",
  "name": "GetExptInsightAnalysisRecord",
  "reqType": "GetExptInsightAnalysisRecordRequest",
  "reqMapping": {
    "body": ["workspace_id", "session"],
    "path": ["expt_id", "insight_analysis_record_id"]
  },
  "resType": "GetExptInsightAnalysisRecordResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const FeedbackExptInsightAnalysisReport = /*#__PURE__*/createAPI<FeedbackExptInsightAnalysisReportRequest, FeedbackExptInsightAnalysisReportResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/:insight_analysis_record_id/feedback",
  "method": "POST",
  "name": "FeedbackExptInsightAnalysisReport",
  "reqType": "FeedbackExptInsightAnalysisReportRequest",
  "reqMapping": {
    "body": ["workspace_id", "feedback_action_type", "comment", "comment_id", "session"],
    "path": ["expt_id", "insight_analysis_record_id"]
  },
  "resType": "FeedbackExptInsightAnalysisReportResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const ListExptInsightAnalysisComment = /*#__PURE__*/createAPI<ListExptInsightAnalysisCommentRequest, ListExptInsightAnalysisCommentResponse>({
  "url": "/api/evaluation/v1/experiments/:expt_id/insight_analysis_records/:insight_analysis_record_id/comments/list",
  "method": "POST",
  "name": "ListExptInsightAnalysisComment",
  "reqType": "ListExptInsightAnalysisCommentRequest",
  "reqMapping": {
    "body": ["workspace_id", "page_number", "page_size", "session"],
    "path": ["expt_id", "insight_analysis_record_id"]
  },
  "resType": "ListExptInsightAnalysisCommentResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const GetAnalysisRecordFeedbackVote = /*#__PURE__*/createAPI<GetAnalysisRecordFeedbackVoteRequest, GetAnalysisRecordFeedbackVoteResponse>({
  "url": "/api/evaluation/v1/experiments/insight_analysis_records/:insight_analysis_record_id/feedback_vote",
  "method": "GET",
  "name": "GetAnalysisRecordFeedbackVote",
  "reqType": "GetAnalysisRecordFeedbackVoteRequest",
  "reqMapping": {
    "query": ["workspace_id", "expt_id", "session"],
    "path": ["insight_analysis_record_id"]
  },
  "resType": "GetAnalysisRecordFeedbackVoteResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
/** 实验模板 */
export const CreateExperimentTemplate = /*#__PURE__*/createAPI<CreateExperimentTemplateRequest, CreateExperimentTemplateResponse>({
  "url": "/api/evaluation/v1/experiment_templates",
  "method": "POST",
  "name": "CreateExperimentTemplate",
  "reqType": "CreateExperimentTemplateRequest",
  "reqMapping": {
    "body": ["workspace_id", "meta", "triple_config", "field_mapping_config", "create_eval_target_param", "default_evaluators_concur_num", "schedule_cron", "expt_source", "expt_info", "session"]
  },
  "resType": "CreateExperimentTemplateResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const BatchGetExperimentTemplate = /*#__PURE__*/createAPI<BatchGetExperimentTemplateRequest, BatchGetExperimentTemplateResponse>({
  "url": "/api/evaluation/v1/experiment_templates/batch_get",
  "method": "POST",
  "name": "BatchGetExperimentTemplate",
  "reqType": "BatchGetExperimentTemplateRequest",
  "reqMapping": {
    "body": ["workspace_id", "template_ids"]
  },
  "resType": "BatchGetExperimentTemplateResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const UpdateExperimentTemplateMeta = /*#__PURE__*/createAPI<UpdateExperimentTemplateMetaRequest, UpdateExperimentTemplateMetaResponse>({
  "url": "/api/evaluation/v1/experiment_templates/update_meta",
  "method": "POST",
  "name": "UpdateExperimentTemplateMeta",
  "reqType": "UpdateExperimentTemplateMetaRequest",
  "reqMapping": {
    "body": ["workspace_id", "template_id", "meta"]
  },
  "resType": "UpdateExperimentTemplateMetaResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const UpdateExperimentTemplate = /*#__PURE__*/createAPI<UpdateExperimentTemplateRequest, UpdateExperimentTemplateResponse>({
  "url": "/api/evaluation/v1/experiment_templates/:template_id",
  "method": "PATCH",
  "name": "UpdateExperimentTemplate",
  "reqType": "UpdateExperimentTemplateRequest",
  "reqMapping": {
    "body": ["workspace_id", "meta", "triple_config", "field_mapping_config", "create_eval_target_param", "default_evaluators_concur_num", "schedule_cron", "expt_info"],
    "path": ["template_id"]
  },
  "resType": "UpdateExperimentTemplateResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
/** 更新实验模板（不允许修改关联的评测对象 / 评测集，仅允许修改默认版本、映射、评估器与配置） */
export const DeleteExperimentTemplate = /*#__PURE__*/createAPI<DeleteExperimentTemplateRequest, DeleteExperimentTemplateResponse>({
  "url": "/api/evaluation/v1/experiment_templates/:template_id",
  "method": "DELETE",
  "name": "DeleteExperimentTemplate",
  "reqType": "DeleteExperimentTemplateRequest",
  "reqMapping": {
    "body": ["workspace_id"],
    "path": ["template_id"]
  },
  "resType": "DeleteExperimentTemplateResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const ListExperimentTemplates = /*#__PURE__*/createAPI<ListExperimentTemplatesRequest, ListExperimentTemplatesResponse>({
  "url": "/api/evaluation/v1/experiment_templates/list",
  "method": "POST",
  "name": "ListExperimentTemplates",
  "reqType": "ListExperimentTemplatesRequest",
  "reqMapping": {
    "body": ["workspace_id", "page_number", "page_size", "filter_option", "order_bys"]
  },
  "resType": "ListExperimentTemplatesResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});
export const CheckExperimentTemplateName = /*#__PURE__*/createAPI<CheckExperimentTemplateNameRequest, CheckExperimentTemplateNameResponse>({
  "url": "/api/evaluation/v1/experiment_templates/check_name",
  "method": "POST",
  "name": "CheckExperimentTemplateName",
  "reqType": "CheckExperimentTemplateNameRequest",
  "reqMapping": {
    "body": ["workspace_id", "name", "template_id"]
  },
  "resType": "CheckExperimentTemplateNameResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.expt",
  "service": "evaluationExpt"
});