// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */

import { useEffect, useState } from 'react';

import { isUndefined } from 'lodash-es';
import classNames from 'classnames';
import { handleScrollToBottom } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { PageLoading } from '@cozeloop/components';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { IconCozMargin, IconCozPadding } from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Modal,
  Skeleton,
  Spin,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { useVersionList } from '@/hooks/use-version-list';

import { usePromptDevProviderContext } from '../prompt-develop/components/prompt-provider';
import { DiffContent } from './diff-content';

interface PromptDiffProps {
  visible?: boolean;
  onCancel?: () => void;
  data: Prompt;
  sameDesc?: string;
  onlineVersion?: string;
  currentVersionTitle?: string;
  diffWithEmptyPreInfo?: boolean;
  defaultDiffPromptVersion?: string;
  onlyOneTab?: boolean;
  onlyShowContent?: boolean;
  contentHeight?: string | number;
}
export function PromptDiff({
  visible,
  onCancel,
  data,
  sameDesc,
  onlineVersion,
  currentVersionTitle,
  diffWithEmptyPreInfo,
  defaultDiffPromptVersion,
  onlyOneTab,
  onlyShowContent,
  contentHeight,
}: PromptDiffProps) {
  const { spaceID } = usePromptDevProviderContext();
  const [currentDiffVersion, setCurrentDiffVersion] = useState<string>();
  const [showFullScreen, setShowFullScreen] = useState(false);

  const versionService = useVersionList({
    promptID: data?.id,
    spaceID,
    withCommitDetail: true,
  });

  const basVersion = data?.prompt_basic?.latest_version;

  useEffect(() => {
    console.info(
      '____',
      visible,
      data?.id,
      basVersion,
      defaultDiffPromptVersion,
    );
    if (data?.id && visible) {
      if (basVersion || !isUndefined(defaultDiffPromptVersion)) {
        versionService.reload();
        setCurrentDiffVersion(basVersion || defaultDiffPromptVersion);
      } else {
        versionService
          .reloadAsync()
          .then(res => {
            const firstVersion = res.list?.[0].version;
            const secondVersion = res.list?.[1].version;
            if (
              firstVersion === data.prompt_commit?.commit_info?.version &&
              secondVersion
            ) {
              setCurrentDiffVersion(secondVersion);
            } else {
              setCurrentDiffVersion(firstVersion);
            }
          })
          .catch(() => {
            setCurrentDiffVersion('');
          });
      }
    }
  }, [data?.id, basVersion, visible, defaultDiffPromptVersion]);

  const curContentHeight =
    contentHeight ??
    (showFullScreen ? 'calc(100vh - 100px)' : 'calc(100vh - 260px)');

  const contentDom = (
    <Skeleton
      loading={versionService.loading || !data?.id || !visible}
      placeholder={
        <div className="w-full h-full" style={{ height: curContentHeight }}>
          <PageLoading className="w-full h-full" />
        </div>
      }
    >
      <div
        className="w-full rounded border border-solid coz-stroke-primary overflow-hidden"
        style={{
          height: curContentHeight,
        }}
      >
        <div className="flex w-full h-full">
          <div
            className={classNames(
              'w-60 flex flex-col border-0 !border-r border-solid coz-stroke-primary',
              {
                hidden: !versionService.data?.list.length,
              },
            )}
          >
            <div className="border-0 border-b border-solid coz-stroke-primary">
              <Typography.Text className="px-3 py-3 block" strong>
                {I18n.t('prompt_comparable_versions')}
              </Typography.Text>
            </div>
            <div
              className="flex-1 overflow-y-auto"
              onScroll={e => {
                if (versionService.noMore) {
                  return;
                }
                handleScrollToBottom(e, versionService.loadMore);
              }}
            >
              {versionService.data?.list?.map(item => (
                <div
                  className="flex flex-col gap-1 px-3 py-2 cursor-pointer"
                  key={item.version}
                  style={{
                    borderBottom: '1px solid #EAEDF1',
                    background:
                      currentDiffVersion === item.version
                        ? 'var(--Brand-Purple-Brand-3, #EFF1FF)'
                        : 'transparent',
                  }}
                  onClick={() => {
                    setCurrentDiffVersion(item.version);
                  }}
                >
                  <Typography.Text className="font-[13px]">
                    {item.version}
                  </Typography.Text>
                  <div className="flex-1 flex items-center w-full gap-1">
                    {item?.version === onlineVersion ? (
                      <Tag className="flex-shrink-0">
                        {I18n.t('prompt_current_version')}
                      </Tag>
                    ) : null}
                    {item?.version === basVersion ? (
                      <Tag className="flex-shrink-0">
                        {I18n.t('prompt_source_version')}
                      </Tag>
                    ) : null}
                  </div>
                </div>
              ))}

              {versionService.loadingMore ? (
                <div className="coz-fg-primary" style={{ textAlign: 'center' }}>
                  <Spin size="small" />
                </div>
              ) : null}
            </div>
          </div>
          <div className="flex-1 flex flex-col h-full w-full coz-bg-primary overflow-hidden">
            <DiffContent
              spaceID={spaceID}
              preVersion={currentDiffVersion}
              currentInfo={data}
              sameDesc={sameDesc}
              onlineVersion={onlineVersion}
              currentVersionTitle={currentVersionTitle}
              diffWithEmptyPreInfo={diffWithEmptyPreInfo}
              onlyOneTab={onlyOneTab}
              showFullScreenBtn={onlyShowContent}
            />
          </div>
        </div>
      </div>
    </Skeleton>
  );

  if (onlyShowContent) {
    return contentDom;
  }

  return (
    <Modal
      visible={visible}
      onCancel={onCancel}
      title={
        <div className="flex items-center justify-between w-full">
          <Typography.Text className="!font-semibold !text-[18px]">
            {I18n.t('prompt_prompt_diff_change_info')}
          </Typography.Text>

          <div className="flex items-center">
            <Button
              icon={
                showFullScreen ? (
                  <IconCozPadding fontSize={12} />
                ) : (
                  <IconCozMargin fontSize={12} />
                )
              }
              color="secondary"
              size="mini"
              onClick={() => setShowFullScreen(!showFullScreen)}
            />

            <Divider layout="vertical" margin={8} />
          </div>
        </div>
      }
      width={1114}
      hasScroll={false}
      fullScreen={showFullScreen}
    >
      {contentDom}
    </Modal>
  );
}
