// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type ColumnItem } from '@cozeloop/components';
import {
  type OrderBy,
  type span,
  type FieldMeta,
} from '@cozeloop/api-schema/observation';
import {
  type OptionProps,
  type DatePickerProps,
} from '@coze-arch/coze-design/.';

import { type ValueOf } from '@/shared/types/utils';
import { type DatePickerOptions } from '@/shared/components/filter-bar/types';
import { type TraceSelectorState } from '@/features/trace-selector';
import { type InitializedFilters } from '@/features/trace-list/utils/filter-initialization';
import { type BUILD_IN_COLUMN, type Span } from '@/features/trace-list/types';
import { type UseFetchTracesResult } from '@/features/trace-list/components/queries/table/hooks/use-fetch-traces';
import { type LogicValue } from '@/features/trace-list/components';

export type FetchSpansFn = (params: {
  platform_type: string;
  start_time: string;
  end_time: string;
  workspace_id?: string | number;
  filters?: LogicValue;
  order_bys?: OrderBy[];
  page_size: number;
  span_list_type?: string;
  page_token?: string;
}) => Promise<{
  spans: span.OutputSpan[];
  next_page_token: string;
  has_more: boolean;
}>;

export type GetFieldMetasFn = (params: {
  platform_type: string | number;
  span_list_type: string | number;
}) => Promise<Record<string, FieldMeta>>;

export interface CozeloopColumnsConfig {
  columns: (ColumnItem | ValueOf<typeof BUILD_IN_COLUMN>)[];
}

export interface FilterOptionsItemConfig {
  defaultValue?: string;
  optionList?: OptionProps[];
  visibility?: boolean;
}

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

export interface CozeloopTraceListProps {
  /**
   * @description 自定义表格列
   * @example
   * columnsConfig={{
   *   columns: ["status", "trace_id", {
   *     title: "custom",
   *     dataIndex: "custom",
   *     render: (text, record) => {
   *       return <div>{text}</div>;
   *     },
   *     displayName: "custom",
   *     checked: true,
   *     width: 180
   *   }]
   * }}
   */
  columnsConfig: CozeloopColumnsConfig;
  onRowClick?: (span: Span, index?: number) => void;
  defaultFilters?: Filters;
  onFiltersChange?: (filters: TraceSelectorState) => void;

  onTraceDataChange?: (result: UseFetchTracesResult) => void;
  /**
   * @description 默认 false 设置为 true 的话会阻止内部组件写 url query 等操作
   */
  disableEffect?: boolean;
  getFieldMetas?: GetFieldMetasFn;
  getTraceList?: FetchSpansFn;
  filterOptions?: {
    /**
     * @description 配置平台数据来源
     */
    platformTypeConfig?: FilterOptionsItemConfig;
    /**
     * @description 配置 span 的类型
     */
    spanListTypeConfig?: FilterOptionsItemConfig;
    datePickerOptions?: DatePickerOptions[];
    datePickerProps?: DatePickerProps;
  };

  customParams?: Record<string, any>;
  onInitLoad?: (state: InitializedFilters) => void;
  className?: string;
  style?: React.CSSProperties;
}
