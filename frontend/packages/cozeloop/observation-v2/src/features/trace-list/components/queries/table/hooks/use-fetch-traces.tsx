/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */

import { useEffect, useRef, useState } from 'react';

import { isEmpty, keyBy, keys } from 'lodash-es';
import { useInfiniteScroll } from 'ahooks';
import {
  QueryType,
  type PlatformType,
  type SpanListType,
  type ListSpansRequest,
  QueryRelation,
  type FieldType,
  type TraceAdvanceInfo,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import { Toast } from '@coze-arch/coze-design';

import { type FetchSpansFn } from '@/types/trace-list';
import { BIZ_EVENTS } from '@/shared/constants';
import { AUTO_EVAL_FEEDBACK_PREFIX } from '@/shared/components/analytics-logic-expr/const';
import { type LogicValue } from '@/shared/components/analytics-logic-expr';
import { type ConvertSpan } from '@/features/trace-list/types/span';
import { useTraceStore } from '@/features/trace-list/stores/trace';
import { usePerformance } from '@/features/trace-list/hooks/use-performance';
import { useConfigContext } from '@/config-provider';

import { TRACE_EXPIRED_CODE } from '../../table';

const fetchTracesAdvanceInfo = async ({
  convertSpans,
  selectedSpanType,
  selectedPlatform,
  spaceID,
  startTime,
  endTime,
  buildInFetchTracesAdvanceInfo,
}: {
  convertSpans: ConvertSpan[];
  selectedSpanType: string | number;
  selectedPlatform: string | number;
  spaceID: string;
  startTime: number;
  endTime: number;
  buildInFetchTracesAdvanceInfo?: (params: {
    traces: { trace_id: string; start_time: string }[];
    platform_type: string;
    workspace_id: string;
  }) => Promise<{
    traces_advance_info: TraceAdvanceInfo[];
  }>;
}): Promise<Record<string, any>> => {
  let tracesAdvanceInfo: TraceAdvanceInfo[] = [];

  if (selectedSpanType !== 'root_span') {
    return tracesAdvanceInfo;
  }

  try {
    const traces = convertSpans.map(item => ({
      trace_id: item.trace_id,
      start_time: (Number(item.started_at) ?? startTime).toString(),
      end_time: (
        Number(item.started_at) + (Number(item.duration) ?? endTime)
      ).toString(),
    }));

    if (String(selectedSpanType) === 'root_span' && !isEmpty(traces)) {
      if (buildInFetchTracesAdvanceInfo) {
        const advanceResult = await buildInFetchTracesAdvanceInfo({
          traces,
          platform_type: selectedPlatform as string,
          workspace_id: spaceID,
        });
        tracesAdvanceInfo = advanceResult?.traces_advance_info ?? [];
      } else {
        // 对于 cozeloop bizId，使用原来的 BatchGetTracesAdvanceInfo 方法
        const advanceResult =
          await observabilityTrace.BatchGetTracesAdvanceInfo({
            traces,
            platform_type: selectedPlatform as PlatformType,
            workspace_id: spaceID,
          });
        tracesAdvanceInfo = advanceResult?.traces_advance_info ?? [];
      }
    }
  } catch (error) {
    console.error(error);
  }

  return tracesAdvanceInfo;
};

export interface UseFetchTracesParams {
  fetchSpansFn: FetchSpansFn;
  tracesRefreshKey?: string | number;
  buildInFetchTracesAdvanceInfo?: (params: {
    traces: { trace_id: string; start_time: string }[];
    platform_type: string;
    workspace_id: string;
  }) => Promise<{
    traces_advance_info: TraceAdvanceInfo[];
  }>;
  timestamps: [number, number];
  fieldMetas?: Record<string, any>;
  selectedSpanType: string;
  selectedPlatform: string;
  applyFilters: LogicValue;
}

export interface UseFetchTracesResult {
  spans: ConvertSpan[];
  loadMore: () => void;
  noMore: boolean;
  loading: boolean;
  loadingMore: boolean;
  spansMutate: unknown;
  spansData?: {
    list: ConvertSpan[];
    hasMore?: boolean;
    pageToken?: string;
    requestId: number;
  };
  traceListCode: number;
}

export const useFetchTraces = (
  params: UseFetchTracesParams,
): UseFetchTracesResult => {
  const {
    fetchSpansFn,
    tracesRefreshKey,
    buildInFetchTracesAdvanceInfo,
    timestamps,
    fieldMetas,
    selectedSpanType,
    selectedPlatform,
    applyFilters,
  } = params;

  const [startTime, endTime] = timestamps;
  const { sendEvent } = useConfigContext();

  const { markStart } = usePerformance();

  const { setFieldMetas, customParams } = useTraceStore();

  const standardFilters: ListSpansRequest['filters'] = {
    query_and_or: (applyFilters?.query_and_or ??
      QueryRelation.And) as QueryRelation,
    filter_fields: (
      applyFilters?.filter_fields?.map(item => ({
        field_name: item.field_name,
        field_type: item.field_type as FieldType,
        values: item.values,
        query_type: item.query_type as QueryType,
        query_and_or: (applyFilters?.query_and_or ??
          QueryRelation.And) as QueryRelation,
        is_custom: item?.is_custom,
      })) ?? []
    ).filter(
      item =>
        !isEmpty(item.values) ||
        item.query_type === QueryType.NotExist ||
        item.query_type === QueryType.Exist,
    ),
  };

  const latestDataRef = useRef<{
    list: ConvertSpan[];
    hasMore?: boolean;
    pageToken?: string;
  }>({
    list: [],
  });
  const latestCountRef = useRef<number>(1);
  const [traceListCode, setTraceListCode] = useState<number>(0);

  const dependenceList = [
    customParams?.spaceID,
    startTime,
    endTime,
    JSON.stringify(applyFilters),
    fieldMetas,
    tracesRefreshKey,
  ];

  useEffect(
    () => () => {
      setFieldMetas(undefined);
    },
    [],
  );

  useEffect(() => {
    latestCountRef.current += 1;
  }, dependenceList);

  const {
    data,
    loading,
    loadMore,
    noMore,
    loadingMore,
    mutate: spansMutate,
  } = useInfiniteScroll<{
    list: ConvertSpan[];
    hasMore?: boolean;
    pageToken?: string;
    requestId: number;
  }>(
    async dataSource => {
      const requestId = ++latestCountRef.current;

      if (!fieldMetas) {
        return Promise.resolve({
          list: [],
          total: 0,
          requestId,
        });
      }
      markStart('trace_list_fetch');

      const { pageToken } = dataSource || {};
      const fetchParams: ListSpansRequest = {
        platform_type: selectedPlatform as PlatformType,
        start_time: startTime.toString(),
        end_time: endTime.toString(),
        workspace_id: customParams?.spaceID ?? '',
        filters: standardFilters,
        order_bys: [{ field: 'start_time', is_asc: false }],
        page_size: 30,
        span_list_type: selectedSpanType as SpanListType,
        page_token: pageToken,
      };

      const result = await fetchSpansFn(fetchParams as any);
      const { spans, has_more, next_page_token } = result;
      sendEvent?.(BIZ_EVENTS.cozeloop_observation_trace_list_query, {
        space_id: customParams?.spaceID ?? '',
        start_time: startTime,
        end_time: endTime,
        filters: JSON.stringify(keys(standardFilters)),
      });

      if (
        keys(standardFilters).some(key =>
          key.startsWith(AUTO_EVAL_FEEDBACK_PREFIX),
        )
      ) {
        sendEvent?.(BIZ_EVENTS.trace_filter_by_evaluator, {
          space_id: customParams?.spaceID ?? '',
        });
      }

      if (requestId === latestCountRef.current) {
        const convertSpans: ConvertSpan[] = spans.map(span => ({
          ...span,
          advanceInfoReady: false,
        }));

        latestDataRef.current = {
          list: [...(dataSource?.list || []), ...convertSpans],
          hasMore: has_more,
          pageToken: next_page_token,
        };

        const tracesAdvanceInfo = await fetchTracesAdvanceInfo({
          convertSpans,
          selectedSpanType,
          selectedPlatform,
          spaceID: customParams?.spaceID ?? '',
          startTime,
          endTime,
          buildInFetchTracesAdvanceInfo,
        });

        const advanceInfoMap = keyBy(tracesAdvanceInfo, 'trace_id');

        return {
          list: convertSpans.map(span => {
            const { tokens } = advanceInfoMap[span.trace_id] || {};

            return {
              ...span,
              tokens,
              advanceInfoReady: !!tokens,
              spanType: selectedSpanType,
            };
          }),
          hasMore: has_more,
          pageToken: next_page_token,
          requestId,
        };
      } else {
        return {
          ...latestDataRef.current,
          requestId,
        };
      }
    },
    {
      reloadDeps: dependenceList,
      onError(err) {
        const apiError = err as unknown as { code: string; message: string };
        if (`${apiError.code}` !== `${TRACE_EXPIRED_CODE}`) {
          Toast.error(apiError.message);
        }
        setTraceListCode(Number(apiError.code));
      },
      isNoMore: d => !d?.hasMore,
    },
  );

  return {
    spans:
      data?.list.map(span => ({
        ...span,
      })) || [],
    loadMore,
    noMore,
    loading,
    loadingMore,
    spansMutate,
    spansData: data,
    traceListCode,
  };
};
