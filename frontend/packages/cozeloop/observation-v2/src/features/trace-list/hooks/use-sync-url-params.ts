/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect } from 'react';

import { useUrlState } from '@/shared/hooks/use-url-state';
import {
  getUrlParamsFromPersistentFilters,
  saveTraceFilterToStorage,
} from '@/features/trace-list/utils/url';
import { encodeJSON } from '@/features/trace-list/utils/json';
import { type TraceFilter } from '@/features/trace-list/types/filter';

interface UseSyncUrlParamsProps {
  disableUrlParams?: boolean;
  state: {
    selectedSpanType: string;
    timestamps: [number, number];
    presetTimeRange: string;
    filters: any;
    persistentFilters: any;
    relation: string;
    selectedPlatform: string;
  };
  options?: {
    spaceId?: string;
  };
  onUrlStateChange?: (params: Record<string, any>) => void;
}

export const useSyncUrlParams = ({
  disableUrlParams,
  state,
  options = {},
  onUrlStateChange,
}: UseSyncUrlParamsProps) => {
  const [, setUrlParams] = useUrlState<TraceFilter>();
  const { spaceId = '' } = options;
  const setSearchValue = (value: TraceFilter) => {
    setUrlParams(pre => ({
      ...pre,
      ...value,
    }));
  };

  useEffect(() => {
    if (!state) {
      return;
    }
    const {
      selectedSpanType,
      timestamps,
      presetTimeRange,
      filters,
      persistentFilters,
      relation,
      selectedPlatform,
    } = state ?? {};
    const [startTime, endTime] = timestamps ?? [];
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
      saveTraceFilterToStorage(urlParams, spaceId);
    }
    onUrlStateChange?.(urlParams);
  }, [state]);
};
