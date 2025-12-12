// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
import React, {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react';

import { useRequest } from 'ahooks';
import { type ColumnItem, ColumnSelector } from '@cozeloop/components';
import { type FieldMeta } from '@cozeloop/api-schema/observation';
import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import { Button, Divider, type DatePickerProps } from '@coze-arch/coze-design';

import {
  type FilterOptionsItemConfig,
  type CozeloopTraceListProps,
  type Filters,
  type FetchSpansFn,
} from '@/types/trace-list';
import { useReport } from '@/shared/hooks/use-report';
import { useCozeloopColumns, useInitTraceListConfig } from '@/shared/hooks';
import { BIZ, BIZ_EVENTS, SDK_INTERNAL_EVENTS } from '@/shared/constants';
import { type DatePickerOptions } from '@/shared/components/filter-bar/types';
import { type CustomRightRenderMap } from '@/shared/components/analytics-logic-expr/logic-expr';
import { useTraceService } from '@/services';
import { i18nService } from '@/i18n';
import {
  TraceSelector,
  type TraceSelectorItems,
  type TraceSelectorRef,
  type TraceSelectorState,
} from '@/features/trace-selector';
import { type InitializedFilters } from '@/features/trace-list/utils/filter-initialization';
import { type ConvertSpan } from '@/features/trace-list/types/span';
import { useTraceStore } from '@/features/trace-list/stores/trace';
import { useTraceTimeRangeOptions } from '@/features/trace-list/hooks/use-trace-time-range-options';
import {
  usePageStay,
  useColumns,
  useFetchTraces,
  useSyncCozeloopUrlParams,
  useInitCozeloopFilters,
  usePerformance,
} from '@/features/trace-list/hooks';
import { useConfigContext } from '@/config-provider';

import { type SizedColumn, type Span } from './types';
import { calcPresetTime, PresetRange } from './constants/time';
import { type UseFetchTracesResult } from './components/queries/table/hooks/use-fetch-traces';
import { Queries, TraceProvider } from './components';
import { getBizConfig, getDefaultBizConfig } from './biz-config';

import '../../tailwind.css';

export type GetFieldMetasFn = (params: {
  platform_type: string | number;
  span_list_type: string | number;
}) => Promise<Record<string, FieldMeta>>;

export interface TraceListAppProps {
  columnRecord: Record<string, SizedColumn<ConvertSpan>>;
  onRowClick?: (span: Span, index?: number) => void;
  onTraceDataChange?: (dataSource: UseFetchTracesResult) => void;
  disableEffect?: boolean;
  platformTypeConfig?: FilterOptionsItemConfig;
  spanListTypeConfig?: FilterOptionsItemConfig;
  getTraceList?: FetchSpansFn;
  banner?: React.ReactNode;
  datePickerOptions?: DatePickerOptions[];
  datePickerProps?: DatePickerProps;
  onFiltersChange?: (filters: TraceSelectorState) => void;
  onInitLoad?: (state: InitializedFilters) => void;
  className?: string;
  style?: React.CSSProperties;
  customParams?: Record<string, any>;
  onUrlStateChange?: (params: Record<string, any>) => void;
}

export type TraceListWithDetailPanelAppProps = TraceListAppProps & {
  tracesRefreshKey?: string | number;
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
};

export const TraceListApp = React.forwardRef<
  TraceSelectorRef,
  TraceListWithDetailPanelAppProps
>((props, ref) => {
  const {
    columnRecord,
    onTraceDataChange,
    disableEffect,
    banner,
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
  usePageStay();
  const { markEnd } = usePerformance();
  const { bizId } = useConfigContext();

  const traceSelectorRef = useRef<TraceSelectorRef>(null);
  const report = useReport();

  useImperativeHandle(ref, () => traceSelectorRef.current as TraceSelectorRef);
  const datePickerOptions = useTraceTimeRangeOptions();
  const { theme, workspaceConfig, sendEvent } = useConfigContext();

  const {
    setFieldMetas,
    customParams,
    customViewConfig,
    fieldMetas,
    getFieldMetas,
  } = useTraceStore();

  const { initValues, initializedFilters } = useInitCozeloopFilters({
    defaultFilters,
    spanListTypeConfig,
    platformTypeConfig,
    platformTypeOptionList: platformTypeConfig?.optionList,
    spanListTypeOptionList: spanListTypeConfig?.optionList,
    workspaceConfig,
    customParams,
    disableEffect,
  });
  const [queryState, setQueryState] = useState<Record<string, any>>(initValues);

  useEffect(() => {
    onInitLoad?.(initializedFilters);
  }, []);

  useSyncCozeloopUrlParams({
    disableEffect,
    queryState,
    workspaceConfig,
    customParams,
    onUrlStateChange,
  });

  useEffect(() => {
    report(SDK_INTERNAL_EVENTS.sdk_trace_list_with_detail_panel);
  }, []);

  const { getTraceList, _getTracesAdvanceInfo } =
    useTraceService(workspaceConfig);
  useEffect(() => {
    report(SDK_INTERNAL_EVENTS.sdk_trace_list);
  }, []);

  useRequest(
    async () => {
      const result = await getFieldMetas?.({
        platform_type:
          queryState.platformType ?? platformTypeConfig?.defaultValue,
        span_list_type:
          queryState.spanListType ?? spanListTypeConfig?.defaultValue,
      });
      setFieldMetas(result);
    },
    {
      refreshDeps: [queryState.platformType, queryState.spanListType],
    },
  );

  const { selectedColumns, cols, onColumnsChange, defaultColumns } = useColumns(
    {
      columnsList: Object.keys(columnRecord),
      columnConfig: columnRecord,
      storageOptions: {
        enabled: true,
        key: 'trace-selected-columns-open',
      },
    },
  );

  const result = useFetchTraces({
    fetchSpansFn: (props.getTraceList ??
      getTraceList) as unknown as FetchSpansFn,
    timestamps: [queryState.timeStamp.startTime, queryState.timeStamp.endTime],
    fieldMetas,
    selectedSpanType: queryState.spanListType,
    selectedPlatform: queryState.platformType,
    applyFilters: queryState.filters,
    tracesRefreshKey,
    buildInFetchTracesAdvanceInfo:
      bizId === BIZ.Cozeloop || bizId === BIZ.Fornax
        ? undefined
        : _getTracesAdvanceInfo,
  });

  useEffect(() => {
    onTraceDataChange?.(result);
  }, [result.loading, result.loadingMore]);

  useEffect(() => {
    if (!result.spans) {
      return;
    }

    const duration = markEnd('trace_list_fetch');

    if (typeof duration === 'number') {
      sendEvent?.(BIZ_EVENTS.cozeloop_observation_trace_list_duration, {
        space_id: customParams?.spaceID ?? '',
        space_name: customParams?.spaceName ?? '',
        duration: Math.ceil(duration),
        span_type: queryState.spanListType,
        module: 'Trace',
        platform: queryState.platformType,
      });
    }
  }, [
    result.spans,
    customParams?.spaceID,
    customParams?.spaceName,
    queryState.platformType,
    queryState.spanListType,
  ]);

  return (
    <div
      className={`h-full max-h-full w-full flex-1 max-w-full overflow-hidden min-w-[980px] flex flex-col ${className}`}
      style={{
        ...theme,
        ...(style ?? {}),
      }}
    >
      {banner}
      <div className="pb-3 flex items-center justify-between w-full max-w-full flex-wrap gap-y-2">
        <TraceSelector
          initValues={initValues}
          ref={traceSelectorRef}
          layoutMode={traceSelectorLayout ?? 'horizontal'}
          platformTypeOptionList={platformTypeConfig?.optionList ?? []}
          spanListTypeOptionList={spanListTypeConfig?.optionList ?? []}
          platformTypeConfig={platformTypeConfig}
          spanListTypeConfig={spanListTypeConfig}
          items={
            traceSelectorItems ?? [
              'dateTimePicker',
              'spanListType',
              'platformType',
              'filterSelect',
              'customView',
            ]
          }
          customViewConfig={customViewConfig}
          onChange={value => {
            setQueryState(value);
            onFiltersChange?.(value);
          }}
          datePickerOptions={restProps.datePickerOptions ?? datePickerOptions}
          datePickerProps={datePickerProps}
          fieldMetas={fieldMetas as unknown as Record<string, FieldMeta>}
          getFieldMetas={getFieldMetas as unknown as GetFieldMetasFn}
          customLeftRenderMap={logicExprConfig?.customLeftRenderMap}
          customRightRenderMap={logicExprConfig?.customRightRenderMap}
          customParams={customParams}
        />

        <div className="flex items-center gap-x-2 flex-nowrap flex-1 w-full justify-end">
          <div className="flex gap-1 items-center">
            <Button
              className="!w-[32px] !h-[32px]"
              icon={<IconCozRefresh />}
              onClick={() => {
                const currentPreset =
                  traceSelectorRef.current?.getState()?.preset;
                if (!currentPreset || currentPreset === PresetRange.Unset) {
                  return;
                }
                const time = calcPresetTime(currentPreset as PresetRange);

                traceSelectorRef.current?.setFieldValues(
                  [
                    {
                      key: 'timeStamp',
                      value: time,
                    },
                  ],
                  'refresh',
                );
              }}
              color="primary"
            />
            <ColumnSelector
              columns={cols as unknown as ColumnItem[]}
              onChange={onColumnsChange}
              buttonText={i18nService.t('column_manage')}
              resetButtonText={i18nService.t('column_manage_tip')}
              defaultColumns={defaultColumns as ColumnItem[]}
            />
            {headerSlot ? (
              <>
                <Divider layout="vertical" margin="4px" />
                {headerSlot}
              </>
            ) : null}
          </div>
        </div>
      </div>

      <Queries
        moduleName="analytics_trace_list"
        selectedColumns={selectedColumns}
        columns={cols}
        onRowClick={(record, index) => {
          props.onRowClick?.(record, index);
        }}
        {...result}
        rowSelection={customParams?.rowSelection ?? undefined}
        disableUrlParams
        spanListType={queryState.spanListType}
        platformType={queryState.platformType}
        applyFilters={queryState.filters}
      />
    </div>
  );
});

export const CozeloopTraceList = forwardRef<
  TraceSelectorRef,
  CozeloopTraceListProps
>((props: CozeloopTraceListProps, ref) => {
  const {
    columnsConfig,
    onRowClick,
    onTraceDataChange,
    disableEffect = false,
    getFieldMetas,
    getTraceList,
    filterOptions,
    customParams,
    style,
    className,
  } = props;
  const { bizId } = useConfigContext();
  const bizConfig = getBizConfig(customParams ?? {});
  const config =
    bizConfig[bizId as keyof typeof bizConfig] ?? getDefaultBizConfig();

  const columns = useCozeloopColumns({ columnsConfig });

  const { customViewConfig } = config;
  const {
    initLogicExprConfig,
    initPlatformTypeConfig,
    initSpanListTypeConfig,
  } = useInitTraceListConfig({ customParams, filterOptions });
  const { datePickerOptions } = filterOptions ?? {};

  return (
    <TraceProvider
      getFieldMetas={getFieldMetas}
      customViewConfig={customViewConfig}
      customParams={customParams}
      disableEffect={disableEffect}
    >
      <TraceListApp
        ref={ref}
        columnRecord={columns}
        onRowClick={onRowClick}
        onTraceDataChange={onTraceDataChange}
        disableEffect={disableEffect}
        getTraceList={getTraceList}
        banner={config.banner as unknown as React.ReactNode}
        datePickerOptions={datePickerOptions}
        platformTypeConfig={initPlatformTypeConfig}
        spanListTypeConfig={initSpanListTypeConfig}
        logicExprConfig={initLogicExprConfig}
        className={className}
        style={style}
        customParams={customParams}
      />
    </TraceProvider>
  );
});
