// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useMemo, useRef } from 'react';

import {
  type Params,
  type PaginationResult,
} from 'ahooks/lib/usePagination/types';
import { useSize } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { LoopTable } from '@cozeloop/components';
import { Empty, Pagination, type TableProps } from '@coze-arch/coze-design';

// eslint-disable-next-line @typescript-eslint/no-magic-numbers
export const PAGE_SIZE_OPTIONS = [1, 10, 20, 50];

// eslint-disable-next-line complexity
export default function TableWithPagination<RecordItem>(
  props: TableProps & {
    heightFull?: boolean;
    service: PaginationResult<{ total: number; list: RecordItem[] }, Params>;
    pageSizeOpts?: number[];
    header?: React.ReactNode;
  },
) {
  const { pageSizeOpts, service, header, heightFull = false } = props;
  const { columns } = props.tableProps ?? {};
  const tableContainerRef = useRef<HTMLDivElement>(null);
  const size = useSize(tableContainerRef.current);
  const tableHeaderSize = useSize(
    tableContainerRef.current?.querySelector('.semi-table-header'),
  );

  const tablePagination = useMemo(
    () => ({
      currentPage: service.pagination.current,
      pageSize: service.pagination.pageSize,
      total: Number(service.pagination.total),
      onChange: (page: number, pageSize: number) => {
        service.pagination.onChange(page, pageSize);
      },
      showSizeChanger: true,
      pageSizeOpts: pageSizeOpts ?? PAGE_SIZE_OPTIONS,
    }),
    [service.pagination, pageSizeOpts],
  );

  useEffect(() => {
    if (service.pagination.current > 1 && service?.data?.list?.length === 0) {
      service.pagination.changeCurrent(1);
    }
  }, [service.pagination.current, service?.data?.list]);

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
            empty: <Empty title={I18n.t('no_data')} />,
            ...(props.tableProps ?? {}),
            scroll: {
              // 表格容器的高度减去表格头的高度
              y:
                size?.height === undefined || !heightFull
                  ? undefined
                  : size.height - tableHeaderHeight,
              ...(props.tableProps?.scroll ?? {}),
            },
            loading: service?.loading,
            columns: columns?.filter(column => column.hidden !== true),
            dataSource: service?.data?.list ?? [],
          }}
        />
      </div>
      {service.pagination.current > 1 ||
      (service?.data?.list?.length && service?.data?.list?.length > 0) ? (
        <div className="shrink-0 flex flex-row-reverse">
          <Pagination
            {...tablePagination}
            showTotal
            showSizeChanger={true}
          ></Pagination>
        </div>
      ) : null}
    </div>
  );
}
