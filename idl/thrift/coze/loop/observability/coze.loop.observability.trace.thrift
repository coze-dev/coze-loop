namespace go coze.loop.observability.trace

include "../../../base.thrift"
include "./domain/span.thrift"
include "./domain/common.thrift"
include "./domain/filter.thrift"
include "./domain/view.thrift"
include "./domain/annotation.thrift"
include "./domain/task.thrift"

struct ListSpansRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id")
    2: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.body="start_time") // ms
    3: required i64 end_time (api.js_conv='true', go.tag='json:"end_time"', api.body="end_time")  // ms
    4: optional filter.FilterFields filters (api.body="filters")
    5: optional i32 page_size (api.body="page_size")
    6: optional list<common.OrderBy> order_bys (api.body="order_bys")
    7: optional string page_token (api.body="page_token")
    8: optional common.PlatformType platform_type (api.body="platform_type")
    9: optional common.SpanListType span_list_type (api.body="span_list_type") // default root span

    255: optional base.Base Base
}

struct ListSpansResponse {
    1: required list<span.OutputSpan> spans
    2: required string next_page_token
    3: required bool has_more

    255: optional base.BaseResp BaseResp
}

struct TokenCost {
    1: required i64 input (api.js_conv='true', go.tag='json:"input"')
    2: required i64 output (api.js_conv='true', go.tag='json:"output"')
}

struct TraceAdvanceInfo {
    1: required string trace_id
    2: required TokenCost tokens
}

struct GetTraceRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.query="workspace_id")
    2: required string trace_id (api.path="trace_id")
    3: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.query="start_time") // ms
    4: required i64 end_time (api.js_conv='true', go.tag='json:"end_time"', api.query="end_time") // ms
    8: optional common.PlatformType platform_type (api.query="platform_type")
    9: optional list<string> span_ids (api.query="span_ids")

    255: optional base.Base Base
}

struct GetTraceResponse {
    1: required list<span.OutputSpan> spans
    2: optional TraceAdvanceInfo traces_advance_info

    255: optional base.BaseResp BaseResp
}

struct TraceQueryParams {
    1: required string trace_id
    2: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"')
    3: required i64 end_time (api.js_conv='true', go.tag='json:"end_time"')
}

struct BatchGetTracesAdvanceInfoRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"',api.body='workspace_id')
    2: required list<TraceQueryParams> traces (api.body='traces')
    6: optional common.PlatformType platform_type (api.body='platform_type')

    255: optional base.Base Base
}

struct BatchGetTracesAdvanceInfoResponse {
    1: required list<TraceAdvanceInfo> traces_advance_info

    255: optional base.BaseResp BaseResp
}

struct IngestTracesRequest {
    1: optional list<span.InputSpan> spans (api.body='spans')

    255: optional base.Base Base
}

struct IngestTracesResponse {
    1: optional i32      code
    2: optional string   msg

    255: base.BaseResp     BaseResp
}

struct FieldMeta {
    1: required filter.FieldType value_type
    2: required list<filter.QueryType> filter_types
    3: optional filter.FieldOptions field_options
    4: optional bool support_customizable_option
}

struct GetTracesMetaInfoRequest {
    1: optional common.PlatformType platform_type (api.query='platform_type')
    2: optional common.SpanListType spanList_type (api.query='span_list_type')
    3: optional i64 workspace_id (api.js_conv='true',api.query='workspace_id') // required

    255: optional base.Base Base
}

struct GetTracesMetaInfoResponse {
    1: required map<string, FieldMeta> field_metas

    255: optional base.BaseResp BaseResp
}

struct CreateViewRequest {
    1: optional string enterprise_id (api.body="enterprise_id")
    2: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id")
    3: required string view_name (api.body="view_name")
    4: required common.PlatformType platform_type (api.body="platform_type")
    5: required common.SpanListType span_list_type (api.body="span_list_type")
    6: required string filters (api.body="filters")

    255: optional base.Base Base
}

struct CreateViewResponse {
    1: required i64 id (api.js_conv='true', go.tag='json:"id"', api.body="id")

    255: optional base.BaseResp BaseResp
}

struct UpdateViewRequest {
    1: required i64 id (api.js_conv='true', go.tag='json:"id"', api.path="view_id")
    2: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id")
    3: optional string view_name (api.body="view_name")
    4: optional common.PlatformType platform_type (api.body="platform_type")
    5: optional common.SpanListType span_list_type (api.body="span_list_type")
    6: optional string filters (api.body="filters")

    255: optional base.Base Base,
}

struct UpdateViewResponse {
    255: optional base.BaseResp BaseResp
}

struct DeleteViewRequest {
    1: required i64 id (api.path="view_id", api.js_conv='true', go.tag='json:"id"'),
    2: required i64 workspace_id (api.query='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"'),

    255: optional base.Base Base
}

struct DeleteViewResponse {
    255: optional base.BaseResp BaseResp
}

struct ListViewsRequest {
    1: optional string enterprise_id (api.body="enterprise_id")
    2: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id")
    3: optional string view_name (api.body="view_name")

