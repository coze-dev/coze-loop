// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useMemo, useRef } from 'react';

import {
  type Params,
  type PaginationResult,
} from 'ahooks/lib/usePagination/types';
import { useSize } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { getStoragePageSize, LoopTable } from '@cozeloop/components';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import {
  EmptyState,
  CozPagination,
  type PaginationProps,
  type TableProps,
} from '@coze-arch/coze-design';

// eslint-disable-next-line @typescript-eslint/no-magic-numbers
export const PAGE_SIZE_OPTIONS = [10, 20, 50];

// eslint-disable-next-line complexity
export default function TableForExperiment<RecordItem>(
  props: TableProps & {
    heightFull?: boolean;
    service: PaginationResult<{ total: number; list: RecordItem[] }, Params>;
    header?: React.ReactNode;
    paginationLeftSlot?: React.ReactNode;
    paginationProps?: PaginationProps;
    pageSizeStorageKey?: string;
  },
) {
  const {
    paginationProps,
    service,
    header,
    heightFull = false,
    paginationLeftSlot,
    pageSizeStorageKey,
  } = props;
  const { columns } = props.tableProps ?? {};
  const tableContainerRef = useRef<HTMLDivElement>(null);
  const size = useSize(tableContainerRef.current);
  const tableHeaderSize = useSize(
    tableContainerRef.current?.querySelector('.semi-table-header'),
  );

  const tablePagination = useMemo(
    () => ({
      showSizeChanger: true,
      pageSizeOpts: PAGE_SIZE_OPTIONS,
      onPageSizeChange(newPageSize: number) {
        if (pageSizeStorageKey) {
          localStorage.setItem(pageSizeStorageKey, String(newPageSize));
        }
      },
      ...(paginationProps ?? {}),
      currentPage: service.pagination.current,
      pageSize:
        getStoragePageSize(pageSizeStorageKey) || service.pagination.pageSize,
      total: service.pagination.total,
      onChange: service.pagination.onChange,
    }),
    [service.pagination, paginationProps],
  );

  useEffect(() => {
    if (service.pagination.current > 1 && service?.data?.list?.length === 0) {
      service.pagination.changeCurrent(1);
    }
  }, [service.pagination.current, service?.data?.list]);

  const tableHeaderHeight = tableHeaderSize?.height ?? 56;

  return (
    <div
      className={`${heightFull ? 'h-full overflow-hidden' : ''} flex flex-col gap-3`}
    >
      {header ? header : null}
      <div
        ref={tableContainerRef}
        className={heightFull ? 'grow overflow-hidden' : ''}
      >
        <LoopTable
          showTableWhenEmpty={true}
          empty={
            <EmptyState
              size="full_screen"
              icon={<IconCozIllusAdd />}
              title={I18n.t('no_data')}
            />
          }
          {...props}
          tableProps={{
            empty: <div className="h-60" />,
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

      <div className="shrink-0 flex items-center">
        {paginationLeftSlot}
        <CozPagination {...tablePagination} className="ml-auto" />
      </div>
    </div>
  );
}
