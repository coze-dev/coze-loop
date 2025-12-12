/* eslint-disable @coze-arch/zustand/prefer-selector */

/* eslint-disable complexity */
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable react-hooks/exhaustive-deps */
/* eslint-disable @coze-arch/max-line-per-function */
import React, { useEffect, useImperativeHandle, useRef, useState } from 'react';

import {
  type FilterFields,
  type PlatformType,
  type FieldMeta,
} from '@cozeloop/api-schema/observation';

import { BIZ_EVENTS } from '@/shared/constants';
import { type CustomRightRenderMap } from '@/shared/components/analytics-logic-expr/logic-expr';
import {
  type TraceSelectorItems,
  type TraceSelectorRef,
} from '@/features/trace-selector';
import { type ConvertSpan } from '@/features/trace-list/types/span';
import { type Filters } from '@/features/trace-list/types';
import { useTraceStore } from '@/features/trace-list/stores/trace';
import { usePerformance } from '@/features/trace-list/hooks';
import { TraceProvider } from '@/features/trace-list/components';
import {
  type GetFieldMetasFn,
  type TraceListAppProps,
  TraceListApp,
} from '@/features/trace-list';
import { type DataSource } from '@/features/trace-detail/types/params';
import {
  CozeloopTraceDetailPanel,
  type CozeloopTraceDetailPanelProps,
  getEndTime,
  getStartTime,
  type TraceDetailContext,
} from '@/features/trace-detail';

import { type CozeloopTraceListProps } from './types/trace-list';
import {
  useInitTraceListConfig,
  useCozeloopColumns,
  useUrlState,
} from './shared/hooks';
import { useTraceService } from './services';
import { type UseFetchTracesResult } from './features/trace-list/components/queries/table/hooks/use-fetch-traces';
import { useConfigContext } from './config-provider';

import './tailwind.css';

// 获取 trace 详情数据的函数类型
export type GetTraceDetailDataFn = (params: {
  trace_id?: string;
  logid?: string;
  platform_type: PlatformType | string;
  start_time: string;
  end_time: string;
  filters?: FilterFields;
  limit?: number;
}) => Promise<DataSource> | DataSource;

// 获取 trace 下 Span 详情数据的函数类型
type GetTraceSpanDetailDataFn = (params: {
  trace_id: string;
  platform_type: PlatformType;
  start_time: string;
  end_time: string;
  span_ids?: string[];
}) => Promise<DataSource> | DataSource;

export type CozeloopTraceListWithDetailPanelProps = CozeloopTraceListProps &
  Pick<
    CozeloopTraceDetailPanelProps,
    | 'spanDetailConfig'
    | 'headerConfig'
    | 'spanDetailHeaderSlot'
    | 'enableTraceSearch'
    | 'keySpanType'
  > & {
    getTraceDetailData?: GetTraceDetailDataFn;
    /** 获取 trace 下 Span 详情数据 */
    getTraceSpanDetailData?: GetTraceSpanDetailDataFn;
    tracesRefreshKey?: string | number;
    traceSelectorItems?: TraceSelectorItems;
    traceSelectorLayout?: 'vertical' | 'horizontal';
    className?: string;
    style?: React.CSSProperties;
    /** 当 disableEffect 为 true 时，用于处理 URL 状态更新的回调函数 */
    onUrlStateChange?: (urlState: Record<string, string | undefined>) => void;
  } & Pick<TraceDetailContext, 'jumpButtonConfig'>;

