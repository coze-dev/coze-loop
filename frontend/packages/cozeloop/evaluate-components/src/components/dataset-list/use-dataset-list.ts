// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { usePagination } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluationSet,
  type ListEvaluationSetsRequest as ListEvaluationSetsReq,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

import { useDatasetListStore } from '../../stores/dataset-list-store';
import { DEFAULT_PAGE_SIZE } from '../../const';

export const useDatasetList = () => {
  const {
    filter: filterStore,
    setFilter: setFilterStore,
    page,
    setPage,
  } = useDatasetListStore(
    useShallow(state => ({
      filter: state.filter,
      setFilter: state.setFilter,
      page: state.page,
      setPage: state.setPage,
    })),
  );
  const [filter, setFilter] =
    useState<Partial<ListEvaluationSetsReq>>(filterStore);
  const { spaceID } = useSpace();

  const service = usePagination(
    async (paginationData: {
      current: number;
      pageSize?: number;
    }): Promise<{
      total: number;
      list: EvaluationSet[];
    }> => {
      const { current, pageSize } = paginationData;
      const res = await StoneEvaluationApi.ListEvaluationSets({
        ...filter,
        workspace_id: spaceID ?? 0,
        page_size: pageSize ?? DEFAULT_PAGE_SIZE,
        page_number: current,
      });
      return {
        total: Number(res.total) ?? 0,
        list: res.evaluation_sets ?? [],
      };
    },
    {
      defaultPageSize: DEFAULT_PAGE_SIZE,
      defaultCurrent: page,
      refreshDeps: [filter],
    },
  );

  useEffect(() => {
    setPage(service.pagination.current);
  }, [service.pagination.current]);

  const onFilterChange = (values: Partial<ListEvaluationSetsReq>) => {
    setFilter(values);
    setFilterStore({ name: values?.name });
  };

  return {
    service,
    onFilterChange,
    filter,
    setFilter,
  };
};
