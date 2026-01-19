// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type TraceAdvanceInfo,
  type span,
  type FilterFields,
  type PlatformType,
} from '@cozeloop/api-schema/observation';

import { type TraceSelectorProps } from '@/features/trace-selector';
import { type DataSource } from '@/features/trace-detail/types/params';
import {
  type SearchService,
  type SelectedSpanService,
} from '@/features/trace-detail/hooks/use-trace-detail-controls';
import { type SpanNode } from '@/features/trace-detail/components/graphs/trace-tree/type';

import { type ResponseApiService } from '../../hooks/use-fetch-response-api';

interface TagListItem {
  title: string;
  item: ((span: span.OutputSpan) => string | React.ReactNode) | React.ReactNode;
  /** 是否支持复制 需要 item 字段是 string 类型 */
  enableCopy: boolean;
}
export type FetchTraceDetailFn =
  | ((params: {
      trace_id: string;
      start_time: string;
      end_time: string;
      platform_type: PlatformType;
      filters?: FilterFields;
    }) => Promise<DataSource>)
  | DataSource;

export interface CozeloopTraceDetailProps {
  /** span ID */
  spanId?: string;
  /** trace ID */
  traceId?: string;
  /** 获取 trace 详情数据 */
  getTraceDetailData?: (params: {
    filters?: FilterFields;
  }) => Promise<DataSource>;

  /** 获取 trace下 Span 详情数据 */
  getTraceSpanDetailData?: (params: {
    span_ids?: string[];
  }) => Promise<DataSource>;

  /** 是否支持Trace搜索能力 */
  enableTraceSearch?: boolean;
  // 是否开启 response_api 能力
  enableResponseApi?: boolean;
  /** 默认选中 span ID */
  defaultSelectedSpanID?: string;
  /** 布局 */
  layout?: 'horizontal' | 'vertical';
  /** trace detail header 配置 */
  headerConfig?: {
    visible?: boolean;
    showClose?: boolean;
    onClose?: () => void;
    minColWidth?: number;
    customRender?: (span?: span.OutputSpan) => React.ReactNode;
    extraRender?: (span?: span.OutputSpan) => React.ReactNode;
    showFullscreenButton?: boolean;
    onFullscreen?: () => void;
  };
  /** span detail 配置 */
  spanDetailConfig?: {
    showTags?: boolean;
    baseInfoPosition?: 'top' | 'right';
    minColWidth?: number;
    maxColNum?: number;
    extraTagList?: TagListItem[];
  };
  /** span 切换配置 */
  switchConfig?: SwitchConfig;
  className?: string;
  style?: React.CSSProperties;
  spanDetailHeaderSlot?: (
    span: span.OutputSpan,
    platform: string | number,
  ) => JSX.Element;
  keySpanType?: string[];
  treeSelectorConfig?: TraceSelectorProps;
  /** 自定义渲染头部复制节点 */
  renderHeaderCopyNode?: (span: span.OutputSpan) => React.ReactNode;
}

export interface SwitchConfig {
  canSwitchPre: boolean;
  canSwitchNext: boolean;
  onSwitch: (action: 'pre' | 'next') => void;
}
export interface CozeloopTraceDetailLayoutProps
  extends CozeloopTraceDetailProps {
  rootNodes: SpanNode[] | undefined;
  spans: span.OutputSpan[];
  selectedSpanService: SelectedSpanService;
  /** 响应 API 服务 */
  responseApiService: ResponseApiService;

  /** 搜索服务 */
  searchService: SearchService;

  loading: boolean;
  selectedSpanId: string;
  onSelect: (id: string) => void;
  onCollapseChange: (id: string) => void;
  onToggleAll?: () => void;
  isAllExpanded?: boolean;
  advanceInfo: TraceAdvanceInfo | undefined;
  /** 受控：搜索匹配到的 spanId 列表，用于高亮 */
  matchedSpanIds?: string[];
  /** 当前搜索过滤条件 */
  searchFilters?: FilterFields;
  setSearchFilters?: (filters: FilterFields) => void;
  onClear?: () => void;
  /** 是否过滤非关键节点 */
  filterNonCritical?: boolean;
  /** 切换过滤非关键节点 */
  onFilterNonCriticalChange?: (checked: boolean) => void;
  /** 是否开启Trace搜索能力 */
  enableTraceSearch?: boolean;
  // 是否开启 response_api 能力
  enableResponseApi?: boolean;
}
