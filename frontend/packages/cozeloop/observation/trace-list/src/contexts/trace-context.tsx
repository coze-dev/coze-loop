// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
// This component contains complex state management logic for the trace feature
import React, {
  createContext,
  useContext,
  useCallback,
  useState,
  useEffect,
} from 'react';

import { type FieldMeta } from '@cozeloop/api-schema/observation';

import { initTraceUrlSearchInfo, type InitValue } from '../utils/url';
import { type PersistentFilter } from '../typings/index';
import {
  calcPresetTime,
  PresetRange,
  TRACE_PRESETS_LIST,
} from '../consts/time';
import { type LogicValue } from '../components/logic-expr';

// Define the state interface (previously TraceStoreState)
export interface TraceContextState {
  timestamps: [number, number];
  fieldMetas?: Record<string, FieldMeta | undefined>;
  presetTimeRange: PresetRange;
  refreshFlag: number;
  selectedSpanType: string | number;
  // 在多种span类型中独立的属性
  filters?: LogicValue;
  persistentFilters: PersistentFilter[];
  // 备份不同span类型下数据
  // trace 列表搜索 loading
  relation: string;
  // 数据来源
  selectedPlatform: string | number;
  // 当前激活的视图名字
  applyFilters?: LogicValue;
  filterPopupVisible: boolean;
  lastUserRecord: {
    filters?: LogicValue;
    selectedPlatform?: string | number;
    selectedSpanType?: string | number;
  };
}

// Define the actions interface (previously TraceStoreAction)
export interface TraceContextActions {
  setTimestamps: (e: [number, number]) => void;
  setFilters: (val?: LogicValue) => void;
  setFieldMetas: (e?: Record<string, FieldMeta>) => void;
  setPresetTimeRange: (e: PresetRange) => void;
  updateRefreshFlag: () => void;
  setSelectedSpanType: (e: string | number) => void;
  setSelectedPlatform: (e: string | number) => void;
  setApplyFilters: (e: LogicValue) => void;
  setFilterPopupVisible: (e: boolean) => void;
  setLastUserRecord: (e: TraceContextState['lastUserRecord']) => void;
}

// Combined type for the context value
export type TraceContextType = TraceContextState & TraceContextActions;

// Helper function to handle auto update time (previously handleAutoUpdateTime)
const handleAutoUpdateTime = (state: TraceContextState): [number, number] => {
  const time = calcPresetTime(state.presetTimeRange);
  if (time && state.presetTimeRange !== PresetRange.Unset) {
    return [time.startTime, time.endTime];
  }
  return state.timestamps;
};

// Create the context with a default undefined value
const TraceContext = createContext<TraceContextType | undefined>(undefined);

// Props for the provider component
export interface TraceProviderProps {
  children: React.ReactNode;
  platformType: InitValue;
  spanListType: InitValue;
}

// Helper function to create state update actions
const useTraceActions = (
  presetTimeRange: PresetRange,
  initPlatform: string | number,
  initSelectedSpanType: string | number,
) => {
  const [timestamps, setTimestampsState] = useState<[number, number]>([0, 0]);
  const [filters, setFiltersState] = useState<LogicValue | undefined>(
    undefined,
  );
  const [relation, setRelation] = useState<string>('and');
  const [fieldMetas, setFieldMetasState] = useState<
    Record<string, FieldMeta | undefined> | undefined
  >(undefined);
  const [refreshFlag, setRefreshFlag] = useState<number>(0);
  const [selectedSpanType, setSelectedSpanTypeState] = useState<
    string | number
  >(initSelectedSpanType);
  const [selectedPlatform, setSelectedPlatformState] = useState<
    string | number
  >(initPlatform);
  const [applyFilters, setApplyFiltersState] = useState<LogicValue | undefined>(
    undefined,
  );
  const [filterPopupVisible, setFilterPopupVisibleState] =
    useState<boolean>(false);
  const [lastUserRecord, setLastUserRecordState] = useState<
    TraceContextState['lastUserRecord']
  >({
    filters: { filter_fields: [] },
    selectedPlatform: initPlatform,
    selectedSpanType: initSelectedSpanType,
  });

  // Simple actions
  const setTimestamps = useCallback((e: [number, number]) => {
    setTimestampsState(e);
  }, []);

  const setFieldMetas = useCallback((e?: Record<string, FieldMeta>) => {
    setFieldMetasState(e);
  }, []);

  const setPresetTimeRange = useCallback((e: PresetRange) => e, []);

  const updateRefreshFlag = useCallback(() => {
    setRefreshFlag(prev => prev + 1);
  }, []);

  const setFilterPopupVisible = useCallback((e: boolean) => {
    setFilterPopupVisibleState(e);
  }, []);

  const setLastUserRecord = useCallback(
    (e: TraceContextState['lastUserRecord']) => {
      setLastUserRecordState(e);
    },
    [],
  );

  // Complex actions that update timestamps
  const updateTimestampsWithState = useCallback(
    (prevTimestamps: [number, number]) => {
      const newState = {
        timestamps: prevTimestamps,
        presetTimeRange,
      };
      return handleAutoUpdateTime(newState as TraceContextState);
    },
    [presetTimeRange],
  );

  const setSelectedPlatform = useCallback(
    (e: string | number) => {
      setSelectedPlatformState(e);
      setTimestampsState(prev => updateTimestampsWithState(prev));
    },
    [updateTimestampsWithState],
  );

  const setSelectedSpanType = useCallback(
    (e: string | number) => {
      setSelectedSpanTypeState(e);
      setTimestampsState(prev => updateTimestampsWithState(prev));
    },
    [updateTimestampsWithState],
  );

  const setFilters = useCallback(
    (e?: any) => {
      setFiltersState(e);
      setRelation(e?.relation ?? 'and');
      setTimestampsState(prev => updateTimestampsWithState(prev));
    },
    [updateTimestampsWithState],
  );

  const setApplyFilters = useCallback(
    (e: any) => {
      setApplyFiltersState(e);
      setTimestampsState(prev => updateTimestampsWithState(prev));
    },
    [updateTimestampsWithState],
  );

  return {
    // State
    timestamps,
    filters,
    relation,
    fieldMetas,
    refreshFlag,
    selectedSpanType,
    selectedPlatform,
    applyFilters,
    filterPopupVisible,
    lastUserRecord,
    // Actions
    setTimestamps,
    setFilters,
    setFieldMetas,
    setPresetTimeRange,
    updateRefreshFlag,
    setSelectedSpanType,
    setSelectedPlatform,
    setApplyFilters,
    setFilterPopupVisible,
    setLastUserRecord,
    // State setters
    setTimestampsState,
    setFiltersState,
    setRelation,
    setFieldMetasState,
    setSelectedSpanTypeState,
    setSelectedPlatformState,
    setApplyFiltersState,
    setFilterPopupVisibleState,
    setLastUserRecordState,
  };
};

