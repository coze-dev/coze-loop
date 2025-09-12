// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useImperativeHandle, useState } from 'react';

import { useUpdateEffect } from 'ahooks';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { PlatformType, SpanType } from '@cozeloop/api-schema/observation';

import {
  changeSpanNodeCollapseStatus,
  getNodeConfig,
} from '@/trace-detail/utils/span';
import { traceDetailContext } from '@/trace-detail/hooks/use-trace-detail-context';
import { useTeaDuration } from '@/trace-detail/hooks/use-tea-duration';
import { useFetchSpans } from '@/trace-detail/hooks/use-fetch-spans';
import { INVALIDATE_CODE } from '@/trace-detail/consts/code';
import { type SpanNode } from '@/trace-detail/components/graphs/trace-tree/type';

import { VerticalTraceDetail } from './vertical';
import { TraceDetailError } from './trace-error';
import { type TraceDetailProps } from './interface';
import { HorizontalTraceDetail } from './horizontal';

export const TraceDetail = (props: TraceDetailProps) => {
  const {
    id,
    spaceName,
    dataSource,
    moduleName,
    searchType,
    spaceID,
    defaultSpanID,
    endTime,
    layout,
    platformType = PlatformType.Cozeloop,
    startTime,
    optionRef,
    onReady,
    switchConfig,
    hideTraceDetailHeader = false,
    defaultActiveTabKey,
    spanDetailHeaderSlot,
    extraSpanDetailTabs,
  } = props;
  const [selectedSpanId, setSelectedSpanId] = useState<string>('');
  const [rootNodes, setRootNodes] = useState<SpanNode[] | undefined>(undefined);
  const { roots, spans, advanceInfo, loading, refresh, statusCode } =
    useFetchSpans({
      id,
      searchType,
      spaceID,
      spaceName,
      endTime,
      moduleName,
      platformType: platformType.toString(),
      startTime,
      options: {
        dataSource,
      },
    });

  useTeaDuration(
    EVENT_NAMES.cozeloop_observation_trace_detail_panel_view_duration,
    {
      space_id: spaceID,
      space_name: spaceName,
      search_type: searchType,
      platform_type: platformType.toString(),
      module_name: moduleName,
    },
  );

  useEffect(() => {
    sendEvent(EVENT_NAMES.cozeloop_trace_detail_show, {
      space_id: spaceID,
      space_name: spaceName,
      search_type: searchType,
      platform_type: platformType,
      module_name: moduleName,
    });
  }, []);

  useUpdateEffect(() => {
    if (defaultSpanID) {
      setSelectedSpanId(defaultSpanID);
    } else if (roots && roots.length > 0) {
      setSelectedSpanId(roots[0].span_id);
    }
  }, [id]);

  useEffect(() => {
    if (!selectedSpanId) {
      if (defaultSpanID) {
        setSelectedSpanId(defaultSpanID);
      } else if (roots && roots.length > 0) {
        setSelectedSpanId(roots[0].span_id);
      }
    }
    setRootNodes(roots);
  }, [roots, defaultSpanID]);

  useEffect(() => {
    if (spans.length > 0) {
      onReady?.();
    }
  }, [spans]);

  const handleCollapseChange = (targetId: string) => {
    if (rootNodes) {
      setRootNodes(changeSpanNodeCollapseStatus(rootNodes, targetId));
    }
  };

  const handleSelected = (selectedId: string) => {
    setSelectedSpanId(selectedId);
    const { type, span_type } =
      spans.find(span => span.span_id === selectedId) || {};
    sendEvent(EVENT_NAMES.cozeloop_click_trace_tree_node, {
      space_id: spaceID,
      space_name: spaceName,
      type:
        span_type ||
        getNodeConfig({
          spanTypeEnum: type ?? SpanType.Unknown,
          spanType: span_type ?? SpanType.Unknown,
        })?.typeName,
      module_name: moduleName,
    });
  };

  useImperativeHandle(optionRef, () => ({
    refresh,
  }));

  const selectedSpan = spans.find(span => span.span_id === selectedSpanId);

  if (INVALIDATE_CODE.includes(statusCode)) {
    return (
      <TraceDetailError
        statusCode={statusCode}
        spaceID={spaceID}
        id={id}
        searchType={searchType}
        headerConfig={props.headerConfig}
      />
    );
  }

  const commonProps = {
    advanceInfo,
    loading,
    onCollapseChange: handleCollapseChange,
    onSelect: handleSelected,
    rootNodes,
    spans,
    selectedSpan,
    selectedSpanId,
    urlParams: {
      spaceID,
      id: id ? id : (dataSource?.spans?.[0]?.trace_id ?? ''),
      searchType,
      defaultSpanID,
      endTime,
      platformType: platformType.toString(),
      startTime,
    },
    hideTraceDetailHeader,
    ...props,
  };

  return (
    <traceDetailContext.Provider
      value={{
        defaultActiveTabKey,
        spanDetailHeaderSlot,
        extraSpanDetailTabs,
        platformType: props.platformType,
      }}
    >
      {layout === 'vertical' ? (
        <VerticalTraceDetail {...commonProps} />
      ) : (
        <HorizontalTraceDetail {...commonProps} switchConfig={switchConfig} />
      )}
    </traceDetailContext.Provider>
  );
};
