// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useMemo, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  LogicEditor,
  type LogicFilter,
  ColumnsManage,
  RefreshButton,
} from '@cozeloop/evaluate-components';
import { IS_HIDDEN_EXPERIMENT_DETAIL_FILTER } from '@cozeloop/biz-hooks-adapter';
import {
  type ColumnEvaluator,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import { type ColumnProps } from '@coze-arch/coze-design';

import { type ColumnInfo } from '@/types/experiment/experiment-contrast';
import { TableHeader } from '@/components/table-for-experiment';
import { AnnotationInfo, EvaluatorInfo } from '@/components/info-tag';
import TableCellExpand from '@/components/common/table-cell-expand';

import { getExperimentContrastLogicFields } from '../../utils/logic-filter-tools';

// eslint-disable-next-line @coze-arch/max-line-per-function
export default function ExperimentContrastTableHeader({
  experiments = [],
  columnInfosMap,
  columnManageStorageKey,
  columns,
  logicFilter,
  setLogicFilter,
  onFilterClose,
  expand,
  setExpand,
  hiddenFieldMap,
  setHiddenFieldMap,
  setHiddenColumnMap,
  onRefresh,
}: {
  experiments: Experiment[] | undefined;
  columnInfosMap?: Record<string, ColumnInfo[]>;
  columnManageStorageKey: string;
  columns: ColumnProps[];
  logicFilter: LogicFilter | undefined;
  setLogicFilter: (logicFilter?: LogicFilter) => void;
  onFilterClose?: () => void;
  expand?: boolean;
  setExpand?: (expand: boolean) => void;
  hiddenFieldMap: Record<string, boolean>;
  setHiddenFieldMap: (map: Record<string, boolean>) => void;
  setHiddenColumnMap: (map: Record<string, boolean>) => void;
  onRefresh?: () => void;
}) {
  const [defaultColumns, setDefaultColumns] = useState<ColumnProps[]>([]);
  const [manageableColumns, setManageableColumns] = useState<ColumnProps[]>([]);

  const logicFields = useMemo(
    () => getExperimentContrastLogicFields(experiments),
    [experiments],
  );

  const handleColumnManageChange: React.Dispatch<
    React.SetStateAction<ColumnProps[]>
  > = newManageColumnsData => {
    setManageableColumns(oldManageableColumns => {
      // 先获得新的管理列数组
      let newManageColumns: ColumnProps[] = [];
      if (typeof newManageColumnsData === 'function') {
        newManageColumns = [...newManageColumnsData(oldManageableColumns)];
      } else {
        newManageColumns = newManageColumnsData;
      }
      // 处理列变化
      const newHiddenColumnMap: Record<string, boolean> = {};
      newManageColumns.forEach(column => {
        if (column.canManage) {
          newHiddenColumnMap[column.key ?? ''] = column.hidden;
        }
      });
      setHiddenColumnMap(newHiddenColumnMap);

      // 处理评估器指标变化
      const newHiddenEvalutorMap: Record<Int64, boolean> = {};
      newManageColumns.forEach(column => {
        if (column.isFieldColumn) {
          newHiddenEvalutorMap[column.key ?? ''] = column.hidden;
        }
      });

      setHiddenFieldMap(newHiddenEvalutorMap);
      // 存储列管理数据到本地
      localStorage.setItem(
        columnManageStorageKey,
        JSON.stringify({
          hiddenFieldMap: newHiddenEvalutorMap,
          hiddenColumnMap: newHiddenColumnMap,
        }),
      );
      return newManageColumns;
    });
  };

  useEffect(() => {
    const newColumns = columns.filter(column => column.canManage);

    // 去重
    const columnInfos: ColumnInfo[] = Object.values(
      columnInfosMap || {},
    ).reduce((prev, cur) => {
      prev.push(...cur);
      return prev;
    }, []);

    const columnInfoMap: Record<string, ColumnInfo> = {};
    columnInfos.forEach(item => {
      if (columnInfoMap[item.key]) {
        return;
      }
      columnInfoMap[item.key] = item;
    });

    const fieldColumns = Object.values(columnInfoMap)
      .sort(a => (a.type === 'evaluator' ? -1 : 1))
      .map(item => ({
        title:
          item.type === 'evaluator' ? (
            <EvaluatorInfo evaluator={item.data as ColumnEvaluator} />
          ) : (
            <AnnotationInfo annotation={item.data} />
          ),
        dataIndex: item.key,
        key: item.key,
        isFieldColumn: true,
        hidden: hiddenFieldMap[item.key] ?? false,
      }));

    newColumns.push(...fieldColumns);

    newColumns.push(
      ...[
        // {
        //   title: 'Latency',
        //   dataIndex: 'latency',
        //   key: 'latency',
        //   isFieldColumn: true,
        //   hidden: hiddenFieldMap.latency ?? false,
        // },
        // {
        //   title: 'Token',
        //   dataIndex: 'token',
        //   key: 'token',
        //   isFieldColumn: true,
        //   hidden: hiddenFieldMap.token ?? false,
        // },
        {
          title: I18n.t('status'),
          dataIndex: 'status',
          key: 'status',
          isFieldColumn: true,
          hidden: hiddenFieldMap.status ?? false,
        },
      ],
    );
    setManageableColumns(newColumns);
    setDefaultColumns(
      newColumns.map(column => ({
        ...column,
        hidden: false,
      })),
    );
  }, [columns, columnInfosMap, hiddenFieldMap]);

  const filters = (
    <>
      <LogicEditor
        fields={logicFields}
        value={logicFilter}
        onConfirm={newVal => setLogicFilter(newVal)}
        onClose={onFilterClose}
      />
    </>
  );

  const actions = (
    <>
      <TableCellExpand
        className="ml-auto"
        expand={expand}
        onChange={setExpand}
      />
      <ColumnsManage
        columns={manageableColumns}
        defaultColumns={defaultColumns}
        onColumnsChange={handleColumnManageChange}
        sortable={false}
      />
      <RefreshButton onRefresh={onRefresh} />
    </>
  );
  return (
    <TableHeader
      filters={IS_HIDDEN_EXPERIMENT_DETAIL_FILTER ? null : filters}
      actions={actions}
    />
  );
}
