// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { cloneDeep, uniqWith } from 'lodash-es';
import { useRequest } from 'ahooks';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { type PlatformType } from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';

import dayjs from '@/utils/dayjs';

import { spans2SpanNodes } from '../utils/span';
import { type DataSource } from '../typings/params';
import { type SpanNode } from '../components/graphs/trace-tree/type';

interface UseFetchSpansInput {
  spaceID: string;
  spaceName: string;
  id: string;
  searchType: 'trace_id';
  startTime?: number | string;
  endTime?: number | string;
  platformType?: string;
  moduleName?: string;
  options?: {
    dataSource?: DataSource;
  };
}

export const useFetchSpans = ({
  spaceID,
  searchType,
  spaceName,
  id,
  endTime,
  platformType = 'cozeloop',
  startTime,
  moduleName,
  options: { dataSource } = {},
}: UseFetchSpansInput) => {
  const [isReady, setIsReady] = useState(false);
  const [statusCode, setStatusCode] = useState(0);
  const {
    data: traceInfo,
    loading,
    refresh,
  } = useRequest(
    async () => {
      let data = dataSource;
      if (dataSource === undefined) {
        const amendEndTime = endTime || dayjs().valueOf();
        const amendStartTime =
          startTime || dayjs().subtract(7, 'day').valueOf();

        const res = await observabilityTrace.GetTrace(
          {
            start_time: amendStartTime.toString(),
            end_time: amendEndTime.toString(),
            trace_id: id,
            workspace_id: spaceID,
            platform_type: platformType as PlatformType,
          },
          {
            disableErrorToast: true,
          },
        );

        data = {
          spans: res.spans,
          advanceInfo: res.traces_advance_info,
        };
      }

      const { spans: rawSpans, advanceInfo } = data || {};
      let resSpans = uniqWith(
        rawSpans,
        (span1, span2) => span1.span_id === span2.span_id,
      );

      const spanNodes = spans2SpanNodes(resSpans);
      const tokens = advanceInfo?.tokens;
      if (spanNodes?.length === 1 && tokens) {
        const newRootSpan: SpanNode = {
          ...spanNodes[0],
          custom_tags: {
            ...(spanNodes[0].custom_tags ?? {}),
          },
        };

        spanNodes[0] = newRootSpan;

        resSpans = resSpans.map(span => {
          if (span.span_id === newRootSpan.span_id) {
            return cloneDeep(newRootSpan);
          }
          return span;
        });
      }

      return {
        spans: resSpans,
        spanNodes,
        advanceInfo,
      };
    },
    {
      ready: Boolean(id) || dataSource !== undefined,
      refreshDeps: [
        id,
        spaceID,
        platformType,
        startTime,
        endTime,
        searchType,
        dataSource,
      ],
      onFinally() {
        setIsReady(true);
      },
      onSuccess({ spans: resSpans, spanNodes }) {
        if (resSpans && resSpans.length > 0) {
          sendEvent(EVENT_NAMES.cozeloop_observation_trace_get_trace_detail, {
            space_id: spaceID,
            space_name: spaceName,
            span_count: resSpans.length,
            psm: spanNodes?.[0].custom_tags?.psm ?? '',
            platform_type: platformType,
            module_name: moduleName,
            is_break: spanNodes !== undefined && spanNodes?.length > 1,
            break_node_count: spanNodes?.length || 0,
          });
        }
      },
      onError(error) {
        const apiError = error as unknown as {
          code: string;
          message: string;
        };
        setStatusCode(Number(apiError.code));
      },
    },
  );

  return {
    roots: traceInfo?.spanNodes,
    spans: traceInfo?.spans || [],
    advanceInfo: traceInfo?.advanceInfo,
    loading: loading || !isReady,
    refresh,
    statusCode,
  };
};
