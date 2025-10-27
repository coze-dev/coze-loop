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
}
export interface FieldOptions {
  i64_list?: string[],
  f64_list?: number[],
  string_list?: string[],
}