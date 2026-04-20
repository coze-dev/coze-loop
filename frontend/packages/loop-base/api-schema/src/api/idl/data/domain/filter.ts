// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export enum QueryType {
  Match = "match",
  NotMatch = "not_match",
  Eq = "eq",
  NotEq = "not_eq",
  Lte = "lte",
  Gte = "gte",
  Lt = "lt",
  Gt = "gt",
  Exist = "exist",
  NotExist = "not_exist",
  In = "in",
  NotIn = "not_in",
  IsNull = "is_null",
  NotNull = "not_null",
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
  Float = "float",
  Tag = "tag",
  Integer = "integer",
}
export interface FilterField {
  field_name: string,
  field_type: FieldType,
  values?: string[],
  query_type?: QueryType,
  query_and_or?: QueryRelation,
  sub_filter?: Filter,
}
export interface Filter {
  query_and_or?: QueryRelation,
  filter_fields: FilterField[],
}
export interface FieldOptions {
  i32?: number[],
  i64?: string[],
  f64?: number[],
  string?: string[],
  obj?: ObjectFieldOption[],
}
export interface ObjectFieldOption {
  id: number,
  display_name: string,
}
export interface FieldMeta {
  /** 字段类型 */
  field_type: FieldType,
  /** 当前字段支持的操作类型 */
  query_types: QueryType[],
  display_name: string,
  /** 支持的可选项 */
  field_options?: FieldOptions,
  /** 当前字段在schema中是否存在 */
  exist?: boolean,
}
export interface FieldMetaInfoData {
  /** 字段元信息 */
  field_metas: {
    [key: string | number]: FieldMeta
  }
}