// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as metric from './domain/metric';
export { metric };
import * as common from './domain/common';
export { common };
import * as filter from './domain/filter';
export { filter };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface GetMetricsRequest {
  workspace_id: string,
  start_time: string,
  end_time: string,
  metric_names: string[],
  granularity?: string,
  filters?: filter.FilterFields,
  platform_type?: common.PlatformType,
  drill_down_fields?: filter.FilterField[],
  compare?: metric.Compare,
}
export interface GetMetricsResponse {
  metrics?: {
    [key: string | number]: metric.Metric
  },
  compared_metrics?: {
    [key: string | number]: metric.Metric
  },
}
export interface GetDrillDownValuesRequest {
  workspace_id: string,
  start_time: string,
  end_time: string,
  filters?: filter.FilterFields,
  platform_type?: common.PlatformType,
  drill_down_value_type: metric.DrillDownValueType,
}
export interface DrillDownValue {
  value: string,
  display_name?: string,
}
export interface GetDrillDownValuesResponse {
  drill_down_values?: DrillDownValue[]
}
export const GetMetrics = /*#__PURE__*/createAPI<GetMetricsRequest, GetMetricsResponse>({
  "url": "/api/observability/v1/metrics/list",
  "method": "POST",
  "name": "GetMetrics",
  "reqType": "GetMetricsRequest",
  "reqMapping": {
    "body": ["workspace_id", "start_time", "end_time", "metric_names", "granularity", "filters", "platform_type", "drill_down_fields", "compare"]
  },
  "resType": "GetMetricsResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.metric",
  "service": "observabilityMetric"
});
export const GetDrillDownValues = /*#__PURE__*/createAPI<GetDrillDownValuesRequest, GetDrillDownValuesResponse>({
  "url": "/api/observability/v1/metrics/drill_down_values",
  "method": "POST",
  "name": "GetDrillDownValues",
  "reqType": "GetDrillDownValuesRequest",
  "reqMapping": {
    "body": ["workspace_id", "start_time", "end_time", "filters", "platform_type", "drill_down_value_type"]
  },
  "resType": "GetDrillDownValuesResponse",
  "schemaRoot": "api://schemas/observability_coze.loop.observability.metric",
  "service": "observabilityMetric"
});