/* eslint-disable complexity */
import { useState, useRef, useEffect } from 'react';

import { formatTimestampToString } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { UserProfile } from '@cozeloop/components';
import { type common } from '@cozeloop/api-schema/evaluation';
import { type tag } from '@cozeloop/api-schema/data';
import { Descriptions, Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

const TAG_METADATA = {
  tag_name: I18n.t('tag_name'),
  tag_value_name: I18n.t('tag_options'),
  tag_description: I18n.t('tag_description'),
  tag_value_status: I18n.t('tag_option_enable_status'),
  inactive: I18n.t('disable'),
  active: I18n.t('enable'),
  tag_status: I18n.t('tag_status'),
};

const CONTENT_MAX_HEIGHT = 120;

interface EditHistoryItemProps {
  updatedAt?: string;
  updatedBy?: common.UserInfo;
  changeLog?: tag.ChangeLog[];
}

const generateDescFromChangeLog = (
  changeLogs: tag.ChangeLog[],
  updatedBy?: string,
  updatedAt?: string,
) => {
  if (!changeLogs || changeLogs.length === 0) {
    return '-';
  }

  return changeLogs.reduce((desc, logItem) => {
    if (logItem.operation === 'create' && logItem.target === 'tag') {
      desc.push(
        <span>
          <span>
            {I18n.t('tag_creator_name', { placeholder1: updatedBy || '-' })}
          </span>
          ,
          <span>
            {I18n.t('tag_creation_time', {
              placeholder1: updatedAt
                ? formatTimestampToString(updatedAt, 'YYYY-MM-DD HH:mm:ss')
                : '-',
            })}
          </span>
        </span>,
      );
    }
    if (logItem.operation === 'create') {
      desc.push(
        <span>
          {I18n.t('add')}
          <span className="font-medium leading-[22px] text-[var(--coz-fg-plus)]">
            {TAG_METADATA[logItem.target ?? ''] ?? logItem.target}
          </span>
          {logItem.target_value}
        </span>,
      );
    } else {
      // 添加具体的更新内容
      if (logItem.target) {
        const fieldName = logItem.target;
        const beforeValue = logItem.before_value || '-';
        const afterValue = logItem.after_value || '-';
        const isStatusChange =
          afterValue === 'active' || afterValue === 'inactive';
        desc.push(
          <span>
            <span>
              {I18n.t('tag_placeholder_update_start', {
                placeholder1: isStatusChange ? logItem.target_value : '',
              })}
            </span>
            <span className="font-medium leading-[22px] text-[var(--coz-fg-plus)]">
              {TAG_METADATA[fieldName] ?? fieldName}
            </span>
            <span>
              {I18n.t('tag_placeholder_update_end', {
                placeholder1: TAG_METADATA[beforeValue] ?? beforeValue,
                placeholder3: TAG_METADATA[afterValue] ?? afterValue,
              })}
            </span>
          </span>,
        );
      }
    }
    return desc;
  }, [] as React.ReactNode[]);
};

export const EditHistoryItem = (props: EditHistoryItemProps) => {
  const { updatedAt, updatedBy, changeLog } = props;
  const [isExpanded, setIsExpanded] = useState(false);
  const [showToggle, setShowToggle] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (contentRef.current) {
      const height = contentRef.current.scrollHeight;
      setShowToggle(height > CONTENT_MAX_HEIGHT);
    }
  }, [changeLog, updatedBy, updatedAt]);

  const description =
    generateDescFromChangeLog(changeLog ?? [], updatedBy?.name, updatedAt) ||
    '-';

  return (
    <Descriptions align="left" className={styles.description}>
      <Descriptions.Item itemKey={I18n.t('submit_time')}>
        <span className="font-medium leading-[22px] text-[13px] text-[var(--coz-fg-plus)]">
          {updatedAt ? formatTimestampToString(updatedAt) : '-'}
        </span>
      </Descriptions.Item>
      <Descriptions.Item itemKey={I18n.t('submit_user')}>
        <UserProfile name={updatedBy?.name} avatarUrl={updatedBy?.avatar_url} />
      </Descriptions.Item>
      <Descriptions.Item itemKey={I18n.t('change_log')}>
        <div className="relative">
          <div
            ref={contentRef}
            className="!text-[13px] text-[var(--coz-fg-primary)]"
            style={{
              wordBreak: 'break-word',
              maxHeight: isExpanded ? 'none' : `${CONTENT_MAX_HEIGHT}px`,
              overflow: isExpanded ? 'visible' : 'hidden',
              transition: 'max-height 0.3s ease',
            }}
          >
            {description}
          </div>
          {showToggle ? (
            <div className="w-full text-right">
              <Typography.Text
                onClick={() => setIsExpanded(!isExpanded)}
                className="text-brand-9 text-[13px] cursor-pointer text-right"
              >
                {isExpanded ? I18n.t('collapse') : I18n.t('extend')}
              </Typography.Text>
            </div>
          ) : null}
        </div>
      </Descriptions.Item>
    </Descriptions>
  );
};
