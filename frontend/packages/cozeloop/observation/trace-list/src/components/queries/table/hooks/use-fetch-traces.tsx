// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */

import { useEffect, useMemo, useRef, useState } from 'react';

import { isEmpty, keyBy, keys } from 'lodash-es';
import { useInfiniteScroll } from 'ahooks';
import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type PlatformType,
  type SpanListType,
  type ListSpansRequest,
  type QueryType,
  QueryRelation,
  FieldType,
} from '@cozeloop/api-schema/observation';
import { tag } from '@cozeloop/api-schema/data';
import { observabilityTrace } from '@cozeloop/api-schema';
import { logger } from '@coze-arch/logger';
import { Toast } from '@coze-arch/coze-design';

import { type ConvertSpan } from '@/typings/span';
import { useTraceStore } from '@/stores/trace';
import { usePerformance } from '@/hooks/use-performance';
import {
  AUTO_EVAL_FEEDBACK,
  AUTO_EVAL_FEEDBACK_PREFIX,
  MANUAL_FEEDBACK,
  MANUAL_FEEDBACK_PREFIX,
} from '@/components/logic-expr/const';

import { TRACE_EXPIRED_CODE } from '..';

const TAG_CONTENT_MAPPDING_TYPE = {
  [tag.TagContentType.Categorical]: FieldType.Long,
  [tag.TagContentType.Boolean]: FieldType.Long,
  [tag.TagContentType.FreeText]: FieldType.String,
  [tag.TagContentType.ContinuousNumber]: FieldType.Double,
};

const getFieldType = (
  fieldName: string,
  fieldMetas?: Record<string, any>,
  item?: any,
) => {
  if (item?.extraInfo) {
    const { content_type } = item.extraInfo ?? {};

    return TAG_CONTENT_MAPPDING_TYPE[content_type] ?? FieldType.String;
  }
  if (!fieldMetas) {
    return FieldType.String;
  }

  let filedKey = fieldName;
  if (fieldName.startsWith(AUTO_EVAL_FEEDBACK_PREFIX)) {
    filedKey = AUTO_EVAL_FEEDBACK;
  }

  if (fieldName.startsWith(MANUAL_FEEDBACK_PREFIX)) {
    filedKey = MANUAL_FEEDBACK;
  }
  return fieldMetas?.[filedKey]?.value_type ?? FieldType.String;
};

export const useFetchTraces = () => {
  const { spaceID } = useSpace();

  const { markStart } = usePerformance();

  const {
    timestamps: [startTime, endTime],
    fieldMetas,
    refreshFlag,
    selectedSpanType,
    setFieldMetas,
    applyFilters,
    selectedPlatform,
  } = useTraceStore();

  const standardFilters: ListSpansRequest['filters'] = useMemo(
    () => ({
      query_and_or: (applyFilters?.query_and_or ??
        QueryRelation.And) as QueryRelation,
      filter_fields:
        applyFilters?.filter_fields?.map(item => ({
          field_name: item.field_name,
          field_type: getFieldType(item.field_name, fieldMetas, item),
          values: item.values,
          query_type: item.query_type as QueryType,
          query_and_or: (applyFilters?.query_and_or ??
            QueryRelation.And) as QueryRelation,
        })) ?? [],
    }),
    [applyFilters, fieldMetas],
  );

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
    spaceID,
    startTime,
    endTime,
    refreshFlag,
    applyFilters,
    selectedSpanType,
    fieldMetas,
    selectedSpanType,
    selectedPlatform,
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

      /**
       * 由于当前服务端filters字段仍然是v1老版本，前端需将v2版本数据进行转换
       * 适配，适配方法依赖fieldMetas，因此需要fieldMetas请求完成后再调用,
       * 后续切换为v2后可以删除此逻辑
       */
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
        workspace_id: spaceID,
        filters: standardFilters,
        order_bys: [{ field: 'start_time', is_asc: false }],
        page_size: 30,
        span_list_type: selectedSpanType as SpanListType,
        page_token: pageToken,
      };

      const result = await observabilityTrace.ListSpans(fetchParams);
      const { spans, has_more, next_page_token } = result;
      sendEvent(EVENT_NAMES.cozeloop_observation_trace_list_query, {
        space_id: spaceID,
        start_time: startTime,
        end_time: endTime,
        filters: JSON.stringify(keys(standardFilters)),
      });

      if (
        keys(standardFilters).some(key =>
          key.startsWith(AUTO_EVAL_FEEDBACK_PREFIX),
        )
      ) {
        sendEvent(EVENT_NAMES.trace_filter_by_evaluator, {
          space_id: spaceID,
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

        let tracesAdvanceInfo = {};
        try {
          const traces = convertSpans.map(item => ({
            trace_id: item.trace_id,
            start_time: (Number(item.started_at) ?? startTime).toString(),
            end_time: (
              Number(item.started_at) + (Number(item.duration) ?? endTime)
            ).toString(),
          }));
          if (selectedSpanType === 'root_span' && !isEmpty(traces)) {
            const advanceResult =
              await observabilityTrace.BatchGetTracesAdvanceInfo({
                traces,
                platform_type: selectedPlatform as PlatformType,
                workspace_id: spaceID,
              });

            tracesAdvanceInfo = advanceResult?.traces_advance_info ?? {};
          }
        } catch (error) {
          logger.error({ error: error as Error });
        }

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
