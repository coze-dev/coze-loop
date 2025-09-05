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
  ext_infos?: {
    [key: string | number]: string
  },
}
export interface PromptTemplate {
  template_type?: TemplateType,
  messages?: Message[],
  variable_defs?: VariableDef[],
}
export enum TemplateType {
  Normal = "normal",
  Jinja2 = "jinja2",
}
export interface Tool {
  type?: ToolType,
  function?: Function,
}
export enum ToolType {
  Function = "function",
}
export interface Function {
  name?: string,
  description?: string,
  parameters?: string,
}
export interface ToolCallConfig {
  tool_choice?: ToolChoiceType
}
export enum ToolChoiceType {
  None = "none",
  Auto = "auto",
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
}
export interface Message {
  role?: Role,
  reasoning_content?: string,
  content?: string,
  parts?: ContentPart[],
  tool_call_id?: string,
  tool_calls?: ToolCall[],
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
}
export enum ContentType {
  Text = 'text',
  ImageURL = 'image_url',
  MultiPartVariable = 'multi_part',
}
export interface ImageURL {
  uri?: string,
  url?: string,
}
export interface ToolCall {
  index?: string,
  id?: string,
  type?: ToolType,
  function_call?: FunctionCall,
}
export interface FunctionCall {
  name?: string,
  arguments?: string,
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
