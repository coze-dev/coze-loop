namespace go stone.fornax.ml_flow.domain.filter

typedef string QueryType
const QueryType QueryType_Match = "match"
const QueryType QueryType_NotMatch = "not_match"
const QueryType QueryType_Eq = "eq"
const QueryType QueryType_NotEq = "not_eq"
const QueryType QueryType_Lte= "lte"
const QueryType QueryType_Gte = "gte"
const QueryType QueryType_Lt = "lt"
const QueryType QueryType_Gt = "gt"
const QueryType QueryType_Exist = "exist"
const QueryType QueryType_NotExist = "not_exist"
const QueryType QueryType_In = "in"
const QueryType QueryType_NotIn = "not_in"
const QueryType QueryType_IsNull = "is_null"
const QueryType QueryType_NotNull = "not_null"

typedef string QueryRelation
const QueryRelation QueryRelation_And = "and"
const QueryRelation QueryRelation_Or = "or"

typedef string FieldType
const FieldType FieldType_String = "string"
const FieldType FieldType_Long = "long"
const FieldType FieldType_Double = "double"
const FieldType FieldType_Bool = "bool"
const FieldType FieldType_Float = "float"
const FieldType FieldType_Tag = "tag"
const FieldType FieldType_Integer = "integer"




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

