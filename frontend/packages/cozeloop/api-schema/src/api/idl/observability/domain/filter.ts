import * as common from './common';
export { common };
export enum QueryType {
  Match = "match",
  Eq = "eq",
  NotEq = "not_eq",
  Lte = "lte",
  Gte = "gte",
  Lt = "lt",
  Gt = "gt",
  Exist = "exist",
  NotExist = "not_exist",
  In = "in",
  not_In = "not_in",
  NotMatch = "not_match",
}
export enum QueryRelation {
  And = "and",
  Or = "or",
}
export enum FieldType {
  String = "string",
  Long = "long",
  Double = "double",
  Bool = "bool",
}
export type TaskFieldName = string;
export const TaskFieldName_TaskStatus = "task_status";
export const TaskFieldName_TaskName = "task_name";
export const TaskFieldName_TaskType = "task_type";
export const TaskFieldName_SampleRate = "sample_rate";
export const TaskFieldName_CreatedBy = "created_by";
export interface FilterFields {
  query_and_or?: QueryRelation,
  filter_fields: FilterField[],
}
export interface FilterField {
  field_name?: string,
  field_type?: FieldType,
  values?: string[],
  query_type?: QueryType,
  query_and_or?: QueryRelation,
  sub_filter?: FilterFields,
  is_custom?: boolean,
  extra_info?: {
    [key: string | number]: string
  },
}
export interface FieldOptions {
  i64_list?: string[],
  f64_list?: number[],
  string_list?: string[],
}
export interface TaskFilterFields {
  query_and_or?: QueryRelation,
  filter_fields: TaskFilterField[],
}
export interface TaskFilterField {
  field_name?: TaskFieldName,
  field_type?: FieldType,
  values?: string[],
  query_type?: QueryType,
  query_and_or?: QueryRelation,
  sub_filter?: TaskFilterField,
}
export interface SpanFilterFields {
  /** Span 过滤条件 */
  filters?: FilterFields,
  /** 平台类型，不填默认是fornax */
  platform_type?: common.PlatformType,
  /** 查询的 span 标签页类型，不填默认是 root span */
  span_list_type?: common.SpanListType,
}