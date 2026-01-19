// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { type ListAnnotationsRequest } from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';

import { useTraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';

interface ListAnnotationsParams {
  span_id: string;
  trace_id: string;
  start_time: string;
  platform_type?: string | number;
  desc_by_updated_at?: boolean;
}

export const useListAnnotations = (params: ListAnnotationsParams) => {
  const { customParams } = useTraceDetailContext();

  const service = useRequest(
    async (descByUpdatedAt: boolean) => {
      const { annotations } = await observabilityTrace.ListAnnotations({
        workspace_id: customParams?.spaceID ?? '',
        ...params,
        desc_by_updated_at: descByUpdatedAt,
      } as unknown as ListAnnotationsRequest);
      return annotations;
    },
    {
      manual: true,
    },
  );

  return service;
};
