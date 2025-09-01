import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { DataApi } from '@cozeloop/api-schema';

export const useGetTagSpec = () => {
  const { spaceID } = useSpace();

  const service = useRequest(
    async () =>
      await DataApi.GetTagSpec({
        workspace_id: spaceID,
      }),
  );

  return service;
};