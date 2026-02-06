// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
// 提供一个函数 接受spanid，根据spanid返回内容，如果有缓存则从缓存中返回，否则调用服务端接口返回

import { type OutputSpan } from '@cozeloop/api-schema/observation';

import { type DataSource } from '../types/params';

const spanDetailCache = new Map<string, OutputSpan>();

export function getSpanDetailCache(spanId: string): OutputSpan | undefined {
  return spanDetailCache.get(spanId);
}

export function setSpanDetailCache(spanId: string, span: OutputSpan): void {
  spanDetailCache.set(spanId, span);
}

export const getSpanDetailCacheOrFetch = async (
  spanId: string,
  getTraceSpanDetailData?: (params: {
    span_ids?: string[];
  }) => Promise<DataSource>,
): Promise<OutputSpan | undefined> => {
  const cached = getSpanDetailCache(spanId);
  if (cached) {
    return cached;
  }
  if (!spanId) {
    return undefined;
  }
  const detail = await getTraceSpanDetailData?.({ span_ids: [spanId] });
  const span = detail?.spans?.find(item => item.span_id === spanId);
  if (span) {
    setSpanDetailCache(spanId, span);
  }
  return span;
};
