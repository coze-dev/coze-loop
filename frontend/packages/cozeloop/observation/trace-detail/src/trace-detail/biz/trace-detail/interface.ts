// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type CSSProperties } from 'react';

import {
  type UrlParams,
  type DataSource,
  type Span,
  type TraceAdvanceInfo,
} from '@/trace-detail/typings/params';
import { type TraceDetailContext } from '@/trace-detail/hooks/use-trace-detail-context';
import { type SpanNode } from '@/trace-detail/components/graphs/trace-tree/type';

export interface TraceDetailOptions {
  refresh: () => void;
}

export interface TraceDetailProps extends TraceDetailContext {
  spaceID: string;
  spaceName: string;
  searchType: 'trace_id';
  id: string;
  dataSource?: DataSource;
  moduleName: string;
  platformType?: string | number;
  startTime?: number | string;
  endTime?: number | string;
  defaultSpanID?: string;
  layout?: 'horizontal' | 'vertical';
  headerConfig?: {
    visible?: boolean;
    disableEnvTag?: boolean;
    showClose?: boolean;
    onClose?: () => void;
    minColWidth?: number;
  };
  spanDetailConfig?: {
    showTags?: boolean;
    baseInfoPosition?: 'top' | 'right';
    minColWidth?: number;
    maxColNum?: number;
  };
  /** 抽屉状态下，进行trace切换 */
  switchConfig?: SwitchConfig;
  optionRef?: React.MutableRefObject<TraceDetailOptions | undefined>;
  className?: string;
  style?: CSSProperties;
  onReady?: () => void;
  hideTraceDetailHeader?: boolean;
}

export interface SwitchConfig {
  canSwitchPre: boolean;
  canSwitchNext: boolean;
  onSwitch: (action: 'pre' | 'next') => void;
}
export interface TraceDetailLayoutProps extends TraceDetailProps {
  rootNodes: SpanNode[] | undefined;
  spans: Span[];
  selectedSpan: Span | undefined;
  loading: boolean;
  selectedSpanId: string;
  onSelect: (id: string) => void;
  onCollapseChange: (id: string) => void;
  advanceInfo: TraceAdvanceInfo | undefined;
  urlParams: UrlParams;
}
