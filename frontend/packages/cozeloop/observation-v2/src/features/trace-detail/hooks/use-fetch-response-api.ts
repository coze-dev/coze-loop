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
