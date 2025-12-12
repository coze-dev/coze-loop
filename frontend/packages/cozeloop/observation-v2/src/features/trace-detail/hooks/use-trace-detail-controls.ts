/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
import { useCallback, useEffect, useMemo, useState } from 'react';

import { useRequest, useUpdateEffect } from 'ahooks';
import {
  type FilterFields,
  type OutputSpan,
} from '@cozeloop/api-schema/observation';

import {
  changeSpanNodeCollapseStatus as changeCollapse,
  toggleAllSpanNodes,
  checkAllNodesExpanded,
  findPathToSpan,
} from '@/features/trace-detail/utils/span';
import { sortMatchedSpans } from '@/features/trace-detail/utils/sort-matched-spans';
import { getSpanDetailCacheOrFetch } from '@/features/trace-detail/utils/get-span-detail-cache';
import { filterCriticalNodes } from '@/features/trace-detail/utils/filter-critical-nodes';
import { type DataSource } from '@/features/trace-detail/types/params';
import { DEFAULT_KEY_SPAN_TYPE } from '@/features/trace-detail/constants/span';
import { type SpanNode } from '@/features/trace-detail/components/graphs/trace-tree/type';

export interface SelectedSpanService {
  data?: OutputSpan | undefined;
  loading: boolean;
  error?: Error | undefined;
}
export interface SearchService {
  data?: OutputSpan[] | undefined;
  loading: boolean;
  error?: Error | undefined;
}

export interface TraceDetailControls {
  /** 受控：当前选中的节点 SpanId */
  selectedSpanId: string;
  /** 受控：当前选中的节点完整 Span 数据（用于右侧详情） */
  selectedSpanService: SelectedSpanService;
  /** 受控：调用树的根节点集合（受折叠影响） */
  rootNodes?: SpanNode[];

  /** 受控：搜索相关状态 */
  matchedSpanIds: string[] | undefined;
  searchFilters: FilterFields;
  setSearchFilters: (filters: FilterFields) => void;
  searchService: SearchService;

  /** 受控：展开收起相关状态 */
  isAllExpanded: boolean;

  /** 受控：过滤非关键节点状态 */
  filterNonCritical: boolean;
  onFilterNonCriticalChange: (filter: boolean) => void;

  /** 动作：清空搜索（不改变展开与选择） */
  onClear: () => void;
  /** 动作：选择某个节点（会展开祖先并设置选中） */
  onSelect: (spanId: string) => void;
  /** 动作：一键展开收起所有节点 */
  onToggleAll: () => void;

  /** 工具：根据 spanId 展开所有折叠的祖先 */
  expandAncestorsBySpanId: (spanId: string) => void;
  /** 工具：切换某个节点的折叠状态 */
  changeSpanNodeCollapseStatus: (id: string) => void;
  /** 工具：设置根节点（用于一键展开收起） */
  setRootNodes: (nodes: SpanNode[]) => void;
}

interface UseTraceDetailControlsParams {
  /** 是否支持Trace搜索能力 */
  enableTraceSearch?: boolean;
  /** 外部提供的根节点（首次/刷新时来自接口） */
  roots?: SpanNode[];
  /** 全量 Span 列表用于搜索与选中 */
  spans: OutputSpan[];
  /** 默认选择的 SpanId（在 roots 刷新后生效） */
  defaultSpanID?: string;
  getTraceDetailData?: (params: {
    filters?: FilterFields;
  }) => Promise<DataSource>;

  /** 外部提供的获取 Span 详情的函数 */
  getTraceSpanDetailData?: (params: {
    span_ids?: string[];
  }) => Promise<DataSource>;
  /** 外部提供的关键节点类型列表 */
  keySpanType?: string[];
}

/**
 * 将 trace 详情页的选择/搜索/折叠等受控状态统一封装为一个 Hook，向下游组件仅传递单一结构体，降低 props 数量与耦合。
 */
