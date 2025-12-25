// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useMemo, useRef, useState } from 'react';

import { isEmpty, keys, isEqual } from 'lodash-es';
import { type ColumnItem } from '@cozeloop/components';

import { safeJsonParse } from '@/shared/utils/json';
import { type ConvertSpan } from '@/features/trace-list/types/span';
import { type SizedColumn } from '@/features/trace-list/types';

interface UseColumnsParams {
  columnsList: string[];
  columnConfig: Record<string, SizedColumn<ConvertSpan>>;
  storageOptions?: {
    enabled: boolean;
    key: string;
  };
}

export const useColumns: (params: UseColumnsParams) => {
  selectedColumns: SizedColumn<ConvertSpan>[];
  onColumnsChange: (newColumns: ColumnItem[]) => void;
  cols: SizedColumn<ConvertSpan>[];
  defaultColumns: SizedColumn<ConvertSpan>[];
} = (params: UseColumnsParams) => {
  const { columnsList, columnConfig, storageOptions } = params;
  const { enabled = false, key = '' } = storageOptions || {};

  const [localValue, setLocalValue] = useState(() => {
    if (!enabled) {
      return [];
    }
    return safeJsonParse(localStorage.getItem(key ?? '') ?? '{}');
  });

  const defaultColumns = useMemo(
    () =>
      columnsList
        .map(item => {
          const column = columnConfig[item as keyof typeof columnConfig];
          return {
            ...column,
            key: column.dataIndex,
            value: column.displayName,
          };
        })
        .filter(Boolean) as SizedColumn<ConvertSpan>[],
    [columnsList, columnConfig],
  );

  const [cols, setCols] = useState<SizedColumn<ConvertSpan>[]>(defaultColumns);
  const latestCols = useRef(cols);

  useEffect(() => {
    if (!enabled || isEmpty(keys(localValue))) {
      if (isEqual(latestCols.current, defaultColumns)) {
        return;
      }
      latestCols.current = defaultColumns;
      setCols(defaultColumns);
      return;
    }

    const newCols = defaultColumns
      .map(item => {
        const cacheItem = localValue[item.key ?? ''];
        return {
          ...item,
          checked: cacheItem?.checked ?? item.checked,
        };
      })
      .sort((a, b) => {
        const aIndex = localValue[a.key ?? '']?.index ?? Infinity;
        const bIndex = localValue[b.key ?? '']?.index ?? Infinity;
        return aIndex - bIndex;
      });

    if (isEqual(latestCols.current, newCols)) {
      return;
    }
    latestCols.current = newCols;
    setCols(newCols);
  }, [enabled, localValue, defaultColumns]);

  const selectedColumns = useMemo(
    () => cols.filter(item => item.checked),
    [cols],
  );

  const onColumnsChange = (newColumns: ColumnItem[]) => {
    const newCols = [...(newColumns as SizedColumn<ConvertSpan>[])];
    setCols(newCols);
    if (enabled) {
      const newState = newCols.reduce(
        (acc, item) => {
          acc[item.key ?? ''] = {
            checked: item.checked,
            index: newCols.findIndex(col => col.key === item.key),
          };
          return acc;
        },
        {} satisfies Record<string, { checked: boolean; index: number }>,
      );
      setLocalValue(newState);
      localStorage.setItem(key, JSON.stringify(newState));
    }
  };

  return {
    selectedColumns,
    onColumnsChange,
    cols,
    defaultColumns,
  };
};
