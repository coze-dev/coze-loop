// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useMemo } from 'react';

import { uniqBy } from 'lodash-es';
import { FieldType } from '@cozeloop/api-schema/observation';
import { type OptionProps } from '@coze-arch/coze-design/.';

import { type FilterOptionsItemConfig } from '@/types/trace-list';

import { initializeTraceFilters } from '../utils/filter-initialization';
import { type Filters, type WorkspaceConfig } from '../types';

interface Params {
  defaultFilters?: Filters;
  spanListTypeConfig?: FilterOptionsItemConfig;
  platformTypeConfig?: FilterOptionsItemConfig;
  platformTypeOptionList?: OptionProps[];
  spanListTypeOptionList?: OptionProps[];
  workspaceConfig?: WorkspaceConfig;
  customParams?: Record<string, string>;
  disableEffect?: boolean;
}

export const useInitCozeloopFilters = ({
  defaultFilters,
  spanListTypeConfig,
  platformTypeConfig,
  platformTypeOptionList,
  spanListTypeOptionList,
  workspaceConfig,
  customParams,
  disableEffect = false,
}: Params) => {
  const initializedFilters = useMemo(
    () =>
      initializeTraceFilters({
        defaultSelectedSpanType:
          defaultFilters?.selectedSpanType ??
          spanListTypeConfig?.defaultValue ??
          '',
        defaultSelectedPlatform:
          defaultFilters?.selectedPlatform ??
          platformTypeConfig?.defaultValue ??
          '',
        validPlatformTypes:
          platformTypeOptionList?.map(item => String(item.value)) ?? [],
        validSpanListTypes:
          spanListTypeOptionList?.map(item => String(item.value)) ?? [],
        spaceId:
          workspaceConfig?.workspaceId?.toString() ||
          customParams?.spaceID ||
          '',
        disableUrlQuery: disableEffect,
      }),
    [
      spanListTypeConfig?.defaultValue,
      platformTypeConfig?.defaultValue,
      platformTypeOptionList,
      spanListTypeOptionList,
      workspaceConfig?.workspaceId,
      customParams?.spaceID,
      disableEffect,
      defaultFilters,
    ],
  );
  const initValues = useMemo(
    () => ({
      platformType: initializedFilters.selectedPlatform as string,
      spanListType: initializedFilters.selectedSpanType as string,
      timeStamp: {
        startTime: initializedFilters.startTime,
        endTime: initializedFilters.endTime,
      },
      preset: initializedFilters.presetTimeRange,
      filters: {
        query_and_or:
          defaultFilters?.filters?.query_and_or ??
          initializedFilters?.filters?.query_and_or,
        filter_fields:
          uniqBy(
            [
              ...(defaultFilters?.filters?.filter_fields ?? []),
              ...(initializedFilters?.filters?.filter_fields ?? []),
            ],
            'field_name',
          ).map(item => ({
            ...item,
            field_type: item.field_type ?? FieldType.String,
          })) ?? [],
      },
    }),
    [initializedFilters, defaultFilters],
  );

  return {
    initValues,
    initializedFilters,
  };
};
