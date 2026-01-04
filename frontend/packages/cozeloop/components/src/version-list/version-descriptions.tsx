// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import cs from 'classnames';
import { formatTimestampToString } from '@cozeloop/toolkit';
import { type UserInfo } from '@cozeloop/api-schema/evaluation';
import { Descriptions, Tag, Typography } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { UserProfile } from '../user-profile';

import styles from './index.module.less';
export type Integer64 = string;

export interface Version {
  id: Integer64;
  version?: string;
  submitTime?: Integer64;
  submitter?: UserInfo;
  description?: string;
  isDraft?: boolean;
  draftSubmitText?: string;
}

export default function VersionDescriptions({
  version,
  className,
}: {
  version: Version | undefined;
  className?: string;
}) {
  const I18n = useI18n();
  const {
    version: versionName,
    draftSubmitText = I18n.t('save_time'),
    submitTime,
    submitter,
    description,
    isDraft = false,
  } = version || {};

  return (
    <Descriptions align="left" className={cs(styles.description, className)}>
      <Tag color={isDraft ? 'primary' : 'green'} className="mb-2">
        {isDraft ? I18n.t('current_draft') : I18n.t('submit')}
      </Tag>
      {isDraft ? null : (
        <Descriptions.Item itemKey={I18n.t('version')}>
          <span className="font-medium">{versionName ?? '-'}</span>
        </Descriptions.Item>
      )}
      {!submitTime ? null : (
        <Descriptions.Item
          itemKey={isDraft ? draftSubmitText : I18n.t('submit_time')}
          className="!text-[13px]"
        >
          <span className="font-medium !text-[13px]">
            {submitTime
              ? formatTimestampToString(submitTime, 'YYYY-MM-DD HH:mm:ss')
              : '-'}
          </span>
        </Descriptions.Item>
      )}
      {isDraft && !submitter ? null : (
        <Descriptions.Item
          itemKey={I18n.t('submit_user')}
          className="!text-[13px]"
        >
          <UserProfile
            name={submitter?.name}
            userNameClassName="!max-w-[130px]"
            avatarUrl={submitter?.avatar_url}
          />
        </Descriptions.Item>
      )}
      {isDraft ? null : (
        <Descriptions.Item
          itemKey={I18n.t('version_description')}
          className="!text-[13px]"
        >
          <Typography.Text
            ellipsis={{ rows: 2, showTooltip: true }}
            className="!text-[13px]"
          >
            {description || '-'}
          </Typography.Text>
        </Descriptions.Item>
      )}
    </Descriptions>
  );
}
