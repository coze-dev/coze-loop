// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type OrderBy } from '@cozeloop/api-schema/evaluation';

/**
 * 表格排序类型
 */
export interface SemiTableSort {
  /** 排序字段 */
  key: string;
  /** 排序方向 */
  sortOrder: 'ascend' | 'descend' | undefined;
}

/** Semi表格的Sorter结构转服务端的排序数据格式 */
export function sorterToOrderBy(sort: SemiTableSort): OrderBy {
  const { key, sortOrder } = sort ?? {};
  const orderBy: OrderBy = {
    field: key,
    is_asc: typeof sortOrder === 'string' ? sortOrder === 'ascend' : undefined,
  };
  return orderBy;
}
