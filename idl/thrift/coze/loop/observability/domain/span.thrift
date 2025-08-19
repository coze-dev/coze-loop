namespace go coze.loop.observability.domain.span

include "annotation.thrift"

typedef string SpanStatus (ts.enum="true")
const SpanStatus SpanStatus_Success = "success"
const SpanStatus SpanStatus_Error = "error"
const SpanStatus SpanStatus_Broken = "broken"

typedef string SpanType (ts.enum="true")
const SpanType SpanType_Unknown = "unknwon"
const SpanType SpanType_Prompt = "prompt"
const SpanType SpanType_Model = "model"

struct AttrTos {
    1: optional string input_data_url
    2: optional string output_data_url
    3: optional map<string, string> multimodal_data
}

struct OutputSpan {
    1: required string trace_id
    2: required string span_id
    3: required string parent_id
    4: required string span_name
    5: required string span_type
    6: required SpanType type
    7: required i64 started_at (api.js_conv='true', go.tag='json:"started_at"')
    8: required i64 duration (api.js_conv='true', go.tag='json:"duration"')
    9: required SpanStatus status
    10: required i32 status_code
    11: required string input
    12: required string output
    13: optional i64 logic_delete_date (api.js_conv='true', go.tag='json:"logic_delete_date"')

    101: optional map<string, string> custom_tags
    102: optional AttrTos attr_tos
    103: optional map<string, string> system_tags
    104: optional list<annotation.Annotation> annotations
}

struct InputSpan {
    1: required i64 started_at_micros (api.js_conv='true', go.tag='json:"started_at_micros"')
    3: required string span_id
    4: required string parent_id
    5: required string trace_id
    6: required i64 duration (api.js_conv='true', go.tag='json:"duration"')
    8: optional string call_type
    9: required string workspace_id
    10: required string span_name
    11: required string span_type
    12: required string method
    13: required i32  status_code
    14: required string input
    15: required string output
    16: optional string object_storage

    17: optional map<string, string> system_tags_string
    18: optional map<string, i64> system_tags_long (api.js_conv='true', go.tag='json:"system_tags_long"')
    19: optional map<string, double> system_tags_double

    20: optional map<string, string> tags_string
    21: optional map<string, i64> tags_long (api.js_conv='true', go.tag='json:"tags_long"')
    22: optional map<string, double> tags_double

    23: optional map<string, bool> tags_bool
    24: optional map<string, string> tags_bytes

    25: optional i64 duration_micros (api.js_conv='true', go.tag='json:"duration_micros"')
}