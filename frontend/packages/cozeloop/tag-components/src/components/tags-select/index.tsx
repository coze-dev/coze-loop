// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import React, { useState, useRef } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { ArrayUtils } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';
import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import { Select, Spin, type SemiSelect } from '@coze-arch/coze-design';

import { useTagFormModal } from '@/hooks/use-tag-form-modal';
import { TagsItem } from '@/components/tags-item';

type TagInfo = tag.TagInfo;
type TagContentType = tag.TagContentType;
const { TagStatus } = tag;

export interface TagsSelectProps {
  /** 当前选中的标签  */
  value?: TagInfo['tag_key_id'];
  onChange?: (value: NonNullable<TagInfo['tag_key_id']>) => void;
  /** 是否禁用 会展示已选中标签的标签值 但无法切换标签，默认 false 不禁用 */
  disabled?: boolean;
  /** 是否显示新建标签按钮，默认 false 不显示 */
  showCreateButton?: boolean;
  /** 是否点击下拉框后自动关闭，默认 false 不自动关闭 */
  clickToHide?: boolean;
  /** 是否指定空间ID，如果指定，则使用指定空间ID，否则使用当前空间ID */
  spaceId?: string;
  /**
   * 标签更新回调，当标签信息发生变化时触发
   *
   * 与 `onChange` 的区别是：
   * `onChange` 只响应选中的 tagKeyID 的变更；
   * 而 `onTagUpdate` 则是响应选中的 tagKeyID 所对应的 tag 的变更，比如刷新后 tagKeyID 没变，但 tag 信息发生变化
   *
   * 当 tagKeyID 找不到对应的 tag 时，`onTagUpdate` 会抛出 undefined
   */
  onTagUpdate?: (t?: TagInfo) => void;
  /** 标签选项类型限制 默认不指定，不限制，所有类型标签都展示 */
  contentTypes?: TagContentType[];
  /** 是否默认选中第一个可用标签项，默认 false 不自动选中 */
  defaultSelectFirst?: boolean;
}

export const TagsSelect: React.FC<TagsSelectProps> = ({
  value,
  onChange,
  onTagUpdate,
  disabled,
  clickToHide,
  spaceId: spaceIdProps,
  contentTypes,
  defaultSelectFirst = false,
}) => {
  const { spaceID } = useSpace();
  // 为了使下拉框的宽度和 trigger 一致，需要获取 trigger 的宽度
  const [triggerWidth, setTriggerWidth] = useState(0);
  const [tagMap, setTagMap] = useState<Record<string, TagInfo>>({});
  const selectRef = useRef<SemiSelect | null>();

  const tagsReq = useRequest(
    async () => {
      const res = await DataApi.SearchTags({
        workspace_id: spaceIdProps || spaceID,
        page_number: 1,
        page_size: 200,
        content_types: contentTypes,
        order_by: {
          field: 'updated_at',
        },
      });
      return res.tagInfos || [];
    },
    {
      onSuccess: (tags: TagInfo[]) => {
        const newTagMap = ArrayUtils.array2Map(tags, 'tag_key_id');
        setTagMap(newTagMap);

        // 如果已经有选中的值，或者没有标签数据，直接返回
        if (value || !tags?.length) {
          onTagUpdate?.(newTagMap[value || '']);
          return;
        }

        // 如果启用了默认选中第一个
        if (defaultSelectFirst) {
          const firstAvailableTag = tags.find(
            t => t.status === TagStatus.Active,
          );
          if (firstAvailableTag) {
            onChange?.(firstAvailableTag.tag_key_id || '');
            onTagUpdate?.(firstAvailableTag);
          }
        }
      },
    },
  );

  const { modal: renderTagModal, openEdit: openTagEditModal } = useTagFormModal(
    {
      onSuccess: () => tagsReq.refresh(),
    },
  );

  if (tagsReq.loading) {
    return <Spin spinning />;
  }

  return (
    <>
      <Select
        ref={ref => {
          const { left, right } =
            ref?.triggerRef.current?.getBoundingClientRect() || {
              left: 0,
              right: 0,
            };
          // clientWidth 不含 border，故使用 right - left
          setTriggerWidth(right - left);
          selectRef.current = ref;
        }}
        loading={tagsReq.loading}
        disabled={disabled}
        value={value}
        optionList={tagsReq.data?.map(t => ({
          value: t.tag_key_id,
        }))}
        renderOptionItem={props => {
          const t = tagMap[props.value as string];
          const optionDisabled = t.status !== TagStatus.Active;

          return (
            <TagsItem
              className="group"
              tagDetail={t}
              selected={value === t.tag_key_id}
              onClick={() => {
                onChange?.(t.tag_key_id || '');
                onTagUpdate?.(t);
                if (clickToHide) {
                  selectRef.current?.close();
                }
              }}
              disabled={optionDisabled}
              disabledTooltip={I18n.t('data_engine_label_disabled')}
              actionItems={[
                {
                  label: I18n.t('data_engine_edit'),
                  onClick: () => {
                    if (optionDisabled) {
                      return;
                    }
                    selectRef.current?.close();
                    openTagEditModal(t);
                  },
                  disabled: optionDisabled,
                },
              ]}
              showActionsOnHover={true}
            />
          );
        }}
        placeholder={
          <span className="flex items-center h-[32px]">
            {I18n.t('data_engine_no_label_selected')}
          </span>
        }
        dropdownStyle={{
          width: triggerWidth || 'auto',
          boxSizing: 'border-box',
        }}
        renderSelectedItem={option => {
          const t = tagMap[option.value as string];
          return (
            <TagsItem
              className="group !hover:bg-transparent !bg-transparent"
              tagDetail={t}
              actionItems={
                disabled
                  ? []
                  : [
                      {
                        label: I18n.t('data_engine_refresh'),
                        icon: <IconCozRefresh className="coz-fg-hglt" />,
                        onClick: () => {
                          tagsReq.refresh();
                        },
                      },
                    ]
              }
              showActionsOnHover={true}
            />
          );
        }}
        className={classNames(
          '!h-auto w-full min-h-[32px]',
          '[&_.semi-select-content-wrapper]:w-full',
        )}
      />
      {renderTagModal}
    </>
  );
};
