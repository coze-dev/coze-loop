// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export interface Prompt {
  id?: string,
  workspace_id?: string,
  prompt_key?: string,
  prompt_basic?: PromptBasic,
  prompt_draft?: PromptDraft,
  prompt_commit?: PromptCommit,
}
export interface PromptBasic {
  display_name?: string,
  description?: string,
  latest_version?: string,
  created_by?: string,
  updated_by?: string,
  created_at?: string,
  updated_at?: string,
  latest_committed_at?: string,
  prompt_type?: PromptType,
  security_level?: SecurityLevel,
}
export enum PromptType {
  Normal = "normal",
  Snippet = "snippet",
}
export enum SecurityLevel {
  L1 = "L1",
  L2 = "L2",
  L3 = "L3",
  L4 = "L4",
}
export interface PromptCommit {
  detail?: PromptDetail,
  commit_info?: CommitInfo,
}
export interface CommitInfo {
  version?: string,
  base_version?: string,
  description?: string,
  committed_by?: string,
  committed_at?: string,
}
export interface PromptDraft {
  detail?: PromptDetail,
  draft_info?: DraftInfo,
}
export interface DraftInfo {
  user_id?: string,
  base_version?: string,
  is_modified?: boolean,
  created_at?: string,
  updated_at?: string,
}
export interface PromptDetail {
  prompt_template?: PromptTemplate,
  tools?: Tool[],
  tool_call_config?: ToolCallConfig,
  model_config?: ModelConfig,
  mcp_config?: McpConfig,
  ext_infos?: {
    [key: string | number]: string
  },
}
export interface McpConfig {
  is_mcp_call_auto_retry?: boolean,
  mcp_servers?: McpServerCombine[],
}
export interface McpServerCombine {
  mcp_server_id?: string,
  access_point_id?: string,
  disabled_tools?: string[],
  enabled_tools?: string[],
  is_enabled_tools?: boolean,
}
export interface PromptTemplate {
  template_type?: TemplateType,
  messages?: Message[],
  variable_defs?: VariableDef[],
  has_snippet?: boolean,
  snippets?: Prompt[],
  metadata?: {
    [key: string | number]: string
  },
}
export enum TemplateType {
  Normal = "normal",
  Jinja2 = "jinja2",
  GoTemplate = "go_template",
  CustomTemplate_M = "custom_template_m",
}
export interface Tool {
  type?: ToolType,
  function?: Function,
}
export enum ToolType {
  Function = "function",
  GoogleSearch = "google_search",
}
export interface Function {
  name?: string,
  description?: string,
  parameters?: string,
}
export interface ToolCallConfig {
  tool_choice?: ToolChoiceType,
  tool_choice_specification?: ToolChoiceSpecification,
}
export interface ToolChoiceSpecification {
  type?: ToolType,
  name?: string,
}
export enum ToolChoiceType {
  None = "none",
  Auto = "auto",
  Specific = "specific",
}
export interface ModelConfig {
  model_id?: string,
  max_tokens?: number,
  temperature?: number,
  top_k?: number,
  top_p?: number,
  presence_penalty?: number,
  frequency_penalty?: number,
  json_mode?: boolean,
  extra?: string,
  thinking?: ThinkingConfig,
  param_config_values?: ParamConfigValue[],
}
export interface ThinkingConfig {
  /** thinking内容的最大输出token */
  budget_tokens?: string,
  thinking_option?: ThinkingOption,
  /** 思考长度 */
  reasoning_effort?: ReasoningEffort,
}
export enum ReasoningEffort {
  Minimal = "minimal",
  Low = "low",
  Medium = "medium",
  High = "high",
}
export enum ThinkingOption {
  Disabled = "disabled",
  Enabled = "enabled",
  Auto = "auto",
}
export interface ParamConfigValue {
  /** 传给下游模型的key，与ParamSchema.name对齐 */
  name?: string,
  /** 展示名称 */
  label?: string,
  /** 传给下游模型的value，与ParamSchema.options对齐 */
  value?: ParamOption,
}
export interface ParamOption {
  /** 实际值 */
  value?: string,
  /** 展示值 */
  label?: string,
}
export interface Message {
  role?: Role,
  reasoning_content?: string,
  content?: string,
  parts?: ContentPart[],
  tool_call_id?: string,
  tool_calls?: ToolCall[],
  /** 是否跳过渲染 */
  skip_render?: boolean,
  /** gemini3 thought_signature */
  signature?: string,
  metadata?: {
    [key: string | number]: string
  },
}
export enum Role {
  System = "system",
  User = "user",
  Assistant = "assistant",
  Tool = "tool",
  Placeholder = "placeholder",
}
export interface ContentPart {
  type?: ContentType,
  text?: string,
  image_url?: ImageURL,
  video_url?: VideoURL,
  media_config?: MediaConfig,
  /** gemini3 thought_signature */
  signature?: string,
}
export enum ContentType {
  Text = "text",
  ImageURL = "image_url",
  VideoURL = "video_url",
  MultiPartVariable = "multi_part_variable",
}
export interface ImageURL {
  uri?: string,
  url?: string,
}
export interface VideoURL {
  url?: string,
  uri?: string,
}
export interface MediaConfig {
  fps?: number
}
export interface ToolCall {
  index?: string,
  id?: string,
  type?: ToolType,
  function_call?: FunctionCall,
  /** gemini3 thought_signature */
  signature?: string,
}
export interface FunctionCall {
  name?: string,
  arguments?: string,
}
export interface Label {
  key?: string
}
export interface VariableDef {
  key?: string,
  desc?: string,
  type?: VariableType,
  type_tags?: string[],
}
export interface VariableVal {
  key?: string,
  value?: string,
  placeholder_messages?: Message[],
  multi_part_values?: ContentPart[],
}
export enum VariableType {
  String = "string",
  Boolean = "boolean",
  Integer = "integer",
  Float = "float",
  Object = "object",
  Array_String = "array<string>",
  Array_Boolean = "array<boolean>",
  Array_Integer = "array<integer>",
  Array_Float = "array<float>",
  Array_Object = "array<object>",
  Placeholder = "placeholder",
  MultiPart = "multi_part",
}
export interface TokenUsage {
  input_tokens?: string,
  output_tokens?: string,
}
export interface DebugContext {
  debug_core?: DebugCore,
  debug_config?: DebugConfig,
  compare_config?: CompareConfig,
}
export interface DebugCore {
  mock_contexts?: DebugMessage[],
  mock_variables?: VariableVal[],
  mock_tools?: MockTool[],
}
export interface CompareConfig {
  groups?: CompareGroup[]
}
export interface CompareGroup {
  prompt_detail?: PromptDetail,
  debug_core?: DebugCore,
}
export interface DebugMessage {
  role?: Role,
  content?: string,
  reasoning_content?: string,
  parts?: ContentPart[],
  tool_call_id?: string,
  tool_calls?: DebugToolCall[],
  /** gemini3 thought_signature */
  signature?: string,
  debug_id?: string,
  input_tokens?: string,
  output_tokens?: string,
  cost_ms?: string,
}
export interface DebugToolCall {
  tool_call?: ToolCall,
  mock_response?: string,
  debug_trace_key?: string,
}
export interface MockTool {
  name?: string,
  mock_response?: string,
}
export interface DebugConfig {
  single_step_debug?: boolean
}
export interface DebugLog {
  id?: string,
  prompt_id?: string,
  workspace_id?: string,
  prompt_key?: string,
  version?: string,
  input_tokens?: string,
  output_tokens?: string,
  cost_ms?: string,
  status_code?: number,
  debugged_by?: string,
  debug_id?: string,
  debug_step?: number,
  started_at?: string,
  ended_at?: string,
}
export enum Scenario {
  Default = "default",
  EvalTarget = "eval_target",
}
export interface OverridePromptParams {
  model_config?: ModelConfig
}
export interface PromptCommitVersions {
  id?: string,
  workspace_id?: string,
  prompt_key?: string,
  prompt_basic?: PromptBasic,
  commit_versions?: string[],
}