    255: optional base.Base Base
}

struct ListViewsResponse {
    1: required list<view.View> views

    255: optional base.BaseResp BaseResp
}

struct CreateManualAnnotationRequest {
    1: required annotation.Annotation annotation (api.body="annotation")
    2: optional common.PlatformType platform_type (api.body="platform_type")

    255: optional base.Base Base
}

struct CreateManualAnnotationResponse {
    1: optional string annotation_id

    255: optional base.BaseResp BaseResp
}

struct UpdateManualAnnotationRequest {
    1: required string annotation_id (api.path="annotation_id")
    2: required annotation.Annotation annotation (api.body="annotation")
    3: optional common.PlatformType platform_type (api.body="platform_type")


    255: optional base.Base Base
}

struct UpdateManualAnnotationResponse {
    255: optional base.BaseResp BaseResp
}

struct DeleteManualAnnotationRequest {
    1: required string annotation_id (api.path="annotation_id")
    2: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.query="workspace_id", vt.gt="0")
    3: required string trace_id (api.query="trace_id", vt.min_size="1")
    4: required string span_id (api.query="span_id", vt.min_size="1")
    5: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.query="start_time", vt.gt="0")
    6: required string annotation_key (api.query="annotation_key", vt.min_size="1")
    7: optional common.PlatformType platform_type (api.query="platform_type")

    255: optional base.Base Base
}

struct DeleteManualAnnotationResponse {
    255: optional base.BaseResp BaseResp
}

struct ListAnnotationsRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: required string span_id (api.body="span_id", vt.min_size="1")
    3: required string trace_id (api.body="trace_id", vt.min_size="1")
    4: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.body="start_time", vt.gt="0")
    5: optional common.PlatformType platform_type (api.body="platform_type")
    6: optional bool desc_by_updated_at (api.body="desc_by_updated_at")

    255: optional base.Base Base
}

struct ListAnnotationsResponse {
    1: required list<annotation.Annotation> annotations

    255: optional base.BaseResp BaseResp
}

struct ChangeEvaluatorScoreRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: required i64 evaluator_record_id (api.query="evaluator_record_id", vt.gt="0")
    3: required string span_id (api.query="span_id", vt.min_size="1")
    4: required i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.query="start_time", vt.gt="0")
    5: required annotation.Correction correction (api.query="correction")

    255: optional base.Base Base
}

struct ChangeEvaluatorScoreResponse {
    1: required annotation.Annotation annotation

    255: optional base.BaseResp BaseResp
}

struct AnnotationEvaluator {
    1: required i64 evaluator_version_id,
    2: required string evaluator_name,
    3: required string evaluator_version,
}

struct ListAnnotationEvaluatorsRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: optional string name (api.query = "name")

    255: optional base.Base Base (api.none="true")
}

struct ListAnnotationEvaluatorsResponse {
    1: required list<AnnotationEvaluator> evaluators

    255: optional base.BaseResp BaseResp
}

struct ExtractSpanInfoRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: required string trace_id (api.query = "trace_id" vt.min_size = "1")
    3: required list<string> span_ids (api.query="span_ids", vt.min_size="1", vt.max_size="500")
    4: optional i64 start_time (api.js_conv='true', go.tag='json:"start_time"', api.query="start_time", vt.gt="0")
    5: optional i64 end_time (api.js_conv='true', go.tag='json:"end_time"', api.query="end_time", vt.gt="0")
    6: optional common.PlatformType platform_type (api.body="platform_type")
    7: optional list<task.FieldMapping> field_mappings (vt.min_size="1", vt.max_size="100")

    255: optional base.Base Base (api.none="true")
}

struct FieldData {
    1: optional string key,
    2: optional string name,
    3: optional Content content,
}
typedef string ContentType

const ContentType ContentType_Text = "Text" // 空间
const ContentType ContentType_Image = "Image"
const ContentType ContentType_Audio = "Audio"
const ContentType ContentType_MultiPart = "MultiPart"

struct Content {
    1: optional ContentType contentType (agw.key = "content_type"  go.tag = "json:\"content_type\""),
    10: optional string text (agw.key = "text" go.tag = "json:\"text\""),
    11: optional Image image (agw.key = "image" go.tag = "json:\"image\""),               // 图片内容
    12: optional list<Content> multiPart (agw.key = "multi_part" go.tag = "json:\"multi_part\""),          // 图文混排时，图文内容
}

struct Image {
    1: optional string name (agw.key = "name" go.tag = "json:\"name\"")
    2: optional string url  (agw.key = "url" go.tag = "json:\"url\"")
}

struct SpanInfo {
    1: required string span_id
    2: required list<FieldData>  field_list
}
struct ExtractSpanInfoResponse {
    1: required list<SpanInfo>  span_infos

    255: optional base.BaseResp BaseResp
}

struct CreateTaskRequest {
    1: required task.Task task (api.body = "task"),

    255: optional base.Base base,
}

