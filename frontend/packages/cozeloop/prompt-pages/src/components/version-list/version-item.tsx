/* eslint-disable complexity */
import cs from 'classnames';
import { formatTimestampToString } from '@cozeloop/toolkit';
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

import styles from './index.module.less';

export default function VersionItem({
  version,
  active,
  labels,
  className,
  onClick,
  onEditLabels,
}: {
  version?: CommitInfo & { user?: UserInfoDetail };
  active?: boolean;
  labels?: Label[];
  className?: string;
  onClick?: () => void;
  onEditLabels?: (labels: Label[]) => void;
}) {
  const isDraft = !version?.version;

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
            {isDraft ? '当前草稿' : '提交'}
          </Tag>
          {isDraft ? null : (
            <Descriptions.Item itemKey="版本">
              <span className="font-medium">{version.version ?? '-'}</span>
            </Descriptions.Item>
          )}
          {!version?.committed_at ? null : (
            <Descriptions.Item
              itemKey={isDraft ? '保存时间' : '提交时间'}
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
            <Descriptions.Item itemKey="提交人" className="!text-[13px]">
              <UserProfile
                avatarUrl={version?.user?.avatar_url}
                name={version?.user?.nick_name}
              />
            </Descriptions.Item>
          )}
          {isDraft ? null : (
            <Descriptions.Item itemKey="版本说明" className="!text-[13px]">
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
          {!isDraft ? (
            <Descriptions.Item itemKey="版本标识" className="!text-[13px]">
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
        </Descriptions>
      </div>
    </div>
  );
}
