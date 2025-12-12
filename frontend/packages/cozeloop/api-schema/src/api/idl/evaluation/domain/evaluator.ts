import * as runtime from './../../llm/domain/runtime';
export { runtime };
import * as common from './common';
export { common };
export enum EvaluatorType {
  Prompt = 1,
  Code = 2,
  CustomRPC = 3,
}
export enum LanguageType {
  Python = "Python",
  /** 空间 */
  JS = "JS",
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
export enum EvaluatorTagType {
  Evaluator = "Evaluator",
  Template = "Template",
}
export enum EvaluatorTagLangType {
  Zh = "zh-CN",
  En = "en-US",
}
/** Evaluator筛选字段 */
export enum EvaluatorTagKey {
  Category = "Category",
  /** 类型筛选 (LLM/Code) */
  TargetType = "TargetType",
  /** 评估对象 (文本/图片/视频等) */
  Objective = "Objective",
  /** 评估目标 (任务完成/内容质量等) */
  BusinessScenario = "BusinessScenario",
  /** 业务场景 (安全风控/AI Coding等) */
  Name = "Name",
}
/** 评估器名称 */
export enum EvaluatorBoxType {
  White = "White",
  /** 白盒 */
  Black = "Black",
}
/** 黑盒 */
export enum EvaluatorAccessProtocol {
  RPC = "rpc",
}
export enum EvaluatorVersionType {
  Latest = "Latest",
  /** 最新版本 */
  BuiltinVisible = "BuiltinVisible",
}
/** 内置可见版本 */
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
  /** 最新版本中存evaluator_template_id */
  prompt_template_key?: string,
  prompt_template_name?: string,
  tools?: Tool[],
}
export interface CodeEvaluator {
  language_type?: LanguageType,
  code_content?: string,
  /** code类型评估器模板中code_template_key + language_type是唯一键；最新版本中存evaluator_template_id */
  code_template_key?: string,
  code_template_name?: string,
  lang_2_code_content?: {
    [key: string | number]: string
  },
}
export interface CustomRPCEvaluator {
  /** 自定义评估器编码，例如：EvalBot的给“代码生成-代码正确”赋予CN:480的评估器ID */
  provider_evaluator_code?: string,
  /** 本期是RPC，后续还可拓展HTTP */
  access_protocol: EvaluatorAccessProtocol,
  service_name?: string,
  cluster?: string,
  /** ms */
  timeout?: number,
  /** 自定义评估器的限流配置 */
  rate_limit?: common.RateLimit,
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
  output_schemas?: common.ArgsSchema[],
  /** 101-200 Evaluator类型 */
  prompt_evaluator?: PromptEvaluator,
  code_evaluator?: CodeEvaluator,
  custom_rpc_evaluator?: CustomRPCEvaluator,
}
/** 明确有顺序的 evaluator 与版本映射元素 */
export interface EvaluatorIDVersionItem {
  evaluator_id?: string,
  version?: string,
}
export interface EvaluatorInfo {
  benchmark?: string,
  vendor?: string,
  vendor_url?: string,
  user_manual_url?: string,
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
  builtin?: boolean,
  evaluator_info?: EvaluatorInfo,
  builtin_visible_version?: string,
  /** 默认白盒 */
  box_type?: EvaluatorBoxType,
  tags?: {
    [key: string | number]: {
      [key: string | number]: string[]
    }
  },
}
export interface EvaluatorTemplate {
  id?: string,
  workspace_id?: string,
  evaluator_type?: EvaluatorType,
  name?: string,
  description?: string,
  /** 热度 */
  popularity?: number,
  evaluator_info?: EvaluatorInfo,
  tags?: {
    [key: string | number]: {
      [key: string | number]: string[]
    }
  },
  evaluator_content?: EvaluatorContent,
  base_info?: common.BaseInfo,
}
/** Evaluator筛选器选项 */
export interface EvaluatorFilterOption {
  /** 模糊搜索关键词，在所有tag中搜索 */
  search_keyword?: string,
  /** 筛选条件 */
  filters?: EvaluatorFilters,
}
/** Evaluator筛选条件 */
export interface EvaluatorFilters {
  /** 筛选条件列表 */
  filter_conditions?: EvaluatorFilterCondition[],
  /** 逻辑操作符 */
  logic_op?: EvaluatorFilterLogicOp,
  sub_filters?: EvaluatorFilters[],
}
/** 筛选逻辑操作符 */
export enum EvaluatorFilterLogicOp {
  Unknown = "Unknown",
  And = "And",
  /** 与操作 */
  Or = "Or",
}
/**
 * 或操作
 * Evaluator筛选条件
*/
export interface EvaluatorFilterCondition {
  /** 筛选字段 */
  tag_key: EvaluatorTagKey,
  /** 操作符 */
  operator: EvaluatorFilterOperatorType,
  /** 操作值 */
  value: string,
}
/** Evaluator筛选操作符 */
export enum EvaluatorFilterOperatorType {
  Unknown = "Unknown",
  Equal = "Equal",
  /** 等于 */
  NotEqual = "NotEqual",
  /** 不等于 */
  In = "In",
  /** 包含于 */
  NotIn = "NotIn",
  /** 不包含于 */
  Like = "Like",
  /** 模糊匹配 */
  IsNull = "IsNull",
  /** 为空 */
  IsNotNull = "IsNotNull",
}
/** 非空 */
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
  stdout?: string,
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
  evaluate_dataset_fields?: {
    [key: string | number]: common.Content
  },
  evaluate_target_output_fields?: {
    [key: string | number]: common.Content
  },
  ext?: {
    [key: string | number]: string
  },
}