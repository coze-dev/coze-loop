// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { IconCozIllusEmpty } from '@coze-arch/coze-design/illustrations';
import { type TableProps, EmptyState, Table } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import styles from './index.module.less';

export const LoopTable: React.FC<TableProps> = ({ className, ...props }) => {
  const I18n = useI18n();
  return (
    <Table
      empty={
        <EmptyState
          size="full_screen"
          icon={<IconCozIllusEmpty />}
          title={I18n.t('no_data')}
        />
      }
      {...props}
      id={styles['loop-table']}
    />
  );
};
