import * as runtime from './../../llm/domain/runtime';
export { runtime };
import * as common from './common';
export { common };
export enum EvaluatorType {
  Prompt = 1,
  Code = 2,
}
export enum LanguageType {
  Python = 1,
  JS = 2,
}
export enum PromptSourceType {
  BuiltinTemplate = 1,
  LoopPrompt = 2,
  Custom = 3,
}
export enum ToolType {
  Function = 1,
  /** for gemini native tool */
  GoogleSearch = 2,
}
export enum TemplateType {
  Prompt = 1,
  Code = 2,
}
export enum EvaluatorRunStatus {
  /** 运行状态, 异步下状态流转, 同步下只有 Success / Fail */
  Unknown = 0,
  Success = 1,
  Fail = 2,
}
export interface Tool {
  type: ToolType,
  function?: Function,
}
export interface Function {
  name: string,
  description?: string,
  parameters?: string,
}
export interface PromptEvaluator {
  message_list: common.Message[],
  model_config?: common.ModelConfig,
  prompt_source_type?: PromptSourceType,
  prompt_template_key?: string,
  prompt_template_name?: string,
  tools?: Tool[],
}
export interface CodeEvaluator {
  language_type?: LanguageType,
  code?: string,
}
export interface EvaluatorVersion {
  /** 版本id */
  id?: string,
  version?: string,
  description?: string,
  base_info?: common.BaseInfo,
  evaluator_content?: EvaluatorContent,
}
export interface EvaluatorContent {
  receive_chat_history?: boolean,
  input_schemas?: common.ArgsSchema[],
  /** 101-200 Evaluator类型 */
  prompt_evaluator?: PromptEvaluator,
  code_evaluator?: CodeEvaluator,
}
export interface Evaluator {
  evaluator_id?: string,
  workspace_id?: string,
  evaluator_type?: EvaluatorType,
  name?: string,
  description?: string,
  draft_submitted?: boolean,
  base_info?: common.BaseInfo,
  current_version?: EvaluatorVersion,
  latest_version?: string,
}
export interface Correction {
  score?: number,
  explain?: string,
  updated_by?: string,
}
export interface EvaluatorRecord {
  id?: string,
  experiment_id?: string,
  experiment_run_id?: string,
  item_id?: string,
  turn_id?: string,
  evaluator_version_id?: string,
  trace_id?: string,
  log_id?: string,
  evaluator_input_data?: EvaluatorInputData,
  evaluator_output_data?: EvaluatorOutputData,
  status?: EvaluatorRunStatus,
  base_info?: common.BaseInfo,
  ext?: {
    [key: string | number]: string
  },
}
export interface EvaluatorOutputData {
  evaluator_result?: EvaluatorResult,
  evaluator_usage?: EvaluatorUsage,
  evaluator_run_error?: EvaluatorRunError,
  time_consuming_ms?: string,
}
export interface EvaluatorResult {
  score?: number,
  correction?: Correction,
  reasoning?: string,
}
export interface EvaluatorUsage {
  input_tokens?: string,
  output_tokens?: string,
}
export interface EvaluatorRunError {
  code?: number,
  message?: string,
}
export interface EvaluatorInputData {
  history_messages?: common.Message[],
  input_fields?: {
    [key: string | number]: common.Content
  },
}