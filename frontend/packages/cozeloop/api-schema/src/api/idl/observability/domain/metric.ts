// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export enum CompareType {
  YoY = "yoy",
  /** 同比 */
  MoM = "mom",
}
/** 环比 */
export enum DrillDownValueType {
  ModelName = "model_name",
  ToolName = "tool_name",
}
export interface Metric {
  summary?: string,
  pie?: {
    [key: string | number]: string
  },
  time_series?: {
    [key: string | number]: MetricPoint[]
  },
}
export interface MetricPoint {
  timestamp?: string,
  value?: string,
}
export interface Compare {
  compare_type?: CompareType,
  shift_seconds?: string,
}