import React, { useState, type ReactNode } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { type tag } from '@cozeloop/api-schema/data';
import {
  Space,
  Typography,
  Tag,
  Divider,
  CozAvatar,
} from '@coze-arch/coze-design';

import { TAG_TYPE_TO_NAME_MAP } from '@/const';

type TagInfo = tag.TagInfo;

/**
 * 操作项配置接口
 */
interface ActionItem {
  /** 操作项标签文本或节点 */
  label: ReactNode;
  /** 操作项图标 当展示 icon 时，label 会被隐藏 */
  icon?: ReactNode;
  /** 是否禁用 */
  disabled?: boolean;
  /** 禁用时的提示文本 */
  disabledTooltip?: string;
  /** 是否隐藏此操作项 */
  hide?: boolean;
  /** 文本类型，如 'danger' 表示危险操作 */
  type?: 'primary' | 'secondary' | 'tertiary' | 'quaternary' | 'danger';
  /** 点击回调函数 */
  onClick?: () => void;
  /** 异步点击回调函数，如果传入此函数，会优先执行此函数，并在执行期间阻止重复点击 */
  asyncOnClick?: () => Promise<unknown>;
}

/**
 * 标签项组件的属性接口
 */
export interface TagsItemProps {
  /** 标签数据 */
  tagDetail: TagInfo;
  /** 是否选中 */
  selected?: boolean;
  /** 是否禁用 */
  disabled?: boolean;
  /** 禁用时的提示文本 */
  disabledTooltip?: string;
  /** 点击回调 */
  onClick?: (tag: TagInfo) => void;
  /** 自定义类名 */
  className?: string;
  /** 自定义操作组件*/
  actions?: ReactNode;
  /** 操作项配置列表 如果传入，则展示操作项，否则展示 actions */
  actionItems?: ActionItem[];
  /** 是否只在鼠标 hover 时展示 actions，默认为 false，直接展示 actions */
  showActionsOnHover?: boolean;
}

/**
 * 防抖操作按钮组件
 */
interface ActionButtonProps {
  action: ActionItem;
  disabled?: boolean;
}

function ActionButton({ action, disabled }: ActionButtonProps) {
  const [isLoading, setIsLoading] = useState(false);
  // 计算禁用状态 操作项级别的禁用状态优先级高于组件级别的禁用状态
  const isDisabled = action.disabled ?? disabled;

  const handleClick = async (e: React.MouseEvent) => {
    e.stopPropagation();

    if (isDisabled || isLoading) {
      return;
    }

    // 优先执行异步函数
    if (action.asyncOnClick) {
      setIsLoading(true);
      try {
        await action.asyncOnClick();
      } catch (error) {
        console.error('异步操作执行失败:', error);
      } finally {
        setIsLoading(false);
      }
    } else {
      // 执行同步函数
      action.onClick?.();
    }
  };

  return (
    <TooltipWhenDisabled
      content={action.disabledTooltip || action.label}
      disabled={
        // 如果 是Icon 或者 是 disabled 并且有 disabledTooltip，则显示tooltip
        Boolean(action.icon) || (isDisabled && !!action.disabledTooltip)
      }
    >
      <Typography.Text
        size="small"
        className={classNames('!text-[13px]', {
          'opacity-40': isDisabled,
          'cursor-not-allowed': isDisabled,
        })}
        type={action.type}
        disabled={isDisabled}
        onClick={handleClick}
        link={!action.type}
      >
        {action.icon ? action.icon : action.label}
      </Typography.Text>
    </TooltipWhenDisabled>
  );
}

