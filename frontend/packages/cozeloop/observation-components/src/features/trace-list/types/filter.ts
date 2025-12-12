// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export interface TraceFilter {
  tab?: string | null;
  selected_span_type?: string | null;
  trace_platform?: string | null;
  trace_filters?: string | null;
  trace_start_time?: string | null;
  trace_end_time?: string | null;
  trace_preset_time_range?: string | null;
  [key: string]: string | string[] | null | undefined;
}
