import { PlatformType, SpanListType } from '@cozeloop/api-schema/observation';

import { i18nService } from '@/i18n';

import { QUERY_PROPERTY } from './trace-attrs';

/** 固定露出的筛选字段 */
export const TRACES_PERSISTENT_FILTER_PROPERTY = [QUERY_PROPERTY.PromptKey];

export const QUERY_PROPERTY_LABEL_MAP = {
  [QUERY_PROPERTY.Status]: 'table_col_status',
  [QUERY_PROPERTY.TraceId]: 'table_col_trace_id',
  [QUERY_PROPERTY.Input]: 'table_col_input',
  [QUERY_PROPERTY.Output]: 'table_col_output',
  [QUERY_PROPERTY.Latency]: 'table_col_latency',
  [QUERY_PROPERTY.Tokens]: 'table_col_tokens',
  [QUERY_PROPERTY.LatencyFirst]: 'table_col_latency_first',
  [QUERY_PROPERTY.PromptKey]: 'table_col_prompt_key',
  [QUERY_PROPERTY.SpanType]: 'table_col_span_type',
  [QUERY_PROPERTY.SpanName]: 'table_col_span_name',
  [QUERY_PROPERTY.SpanId]: 'table_col_span_id',
  [QUERY_PROPERTY.InputTokens]: 'table_col_input_tokens',
  [QUERY_PROPERTY.OutputTokens]: 'table_col_output_tokens',
  [QUERY_PROPERTY.LogicDeleteDate]: 'table_col_logic_delete_date',
  [QUERY_PROPERTY.StartTime]: 'table_col_start_time',
  [QUERY_PROPERTY.Feedback]: 'table_col_feedback',
};

export const SPAN_TAB_OPTION_LIST = [
  {
    value: SpanListType.RootSpan,
    label: 'Root Span',
  },
  {
    value: SpanListType.AllSpan,
    label: 'All Span',
  },
  {
    value: SpanListType.LlmSpan,
    label: 'Model Span',
  },
];

export const PLATFORM_ENUM_OPTION_LIST = [
  {
    value: PlatformType.Cozeloop,
    label: i18nService.t('sdk_reporting'),
  },
  {
    value: PlatformType.Prompt,
    label: i18nService.t('prompt_development'),
  },
];
