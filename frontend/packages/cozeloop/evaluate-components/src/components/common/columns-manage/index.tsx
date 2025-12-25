// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  type ColumnItem,
  ColumnSelector,
  dealColumnsWithStorage,
  setColumnsManageStorage,
} from '@cozeloop/components';
import { Tooltip, type ColumnProps } from '@coze-arch/coze-design';

type CustomColumnItem = ColumnItem & {
  column: ColumnProps;
};

function tableColumnToColumnItem(column: ColumnProps): CustomColumnItem {
  const { title, disableColumnManage } = column;
  const displayName = ((column.displayName ?? title) as string) || '-';
  const disabled = Boolean(disableColumnManage);
  return {
    key: column.dataIndex?.toString() ?? '',
    value: displayName,
    disabled,
    checked: !column.hidden,
    column,
  };
}

export function tableColumnsToColumnItems(
  columns: ColumnProps[],
): CustomColumnItem[] {
  return columns.map(column => tableColumnToColumnItem(column));
}

/**
 * 从 storage 中还原列配置的顺序和显隐
 */
export function dealColumnsFromStorage(
  columns: ColumnProps[],
  storageKey?: string,
) {
  try {
    if (!storageKey) {
      return columns;
    }
    const newColumnItems = dealColumnsWithStorage(
      storageKey,
      tableColumnsToColumnItems(columns),
    );
    return newColumnItems.map(item => {
      const column: ColumnProps = {
        ...(item.column || {}),
        hidden: !item.checked,
      };
      return column;
    });
  } catch (error) {
    console.error(error);
    return columns;
  }
}

export function ColumnsManage({
  columns = [],
  onColumnsChange,
  defaultColumns = [],
  hiddenFieldName = 'hidden',
  storageKey,
  sortable = true,
}: {
  columns: ColumnProps[];
  hiddenFieldName?: string;
  defaultColumns?: ColumnProps[];
  sortable?: boolean;
  storageKey?: string;
  onColumnsChange?: (columns: ColumnProps[]) => void;
}) {
  const hanldeColumnsChange = (columnItems: ColumnItem[]) => {
    const newColumns = columnItems.map(item => ({
      ...((item as CustomColumnItem).column || {}),
      [hiddenFieldName]: !item.checked,
    }));
    onColumnsChange?.(newColumns);
    if (storageKey) {
      setColumnsManageStorage(
        storageKey,
        tableColumnsToColumnItems(newColumns),
      );
    }
  };
  const options = useMemo(() => tableColumnsToColumnItems(columns), [columns]);
  const defaultOptions = useMemo(
    () => tableColumnsToColumnItems(defaultColumns),
    [defaultColumns],
  );

  return (
    <Tooltip theme="dark" content={I18n.t('column_management')}>
      <div>
        <ColumnSelector
          columns={options}
          defaultColumns={defaultOptions}
          sortable={sortable}
          onChange={hanldeColumnsChange}
        />
      </div>
    </Tooltip>
  );
}
