// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as common from './common';
export { common };
import * as dataset from './../../data/domain/dataset';
export { dataset };
export interface EvaluationSet {
  /** 主键&外键 */
  id?: string,
  app_id?: number,
  workspace_id?: string,
  /** 基础信息 */
  name?: string,
  description?: string,
  status?: dataset.DatasetStatus,
  /** 规格限制 */
  spec?: dataset.DatasetSpec,
  /** 功能开关 */
  features?: dataset.DatasetFeatures,
  /** 数据条数 */
  item_count?: string,
  /** 是否有未提交的修改 */
  change_uncommitted?: boolean,
  /** 业务分类 */
  biz_category?: BizCategory,
  /**
   * 版本信息
   * 版本详情信息
  */
  evaluation_set_version?: EvaluationSetVersion,
  /** 最新的版本号 */
  latest_version?: string,
  /** 下一个的版本号 */
  next_version_num?: string,
  /** 系统信息 */
  base_info?: common.BaseInfo,
}
export interface EvaluationSetVersion {
  /** 主键&外键 */
  id?: string,
  app_id?: number,
  workspace_id?: string,
  evaluation_set_id?: string,
  /**
   * 版本信息
   * 展示的版本号，SemVer2 三段式
  */
  version?: string,
  /** 后端记录的数字版本号，从 1 开始递增 */
  version_num?: string,
  /** 版本描述 */
  description?: string,
  /** schema */
  evaluation_set_schema?: EvaluationSetSchema,
  /** 数据条数 */
  item_count?: string,
  /** 系统信息 */
  base_info?: common.BaseInfo,
}
/** EvaluationSetSchema 评测集 Schema，包含字段的类型限制等信息 */
export interface EvaluationSetSchema {
  /** 主键&外键 */
  id?: string,
  app_id?: number,
  workspace_id?: string,
  evaluation_set_id?: string,
  /** 数据集字段约束 */
  field_schemas?: FieldSchema[],
  /** 系统信息 */
  base_info?: common.BaseInfo,
}
export interface FieldSchema {
  /** 唯一键 */
  key?: string,
  /** 展示名称 */
  name?: string,
  /** 描述 */
  description?: string,
  /** 类型，如 文本，图片，etc. */
  content_type?: common.ContentType,
  /** 默认渲染格式，如 code, json, etc.mai */
  default_display_format?: dataset.FieldDisplayFormat,
  /** 当前列的状态 */
  status?: dataset.FieldStatus,
  /** 是否必填 */
  isRequired?: boolean,
  /** 对应的内置 schema */
  schema_key?: dataset.SchemaKey,
  /**
   * [20,50) 内容格式限制相关
   * 文本内容格式限制，格式为 JSON schema，协议参考 https://json-schema.org/specification
  */
  text_schema?: string,
  /** 多模态规格限制 */
  multi_model_spec?: dataset.MultiModalSpec,
  /** 用户是否不可见 */
  hidden?: boolean,
  /** 默认的预置转换配置，目前在数据校验后执行 */
  default_transformations?: dataset.FieldTransformationConfig[],
}
export interface EvaluationSetItem {
  /**
   * 主键&外键
   * 主键，随版本变化
  */
  id?: string,
  app_id?: number,
  workspace_id?: string,
  evaluation_set_id?: string,
  schema_id?: string,
  /** 数据在当前数据集内的唯一 ID，不随版本发生改变 */
  item_id?: string,
  /** 数据插入的幂等 key */
  item_key?: string,
  /** 轮次数据内容 */
  turns?: Turn[],
  /** 系统信息 */
  base_info?: common.BaseInfo,
}
export interface Turn {
  /** 轮次ID，如果是单轮评测集，id=0 */
  id?: string,
  /** 字段数据 */
  field_data_list?: FieldData[],
}
export interface FieldData {
  key?: string,
  name?: string,
  content?: common.Content,
}
export enum BizCategory {
  FromOnlineTrace = "from_online_trace",
}