/**
 * 标签项组件
 *
 * 用于展示单个标签的详细信息，包括标签名称、分类、描述和更新人信息。
 * 支持选中状态、禁用状态和自定义操作。
 *
 * @param props - 组件属性
 * @param props.tagDetail - 标签数据对象
 * @param props.selected - 是否选中，默认为 false
 * @param props.disabled - 是否禁用，默认为 false
 * @param props.onClick - 点击回调函数，接收标签数据作为参数
 * @param props.className - 自定义 CSS 类名
 * @param props.actions - 自定义操作组件，显示在标签项右侧
 * @param props.actionItems - 操作项配置列表
 * @param props.showActionsOnHover - 是否只在鼠标 hover 时展示 actions，默认为 false
 *
 * @example
 * ```tsx
 * <TagsItem
 *   tagDetail={tagInfo}
 *   selected={true}
 *   onClick={(tag) => console.log('Clicked:', tag)}
 *   actions={<Button>操作</Button>}
 *   actionItems={[
 *     { label: '编辑', onClick: () => {} },
 *     { label: '删除', type: 'danger', onClick: () => {} }
 *   ]}
 *   showActionsOnHover={true}
 * />
 * ```
 */
export const TagsItem: React.FC<TagsItemProps> = ({
  tagDetail,
  selected = false,
  disabled = false,
  disabledTooltip,
  onClick,
  className = '',
  actions,
  actionItems,
  showActionsOnHover = false,
}) => {
  // 渲染操作项
  const renderActions = () => {
    if (actionItems && actionItems.length > 0) {
      const filteredActions = actionItems.filter(action => !action.hide);
      return (
        <Space spacing={20}>
          {filteredActions.map((action, index) => (
            <ActionButton key={index} action={action} disabled={disabled} />
          ))}
        </Space>
      );
    }
    return actions;
  };

  return (
    <TooltipWhenDisabled
      disabled={disabled && !!disabledTooltip}
      content={disabledTooltip}
    >
      <div
        className={classNames(
          'flex items-center p-[10px] rounded-[6px] w-full cursor-default group',
          selected
            ? 'coz-mg-hglt hover:coz-mg-hglt-hovered'
            : 'hover:coz-mg-secondary-hovered active:coz-mg-secondary-pressed',
          {
            'opacity-40 cursor-not-allowed': disabled,
          },
          className,
        )}
        onClick={e => {
          if (disabled) {
            return e.stopPropagation();
          }
          onClick?.(tagDetail);
        }}
      >
        <div className="flex-1 min-w-0">
          {/* 标签名称和分类 */}
          <Space className="mb-[7px] w-full">
            <div className="min-w-0">
              <Typography.Text
                className={classNames('!font-semibold !coz-fg-primary')}
                ellipsis={{
                  rows: 1,
                  showTooltip: { opts: { theme: 'dark' } },
                }}
              >
                {tagDetail.tag_key_name}
              </Typography.Text>
            </div>
            {tagDetail.content_type ? (
              <Tag
                color="grey"
                className="shrink-0 font-semibold !coz-fg-primary"
              >
                {TAG_TYPE_TO_NAME_MAP[tagDetail.content_type]}
              </Tag>
            ) : null}
          </Space>

          <div className="flex w-full">
            {/* 标签描述 */}
            <div className="min-w-0 flex items-center">
              <Typography.Text
                ellipsis={{
                  rows: 1,
                  showTooltip: { opts: { theme: 'dark' } },
                }}
                className={classNames('text-xs !coz-fg-secondary')}
              >
                {tagDetail.description}
              </Typography.Text>
            </div>
            {tagDetail.description ? (
              <Divider layout="vertical" margin={12} />
            ) : null}

            {/* 更新人信息 */}
            <Space align="center" className="shrink-0">
              <Typography.Text
                type="secondary"
                className={classNames('text-xs !coz-fg-secondary')}
              >
                {I18n.t('updated_by')}
              </Typography.Text>
              <CozAvatar
                size="small"
                src={tagDetail.base_info?.updated_by?.avatar_url}
                className="w-[20px] h-[20px]"
              />
              <Typography.Text
                className={classNames('text-[13px] !coz-fg-primary')}
              >
                {tagDetail.base_info?.updated_by?.name}
              </Typography.Text>
            </Space>
          </div>
        </div>
        <div
          className={classNames(
            'ml-6 mr-[2px] whitespace-nowrap flex items-center',
            showActionsOnHover ? 'invisible group-hover:visible' : '',
          )}
        >
          {renderActions()}
        </div>
      </div>
    </TooltipWhenDisabled>
  );
};
