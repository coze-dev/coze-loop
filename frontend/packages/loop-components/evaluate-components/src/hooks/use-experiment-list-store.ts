// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
import { useEffect, useState } from 'react';

import {
  type Params,
  type PaginationResult,
} from 'ahooks/lib/usePagination/types';
import { useDebounceFn, usePagination } from 'ahooks';
import {
  DEFAULT_PAGE_SIZE,
  getStoragePageSize,
  type TableColAction,
} from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type ListExperimentsRequest,
  type ListExperimentsResponse,
  type Experiment,
  type FieldType,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { type ColumnProps } from '@coze-arch/coze-design';

import { type SemiTableSort, sorterToOrderBy } from '../utils/order-by';
import { filterToFilters } from '../utils/evaluate-logic-condition';
import { type LogicFilter } from '../components/logic-editor';
import {
  useExperimentListColumns,
  type UseExperimentListColumnsProps,
} from './use-experiment-list-columns';

interface ServiceParams<Filter> {
  current: number;
  pageSize: number;
  filter?: Filter;
  logicFilter?: LogicFilter;
  sort?: SemiTableSort;
}

interface ExperimentListStore<Filter> {
  service: PaginationResult<
    {
      total: number;
      list: Experiment[];
    },
    Params
  >;
  columns: ColumnProps<Experiment>[];
  defaultColumns: ColumnProps<Experiment>[];
  setColumns: (columns: ColumnProps<Experiment>[]) => void;
  filter: Filter | undefined;
  setFilter: React.Dispatch<React.SetStateAction<Filter | undefined>>;
  logicFilter: LogicFilter | undefined;
  setLogicFilter: React.Dispatch<React.SetStateAction<LogicFilter | undefined>>;
  hasFilterCondition: boolean;
  isDatabaseEmpty: boolean;
  sort: SemiTableSort | undefined;
  setSort: (sort: SemiTableSort | undefined) => void;
  batchOperate: boolean;
  setBatchOperate: React.Dispatch<React.SetStateAction<boolean>>;
  selectedExperiments: Experiment[];
  setSelectedExperiments: React.Dispatch<React.SetStateAction<Experiment[]>>;
  refreshAsync: () => Promise<void>;
  onFilterDebounceChange: () => void;
  onLogicFilterChange: (logicFilter: LogicFilter | undefined) => void;
  onSortChange: (sort: SemiTableSort) => void;
}

function isFilterConditionExist(
  filter: Record<string, unknown> | undefined,
  logicFilter: LogicFilter | undefined,
) {
  const hasFilter = Object.values(filter ?? {}).some(
    val =>
      val !== undefined &&
      val !== null &&
      val !== '' &&
      !(Array.isArray(val) && val.length === 0),
  );
  const hasLogicFilter = logicFilter?.exprs?.some(expr => {
    const { right } = expr;
    const hasRight =
      right !== undefined &&
      right !== null &&
      right !== '' &&
      !(Array.isArray(right) && right.length === 0);
    return hasRight;
  });
  return hasFilter || hasLogicFilter || false;
}

export type ExperimentListColumnsOptions = Omit<
  UseExperimentListColumnsProps,
  'spaceID' | 'onRefresh'
>;

/**
 * 实验列表状态管理hooks，主要管理筛选条件、排序等
 * desc: 主要用于有实验列表的页面状态复用
 */

