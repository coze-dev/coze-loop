// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { ErrorBoundary } from 'react-error-boundary';
import { useRef, useState, useMemo } from 'react';

import cls from 'classnames';
import {
  type OutputSpan,
  type FilterFields,
} from '@cozeloop/api-schema/observation';
import { Spin, Typography } from '@coze-arch/coze-design';

import { RunTreeEmpty, SearchEmptyComponent } from '@/shared/ui/empty-status';
import { useLocale } from '@/i18n';
import { type TraceSelectorProps } from '@/features/trace-selector';
import { useTraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';

import { getRootNodesLengthWithChildren } from '../../utils/span';
import { type SpanNode } from './trace-tree/type';
import { TraceTree } from './trace-tree';
import { TraceSearch, type TraceSearchRef } from './trace-search';

import styles from './index.module.less';

interface TraceGraphsProps {
  rootNodes?: SpanNode[];
  spans: OutputSpan[];
  selectedSpanId: string;
  /** 受控：搜索匹配到的 spanId 列表，用于高亮 */
  matchedSpanIds?: string[];
  onSelect: (id: string) => void;
  onCollapseChange: (id: string) => void;
  onToggleAll?: () => void;
  isAllExpanded?: boolean;
  /** 当前搜索过滤条件 */
  searchFilters?: FilterFields;
  /** 受控：提交搜索 */
  setSearchFilters?: (filters: FilterFields) => void;
  /** 受控：清空搜索 */
  onClear?: () => void;
  /** 是否过滤非关键节点 */
  filterNonCritical?: boolean;
  /** 切换过滤非关键节点 */
  onFilterNonCriticalChange?: (checked: boolean) => void;
  loading?: boolean;
  className?: string;
  treeSelectorConfig?: TraceSelectorProps;
  /** 是否开启Trace搜索能力 */
  enableTraceSearch?: boolean;
}

export const TraceGraphs = ({
  rootNodes,
  loading = false,
  selectedSpanId,
  matchedSpanIds,
  onSelect,
  onCollapseChange,
  onToggleAll,
  isAllExpanded = false,
  setSearchFilters,
  searchFilters,
  onClear,
  filterNonCritical = false,
  onFilterNonCriticalChange,
  className,
  treeSelectorConfig,
  spans,
  enableTraceSearch,
}: TraceGraphsProps) => {
  const { t } = useLocale();
  const { customParams } = useTraceDetailContext();
  // 精简过滤状态下，只展示matchedSpan 列表
  const [isSlimMode, setIsSlimMode] = useState(false);
  const traceSearchRef = useRef<TraceSearchRef>(null);
  const RunTreeEmptyComponent = customParams?.RunTreeEmpty ?? RunTreeEmpty;
  const isSearchEmpty =
    searchFilters &&
    searchFilters.filter_fields.length > 0 &&
    matchedSpanIds !== undefined &&
    matchedSpanIds.length === 0;
  const showSlimModel =
    isSlimMode && matchedSpanIds && matchedSpanIds?.length > 0;

  const handleClear = () => {
    traceSearchRef.current?.clear();
    onClear?.();
  };
  const renderFilterNodeList = () => {
    const spansList: SpanNode[] =
      matchedSpanIds
        ?.map(id => {
          const span = spans?.find(node => node.span_id === id);
          if (!span) {
            return undefined;
          }
          const spanNode: SpanNode = {
            ...span,
            children: undefined,
            isLeaf: true,
            isCollapsed: false,
          };
          return spanNode;
        })
        ?.filter(node => node !== undefined) || [];
    return (
      // 监听双击事件
      <div
        onDoubleClick={() => {
          setIsSlimMode(false);
        }}
        className="h-[calc(100%-48px)]"
      >
        <TraceTree
          dataSource={spansList}
          className={styles['run-tree']}
          selectedSpanId={selectedSpanId}
          onCollapseChange={onCollapseChange}
          matchedSpanIds={matchedSpanIds}
          onSelect={({ node }) => onSelect(node.key)}
        />
      </div>
    );
  };
  const rootNodesLength = useMemo(
    () => getRootNodesLengthWithChildren(rootNodes),
    [rootNodes],
  );
  return (
    <div className="flex flex-col h-full overflow-hidden">
      {enableTraceSearch ? (
        <TraceSearch
          setSearchFilters={setSearchFilters}
          onClear={onClear}
          ref={traceSearchRef}
          isSlimMode={isSlimMode}
          setIsSlimMode={setIsSlimMode}
          searchFilters={searchFilters}
          selectedSpanId={selectedSpanId}
          onToggleAll={onToggleAll}
          isAllExpanded={isAllExpanded}
          filterNonCritical={filterNonCritical}
          onFilterNonCriticalChange={onFilterNonCriticalChange}
          matchedSpanIds={matchedSpanIds}
          onSelect={onSelect}
          rootNodes={rootNodes}
          treeSelectorConfig={treeSelectorConfig}
        />
      ) : null}
      <div className={cls(className, styles['trace-graph'])}>
        <ErrorBoundary
          fallback={<RunTreeEmptyComponent />}
          onError={(error, info) => {
            console.error('TraceTree error:', error, info);
          }}
        >
          <Spin
            spinning={loading}
            wrapperClassName="!h-full"
            childStyle={{ height: '100%' }}
          >
            <div className={cls(styles['run-tree-area'])}>
              {isSearchEmpty ? (
                <SearchEmptyComponent onClear={handleClear} />
              ) : showSlimModel ? (
                <div className=" flex flex-col h-full">
                  {renderFilterNodeList()}
                  <div className="flex py-[12px] items-center justify-center gap-2 coz-fg-secondary">
                    {t('nodes_hidden', {
                      count: spans?.length - matchedSpanIds?.length || 0,
                    })}
                    <Typography.Text
                      link
                      onClick={() => {
                        setIsSlimMode(false);
                        handleClear();
                      }}
                    >
                      {t('click_to_show')}
                    </Typography.Text>
                  </div>
                </div>
              ) : rootNodes && rootNodes.length > 0 ? (
                <>
                  <TraceTree
                    dataSource={rootNodes}
                    className={styles['run-tree']}
                    selectedSpanId={selectedSpanId}
                    onCollapseChange={onCollapseChange}
                    matchedSpanIds={matchedSpanIds}
                    onSelect={({ node }) => onSelect(node.key)}
                  />
                  {filterNonCritical ? (
                    <div className="flex py-[12px]  items-center justify-center gap-2 coz-fg-secondary">
                      {t('nodes_hidden', {
                        count: spans?.length - rootNodesLength,
                      })}
                      <Typography.Text
                        link
                        onClick={() => {
                          onFilterNonCriticalChange?.(false);
                        }}
                      >
                        {t('click_to_show')}
                      </Typography.Text>
                    </div>
                  ) : null}
                </>
              ) : (
                !loading && <RunTreeEmptyComponent />
              )}
            </div>
          </Spin>
        </ErrorBoundary>
      </div>
    </div>
  );
};
