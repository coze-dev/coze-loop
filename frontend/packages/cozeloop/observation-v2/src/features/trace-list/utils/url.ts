/* eslint-disable complexity */
import queryString from 'query-string';
import { isNil } from 'lodash-es';

import { type LogicValue } from '@/shared/components/analytics-logic-expr';
import { decodeJSON } from '@/features/trace-list/utils/json';
import { type PersistentFilter } from '@/features/trace-list/types/index';
import { type TraceFilter } from '@/features/trace-list/types/filter';
import {
  PresetRange,
  getTimePickerPresets,
} from '@/features/trace-list/constants/time';
import { TRACES_PERSISTENT_FILTER_PROPERTY } from '@/features/trace-list/constants/filter';

const TRACE_FILTER_STORAGE_KEY = 'coze_loop_trace-filter-storage';
export const getPersistentFiltersFromUrl = (
  value: Record<string, string | string[] | undefined | null>,
) => {
  const persistentKeys = TRACES_PERSISTENT_FILTER_PROPERTY.filter(
    property => !isNil(value?.[property]),
  );
  if (persistentKeys.length === 0) {
    return [];
  } else {
    return persistentKeys.map(key => ({
      type: key,
      value: value?.[key] || '',
    }));
  }
};

export const getUrlParamsFromPersistentFilters = (
  persistentFilters: PersistentFilter[],
) => {
  const params: Record<string, string | undefined> = {};

  TRACES_PERSISTENT_FILTER_PROPERTY.map(property => {
    const filterId = persistentFilters.find(({ type }) => type === property);
    params[property] = filterId ? (filterId.value as string) : undefined;
  });

  return params;
};

export const getUrlTraceFilterData = (spaceId: string): TraceFilter => {
  const urlParams = queryString.parse(window.location.search, {
    arrayFormat: 'bracket',
  });

  const hasUrlParams = Object.keys(urlParams).length > 0;

  if (!hasUrlParams) {
    const storedFilter = getTraceFilterFromStorage(spaceId);
    return { ...storedFilter } as unknown as TraceFilter;
  }

  return urlParams as TraceFilter;
};

export interface InitValue {
  value: string[];
  format: 'number' | 'string';
  defaultValue: string;
}

export const initTraceUrlSearchInfo = (
  validPlatformTypes?: string[],
  validSpanListTypes?: string[],
  spaceId?: string,
) => {
  const {
    selected_span_type,
    trace_platform,
    trace_filters,
    trace_start_time,
    trace_end_time,
    trace_preset_time_range,
    relation,
    ...restParams
  } = getUrlTraceFilterData(spaceId ?? '');

  const timePickerPresets = getTimePickerPresets();
  const initUrlPresetTimeRange =
    trace_preset_time_range &&
    Object.keys(timePickerPresets).includes(trace_preset_time_range)
      ? (trace_preset_time_range as PresetRange)
      : undefined;

  const initStartTime =
    trace_start_time &&
    (!initUrlPresetTimeRange || initUrlPresetTimeRange === PresetRange.Unset)
      ? Number(trace_start_time)
      : (timePickerPresets[initUrlPresetTimeRange ?? PresetRange.Day3]
          ?.start()
          ?.getTime() ?? undefined);
  const initEndTime =
    trace_end_time &&
    (!initUrlPresetTimeRange || initUrlPresetTimeRange === PresetRange.Unset)
      ? Number(trace_end_time)
      : (timePickerPresets[initUrlPresetTimeRange ?? PresetRange.Day3]
          ?.end()
          ?.getTime() ?? undefined);
  // 验证 platform type 是否在有效选项中
  const isValidPlatform =
    trace_platform !== undefined &&
    (!validPlatformTypes ||
      validPlatformTypes.includes(trace_platform as string));
  const initPlatform = (isValidPlatform ? trace_platform : undefined) as string;

  // 验证 span type 是否在有效选项中
  const isValidSpanType =
    selected_span_type !== undefined &&
    (!validSpanListTypes ||
      validSpanListTypes.includes(selected_span_type as string));
  const initSelectedSpanType = isValidSpanType ? selected_span_type : undefined;

  const initFilters = trace_filters
    ? decodeJSON<LogicValue>(trace_filters)
    : undefined;

  const initPersistentFilters = getPersistentFiltersFromUrl(restParams);
  const initRelation = relation ? (relation as string) : 'and';

  return {
    initStartTime,
    initEndTime,
    initPlatform,
    initUrlPresetTimeRange,
    initSelectedSpanType,
    initFilters,
    initPersistentFilters,
    initRelation,
  };
};

export const saveTraceFilterToStorage = (
  filter: Partial<TraceFilter>,
  spaceId: string,
) => {
  try {
    const filteredData: Partial<TraceFilter> = {};
    Object.keys(filter).forEach(key => {
      const value = filter[key as keyof TraceFilter];
      if (value !== undefined && value !== null && value !== '') {
        filteredData[key as keyof TraceFilter] = value;
      }
    });

    if (Object.keys(filteredData).length > 0) {
      sessionStorage.setItem(
        `${TRACE_FILTER_STORAGE_KEY}_${spaceId}`,
        JSON.stringify(filteredData),
      );
    }
  } catch (error) {
    console.warn('Failed to save trace filter to sessionStorage:', error);
  }
};

export const getTraceFilterFromStorage = (
  spaceId: string,
): Partial<TraceFilter> => {
  try {
    const stored = sessionStorage.getItem(
      `${TRACE_FILTER_STORAGE_KEY}_${spaceId}`,
    );
    return stored ? JSON.parse(stored) : {};
  } catch (error) {
    console.warn('Failed to get trace filter from sessionStorage:', error);
    return {};
  }
};
