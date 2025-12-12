import * as task from './domain/task';
export { task };
import * as export_dataset from './domain/export_dataset';
export { export_dataset };
import * as annotation from './domain/annotation';
export { annotation };
import * as view from './domain/view';
export { view };
import * as filter from './domain/filter';
export { filter };
import * as common from './domain/common';
export { common };
import * as span from './domain/span';
export { span };
import * as dataset from './../data/domain/dataset';
export { dataset };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface ListSpansRequest {
  workspace_id: string,
  /** ms */
  start_time: string,
  /** ms */
  end_time: string,
  filters?: filter.FilterFields,
  page_size?: number,
  order_bys?: common.OrderBy[],
  page_token?: string,
  platform_type?: common.PlatformType,
  /** default root span */
  span_list_type?: common.SpanListType,
}
export interface ListSpansResponse {
  spans: span.OutputSpan[],
  next_page_token: string,
  has_more: boolean,
}
export interface ListPreSpanRequest {
  workspace_id: string,
  trace_id: string,
  /** ms */
  start_time: string,
  span_id?: string,
  previous_response_id?: string,
  platform_type?: common.PlatformType,
}
export interface ListPreSpanResponse {
  spans: span.OutputSpan[]
}
export interface TokenCost {
  input: string,
  output: string,
}
export interface TraceAdvanceInfo {
  trace_id: string,
  tokens: TokenCost,
}
export interface GetTraceRequest {
  workspace_id: string,
  trace_id: string,
  /** ms */
  start_time: string,
  /** ms */
  end_time: string,
  platform_type?: common.PlatformType,
  span_ids?: string[],
}
export interface GetTraceResponse {
  spans: span.OutputSpan[],
  traces_advance_info?: TraceAdvanceInfo,
}
export interface SearchTraceTreeRequest {
  workspace_id: string,
  trace_id: string,
  /** ms */
  start_time: string,
  /** ms */
  end_time: string,
  platform_type?: common.PlatformType,
  filters?: filter.FilterFields,
}
export interface SearchTraceTreeResponse {
  spans: span.OutputSpan[],
  traces_advance_info?: TraceAdvanceInfo,
}
export interface TraceQueryParams {
  trace_id: string,
  start_time: string,
  end_time: string,
}
export interface BatchGetTracesAdvanceInfoRequest {
  workspace_id: string,
  traces: TraceQueryParams[],
  platform_type?: common.PlatformType,
}
export interface BatchGetTracesAdvanceInfoResponse {
  traces_advance_info: TraceAdvanceInfo[]
}
export interface IngestTracesRequest {
  spans?: span.InputSpan[]
}
export interface IngestTracesResponse {
  code?: number,
  msg?: string,
}
export interface FieldMeta {
  value_type: filter.FieldType,
  filter_types: filter.QueryType[],
  field_options?: filter.FieldOptions,
  support_customizable_option?: boolean,
}
export interface GetTracesMetaInfoRequest {
  platform_type?: common.PlatformType,
  span_list_type?: common.SpanListType,
  /** required */
  workspace_id?: string,
}
export interface GetTracesMetaInfoResponse {
  field_metas: {
    [key: string | number]: FieldMeta
  },
  key_span_type?: string[],
}
export interface CreateViewRequest {
  enterprise_id?: string,
  workspace_id: string,
  view_name: string,
  platform_type: common.PlatformType,
  span_list_type: common.SpanListType,
  filters: string,
}
export interface CreateViewResponse {
  id: string
}
export interface UpdateViewRequest {
  view_id: string,
  workspace_id: string,
  view_name?: string,
  platform_type?: common.PlatformType,
  span_list_type?: common.SpanListType,
  filters?: string,
}
export interface UpdateViewResponse {}
export interface DeleteViewRequest {
  view_id: string,
  workspace_id: string,
}
export interface DeleteViewResponse {}
export interface ListViewsRequest {
  enterprise_id?: string,
  workspace_id: string,
  view_name?: string,
}
export interface ListViewsResponse {
  views: view.View[]
}
export interface CreateManualAnnotationRequest {
  annotation: annotation.Annotation,
  platform_type?: common.PlatformType,
}
export interface CreateManualAnnotationResponse {
  annotation_id?: string
}
export interface UpdateManualAnnotationRequest {
  annotation_id: string,
  annotation: annotation.Annotation,
  platform_type?: common.PlatformType,
}
export interface UpdateManualAnnotationResponse {}
export interface DeleteManualAnnotationRequest {
  annotation_id: string,
  workspace_id: string,
  trace_id: string,
  span_id: string,
  start_time: string,
  annotation_key: string,
  platform_type?: common.PlatformType,
}
export interface DeleteManualAnnotationResponse {}
export interface ListAnnotationsRequest {
  workspace_id: string,
  span_id: string,
  trace_id: string,
  start_time: string,
  platform_type?: common.PlatformType,
  desc_by_updated_at?: boolean,
}
export interface ListAnnotationsResponse {
  annotations: annotation.Annotation[]
}
export interface ExportTracesToDatasetRequest {
  workspace_id: string,
  span_ids: SpanID[],
  category: dataset.DatasetCategory,
  config: DatasetConfig,
  start_time: string,
  end_time: string,
  platform_type?: common.PlatformType,
  /** 导入方式，不填默认为追加 */
  export_type: export_dataset.ExportType,
  field_mappings?: export_dataset.FieldMapping[],
}
export interface SpanID {
  trace_id: string,
  span_id: string,
}
export interface DatasetConfig {
  /** 是否是新增数据集 */
  is_new_dataset: boolean,
  /** 数据集id，新增数据集时可为空 */
  dataset_id?: string,
  /** 数据集名称，选择已有数据集时可为空 */
  dataset_name?: string,
  /** 数据集列数据schema */
  dataset_schema?: export_dataset.DatasetSchema,
}
export interface ExportTracesToDatasetResponse {
  /** 成功导入的数量 */
  success_count?: number,
  /** 错误信息 */
  errors?: dataset.ItemErrorGroup[],
  /** 数据集id */
  dataset_id?: string,
  /** 数据集名称 */
  dataset_name?: string,
  /** 仅供http请求使用; 内部RPC不予使用，统一通过BaseResp获取Code和Msg */
  code?: number,
  /** 仅供http请求使用; 内部RPC不予使用，统一通过BaseResp获取Code和Msg */
  msg?: string,
}
export interface PreviewExportTracesToDatasetRequest {
  workspace_id: string,
  span_ids: SpanID[],
  category: dataset.DatasetCategory,
  config: DatasetConfig,
  start_time: string,
  end_time: string,
  platform_type?: common.PlatformType,
  /** 导入方式，不填默认为追加 */
  export_type: export_dataset.ExportType,
  field_mappings?: export_dataset.FieldMapping[],
}
export interface PreviewExportTracesToDatasetResponse {
  /** 预览数据 */
  items?: export_dataset.Item[],
  /** 概要错误信息 */
  errors?: dataset.ItemErrorGroup[],
  /** 仅供http请求使用; 内部RPC不予使用，统一通过BaseResp获取Code和Msg */
  code?: number,
  /** 仅供http请求使用; 内部RPC不予使用，统一通过BaseResp获取Code和Msg */
  msg?: string,
}
export interface ChangeEvaluatorScoreRequest {
  workspace_id: string,
  annotation_id: string,
  span_id: string,
  start_time: string,
  correction: annotation.Correction,
  platform_type?: common.PlatformType,
}
export interface ChangeEvaluatorScoreResponse {
  annotation: annotation.Annotation
}
export interface ListAnnotationEvaluatorsRequest {
  workspace_id: string,
  name?: string,
}
export interface ListAnnotationEvaluatorsResponse {
  evaluators: annotation.AnnotationEvaluator[]
}
export interface ExtractSpanInfoRequest {
  workspace_id: string,
  trace_id: string,
  span_ids: string[],
  start_time?: string,
  end_time?: string,
  platform_type?: common.PlatformType,
  field_mappings?: export_dataset.FieldMapping[],
}
export interface SpanInfo {
  span_id: string,
  field_list: export_dataset.FieldData[],
}
export interface ExtractSpanInfoResponse {
  span_infos: SpanInfo[]
}
export const ListSpans = /*#__PURE__*/createAPI<ListSpansRequest, ListSpansResponse>({
  "url": "/api/observability/v1/spans/list",
  "method": "POST",
  "name": "ListSpans",
  "reqType": "ListSpansRequest",
  "reqMapping": {
    "body": ["workspace_id", "start_time", "end_time", "filters", "page_size", "order_bys", "page_token", "platform_type", "span_list_type"]
  },
  "resType": "ListSpansResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const ListPreSpan = /*#__PURE__*/createAPI<ListPreSpanRequest, ListPreSpanResponse>({
  "url": "/api/observability/v1/spans/pre_list",
  "method": "POST",
  "name": "ListPreSpan",
  "reqType": "ListPreSpanRequest",
  "reqMapping": {
    "body": ["workspace_id", "trace_id", "start_time", "span_id", "previous_response_id", "platform_type"]
  },
  "resType": "ListPreSpanResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const GetTrace = /*#__PURE__*/createAPI<GetTraceRequest, GetTraceResponse>({
  "url": "/api/observability/v1/traces/:trace_id",
  "method": "GET",
  "name": "GetTrace",
  "reqType": "GetTraceRequest",
  "reqMapping": {
    "query": ["workspace_id", "start_time", "end_time", "platform_type", "span_ids"],
    "path": ["trace_id"]
  },
  "resType": "GetTraceResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const SearchTraceTree = /*#__PURE__*/createAPI<SearchTraceTreeRequest, SearchTraceTreeResponse>({
  "url": "/api/observability/v1/traces/search_tree",
  "method": "POST",
  "name": "SearchTraceTree",
  "reqType": "SearchTraceTreeRequest",
  "reqMapping": {
    "body": ["workspace_id", "trace_id", "start_time", "end_time", "platform_type", "filters"]
  },
  "resType": "SearchTraceTreeResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const BatchGetTracesAdvanceInfo = /*#__PURE__*/createAPI<BatchGetTracesAdvanceInfoRequest, BatchGetTracesAdvanceInfoResponse>({
  "url": "/api/observability/v1/traces/batch_get_advance_info",
  "method": "POST",
  "name": "BatchGetTracesAdvanceInfo",
  "reqType": "BatchGetTracesAdvanceInfoRequest",
  "reqMapping": {
    "body": ["workspace_id", "traces", "platform_type"]
  },
  "resType": "BatchGetTracesAdvanceInfoResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const GetTracesMetaInfo = /*#__PURE__*/createAPI<GetTracesMetaInfoRequest, GetTracesMetaInfoResponse>({
  "url": "/api/observability/v1/traces/meta_info",
  "method": "GET",
  "name": "GetTracesMetaInfo",
  "reqType": "GetTracesMetaInfoRequest",
  "reqMapping": {
    "query": ["platform_type", "span_list_type", "workspace_id"]
  },
  "resType": "GetTracesMetaInfoResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const CreateView = /*#__PURE__*/createAPI<CreateViewRequest, CreateViewResponse>({
  "url": "/api/observability/v1/views",
  "method": "POST",
  "name": "CreateView",
  "reqType": "CreateViewRequest",
  "reqMapping": {
    "body": ["enterprise_id", "workspace_id", "view_name", "platform_type", "span_list_type", "filters"]
  },
  "resType": "CreateViewResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const UpdateView = /*#__PURE__*/createAPI<UpdateViewRequest, UpdateViewResponse>({
  "url": "/api/observability/v1/views/:view_id",
  "method": "PUT",
  "name": "UpdateView",
  "reqType": "UpdateViewRequest",
  "reqMapping": {
    "path": ["view_id"],
    "body": ["workspace_id", "view_name", "platform_type", "span_list_type", "filters"]
  },
  "resType": "UpdateViewResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const DeleteView = /*#__PURE__*/createAPI<DeleteViewRequest, DeleteViewResponse>({
  "url": "/api/observability/v1/views/:view_id",
  "method": "DELETE",
  "name": "DeleteView",
  "reqType": "DeleteViewRequest",
  "reqMapping": {
    "path": ["view_id"],
    "query": ["workspace_id"]
  },
  "resType": "DeleteViewResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const ListViews = /*#__PURE__*/createAPI<ListViewsRequest, ListViewsResponse>({
  "url": "/api/observability/v1/views/list",
  "method": "POST",
  "name": "ListViews",
  "reqType": "ListViewsRequest",
  "reqMapping": {
    "body": ["enterprise_id", "workspace_id", "view_name"]
  },
  "resType": "ListViewsResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const CreateManualAnnotation = /*#__PURE__*/createAPI<CreateManualAnnotationRequest, CreateManualAnnotationResponse>({
  "url": "/api/observability/v1/annotations",
  "method": "POST",
  "name": "CreateManualAnnotation",
  "reqType": "CreateManualAnnotationRequest",
  "reqMapping": {
    "body": ["annotation", "platform_type"]
  },
  "resType": "CreateManualAnnotationResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const UpdateManualAnnotation = /*#__PURE__*/createAPI<UpdateManualAnnotationRequest, UpdateManualAnnotationResponse>({
  "url": "/api/observability/v1/annotations/:annotation_id",
  "method": "PUT",
  "name": "UpdateManualAnnotation",
  "reqType": "UpdateManualAnnotationRequest",
  "reqMapping": {
    "path": ["annotation_id"],
    "body": ["annotation", "platform_type"]
  },
  "resType": "UpdateManualAnnotationResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const DeleteManualAnnotation = /*#__PURE__*/createAPI<DeleteManualAnnotationRequest, DeleteManualAnnotationResponse>({
  "url": "/api/observability/v1/annotations/:annotation_id",
  "method": "DELETE",
  "name": "DeleteManualAnnotation",
  "reqType": "DeleteManualAnnotationRequest",
  "reqMapping": {
    "path": ["annotation_id"],
    "query": ["workspace_id", "trace_id", "span_id", "start_time", "annotation_key", "platform_type"]
  },
  "resType": "DeleteManualAnnotationResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const ListAnnotations = /*#__PURE__*/createAPI<ListAnnotationsRequest, ListAnnotationsResponse>({
  "url": "/api/observability/v1/annotations/list",
  "method": "POST",
  "name": "ListAnnotations",
  "reqType": "ListAnnotationsRequest",
  "reqMapping": {
    "body": ["workspace_id", "span_id", "trace_id", "start_time", "platform_type", "desc_by_updated_at"]
  },
  "resType": "ListAnnotationsResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const ExportTracesToDataset = /*#__PURE__*/createAPI<ExportTracesToDatasetRequest, ExportTracesToDatasetResponse>({
  "url": "/api/observability/v1/traces/export_to_dataset",
  "method": "POST",
  "name": "ExportTracesToDataset",
  "reqType": "ExportTracesToDatasetRequest",
  "reqMapping": {
    "body": ["workspace_id", "span_ids", "category", "config", "start_time", "end_time", "platform_type", "export_type", "field_mappings"]
  },
  "resType": "ExportTracesToDatasetResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const PreviewExportTracesToDataset = /*#__PURE__*/createAPI<PreviewExportTracesToDatasetRequest, PreviewExportTracesToDatasetResponse>({
  "url": "/api/observability/v1/traces/preview_export_to_dataset",
  "method": "POST",
  "name": "PreviewExportTracesToDataset",
  "reqType": "PreviewExportTracesToDatasetRequest",
  "reqMapping": {
    "body": ["workspace_id", "span_ids", "category", "config", "start_time", "end_time", "platform_type", "export_type", "field_mappings"]
  },
  "resType": "PreviewExportTracesToDatasetResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const ChangeEvaluatorScore = /*#__PURE__*/createAPI<ChangeEvaluatorScoreRequest, ChangeEvaluatorScoreResponse>({
  "url": "/api/observability/v1/traces/change_eval_score",
  "method": "POST",
  "name": "ChangeEvaluatorScore",
  "reqType": "ChangeEvaluatorScoreRequest",
  "reqMapping": {
    "body": ["workspace_id", "annotation_id", "span_id", "start_time", "correction", "platform_type"]
  },
  "resType": "ChangeEvaluatorScoreResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const ListAnnotationEvaluators = /*#__PURE__*/createAPI<ListAnnotationEvaluatorsRequest, ListAnnotationEvaluatorsResponse>({
  "url": "/api/observability/v1/annotation/list_evaluators",
  "method": "GET",
  "name": "ListAnnotationEvaluators",
  "reqType": "ListAnnotationEvaluatorsRequest",
  "reqMapping": {
    "query": ["workspace_id", "name"]
  },
  "resType": "ListAnnotationEvaluatorsResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});
export const ExtractSpanInfo = /*#__PURE__*/createAPI<ExtractSpanInfoRequest, ExtractSpanInfoResponse>({
  "url": "/api/observability/v1/trace/extract_span_info",
  "method": "POST",
  "name": "ExtractSpanInfo",
  "reqType": "ExtractSpanInfoRequest",
  "reqMapping": {
    "body": ["workspace_id", "trace_id", "span_ids", "start_time", "end_time", "platform_type", "field_mappings"]
  },
  "resType": "ExtractSpanInfoResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.trace",
  "service": "observabilityTrace"
});