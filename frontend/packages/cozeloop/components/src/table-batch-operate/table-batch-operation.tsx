// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Button } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { type BatchOperateStore } from './use-batch-operate';

export interface BatchOperateProps<RecordItem> {
  /** 批量操作接口 */
  batchOperateStore: BatchOperateStore<RecordItem>;
  /** 自定义批量操作按钮 */
  actions?: React.ReactNode;
}

/** 表格批量操作 */
export function TableBatchOperate<RecordItem>({
  actions,
  batchOperateStore,
}: BatchOperateProps<RecordItem>) {
  const I18n = useI18n();
  const {
    selectedItems,
    setSelectedItems,
    enableBatchOperate,
    setEnableBatchOperate,
  } = batchOperateStore;
  if (!enableBatchOperate) {
    return (
      <Button color="primary" onClick={() => setEnableBatchOperate?.(true)}>
        {I18n.t('bulk_select')}
      </Button>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <div className="text-xs">
        {I18n.t('cozeloop_open_evaluate_selected_data_count', {
          placeholder1: selectedItems.length,
        })}
        <span
          className="ml-1 text-[rgb(var(--coze-up-brand-9))] cursor-pointer"
          onClick={() => {
            setSelectedItems([]);
            setEnableBatchOperate?.(false);
          }}
        >
          {I18n.t('unselect')}
        </span>
      </div>
      {actions}
    </div>
  );
}
