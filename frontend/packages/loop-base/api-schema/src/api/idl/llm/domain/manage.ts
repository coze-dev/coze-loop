// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as common from './common';
export { common };
export interface Model {
  model_id?: string,
  workspace_id?: string,
  name?: string,
  desc?: string,
  ability?: Ability,
  protocol?: Protocol,
  protocol_config?: ProtocolConfig,
  scenario_configs?: {
    [key: string | number]: ScenarioConfig
  },
  param_config?: ParamConfig,
  /** 模型表示 (name, endpoint) */
  identification?: string,
  /** 模型 */
  series?: Series,
  visibility?: Visibility,
  /** 模型图标 */
  icon?: string,
  /** 模型标签 */
  tags?: string[],
  /** 模型状态 */
  status?: ModelStatus,
  /** 模型跳转链接 */
  original_model_url?: string,
  /** 是否为预置模型 */
  preset_model?: boolean,
  created_by?: string,
  created_at?: number,
  updated_by?: string,
  updated_at?: number,
}
export interface Series {
  /** series name */
  name?: string,
  /** series icon url */
  icon?: string,
  /** family name */
  family?: Family,
}
export interface Visibility {
  mode?: VisibleMode,
  /** Mode为Specified有效，配置为除模型所属空间外的其他空间 */
  spaceIDs?: number[],
}
export interface ProviderInfo {
  maas_info?: MaaSInfo
}
export interface MaaSInfo {
  host?: string,
  region?: string,
  /** v3 sdk */
  baseURL?: string,
  /** 精调模型任务的 ID */
  customizationJobsID?: string,
}
export interface Ability {
  max_context_tokens?: string,
  max_input_tokens?: string,
  max_output_tokens?: string,
  function_call?: boolean,
  json_mode?: boolean,
  multi_modal?: boolean,
  ability_multi_modal?: AbilityMultiModal,
  interface_category?: InterfaceCategory,
}
export interface AbilityMultiModal {
  /** 图片 */
  image?: boolean,
  ability_image?: AbilityImage,
  /** 视频 */
  video?: boolean,
  ability_video?: AbilityVideo,
}
export interface AbilityImage {
  url_enabled?: boolean,
  binary_enabled?: boolean,
  max_image_size?: string,
  max_image_count?: string,
  image_gen_enabled?: boolean,
}
export interface AbilityVideo {
  /** the size limit of single video */
  max_video_size_in_mb?: number,
  supported_video_formats?: VideoFormat[],
}
export interface ProtocolConfig {
  base_url?: string,
  api_key?: string,
  model?: string,
  protocol_config_ark?: ProtocolConfigArk,
  protocol_config_openai?: ProtocolConfigOpenAI,
  protocol_config_claude?: ProtocolConfigClaude,
  protocol_config_deepseek?: ProtocolConfigDeepSeek,
  protocol_config_ollama?: ProtocolConfigOllama,
  protocol_config_qwen?: ProtocolConfigQwen,
  protocol_config_qianfan?: ProtocolConfigQianfan,
  protocol_config_gemini?: ProtocolConfigGemini,
  protocol_config_arkbot?: ProtocolConfigArkbot,
}
export interface ProtocolConfigArk {
  /** Default: "cn-beijing" */
  region?: string,
  access_key?: string,
  secret_key?: string,
  retry_times?: string,
  custom_headers?: {
    [key: string | number]: string
  },
}
export interface ProtocolConfigOpenAI {
  by_azure?: boolean,
  api_version?: string,
  response_format_type?: string,
  response_format_json_schema?: string,
}
export interface ProtocolConfigClaude {
  by_bedrock?: boolean,
  /** bedrock config */
  access_key?: string,
  secret_access_key?: string,
  session_token?: string,
  region?: string,
}
export interface ProtocolConfigDeepSeek {
  response_format_type?: string
}
export interface ProtocolConfigGemini {
  response_schema?: string,
  enable_code_execution?: boolean,
  safety_settings?: ProtocolConfigGeminiSafetySetting[],
}
export interface ProtocolConfigGeminiSafetySetting {
  category?: number,
  threshold?: number,
}
export interface ProtocolConfigOllama {
  format?: string,
  keep_alive_ms?: string,
}
export interface ProtocolConfigQwen {
  response_format_type?: string,
  response_format_json_schema?: string,
}
export interface ProtocolConfigQianfan {
  llm_retry_count?: number,
  llm_retry_timeout?: number,
  llm_retry_backoff_factor?: number,
  parallel_tool_calls?: boolean,
  response_format_type?: string,
  response_format_json_schema?: string,
}
export interface ProtocolConfigArkbot {
  /** Default: "cn-beijing" */
  region?: string,
  access_key?: string,
  secret_key?: string,
  retry_times?: string,
  custom_headers?: {
    [key: string | number]: string
  },
}
export interface ScenarioConfig {
  scenario?: common.Scenario,
  quota?: Quota,
  unavailable?: boolean,
}
export interface ParamConfig {
  param_schemas?: ParamSchema[]
}
export interface ParamSchema {
  /** 实际名称 */
  name?: string,
  /** 展示名称 */
  label?: string,
  desc?: string,
  type?: ParamType,
  min?: string,
  max?: string,
  default_value?: string,
  options?: ParamOption[],
  properties?: ParamSchema[],
  /** 依赖参数 */
  reaction?: Reaction,
  /** 赋值路径 */
  jsonpath?: string,
}
export interface Reaction {
  /** 依赖的字段 */
  dependency?: string,
  /** 可见性表达式 */
  visible?: string,
}
export interface ParamOption {
  /** 实际值 */
  value?: string,
  /** 展示值 */
  label?: string,
}
export interface Quota {
  qpm?: string,
  tpm?: string,
}
export enum Protocol {
  protocol_ark = "ark",
  protocol_openai = "openai",
  protocol_claude = "claude",
  protocol_deepseek = "deepseek",
  protocol_ollama = "ollama",
  protocol_gemini = "gemini",
  protocol_qwen = "qwen",
  protocol_qianfan = "qianfan",
  protocol_arkbot = "arkbot",
}
export enum ParamType {
  param_type_float = "float",
  param_type_int = "int",
  param_type_boolean = "boolean",
  param_type_string = "string",
  param_type_void = "void",
  param_type_object = "object",
}
export enum Family {
  family_undefined = "undefined",
  family_gpt = "gpt",
  family_seed = "seed",
  family_gemini = "gemini",
  family_claude = "claude",
  family_ernie = "ernie",
  family_baichuan = "baichuan",
  family_qwen = "qwen",
  family_glm = "glm",
  family_skylark = "skylark",
  family_moonshot = "moonshot",
  family_minimax = "minimax",
  family_doubao = "doubao",
  family_baichuan2 = "baichuan2",
  family_deepseekv2 = "deepseekv2",
  family_deepseek_coder_v2 = "deepseek_coder_v2",
  family_deepseek_coder = "deepseek_coder",
  family_internalm25 = "internalm2_5",
  family_qwen2 = "qwen2",
  family_qwen25 = "qwen2.5",
  family_qwen25_coder = "qwen2.5_coder",
  family_mini_cpm = "mini_cpm",
  family_mini_cpm3 = "mini_cpm_3",
  family_chat_glm3 = "chat_glm_3",
  family_mistra = "mistral",
  family_gemma = "gemma",
  family_gemma_2 = "gemma_2",
  family_intern_vl2 = "intern_vl2",
  family_intern_vl25 = "intern_vl2.5",
  family_deepseek_v3 = "deepseek_v3",
  family_deepseek_r1 = "deepseek_r1",
  family_kimi = "kimi",
  family_seedream = "seedream",
  family_intern_vl3 = "intern_vl3",
  family_deepseek = "deepseek",
}
export enum Provider {
  provider_undefined = "undefined",
  provider_maas = "maas",
}
export enum VisibleMode {
  visible_mode_default = "default",
  visible_mode_specified = "specified",
  visible_mode_undefined = "undefined",
  visible_mode_all = "all",
}
export enum ModelStatus {
  model_status_undefined = "undefined",
  model_status_available = "available",
  /** 可用 */
  model_status_unavailable = "unavailable",
}
/** 不可用 */
export enum InterfaceCategory {
  interface_category_undefined = "undefined",
  interface_category_chat_completion_api = "chat_completion_api",
  interface_category_response_api = "response_api",
}
export enum AbilityEnum {
  ability_undefined = "undefined",
  ability_json_mode = "json_mode",
  ability_function_call = "function_call",
  ability_multi_modal = "multi_modal",
}
export enum VideoFormat {
  video_format_undefined = "undefined",
  video_format_mp4 = "mp4",
  video_format_avi = "avi",
  video_format_mov = "mov",
  video_format_mpg = "mpg",
  video_format_webm = "webm",
  video_format_rvmb = "rvmb",
  video_format_wmv = "wmv",
  video_format_mkv = "mkv",
  video_format_t3gp = "t3gp",
  video_format_flv = "flv",
  video_format_mpeg = "mpeg",
  video_format_ts = "ts",
  video_format_rm = "rm",
  video_format_m4v = "m4v",
}