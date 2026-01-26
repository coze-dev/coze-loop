// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type WorkspaceConfig } from '../types';
import { useSyncUrlParams } from './use-sync-url-params';

interface Params {
  disableEffect?: boolean;
  queryState: Record<string, any>;
  workspaceConfig?: WorkspaceConfig;
  customParams?: Record<string, string>;
  onUrlStateChange?: (params: Record<string, any>) => void;
}

export const useSyncCozeloopUrlParams = (params: Params) => {
  const {
    disableEffect,
    queryState,
    workspaceConfig,
    customParams,
    onUrlStateChange,
  } = params;
  useSyncUrlParams({
    disableUrlParams: disableEffect,
    state: {
      selectedSpanType: queryState.spanListType,
      timestamps: [
        queryState.timeStamp.startTime,
        queryState.timeStamp.endTime,
      ],
      presetTimeRange: queryState.preset,
      filters: queryState.filters,
      persistentFilters: queryState.persistentFilters ?? [],
      relation: queryState.filters?.query ?? 'and',
      selectedPlatform: queryState.platformType,
    },
    options: {
      spaceId:
        workspaceConfig?.workspaceId?.toString() || customParams?.spaceID || '',
    },
    onUrlStateChange,
  });
};
