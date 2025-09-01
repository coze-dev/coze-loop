import { formatTimestampToString } from '@cozeloop/toolkit';
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
    title: '名称',
    displayName: '名称',
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
            修改未提交
          </Tag>
        ) : null}
      </div>
    ),
  },
  description: {
    title: '描述',
    displayName: '描述',
    key: 'description',
    dataIndex: 'description',
    width: 170,
    render: text => <TextEllipsis>{text}</TextEllipsis>,
  },
  columns: {
    title: '列名',
    displayName: '列名',
    key: 'columns',
    width: 300,
    render: record => <ColumnNameListTag set={record} />,
  },
  item_count: {
    title: <div className="text-right">数据项</div>,
    displayName: '数据项',
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
    title: '最新版本',
    key: 'latest_version',
    displayName: '最新版本',
    dataIndex: 'latest_version',
    width: 100,
    render: text => (text ? <Tag color="primary">{text}</Tag> : '-'),
  },
  updated_by: {
    title: '更新人',
    displayName: '更新人',
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
    title: '更新时间',
    key: 'updated_at',
    displayName: '更新时间',
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
    title: '创建人',
    displayName: '创建人',

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
    title: '创建时间',
    displayName: '创建时间',
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
