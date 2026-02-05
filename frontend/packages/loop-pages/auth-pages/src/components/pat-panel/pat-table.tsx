// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type PersonalAccessToken } from '@cozeloop/api-schema/foundation';
import { Empty, Table } from '@coze-arch/coze-design';

import { getColumns } from './columns';

import s from './pat-table.module.less';

interface Props {
  loading?: boolean;
  dataSource?: PersonalAccessToken[];
  onEdit: (v: PersonalAccessToken) => void;
  onDelete: (id: string) => void;
}

export function PatTable({ loading, dataSource, onEdit, onDelete }: Props) {
  const tableRef = useRef<HTMLDivElement>(null);
  const columns = getColumns({ onEdit, onDelete });

  return (
    <div className={s.container} ref={tableRef}>
      <Table
        useHoverStyle={false}
        tableProps={{
          rowKey: 'id',
          loading,
          dataSource,
          columns,
          sticky: true,
          scroll: {},
        }}
        empty={
          <Empty title={I18n.t('no_pat')} description={I18n.t('add_pat')} />
        }
      />
    </div>
  );
}
