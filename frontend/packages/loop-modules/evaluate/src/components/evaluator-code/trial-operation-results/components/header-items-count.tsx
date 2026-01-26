// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
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
    {I18n.t('total_number')} {totalCount || 0}
    <Divider
      layout="vertical"
      style={{ marginLeft: 8, marginRight: 8, height: 12 }}
    />
    {I18n.t('success')} {successCount}
    <Divider
      layout="vertical"
      style={{ marginLeft: 8, marginRight: 8, height: 12 }}
    />
    {I18n.t('failure')} {failedCount}
  </Tag>
);

export default HeaderItemsCount;
