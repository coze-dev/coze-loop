// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as dataset from './../../data/domain/dataset';
export { dataset };
import * as tag from './../../data/domain/tag';
export { tag };
import * as eval_set from './eval_set';
export { eval_set };
import * as evaluator from './evaluator';
export { evaluator };
import * as eval_target from './eval_target';
export { eval_target };
import * as common from './common';
export { common };
export enum ExptStatus {
  Unknown = 0,
  /** Awaiting execution */
  Pending = 2,
  /** In progress */
  Processing = 3,
  /** Execution succeeded */
  Success = 11,
  /** Execution failed */
  Failed = 12,
  /** User terminated */
  Terminated = 13,
  /** System terminated */
  SystemTerminated = 14,
  /** Terminating */
  Terminating = 15,
  /** online expt draining */
  Draining = 21,
}
export enum ExptType {
  Offline = 1,
  Online = 2,
}
export enum SourceType {
  Evaluation = 1,
  AutoTask = 2,
}
export interface Experiment {
  id?: string,
  name?: string,
  desc?: string,
  creator_by?: string,
  status?: ExptStatus,
  status_message?: string,
  start_time?: string,
  end_time?: string,
  item_concur_num?: number,
  eval_set_version_id?: string,
  target_version_id?: string,
  evaluator_version_ids?: string[],
  eval_set?: eval_set.EvaluationSet,
  eval_target?: eval_target.EvalTarget,
  evaluators?: evaluator.Evaluator[],
  eval_set_id?: string,
  target_id?: string,
  base_info?: common.BaseInfo,
  expt_stats?: ExptStatistics,
  target_field_mapping?: TargetFieldMapping,
  evaluator_field_mapping?: EvaluatorFieldMapping[],
  target_runtime_param?: common.RuntimeParam,
  expt_type?: ExptType,
  max_alive_time?: number,
  source_type?: SourceType,
  source_id?: string,
  /** 补充的评估器id+version关联评估器方式，和evaluator_version_ids共同使用，兼容老逻辑 */
  evaluator_id_version_list?: evaluator.EvaluatorIDVersionItem[],
}
export interface TokenUsage {
  input_tokens?: string,
  output_tokens?: string,
}
export interface ExptStatistics {
  evaluator_aggregate_results?: EvaluatorAggregateResult[],
  token_usage?: TokenUsage,
  credit_cost?: number,
  pending_turn_cnt?: number,
  success_turn_cnt?: number,
  fail_turn_cnt?: number,
  terminated_turn_cnt?: number,
  processing_turn_cnt?: number,
}
export interface EvaluatorFmtResult {
  name?: string,
  score?: number,
}
export const PromptUserQueryFieldKey = "builtin_prompt_user_query";
export interface TargetFieldMapping {
  from_eval_set?: FieldMapping[]
}
export interface EvaluatorFieldMapping {
  evaluator_version_id: string,
  from_eval_set?: FieldMapping[],
  from_target?: FieldMapping[],
  evaluator_id_version_item?: evaluator.EvaluatorIDVersionItem,
}
export interface FieldMapping {
  field_name?: string,
  const_value?: string,
  from_field_name?: string,
}
export interface ExptFilterOption {
  fuzzy_name?: string,
  filters?: Filters,
}
export enum ExptRetryMode {
  Unknown = 0,
  RetryAll = 1,
  RetryFailure = 2,
  RetryTargetItems = 3,
}
export enum ItemRunState {
  Unknown = -1,
  /** Queuing */
  Queueing = 0,
  /** Processing */
  Processing = 1,
  /** Success */
  Success = 2,
  /** Failure */
  Fail = 3,
  /** Terminated */
  Terminal = 5,
}
export enum TurnRunState {
  /** Not started */
  Queueing = 0,
  /** Execution succeeded */
  Success = 1,
  /** Execution failed */
  Fail = 2,
  /** In progress */
  Processing = 3,
  /** Terminated */
  Terminal = 4,
}
export interface ItemSystemInfo {
  run_state?: ItemRunState,
  log_id?: string,
  error?: RunError,
}
export interface ExptColumnEvaluator {
  experiment_id: string,
  column_evaluators?: ColumnEvaluator[],
}
export interface ColumnEvaluator {
  evaluator_version_id: string,
  evaluator_id: string,
  evaluator_type: evaluator.EvaluatorType,
  name?: string,
  version?: string,
  description?: string,
  builtin?: boolean,
}
export interface ExptColumnEvalTarget {
  experiment_id?: string,
  column_eval_targets?: ColumnEvalTarget[],
}
export const ColumnEvalTargetName_ActualOutput = "actual_output";
export const ColumnEvalTargetName_Trajectory = "trajectory";
export const ColumnEvalTargetName_EvalTargetTotalLatency = "eval_target_total_latency";
export const ColumnEvalTargetName_EvaluatorInputTokens = "eval_target_input_tokens";
export const ColumnEvalTargetName_EvaluatorOutputTokens = "eval_target_output_tokens";
export const ColumnEvalTargetName_EvaluatorTotalTokens = "eval_target_total_tokens";
export interface ColumnEvalTarget {
  name?: string,
  description?: string,
  label?: string,
}
export interface ColumnEvalSetField {
  key?: string,
  name?: string,
  description?: string,
  content_type?: common.ContentType,
  /** 5: optional datasetv3.FieldDisplayFormat DefaultDisplayFormat */
  text_schema?: string,
  schema_key?: dataset.SchemaKey,
}
export interface ItemResult {
  item_id: string,
  /** row粒度实验结果详情 */
  turn_results?: TurnResult[],
  system_info?: ItemSystemInfo,
  item_index?: string,
  ext?: {
    [key: string | number]: string
  },
}
/** 行级结果 可能包含多个实验 */
export interface TurnResult {
  turn_id: string,
  /** 参与对比的实验序列，对于单报告序列长度为1 */
  experiment_results?: ExperimentResult[],
  turn_index?: string,
}
export interface ExperimentResult {
  experiment_id: string,
  payload?: ExperimentTurnPayload,
}
export interface TurnSystemInfo {
  turn_run_state?: TurnRunState,
  log_id?: string,
  error?: RunError,
}
export interface RunError {
  code: string,
  message?: string,
  detail?: string,
}
export interface TurnEvalSet {
  turn: eval_set.Turn
}
export interface TurnTargetOutput {
  eval_target_record?: eval_target.EvalTargetRecord
}
export interface TurnEvaluatorOutput {
  evaluator_records: {
    [key: string | number]: evaluator.EvaluatorRecord
  }
}
export interface TurnAnnotateResult {
  /** tag_key_id -> annotate_record */
  annotate_records: {
    [key: string | number]: AnnotateRecord
  }
}
export interface AnnotateRecord {
  annotate_record_id?: string,
  /** 标签ID */
  tag_key_id?: string,
  score?: string,
  boolean_option?: string,
  categorical_option?: string,
  plain_text?: string,
  tag_content_type?: tag.TagContentType,
  /** 标签选项值ID */
  tag_value_id?: string,
}
/** 实际行级payload */
export interface ExperimentTurnPayload {
  turn_id: string,
  /** 评测数据集数据 */
  eval_set?: TurnEvalSet,
  /** 评测对象结果 */
  target_output?: TurnTargetOutput,
  /** 评测规则执行结果 */
  evaluator_output?: TurnEvaluatorOutput,
  /** 评测系统相关数据日志、error */
  system_info?: TurnSystemInfo,
  /** 人工标注结果结果 */
  annotate_result?: TurnAnnotateResult,
  /** 轨迹分析结果 */
  trajectory_analysis_result?: TrajectoryAnalysisResult,
}
export interface TrajectoryAnalysisResult {
  record_id?: string,
  Status?: InsightAnalysisStatus,
}
export interface KeywordSearch {
  keyword?: string,
  filter_fields?: FilterField[],
}
export interface ExperimentFilter {
  filters?: Filters,
  keyword_search?: KeywordSearch,
}
export interface Filters {
  filter_conditions?: FilterCondition[],
  logic_op?: FilterLogicOp,
}
export enum FilterLogicOp {
  Unknown = 0,
  And = 1,
  Or = 2,
}
export interface FilterField {
  field_type: FieldType,
  /** 二级key放此字段里 */
  field_key?: string,
}
export enum FieldType {
  Unknown = 0,
  /** 评估器得分, FieldKey为evaluatorVersionID,value为score */
  EvaluatorScore = 1,
  CreatorBy = 2,
  ExptStatus = 3,
  TurnRunState = 4,
  TargetID = 5,
  EvalSetID = 6,
  EvaluatorID = 7,
  TargetType = 8,
  SourceTarget = 9,
  EvaluatorVersionID = 20,
  TargetVersionID = 21,
  EvalSetVersionID = 22,
  ExptType = 30,
  SourceType = 31,
  SourceID = 32,
  KeywordSearch = 41,
  /** 使用二级key，column_key */
  EvalSetColumn = 42,
  /** 使用二级key, Annotation_key（具体参考人工标注设计） */
  Annotation = 43,
  /** 使用二级key，目前使用固定key：content */
  ActualOutput = 44,
  EvaluatorScoreCorrected = 45,
  /** 使用二级key，evaluator_version_id */
  Evaluator = 46,
  ItemID = 47,
  ItemRunState = 48,
  /** 使用二级key, field_key为tag_key_id, value为score */
  AnnotationScore = 49,
  /** 使用二级key, field_key为tag_key_id, value为文本 */
  AnnotationText = 50,
  /** 使用二级key, field_key为tag_key_id, value为tag_value_id */
  AnnotationCategorical = 51,
  /** 目前使用固定key：total_latency */
  TotalLatency = 60,
  /** 目前使用固定key：input_tokens */
  InputTokens = 61,
  /** 目前使用固定key：output_tokens */
  OutputTokens = 62,
  /** 目前使用固定key：total_tokens */
  TotalTokens = 63,
}
/** 字段过滤器 */
export interface FilterCondition {
  /** 过滤字段，比如评估器ID */
  field: FilterField,
  /** 操作符，比如等于、包含、大于、小于等 */
  operator: FilterOperatorType,
  /** 操作值;支持多种类型的操作值； */
  value: string,
  source_target?: SourceTarget,
}
export interface SourceTarget {
  eval_target_type?: eval_target.EvalTargetType,
  source_target_ids?: string[],
}
export enum FilterOperatorType {
  Unknown = 0,
  /** 等于 */
  Equal = 1,
  /** 不等于 */
  NotEqual = 2,
  /** 大于 */
  Greater = 3,
  /** 大于等于 */
  GreaterOrEqual = 4,
  /** 小于 */
  Less = 5,
  /** 小于等于 */
  LessOrEqual = 6,
  /** 包含 */
  In = 7,
  /** 不包含 */
  NotIn = 8,
  /** 全文搜索 */
  Like = 9,
  /** 全文搜索反选 */
  NotLike = 10,
  /** 为空 */
  IsNull = 11,
  /** 非空 */
  IsNotNull = 12,
}
export enum ExptAggregateCalculateStatus {
  Unknown = 0,
  Idle = 1,
  Calculating = 2,
}
/** 实验粒度聚合结果 */
export interface ExptAggregateResult {
  experiment_id: string,
  evaluator_results?: {
    [key: string | number]: EvaluatorAggregateResult
  },
  status?: ExptAggregateCalculateStatus,
  /** tag_key_id -> result */
  annotation_results?: {
    [key: string | number]: AnnotationAggregateResult
  },
  eval_target_aggr_result?: EvalTargetAggregateResult,
  /** timestamp in seconds */
  update_time?: number,
}
export interface EvalTargetAggregateResult {
  target_id?: string,
  target_version_id?: string,
  latency?: AggregatorResult[],
  input_tokens?: AggregatorResult[],
  output_tokens?: AggregatorResult[],
  total_tokens?: AggregatorResult[],
}
/** 评估器版本粒度聚合结果 */
export interface EvaluatorAggregateResult {
  evaluator_version_id: string,
  aggregator_results?: AggregatorResult[],
  name?: string,
  version?: string,
}
/** 人工标注项粒度聚合结果 */
export interface AnnotationAggregateResult {
  tag_key_id: string,
  aggregator_results?: AggregatorResult[],
  name?: string,
}
/** 一种聚合器类型的聚合结果 */
export interface AggregatorResult {
  aggregator_type: AggregatorType,
  data?: AggregateData,
}
/** 聚合器类型 */
export enum AggregatorType {
  Average = 1,
  Sum = 2,
  Max = 3,
  Min = 4,
  /** 得分的分布情况 */
  Distribution = 5,
}
export enum DataType {
  /** 默认，有小数的浮点数值类型 */
  Double = 0,
  /** 得分分布 */
  ScoreDistribution = 1,
  /** 选项分布 */
  OptionDistribution = 2,
}
export interface ScoreDistribution {
  score_distribution_items?: ScoreDistributionItem[]
}
export interface ScoreDistributionItem {
  score: string,
  count: string,
  percentage: number,
}
export interface AggregateData {
  data_type: DataType,
  value?: number,
  score_distribution?: ScoreDistribution,
  option_distribution?: OptionDistribution,
}
export interface OptionDistribution {
  option_distribution_items?: OptionDistributionItem[]
}
export interface OptionDistributionItem {
  /** 值为tag_value_id,或`其他` */
  option: string,
  count: string,
  percentage: number,
}
export interface ExptStatsInfo {
  expt_id?: number,
  source_id?: string,
  expt_stats?: ExptStatistics,
}
export interface ExptColumnAnnotation {
  experiment_id: string,
  column_annotations?: ColumnAnnotation[],
}
/** 标签信息，沿用数据基座Tag定义 */
export interface ColumnAnnotation {
  tag_key_id?: string,
  /** tag key name */
  tag_key_name?: string,
  /** 描述 */
  description?: string,
  status?: tag.TagStatus,
  /** 标签选项值 */
  tag_values?: tag.TagValue[],
  /** 标签内容类型 */
  content_type?: tag.TagContentType,
  /** 标签内容限制 */
  content_spec?: tag.TagContentSpec,
}
export enum ExptResultExportType {
  CSV = "CSV",
}
export enum CSVExportStatus {
  Unknown = "Unknown",
  Running = "Running",
  Success = "Success",
  Failed = "Failed",
}
export interface ExptResultExportRecord {
  export_id: string,
  workspace_id: string,
  expt_id: string,
  csv_export_status: CSVExportStatus,
  base_info?: common.BaseInfo,
  start_time?: string,
  end_time?: string,
  URL?: string,
  expired?: boolean,
  error?: RunError,
}
/** 分析任务状态 */
export enum InsightAnalysisStatus {
  Unknown = "Unknown",
  Running = "Running",
  Success = "Success",
  Failed = "Failed",
}
/** 投票类型 */
export enum InsightAnalysisReportVoteType {
  /** 未投票 */
  None = "None",
  /** 点赞 */
  Upvote = "Upvote",
  /** 点踩 */
  Downvote = "Downvote",
}
/** 洞察分析记录 */
export interface ExptInsightAnalysisRecord {
  record_id: string,
  workspace_id: string,
  expt_id: string,
  analysis_status: InsightAnalysisStatus,
  analysis_report_id?: string,
  analysis_report_content?: string,
  expt_insight_analysis_feedback?: ExptInsightAnalysisFeedback,
  base_info?: common.BaseInfo,
  analysis_report_index?: ExptInsightAnalysisIndex[],
}
export interface ExptInsightAnalysisIndex {
  id?: string,
  title?: string,
}
/** 洞察分析反馈统计 */
export interface ExptInsightAnalysisFeedback {
  upvote_cnt?: number,
  downvote_cnt?: number,
  /** 当前用户点赞状态，用于展示用户是否已点赞点踩 */
  current_user_vote_type?: InsightAnalysisReportVoteType,
}
/** 洞察分析反馈评论 */
export interface ExptInsightAnalysisFeedbackComment {
  comment_id: string,
  workspace_id: string,
  expt_id: string,
  record_id: string,
  content: string,
  base_info?: common.BaseInfo,
}
export interface ExptInsightAnalysisFeedbackVote {
  comment_id?: string,
  feedback_action_type?: FeedbackActionType,
}
/** 反馈动作 */
export enum FeedbackActionType {
  Upvote = "Upvote",
  Cancel_Upvote = "Cancel_Upvote",
  Downvote = "Downvote",
  Cancel_Downvote = "Cancel_Downvote",
  Create_Comment = "Create_Comment",
  Update_Comment = "Update_Comment",
  Delete_Comment = "Delete_Comment",
}