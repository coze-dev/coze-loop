// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as manage from './../../llm/domain/manage';
export { manage };
import * as dataset from './../../data/domain/dataset';
export { dataset };
export enum ContentType {
  Text = "Text",
  /** 空间 */
  Image = "Image",
  Audio = "Audio",
  Video = "Video",
  MultiPart = "MultiPart",
  MultiPartVariable = "multi_part_variable",
}
export interface Content {
  content_type?: ContentType,
  format?: dataset.FieldDisplayFormat,
  text?: string,
  image?: Image,
  multi_part?: Content[],
  audio?: Audio,
  video?: Video,
  /**
   * 超大文本相关字段
   * 当前列的数据是否省略, 如果此处返回 true, 需要通过 GetDatasetItemField 获取当前列的具体内容, 或者是通过 omittedDataStorage.url 下载
  */
  content_omitted?: boolean,
  /** 被省略数据的完整信息，批量返回时会签发相应的 url，用户可以点击下载. 同时支持通过该字段传入已经上传好的超长数据(dataOmitted 为 true 时生效) */
  full_content?: dataset.ObjectStorage,
  /** 超长数据完整内容的大小，单位 byte */
  full_content_bytes?: number,
}
export interface AudioContent {
  audios?: Audio[]
}
export interface Video {
  name?: string,
  url?: string,
  uri?: string,
  thumb_url?: string,
  /** 当前多模态附件存储的 provider. 如果为空，则会从对应的 url 下载文件并上传到默认的存储中，并填充uri */
  storage_provider?: dataset.StorageProvider,
}
export interface Audio {
  format?: string,
  url?: string,
  name?: string,
  uri?: string,
  /** 当前多模态附件存储的 provider. 如果为空，则会从对应的 url 下载文件并上传到默认的存储中，并填充uri */
  storage_provider?: dataset.StorageProvider,
}
export interface Image {
  name?: string,
  url?: string,
  uri?: string,
  thumb_url?: string,
  /** 当前多模态附件存储的 provider. 如果为空，则会从对应的 url 下载文件并上传到默认的存储中，并填充uri */
  storage_provider?: dataset.StorageProvider,
}
export interface OrderBy {
  field?: string,
  is_asc?: boolean,
  /** 用于区分当前字段是否是 field key，仅在评测集场景下生效 */
  is_field_key?: boolean,
}
export enum Role {
  System = 1,
  User = 2,
  Assistant = 3,
  Tool = 4,
}
export interface Message {
  role?: Role,
  content?: Content,
  ext?: {
    [key: string | number]: string
  },
}
export enum ArgSchemaTextType {
  Trajectory = 1,
}
export const ArgSchemaKey_ActualOutput = "actual_output";
export const ArgSchemaKey_Trajectory = "trajectory";
export interface ArgsSchema {
  key?: string,
  support_content_types?: ContentType[],
  /** 序列化后的jsonSchema字符串，例如："{\"type\": \"object\", \"properties\": {\"name\": {\"type\": \"string\"}, \"age\": {\"type\": \"integer\"}, \"isStudent\": {\"type\": \"boolean\"}}, \"required\": [\"name\", \"age\", \"isStudent\"]}" */
  json_schema?: string,
  default_value?: Content,
  text_type?: ArgSchemaTextType,
}
export interface UserInfo {
  /** 姓名 */
  name?: string,
  /** 英文名称 */
  en_name?: string,
  /** 用户头像url */
  avatar_url?: string,
  /** 72 * 72 头像 */
  avatar_thumb?: string,
  /** 用户应用内唯一标识 */
  open_id?: string,
  /** 用户应用开发商内唯一标识 */
  union_id?: string,
  /** 用户在租户内的唯一标识 */
  user_id?: string,
  /** 用户邮箱 */
  email?: string,
}
export interface BaseInfo {
  created_by?: UserInfo,
  updated_by?: UserInfo,
  created_at?: string,
  updated_at?: string,
  deleted_at?: string,
}
/** 评测模型配置 */
export interface ModelConfig {
  /** 模型id */
  model_id?: string,
  /** 模型名称 */
  model_name?: string,
  temperature?: number,
  max_tokens?: number,
  top_p?: number,
  protocol?: manage.Protocol,
  identification?: string,
  preset_model?: boolean,
  json_ext?: string,
}
export interface Session {
  user_id?: number,
  app_id?: number,
}
export interface RuntimeParam {
  json_value?: string,
  json_demo?: string,
}
export interface RateLimit {
  rate?: number,
  burst?: number,
  period?: string,
}
export enum AgentType {
  Vibe = "vibe",
}
export interface AgentConfig {
  /** Agent type */
  agent_type?: AgentType
}
export interface SkillConfig {
  /** skill id */
  skill_id?: string,
  /** skill version */
  version?: string,
}