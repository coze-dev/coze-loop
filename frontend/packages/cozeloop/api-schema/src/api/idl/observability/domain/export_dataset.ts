import * as common from './common';
export { common };
import * as dataset from './../../data/domain/dataset';
export { dataset };
export enum ExportType {
  Append = "append",
  Overwrite = "overwrite",
}
export enum ItemStatus {
  Success = "success",
  Error = "error",
}
/** DatasetSchema 数据集 Schema，包含字段的类型限制等信息 */
export interface DatasetSchema {
  /** 数据集字段约束 */
  field_schemas?: FieldSchema[]
}
export interface FieldSchema {
  /** 数据集 schema 版本变化中 key 唯一，新建时自动生成，不需传入 */
  key?: string,
  /** 展示名称 */
  name?: string,
  /** 描述 */
  description?: string,
  /** 类型，如 文本，图片，etc. */
  content_type?: common.ContentType,
  /** 默认渲染格式，如 code, json, etc. */
  default_format?: dataset.FieldDisplayFormat,
  /**
   * [20,50) 内容格式限制相关
   * 文本内容格式限制，格式为 JSON schema，协议参考 https://json-schema.org/specification
  */
  text_schema?: string,
}
export interface Item {
  status: ItemStatus,
  /** todo 多模态需要修改 */
  field_list?: FieldData[],
  /** 错误信息 */
  errors?: ItemError[],
}
export interface FieldData {
  key?: string,
  name?: string,
  content?: Content,
}
export interface Content {
  content_type?: common.ContentType,
  text?: string,
  /** 图片内容 */
  image?: Image,
  /** 图文混排时，图文内容 */
  multi_part?: Content[],
}
export interface Image {
  name?: string,
  url?: string,
}
export interface ItemError {
  type?: dataset.ItemErrorType,
  /** 有错误的字段名，非必填 */
  field_names?: string[],
}
export interface FieldMapping {
  /** 数据集字段约束 */
  field_schema: FieldSchema,
  trace_field_key: string,
  trace_field_jsonpath: string,
}