export type TraceListWithDetailPanelAppProps = TraceListAppProps &
  Pick<
    CozeloopTraceDetailPanelProps,
    'spanDetailConfig' | 'headerConfig' | 'spanDetailHeaderSlot'
  > & {
    tracesRefreshKey?: string | number;
    getTraceDetailData?: GetTraceDetailDataFn;
    /** 获取 trace 下 Span 详情数据 */
    getTraceSpanDetailData?: GetTraceSpanDetailDataFn;
    headerSlot?: React.ReactNode;
    logicExprConfig?: {
      customRightRenderMap?: CustomRightRenderMap;
      customLeftRenderMap?: CustomRightRenderMap;
    };
    traceSelectorItems?: TraceSelectorItems;
    traceSelectorLayout?: 'vertical' | 'horizontal';
    className?: string;
    style?: React.CSSProperties;
    defaultFilters?: Filters;
    /** 当 disableEffect 为 true 时，用于处理 URL 状态更新的回调函数 */
    onUrlStateChange?: (urlState: Record<string, string | undefined>) => void;
  } & Pick<TraceDetailContext, 'jumpButtonConfig'>;

interface SelectedSpan {
  trace_id: string;
  started_at: string;
  duration: string;
  span_id: string;
}

const TraceListWithDetailPanelApp = React.forwardRef<
  TraceSelectorRef,
  TraceListWithDetailPanelAppProps
