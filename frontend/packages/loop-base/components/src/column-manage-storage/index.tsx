// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isEmpty } from 'lodash-es';
import { safeJsonParse } from '@cozeloop/toolkit';

import { type ColumnItem } from '../columns-select';

export const DATASET_COLUMN_STORAGE_KEY = 'dataset-column';

export const getColumnManageStorage = (storageKey: string) => {
  const storage = localStorage.getItem(storageKey);
  return safeJsonParse(storage || '{}') || {};
};
export interface ColumnSort {
  [key: string]: {
    index: number;
    checked: boolean;
  };
}
export const setColumnsManageStorage = (
  storageKey: string,
  columns: ColumnItem[],
) => {
  if (!storageKey) {
    return;
  }
  const data: ColumnSort = {};
  columns.forEach((column, index) => {
    data[column.key] = {
      index,
      checked: column.checked ?? true,
    };
  });
  localStorage.setItem(storageKey, JSON.stringify(data));
};

export const dealColumnsWithStorage = (
  storageKey: string,
  columns: ColumnItem[],
): ColumnItem[] => {
  const sort = getColumnManageStorage(storageKey);
  if (!sort || isEmpty(sort)) {
    return columns;
  }
  const newColumns = [...(columns || [])].sort((a, b) => {
    const indexA = sort[a.key]?.index ?? Infinity;
    const indexB = sort[b.key]?.index ?? Infinity;
    // 如果两个元素都在 arrayB 中，按照 arrayB 的顺序排序
    if (indexA !== Infinity && indexB !== Infinity) {
      return indexA - indexB;
    }
    // 如果只有一个元素在 arrayB 中，将其排在前面
    if (indexA !== Infinity) {
      return -1;
    }
    if (indexB !== Infinity) {
      return 1;
    }
    // 如果两个元素都不在 arrayB 中，保持原有顺序
    return 0;
  });
  return newColumns.map(column => ({
    ...column,
    checked: sort[column.key]?.checked ?? true,
  }));
};
