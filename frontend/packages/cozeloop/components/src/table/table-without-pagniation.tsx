// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import { useSize } from 'ahooks';
import { type TableProps } from '@coze-arch/coze-design';

import LoopTableSortIcon from './sort-icon';
import { LoopTable } from './index';

export const PAGE_SIZE_OPTIONS = [10, 20, 50];
export const DEFAULT_PAGE_SIZE = 10;
// eslint-disable-next-line complexity
export function TableWithoutPagination(
  props: TableProps & {
    heightFull?: boolean;
    pageSizeOpts?: number[];
    header?: React.ReactNode;
  },
) {
  const { header, heightFull = false } = props;
  const { columns } = props.tableProps ?? {};
  const tableContainerRef = useRef<HTMLDivElement>(null);
  const size = useSize(tableContainerRef.current);
  const tableHeaderSize = useSize(
    tableContainerRef.current?.querySelector('.semi-table-header'),
  );

  const tableHeaderHeight = tableHeaderSize?.height ?? 56;

  return (
    <div
      className={`${heightFull ? 'h-full flex overflow-hidden' : ''} flex flex-col gap-3`}
    >
      {header ? header : null}
      <div
        ref={tableContainerRef}
        className={heightFull ? 'flex-1 overflow-hidden' : ''}
      >
        <LoopTable
          {...props}
          tableProps={{
            empty: <></>,
            ...(props.tableProps ?? {}),
            scroll: {
              // 表格容器的高度减去表格头的高度
              y:
                size?.height === undefined || !heightFull
                  ? undefined
                  : size.height - tableHeaderHeight - 2,
              ...(props.tableProps?.scroll ?? {}),
            },
            columns: columns
              ?.filter(
                column => column.hidden !== true && column.checked !== false,
              )
              ?.map(column => ({
                ...column,
                ...(column.sorter && !column.sortIcon
                  ? { sortIcon: LoopTableSortIcon }
                  : {}),
              })),
          }}
        />
      </div>
    </div>
  );
}
