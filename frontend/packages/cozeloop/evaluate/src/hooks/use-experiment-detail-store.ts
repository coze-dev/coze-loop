import { useEffect, useState } from 'react';

import {
  type Params,
  type PaginationResult,
} from 'ahooks/lib/usePagination/types';
import { useDebounceFn, usePagination } from 'ahooks';
import {
  DEFAULT_PAGE_SIZE,
  filterToFilters,
  type LogicFilter,
  type SemiTableSort,
} from '@cozeloop/evaluate-components';
import { getStoragePageSize } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type KeywordSearch,
  type BatchGetExperimentResultResponse,
  type FieldType,
} from '@cozeloop/api-schema/evaluation';

import { batchGetExperimentResult } from '@/request/experiment';

function isFilterConditionExist(
  filter: object | undefined,
  logicFilter: LogicFilter | undefined,
) {
  const hasFilter = Object.values(filter ?? {}).some(
    val => val !== undefined && val !== null && val !== '',
  );
  const hasLogicFilter = logicFilter?.exprs?.some(expr => {
    const { right } = expr;
    const hasRight = right !== undefined && right !== null && right !== '';
    return hasRight;
  });
  return hasFilter || hasLogicFilter || false;
}

interface IReturnType<RecordItem, Filter> {
  service: PaginationResult<
    {
      total: number;
      list: RecordItem[];
      result: BatchGetExperimentResultResponse;
    },
    Params
  >;
  expand: boolean;
  setExpand: React.Dispatch<React.SetStateAction<boolean>>;
  filter: Filter | undefined;
  setFilter: React.Dispatch<React.SetStateAction<Filter | undefined>>;
  onFilterDebounceChange: () => void;
  logicFilter: LogicFilter | undefined;
  setLogicFilter: React.Dispatch<React.SetStateAction<LogicFilter | undefined>>;
  onLogicFilterChange: (newLogicFilter: LogicFilter | undefined) => void;
  hasFilterCondition: boolean;
  sort: SemiTableSort | undefined;
  onSortChange: (sorter: SemiTableSort) => void;
}

// eslint-disable-next-line max-lines-per-function
export function useExperimentDetailStore<
  RecordItem extends { groupIndex: number },
  Filter extends object = {},
>({
  experimentIds,
  filterFields = [],
  refreshKey,
  pageSizeStorageKey,
  keywordSearch,
  experimentResultToRecordItems,
}: {
  experimentIds: string[] | undefined;
  filterFields?: { key: keyof Filter; type: FieldType }[];
  refreshKey?: string | number;
  pageSizeStorageKey?: string;
  /** 关键词搜索 */
  keywordSearch?: KeywordSearch;
  experimentResultToRecordItems: (
    result: BatchGetExperimentResultResponse,
  ) => RecordItem[];
}): IReturnType<RecordItem, Filter> {
  const { spaceID } = useSpace();
  const [logicFilter, setLogicFilter] = useState<LogicFilter | undefined>();
  const [filter, setFilter] = useState<Filter>();
  const [sort, setSort] = useState<SemiTableSort | undefined>();
  const [expand, setExpand] = useState(false);

  const baseExperimentID = experimentIds?.[0] ?? '';

  const service = usePagination(
    async (params: {
      current: number;
      pageSize?: number;
      filter?: Filter;
      logicFilter?: LogicFilter;
      // 排序331版本暂未支持，先注释掉
      // sort?: SemiTableSort;
      keywordSearch?: KeywordSearch;
    }) => {
      const { current = 1, pageSize, ...rest } = params ?? {};
      const filters = filterToFilters<Filter>({ ...rest, filterFields });
      const res = await batchGetExperimentResult({
        workspace_id: spaceID,
        page_number: current,
        page_size: pageSize,
        filters: {
          [baseExperimentID]: { filters, keyword_search: keywordSearch },
        },
        baseline_experiment_id: baseExperimentID,
        experiment_ids: experimentIds ?? [],
        // 开启模糊匹配搜索
        use_accelerator: true,
      });
      const list = experimentResultToRecordItems(res);
      return {
        total: Number(res.total ?? 0),
        list,
        result: res,
      };
    },
    {
      defaultPageSize:
        getStoragePageSize(pageSizeStorageKey) || DEFAULT_PAGE_SIZE,
      manual: true,
    },
  );

  const { run: onFilterDebounceChange } = useDebounceFn(
    () => {
      service.run({
        current: 1,
        pageSize: service.pagination?.pageSize,
        filter,
        logicFilter,
        sort,
        keywordSearch,
      });
    },
    { wait: 600 },
  );

  const onLogicFilterChange = (newLogicFilter: LogicFilter | undefined) => {
    setLogicFilter(newLogicFilter ?? {});
    service.run({
      current: 1,
      pageSize: service.pagination?.pageSize,
      filter,
      logicFilter: newLogicFilter,
      sort,
      keywordSearch,
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
      keywordSearch,
    });
  };

  /** 是否有筛选条件 */
  const hasFilterCondition = isFilterConditionExist(filter, logicFilter);

  // 初始加载请求数据
  useEffect(() => {
    if (!experimentIds?.length) {
      return;
    }
    service.run({
      current: 1,
      pageSize: service.pagination?.pageSize,
      filter,
      logicFilter,
      sort,
      keywordSearch,
    });
  }, [experimentIds]);

  useEffect(() => {
    if (refreshKey) {
      service.refresh();
    }
  }, [refreshKey]);

  return {
    service,
    expand,
    setExpand,
    filter,
    setFilter,
    onFilterDebounceChange,
    logicFilter,
    setLogicFilter,
    onLogicFilterChange,
    hasFilterCondition,
    sort,
    onSortChange,
  };
}