// Provider component
export const TraceProvider: React.FC<TraceProviderProps> = ({
  children,
  platformType,
  spanListType,
}) => {
  const {
    initSelectedSpanType,
    initStartTime,
    initEndTime,
    initFilters,
    initPersistentFilters,
    initUrlPresetTimeRange,
    initRelation,
    initPlatform,
  } = initTraceUrlSearchInfo(platformType, spanListType);

  // Initialize preset time range
  const [presetTimeRange, setPresetTimeRangeState] = useState<PresetRange>(
    initUrlPresetTimeRange &&
      TRACE_PRESETS_LIST.includes(initUrlPresetTimeRange)
      ? initUrlPresetTimeRange
      : PresetRange.Day3,
  );

  // Initialize persistent filters
  const [persistentFilters] = useState<PersistentFilter[]>(
    initPersistentFilters,
  );

  // Get all actions and state from the hook
  const actions = useTraceActions(
    presetTimeRange,
    initPlatform,
    initSelectedSpanType,
  );

  // Initialize state with URL values
  useEffect(() => {
    actions.setTimestampsState([initStartTime, initEndTime]);
    actions.setFiltersState(initFilters as LogicValue);
    actions.setApplyFiltersState(initFilters as LogicValue);
    actions.setSelectedSpanTypeState(initSelectedSpanType);
    actions.setRelation(initRelation);
    actions.setSelectedPlatformState(initPlatform);
  }, []);

  // Create the context value object
  const contextValue = {
    timestamps: actions.timestamps,
    fieldMetas: actions.fieldMetas,
    presetTimeRange,
    refreshFlag: actions.refreshFlag,
    selectedSpanType: actions.selectedSpanType,
    filters: actions.filters,
    persistentFilters,
    relation: actions.relation,
    selectedPlatform: actions.selectedPlatform,
    applyFilters: actions.applyFilters,
    filterPopupVisible: actions.filterPopupVisible,
    lastUserRecord: actions.lastUserRecord,
    setTimestamps: actions.setTimestamps,
    setFilters: actions.setFilters,
    setFieldMetas: actions.setFieldMetas,
    setPresetTimeRange: (e: PresetRange) => {
      setPresetTimeRangeState(e);
    },
    updateRefreshFlag: actions.updateRefreshFlag,
    setSelectedSpanType: actions.setSelectedSpanType,
    setSelectedPlatform: actions.setSelectedPlatform,
    setApplyFilters: actions.setApplyFilters,
    setFilterPopupVisible: actions.setFilterPopupVisible,
    setLastUserRecord: actions.setLastUserRecord,
  };

  return (
    <TraceContext.Provider value={contextValue}>
      {children}
    </TraceContext.Provider>
  );
};

// Custom hook to use the trace context
export const useTraceContext = (): TraceContextType => {
  const context = useContext(TraceContext);
  if (context === undefined) {
    throw new Error('useTraceContext must be used within a TraceProvider');
  }
  return context;
};

// Selector hook for optimized re-renders (similar to useShallow with zustand)
export function useTraceSelector<T>(
  selector: (state: TraceContextType) => T,
): T {
  const context = useTraceContext();
  return selector(context);
}
