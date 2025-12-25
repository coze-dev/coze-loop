// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels';
import React, { useState } from 'react';

import cls from 'classnames';
import { Spin } from '@coze-arch/coze-design';

import { useCustomComponents } from '@/features/trace-detail/hooks/use-custom-components';
import { type CozeloopTraceDetailLayoutProps } from '@/features/trace-detail/containers/trace-detail/interface';
import { SpanDetail } from '@/features/trace-detail/components/span-detail';
import { HorizontalTraceHeader } from '@/features/trace-detail/components/header';
import { TraceGraphs } from '@/features/trace-detail/components/graphs';

export const HorizontalTraceDetail = ({
  loading,
  onCollapseChange,
  onSelect,
  onToggleAll,
  isAllExpanded,
  rootNodes,
  selectedSpanService,
  selectedSpanId,
  spans,
  headerConfig,
  spanDetailConfig,
  switchConfig,
  className,
  style,
  advanceInfo,
  matchedSpanIds,
  setSearchFilters,
  searchFilters,
  onClear,
  filterNonCritical,
  onFilterNonCriticalChange,
  searchService,
  treeSelectorConfig,
  enableTraceSearch,
  renderHeaderCopyNode,
  responseApiService,
}: CozeloopTraceDetailLayoutProps) => {
  const [dragging, setDragging] = useState(false);
  const { visible = true } = headerConfig ?? {};
  const { NodeDetailEmpty } = useCustomComponents();

  return (
    <div
      className={cls('flex-1 flex flex-col overflow-hidden', className)}
      style={style}
    >
      {visible && !headerConfig?.customRender ? (
        <HorizontalTraceHeader
          renderHeaderCopyNode={renderHeaderCopyNode}
          rootSpan={rootNodes?.[0]}
          showClose={headerConfig?.showClose}
          onClose={headerConfig?.onClose}
          switchConfig={switchConfig}
          extraRender={headerConfig?.extraRender}
          advanceInfo={advanceInfo}
          showFullscreenButton={headerConfig?.showFullscreenButton}
          onFullscreen={headerConfig?.onFullscreen}
        />
      ) : null}

      {headerConfig?.customRender
        ? headerConfig?.customRender(rootNodes?.[0])
        : null}

      <div className="flex-1 flex flex-col overflow-hidden">
        <PanelGroup
          direction="horizontal"
          className="border-solid border-0 border-t border-[var(--coz-stroke-primary)]"
        >
          <Panel
            minSize={20}
            defaultSize={26}
            maxSize={35}
            className="!min-w-[300px]"
          >
            <TraceGraphs
              enableTraceSearch={enableTraceSearch}
              treeSelectorConfig={treeSelectorConfig}
              rootNodes={rootNodes}
              loading={loading || searchService.loading}
              spans={spans}
              selectedSpanId={selectedSpanId}
              matchedSpanIds={matchedSpanIds}
              onSelect={onSelect}
              onCollapseChange={onCollapseChange}
              onToggleAll={onToggleAll}
              isAllExpanded={isAllExpanded}
              searchFilters={searchFilters}
              setSearchFilters={setSearchFilters}
              onClear={onClear}
              filterNonCritical={filterNonCritical}
              onFilterNonCriticalChange={onFilterNonCriticalChange}
            />
          </Panel>
          <PanelResizeHandle
            className="w-[2px] group hover:cursor-col-resize"
            onDragging={isDragging => {
              setDragging(isDragging);
            }}
          >
            <div
              className="w-[1px] h-full box-border transition group-hover:bg-[rgb(var(--coze-up-brand-7))]"
              style={{
                background: dragging
                  ? 'rgb(var(--coze-up-brand-7))'
                  : 'var(--coz-stroke-primary)',
                width: dragging ? '2px' : '1px',
              }}
            />
          </PanelResizeHandle>
          <Panel>
            <Spin
              spinning={
                loading ||
                selectedSpanService?.loading ||
                responseApiService?.loading
              }
              wrapperClassName="!w-full !h-full flex items-center justify-center max-h-full overflow-auto"
              childStyle={{ height: '100%', width: '100%' }}
            >
              {selectedSpanService?.data ? (
                <SpanDetail
                  span={selectedSpanService?.data}
                  className="h-full overflow-auto max-h-full w-full"
                  spanConfig={spanDetailConfig}
                />
              ) : (
                <>{!loading ? <NodeDetailEmpty /> : null}</>
              )}
            </Spin>
          </Panel>
        </PanelGroup>
      </div>
    </div>
  );
};