>((props, ref) => {
  const {
    columnRecord,
    onTraceDataChange,
    disableEffect,
    banner,
    getTraceDetailData,
    getTraceSpanDetailData,
    onFiltersChange,
    tracesRefreshKey,
    headerSlot,
    platformTypeConfig,
    spanListTypeConfig,
    datePickerProps,
    onInitLoad,
    style,
    className,
    logicExprConfig,
    traceSelectorItems,
    traceSelectorLayout,
    defaultFilters,
    onUrlStateChange,
    ...restProps
  } = props;
  const { markStart, markEnd } = usePerformance();
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedSpan, setSelectedSpan] = useState<SelectedSpan | undefined>();
  const [urlSelectedSpan, setUrlSelectedSpan] =
    useUrlState<Record<string, string | undefined>>();
  const isOpened = useRef(false);
  const [selectedSpanIndex, setSelectedSpanIndex] = useState(0);
  const traceSelectorRef = useRef<TraceSelectorRef>(null);

  useImperativeHandle(ref, () => traceSelectorRef.current as TraceSelectorRef);
  const { theme, workspaceConfig, sendEvent } = useConfigContext();

  const { customParams, fieldMetas, getFieldMetas } = useTraceStore();

  const { getTraceDetail } = useTraceService(workspaceConfig);

  const [listResult, setListResult] = useState<
    UseFetchTracesResult | undefined
  >(undefined);
  useEffect(() => {
    if (listResult) {
      onTraceDataChange?.(listResult);
    }
  }, [listResult?.loading, listResult?.loadingMore]);

  const onDetailLoad = () => {
    const duration = markEnd('cozeloop_trace_detail_panel');
    if (typeof duration === 'number') {
      sendEvent?.(BIZ_EVENTS.cozeloop_observation_trace_detail_panel_duration, {
        duration: Math.ceil(duration),
        search_type: 'trace_id',
        space_id: customParams?.spaceID ?? '',
        space_name: customParams?.spaceName ?? '',
        platform_type: traceSelectorRef.current?.getState()?.platformType,
        module_name: 'trace',
      });
    }
  };
  const openDetailPanel = (record?: ConvertSpan, index?: number) => {
    markStart('cozeloop_trace_detail_panel');
    const { started_at, duration, span_id, trace_id } = record ?? {};
    setSelectedSpan({
      started_at: record?.started_at ?? '',
      duration: record?.duration ?? '',
      span_id: record?.span_id ?? '',
      trace_id: record?.trace_id ?? '',
    });

    const detailPanelUrl = {
      id: span_id,
      start_time: started_at,
      latency: duration,
      trace_id,
    };

    if (!disableEffect) {
      setUrlSelectedSpan(detailPanelUrl);
    }
    onUrlStateChange?.(detailPanelUrl);

    setSelectedSpanIndex(index ?? 0);
    setDetailVisible(true);
  };

  useEffect(() => {
    if (
      !urlSelectedSpan ||
      listResult?.loading ||
      !urlSelectedSpan.trace_id ||
      isOpened.current
    ) {
      return;
    }
    isOpened.current = true;
    setSelectedSpan({
      trace_id: urlSelectedSpan.trace_id ?? '',
      started_at: urlSelectedSpan.start_time ?? '',
      duration: urlSelectedSpan.latency ?? '',
      span_id: urlSelectedSpan.id ?? '',
    });
    const index =
      listResult?.spans?.findIndex(
        span => span.span_id === urlSelectedSpan.id,
      ) ?? -1;
    setSelectedSpanIndex(index);
    setDetailVisible(true);
  }, [urlSelectedSpan, listResult?.loading, listResult?.spans]);

  useEffect(() => {
    if (!listResult?.spans) {
      return;
    }

    const duration = markEnd('trace_list_fetch');

    if (typeof duration === 'number') {
      sendEvent?.(BIZ_EVENTS.cozeloop_observation_trace_list_duration, {
        space_id: customParams?.spaceID ?? '',
        space_name: customParams?.spaceName ?? '',
        duration: Math.ceil(duration),
        span_type: traceSelectorRef.current?.getState()?.spanListType,
        module: 'Trace',
        platform: traceSelectorRef.current?.getState()?.platformType,
      });
    }
  }, [listResult?.spans, customParams?.spaceID, customParams?.spaceName]);

  const onSwitch = (action: 'pre' | 'next') => {
    if (action === 'pre' && selectedSpanIndex > 0) {
      setSelectedSpan(listResult?.spans?.[selectedSpanIndex - 1]);
      setSelectedSpanIndex(selectedSpanIndex - 1);
    }
    if (
      action === 'next' &&
      selectedSpanIndex < (listResult?.spans?.length ?? 0) - 1
    ) {
      setSelectedSpan(listResult?.spans?.[selectedSpanIndex + 1]);
      setSelectedSpanIndex(selectedSpanIndex + 1);
    }
  };

  return (
    <div
      className={`h-full max-h-full w-full flex-1 max-w-full overflow-hidden min-w-[980px] flex flex-col ${className}`}
      style={{
        ...theme,
        ...(style ?? {}),
      }}
    >
      {banner}
      <TraceListApp
        ref={traceSelectorRef}
        columnRecord={columnRecord}
        onRowClick={(record, index) => {
          setSelectedSpan(record);
          openDetailPanel(record, index);

          props.onRowClick?.(record, index);
        }}
        onTraceDataChange={data => setListResult(data)}
        disableEffect={disableEffect}
        banner={undefined}
        datePickerProps={datePickerProps}
        headerSlot={headerSlot}
        platformTypeConfig={platformTypeConfig}
        spanListTypeConfig={spanListTypeConfig}
        logicExprConfig={logicExprConfig}
        traceSelectorItems={traceSelectorItems}
        traceSelectorLayout={traceSelectorLayout}
        defaultFilters={defaultFilters}
        onFiltersChange={onFiltersChange}
        onInitLoad={onInitLoad}
        onUrlStateChange={onUrlStateChange}
        {...restProps}
      />
      {selectedSpan ? (
        <CozeloopTraceDetailPanel
          {...restProps}
          customParams={customParams}
          spanId={selectedSpan.span_id}
          traceId={selectedSpan.trace_id}
          getTraceDetailData={async (params?: { filters?: FilterFields }) => {
            const runtimeGetter =
              typeof getTraceDetailData === 'function'
                ? getTraceDetailData
                : getTraceDetail;
            try {
              const data = await runtimeGetter({
                trace_id: selectedSpan.trace_id,
                start_time: getStartTime(selectedSpan.started_at),
                end_time: getEndTime(
                  selectedSpan.started_at,
                  selectedSpan.duration,
                ),
                platform_type: (traceSelectorRef.current?.getState()
                  ?.platformType ?? '') as PlatformType,
                ...(params?.filters ? { filters: params?.filters } : {}),
              });

              return data as DataSource;
            } catch (error) {
              return {
                spans: [],
              };
            } finally {
              onDetailLoad();
            }
          }}
          getTraceSpanDetailData={async params => {
            const runtimeGetter =
              typeof getTraceSpanDetailData === 'function'
                ? getTraceSpanDetailData
                : getTraceDetail;
            try {
              const data = await runtimeGetter({
                trace_id: selectedSpan.trace_id,
                start_time: getStartTime(selectedSpan.started_at),
                end_time: getEndTime(
                  selectedSpan.started_at,
                  selectedSpan.duration,
                ),
                span_ids: params?.span_ids,
                platform_type: (traceSelectorRef.current?.getState()
                  ?.platformType ??
                  platformTypeConfig?.defaultValue) as PlatformType,
              });

              return data as DataSource;
            } catch (error) {
              return {
                spans: [],
              };
            }
          }}
          visible={detailVisible}
          onClose={() => {
            const params = {
              trace_id: undefined,
              log_id: undefined,
              start_time: undefined,
              latency: undefined,
              id: undefined,
            };
            if (!disableEffect) {
              setUrlSelectedSpan(params);
            }
            onUrlStateChange?.(params);
            setDetailVisible(false);
          }}
          platformType={traceSelectorRef.current?.getState()?.platformType}
          className="!p-0"
          defaultSelectedSpanID={selectedSpan.span_id}
          switchConfig={{
            canSwitchNext:
              selectedSpanIndex < (listResult?.spans?.length ?? 0) - 1,
            canSwitchPre: selectedSpanIndex > 0,
            onSwitch,
          }}
          extraSpanDetailTabs={customParams?.extraSpanDetailTabs ?? []}
          spanDetailConfig={restProps.spanDetailConfig}
          treeSelectorConfig={{
            initValues: {
              platformType: traceSelectorRef.current?.getState()?.platformType,
              spanListType: traceSelectorRef.current?.getState()?.spanListType,
            },
            platformTypeOptionList: [],
            spanListTypeOptionList: [],
            items: ['filterSelect'],
            datePickerOptions: [],
            datePickerProps: {},
            fieldMetas: fieldMetas as unknown as Record<string, FieldMeta>,
            getFieldMetas: getFieldMetas as unknown as GetFieldMetasFn,
            customLeftRenderMap: logicExprConfig?.customLeftRenderMap,
            customRightRenderMap: logicExprConfig?.customRightRenderMap,
            customParams,
          }}
          jumpButtonConfig={props.jumpButtonConfig}
        />
      ) : null}
    </div>
  );
});

