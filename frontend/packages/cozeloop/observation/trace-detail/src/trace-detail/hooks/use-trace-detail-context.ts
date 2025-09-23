// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createContext, useContext } from 'react';

import { type span } from '@cozeloop/api-schema/observation';

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
}
export const traceDetailContext = createContext<TraceDetailContext>({});
export const useTraceDetailContext = () => useContext(traceDetailContext);
