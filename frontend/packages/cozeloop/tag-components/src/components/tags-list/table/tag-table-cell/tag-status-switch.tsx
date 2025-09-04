import { I18n } from '@cozeloop/i18n-adapter';
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

  const title = enabled ? I18n.t('disable_tag') : I18n.t('enable_tag');
  const content = enabled
    ? I18n.t('disabled_tag_not_searchable')
    : I18n.t('confirm_enable_tag');
  const okText = enabled ? I18n.t('disable') : I18n.t('enable');

  return (
    <div onClick={e => e.stopPropagation()}>
      <Popconfirm
        title={title}
        content={content}
        okText={okText}
        cancelText={I18n.t('cancel')}
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
          <Tooltip
            theme="dark"
            content={enabled ? I18n.t('enable_tag') : I18n.t('disable_tag')}
          >
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
