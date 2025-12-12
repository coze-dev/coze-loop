/* eslint-disable max-params */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
import dayjs from 'dayjs';

import { type LogicValue } from '@/shared/components/analytics-logic-expr';
import { type PersistentFilter } from '@/features/trace-list/types/index';
import { type Filters } from '@/features/trace-list/types';
import {
  PresetRange,
  TRACE_PRESETS_LIST,
} from '@/features/trace-list/constants/time';

import { initTraceUrlSearchInfo } from './url';
import { encodeJSON } from './json';

export interface InitializedFilters {
  startTime: number;
  endTime: number;
  selectedSpanType: string | number;
  selectedPlatform: string | number;
  filters: LogicValue | undefined;
  relation: string;
  presetTimeRange: PresetRange;
  persistentFilters: PersistentFilter[];
}

export interface TraceActionsInterface {
  setTimestampsState: (timestamps: [number, number]) => void;
  setSelectedSpanTypeState: (spanType: string | number) => void;
  setSelectedPlatformState: (platform: string | number) => void;
  setFiltersState: (filters: LogicValue | undefined) => void;
  setApplyFiltersState: (filters: LogicValue | undefined) => void;
  setRelation: (relation: string) => void;
}

/**
 * @deprecated Use initializeTraceFilters instead
 * Read filters from URL and storage only (no external filters)
 */
export const readFiltersFromQuery = (
  platformType: string,
  spanListType: string,
  validPlatformTypes?: string[],
  validSpanListTypes?: string[],
  spaceId?: string,
): InitializedFilters => {
  const {
    initSelectedSpanType,
    initStartTime,
    initEndTime,
    initFilters,
    initPersistentFilters,
    initUrlPresetTimeRange,
    initRelation,
    initPlatform,
  } = initTraceUrlSearchInfo(validPlatformTypes, validSpanListTypes, spaceId);

  const presetTimeRange: PresetRange =
    initUrlPresetTimeRange &&
    TRACE_PRESETS_LIST.includes(initUrlPresetTimeRange)
      ? initUrlPresetTimeRange
      : PresetRange.Day3;

  return {
    startTime: initStartTime,
    endTime: initEndTime,
    selectedSpanType: initSelectedSpanType || spanListType,
    selectedPlatform: initPlatform || platformType,
    filters: initFilters,
    relation: initRelation,
    presetTimeRange,
    persistentFilters: initPersistentFilters,
  };
};

interface SyncFiltersToQueryParams {
  selectedSpanType: string | number;
  selectedPlatform: string | number;
  filters: Record<string, any>;
  startTime: string | number;
  endTime: string | number;
  presetTimeRange: string | number;
  relation: string | number;
}

export const formatFiltersToQuery = (params: SyncFiltersToQueryParams) => {
  const {
    selectedSpanType,
    selectedPlatform,
    filters,
    startTime,
    endTime,
    presetTimeRange,
    relation,
  } = params;
  const urlParams = {
    selected_span_type: selectedSpanType.toString(),
    trace_platform: selectedPlatform.toString(),
    trace_filters: filters ? encodeJSON(filters) : undefined,
    trace_start_time: startTime.toString(),
    trace_end_time: endTime.toString(),
    trace_preset_time_range: presetTimeRange,
    relation: relation.toString(),
  };

  return urlParams;
};

/**
 * Initialize trace filters with comprehensive configuration
 * Combines URL/storage query data with external filters
 * Priority: externalFilters > defaultFilters > URL/storage > defaults
 */
