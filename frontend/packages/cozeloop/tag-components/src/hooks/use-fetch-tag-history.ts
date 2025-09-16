// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useInfiniteScroll } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

interface TagHistoryParams {
  tagKeyId: string;
  target: HTMLElement | null;
}

interface Data {
  list: tag.TagInfo[];
  next_page_token: string | undefined;
}

export const useFetchTagHistory = (params: TagHistoryParams) => {
  const { tagKeyId, target } = params;
  const { spaceID } = useSpace();

  const service = useInfiniteScroll<Data>(
    async data => {
      const { next_page_token, tags } = await DataApi.GetTagDetail({
        workspace_id: spaceID,
        tag_key_id: tagKeyId,
        page_token: data?.next_page_token,
      });

      return {
        list: tags ?? [],
        next_page_token,
      };
    },
    {
      target,
      isNoMore: data => !data?.next_page_token,
    },
  );

  return service;
};