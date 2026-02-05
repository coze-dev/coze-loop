// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { type PersonalAccessToken } from '@cozeloop/api-schema/foundation';
import { Tag, type ColumnProps } from '@coze-arch/coze-design';

import { getDetailTime, getExpirationTime, getStatus } from './utils';
import { PatOperation } from './pat-op';

interface ColumnAction {
  onEdit: (v: PersonalAccessToken) => void;
  onDelete: (id: string) => void;
}

export function getColumns({
  onEdit,
  onDelete,
}: ColumnAction): ColumnProps<PersonalAccessToken>[] {
  return [
    {
      title: I18n.t('name'),
      dataIndex: 'name',
      width: 120,
      render: (name: string) => <span className="break-all">{name}</span>,
    },
    {
      title: I18n.t('create_time'),
      dataIndex: 'created_at',
      render: (createTime: number) => getDetailTime(createTime),
    },
    {
      title: I18n.t('last_used'),
      dataIndex: 'last_used_at',
      render: (lastUseTime: number) => getDetailTime(lastUseTime),
    },
    {
      title: I18n.t('expiration_time'), // 状态
      dataIndex: 'expire_at',
      render: (expireTime: number) => getExpirationTime(expireTime),
    },
    {
      title: I18n.t('status'),
      dataIndex: 'id',
      width: 80,
      render: (_: string, record: PersonalAccessToken) => {
        const isActive = getStatus(record?.expire_at);
        return (
          <Tag size="small" color={isActive ? 'green' : 'grey'}>
            {isActive ? I18n.t('active') : I18n.t('expired')}
          </Tag>
        );
      },
    },
    {
      title: I18n.t('operation'),
      width: 120,
      render: (_, record) => (
        <PatOperation pat={record} onDelete={onDelete} onEdit={onEdit} />
      ),
    },
  ];
}