export const initializeTraceFilters = (config: {
  defaultSelectedSpanType: string | number;
  defaultSelectedPlatform: string | number;
  validPlatformTypes?: string[];
  validSpanListTypes?: string[];
  spaceId?: string;
  externalFilters?: Filters;
  defaultFilters?: Filters;
  disableUrlQuery?: boolean;
}): InitializedFilters => {
  const {
    defaultSelectedSpanType,
    defaultSelectedPlatform,
    validPlatformTypes,
    validSpanListTypes,
    spaceId,
    externalFilters,
    defaultFilters,
    disableUrlQuery = false,
  } = config;

  // 从URL和存储中读取查询数据
  let queryData: InitializedFilters | undefined;
  if (!disableUrlQuery) {
    const {
      initSelectedSpanType,
      initStartTime,
      initEndTime,
      initFilters,
      initPersistentFilters,
      initUrlPresetTimeRange,
      initRelation,
      initPlatform,
    } = initTraceUrlSearchInfo(validPlatformTypes, validSpanListTypes, spaceId);

    const presetTimeRange: PresetRange =
      initUrlPresetTimeRange &&
      TRACE_PRESETS_LIST.includes(initUrlPresetTimeRange)
        ? initUrlPresetTimeRange
        : PresetRange.Day3;

    queryData = {
      startTime: initStartTime,
      endTime: initEndTime,
      selectedSpanType: initSelectedSpanType || defaultSelectedSpanType,
      selectedPlatform: initPlatform || defaultSelectedPlatform,
      filters: initFilters,
      relation: initRelation,
      presetTimeRange,
      persistentFilters: initPersistentFilters,
    };
  }

  // 合并所有配置，按优先级返回最终结果
  return {
    startTime:
      externalFilters?.timestamps?.[0] ??
      defaultFilters?.timestamps?.[0] ??
      queryData?.startTime ??
      dayjs().subtract(7, 'day').valueOf(),
    endTime:
      externalFilters?.timestamps?.[1] ??
      defaultFilters?.timestamps?.[1] ??
      queryData?.endTime ??
      dayjs().valueOf(),
    selectedSpanType:
      externalFilters?.selectedSpanType ??
      defaultFilters?.selectedSpanType ??
      queryData?.selectedSpanType ??
      defaultSelectedSpanType,

    selectedPlatform:
      externalFilters?.selectedPlatform ??
      defaultFilters?.selectedPlatform ??
      queryData?.selectedPlatform ??
      defaultSelectedPlatform,
    filters:
      externalFilters?.filters ??
      defaultFilters?.filters ??
      queryData?.filters ??
      undefined,
    relation:
      externalFilters?.relation ??
      defaultFilters?.relation ??
      queryData?.relation ??
      '',
    presetTimeRange:
      (externalFilters?.presetTimeRange as PresetRange) ??
      (defaultFilters?.presetTimeRange as PresetRange) ??
      (queryData?.presetTimeRange as PresetRange) ??
      PresetRange.Day3,
    persistentFilters: [],
  };
};

/**
 * @deprecated Use initializeTraceFilters instead
 * Combine query data with external filters
 * Priority: externalFilters > defaultFilters > URL/storage
 */
export const getInitializedFilters = (
  defaultSelectedSpanType: string | number,
  defaultSelectedPlatform: string | number,
  externalFilters?: Filters,
  defaultFilters?: Filters,
  queryData?: InitializedFilters,
): InitializedFilters => ({
  startTime:
    externalFilters?.timestamps?.[0] ??
    defaultFilters?.timestamps?.[0] ??
    queryData?.startTime ??
    dayjs().subtract(7, 'day').valueOf(),
  endTime:
    externalFilters?.timestamps?.[1] ??
    defaultFilters?.timestamps?.[1] ??
    queryData?.endTime ??
    dayjs().valueOf(),
  selectedSpanType:
    externalFilters?.selectedSpanType ??
    defaultFilters?.selectedSpanType ??
    queryData?.selectedSpanType ??
    defaultSelectedSpanType,
  selectedPlatform:
    externalFilters?.selectedPlatform ??
    defaultFilters?.selectedPlatform ??
    queryData?.selectedPlatform ??
    defaultSelectedPlatform,
  filters:
    externalFilters?.filters ??
    defaultFilters?.filters ??
    queryData?.filters ??
    undefined,
  relation:
    externalFilters?.relation ??
    defaultFilters?.relation ??
    queryData?.relation ??
    '',
  presetTimeRange:
    (externalFilters?.presetTimeRange as PresetRange) ??
    (defaultFilters?.presetTimeRange as PresetRange) ??
    (queryData?.presetTimeRange as PresetRange) ??
    PresetRange.Day3,
  persistentFilters: [],
});

/**
 * Apply initialized filters to store actions
 */
export const applyFiltersToStore = (
  filters: InitializedFilters,
  actions: TraceActionsInterface,
) => {
  actions.setTimestampsState([filters.startTime, filters.endTime]);
  actions.setFiltersState(filters.filters);
  actions.setApplyFiltersState(filters.filters);
  actions.setSelectedSpanTypeState(filters.selectedSpanType);
  actions.setRelation(filters.relation);
  actions.setSelectedPlatformState(filters.selectedPlatform);
};

/**
 * Update filters state with new external filters
 */
export const updateFiltersFromExternal = (
  externalFilters: Filters,
  actions: TraceActionsInterface,
) => {
  if (externalFilters.timestamps) {
    actions.setTimestampsState(externalFilters.timestamps);
  }
  if (externalFilters.selectedSpanType !== undefined) {
    actions.setSelectedSpanTypeState(externalFilters.selectedSpanType);
  }
  if (externalFilters.selectedPlatform !== undefined) {
    actions.setSelectedPlatformState(externalFilters.selectedPlatform);
  }
  if (externalFilters.filters !== undefined) {
    actions.setFiltersState(externalFilters.filters as LogicValue);
    actions.setApplyFiltersState(externalFilters.filters as LogicValue);
  }
  if (externalFilters.relation !== undefined) {
    actions.setRelation(externalFilters.relation);
  }
};
