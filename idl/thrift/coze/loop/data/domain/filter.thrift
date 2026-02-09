namespace go stone.fornax.ml_flow.domain.filter

typedef string QueryType
const QueryType query_type_match = "match"
const QueryType query_type_not_match = "not_match"
const QueryType query_type_eq = "eq"
const QueryType query_type_not_eq = "not_eq"
const QueryType query_type_lte= "lte"
const QueryType query_type_gte = "gte"
const QueryType query_type_lt = "lt"
const QueryType query_type_gt = "gt"
const QueryType query_type_exist = "exist"
const QueryType query_type_not_exist = "not_exist"
const QueryType query_type_in = "in"
const QueryType query_type_not_in = "not_in"
const QueryType query_type_is_null = "is_null"
const QueryType query_type_not_null = "not_null"

typedef string QueryRelation
const QueryRelation query_relation_and = "and"
const QueryRelation query_relation_or = "or"

typedef string FieldType
const FieldType field_type_string = "string"
const FieldType field_type_long = "long"
const FieldType field_type_double = "double"
const FieldType field_type_bool = "bool"
const FieldType field_type_float = "float"
const FieldType field_type_tag = "tag"
const FieldType field_type_integer = "integer"




struct FilterField {
  1: required string field_name,
  2: required FieldType field_type,
  3: optional list<string> values,
  4: optional QueryType query_type,
  5: optional QueryRelation query_and_or,
  6: optional Filter sub_filter
}

struct Filter {
  1: optional QueryRelation query_and_or,
  2: required list<FilterField> filter_fields
}

struct FieldOptions {
    1: optional list<i32> i32_field_option (agw.key = "i32")
    2: optional list<i64> i64_field_option (agw.js_conv = "str" agw.key = "i64")
    3: optional list<double> f64_field_option (agw.key = "f64")
    4: optional list<string> string_field_option (agw.key = "string")
    5: optional list<ObjectFieldOption> obj_field_option (agw.key = "obj")
}

struct ObjectFieldOption {
    1: required i64 id
    2: required string display_name
}

struct FieldMeta {
    // 字段类型
    1: required FieldType field_type
    // 当前字段支持的操作类型
    2: required list<QueryType> query_types
    3: required string display_name
    // 支持的可选项
    4: optional FieldOptions field_options

    5: optional bool exist  // 当前字段在schema中是否存在
}

struct FieldMetaInfoData {
    // 字段元信息
    1: required map<string, FieldMeta> field_metas
}

