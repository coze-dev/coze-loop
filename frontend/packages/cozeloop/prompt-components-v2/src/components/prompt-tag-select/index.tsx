import { useEffect } from 'react';

import { handleScrollToBottom } from '@cozeloop/toolkit';
import { BaseSearchSelect, InfoTooltip, useI18n } from '@cozeloop/components';
import { type SelectProps, Spin, Typography } from '@coze-arch/coze-design';

import { useTagList } from '@/hooks/use-tag-list';

interface PromptTagSelectProps extends SelectProps {
  className?: string;
  spaceID?: string;
  descInIcon?: boolean;
}

export function PromptTagSelect({
  spaceID,
  className,
  placeholder,
  descInIcon,
  ...rest
}: PromptTagSelectProps) {
  const I18n = useI18n();
  const tagListService = useTagList({ spaceID });
  const {
    reload: tagListReload,
    data: tagListData,
    loading: tagListLoading,
    loadMore: tagListLoadMore,
    loadingMore: tagListLoadingMore,
    noMore: tagListLoadingNoMore,
  } = tagListService;

  useEffect(() => {
    tagListReload();
  }, [spaceID]);

  return (
    <BaseSearchSelect
      className={className}
      emptyContent={I18n.t('prompt_version_empty_submitted')}
      loading={tagListLoading}
      onClickRefresh={tagListReload}
      optionList={tagListData?.list?.map(item => ({
        label: (
          <div className="flex flex-row items-center w-full pr-2">
            <div className="flex-shrink-0 text-[13px] coz-fg-plus">
              {item.tag_key_name}
            </div>
            {descInIcon ? (
              <InfoTooltip content={item?.description} className="ml-2" />
            ) : (
              <Typography.Text
                className="flex-1 w-0 ml-3 text-xs font-medium coz-fg-secondary"
                ellipsis={{
                  showTooltip: { opts: { theme: 'dark' } },
                  rows: 1,
                }}
              >
                {item?.description}
              </Typography.Text>
            )}
          </div>
        ),

        value: item.tag_key_id,
      }))}
      filter={false}
      onListScroll={e => {
        if (tagListLoadingNoMore) {
          return;
        }
        handleScrollToBottom(e, tagListLoadMore);
      }}
      showRefreshBtn
      placeholder={placeholder ?? I18n.t('version')}
      innerBottomSlot={
        tagListLoadingMore ? (
          <div className="w-full text-center">
            <Spin size="small" />
          </div>
        ) : null
      }
      multiple
      {...rest}
    />
  );
}
