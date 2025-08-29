import * as common from './common';
export { common };
export enum AnnotationType {
  AutoEvaluate = "auto_evaluate",
  EvaluationSet = "manual_evaluation_set",
  ManualFeedback = "manual_feedback",
  CozeFeedback = "coze_feedback",
}
export enum ValueType {
  String = "string",
  Long = "long",
  Double = "double",
  Bool = "bool",
}
export interface Correction {
  score?: number,
  explain?: string,
  base_info?: common.BaseInfo,
}
export interface EvaluatorResult {
  score?: number,
  correction?: Correction,
  reasoning?: string,
}
export interface AutoEvaluate {
  evaluator_version_id: string,
  evaluator_name: string,
  evaluator_version: string,
  evaluator_result?: EvaluatorResult,
  record_id: string,
  task_id: string,
}
export interface ManualFeedback {
  tag_key_id: string,
  tag_key_name: string,
  tag_value_id?: string,
  tag_value?: string,
}
export interface Annotation {
  id?: string,
  span_id?: string,
  trace_id?: string,
  workspace_id?: string,
  start_time?: string,
  type?: AnnotationType,
  key?: string,
  value_type?: ValueType,
  value?: string,
  status?: string,
  reasoning?: string,
  base_info?: common.BaseInfo,
  auto_evaluate?: AutoEvaluate,
  manual_feedback?: ManualFeedback,
}