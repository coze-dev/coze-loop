// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import { Button, Tooltip } from '@coze-arch/coze-design';

export function RefreshButton({
  onRefresh,
}: {
  onRefresh: (() => void) | undefined;
}) {
  return (
    <Tooltip content={I18n.t('refresh')} theme="dark">
      <Button
        color="primary"
        icon={<IconCozRefresh />}
        onClick={() => onRefresh?.()}
      />
    </Tooltip>
  );
}
