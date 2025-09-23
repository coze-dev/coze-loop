// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const QUERY_PROPERTY = {
  Status: 'status',
  TraceId: 'trace_id',
  Input: 'input',
  Output: 'output',
  Tokens: 'tokens',
  InputTokens: 'input_tokens',
  OutputTokens: 'output_tokens',
  Latency: 'latency',
  LatencyFirst: 'latency_first_resp',
  SpanId: 'span_id',
  SpanName: 'span_name',
  SpanType: 'span_type',
  PromptKey: 'prompt_key',
  LogicDeleteDate: 'logic_delete_date',
  StartTime: 'start_time',
  Feedback: 'feedback',
} as const;

export const FILTER_INVALIDATE = {
  PromptKey: 'prompt_key',
  Channel: 'chanel',
  Version: 'version',
} as const;

export type QueryPropertyEnum =
  (typeof QUERY_PROPERTY)[keyof typeof QUERY_PROPERTY];

export type FilterInvalidateEnum =
  (typeof FILTER_INVALIDATE)[keyof typeof FILTER_INVALIDATE];
