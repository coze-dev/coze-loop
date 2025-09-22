namespace go coze.loop.observability.trace

include "../../../base.thrift"
include "../data/domain/dataset.thrift"
include "./domain/span.thrift"
include "./domain/common.thrift"
include "./domain/filter.thrift"
include "./domain/view.thrift"
include "./domain/annotation.thrift"
include "./domain/export_dataset.thrift"

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

struct ExportTracesToDatasetRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: required list<SpanID> span_ids (api.body="span_ids", vt.min_size="1", vt.max_size="500")
    3: required dataset.DatasetCategory category (api.body="category")
    4: required DatasetConfig config (api.body="config")
    5: required i64 start_time (api.js_conv="true", go.tag='json:"start_time"', api.body="start_time")
    6: required i64 end_time (api.js_conv="true", go.tag='json:"end_time"', api.body="end_time")
    7: optional common.PlatformType platform_type (api.body="platform_type")
    8: required export_dataset.ExportType export_type (api.body="export_type")                 // 导入方式，不填默认为追加
    9: optional list<export_dataset.FieldMapping> field_mappings (api.body="field_mappings", vt.min_size="1", vt.max_size="100")

    255: optional base.Base Base
}

struct SpanID {
    1: required string trace_id
    2: required string span_id
}

struct DatasetConfig {
    1: required bool   is_new_dataset                        // 是否是新增数据集
    2: optional i64    dataset_id (api.js_conv="true", go.tag='json:"dataset_id"')   // 数据集id，新增数据集时可为空
    3: optional string dataset_name                          // 数据集名称，选择已有数据集时可为空
    4: optional export_dataset.DatasetSchema dataset_schema (vt.not_nil="true")   // 数据集列数据schema
}

struct ExportTracesToDatasetResponse {
    1: optional i32 success_count                       // 成功导入的数量
    2: optional list<dataset.ItemErrorGroup> errors     // 错误信息
    3: optional i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"')    // 数据集id
    4: optional string dataset_name                     // 数据集名称

    255: optional base.BaseResp BaseResp (api.none="true")
    256: optional i32 Code (agw.key = "code")     // 仅供http请求使用; 内部RPC不予使用，统一通过BaseResp获取Code和Msg
    257: optional string Msg (agw.key = "msg")    // 仅供http请求使用; 内部RPC不予使用，统一通过BaseResp获取Code和Msg
}

struct PreviewExportTracesToDatasetRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"', api.body="workspace_id", vt.gt="0")
    2: required list<SpanID> span_ids (api.body="span_ids", vt.min_size="1", vt.max_size="500")
    3: required dataset.DatasetCategory category (api.body="category")
    4: required DatasetConfig config (api.body="config")
    5: required i64 start_time (api.js_conv="true", go.tag='json:"start_time"', api.body="start_time")
    6: required i64 end_time (api.js_conv="true", go.tag='json:"end_time"', api.body="end_time")
    7: optional common.PlatformType platform_type (api.body="platform_type")
    8: required export_dataset.ExportType export_type (api.body="export_type")                 // 导入方式，不填默认为追加
    9: optional list<export_dataset.FieldMapping> field_mappings (api.body="field_mappings", vt.min_size="1", vt.max_size="100")

    255: optional base.Base Base (api.none="true")
}

struct PreviewExportTracesToDatasetResponse {
    1: optional list<export_dataset.Item> items         // 预览数据
    2: optional list<dataset.ItemErrorGroup> errors     // 概要错误信息

    255: optional base.BaseResp BaseResp (api.none="true")
    256: optional i32 Code (agw.key = "code")     // 仅供http请求使用; 内部RPC不予使用，统一通过BaseResp获取Code和Msg
    257: optional string Msg (agw.key = "msg")    // 仅供http请求使用; 内部RPC不予使用，统一通过BaseResp获取Code和Msg
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
    ExportTracesToDatasetResponse ExportTracesToDataset(1: ExportTracesToDatasetRequest Req)(api.post = '/api/observability/v1/traces/export_to_dataset')
    PreviewExportTracesToDatasetResponse PreviewExportTracesToDataset(1: PreviewExportTracesToDatasetRequest Req)(api.post = '/api/observability/v1/traces/preview_export_to_dataset')
}
