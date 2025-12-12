// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useRef } from 'react';

import cls from 'classnames';
import { useSize } from 'ahooks';
import { type TableProps } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { createLoopTableSortIcon } from './sort-icon';
import { LoopTable } from './index';

export type TableWithoutPaginationProps = TableProps & {
  heightFull?: boolean;
  header?: React.ReactNode;
};

export function TableWithoutPagination(props: TableWithoutPaginationProps) {
  const { header, heightFull = false, className } = props;
  const I18n = useI18n();
  const { columns } = props.tableProps ?? {};
  const tableContainerRef = useRef<HTMLDivElement>(null);
  const size = useSize(tableContainerRef.current);
  const tableHeaderSize = useSize(
    tableContainerRef.current?.querySelector('.semi-table-header'),
  );

  const tableHeaderHeight = tableHeaderSize?.height ?? 56;

  return (
    <div
      className={cls('flex flex-col gap-3 w-full', className, {
        'h-full flex overflow-hidden': heightFull,
      })}
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
                  ? { sortIcon: createLoopTableSortIcon(I18n) }
                  : {}),
              })),
          }}
        />
      </div>
    </div>
  );
}
