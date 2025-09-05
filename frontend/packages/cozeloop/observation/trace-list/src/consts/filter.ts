// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';

import { QUERY_PROPERTY } from './trace-attrs';

export const SPAN_COLUMNS = [
  QUERY_PROPERTY.Status,
  QUERY_PROPERTY.TraceId,
  QUERY_PROPERTY.Input,
  QUERY_PROPERTY.Output,
  QUERY_PROPERTY.Tokens,
  QUERY_PROPERTY.InputTokens,
  QUERY_PROPERTY.OutputTokens,
  QUERY_PROPERTY.Latency,
  QUERY_PROPERTY.LatencyFirst,
  QUERY_PROPERTY.StartTime,
  QUERY_PROPERTY.SpanId,
  QUERY_PROPERTY.SpanName,
  QUERY_PROPERTY.SpanType,
  QUERY_PROPERTY.PromptKey,
  QUERY_PROPERTY.LogicDeleteDate,
];

/** 固定露出的筛选字段 */
export const TRACES_PERSISTENT_FILTER_PROPERTY = [QUERY_PROPERTY.PromptKey];

export const QUERY_PROPERTY_LABEL_MAP: Record<
  (typeof QUERY_PROPERTY)[keyof typeof QUERY_PROPERTY],
  string
> = {
  [QUERY_PROPERTY.Status]: 'Status',
  [QUERY_PROPERTY.TraceId]: 'Trace ID',
  [QUERY_PROPERTY.Input]: 'Input',
  [QUERY_PROPERTY.Output]: 'Output',
  [QUERY_PROPERTY.Latency]: 'Latency',
  [QUERY_PROPERTY.Tokens]: 'Tokens',
  [QUERY_PROPERTY.LatencyFirst]: 'LatencyFirstResp',
  [QUERY_PROPERTY.PromptKey]: 'PromptKey',
  [QUERY_PROPERTY.SpanType]: 'SpanType',
  [QUERY_PROPERTY.SpanName]: 'SpanName',
  [QUERY_PROPERTY.SpanId]: 'SpanID',
  [QUERY_PROPERTY.InputTokens]: 'Input Tokens',
  [QUERY_PROPERTY.OutputTokens]: 'Output Tokens',
  [QUERY_PROPERTY.LogicDeleteDate]: I18n.t('data_expiration_time'),
  [QUERY_PROPERTY.StartTime]: 'Start Time',
  [QUERY_PROPERTY.Feedback]: 'Feedback',
};

export const SPAN_TAB_OPTION_LIST = [
  {
    value: 'root_span',
    label: 'Root Span',
  },
  {
    value: 'all_span',
    label: 'All Span',
  },
  {
    value: 'llm_span',
    label: 'Model Span',
  },
];

export const PLATFORM_ENUM_OPTION_LIST = [
  {
    value: 'cozeloop',
    label: I18n.t('sdk_reporting'),
  },
  {
    value: 'prompt',
    label: I18n.t('prompt_development'),
  },
];
