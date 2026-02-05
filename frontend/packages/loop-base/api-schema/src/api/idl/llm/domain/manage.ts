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
}
export interface Ability {
  max_context_tokens?: string,
  max_input_tokens?: string,
  max_output_tokens?: string,
  function_call?: boolean,
  json_mode?: boolean,
  multi_modal?: boolean,
  ability_multi_modal?: AbilityMultiModal,
}
export interface AbilityMultiModal {
  image?: boolean,
  ability_image?: AbilityImage,
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