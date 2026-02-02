// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useSearchParams } from 'react-router-dom';
import { useState, useEffect } from 'react';

import { usePagination } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type OrderBy, type tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

const DEFAULT_PAGE_SIZE = 10;

export type TagInfo = tag.TagInfo;
export const useSearchTags = () => {
  const { spaceID } = useSpace();
  const [searchParams, setSearchParams] = useSearchParams();

  // 从URL中读取初始值
  const initialSearchValue = searchParams.get('search') || '';
  const initialContentTypes = searchParams.get('contentTypes')
    ? ((searchParams.get('contentTypes') || '').split(
        ',',
      ) as tag.TagContentType[])
    : undefined;
  const initialCreatedBys = searchParams.get('createdBys')
    ? (searchParams.get('createdBys') || '').split(',')
    : undefined;
  const initialOrderBy = searchParams.get('orderBy')
    ? JSON.parse(searchParams.get('orderBy') || '{}')
    : undefined;

  const [searchValue, setSearchValue] = useState(initialSearchValue);
  const [contentTypes, setContentTypes] = useState<
    tag.TagContentType[] | undefined
  >(initialContentTypes);
  const [createdBys, setCreatedBys] = useState<string[] | undefined>(
    initialCreatedBys,
  );
  const [orderBy, setOrderBy] = useState<OrderBy>(initialOrderBy as OrderBy);

  // 同步参数到URL
  useEffect(() => {
    const newSearchParams = new URLSearchParams(searchParams);

    if (searchValue) {
      newSearchParams.set('search', searchValue);
    } else {
      newSearchParams.delete('search');
    }

    if (contentTypes && contentTypes.length > 0) {
      newSearchParams.set('contentTypes', contentTypes.join(','));
    } else {
      newSearchParams.delete('contentTypes');
    }

    if (createdBys && createdBys.length > 0) {
      newSearchParams.set('createdBys', createdBys.join(','));
    } else {
      newSearchParams.delete('createdBys');
    }

    if (orderBy) {
      newSearchParams.set('orderBy', JSON.stringify(orderBy));
    } else {
      newSearchParams.delete('orderBy');
    }

    setSearchParams(newSearchParams, { replace: true });
  }, [
    searchValue,
    contentTypes,
    createdBys,
    searchParams,
    setSearchParams,
    orderBy,
  ]);

  const service = usePagination(
    async (paginationData: {
      current: number;
      pageSize?: number;
    }): Promise<{ total: number; list: TagInfo[] }> => {
      const { current, pageSize } = paginationData;

      const res = await DataApi.SearchTags({
        workspace_id: spaceID,
        page_number: current,
        page_size: pageSize ?? DEFAULT_PAGE_SIZE,
        created_bys: createdBys,
        tag_key_name_like: searchValue,
        content_types: contentTypes,
        order_by: orderBy,
      });

      return {
        total: isNaN(Number(res.total)) ? 0 : Number(res.total),
        list: res.tagInfos ?? [],
      };
    },
    {
      refreshDeps: [spaceID, searchValue, contentTypes, createdBys, orderBy],
    },
  );

  return {
    service,
    searchValue,
    setSearchValue,
    createdBys,
    setCreatedBys,
    contentTypes,
    setContentTypes,
    orderBy,
    setOrderBy,
  };
};
