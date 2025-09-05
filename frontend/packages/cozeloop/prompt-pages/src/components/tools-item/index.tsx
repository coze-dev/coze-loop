import { type Tool } from '@cozeloop/api-schema/prompt';
import { IconCozTrashCan } from '@coze-arch/coze-design/icons';
import { IconButton, Popconfirm, Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

export type ToolWithMock = Tool & { mock_response?: string };

interface ToolItemProps {
  data: ToolWithMock;
  showDelete?: boolean;
  onClick?: (data: ToolWithMock) => void;
  onDelete?: (name?: string) => void;
  disabled?: boolean;
}

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
            title="删除函数"
            content="确认删除该函数吗？"
            cancelText="取消"
            okText="删除"
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
          模拟值:
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
