import { type Result } from 'ahooks/lib/useRequest/src/types';
import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type GetTagDetailResponse } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

interface TagDetailParams {
  tagKeyID: string;
}

export const useFetchTagDetail: () => Result<
  GetTagDetailResponse,
  [TagDetailParams]
> = () => {
  const { spaceID } = useSpace();
  const service = useRequest(
    async ({ tagKeyID }: TagDetailParams) => {
      const result = await DataApi.GetTagDetail({
        workspace_id: spaceID,
        tag_key_id: tagKeyID,
        page_number: 1,
        page_size: 1,
      });
      return result;
    },
    {
      manual: true,
    },
  );

  return service;
};