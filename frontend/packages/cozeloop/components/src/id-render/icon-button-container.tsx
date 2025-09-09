// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type DOMAttributes } from 'react';

import cls from 'classnames';

export default function IconButtonContainer({
  icon,
  className,
  style,
  onClick,
  active,
  disabled,
  ...rest
}: {
  icon: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  disabled?: boolean;
  active?: boolean;
} & DOMAttributes<HTMLDivElement>) {
  return (
    <div
      {...rest}
      onClick={onClick}
      style={style}
      className={cls(
        'inline-flex items-center justify-center shrink-0 w-5 h-5 rounded-[4px] text-sm text-[var(--coz-fg-secondary)] ',
        active && !disabled ? 'bg-[var(--coz-mg-plus)]' : '',
        disabled
          ? 'cursor-not-allowed'
          : 'cursor-pointer hover:text-[var(--coz-fg-primary)] hover:bg-[var(--coz-mg-plus)]',
        className,
      )}
    >
      {icon}
    </div>
  );
}
