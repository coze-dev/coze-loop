// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect } from 'react';

import {
  getUrlParamsFromPersistentFilters,
  saveTraceFilterToStorage,
} from '../utils/url';
import { encodeJSON } from '../utils/json';
import { type TraceFilter } from '../typings/filter';
import { useTraceStore } from '../stores/trace';
import { useUrlState } from './use-url-state';

export const useSyncUrlParams = (disableUrlParams?: boolean) => {
  const [, setUrlParams] = useUrlState<TraceFilter>();
  const setSearchValue = (value: TraceFilter) => {
    setUrlParams(pre => ({
      ...pre,
      ...value,
    }));
  };

  const {
    selectedSpanType,
    timestamps: [startTime, endTime],
    presetTimeRange,
    filters,
    persistentFilters,
    relation,
    selectedPlatform,
  } = useTraceStore();

  useEffect(() => {
    const urlParams = {
      selected_span_type: selectedSpanType.toString(),
      trace_platform: selectedPlatform.toString(),
      trace_filters: filters ? encodeJSON(filters) : undefined,
      trace_start_time: startTime.toString(),
      trace_end_time: endTime.toString(),
      trace_preset_time_range: presetTimeRange,
      relation: relation.toString(),
      ...getUrlParamsFromPersistentFilters(persistentFilters),
    };
    if (!disableUrlParams) {
      setSearchValue(urlParams);
    }
    saveTraceFilterToStorage(urlParams);
  }, [
    disableUrlParams,
    selectedSpanType,
    startTime,
    endTime,
    presetTimeRange,
    filters,
    persistentFilters,
    relation,
    selectedPlatform,
  ]);
};
