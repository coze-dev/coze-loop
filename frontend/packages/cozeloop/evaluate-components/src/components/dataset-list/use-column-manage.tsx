// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { type ColumnItem } from '@cozeloop/components';
import {
  setColumnsManageStorage,
  dealColumnsWithStorage,
} from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';

import { getDatasetListColumnSortStorageKey } from '../../utils/column-manage/dataset-list-column-storage';
import { getColumnConfigs } from './column-config';

export const useColumnManage = () => {
  const { spaceID } = useSpace();
  const defaultColumns = getColumnConfigs();
  const storageKey = getDatasetListColumnSortStorageKey(spaceID);
  const convertColumns = dealColumnsWithStorage(storageKey, [
    ...defaultColumns,
  ]);
  const [columns, setColumns] = useState<ColumnItem[]>(convertColumns);
  const onColumnsChange = (newColumns: ColumnItem[]) => {
    setColumnsManageStorage(storageKey, newColumns);
    setColumns(newColumns);
  };
  return {
    columns,
    setColumns: onColumnsChange,
    defaultColumns,
  };
};
