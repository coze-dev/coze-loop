// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-object-injection */
import { useMemo, useState } from 'react';

import { type RowSelection } from '@coze-arch/coze-design';

/** 计算数组元素的映射表 */
function arrayToMap<T>(array: T[], key: keyof T): Record<string, T> {
  const map: Record<string, T> = {};
  array.forEach(item => {
    const mapKey = item[key as keyof T] as string;
    if (mapKey !== undefined) {
      const val = item;
      map[mapKey] = val;
    }
  });
  return map;
}

/** 批量操作接口 */
export interface BatchOperateStore<RecordItem> {
  /** 表格行选择器 */
  rowSelection: RowSelection<RecordItem>;
  /** 已选择的表格行记录数据 */
  selectedItems: RecordItem[];
  /** 设置已选择的表格行记录数据 */
  setSelectedItems: (items: RecordItem[]) => void;
  /** 批量操作状态 */
  enableBatchOperate: boolean;
  /** 设置批量操作状态 */
  setEnableBatchOperate: (enable: boolean) => void;
  /** 取消批量操作 */
  onCancelBatchOperate: () => void;
}

/** 批量操作配置 */
export interface BatchOperateOptions<RecordItem> {
  /** 表格行记录数据的唯一ID字段，默认 `id` */
  recordKey?: keyof RecordItem;
  /** 最大可选数量，默认不限制 */
  maxSelectionCount?: number;
}

/**
 * 表格批量操作状态管理。
 * 支持跨分页选择数据。
 * @param recordKey 表格行记录数据的唯一ID字段，默认 `id`
 * @returns 批量操作接口
 */
export function useBatchOperate<RecordItem>({
  recordKey = 'id' as keyof RecordItem,
  maxSelectionCount,
}: BatchOperateOptions<RecordItem> = {}): BatchOperateStore<RecordItem> {
  const [enableBatchOperate, setEnableBatchOperate] = useState(false);
  const [selectedItems, setSelectedItems] = useState<RecordItem[]>([]);

  const rowSelection = useMemo(() => {
    // 是否开启最大可选数量限制
    const hasMaxLimit =
      typeof maxSelectionCount === 'number' && !Number.isNaN(maxSelectionCount);
    // 选择数量是否达到最大数量
    const isSelectionMaximum =
      hasMaxLimit && selectedItems.length >= maxSelectionCount;
    // 表格行选择器配置
    const newRowSelection: RowSelection<RecordItem> = {
      // 禁用表格列标题处的checkbox
      disabled: hasMaxLimit && selectedItems.length >= maxSelectionCount,
      // 隐藏表格行选择器
      hidden: !enableBatchOperate,
      // 已选择的表格行记录数据的唯一ID字段值数组
      selectedRowKeys: selectedItems?.map(item => item[recordKey] as string),
      // 表格行选择器变化事件
      onChange(newKeys = [], rows = []) {
        const map = arrayToMap<RecordItem>(
          [...rows, ...selectedItems],
          recordKey,
        );
        const newSelectKeys = hasMaxLimit
          ? newKeys.slice(0, maxSelectionCount)
          : newKeys;
        const newSelectedItems = newSelectKeys
          .map(key => map[key])
          .filter(Boolean);
        setSelectedItems(newSelectedItems as RecordItem[]);
      },
    };
    // 表格行选择器获取checkbox属性事件, 超出最大选中数量时禁用所有未选项
    if (hasMaxLimit) {
      const selectedKeyMap = new Map(
        selectedItems.map(item => [item[recordKey], item]),
      );
      newRowSelection.getCheckboxProps = (record: RecordItem) => ({
        disabled: isSelectionMaximum && !selectedKeyMap.has(record[recordKey]),
      });
    }
    return newRowSelection;
  }, [enableBatchOperate, selectedItems, maxSelectionCount, recordKey]);

  const onCancelBatchOperate = () => {
    setEnableBatchOperate(false);
    setSelectedItems([]);
  };

  return {
    enableBatchOperate,
    selectedItems,
    setEnableBatchOperate,
    onCancelBatchOperate,
    setSelectedItems,
    rowSelection,
  };
}
