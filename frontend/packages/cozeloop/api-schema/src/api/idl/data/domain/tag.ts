import * as common from './common';
export { common };
export enum TagStatus {
  /** 标签状态 */
  Active = "active",
  /** 启用 */
  Inactive = "inactive",
  /** 禁用 */
  Deprecated = "deprecated",
}
/** 旧版本状态 */
export enum TagType {
  /** 标签类型 */
  Tag = "tag",
  /** 标签 */
  Option = "option",
}
/** 单选类型,不在标签管理中 */
export enum OperationType {
  /** 操作类型 */
  Create = "create",
  /** 创建 */
  Update = "update",
  /** 更新 */
  Delete = "delete",
}
/** 删除 */
export enum ChangeTargetType {
  /** 变更对象 */
  Tag = "tag",
  /** 标签 */
  TagName = "tag_name",
  /** 标签名 */
  TagDescription = "tag_description",
  /** 标签描述 */
  TagStatus = "tag_status",
  /** 标签状态 */
  TagType = "tag_type",
  /** 标签类型 */
  TagContentType = "tag_content_type",
  /** 标签内容类型 */
  TagValueName = "tag_value_name",
  /** 标签选项值 */
  TagValueStatus = "tag_value_status",
}
/** 标签选项状态 */
export enum TagDomainType {
  Data = "data",
  /** 数据基座 */
  Observe = "observe",
  /** 观测 */
  Evaluation = "evaluation",
}
/** 评测 */
export enum TagContentType {
  Categorical = "categorical",
  /** 分类标签 */
  Boolean = "boolean",
  /** 布尔标签 */
  ContinuousNumber = "continuous_number",
  /** 连续分支类型 */
  FreeText = "free_text",
}
/** 自由文本 */
export interface TagContentSpec {
  continuous_number_spec?: ContinuousNumberSpec
}
export interface ContinuousNumberSpec {
  min_value?: number,
  min_value_description?: string,
  max_value?: number,
  max_value_description?: string,
}
export interface TagInfo {
  id?: string,
  appID?: number,
  workspace_id?: string,
  /** 数字版本号 */
  version_num?: number,
  /** SemVer 三段式版本号 */
  version?: string,
  /** tag key id */
  tag_key_id?: string,
  /** tag key name */
  tag_key_name?: string,
  /** 描述 */
  description?: string,
  /** 状态，启用active、禁用inactive、弃用deprecated(最新版之前的版本的状态) */
  status?: TagStatus,
  /** 类型: tag: 标签管理中的标签类型; option: 临时单选类型 */
  tag_type?: TagType,
  parent_tag_key_id?: string,
  /** 标签值 */
  tag_values?: TagValue[],
  /** 变更历史 */
  change_logs?: ChangeLog[],
  /** 内容类型 */
  content_type?: TagContentType,
  /** 内容约束 */
  content_spec?: TagContentSpec,
  /** 应用领域 */
  domain_type_list?: TagDomainType[],
  /** 基础信息 */
  base_info?: common.BaseInfo,
}
export interface TagValue {
  /** 主键 */
  id?: string,
  app_id?: number,
  workspace_id?: string,
  /** tag_key_id */
  tag_key_id?: string,
  /** tag_value_id */
  tag_value_id?: string,
  /** 标签值 */
  tag_value_name?: string,
  /** 描述 */
  description?: string,
  /** 状态 */
  status?: TagStatus,
  /** 数字版本号 */
  version_num?: number,
  /** 父标签选项的ID */
  parent_tag_value_id?: string,
  /** 子标签 */
  children?: TagValue[],
  /** 是否是系统标签而非用户标签 */
  is_system?: boolean,
  /** 基础信息 */
  base_info?: common.BaseInfo,
}
export interface ChangeLog {
  /** 变更的属性 */
  target?: ChangeTargetType,
  /** 变更类型: create, update, delete */
  operation?: OperationType,
  /** 变更前的值 */
  before_value?: string,
  /** 变更后的值 */
  after_value?: string,
  /** 变更属性的值：如果是标签选项变更，该值为变更属选项值名字 */
  target_value?: string,
}