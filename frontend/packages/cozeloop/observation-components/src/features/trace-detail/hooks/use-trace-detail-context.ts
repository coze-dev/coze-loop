// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { createContext, useContext } from 'react';

import {
  type PlatformType,
  type GetTraceResponse,
  type span,
} from '@cozeloop/api-schema/observation';

import { type JumpButtonConfig } from '../types';

interface ExtraTab {
  label: string;
  tabKey: string;
  render: (span: span.OutputSpan, platformType: string | number) => JSX.Element;
  visible?: ((span: span.OutputSpan) => boolean) | boolean;
}
export interface TraceDetailContext {
  extraSpanDetailTabs?: ExtraTab[];
  defaultActiveTabKey?: string;
  spanDetailHeaderSlot?: (
    span: span.OutputSpan,
    platform: string | number,
  ) => JSX.Element;
  platformType?: string | number;
  getTraceDetailData?:
    | ((params?: {
        trace_id: string;
        start_time: string;
        end_time: string;
        platform_type: PlatformType;
        span_ids?: string[];
      }) => Promise<GetTraceResponse>)
    | GetTraceResponse;
  customParams?: Record<string, any>;
  jumpButtonConfig?: JumpButtonConfig;
}
export const traceDetailContext = createContext<TraceDetailContext>({
  getTraceDetailData: () => Promise.resolve({ spans: [] }),
});
export const useTraceDetailContext = () => useContext(traceDetailContext);
