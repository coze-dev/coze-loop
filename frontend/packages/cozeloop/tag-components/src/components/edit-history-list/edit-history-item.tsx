/* eslint-disable complexity */
import { useState, useRef, useEffect } from 'react';

import { formatTimestampToString } from '@cozeloop/toolkit';
import { UserProfile } from '@cozeloop/components';
import { type common } from '@cozeloop/api-schema/evaluation';
import { type tag } from '@cozeloop/api-schema/data';
import { Descriptions, Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

const TAG_METADATA = {
  tag_name: '标签名称',
  tag_value_name: '标签选项',
  tag_description: '标签描述',
  tag_value_status: '标签选项启用状态',
  inactive: '禁用',
  active: '启用',
  tag_status: '标签状态',
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
          <span>创建人:@{updatedBy || '-'}</span>,
          <span>
            创建时间:
            {updatedAt
              ? formatTimestampToString(updatedAt, 'YYYY-MM-DD HH:mm:ss')
              : '-'}
          </span>
        </span>,
      );
    }
    if (logItem.operation === 'create') {
      desc.push(
        <span>
          新增
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
            <span>将标签{isStatusChange ? logItem.target_value : ''}的</span>
            <span className="font-medium leading-[22px] text-[var(--coz-fg-plus)]">
              {TAG_METADATA[fieldName] ?? fieldName}
            </span>
            <span>
              从{TAG_METADATA[beforeValue] ?? beforeValue}更新为
              {TAG_METADATA[afterValue] ?? afterValue}。
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
      <Descriptions.Item itemKey="提交时间">
        <span className="font-medium leading-[22px] text-[13px] text-[var(--coz-fg-plus)]">
          {updatedAt ? formatTimestampToString(updatedAt) : '-'}
        </span>
      </Descriptions.Item>
      <Descriptions.Item itemKey="提交人">
        <UserProfile name={updatedBy?.name} avatarUrl={updatedBy?.avatar_url} />
      </Descriptions.Item>
      <Descriptions.Item itemKey="修改记录">
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
                {isExpanded ? '收起' : '展开'}
              </Typography.Text>
            </div>
          ) : null}
        </div>
      </Descriptions.Item>
    </Descriptions>
  );
};
