// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { formatTimestampToString } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { TableWithPagination, UserProfile } from '@cozeloop/components';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { type tag, type OrderBy } from '@cozeloop/api-schema/data';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState, type ColumnProps } from '@coze-arch/coze-design';

import { type useSearchTags } from '@/hooks/use-search-tags';
import { TAG_TYPE_TO_NAME_MAP } from '@/const';

import { TagStatusSwitch } from './tag-table-cell/tag-status-switch';
import { TableCellText } from './tag-table-cell/table-cell-text';
import { Operator } from './tag-table-cell/operator';

type TagInfo = tag.TagInfo;

interface TagsListTableProps {
  service: ReturnType<typeof useSearchTags>['service'];
  setOrderBy?: React.Dispatch<React.SetStateAction<OrderBy>>;
  /**
   * 标签列表路由路径，用于跳转和拼接 标签详情 / 创建标签 路由路径
   */
  tagListPagePath?: string;
}

export const TagsListTable = ({
  service,
  setOrderBy,
  tagListPagePath,
}: TagsListTableProps) => {
  const navigate = useNavigateModule();

  const columns: ColumnProps<TagInfo>[] = [
    {
      title: I18n.t('tag_name'),
      dataIndex: 'name',
      render: (_: string, record) => (
        <TableCellText text={record.tag_key_name ?? '-'} />
      ),
    },
    {
      title: I18n.t('type'),
      dataIndex: 'type',
      width: 95,
      render: (_: string, record) => (
        <div className="font-semibold leading-[16px] text-[12px] text-[var(--coz-fg-primary)] py-[1px] px-[3px] rounded-[4px] bg-[var(--coz-mg-primary)] inline-block">
          {TAG_TYPE_TO_NAME_MAP[record.content_type ?? ''] ?? '-'}
        </div>
      ),
    },
    {
      title: I18n.t('description'),
      dataIndex: 'description',
      width: 280,
      render: (_: string, record) => (
        <TableCellText text={record.description ?? '-'} />
      ),
    },
    {
      title: I18n.t('creator'),
      dataIndex: 'creator',
      width: 170,
      render: (_: string, record) => (
        <UserProfile
          name={record.base_info?.created_by?.name ?? '-'}
          avatarUrl={record.base_info?.created_by?.avatar_url ?? ''}
        />
      ),
    },
    {
      title: I18n.t('create_time'),
      dataIndex: 'createTime',
      sorter: true,
      width: 178,
      render: (_: string, record) => (
        <TableCellText
          text={
            record.base_info?.created_at
              ? formatTimestampToString(record.base_info.created_at)
              : '-'
          }
        />
      ),
    },
    {
      title: I18n.t('enable'),
      dataIndex: 'enable',
      width: 68,
      render: (_: string, record) => <TagStatusSwitch tagInfo={record} />,
    },
    {
      title: I18n.t('operate'),
      dataIndex: 'operator',
      width: 68,
      render: (_: string, record) => (
        <Operator tagInfo={record} tagListPagePath={tagListPagePath} />
      ),
    },
  ];

  return (
    <TableWithPagination<TagInfo>
      service={service}
      heightFull
      tableProps={{
        columns,
        sticky: { top: 0 },
        rowKey: 'tag_key_id',
        onRow: record => ({
          onClick: () => {
            navigate(`${tagListPagePath}/${record.tag_key_id}`);
          },
        }),
        onChange: data => {
          setOrderBy?.(
            data.sorter?.sortOrder === false
              ? {}
              : {
                  // 这里setOrderBy的时候，处理其中的field参数赋值时，将赋值的data.sorter?.key参数，做一次转化，将小驼峰转化为下划线连接命名。例如'updateAt'转化为'update_at'
                  field: data.sorter?.key
                    ? data.sorter.key
                        .replace(/([a-z])([A-Z])/g, '$1_$2')
                        .toLowerCase()
                    : undefined,
                  is_asc: data.sorter?.sortOrder === 'ascend',
                },
          );
        },
      }}
      empty={
        <EmptyState
          size="full_screen"
          icon={<IconCozIllusAdd />}
          title={I18n.t('no_tags_available')}
          description={I18n.t('click_create_button')}
        />
      }
    />
  );
};
