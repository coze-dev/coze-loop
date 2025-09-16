// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { IconCozIllusEmpty } from '@coze-arch/coze-design/illustrations';
import { type TableProps, EmptyState, Table } from '@coze-arch/coze-design';

import styles from './index.module.less';

export const LoopTable: React.FC<TableProps> = ({ className, ...props }) => (
  <Table
    empty={
      <EmptyState
        size="full_screen"
        icon={<IconCozIllusEmpty />}
        title="暂无数据"
      />
    }
    {...props}
    id={styles['loop-table']}
  />
);
