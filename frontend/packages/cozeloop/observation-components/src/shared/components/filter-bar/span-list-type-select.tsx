// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import cls from 'classnames';
import { SpanListType } from '@cozeloop/api-schema/observation';
import { type OptionProps, Select, Tooltip } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';

interface SpanTypeSelectProps {
  onChange: (value: string | number) => void;
  value: string | number;
  optionList: OptionProps[];
  className?: string;
  layoutMode?: 'horizontal' | 'vertical';
  disabled?: boolean;
}

export const SpanTypeSelect = ({
  onChange,
  value,
  optionList,
  className,
  layoutMode = 'horizontal',
  disabled,
}: SpanTypeSelectProps) => {
  const { t } = useLocale();
  const TOOLTIP_CONTENT = {
    [SpanListType.AllSpan]: t('all_span_tip'),
    [SpanListType.RootSpan]: t('root_span_tip'),
    [SpanListType.LlmSpan]: t('llm_span_tip'),
  };
  const [showTooltip, setShowTooltip] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);

  if (layoutMode === 'vertical') {
    return (
      <Select
        disabled={disabled}
        onMouseEnter={() => setShowTooltip(true)}
        onMouseLeave={() => setShowTooltip(false)}
        className={cls('w-[144px] box-border', className)}
        value={value}
        onDropdownVisibleChange={setShowDropdown}
        optionList={optionList}
        onSelect={event => {
          onChange(event as string);
        }}
      />
    );
  }

  return (
    <Tooltip
      content={TOOLTIP_CONTENT?.[value]}
      theme="dark"
      visible={showDropdown ? false : showTooltip}
      trigger="custom"
    >
      <Select
        disabled={disabled}
        onMouseEnter={() => setShowTooltip(true)}
        onMouseLeave={() => setShowTooltip(false)}
        className={cls('w-[144px] box-border', className)}
        value={value}
        onDropdownVisibleChange={setShowDropdown}
        optionList={optionList}
        onSelect={event => {
          onChange(event as string);
        }}
      />
    </Tooltip>
  );
};
