// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useInfiniteScroll } from 'ahooks';
import { DEFAULT_PAGE_SIZE } from '@cozeloop/components';
import { type tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

interface TagListParams {
  spaceID?: string;
}

export const useTagList = ({ spaceID }: TagListParams) =>
  useInfiniteScroll<{
    list: tag.TagInfo[];
    cursorID: string;
    hasMore: boolean;
  }>(
    async dataSource => {
      if (!spaceID) {
        return {
          list: [],
          cursorID: '',
          hasMore: false,
        };
      }

      // 计算当前页码，如果没有数据源则从第1页开始
      const currentPageNumber = dataSource
        ? Math.ceil(dataSource.list.length / DEFAULT_PAGE_SIZE) + 1
        : 1;

      const resp = await DataApi.SearchTags({
        page_number: currentPageNumber,
        page_size: DEFAULT_PAGE_SIZE,
        workspace_id: spaceID,
      }).catch(() => undefined);

      const newList = resp?.tagInfos || [];
      const existingList = dataSource?.list || [];
      const combinedList = [...existingList, ...newList];

      // 使用total属性来判断是否还有更多数据
      const total = resp?.total ? parseInt(resp.total, 10) : 0;
      const hasMore = combinedList.length < total;

      return {
        list: combinedList,
        cursorID: resp?.next_page_token || '',
        hasMore,
      };
    },
    {
      manual: true,
      reloadDeps: [spaceID],
      isNoMore: dataSource => !dataSource?.hasMore,
    },
  );
