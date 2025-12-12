import * as common from './common';
export { common };
export interface EvalTarget {
  /**
   * 基本信息
   * 一个对象的唯一标识
  */
  id?: string,
  /** 空间ID */
  workspace_id?: string,
  /** 源对象ID，例如prompt ID */
  source_target_id?: string,
  /** 评测对象类型 */
  eval_target_type?: EvalTargetType,
  /**
   * 版本信息
   * 目标版本
  */
  eval_target_version?: EvalTargetVersion,
  /** 系统信息 */
  base_info?: common.BaseInfo,
}
export interface EvalTargetVersion {
  /**
   * 基本信息
   * 版本唯一标识
  */
  id?: string,
  /** 空间ID */
  workspace_id?: string,
  /** 对象唯一标识 */
  target_id?: string,
  /** 源对象版本，例如prompt是0.0.1，bot是版本号12233等 */
  source_target_version?: string,
  /** 目标对象内容 */
  eval_target_content?: EvalTargetContent,
  /** 系统信息 */
  base_info?: common.BaseInfo,
}
export interface EvalTargetContent {
  /** 输入schema */
  input_schemas?: common.ArgsSchema[],
  /** 输出schema */
  output_schemas?: common.ArgsSchema[],
  runtime_param_json_demo?: string,
  /**
   * 101-200 EvalTarget类型
   * EvalTargetType=0 时，传参此字段。 评测对象为 CozeBot 时, 需要设置 CozeBot 信息
  */
  coze_bot?: CozeBot,
  /** EvalTargetType=1 时，传参此字段。 评测对象为 EvalPrompt 时, 需要设置 Prompt 信息 */
  prompt?: EvalPrompt,
  /** EvalTargetType=4 时，传参此字段。 评测对象为 CozeWorkflow 时, 需要设置 CozeWorkflow 信息 */
  coze_workflow?: CozeWorkflow,
  /** EvalTargetType=5 时，传参此字段。 评测对象为 VolcengineAgent 时, 需要设置 VolcengineAgent 信息 */
  volcengine_agent?: VolcengineAgent,
  /** EvalTargetType=6 时，传参此字段。 评测对象为 CustomRPCServer 时, 需要设置 CustomRPCServer 信息 */
  custom_rpc_server?: CustomRPCServer,
}
export enum EvalTargetType {
  /** CozeBot */
  CozeBot = 1,
  /** Prompt */
  CozeLoopPrompt = 2,
  /** Trace */
  Trace = 3,
  CozeWorkflow = 4,
  /** 火山智能体 */
  VolcengineAgent = 5,
  /** 自定义RPC服务 for内场 */
  CustomRPCServer = 6,
}
/** Agent协议类型 */
export enum VolcengineAgentProtocol {
  MCP = "mcp",
  /** mcp */
  A2A = "a2a",
  /** a2a */
  Other = "other",
}
/** other */
export interface CustomRPCServer {
  /** 应用ID */
  id?: number,
  /** DTO使用，不存数据库 */
  name?: string,
  /** DTO使用，不存数据库 */
  description?: string,
  /** 注意以下信息会存储到DB，也就是说实验创建时以下内容就确定了，运行时直接从评测DB中获取，而不是实时从app模块拉 */
  server_name?: string,
  /** 接入协议 */
  access_protocol?: AccessProtocol,
  regions?: Region[],
  cluster?: string,
  /** 执行http信息 */
  invoke_http_info?: HTTPInfo,
  /** 异步执行http信息，如果用户选了异步就传入这个字段 */
  async_invoke_http_info?: HTTPInfo,
  /** 是否需要搜索对象 */
  need_search_target?: boolean,
  /** 搜索对象http信息 */
  search_http_info?: HTTPInfo,
  /** 搜索对象返回的信息 */
  custom_eval_target?: CustomEvalTarget,
  /** 是否异步 */
  is_async?: boolean,
  /** 执行区域 */
  exec_region?: Region,
  /** 执行环境 */
  exec_env?: string,
  /** 执行超时时间，单位ms */
  timeout?: number,
  /** 异步执行超时时间，单位ms */
  async_timeout?: number,
  ext?: {
    [key: string | number]: string
  },
}
export interface CustomEvalTarget {
  /** 唯一键，平台不消费，仅做透传 */
  id?: string,
  /** 名称，平台用于展示在对象搜索下拉列表 */
  name?: string,
  /** 头像url，平台用于展示在对象搜索下拉列表 */
  avatar_url?: string,
  /** 扩展字段，目前主要存储旧版协议response中的额外字段：object_type(旧版ID)、object_meta、space_id */
  ext?: {
    [key: string | number]: string
  },
}
export interface HTTPInfo {
  method?: HTTPMethod,
  path?: string,
}
export enum Region {
  BOE = "boe",
  CN = "cn",
  I18N = "i18n",
}
export enum AccessProtocol {
  RPC = "rpc",
  RPCOld = "rpc_old",
  FaasHTTP = "faas_http",
  FaasHTTPOld = "faas_http_old",
}
export enum HTTPMethod {
  Get = "get",
  Post = "post",
}
export interface VolcengineAgent {
  /** 罗盘应用ID */
  id?: string,
  /** DTO使用，不存数据库 */
  name?: string,
  /** DTO使用，不存数据库 */
  description?: string,
  /** DTO使用，不存数据库 */
  volcengine_agent_endpoints?: VolcengineAgentEndpoint[],
  /** 注册协议 */
  protocol?: VolcengineAgentProtocol,
  base_info?: common.BaseInfo,
}
export interface VolcengineAgentEndpoint {
  endpoint_id?: string,
  api_key?: string,
}
export interface CozeWorkflow {
  id?: string,
  version?: string,
  /** DTO使用，不存数据库 */
  name?: string,
  /** DTO使用，不存数据库 */
  avatar_url?: string,
  /** DTO使用，不存数据库 */
  description?: string,
  base_info?: common.BaseInfo,
}
export interface EvalPrompt {
  prompt_id?: string,
  version?: string,
  /** DTO使用，不存数据库 */
  name?: string,
  /** DTO使用，不存数据库 */
  prompt_key?: string,
  /** DTO使用，不存数据库 */
  submit_status?: SubmitStatus,
  /** DTO使用，不存数据库 */
  description?: string,
}
export enum SubmitStatus {
  Undefined = 0,
  /** 未提交 */
  UnSubmit,
  /** 已提交 */
  Submitted,
}
/** Coze2.0Bot */
export interface CozeBot {
  bot_id?: string,
  bot_version?: string,
  bot_info_type?: CozeBotInfoType,
  model_info?: ModelInfo,
  /** DTO使用，不存数据库 */
  bot_name?: string,
  /** DTO使用，不存数据库 */
  avatar_url?: string,
  /** DTO使用，不存数据库 */
  description?: string,
  /** 如果是发布版本则这个字段不为空 */
  publish_version?: string,
  base_info?: common.BaseInfo,
}
export enum CozeBotInfoType {
  /** 草稿 bot */
  DraftBot = 1,
  /** 商店 bot */
  ProductBot = 2,
}
export interface ModelInfo {
  model_id: string,
  model_name: string,
  /** DTO使用，不存数据库 */
  show_name: string,
  /** DTO使用，不存数据库 */
  max_tokens: string,
  /** 模型家族信息 */
  model_family: string,
  /** 模型平台 */
  platform?: ModelPlatform,
}
export enum ModelPlatform {
  Unknown = 0,
  GPTOpenAPI = 1,
  MAAS = 2,
}
export interface EvalTargetRecord {
  /** 评估记录ID */
  id?: string,
  /** 空间ID */
  workspace_id?: string,
  target_id?: string,
  target_version_id?: string,
  /** 实验执行ID */
  experiment_run_id?: string,
  /** 评测集数据项ID */
  item_id?: string,
  /** 评测集数据项轮次ID */
  turn_id?: string,
  /** 链路ID */
  trace_id?: string,
  /** 链路ID */
  log_id?: string,
  /** 输入数据 */
  eval_target_input_data?: EvalTargetInputData,
  /** 输出数据 */
  eval_target_output_data?: EvalTargetOutputData,
  status?: EvalTargetRunStatus,
  base_info?: common.BaseInfo,
}
export enum EvalTargetRunStatus {
  Unknown = 0,
  Success = 1,
  Fail = 2,
  AsyncInvoking = 3,
}
export interface EvalTargetInputData {
  /** 历史会话记录 */
  history_messages?: common.Message[],
  /** 变量 */
  input_fields?: {
    [key: string | number]: common.Content
  },
  ext?: {
    [key: string | number]: string
  },
}
export interface EvalTargetOutputData {
  /** 变量 */
  output_fields?: {
    [key: string | number]: common.Content
  },
  /** 运行消耗 */
  eval_target_usage?: EvalTargetUsage,
  /** 运行报错 */
  eval_target_run_error?: EvalTargetRunError,
  /** 运行耗时 */
  time_consuming_ms?: string,
}
export interface EvalTargetUsage {
  input_tokens: string,
  output_tokens: string,
}
export interface EvalTargetRunError {
  code?: number,
  message?: string,
}
export interface PromptRuntimeParam {
  model_config?: common.ModelConfig
}