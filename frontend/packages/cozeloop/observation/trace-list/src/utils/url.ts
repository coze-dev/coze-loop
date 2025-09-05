// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import queryString from 'query-string';
import { isNil } from 'lodash-es';
import { CozeLoopStorage } from '@cozeloop/toolkit';

import { decodeJSON } from '@/utils/json';
import { PresetRange, timePickerPresets } from '@/consts/time';
import { type LogicValue } from '@/components/logic-expr';

import { type PersistentFilter } from '../typings/index';
import { type TraceFilter } from '../typings/filter';
import { TRACES_PERSISTENT_FILTER_PROPERTY } from '../consts/filter';

const traceListStorage = new CozeLoopStorage({
  field: 'trace',
});

// const TRACE_FILTER_STORAGE_KEY = 'trace-filter-storage';
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

export const getUrlTraceFilterData = (): TraceFilter => {
  const urlParams = queryString.parse(window.location.search, {
    arrayFormat: 'bracket',
  });

  const hasUrlParams = Object.keys(urlParams).length > 0;

  if (!hasUrlParams) {
    const storedFilter = getTraceFilterFromStorage();
    return { ...storedFilter } as unknown as TraceFilter;
  }

  return urlParams as TraceFilter;
};

export interface InitValue {
  value: string[];
  format: 'number' | 'string';
  defaultValue: number | string;
}
export const initTraceUrlSearchInfo = (
  platformType: InitValue,
  spanListType: InitValue,
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
  } = getUrlTraceFilterData();

  const initUrlPresetTimeRange = trace_preset_time_range as
    | PresetRange
    | undefined;

  const initStartTime =
    trace_start_time &&
    (!initUrlPresetTimeRange || initUrlPresetTimeRange === PresetRange.Unset)
      ? Number(trace_start_time)
      : timePickerPresets[initUrlPresetTimeRange ?? PresetRange.Day3]
          .start()
          .getTime();
  const initEndTime =
    trace_end_time &&
    (!initUrlPresetTimeRange || initUrlPresetTimeRange === PresetRange.Unset)
      ? Number(trace_end_time)
      : timePickerPresets[initUrlPresetTimeRange ?? PresetRange.Day3]
          .end()
          .getTime();
  const initPlatform =
    trace_platform !== undefined &&
    platformType.value.includes(`${trace_platform}`)
      ? platformType.format === 'number'
        ? Number(trace_platform)
        : String(trace_platform)
      : platformType.defaultValue;
  const initSelectedSpanType =
    selected_span_type !== undefined &&
    spanListType.value.includes(`${selected_span_type}`)
      ? spanListType.format === 'number'
        ? Number(selected_span_type)
        : String(selected_span_type)
      : spanListType.defaultValue;

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

export const saveTraceFilterToStorage = (filter: Partial<TraceFilter>) => {
  try {
    const filteredData: Partial<TraceFilter> = {};
    Object.keys(filter).forEach(key => {
      const value = filter[key as keyof TraceFilter];
      if (value !== undefined && value !== null && value !== '') {
        filteredData[key as keyof TraceFilter] = value;
      }
    });

    if (Object.keys(filteredData).length > 0) {
      traceListStorage.setItem(
        'trace-filter-storage',
        JSON.stringify(filteredData),
      );
    }
  } catch (error) {
    console.warn('Failed to save trace filter to localStorage:', error);
  }
};

export const getTraceFilterFromStorage = (): Partial<TraceFilter> => {
  try {
    const stored = traceListStorage.getItem('trace-filter-storage');
    return stored ? JSON.parse(stored) : {};
  } catch (error) {
    console.warn('Failed to get trace filter from localStorage:', error);
    return {};
  }
};
