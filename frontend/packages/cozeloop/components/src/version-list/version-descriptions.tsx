/* eslint-disable complexity */
import cs from 'classnames';
import { formatTimestampToString } from '@cozeloop/toolkit';
import { type UserInfo } from '@cozeloop/api-schema/evaluation';
import { Descriptions, Tag, Typography } from '@coze-arch/coze-design';

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
  const {
    version: versionName,
    draftSubmitText = '保存时间',
    submitTime,
    submitter,
    description,
    isDraft = false,
  } = version || {};

  return (
    <Descriptions align="left" className={cs(styles.description, className)}>
      <Tag color={isDraft ? 'primary' : 'green'} className="mb-2">
        {isDraft ? '当前草稿' : '提交'}
      </Tag>
      {isDraft ? null : (
        <Descriptions.Item itemKey="版本">
          <span className="font-medium">{versionName ?? '-'}</span>
        </Descriptions.Item>
      )}
      {!submitTime ? null : (
        <Descriptions.Item
          itemKey={isDraft ? draftSubmitText : '提交时间'}
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
        <Descriptions.Item itemKey="提交人" className="!text-[13px]">
          <UserProfile
            name={submitter?.name}
            avatarUrl={submitter?.avatar_url}
          />
        </Descriptions.Item>
      )}
      {isDraft ? null : (
        <Descriptions.Item itemKey="版本说明" className="!text-[13px]">
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
