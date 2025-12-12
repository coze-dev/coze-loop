/* eslint-disable @typescript-eslint/no-explicit-any */
import { type span } from '@cozeloop/api-schema/observation';
import { type ColumnProps } from '@coze-arch/coze-design';

import { type LogicValue } from './components';

export type Span = span.OutputSpan;

export interface ColumnItem extends ColumnProps {
  /** 列管理展示名字 */
  displayName: string;
  /** 是不是默认选中 */
  checked: boolean;
  /** 列的值，用于标识 */
  value?: string;
  /** 列的key，必须为string */
  key?: string;
}

export const BUILD_IN_COLUMN = {
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
  PromptKey: 'prompt_key',
  LogicDeleteDate: 'logic_delete_date',
  StartTime: 'start_time',
  SpanType: 'span_type',
};

export interface Filters {
  selectedSpanType?: string | number;
  timestamps?: [number, number];
  presetTimeRange?: string;
  filters?: LogicValue; // LogicValue
  persistentFilters?: any[];
  relation?: string;
  selectedPlatform?: string | number;
  startTime?: number;
  endTime?: number;
}

export interface SizedColumn<RecordType extends Record<string, any>>
  extends ColumnProps<RecordType> {
  width?: number;
  checked?: boolean;
  key?: string;
  disabled?: boolean;
  value?: string;
  displayName?: string;
}

export interface WorkspaceConfig {
  // env: 'boe' | 'online' | 'i18n' | 'boeI18n';
  workspaceId: string | number;
  domain: string;
  token: string;
}
