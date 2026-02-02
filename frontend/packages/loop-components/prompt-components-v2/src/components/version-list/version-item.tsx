// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { type ReactNode } from 'react';

import cs from 'classnames';
import { formatTimestampToString } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { UserProfile } from '@cozeloop/components';
import { type CommitInfo, type Label } from '@cozeloop/api-schema/prompt';
import { type UserInfoDetail } from '@cozeloop/api-schema/foundation';
import { IconCozEdit } from '@coze-arch/coze-design/icons';
import {
  Button,
  Descriptions,
  Space,
  Tag,
  Typography,
} from '@coze-arch/coze-design';

import { usePromptDevProviderContext } from '../prompt-develop/components/prompt-provider';

import styles from './index.module.less';

export default function VersionItem({
  version,
  baseVersion,
  active,
  labels,
  className,
  onClick,
  onEditLabels,
  renderExraInfos,
}: {
  version?: CommitInfo & { user?: UserInfoDetail };
  baseVersion?: string;
  active?: boolean;
  labels?: Label[];
  className?: string;
  onClick?: () => void;
  onEditLabels?: (labels: Label[]) => void;
  renderExraInfos?: (
    version?: CommitInfo & { user?: UserInfoDetail },
  ) => ReactNode;
}) {
  const isDraft = !version?.version;
  const { submitConfig } = usePromptDevProviderContext();

  return (
    <div className={`group flex cursor-pointer ${className}`} onClick={onClick}>
      <div className="w-6 h-10 flex items-center shrink-0">
        <div
          className={`w-2 h-2 rounded-full ${active ? 'bg-green-700' : 'bg-gray-300'} `}
        />
      </div>
      <div
        className={`grow px-2 pt-2 rounded-m ${active ? 'bg-gray-100' : ''} group-hover:bg-gray-100`}
      >
        <Descriptions
          align="left"
          className={cs(styles.description, className)}
        >
          <Tag color={isDraft ? 'primary' : 'green'} className="mb-2">
            {isDraft ? I18n.t('current_draft') : I18n.t('submit')}
          </Tag>
          {isDraft ? null : (
            <Descriptions.Item itemKey={I18n.t('version')}>
              <span className="font-medium">{version.version ?? '-'}</span>
              {baseVersion === version.version ? (
                <Tag color="brand" className="ml-1">
                  {I18n.t('prompt_source_version')}
                </Tag>
              ) : null}
            </Descriptions.Item>
          )}
          {!version?.committed_at ? null : (
            <Descriptions.Item
              itemKey={isDraft ? I18n.t('save_time') : I18n.t('submit_time')}
              className="!text-[13px]"
            >
              <span className="font-medium !text-[13px]">
                {version?.committed_at
                  ? formatTimestampToString(
                      version?.committed_at,
                      'YYYY-MM-DD HH:mm:ss',
                    )
                  : '-'}
              </span>
            </Descriptions.Item>
          )}
          {isDraft && !version?.committed_by ? null : (
            <Descriptions.Item
              itemKey={I18n.t('submit_user')}
              className="!text-[13px]"
            >
              <UserProfile
                avatarUrl={version?.user?.avatar_url}
                name={version?.user?.nick_name}
              />
            </Descriptions.Item>
          )}
          {isDraft ? null : (
            <Descriptions.Item
              itemKey={I18n.t('version_description')}
              className="!text-[13px]"
            >
              <div className="max-w-[195px]">
                <Typography.Text
                  ellipsis={{
                    showTooltip: {
                      opts: {
                        theme: 'dark',
                      },
                    },
                  }}
                  className="!text-[13px]"
                >
                  {version.description || '-'}
                </Typography.Text>
              </div>
            </Descriptions.Item>
          )}
          {!isDraft && !submitConfig?.hideVersionLabel ? (
            <Descriptions.Item
              itemKey={I18n.t('prompt_version_tag')}
              className="!text-[13px]"
            >
              <Space spacing={4} wrap>
                {labels?.map(item => (
                  <Tag key={item.key} color="grey" className="max-w-[80px]">
                    <Typography.Text
                      className="!coz-fg-primary !text-[12px]"
                      ellipsis={{
                        showTooltip: {
                          opts: {
                            theme: 'dark',
                          },
                        },
                      }}
                    >
                      {item.key}
                    </Typography.Text>
                  </Tag>
                ))}

                <Button
                  icon={<IconCozEdit />}
                  size="mini"
                  color="secondary"
                  onClick={e => {
                    e.stopPropagation();
                    onEditLabels?.(labels || []);
                  }}
                ></Button>
              </Space>
            </Descriptions.Item>
          ) : null}
          {renderExraInfos?.(version)}
        </Descriptions>
      </div>
    </div>
  );
}