export const CozeloopTraceListWithDetailPanel = React.forwardRef<
  TraceSelectorRef,
  CozeloopTraceListWithDetailPanelProps
>((props, ref) => {
  const {
    columnsConfig,
    disableEffect = false,
    customParams,
    filterOptions,
    onUrlStateChange,
    ...restProps
  } = props;

  const {
    initLogicExprConfig,
    initPlatformTypeConfig,
    initSpanListTypeConfig,
    initBizConfig,
  } = useInitTraceListConfig({ customParams, filterOptions });
  const columns = useCozeloopColumns({ columnsConfig });

  const { datePickerOptions, datePickerProps } = filterOptions ?? {};

  const { customViewConfig } = initBizConfig;

  return (
    <TraceProvider
      getFieldMetas={props.getFieldMetas}
      customViewConfig={customViewConfig}
      customParams={customParams}
      disableEffect={disableEffect}
    >
      <TraceListWithDetailPanelApp
        {...restProps}
        ref={ref}
        columnRecord={columns}
        disableEffect={disableEffect}
        banner={initBizConfig.banner as unknown as React.ReactNode}
        datePickerOptions={datePickerOptions}
        headerSlot={customParams?.headerSlot}
        platformTypeConfig={initPlatformTypeConfig}
        spanListTypeConfig={initSpanListTypeConfig}
        datePickerProps={datePickerProps}
        logicExprConfig={initLogicExprConfig}
        onUrlStateChange={onUrlStateChange}
      />
    </TraceProvider>
  );
});
