// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-explicit-any */

/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-magic-numbers */
import { useEffect, useRef, useState } from 'react';

import { isEmpty } from 'lodash-es';
import classNames from 'classnames';
import { useSize } from 'ahooks';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import {
  TraceDetailPanel,
  getEndTime,
  getStartTime,
} from '@cozeloop/observation-component-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { LoopTable } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { IconCozIllusNone } from '@coze-arch/coze-design/illustrations';
import { EmptyState, type Table, Typography } from '@coze-arch/coze-design';

import dayjs from '@/utils/dayjs';
import { type ConvertSpan } from '@/typings/span';
import { type SizedColumn } from '@/typings/index';
import { useTraceStore } from '@/stores/trace';
import { useUrlState } from '@/hooks/use-url-state';
import { useSizedColumns } from '@/hooks/use-sized-column';
import { usePerformance } from '@/hooks/use-performance';

import {
  ITEM_SIZE,
  TABLE_HEADER_HEIGHT,
  DEFAULT_SCROLL_AREA_SIZE,
  MIN_DISTANCE_TO_BOTTOM,
} from './config';

import styles from './index.module.less';

type Virtualized = NonNullable<
  React.ComponentProps<typeof Table>['tableProps']
>['virtualized'];

export interface QueryTableProps {
  className?: string;
  moduleName: string;
  onRowClick?: (data: {
    traceID: string;
    startTime: string;
    endTime: string;
  }) => void;
  selectedColumns: SizedColumn<ConvertSpan>[];
  columns: SizedColumn<ConvertSpan>[];
  spans: any[];
  noMore: boolean;
  loading: boolean;
  loadMore: () => void;
  loadingMore: boolean;
  traceListCode: number;
  rowSelection?: any;
}

interface SelectedSpan {
  trace_id: string;
  log_id: string;
  started_at: string;
  duration: string;
  span_id: string;
}

export const TRACE_EXPIRED_CODE = 600903208;

export enum JumpSource {
  PromptCard = 'prompt_card',
  None = '',
}