export function useTraceDetailControls(
  params: UseTraceDetailControlsParams,
): TraceDetailControls {
  const {
    roots,
    spans,
    defaultSpanID,
    getTraceSpanDetailData,
    getTraceDetailData,
    keySpanType = DEFAULT_KEY_SPAN_TYPE,
    enableTraceSearch = false,
  } = params;

  const [rootNodes, setRootNodes] = useState<SpanNode[] | undefined>(undefined);
  const [selectedSpanId, setSelectedSpanId] = useState<string>('');
  const [filterNonCritical, setFilterNonCritical] = useState<boolean>(false);

  const [matchedSpanIds, setMatchedSpanIds] = useState<string[] | undefined>(
    undefined,
  );
  const [searchFilters, setSearchFilters] = useState<FilterFields>({
    filter_fields: [],
  });

  useUpdateEffect(() => {
    if (defaultSpanID) {
      setSelectedSpanId(defaultSpanID);
    } else if (roots && roots.length > 0) {
      setSelectedSpanId(roots[0].span_id);
    }
  }, [defaultSpanID, roots]);

  useEffect(() => {
    setRootNodes(roots);
  }, [roots]);
  const selectedSpanService = useRequest(
    async () => {
      if (!enableTraceSearch) {
        return (
          spans?.find(span => span.span_id === selectedSpanId) ?? undefined
        );
      }
      const spanDetail = await getSpanDetailCacheOrFetch(
        selectedSpanId,
        getTraceSpanDetailData,
      );
      return spanDetail;
    },
    {
      ready: Boolean(spans?.length),
      refreshDeps: [selectedSpanId],
    },
  );

  // 检查是否所有节点都已展开
  const isAllExpanded = useMemo(
    () => (rootNodes ? checkAllNodesExpanded(rootNodes) : false),
    [rootNodes],
  );

  const expandAncestorsBySpanId = (targetId: string) => {
    if (!rootNodes || !targetId) {
      return;
    }
    const path = findPathToSpan(targetId, rootNodes);
    if (!path.length) {
      return;
    }
    let updated = rootNodes;
    path.forEach(node => {
      if (node.isCollapsed) {
        updated = changeCollapse(updated, node.span_id);
      }
    });
    setRootNodes(updated);
  };

  const changeSpanNodeCollapseStatus = useCallback(
    (id: string) => {
      if (!rootNodes || !id) {
        return;
      }
      const updated = changeCollapse(rootNodes, id);
      setRootNodes(updated);
    },
    [rootNodes],
  );

  const onSelect = (spanId: string) => {
    if (!spanId) {
      return;
    }
    // 展开祖先并选中
    expandAncestorsBySpanId(spanId);
    setSelectedSpanId(spanId);
  };

  const searchService = useRequest(
    async () => {
      if (!enableTraceSearch) {
        return [];
      }
      if (!searchFilters || searchFilters.filter_fields.length === 0) {
        setMatchedSpanIds(undefined);
        return [];
      }
      const searchRes = await getTraceDetailData?.({
        filters: searchFilters,
      });
      const searchSpans = searchRes?.spans || [];
      const sortedSpans = sortMatchedSpans(searchSpans, rootNodes);
      setMatchedSpanIds(sortedSpans);
      // 对所有的sortedSpans展开祖先
      // sortedSpans.forEach(span => expandAncestorsBySpanId(span));
      // 自动跳转到第一个命中并展开祖先
      const first = sortedSpans[0];
      if (first) {
        setSelectedSpanId(first);
        expandAncestorsBySpanId(first);
      }
      return searchSpans;
    },
    {
      debounceWait: 500,
      refreshDeps: [searchFilters],
    },
  );

  const onClear = useCallback(() => {
    setMatchedSpanIds(undefined);
    setSearchFilters({ filter_fields: [] });
  }, []);

  // 当搜索过滤条件变化时，自动关闭关键节点过滤
  useEffect(() => {
    if (searchFilters.filter_fields.length > 0 && filterNonCritical) {
      onFilterNonCriticalChange(false);
    }
  }, [searchFilters]);

  const onFilterNonCriticalChange = (checked: boolean) => {
    setFilterNonCritical(checked);
    if (roots) {
      const filteredRoots = filterCriticalNodes(roots, checked, 1, keySpanType);
      setRootNodes(filteredRoots);
      if (checked && filteredRoots?.[0]?.span_id) {
        setSelectedSpanId(filteredRoots?.[0]?.span_id);
      }
    }
  };

  // 一键展开收起处理函数
  const onToggleAll = useCallback(() => {
    if (rootNodes) {
      const updatedNodes = toggleAllSpanNodes(rootNodes, !isAllExpanded);
      setRootNodes(updatedNodes);
    }
  }, [rootNodes, isAllExpanded]);

  const setRootNodesDirectly = useCallback((nodes: SpanNode[]) => {
    setRootNodes(nodes);
  }, []);

  return {
    selectedSpanId,
    selectedSpanService,
    rootNodes,
    matchedSpanIds,
    searchFilters,
    setSearchFilters,
    searchService: {
      data: searchService.data,
      loading: searchService.loading,
      error: searchService.error,
    },
    isAllExpanded,
    filterNonCritical,
    onFilterNonCriticalChange,
    onClear,
    onSelect,

    onToggleAll,
    expandAncestorsBySpanId,
    changeSpanNodeCollapseStatus,
    setRootNodes: setRootNodesDirectly,
  };
}
