// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';
import { type OptionProps, Select } from '@coze-arch/coze-design';

interface PlatformSelectProps {
  onChange: (value: string | number) => void;
  value: string | number;
  optionList: OptionProps[];
  className?: string;
  disabled?: boolean;
}

export const PlatformSelect = ({
  onChange,
  value,
  optionList,
  className,
  disabled,
}: PlatformSelectProps) => (
  <Select
    className={cls('w-[144px] box-border', className)}
    value={value}
    optionList={optionList}
    disabled={disabled}
    onSelect={event => {
      onChange(event as string | number);
    }}
  />
);
