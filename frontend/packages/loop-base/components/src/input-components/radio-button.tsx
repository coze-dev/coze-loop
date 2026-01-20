// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';
import { Tooltip } from '@coze-arch/coze-design';

export interface RadioButtonProps<T extends string | number> {
  value?: T;
  onChange?: (value: T) => void;
  disabled?: boolean;
  options: Array<{
    value: T;
    label: React.ReactNode;
    disabled?: boolean;
    tooltip?: string;
  }>;
  className?: string;
}

/** coze design 的 SegmentTab 和 Radio 的 advanced + pureCard 模式均有样式问题，不得不自行造轮子 */
export function RadioButton<T extends string | number>({
  value,
  onChange,
  options,
  disabled,
  className,
}: RadioButtonProps<T>) {
  return (
    <div className={cls('w-full flex items-center gap-[8px]', className)}>
      {options.map(option => {
        const selected = value === option.value;
        const optionDisabled = option.disabled || disabled;
        return (
          <Tooltip
            key={option.value}
            trigger={option.tooltip ? 'hover' : 'custom'}
            content={option.tooltip}
          >
            <div
              className={cls(
                'flex-1 px-[20px] py-[4px] rounded-[6px]',
                'flex items-center justify-center',
                'border border-solid',
                'text-[14px] leading-[20px]',
                selected
                  ? 'coz-fg-hglt coz-stroke-hglt coz-mg-hglt hover:coz-mg-hglt-hovered active:coz-mg-hglt-pressed'
                  : 'coz-stroke-primary coz-bg-max hover:coz-mg-primary-hovered active:coz-mg-primary-pressed',
                optionDisabled
                  ? 'cursor-not-allowed opacity-60'
                  : 'cursor-pointer',
              )}
              onClick={() => {
                if (optionDisabled) {
                  return;
                }
                onChange?.(option.value);
              }}
            >
              {option.label}
            </div>
          </Tooltip>
        );
      })}
    </div>
  );
}
