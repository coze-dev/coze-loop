namespace go coze.loop.observability.openapi

include "../../../base.thrift"
include "./domain/annotation.thrift"
include "./domain/span.thrift"
include "./domain/common.thrift"
include "./domain/filter.thrift"

struct IngestTracesRequest {
    1: optional list<span.InputSpan> spans (api.body='spans')

    255: optional base.Base Base
}

struct IngestTracesResponse {
    1: optional i32      code
    2: optional string   msg

    255: base.BaseResp     BaseResp
}

struct OtelIngestTracesRequest {
    1: required binary body (api.body="body", agw.source="raw_body"),
    2: required string content_type (api.header="Content-Type", agw.source="header"),
    3: required string content_encoding (api.header="Content-Encoding", agw.source="header"),
    4: required string workspace_id (api.header="cozeloop-workspace-id", agw.source="header"),

    255: optional base.Base Base
}

struct OtelIngestTracesResponse {
    1: optional binary   body         (api.body="body")
    2: optional string   content_type (api.header = "Content-Type")

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

struct SearchTraceOApiRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id" vt.gt="0")
    2: optional string logid (api.body="logid")
    3: optional string trace_id (api.body='trace_id')
    4: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.body="start_time") // ms
    5: required i64 end_time (api.js_conv='true', go.tag='json:"end_time"', api.body="end_time") // ms
    6: required i32 limit (api.body="limit")
    8: optional common.PlatformType platform_type (api.body="platform_type")

    255: optional base.Base Base
}

struct SearchTraceOApiResponse {
    1: optional i32 code (api.body = "code")
    2: optional string msg  (api.body = "msg")
    3: optional SearchTraceOApiData data (api.body = "data")

    255: optional base.BaseResp BaseResp
}

struct SearchTraceOApiData {
    1: required list<span.OutputSpan> spans
}

struct ListSpansOApiRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id" vt.gt="0")
    2: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.body="start_time") // ms
    3: required i64 end_time (api.js_conv='true', go.tag='json:"end_time"', api.body="end_time")  // ms
    4: optional filter.FilterFields filters (api.body="filters")
    5: optional i32 page_size (api.body="page_size")
    6: optional list<common.OrderBy> order_bys (api.body="order_bys")
    7: optional string page_token (api.body="page_token")
    8: optional common.PlatformType platform_type (api.body="platform_type")
    9: optional common.SpanListType span_list_type (api.body="span_list_type")

    255: optional base.Base Base
}

struct ListSpansOApiResponse {
    1: optional i32 code (api.body = "code")
    2: optional string msg  (api.body = "msg")
    3: optional ListSpansOApiData data (api.body = "data")

    255: optional base.BaseResp BaseResp
}

struct ListSpansOApiData {
    1: required list<span.OutputSpan> spans
    2: required string next_page_token
    3: required bool has_more
}

service OpenAPIService {
    IngestTracesResponse IngestTraces(1: IngestTracesRequest req) (api.post = '/v1/loop/traces/ingest')
    OtelIngestTracesResponse OtelIngestTraces(1: OtelIngestTracesRequest req) (api.post = '/v1/loop/opentelemetry/v1/traces')
    SearchTraceOApiResponse SearchTraceOApi(1: SearchTraceOApiRequest req) (api.post = '/v1/loop/traces/search')
    ListSpansOApiResponse ListSpansOApi(1: ListSpansOApiRequest req) (api.post = '/v1/loop/spans/search')
    CreateAnnotationResponse CreateAnnotation(1: CreateAnnotationRequest req)
    DeleteAnnotationResponse DeleteAnnotation(1: DeleteAnnotationRequest req)
}