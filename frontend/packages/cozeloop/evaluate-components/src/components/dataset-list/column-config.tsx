// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { formatTimestampToString } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { type ColumnItem, UserProfile } from '@cozeloop/components';
import {
  type UserInfo,
  type EvaluationSet,
} from '@cozeloop/api-schema/evaluation';
import { Tag, type ColumnProps } from '@coze-arch/coze-design';

import { TextEllipsis } from '../text-ellipsis';
import LoopTableSortIcon from './sort-icon';
import { ColumnNameListTag } from './column-name-list-tag';

export type EvaluationSetKey =
  | 'name'
  | 'columns'
  | 'item_count'
  | 'latest_version'
  | 'update_at'
  | 'created_at'
  | 'description'
  | 'created_by'
  | 'updated_by';

export const DatasetColumnConfig: Record<
  EvaluationSetKey,
  ColumnProps<EvaluationSet>
> = {
  name: {
    title: I18n.t('name'),
    displayName: I18n.t('name'),
    key: 'name',
    disabled: true,
    dataIndex: 'name',
    width: 200,
    render: (text: string, record: EvaluationSet) => (
      <div className="flex items-center gap-1">
        <TextEllipsis>{text}</TextEllipsis>
        {record?.change_uncommitted ? (
          <Tag
            color="yellow"
            size="small"
            className="!min-w-[70px] !h-[20px] !px-[4px] !font-normal"
          >
            {I18n.t('changes_not_submitted')}
          </Tag>
        ) : null}
      </div>
    ),
  },
  description: {
    title: I18n.t('description'),
    displayName: I18n.t('description'),
    key: 'description',
    dataIndex: 'description',
    width: 170,
    render: text => <TextEllipsis>{text}</TextEllipsis>,
  },
  columns: {
    title: I18n.t('column_name'),
    displayName: I18n.t('column_name'),
    key: 'columns',
    width: 300,
    render: record => <ColumnNameListTag set={record} />,
  },
  item_count: {
    title: <div className="text-right">{I18n.t('data_item')}</div>,
    displayName: I18n.t('data_item'),
    key: 'item_count',
    dataIndex: 'item_count',
    width: 100,
    render: text => (
      <div className="text-right">
        <TextEllipsis>{text}</TextEllipsis>
      </div>
    ),
  },
  latest_version: {
    title: I18n.t('latest_version'),
    key: 'latest_version',
    displayName: I18n.t('latest_version'),
    dataIndex: 'latest_version',
    width: 100,
    render: text => (text ? <Tag color="primary">{text}</Tag> : '-'),
  },
  updated_by: {
    title: I18n.t('updated_person'),
    displayName: I18n.t('updated_person'),
    key: 'updated_by',
    dataIndex: 'base_info.updated_by',
    width: 180,
    render: (user?: UserInfo) =>
      user?.name ? (
        <UserProfile name={user?.name} avatarUrl={user?.avatar_url} />
      ) : (
        '-'
      ),
  },
  update_at: {
    title: I18n.t('update_time'),
    key: 'updated_at',
    displayName: I18n.t('update_time'),
    width: 180,
    dataIndex: 'base_info.updated_at',
    sorter: true,
    sortIcon: LoopTableSortIcon,
    render: (record: string) =>
      record ? (
        <TextEllipsis>
          {formatTimestampToString(record, 'YYYY-MM-DD HH:mm:ss')}
        </TextEllipsis>
      ) : (
        '-'
      ),
  },
  created_by: {
    title: I18n.t('creator'),
    displayName: I18n.t('creator'),

    key: 'created_by',
    dataIndex: 'base_info.created_by',
    width: 180,
    render: (user?: UserInfo) =>
      user?.name ? (
        <UserProfile name={user?.name} avatarUrl={user?.avatar_url} />
      ) : (
        '-'
      ),
  },
  created_at: {
    title: I18n.t('create_time'),
    displayName: I18n.t('create_time'),
    key: 'created_at',
    width: 180,
    render: (record?: EvaluationSet) =>
      record?.base_info?.created_at ? (
        <TextEllipsis>
          {formatTimestampToString(
            record?.base_info?.created_at,
            'YYYY-MM-DD HH:mm:ss',
          )}
        </TextEllipsis>
      ) : (
        '-'
      ),
  },
};
const DefaultColumnConfig: EvaluationSetKey[] = [
  'name',
  'columns',
  'item_count',
  'latest_version',
  'description',
  'updated_by',
  'update_at',
  'created_by',
  'created_at',
];
export const getColumnConfigs = (columns?: EvaluationSetKey[]): ColumnItem[] =>
  (columns || DefaultColumnConfig).map(column => ({
    ...DatasetColumnConfig[column],
    key: DatasetColumnConfig[column]?.key as string,
    value: DatasetColumnConfig[column]?.displayName as string,
    disabled: DatasetColumnConfig[column]?.disabled || false,
    checked: true,
  }));
