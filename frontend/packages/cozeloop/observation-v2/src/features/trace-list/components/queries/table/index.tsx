/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-magic-numbers */
import { useEffect, useRef, useCallback } from 'react';

import { isEmpty } from 'lodash-es';
import cls from 'classnames';
import { useSize } from 'ahooks';
import { LoopTable } from '@cozeloop/components';
import {
  type PlatformType,
  type SpanListType,
} from '@cozeloop/api-schema/observation';
import { IconCozIllusNone } from '@coze-arch/coze-design/illustrations';
import { EmptyState, type Table, Typography } from '@coze-arch/coze-design';

import { useUrlState } from '@/shared/hooks/use-url-state';
import { BIZ_EVENTS } from '@/shared/constants';
import { type LogicValue } from '@/shared/components/analytics-logic-expr';
import { useLocale } from '@/i18n';
import { type ConvertSpan } from '@/features/trace-list/types/span';
import { type SizedColumn } from '@/features/trace-list/types/index';
import { useTraceStore } from '@/features/trace-list/stores/trace';
import { useSizedColumns } from '@/features/trace-list/hooks/use-sized-column';
import { useConfigContext } from '@/config-provider';

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
  onRowClick?: (span: ConvertSpan, index?: number) => void;
  selectedColumns: SizedColumn<ConvertSpan>[];
  columns: SizedColumn<ConvertSpan>[];
  spans: any[];
  noMore: boolean;
  loading: boolean;
  loadMore: () => void;
  loadingMore: boolean;
  traceListCode: number;
  rowSelection?: any;
  platformType: PlatformType;
  spanListType: SpanListType;
  applyFilters: LogicValue;
}

export const TRACE_EXPIRED_CODE = 600903208;

export enum JumpSource {
  PromptCard = 'prompt_card',
  None = '',
}

export const QueryTable = ({
  className,
  onRowClick,
  selectedColumns,
  spans,
  noMore,
  loading,
  loadMore,
  loadingMore,
  traceListCode,
  rowSelection,
  platformType,
  spanListType,
  applyFilters,
}: QueryTableProps) => {
  const { t } = useLocale();
  const { sendEvent } = useConfigContext();
  const { disableEffect } = useTraceStore();

  const containerRef = useRef<HTMLDivElement>(null);

  const [params, setParams] = useUrlState();
  const { trace_jump_from } = params;
  const { customParams } = useTraceStore();

  useEffect(() => {
    if (disableEffect) {
      return;
    }
    if (trace_jump_from !== JumpSource.None) {
      setParams({ trace_jump_from: JumpSource.None });
    }
  }, [applyFilters, disableEffect]);

  useEffect(() => {
    sendEvent?.(BIZ_EVENTS.cozeloop_observation_page_view, {
      page: 'traces',
      platform: platformType,
      space_id: customParams?.spaceID ?? '',
      space_name: customParams?.spaceName ?? '',
      span_type: spanListType,
      trace_jump_from: trace_jump_from as JumpSource,
    });
  }, []);

  const scrollSize = useSize(containerRef);
  const { width, height } = scrollSize || DEFAULT_SCROLL_AREA_SIZE;

  const sizedColumns = useSizedColumns(
    scrollSize?.width || DEFAULT_SCROLL_AREA_SIZE.width,
    selectedColumns,
  );

  // 检测并自动填充视口的函数
  const checkAndFillViewport = useCallback(() => {
    if (loading || loadingMore || noMore) {
      return;
    }

    const contentHeight = spans.length * ITEM_SIZE;
    const viewportHeight = height - TABLE_HEADER_HEIGHT;

    // 如果内容高度小于等于视口高度，且还有更多数据，则自动加载
    if (contentHeight <= viewportHeight && !noMore) {
      loadMore();
    }
  }, [spans.length, height, loading, loadingMore, noMore, loadMore]);

  // 当数据变化时触发检测
  useEffect(() => {
    // 当数据加载完成且不在加载状态时，检查是否需要自动填充
    if (!loading && !loadingMore) {
      checkAndFillViewport();
    }
  }, [spans, loading, loadingMore, checkAndFillViewport]);

  // 当容器尺寸变化时触发检测
  useEffect(() => {
    // 当容器尺寸发生变化时，重新检查是否需要填充
    if (scrollSize && !loading && !loadingMore) {
      checkAndFillViewport();
    }
  }, [scrollSize, checkAndFillViewport, loading, loadingMore]);

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

  if (isEmpty(spans) && !loading && !loadingMore) {
    return (
      <div className="flex justify-center items-center h-full w-full">
        <EmptyState
          size="full_screen"
          icon={<IconCozIllusNone />}
          title={t('observation_data_empty')}
          description={
            <div className="text-sm max-w-[540px]">
              {traceListCode === TRACE_EXPIRED_CODE ? (
                <span>{t('current_trace_expired_to_view')}</span>
              ) : (
                <>
                  {t('trace_empty_tip', {
                    manual:
                      platformType === 'cozeloop' ? (
                        <Typography.Text
                          link={{
                            href: 'https://loop.coze.cn/open/docs/cozeloop/sdk',
                            target: '_blank',
                          }}
                        >
                          <span className="text-brand-9">
                            {t('cozeloop_sdk_manual')}
                          </span>
                        </Typography.Text>
                      ) : null,
                  })}
                </>
              )}
            </div>
          }
        />
      </div>
    );
  }
  return (
    <div className={cls('flex', 'relative', className)}>
      <div className="flex-1 h-full overflow-hidden" ref={containerRef}>
        <LoopTable
          tableProps={{
            id: styles['trace-table'],
            style: { width: '100%' },
            onRow: (record, index) => ({
              onClick() {
                const isJustSelecting = Boolean(getSelection()?.toString());
                if (isJustSelecting) {
                  return;
                }

                const { trace_id } = record || {};
                if (!trace_id) {
                  return;
                }

                sendEvent?.(
                  BIZ_EVENTS.cozeloop_observation_trace_jump_detail_from_list,
                  {
                    space_id: customParams?.spaceID ?? '',
                    space_name: customParams?.spaceName ?? '',
                  },
                );

                if (onRowClick) {
                  onRowClick?.(record, index);
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
    </div>
  );
};
