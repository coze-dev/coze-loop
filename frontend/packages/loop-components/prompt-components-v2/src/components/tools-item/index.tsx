// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { IconButton, Popconfirm, Typography } from '@coze-arch/coze-design';

import { type ToolItemProps } from './type';

import styles from './index.module.less';

export function ToolItem({
  data,
  onClick,
  onDelete,
  showDelete,
  disabled,
}: ToolItemProps) {
  return (
    <div
      className={styles['tools-list-item']}
      key={data?.function?.name}
      onClick={() => !disabled && onClick?.(data)}
    >
      <div className="flex items-center justify-between w-full h-8">
        <Typography.Text
          className="flex items-center gap-1 cursor-pointer variable-text"
          ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
          style={{ maxWidth: 'calc(100% - 30px)' }}
        >
          {data?.function?.name}
        </Typography.Text>
        {!showDelete ? null : (
          <Popconfirm
            title={I18n.t('delete_function')}
            content={I18n.t('confirm_delete_function')}
            cancelText={I18n.t('cancel')}
            okText={I18n.t('delete')}
            okButtonProps={{ color: 'red' }}
            stopPropagation={true}
            onConfirm={e => {
              onDelete?.(data?.function?.name);
              e.stopPropagation();
            }}
          >
            <IconButton
              size="mini"
              color="secondary"
              className={styles['delete-btn']}
              onClick={e => e.stopPropagation()}
              icon={<IconCozTrashCan />}
            />
          </Popconfirm>
        )}
      </div>
      <div className="flex gap-1 w-full">
        <Typography.Text type="tertiary" size="small" className="flex-shrink-0">
          Description:
        </Typography.Text>
        <Typography.Text
          type="secondary"
          size="small"
          className="flex-1"
          ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
        >
          {data?.function?.description}
        </Typography.Text>
      </div>
      <div className="flex gap-1 w-full">
        <Typography.Text type="tertiary" size="small" className="flex-shrink-0">
          {I18n.t('prompt_simulated_value_colon')}
        </Typography.Text>
        <Typography.Text
          type="secondary"
          size="small"
          className="flex-1"
          ellipsis={{
            showTooltip: {
              opts: {
                theme: 'dark',
                content: (
                  <div className="max-h-[300px] overflow-auto styled-scrollbar !pr-[6px]">
                    {data.mock_response}
                  </div>
                ),
              },
            },
          }}
        >
          {data.mock_response}
        </Typography.Text>
      </div>
    </div>
  );
}
