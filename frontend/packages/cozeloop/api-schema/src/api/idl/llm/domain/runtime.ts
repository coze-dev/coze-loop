import * as manage from './manage';
export { manage };
import * as common from './common';
export { common };
export interface ModelConfig {
  /** 模型id */
  model_id: string,
  temperature?: number,
  max_tokens?: string,
  top_p?: number,
  stop?: string[],
  tool_choice?: ToolChoice,
  /** support json */
  response_format?: ResponseFormat,
  top_k?: number,
  presence_penalty?: number,
  frequency_penalty?: number,
  /** 与ParamSchema对应 */
  param_config_values?: ParamConfigValue[],
}
export interface ParamConfigValue {
  /** 传给下游模型的key，与ParamSchema.name对齐 */
  name?: string,
  /** 展示名称 */
  label?: string,
  /** 传给下游模型的value，与ParamSchema.options对齐 */
  value?: manage.ParamOption,
}
export interface Message {
  role: Role,
  content?: string,
  multimodal_contents?: ChatMessagePart[],
  /** only for AssistantMessage */
  tool_calls?: ToolCall[],
  /** only for ToolMessage */
  tool_call_id?: string,
  /** collects meta information about a chat response */
  response_meta?: ResponseMeta,
  /**
   * only for AssistantMessage, And when reasoning_content is not empty, content must be empty
   * 8: optional map<string,string> extra
  */
  reasoning_content?: string,
}
export interface ChatMessagePart {
  type?: ChatMessagePartType,
  text?: string,
  image_url?: ChatMessageImageURL,
  /**
   * 4: optional ChatMessageAudioURL audio_url 占位,暂不支持
   * 6: optional ChatMessageFileURL file_url 占位,暂不支持
  */
  video_url?: ChatMessageVideoURL,
}
export interface ChatMessageVideoURL {
  url?: string,
  detail?: VideoURLDetail,
  mime_type?: string,
}
export interface VideoURLDetail {
  fps?: number
}
export interface ChatMessageImageURL {
  url?: string,
  detail?: ImageURLDetail,
  mime_type?: string,
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
export interface ResponseMeta {
  finish_reason?: string,
  /** 3: optional LogProbs log_probs */
  usage?: TokenUsage,
}
export interface TokenUsage {
  prompt_tokens?: string,
  completion_tokens?: string,
  total_tokens?: string,
}
export interface Tool {
  name?: string,
  desc?: string,
  def_type?: ToolDefType,
  /** 必须使用openapi3.Schema序列化后的json */
  def?: string,
}
export interface BizParam {
  workspace_id?: string,
  user_id?: string,
  /** 使用场景 */
  scenario?: common.Scenario,
  /** 场景实体id(非必填) */
  scenario_entity_id?: string,
  /** 场景实体version(非必填) */
  scenario_entity_version?: string,
  /** 场景实体key(非必填), prompt场景需要传prompt key */
  scenario_entity_key?: string,
}
export interface ResponseFormat {
  type?: ResponseFormatType
}
export type ResponseFormatType = string;
export const response_format_json_object = "json_object";
export const response_format_text = "text";
export enum ToolChoice {
  tool_choice_auto = "auto",
  tool_choice_required = "required",
  tool_choice_none = "none",
}
export enum ToolDefType {
  tool_def_type_open_api_v3 = "open_api_v3",
}
export enum Role {
  role_system = "system",
  role_assistant = "assistant",
  role_user = "user",
  role_tool = "tool",
}
export enum ToolType {
  tool_type_function = "function",
}
export enum ChatMessagePartType {
  chat_message_part_type_text = "text",
  chat_message_part_type_image_url = "image_url",
  /** const ChatMessagePartType chat_message_part_type_audio_url = "audio_url" */
  chat_message_part_type_video_url = "video_url",
}
/** const ChatMessagePartType chat_message_part_type_file_url = "file_url" */
export enum ImageURLDetail {
  image_url_detail_auto = "auto",
  image_url_detail_low = "low",
  image_url_detail_high = "high",
}
export enum MimeTypePrefix {
  mime_prefix_image = "image/",
  mime_prefix_video = "video/",
}