// eslint-disable-next-line @coze-arch/max-line-per-function
export function useExperimentListStore<Filter extends { name?: string }>({
  defaultFilter,
  filterFields,
  columnsOptions = {},
  defaultPageSize,
  pageSizeStorageKey,
  pullExperiments,
  extraShrinkActions = [],
  source,
  baseNavgiateUrl,
  createUrl,
}: {
  /** 默认筛选值 */
  defaultFilter?: Filter;
  filterFields?: { key: keyof Filter; type: FieldType }[];
  columnsOptions?: ExperimentListColumnsOptions;
  defaultPageSize?: number;
  // 存储分页大小的storage key
  pageSizeStorageKey?: string;
  // 拉取实验列表的接口，默认使用 StoneEvaluationApi.PullExperiments
  pullExperiments?: (
    req: ListExperimentsRequest,
  ) => Promise<ListExperimentsResponse>;
  extraShrinkActions?: TableColAction[];
  source?: string;
  baseNavgiateUrl?: string;
  createUrl?: string;
} = {}): ExperimentListStore<Filter> {
  const { spaceID } = useSpace();
  const [filter, setFilter] = useState<Filter | undefined>(defaultFilter);
  const [logicFilter, setLogicFilter] = useState<LogicFilter | undefined>();
  const [sort, setSort] = useState<SemiTableSort | undefined>();
  const [batchOperate, setBatchOperate] = useState<boolean>(false);

  const [selectedExperiments, setSelectedExperiments] = useState<Experiment[]>(
    [],
  );
  const [isDatabaseEmpty, setIsDatabaseEmpty] = useState(false);

  const service = usePagination(
    async (
      params: ServiceParams<Filter>,
    ): Promise<{
      total: number;
      list: Experiment[];
    }> => {
      const { current, pageSize } = params;
      const filters = filterToFilters<Filter>({
        filter: params.filter,
        logicFilter: params.logicFilter,
        filterFields,
      });
      const reqParams: ListExperimentsRequest = {
        workspace_id: spaceID,
        page_number: current,
        page_size: pageSize,
        filter_option: { fuzzy_name: filter?.name, filters },
        order_bys: params.sort ? [sorterToOrderBy(params.sort)] : undefined,
      };
      let res: ListExperimentsResponse | undefined = undefined;
      if (pullExperiments) {
        res = await pullExperiments(reqParams);
      } else {
        res = await StoneEvaluationApi.ListExperiments(reqParams);
      }
      const list = res?.experiments ?? [];
      // 如果没有筛选条件，且没有实验数据，说明数据库是空的
      setIsDatabaseEmpty(() => {
        if (
          !isFilterConditionExist(params.filter, params.logicFilter) &&
          list.length === 0
        ) {
          return true;
        }
        return false;
      });
      return {
        total: Number(res?.total) || 0,
        list,
      };
    },
    {
      defaultPageSize:
        (getStoragePageSize(pageSizeStorageKey) || defaultPageSize) ??
        DEFAULT_PAGE_SIZE,
      manual: true,
    },
  );

  const { columns, defaultColumns, setColumns } = useExperimentListColumns({
    ...columnsOptions,
    spaceID,
    onRefresh: service.refresh,
    extraShrinkActions,
    source,
    baseNavgiateUrl,
    createUrl,
  });

  const { run: onFilterDebounceChange } = useDebounceFn(
    () => {
      service.run({
        current: 1,
        pageSize: service.pagination?.pageSize,
        filter,
        logicFilter,
        sort,
      });
    },
    { wait: 1000 },
  );

  const onLogicFilterChange = (newLogicFilter: LogicFilter | undefined) => {
    setLogicFilter(newLogicFilter ?? {});
    service.run({
      current: 1,
      pageSize: service.pagination?.pageSize,
      filter,
      logicFilter: newLogicFilter,
      sort,
    });
  };

  const onSortChange = (sorter: SemiTableSort) => {
    setSort(sorter);
    service.run({
      current: 1,
      pageSize: service.pagination?.pageSize,
      filter,
      logicFilter,
      sort: sorter,
    });
  };

  const refreshAsync = async () => {
    await service.refreshAsync();
  };

  /** 是否有筛选条件 */
  const hasFilterCondition = isFilterConditionExist(filter, logicFilter);

  // 初始加载请求数据
  useEffect(() => {
    service.run({
      current: 1,
      pageSize: service.pagination?.pageSize,
      filter,
      logicFilter,
      sort,
    });
  }, []);

  return {
    service,
    columns,
    defaultColumns,
    setColumns,
    filter,
    setFilter,
    logicFilter,
    setLogicFilter,
    hasFilterCondition,
    isDatabaseEmpty,
    sort,
    setSort,
    refreshAsync,
    batchOperate,
    setBatchOperate,
    selectedExperiments,
    setSelectedExperiments,
    onFilterDebounceChange,
    onLogicFilterChange,
    onSortChange,
  };
}
