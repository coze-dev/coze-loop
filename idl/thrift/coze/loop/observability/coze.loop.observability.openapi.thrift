namespace go coze.loop.observability.openapi

include "../../../base.thrift"
include "./domain/annotation.thrift"
include "./domain/span.thrift"

struct IngestTracesRequest {
    1: optional list<span.InputSpan> spans (api.body='spans')

    255: optional base.Base Base
}

struct IngestTracesResponse {
    1: optional i32      code
    2: optional string   msg

    255: base.BaseResp     BaseResp
}

struct CreateAnnotationRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id" vt.gt="0")
    2: required string span_id (api.body="span_id", vt.min_size="1")
    3: required string trace_id (api.body="trace_id", vt.min_size="1")
    4: required string annotation_key (api.body="annotation_key", vt.min_size="1")
    5: required string annotation_value (api.body="annotation_value")
    6: optional annotation.ValueType annotation_value_type (api.body="annotation_value_type")
    7: optional string reasoning (api.body="reasoning")

    255: optional base.Base Base
}

struct CreateAnnotationResponse {
    255: optional base.BaseResp BaseResp
}

struct DeleteAnnotationRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id" vt.gt="0")
    2: required string span_id (api.query='span_id', vt.min_size="1")
    4: required string trace_id (api.query="trace_id", vt.min_size="1")
    3: required string annotation_key (api.query='annotation_key', vt.min_size="1")

    255: optional base.Base Base
}

struct DeleteAnnotationResponse {
    255: optional base.BaseResp BaseResp
}

service OpenAPIService {
    IngestTracesResponse IngestTraces(1: IngestTracesRequest req) (api.post = '/v1/loop/traces/ingest')
    CreateAnnotationResponse CreateAnnotation(1: CreateAnnotationRequest req)
    DeleteAnnotationResponse DeleteAnnotation(1: DeleteAnnotationRequest req)
}