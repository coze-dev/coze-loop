// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { type ListPreSpanRequest } from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';

export interface ResponseApiService {
  loading?: boolean;
}

export const useFetchResponseApi = () => {
  const responseApiService = useRequest(
    async (params: ListPreSpanRequest) => {
      const response = await observabilityTrace.ListPreSpan({
        ...params,
      });
      return response;
    },
    {
      manual: true,
    },
  );

  return responseApiService;
};
