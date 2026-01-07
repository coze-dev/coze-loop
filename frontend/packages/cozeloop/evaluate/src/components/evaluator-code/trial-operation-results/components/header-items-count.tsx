// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { Divider, Tag } from '@coze-arch/coze-design';

const HeaderItemsCount = ({
  totalCount,
  successCount,
  failedCount,
}: {
  totalCount: number;
  successCount: number;
  failedCount: number;
}) => (
  <Tag color="primary" size="small" className="ml-2">
    总条数 {totalCount || 0}
    <Divider
      layout="vertical"
      style={{ marginLeft: 8, marginRight: 8, height: 12 }}
    />
    成功 {successCount}
    <Divider
      layout="vertical"
      style={{ marginLeft: 8, marginRight: 8, height: 12 }}
    />
    失败 {failedCount}
  </Tag>
);

export default HeaderItemsCount;
