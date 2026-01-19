// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type ColumnPropsPro, type ColKey } from './type';

export function generateColumnsWithKey<T extends Record<string, any> = any>(
  cols?: ColumnPropsPro<T>[],
  hiddenColKeys?: ColKey[],
) {
  function mapCol(
    col: ColumnPropsPro<T>,
    index: number,
    parentKey?: ColKey,
  ): ColumnPropsPro<T> {
    const { key, children } = col;
    let colKey: ColKey;
    if (key) {
      colKey = key;
    } else {
      const indexKey = `index${index}`;
      colKey = parentKey ? `${parentKey}-${indexKey}` : indexKey;
    }

    return {
      ...col,
      colKey,
      children: children?.map((child, idx) => mapCol(child, idx, colKey)),
    };
  }

  return cols?.map((col, index) => mapCol(col, index));
}

export function generateConfigColumns<T extends Record<string, any> = any>(
  cols?: ColumnPropsPro<T>[],
  hiddenColKeys?: ColKey[],
) {
  if (!hiddenColKeys?.length) {
    return cols;
  }

  function mapCols(list?: ColumnPropsPro<T>[]) {
    const res: ColumnPropsPro<T>[] = [];
    list?.forEach(col => {
      if (hiddenColKeys?.includes(col.colKey as ColKey)) {
        return;
      }
      res.push({
        ...col,
        children: col.children && mapCols(col.children as ColumnPropsPro[]),
      });
    });

    return res;
  }

  return mapCols(cols);
}
