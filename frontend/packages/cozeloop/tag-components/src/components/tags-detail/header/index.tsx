import { formatTimestampToString } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardActionType } from '@cozeloop/guard';
import { UserProfile } from '@cozeloop/components';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { type tag } from '@cozeloop/api-schema/data';
import { IconCozLongArrowUp } from '@coze-arch/coze-design/icons';
import { Typography, Divider, Button } from '@coze-arch/coze-design';

// 头部组件
interface TagDetailHeaderProps {
  tagDetail?: tag.TagInfo;
  onShowEditHistory: () => void;
  onSubmit: () => void;
  changed: boolean;
  guardType: GuardActionType;
  /**
   * 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
   */
  tagListPagePath?: string;
  /**
   * 标签列表参数，用于跳转和拼接 标签详情 / 创建标签 路由路径的查询参数 格式为 key1=value1&key2=value2 不需要带 ?
   */
  tagListPageQuery?: string;
}

export const TagDetailHeader = ({
  tagDetail,
  onShowEditHistory,
  onSubmit,
  changed,
  guardType,
  tagListPagePath,
  tagListPageQuery,
}: TagDetailHeaderProps) => {
  const navigate = useNavigateModule();
  const { tag_key_name, description, base_info } = tagDetail || {};

  return (
    <div className="h-[64px] py-2 px-6 box-border w-full max-w-full min-w-0 flex items-center justify-between border-0 border-b border-solid border-[var(--coz-stroke-primary)] gap-x-2">
      <div className="flex items-center flex-1 overflow-hidden">
        <div
          className="-rotate-90 cursor-pointer mr-2"
          onClick={() =>
            navigate(
              `${tagListPagePath}${tagListPageQuery ? `?${tagListPageQuery}` : ''}`,
            )
          }
        >
          <IconCozLongArrowUp className="w-5 h-5 min-w-[25px]" />
        </div>
        <div className="flex flex-col overflow-hidden w-full">
          <div className="text-[14px] font-medium leading-5 text-[var(--coz-fg-plus)]">
            {tag_key_name ?? ''}
          </div>
          <div className="flex items-center justify-start gap-x-2 text-[12px] font-normal leading-4 text-[var(--coz-fg-secondary)]">
            <div className="inline-block max-w-[380px]">
              <Typography.Text
                className="font-inherit text-inherit leading-inherit overflow-hidden !text-[12px]"
                ellipsis={{
                  showTooltip: {
                    opts: {
                      theme: 'dark',
                    },
                  },
                }}
              >
                {description ?? '-'}
              </Typography.Text>
            </div>
            <Divider layout="vertical" className="h-[12px] mx-[3px]" />
            <div className="text-[var(--coz-fg-secondary)] whitespace-nowrap">
              {I18n.t('update_time', {
                placeholder1: formatTimestampToString(
                  base_info?.updated_at ?? '',
                ),
              })}
            </div>
            <Divider layout="vertical" className="h-[12px] mx-[3px]" />
            <div className="text-[var(--coz-fg-secondary)] whitespace-nowrap">
              {I18n.t('create_time', {
                placeholder1: formatTimestampToString(
                  base_info?.created_at ?? '',
                ),
              })}
            </div>
            <Divider layout="vertical" className="h-[12px] mx-[3px]" />
            <div className="flex items-center flex-1 overflow-hidden">
              <UserProfile
                avatarUrl={base_info?.created_by?.avatar_url}
                name={base_info?.created_by?.name ?? '-'}
                userNameClassName="!text-[12px]"
              />
            </div>
          </div>
        </div>
      </div>
      <div className="flex items-center gap-x-2">
        <Button color="primary" onClick={onShowEditHistory}>
          {I18n.t('change_log')}
        </Button>
        <Button
          color="brand"
          disabled={!changed || guardType === GuardActionType.READONLY}
          onClick={onSubmit}
        >
          {I18n.t('save')}
        </Button>
      </div>
    </div>
  );
};
