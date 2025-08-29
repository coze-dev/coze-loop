import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { DataApi } from '@cozeloop/api-schema';

export const useBatchGetTags = () => {
  const { spaceID } = useSpace();
  const service = useRequest(
    async (tagKeyIds: string[]) => {
      const result = await DataApi.BatchGetTags({
        workspace_id: spaceID,
        tag_key_ids: tagKeyIds,
      });
      return result;
    },
    {
      manual: true,
    },
  );
  return service;
};
