/* eslint-disable complexity */
import { useState } from 'react';

import { cloneDeep, uniqWith } from 'lodash-es';
import { useRequest } from 'ahooks';
import {
  type GetTraceResponse,
  type span,
  type PlatformType,
} from '@cozeloop/api-schema/observation';

import { BIZ_EVENTS } from '@/shared/constants';
import { spans2SpanNodes } from '@/features/trace-detail/utils/span';
import { type DataSource } from '@/features/trace-detail/types/params';
import { type SpanNode } from '@/features/trace-detail/components/graphs/trace-tree/type';
import { useConfigContext } from '@/config-provider';

import { useTraceDetailContext } from './use-trace-detail-context';

interface UseFetchSpansInput {
  getTraceDetailData?:
    | ((params?: {
        trace_id: string;
        start_time: string;
        end_time: string;
        platform_type: PlatformType;
        span_ids?: string[];
      }) => Promise<GetTraceResponse>)
    | GetTraceResponse;
  spanId?: string;
}

export const useFetchSpans = ({
  getTraceDetailData,
  spanId,
}: UseFetchSpansInput) => {
  const [isReady, setIsReady] = useState(false);
  const { sendEvent } = useConfigContext();
  const [statusCode, setStatusCode] = useState(0);
  const { customParams } = useTraceDetailContext();
  const {
    data: traceInfo,
    loading,
    refresh,
  } = useRequest(
    async () => {
      let data: DataSource | undefined = undefined;
      if (typeof getTraceDetailData === 'function') {
        const res = await getTraceDetailData();
        data = {
          spans: res.spans,
          advanceInfo: res.traces_advance_info,
        };
      } else {
        data = getTraceDetailData;
      }

      const { spans: rawSpans, advanceInfo } = data || {};
      let resSpans = uniqWith(
        rawSpans,
        (span1: span.OutputSpan, span2: span.OutputSpan) =>
          span1.span_id === span2.span_id,
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

        resSpans = resSpans.map((span: span.OutputSpan) => {
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
      refreshDeps: [spanId],
      onFinally() {
        setIsReady(true);
      },
      onSuccess({ spans: resSpans, spanNodes }) {
        if (resSpans && resSpans.length > 0) {
          sendEvent?.(BIZ_EVENTS.cozeloop_observation_trace_get_trace_detail, {
            space_id: customParams?.spaceID ?? '',
            space_name: customParams?.spaceName ?? '',
            span_count: resSpans.length,
            psm: spanNodes?.[0].custom_tags?.psm ?? '',
            platform_type: customParams?.platformType ?? '',
            module_name: customParams?.moduleName ?? '',
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
