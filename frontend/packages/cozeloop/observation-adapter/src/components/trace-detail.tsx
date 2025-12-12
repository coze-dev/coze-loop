// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/naming-convention */
import dayjs from 'dayjs';
import { AnnotationPanel } from '@cozeloop/tag-components';
import zhCH from '@cozeloop/observation-components/zh-CN';
import enUS from '@cozeloop/observation-components/en-US';
import {
  CozeloopTraceDetailPanel as TraceDetailPanelInner,
  type CozeloopTraceDetailPanelProps,
  type TraceDetailContext,
  tabs,
  CozeloopTraceDetail as TraceDetailInner,
  type CozeloopTraceDetailProps,
  ConfigProvider,
} from '@cozeloop/observation-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type span } from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import { type TraceDetailExtraProps } from '@cozeloop/adapter-interfaces';

const MAX_LAST_7_DAYS = 7;

interface TraceDetailOpenPanelProps {
  forceOverwrite?: boolean;
}

type TracePanelWrapperProps = CozeloopTraceDetailPanelProps &
  TraceDetailContext &
  TraceDetailOpenPanelProps &
  TraceDetailExtraProps;

type TraceWrapperDetailProps = CozeloopTraceDetailProps &
  TraceDetailContext &
  TraceDetailOpenPanelProps &
  TraceDetailExtraProps;

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
    const { forceOverwrite, traceID, startTime, endTime, customParams } = props;
    const space = useSpace();
    const spaceID = customParams?.spaceID || space.spaceID;
    const lang = I18n.language === 'zh-CN' ? zhCH : enUS;

    const amendStartTime = startTime
      ? startTime
      : dayjs().subtract(MAX_LAST_7_DAYS, 'day').valueOf().toString();
    const amendEndTime = endTime ? endTime : dayjs().valueOf().toString();

    const traceDetailOpenPanelProps = forceOverwrite
      ? {
          extraSpanDetailTabs: tabs,
          spanDetailHeaderSlot: (span: span.OutputSpan) => (
            <AnnotationPanel span={span} platformType={props.platformType} />
          ),
          ...props,
        }
      : {
          ...props,
          extraSpanDetailTabs: tabs,
          spanDetailHeaderSlot: (span: span.OutputSpan) => (
            <AnnotationPanel span={span} platformType={props.platformType} />
          ),
        };

    return (
      <ConfigProvider
        locale={{
          language: I18n.lang,
          locale: lang,
        }}
      >
        <Component
          {...(traceDetailOpenPanelProps as any)}
          getTraceDetailData={
            props.getTraceDetailData ??
            observabilityTrace.GetTrace({
              workspace_id: spaceID,
              start_time: amendStartTime,
              end_time: amendEndTime,
              trace_id: traceID,
            })
          }
        />
      </ConfigProvider>
    );
  };

  return Wrapper;
};

export const TraceDetailPanel = TraceDetailWrapper({
  Component: TraceDetailPanelInner,
});
export const TraceDetail = TraceDetailWrapper({
  Component: TraceDetailInner,
});
