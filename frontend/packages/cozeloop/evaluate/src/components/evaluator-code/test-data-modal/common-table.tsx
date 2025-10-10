{
  /* start_aigc */
}
import { useMemo, useEffect } from 'react';

import { usePagination } from 'ahooks';
import { useBatchSelect } from '@cozeloop/evaluate-components/src/components/dataset-detail/table/use-batch-select';
import { getFieldColumnConfig } from '@cozeloop/evaluate-components';
import { TableWithPagination } from '@cozeloop/components';
import { Typography } from '@coze-arch/coze-design';

import { MAX_SELECT_COUNT } from '@/constants/code-evaluator';

import type { CommonTableProps } from '../types';

const CommonTable: React.FC<CommonTableProps> = ({
  data,
  onSelectionChange,
  loading = false,
  fieldSchemas = [],
  supportMultiSelect = false,
  pageSize = 10,
  defaultPageSize = 10,
  showSizeChanger = true,
  pageSizeOptions = [10, 20, 50],
  prevCount,
}) => {
  // 使用分页hook
  const paginationService = usePagination(
    (paginationData: { current: number; pageSize?: number }) => {
      const { current, pageSize: currentPageSize } = paginationData;
      const pageSizeToUse = currentPageSize || pageSize;

      // 计算分页数据
      const startIndex = (current - 1) * pageSizeToUse;
      const endIndex = startIndex + pageSizeToUse;
      const paginatedData = data.slice(startIndex, endIndex);

      return Promise.resolve({
        total: data.length,
        list: paginatedData,
      });
    },
    {
      defaultPageSize,
      refreshDeps: [data],
    },
  );

  const maxCount = useMemo(
    () => (prevCount ? MAX_SELECT_COUNT - prevCount : MAX_SELECT_COUNT),
    [prevCount],
  );

  // 使用批量选择hook
  const { selectColumn, batchSelectItems } = useBatchSelect({
    itemList: paginationService.data?.list || [],
    datasetDetail: undefined,
    maxNumber: maxCount,
  });

  const columns = useMemo(() => {
    const result =
      fieldSchemas?.map(field =>
        getFieldColumnConfig({
          field,
          prefix: 'trunFieldData.fieldDataMap.',
          expand: false,
          editNode: null,
        }),
      ) || [];
    return result;
  }, [fieldSchemas]);

  // 同步多选状态到批量选择hook
  useEffect(() => {
    if (supportMultiSelect) {
      onSelectionChange?.(new Set(batchSelectItems));
    }
  }, [batchSelectItems, onSelectionChange, supportMultiSelect]);

  // 创建一个自定义的多选头部，与原多选头部样式保持一致
  const CustomMultiSelectHeader = useMemo(
    () => (
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Typography.Text size="small">{`选择数据(${batchSelectItems.size}/${maxCount})`}</Typography.Text>
        </div>
      </div>
    ),
    [batchSelectItems.size],
  );

  // 使用分页service对象
  const service = useMemo(
    () => ({
      data: paginationService.data,
      loading: paginationService.loading || loading,
      mutate: paginationService.mutate,
      pagination: paginationService.pagination,
    }),
    [paginationService, loading],
  );

  return (
    <TableWithPagination
      style={{ minHeight: '400px', height: '400px' }}
      service={service}
      tableProps={{
        columns: [...(supportMultiSelect ? [selectColumn] : []), ...columns],
        loading: service.loading,
      }}
      showTableWhenEmpty={false}
      heightFull={true}
      showSizeChanger={showSizeChanger}
      pageSizeOpts={pageSizeOptions}
      // 仅在支持多选时显示头部
      header={supportMultiSelect ? CustomMultiSelectHeader : null}
    />
  );
};

export default CommonTable;
{
  /* end_aigc */
}
