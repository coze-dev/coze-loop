// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozLoose, IconCozTight } from '@coze-arch/coze-design/icons';
import { Radio, Tooltip } from '@coze-arch/coze-design';

export default function TableCellExpand({
  expand,
  onChange,
  className,
}: {
  expand?: boolean;
  onChange?: (expand: boolean) => void;
  className?: string;
}) {
  return (
    <Radio.Group
      type="button"
      className={`${className} !gap-0`}
      buttonSize="middle"
      value={expand ? 'expand' : 'shrink'}
      onChange={e => onChange?.(e.target.value === 'expand' ? true : false)}
    >
      <Tooltip content={I18n.t('compact_view')} theme="dark">
        <Radio
          value="shrink"
          addonClassName="flex items-center"
          addonStyle={{ padding: '4px 6px' }}
        >
          <IconCozTight className="text-xxl" />
        </Radio>
      </Tooltip>
      <Tooltip content={I18n.t('loose_view')} theme="dark">
        <Radio
          value="expand"
          addonClassName="flex items-center"
          addonStyle={{ padding: '4px 6px' }}
        >
          <IconCozLoose className="text-xxl" />
        </Radio>
      </Tooltip>
    </Radio.Group>
  );
}
