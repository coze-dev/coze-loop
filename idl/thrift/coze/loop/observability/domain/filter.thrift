namespace go coze.loop.observability.domain.filter

typedef string QueryType (ts.enum="true")
const QueryType QueryType_Match = "match"
const QueryType QueryType_Eq = "eq"
const QueryType QueryType_NotEq = "not_eq"
const QueryType QueryType_Lte= "lte"
const QueryType QueryType_Gte = "gte"
const QueryType QueryType_Lt = "lt"
const QueryType QueryType_Gt = "gt"
const QueryType QueryType_Exist = "exist"
const QueryType QueryType_NotExist = "not_exist"
const QueryType QueryType_In = "in"
const QueryType QueryType_not_In = "not_in"
const QueryType QueryType_NotMatch = "not_match"

typedef string QueryRelation (ts.enum="true")
const QueryRelation QueryRelation_And = "and"
const QueryRelation QueryRelation_Or = "or"

typedef string FieldType (ts.enum="true")
const FieldType FieldType_String = "string"
const FieldType FieldType_Long = "long"
const FieldType FieldType_Double = "double"
const FieldType FieldType_Bool = "bool"

struct FilterFields {
    1: optional QueryRelation query_and_or
    2: required list<FilterField> filter_fields
}

struct FilterField {
    1: optional string field_name
    2: optional FieldType field_type
    3: optional list<string> values
    4: optional QueryType query_type
    5: optional QueryRelation query_and_or
    6: optional FilterFields sub_filter
    7: optional bool is_custom
}

struct FieldOptions {
    2: optional list<i64> i64_list (api.js_conv='true', go.tag='json:"i64_list"')
    3: optional list<double> f64_list
    4: optional list<string> string_list
}
