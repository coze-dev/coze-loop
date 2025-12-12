/* eslint-disable complexity */
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels';
import { useState } from 'react';

import cls from 'classnames';
import { Spin } from '@coze-arch/coze-design';

import { NodeDetailEmpty } from '@/shared/ui/empty-status';
import { type CozeloopTraceDetailLayoutProps } from '@/features/trace-detail/containers/trace-detail/interface';
import { SpanDetail } from '@/features/trace-detail/components/span-detail';
import { VerticalTraceHeader } from '@/features/trace-detail/components/header';
import { TraceGraphs } from '@/features/trace-detail/components/graphs';

export const VerticalTraceDetail = ({
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
  className,
  style,
  matchedSpanIds,
  setSearchFilters,
  searchFilters,
  onClear,
  filterNonCritical,
  searchService,
  onFilterNonCriticalChange,
  enableTraceSearch,
  renderHeaderCopyNode,
  responseApiService,
}: CozeloopTraceDetailLayoutProps) => {
  const [dragging, setDragging] = useState(false);
  const { visible = true } = headerConfig ?? {};
  return (
    <div
      className={cls('flex-1 flex flex-col  overflow-hidden', className)}
      style={style}
    >
      {visible && !headerConfig?.customRender ? (
        <VerticalTraceHeader
          rootSpan={rootNodes?.[0]}
          showClose={headerConfig?.showClose}
          onClose={headerConfig?.onClose}
          renderHeaderCopyNode={renderHeaderCopyNode}
        />
      ) : null}

      {headerConfig?.customRender
        ? headerConfig?.customRender(rootNodes?.[0])
        : null}

      <div className="flex-1 flex flex-col overflow-hidden">
        <PanelGroup direction="vertical">
          <Panel
            className="border-solid border border-[var(--coz-stroke-primary)] rounded"
            minSize={20}
            defaultSize={40}
            maxSize={60}
          >
            <TraceGraphs
              rootNodes={rootNodes}
              enableTraceSearch={enableTraceSearch}
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
            className="h-2 group hover:cursor-row-resize"
            onDragging={isDragging => {
              setDragging(isDragging);
            }}
          >
            <div
              className="h-[2px] box-border my-[3px] mx-2.5 transition group-hover:bg-[#336df4]"
              style={{ background: dragging ? '#336df4' : undefined }}
            />
          </PanelResizeHandle>
          <Panel className="border-solid border border-[var(--coz-stroke-primary)] rounded">
            <Spin
              spinning={
                loading ||
                selectedSpanService?.loading ||
                responseApiService.loading
              }
              wrapperClassName="!w-full !h-full flex items-center justify-center max-h-full overflow-auto"
              childStyle={{ height: '100%', width: '100%' }}
            >
              {selectedSpanService?.data ? (
                <div className="flex">
                  <SpanDetail
                    span={selectedSpanService?.data}
                    className="h-full overflow-auto max-h-full w-full"
                    spanConfig={spanDetailConfig}
                  />
                </div>
              ) : (
                <NodeDetailEmpty />
              )}
            </Spin>
          </Panel>
        </PanelGroup>
      </div>
    </div>
  );
};
