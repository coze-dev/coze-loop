// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type PaginationResult } from 'ahooks/lib/usePagination/types';
import {
  type LogicFilter,
  type SemiTableSort,
} from '@cozeloop/evaluate-components';
import { type ItemRunState } from '@cozeloop/api-schema/evaluation';

import { type ExperimentItem } from './experiment-detail';

export interface Filter {
  status?: ItemRunState[];
}

export interface RequestParams {
  current: number;
  pageSize: number;
  sorter?: SemiTableSort;
  filter?: Filter;
  logicFilter?: LogicFilter;
}

export type Service = PaginationResult<
  { total: number; list: ExperimentItem[] },
  [RequestParams]
>;
