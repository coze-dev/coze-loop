// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { type CozeloopColumnsConfig } from '@/types/trace-list';
import { type ConvertSpan } from '@/features/trace-list/types/span';
import { type SizedColumn } from '@/features/trace-list/types';
import { useTableCols } from '@/features/trace-list/components/column-item';

interface Params {
  columnsConfig: CozeloopColumnsConfig;
}

export const useCozeloopColumns = ({ columnsConfig }: Params) => {
  const buildInColumns = useTableCols();

  const columns = useMemo(
    () =>
      columnsConfig.columns.reduce(
        (pre, cur) => {
          if (typeof cur === 'string') {
            const buildInColumn = buildInColumns[cur];

            if (!buildInColumn) {
              throw new Error(
                `${cur} is not a build in column keys, please custom your own column`,
              );
            }
            return {
              ...(pre as Record<string, SizedColumn<ConvertSpan>>),
              [cur]: buildInColumn,
            };
          }
          const { dataIndex } = cur;
          return {
            ...(pre as Record<string, SizedColumn<ConvertSpan>>),
            [dataIndex ?? '']: cur,
          };
        },
        {} satisfies Record<string, SizedColumn<ConvertSpan>>,
      ) as Record<string, SizedColumn<ConvertSpan>>,
    [columnsConfig.columns, buildInColumns],
  );

  return columns;
};
