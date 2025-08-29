import * as dataset from './../../data/domain/dataset';
export { dataset };
export enum ContentType {
  Text = "Text",
  /** 空间 */
  Image = "Image",
  Audio = "Audio",
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
}
export interface AudioContent {
  audios?: Audio[]
}
export interface Audio {
  format?: string,
  url?: string,
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
export interface ArgsSchema {
  key?: string,
  support_content_types?: ContentType[],
  /** 序列化后的jsonSchema字符串，例如："{\"type\": \"object\", \"properties\": {\"name\": {\"type\": \"string\"}, \"age\": {\"type\": \"integer\"}, \"isStudent\": {\"type\": \"boolean\"}}, \"required\": [\"name\", \"age\", \"isStudent\"]}" */
  json_schema?: string,
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