export const QueryTable = ({
  className,
  onRowClick,
  moduleName,
  selectedColumns,
  spans,
  noMore,
  loading,
  loadMore,
  loadingMore,
  traceListCode,
  rowSelection,
}: QueryTableProps) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedSpan, setSelectedSpan] = useState<SelectedSpan | undefined>();
  const [urlSelectedSpan, setUrlSelectedSpan] =
    useUrlState<Record<string, string | undefined>>();
  const isOpened = useRef(false);
  const [selectedSpanIndex, setSelectedSpanIndex] = useState(0);
  const { spaceID, space: { name: spaceName } = {} } = useSpace();
  const { markStart, markEnd } = usePerformance();
  const [params, setParams] = useUrlState();
  const { trace_jump_from } = params;
  const { selectedSpanType, selectedPlatform, applyFilters } = useTraceStore();

  useEffect(() => {
    if (trace_jump_from !== JumpSource.None) {
      setParams({ trace_jump_from: JumpSource.None });
    }
  }, [applyFilters]);

  useEffect(() => {
    sendEvent(EVENT_NAMES.cozeloop_observation_page_view, {
      page: 'traces',
      platform: selectedPlatform,
      space_id: spaceID,
      space_name: spaceName ?? '',
      span_type: selectedSpanType,
      trace_jump_from: trace_jump_from as JumpSource,
    });
  }, []);

  const onSwitch = (action: 'pre' | 'next') => {
    if (action === 'pre' && selectedSpanIndex > 0) {
      setSelectedSpan(spans?.[selectedSpanIndex - 1]);
      setSelectedSpanIndex(selectedSpanIndex - 1);
    }
    if (action === 'next' && selectedSpanIndex < spans?.length - 1) {
      setSelectedSpan(spans?.[selectedSpanIndex + 1]);
      setSelectedSpanIndex(selectedSpanIndex + 1);
    }
  };

  const openDetailPanel = (record?: ConvertSpan, index?: number) => {
    markStart('cozeloop_trace_detail_panel');
    const { started_at, duration, span_id, trace_id } = record ?? {};
    setSelectedSpan({
      started_at: record?.started_at ?? '',
      duration: record?.duration ?? '',
      log_id: '',
      span_id: record?.span_id ?? '',
      trace_id: record?.trace_id ?? '',
    });

    const span = {
      id: span_id,
      start_time: started_at,
      latency: duration,
      trace_id,
    };
    setUrlSelectedSpan(span);

    setSelectedSpanIndex(index ?? 0);
    setDetailVisible(true);
  };

  const onDetailLoad = () => {
    const duration = markEnd('cozeloop_trace_detail_panel');
    if (typeof duration === 'number') {
      sendEvent(EVENT_NAMES.cozeloop_observation_trace_detail_panel_duration, {
        duration: Math.ceil(duration),
        search_type: 'trace_id',
        space_id: spaceID,
        space_name: spaceName ?? '',
        platform_type: selectedPlatform,
        module_name: moduleName,
      });
    }
  };

  const scrollSize = useSize(containerRef);
  const { width, height } = scrollSize || DEFAULT_SCROLL_AREA_SIZE;

  const sizedColumns = useSizedColumns(
    scrollSize?.width || DEFAULT_SCROLL_AREA_SIZE.width,
    selectedColumns,
  );

  const virtualized: Virtualized = {
    itemSize: ITEM_SIZE,
    onScroll: ({
      scrollDirection,
      scrollOffset = 0,
      scrollUpdateWasRequested,
    }) => {
      let triggerScrollOffset =
        spans.length * ITEM_SIZE -
        (height - TABLE_HEADER_HEIGHT) -
        MIN_DISTANCE_TO_BOTTOM;

      triggerScrollOffset = triggerScrollOffset > 0 ? triggerScrollOffset : 0;
      if (
        scrollDirection === 'forward' &&
        scrollOffset > triggerScrollOffset &&
        !scrollUpdateWasRequested &&
        !loading &&
        !loadingMore
      ) {
        if (!noMore) {
          loadMore();
        }
      }
    },
  };

  useEffect(() => {
    if (
      !urlSelectedSpan ||
      loading ||
      !urlSelectedSpan.trace_id ||
      isOpened.current
    ) {
      return;
    }
    isOpened.current = true;
    setSelectedSpan({
      trace_id: urlSelectedSpan.trace_id ?? '',
      log_id: urlSelectedSpan.log_id ?? '',
      started_at: urlSelectedSpan.start_time ?? '',
      duration: urlSelectedSpan.latency ?? '',
      span_id: urlSelectedSpan.id ?? '',
    });
    const index = spans.findIndex(span => span.id === urlSelectedSpan.id);
    setSelectedSpanIndex(index);
    setDetailVisible(true);
  }, [urlSelectedSpan, loading, spans]);

  useEffect(() => {
    if (!spans) {
      return;
    }

    const duration = markEnd('trace_list_fetch');

    if (typeof duration === 'number') {
      sendEvent(EVENT_NAMES.cozeloop_observation_trace_list_duration, {
        space_id: spaceID,
        space_name: spaceName ?? '',
        duration: Math.ceil(duration),
        span_type: selectedSpanType,
        module: 'Trace',
        platform: selectedPlatform,
      });
    }
  }, [spans]);

  if (isEmpty(spans) && !loading && !loadingMore) {
    return (
      <div className="flex justify-center items-center h-full w-full">
        <EmptyState
          size="full_screen"
          icon={<IconCozIllusNone />}
          title={I18n.t('observation_data_empty')}
          description={
            <div className="text-sm max-w-[540px]">
              {traceListCode === TRACE_EXPIRED_CODE ? (
                <span>{I18n.t('current_trace_expired_to_view')}</span>
              ) : (
                selectedPlatform === 'cozeloop' &&
                I18n.t('trace_empty_tip', {
                  manual: (
                    <Typography.Text
                      link={{
                        href: 'https://loop.coze.cn/open/docs/cozeloop/sdk',
                        target: '_blank',
                      }}
                    >
                      <span className="text-brand-9">
                        &nbsp;{I18n.t('cozeloop_sdk_manual')}&nbsp;
                      </span>
                    </Typography.Text>
                  ),
                })
              )}
            </div>
          }
        />
      </div>
    );
  }
  return (
    <div className={classNames('flex', 'relative', className)}>
      <div className="flex-1 h-full overflow-hidden" ref={containerRef}>
        <LoopTable
          tableProps={{
            id: styles['trace-table'],
            style: { width: '100%' },
            onRow: (record, index) => ({
              onClick() {
                // 用户可能并不想点击
                const isJustSelecting = Boolean(getSelection()?.toString());
                if (isJustSelecting) {
                  return;
                }

                const { trace_id, start_time, latency } = record || {};
                if (!trace_id) {
                  return;
                }
                const offsetTime = selectedSpanType === 'root_span' ? 0 : 30;
                const startTime = dayjs(Number(start_time))
                  .subtract(offsetTime, 'minute')
                  .valueOf()
                  .toString();
                const endTime = dayjs(Number(start_time))
                  .add(Number(latency) + 1000, 'millisecond')
                  .add(offsetTime, 'minute')
                  .valueOf()
                  .toString();

                sendEvent(
                  EVENT_NAMES.cozeloop_observation_trace_jump_detail_from_list,
                  {
                    space_id: spaceID,
                    space_name: spaceName ?? '',
                  },
                );

                if (onRowClick) {
                  onRowClick?.({
                    traceID: trace_id,
                    startTime,
                    endTime,
                  });
                } else {
                  openDetailPanel(record, index);
                }
              },
            }),
            rowKey: 'span_id',
            sticky: true,
            loading: loading || loadingMore,
            virtualized: spans?.length > 0 ? virtualized : false,
            dataSource: spans,
            columns: sizedColumns,
            pagination: false,
            rowSelection,
            scroll: {
              x: width,
              y: height - TABLE_HEADER_HEIGHT - 13, // 13 是底部的 padding
            },
          }}
        />
      </div>
      {selectedSpan ? (
        <TraceDetailPanel
          visible={detailVisible}
          onClose={() => {
            setUrlSelectedSpan({
              trace_id: undefined,
              log_id: undefined,
              start_time: undefined,
              latency: undefined,
              id: undefined,
            });
            setDetailVisible(false);
          }}
          spaceID={spaceID}
          spaceName={spaceName ?? ''}
          platformType={selectedPlatform.toString()}
          id={selectedSpan.trace_id}
          moduleName={moduleName}
          className="!p-0"
          searchType="trace_id"
          startTime={getStartTime(selectedSpan.started_at)}
          endTime={getEndTime(selectedSpan.started_at, selectedSpan.duration)}
          defaultSpanID={selectedSpan.span_id}
          onReady={onDetailLoad}
          switchConfig={{
            canSwitchNext: selectedSpanIndex < spans?.length - 1,
            canSwitchPre: selectedSpanIndex > 0,
            onSwitch,
          }}
        />
      ) : null}
    </div>
  );
};
