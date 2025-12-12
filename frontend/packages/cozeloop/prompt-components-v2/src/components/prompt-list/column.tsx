import { formatTimestampToString } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { TextWithCopy, UserProfile } from '@cozeloop/components';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import { type UserInfoDetail } from '@cozeloop/api-schema/foundation';
import { type ColumnProps, Tag, Typography } from '@coze-arch/coze-design';

export const promptDisplayColumns: ColumnProps<
  Prompt & { user?: UserInfoDetail; lastUpdateUser?: UserInfoDetail }
>[] = [
  {
    title: 'Prompt Key',
    dataIndex: 'prompt_key',
    width: 260,
    render: (key?: string, item?: Prompt) => (
      <div className="w-full flex items-center justify-start gap-1 overflow-hidden">
        <TextWithCopy
          content={key}
          className="overflow-hidden !text-[13px]"
          copyTooltipText={I18n.t('copy_prompt_key')}
          textType="primary"
        />

        {item?.prompt_draft?.draft_info?.is_modified ? (
          <Tag size="small" color="yellow" className="flex-shrink-0">
            {I18n.t('unsubmitted_changes')}
          </Tag>
        ) : null}
      </div>
    ),
  },
  {
    title: I18n.t('prompt_name'),
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
    title: I18n.t('prompt_description'),
    dataIndex: 'prompt_basic.description',
    width: 220,
    render: (text: string) => (
      <Typography.Text
        ellipsis={{
          showTooltip: {
            opts: {
              theme: 'dark',
              content: (
                <div className="w-full overflow-y-auto max-h-[400px]">
                  {text || '-'}
                </div>
              ),
            },
          },
        }}
        style={{
          fontSize: 'inherit',
        }}
      >
        {text || '-'}
      </Typography.Text>
    ),
  },
  {
    title: I18n.t('latest_version'),
    dataIndex: 'prompt_basic.latest_version',
    width: 140,
    render: (text: string) => (text ? <Tag color="primary">{text}</Tag> : '-'),
  },
  {
    title: I18n.t('prompt_latest_committer'),
    dataIndex: 'lastUpdateUser',
    width: 140,
    render: (user?: UserInfoDetail) =>
      user ? (
        <UserProfile avatarUrl={user?.avatar_url} name={user?.nick_name} />
      ) : (
        '-'
      ),
  },
  {
    title: I18n.t('recent_submission_time'),
    dataIndex: 'prompt_basic.latest_committed_at',
    width: 180,
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
    title: I18n.t('creator'),
    dataIndex: 'user',
    width: 140,
    render: (user?: UserInfoDetail) =>
      user ? (
        <UserProfile avatarUrl={user?.avatar_url} name={user?.nick_name} />
      ) : (
        '-'
      ),
  },
  {
    title: I18n.t('create_time'),
    dataIndex: 'prompt_basic.created_at',
    width: 180,
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
