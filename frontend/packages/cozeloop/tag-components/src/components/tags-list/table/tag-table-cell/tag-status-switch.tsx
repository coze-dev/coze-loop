import { GuardActionType, GuardPoint, useGuard } from '@cozeloop/guard';
import { tag } from '@cozeloop/api-schema/data';
import { Popconfirm, Switch, Tooltip } from '@coze-arch/coze-design';

import { useUpdateTagStatus } from '@/hooks/use-update-tag-status';

const { TagStatus } = tag;

interface TagStatusSwitchProps {
  tagInfo: tag.TagInfo;
}

export const TagStatusSwitch = (props: TagStatusSwitchProps) => {
  const { tagInfo } = props;

  const guard = useGuard({
    point: GuardPoint['data.tag.edit'],
  });

  const enabled = tagInfo.status === TagStatus.Active;

  const { runAsync: updateTagStatus, loading } = useUpdateTagStatus();

  const title = enabled ? '禁用标签' : '启用标签';
  const content = enabled ? '禁用后该标签无法被搜索添加' : '确定启用该标签吗？';
  const okText = enabled ? '禁用' : '启用';

  return (
    <div onClick={e => e.stopPropagation()}>
      <Popconfirm
        title={title}
        content={content}
        okText={okText}
        cancelText="取消"
        cancelButtonProps={{
          color: 'primary',
        }}
        okButtonProps={{
          color: enabled ? 'red' : 'brand',
        }}
        onConfirm={() => {
          updateTagStatus({
            tagKeyIds: [tagInfo.tag_key_id ?? ''],
            toStatus: enabled ? TagStatus.Inactive : TagStatus.Active,
          }).then(() => {
            tagInfo.status = enabled ? TagStatus.Inactive : TagStatus.Active;
          });
        }}
      >
        <span>
          <Tooltip theme="dark" content={enabled ? '启用标签' : '禁用标签'}>
            <Switch
              size="mini"
              checked={enabled}
              loading={loading}
              disabled={guard.data.type === GuardActionType.READONLY}
            />
          </Tooltip>
        </span>
      </Popconfirm>
    </div>
  );
};
