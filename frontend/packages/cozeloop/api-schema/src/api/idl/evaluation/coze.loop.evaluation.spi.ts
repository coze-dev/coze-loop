import * as base from './../../../base';
export { base };
export interface SearchEvalTargetRequest {
  /** 空间id */
  workspace_id?: number,
  /** 搜索关键字，如需使用请用户自行实现 */
  keyword?: string,
  /** 扩展字段：目前会透传regoin和空间id信息，key名如下：search_region、search_space_id */
  ext?: {
    [key: string | number]: string
  },
  page_size?: number,
  page_token?: string,
}
export interface SearchEvalTargetResponse {
  custom_eval_targets?: CustomEvalTarget[],
  next_page_token?: string,
  has_more?: boolean,
}
export interface CustomEvalTarget {
  /** 唯一键，平台不消费，仅做透传 */
  id?: string,
  /** 名称，平台用于展示在对象搜索下拉列表 */
  name?: string,
  /** 头像url，平台用于展示在对象搜索下拉列表 */
  avatar_url?: string,
}
export interface InvokeEvalTargetRequest {
  /** 空间id */
  workspace_id?: number,
  /** 输入信息 */
  input?: InvokeEvalTargetInput,
  /** 如果创建实验时选了二级对象，则会透传search接口返回的二级对象信息 */
  custom_eval_target?: CustomEvalTarget,
}
export interface InvokeEvalTargetResponse {
  status?: InvokeEvalTargetStatus,
  /** set output if status=SUCCESS */
  output?: InvokeEvalTargetOutput,
  /** set usage if status=SUCCESS */
  usage?: InvokeEvalTargetUsage,
  /** set error_message if status=FAILED */
  error_message?: string,
}
export interface InvokeEvalTargetInput {
  /** 评测集字段信息，key=评测集列名,value=评测集列值 */
  eval_set_fields?: {
    [key: string | number]: Content
  },
  /** 扩展字段，动态参数会通过ext字段传递 */
  ext?: {
    [key: string | number]: string
  },
}
export enum InvokeEvalTargetStatus {
  UNKNOWN = 0,
  SUCCESS = 1,
  FAILED = 2,
}
/** 新增 */
export interface InvokeEvalTargetOutput {
  actual_output?: Content,
  /** 扩展字段，用户如果想返回一些额外信息可以塞在这个字段 */
  ext?: {
    [key: string | number]: string
  },
}
export interface Content {
  /** 类型 */
  content_type?: ContentType,
  /** 当content_type=text，则从此字段中取值 */
  text?: string,
  /** 当content_type=image，则从此字段中取图片信息 */
  image?: Image,
  /** 当content_type=multi_part，则从此字段遍历获取多模态的值 */
  multi_part?: Content[],
}
export enum ContentType {
  Text = "text",
  /** 文本类型：string、integer、float、boolean、object、array都属于文本类型 */
  Image = "image",
  MultiPart = "multi_part",
}
/** 多模态，例如图+文 */
export interface Image {
  url?: string
}
export interface InvokeEvalTargetUsage {
  /** 输入token消耗 */
  input_tokens?: number,
  /** 输出token消耗 */
  output_tokens?: number,
}
export interface AsyncInvokeEvalTargetRequest {
  workspace_id?: number,
  /** 执行id，传递给自定义对象，在回传结果时透传 */
  invoke_id?: number,
  /** 执行输入信息 */
  input?: InvokeEvalTargetInput,
  /** 如果创建实验时选了二级对象，则会透传二级对象信息 */
  custom_eval_target?: CustomEvalTarget,
}
export interface AsyncInvokeEvalTargetResponse {}
/** the run status enumerate for custom evaluator */
export enum InvokeEvaluatorRunStatus {
  UNKNOWN = 0,
  SUCCESS = 1,
  FAILED = 2,
}
/** the custom evaluator identity and parameter information */
export interface InvokeCustomEvaluator {
  /** provider-side evaluator identity code */
  provider_evaluator_code?: string
}
/** the input data structure for custom evaluator */
export interface InvokeEvaluatorInputData {
  /** key-value structure of input variables required by the evaluator */
  input_fields?: {
    [key: string | number]: Content
  }
}
/** the output data structure for custom evaluator */
export interface InvokeEvaluatorOutputData {
  evaluator_result?: InvokeEvaluatorResult,
  evaluator_usage?: InvokeEvaluatorUsage,
  evaluator_run_error?: InvokeEvaluatorRunError,
}
/** the result data structure for custom evaluator */
export interface InvokeEvaluatorResult {
  score?: number,
  reasoning?: string,
}
/** the usage data structure for custom evaluator */
export interface InvokeEvaluatorUsage {
  input_tokens?: number,
  output_tokens?: number,
}
/** the error data structure for custom evaluator */
export interface InvokeEvaluatorRunError {
  code?: number,
  message?: string,
}
/** invoke custom evaluator request */
export interface InvokeEvaluatorRequest {
  workspace_id?: number,
  evaluator?: InvokeCustomEvaluator,
  input_data?: InvokeEvaluatorInputData,
}
/** invoke custom evaluator response */
export interface InvokeEvaluatorResponse {
  output_data?: InvokeEvaluatorOutputData,
  status?: InvokeEvaluatorRunStatus,
}