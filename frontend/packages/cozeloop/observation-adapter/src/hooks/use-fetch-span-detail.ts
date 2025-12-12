// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type PlatformType, type span } from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';

const getTime = (spans: span.OutputSpan[]) => {
  const startTimes = spans.map(s => s.started_at).map(Number);
  const minStartTime = Math.min(...startTimes);
  const maxStartTime = Math.max(...startTimes);
  return {
    startTime: minStartTime,
    endTime: maxStartTime,
  };
};

interface UseFetchSpanDetailParams {
  spans: span.OutputSpan[];
  platformType?: string | number;
  getStartTime: (startTime: number | string) => string;
  getEndTime: (endTime: number | string, latency: number | string) => string;
}

export const useFetchSpanDetail = ({
  spans,
  platformType,
  getStartTime,
  getEndTime,
}: UseFetchSpanDetailParams) => {
  const { spaceID } = useSpace();
  const service = useRequest(
    async () => {
      if (!spaceID || !spans?.length) {
        return [];
      }
      const { startTime, endTime } = getTime(spans);
      const res = await observabilityTrace.GetTrace({
        start_time: getStartTime(startTime),
        end_time: getEndTime(endTime, spans[0].duration),
        trace_id: spans?.[0]?.trace_id,
        workspace_id: spaceID,
        platform_type: platformType as PlatformType,
        span_ids: [spans?.[0]?.span_id ?? ''],
      });
      return [res.spans?.[0] ?? spans?.[0], ...(spans?.slice(1) ?? [])];
    },
    {
      manual: true,
    },
  );

  return service;
};
