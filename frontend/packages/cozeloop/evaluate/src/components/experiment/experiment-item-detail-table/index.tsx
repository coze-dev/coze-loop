// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
// export interface TableColumn<T> {
//   title?: React.ReactNode;
//   dataIndex: string;
//   width?: number;
//   render?: (value: T[keyof T], record: T, index: number) => React.ReactNode;
// }

import { get } from 'lodash-es';
import classNames from 'classnames';
import { type ColumnProps } from '@coze-arch/coze-design';

import styles from './index.module.less';

/** 在实验单个数据项详情探矿中使用的表格 */
export default function ExperimentItemDetailTable<
  T extends Record<string, unknown>,
>({
  columns,
  dataSource,
  rowKey = 'id',
  className = '',
  weakHeader,
  tdClassName,
  thClassName,
}: {
  columns: ColumnProps<T>[];
  dataSource: T[];
  rowKey?: keyof T;
  className?: string;
  /** 表格头部样式简化，没有背景色等 */
  weakHeader?: boolean;
  tdClassName?: string;
  thClassName?: string;
}) {
  return (
    <table
      className={`w-full border-collapse table-fixed ${weakHeader ? styles['experiment-itemdetaildatasettable'] : ''} ${className}`}
    >
      <thead className="bg-[#5A6CA70A]">
        <tr>
          {columns.map((column, columnIndex) => (
            <th
              key={column.dataIndex ?? column.key ?? columnIndex}
              className={classNames(
                'border-[var(--coz-stroke-primary)] border-solid border-0 border-r last:border-r-0 px-5 py-3 text-left font-medium text-[var(--coz-fg-primary)] box-border',
                thClassName,
              )}
              style={{ width: column.width }}
            >
              {column.title as React.ReactNode}
            </th>
          ))}
        </tr>
      </thead>
      <tbody className="bg-white divide-y divide-[var(--coz-stroke-primary)]">
        {dataSource.map((item, index) => (
          <tr key={item[rowKey] as string}>
            {columns.map((column, columnIndex) => {
              const columnKey = column.dataIndex ?? column.key ?? columnIndex;
              const val = get(item, columnKey);
              return (
                <td
                  key={column.dataIndex ?? column.key ?? columnIndex}
                  className={classNames(
                    'border-[var(--coz-stroke-primary)] border-solid border-0 border-r last:border-r-0 px-5 py-5 whitespace-pre-line text-sm text-[var(--coz-fg-primary)] align-top',
                    tdClassName,
                  )}
                >
                  {column.render ? column.render(val, item, index) : val}
                </td>
              );
            })}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
