// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect } from 'react';

import { handleScrollToBottom } from '@cozeloop/toolkit';
import { BaseSearchSelect, InfoTooltip, useI18n } from '@cozeloop/components';
import { type SelectProps, Spin, Typography } from '@coze-arch/coze-design';

import { useVersionList } from '@/hooks/use-version-list';

interface PromptVersionSelectProps extends SelectProps {
  className?: string;
  promptID?: string;
  spaceID?: string;
  descInIcon?: boolean;
}

export function PromptVersionSelect({
  promptID,
  spaceID,
  className,
  placeholder,
  descInIcon,
  ...rest
}: PromptVersionSelectProps) {
  const I18n = useI18n();
  const versionListService = useVersionList({ promptID, spaceID });
  const {
    reload: versionListReload,
    data: versionListData,
    loading: versionListLoading,
    loadMore: versionListLoadMore,
    loadingMore: versionListLoadingMore,
    noMore: versionListLoadingNoMore,
  } = versionListService;

  useEffect(() => {
    versionListReload();
  }, [promptID, spaceID]);

  return (
    <BaseSearchSelect
      className={className}
      emptyContent={I18n.t('prompt_version_empty_submitted')}
      loading={versionListLoading}
      onClickRefresh={versionListReload}
      optionList={versionListData?.list?.map(item => ({
        label: (
          <div className="flex flex-row items-center w-full pr-2">
            <div className="flex-shrink-0 text-[13px] coz-fg-plus">
              {item.version}
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

        value: item.version,
      }))}
      filter={false}
      onListScroll={e => {
        if (versionListLoadingNoMore) {
          return;
        }
        handleScrollToBottom(e, versionListLoadMore);
      }}
      showRefreshBtn
      placeholder={placeholder ?? I18n.t('version')}
      innerBottomSlot={
        versionListLoadingMore ? (
          <div className="w-full text-center">
            <Spin size="small" />
          </div>
        ) : null
      }
      {...rest}
    />
  );
}
