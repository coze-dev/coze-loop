// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/naming-convention */
import {
  TraceDetailPanel as TraceDetailPanelInner,
  type TraceDetailPanelProps,
  type TraceDetailContext,
  TraceDetail as TraceDetailInner,
  type TraceDetailProps,
} from '@cozeloop/observation-component';

interface TraceDetailOpenPanelProps {
  forceOverwrite?: boolean;
}

interface TraceDetailOpenPanelProps {
  forceOverwrite?: boolean;
}

type TracePanelWrapperProps = TraceDetailPanelProps &
  TraceDetailContext &
  TraceDetailOpenPanelProps;

type TraceWrapperDetailProps = TraceDetailProps &
  TraceDetailContext &
  TraceDetailOpenPanelProps;

export const TraceDetailWrapper = <
  T extends
    | ((props: TraceWrapperDetailProps) => JSX.Element)
    | ((props: TracePanelWrapperProps) => JSX.Element),
>({
  Component,
}: {
  Component: T;
}) => {
  const Wrapper = (props: Parameters<T>[number]) => {
    const { forceOverwrite } = props;

    const traceDetailOpenPanelProps = forceOverwrite
      ? {
          extraSpanDetailTabs: [],
          ...props,
        }
      : {
          ...props,
          extraSpanDetailTabs: [],
        };

    return <Component {...(traceDetailOpenPanelProps as any)} />;
  };

  return Wrapper;
};

export const TraceDetailPanel = TraceDetailWrapper({
  Component: TraceDetailPanelInner,
});
export const TraceDetail = TraceDetailWrapper({
  Component: TraceDetailInner,
});

export {
  NODE_CONFIG_MAP,
  SpanType,
  getEndTime,
  getStartTime,
} from '@cozeloop/observation-component';
