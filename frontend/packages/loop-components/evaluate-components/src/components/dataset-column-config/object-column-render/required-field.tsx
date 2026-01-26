// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Select } from '@coze-arch/coze-design';

interface RequiredFieldProps {
  value: boolean;
  onChange?: (value: boolean) => void;
  disabled?: boolean;
  className?: string;
}

export const RequiredField = ({
  value,
  onChange,
  disabled,
  className,
}: RequiredFieldProps) => (
  <Select
    disabled={disabled}
    className={className}
    value={value === true ? 'true' : 'false'}
    optionList={[
      { label: I18n.t('yes'), value: 'true' },
      { label: I18n.t('no'), value: 'false' },
    ]}
    onChange={newValue => {
      onChange?.(newValue === 'true');
    }}
  ></Select>
);
