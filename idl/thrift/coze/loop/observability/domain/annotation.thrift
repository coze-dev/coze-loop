namespace go coze.loop.observability.domain.annotation

include "common.thrift"

typedef string AnnotationType (ts.enum="true")
const AnnotationType AnnotationType_AutoEvaluate = "auto_evaluate"
const AnnotationType AnnotationType_EvaluationSet = "manual_evaluation_set"
const AnnotationType AnnotationType_ManualFeedback = "manual_feedback"
const AnnotationType AnnotationType_CozeFeedback = "coze_feedback"

typedef string ValueType (ts.enum="true")
const ValueType ValueType_String = "string"
const ValueType ValueType_Long = "long"
const ValueType ValueType_Double = "double"
const ValueType ValueType_Bool = "bool"

struct Correction {
    1: optional double score
    2: optional string explain
    100: optional common.BaseInfo base_info
}

struct EvaluatorResult {
    1: optional double score
    2: optional Correction correction
    3: optional string reasoning
}

struct AutoEvaluate {
    1: required i64 evaluator_version_id (api.js_conv='true', go.tag='json:"evaluator_version_id"')
    2: required string evaluator_name
    3: required string evaluator_version
    4: optional EvaluatorResult evaluator_result
    5: required i64 record_id (api.js_conv='true', go.tag='json:"record_id"')
    6: required string task_id
}

struct ManualFeedback {
    1: required i64 tag_key_id (api.js_conv='true', go.tag='json:"tag_key_id"')
    2: required string tag_key_name
    3: optional i64 tag_value_id (api.js_conv='true', go.tag='json:"tag_value_id"')
    4: optional string tag_value
}

struct Annotation {
    1: optional string id
    2: optional string span_id (vt.min_size="1", vt.not_nil="true")
    3: optional string trace_id (vt.min_size="1", vt.not_nil="true")
    4: optional string workspace_id (vt.min_size="1", vt.not_nil="true")
    5: optional i64 start_time (api.js_conv='true', go.tag='json:"start_time"', vt.gt="0", vt.not_nil="true")
    6: optional AnnotationType type
    7: optional string key (vt.min_size="1", vt.not_nil="true")
    8: optional ValueType value_type
    9: optional string value
    10: optional string status
    11: optional string reasoning

    100: optional common.BaseInfo base_info
    101: optional AutoEvaluate auto_evaluate
    102: optional ManualFeedback manual_feedback
}