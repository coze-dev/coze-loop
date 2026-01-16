// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useEffect } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { handleScrollToBottom } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { PromptType, type Prompt } from '@cozeloop/api-schema/prompt';
import { Banner, Popover, Spin, Typography } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { useVersionList } from '@/hooks/use-version-list';
import { PromptDisplayCard } from '@/components/prompt-display-card';

import styles from './segment-version-change.module.less';

export function SegmentVersionChange({
  segmentInfo,
  onItemClick,
}: {
  segmentInfo?: Prompt;
  onItemClick?: (prompt: Prompt) => void;
}) {
  const spaceID = segmentInfo?.workspace_id;

  const { promptInfo, templateType } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      templateType: state.templateType,
    })),
  );

  const versionListService = useVersionList({
    promptID: segmentInfo?.id,
    spaceID,
    withCommitDetail: true,
  });

  useEffect(() => {
    versionListService.reload();
  }, []);

  return (
    <div
      style={{
        maxHeight: 200,
        width: 100,
        overflowY: 'auto',
        overflowX: 'hidden',
      }}
      onScroll={e => {
        if (versionListService.noMore) {
          return;
        }

        handleScrollToBottom(e, versionListService.loadMore);
      }}
    >
      {versionListService.data?.list?.length && !versionListService.loading
        ? versionListService.data.list.map(info => {
            const promptCommit =
              versionListService?.data?.promptCommitMap?.[info.version || ''];
            console.log('promptCommit', promptCommit, templateType);
            const isSameTemplateType =
              promptCommit?.prompt_template?.template_type ===
              templateType?.type;
            const isMultiSnippet =
              promptCommit?.prompt_template?.has_snippet &&
              promptInfo?.prompt_basic?.prompt_type === PromptType.Snippet;

            const versionDisabled = isMultiSnippet || !isSameTemplateType;
            // && info.has_segment
            return (
              <Popover
                key={info.version}
                content={
                  <div className="w-full flex flex-col gap-1">
                    {versionDisabled ? (
                      <Banner
                        bordered
                        type="warning"
                        description={
                          !isSameTemplateType
                            ? I18n.t(
                                'prompt_version_inconsistent_with_prompt_template',
                              )
                            : I18n.t(
                                'prompt_version_contains_first_level_nesting',
                              )
                        }
                        closeIcon={null}
                        className="mt-1"
                      />
                    ) : null}
                    <PromptDisplayCard
                      promptID={segmentInfo?.id}
                      promptVersion={info.version}
                      userDetail={info.user}
                    />
                  </div>
                }
                position="rightTop"
                trigger="hover"
                stopPropagation
              >
                <div
                  className={classNames(
                    'w-full px-2 py-1.5 flex items-center gap-1 cursor-pointer hover:coz-mg-primary-hovered rounded-[6px] overflow-hidden',
                    {
                      '!cursor-not-allowed': versionDisabled,
                    },
                  )}
                  onClick={() =>
                    !versionDisabled &&
                    onItemClick?.({
                      id: segmentInfo?.id,
                      prompt_key: segmentInfo?.prompt_key,
                      workspace_id: segmentInfo?.workspace_id,
                      prompt_basic: {
                        prompt_type: PromptType.Snippet,
                        display_name: segmentInfo?.prompt_basic?.display_name,
                      },
                      prompt_commit: {
                        commit_info: {
                          version: info.version,
                        },
                        detail: promptCommit,
                      },
                    })
                  }
                >
                  <Typography.Text
                    ellipsis={{ showTooltip: true }}
                    style={{ width: '100%' }}
                  >
                    {info.version}
                  </Typography.Text>
                </div>
              </Popover>
            );
          })
        : !versionListService.loading && (
            <div className={styles['empty-item']}>
              {I18n.t('prompt_no_content')}
            </div>
          )}
      {(versionListService.loading || versionListService.loadingMore) &&
      segmentInfo?.id ? (
        <div className="w-full flex items-center justify-center h-8">
          <Spin size="small" />
        </div>
      ) : null}
    </div>
  );
}
