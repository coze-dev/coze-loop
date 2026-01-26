// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { Typography, type SelectProps } from '@coze-arch/coze-design';

import { handleScrollToBottom } from '@/utils/base';
import { useVersionList } from '@/hooks/use-version-list';

interface PromptVersionSelectProps extends SelectProps {
  className?: string;
  promptID?: Int64;
}

export function PromptVersionSelect({
  className,
  promptID,
  ...rest
}: PromptVersionSelectProps) {
  const {
    versionListData,
    versionListLoadMore,
    versionListLoading,
    versionListReload,
    versionListLoadingMore,
  } = useVersionList({
    promptID: `${promptID ?? ''}`,
  });

  useEffect(() => {
    versionListReload();
  }, [promptID]);

  return (
    <BaseSearchSelect
      className={classNames(className)}
      emptyContent={I18n.t('no_data')}
      loading={versionListLoading}
      onClickRefresh={versionListReload}
      optionList={versionListData?.list?.map(item => ({
        label: (
          <div className="flex flex-row items-center w-full pr-2">
            <div className="flex-shrink-0 text-[13px] coz-fg-plus">
              {item.version}
            </div>
            <Typography.Text
              className="flex-1 w-0 ml-3 text-xs font-medium coz-fg-secondary"
              ellipsis={{
                showTooltip: { opts: { theme: 'dark' } },
                rows: 1,
              }}
            >
              {item?.description}
            </Typography.Text>
          </div>
        ),

        value: item.version,
      }))}
      filter={false}
      onListScroll={e => {
        if (versionListLoadingMore) {
          return;
        }
        handleScrollToBottom(e, versionListLoadMore);
      }}
      showRefreshBtn
      {...rest}
    />
  );
}