struct CreateTaskResponse {
    1: optional i64 task_id (api.js_conv="true" api.body = "task_id"),

    255: optional base.BaseResp BaseResp
}

struct UpdateTaskRequest {
    1: required i64 task_id (api.js_conv="true" api.path = "task_id"),
    2: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    3: optional task.TaskStatus task_status (api.body = "task_status"),
    4: optional string description  (api.body = "description"),
    5: optional task.EffectiveTime effective_time (api.body = "effective_time"),
    6: optional double sample_rate (api.body = "sample_rate"),

    255: optional base.Base base,
}

struct UpdateTaskResponse {
    255: optional base.BaseResp BaseResp
}
enum OrderType {
    Unknown = 0
    Asc     = 1
    Desc    = 2
}
struct ListTasksRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: optional filter.TaskFilterFields task_filters (api.body = "task_filters"),

    101: optional i32 limit (api.body = "limit")   /* default 20 max 200 */
    102: optional i32 offset (api.body = "offset")
    103: optional OrderType order_by (api.body = "order_by")
    255: optional base.Base base,
}

struct ListTasksResponse {
    1: optional list<task.Task> tasks (api.body = "tasks"),

    100: optional i64 total (api.js_conv="true" api.body = "total"),
    255: optional base.BaseResp BaseResp
}

struct GetTaskRequest {
    1: required i64 task_id (api.path = "task_id" api.js_conv="true"),
    2: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")

    255: optional base.Base base,
}

struct GetTaskResponse {
    1: optional task.Task task (api.body = "task"),

    255: optional base.BaseResp BaseResp
}

struct CheckTaskNameRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: required string name                 (api.body='name')
    255: optional base.Base Base
}

struct CheckTaskNameResponse {
    1: optional bool Pass (agw.key = 'pass')
    2: optional string Message (agw.key ='message')
    255: base.BaseResp BaseResp
}

service TraceService {
    ListSpansResponse ListSpans(1: ListSpansRequest req) (api.post = '/api/observability/v1/spans/list')
    GetTraceResponse GetTrace(1: GetTraceRequest req) (api.get = '/api/observability/v1/traces/:trace_id')
    BatchGetTracesAdvanceInfoResponse BatchGetTracesAdvanceInfo(1: BatchGetTracesAdvanceInfoRequest req) (api.post = '/api/observability/v1/traces/batch_get_advance_info')
    IngestTracesResponse IngestTracesInner(1: IngestTracesRequest req)
    GetTracesMetaInfoResponse GetTracesMetaInfo(1: GetTracesMetaInfoRequest req) (api.get = '/api/observability/v1/traces/meta_info')
    CreateViewResponse CreateView(1: CreateViewRequest req) (api.post = '/api/observability/v1/views')
    UpdateViewResponse UpdateView(1: UpdateViewRequest req) (api.put = '/api/observability/v1/views/:view_id')
    DeleteViewResponse DeleteView(1: DeleteViewRequest req) (api.delete = '/api/observability/v1/views/:view_id')
    ListViewsResponse ListViews(1: ListViewsRequest req) (api.post = '/api/observability/v1/views/list')
    CreateManualAnnotationResponse CreateManualAnnotation(1: CreateManualAnnotationRequest req) (api.post = '/api/observability/v1/annotations')
    UpdateManualAnnotationResponse UpdateManualAnnotation(1: UpdateManualAnnotationRequest req) (api.put = '/api/observability/v1/annotations/:annotation_id')
    DeleteManualAnnotationResponse DeleteManualAnnotation(1: DeleteManualAnnotationRequest req) (api.delete = '/api/observability/v1/annotations/:annotation_id')
    ListAnnotationsResponse ListAnnotations(1: ListAnnotationsRequest req) (api.post = '/api/observability/v1/annotations/list')

    ChangeEvaluatorScoreResponse ChangeEvaluatorScore(1: ChangeEvaluatorScoreRequest req) (api.post = '/api/observability/v1/annotations/change_eEvaluator_sScore')
    ListAnnotationEvaluatorsResponse ListAnnotationEvaluators(1: ListAnnotationEvaluatorsRequest req) (api.post = '/api/observability/v1/annotations/lis_annotation_evaluators')
    ExtractSpanInfoResponse ExtractSpanInfo(1: ExtractSpanInfoRequest req) (api.post = '/api/observability/v1/traces/extract_span_info')
    CheckTaskNameResponse CheckTaskName(1: CheckTaskNameRequest req) (api.get = '/api/observability/v1/tasks/check_name')
    CreateTaskResponse CreateTask(1: CreateTaskRequest req) (api.post = '/api/observability/v1/tasks')
    UpdateTaskResponse UpdateTask(1: UpdateTaskRequest req) (api.put = '/api/observability/v1/tasks/:task_id')
    ListTasksResponse ListTasks(1: ListTasksRequest req) (api.post = '/api/observability/v1/tasks/list')
    GetTaskResponse GetTask(1: GetTaskRequest req) (api.get = '/api/observability/v1/tasks/:task_id')
}
