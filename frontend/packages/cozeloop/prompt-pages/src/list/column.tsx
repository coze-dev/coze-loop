// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { formatTimestampToString } from '@cozeloop/toolkit';
import { TextWithCopy, UserProfile } from '@cozeloop/components';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { type UserInfoDetail } from '@cozeloop/api-schema/foundation';
import { type ColumnProps, Tag, Typography } from '@coze-arch/coze-design';

export const columns: ColumnProps<Prompt & { user?: UserInfoDetail }>[] = [
  {
    title: 'Prompt Key',
    dataIndex: 'prompt_key',
    width: 260,
    render: (key?: string, item?: Prompt) => (
      <div className="w-full flex items-center justify-start gap-1 overflow-hidden">
        <TextWithCopy
          content={key}
          className="overflow-hidden !text-[13px]"
          copyTooltipText="复制 Prompt Key"
          textType="primary"
        />
        {item?.prompt_draft?.draft_info?.is_modified ? (
          <Tag size="small" color="yellow" className="flex-shrink-0">
            修改未提交
          </Tag>
        ) : null}
      </div>
    ),
  },
  {
    title: 'Prompt 名称',
    dataIndex: 'prompt_basic.display_name',
    width: 200,
    render: (text: string) => (
      <Typography.Text
        ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
        style={{
          fontSize: 'inherit',
        }}
      >
        {text}
      </Typography.Text>
    ),
  },
  {
    title: 'Prompt 描述',
    dataIndex: 'prompt_basic.description',
    width: 220,
    render: (text: string) => (
      <Typography.Text
        ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
        style={{
          fontSize: 'inherit',
        }}
      >
        {text || '-'}
      </Typography.Text>
    ),
  },
  {
    title: '最新版本',
    dataIndex: 'prompt_basic.latest_version',
    width: 140,
    render: (text: string) => (text ? <Tag color="primary">{text}</Tag> : '-'),
  },
  {
    title: '最近提交时间',
    dataIndex: 'prompt_basic.latest_committed_at',
    width: 200,
    render: (text: string) => (
      <Typography.Text
        style={{
          fontSize: 'inherit',
        }}
      >
        {text ? formatTimestampToString(text) : '-'}
      </Typography.Text>
    ),
    sorter: true,
  },
  {
    title: '创建人',
    dataIndex: 'user',
    width: 140,
    render: (user?: UserInfoDetail) => (
      <UserProfile avatarUrl={user?.avatar_url} name={user?.nick_name} />
    ),
  },
  {
    title: '创建时间',
    dataIndex: 'prompt_basic.created_at',
    width: 200,
    render: (text: string) => (
      <Typography.Text
        style={{
          fontSize: 'inherit',
        }}
      >
        {text ? formatTimestampToString(text) : '-'}
      </Typography.Text>
    ),
    sorter: true,
  },
];
