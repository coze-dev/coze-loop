// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type MouseEvent } from 'react';

import classNames from 'classnames';
import { IconCozPencil } from '@coze-arch/coze-design/icons';

interface Props {
  className?: string;
  disabled?: boolean;
  onClick?: (e: MouseEvent<SVGElement>) => void;
}

export function EditIconButton({ disabled, className, onClick }: Props) {
  return (
    <IconCozPencil
      fontSize={14}
      className={classNames(
        'text-[var(--coz-fg-dim)]',
        disabled
          ? 'cursor-not-allowed'
          : 'cursor-pointer hover:text-[rgba(var(--coze-up-brand-9))]',
        className,
      )}
      onClick={e => {
        if (!disabled) {
          onClick?.(e);
        }
      }}
    />
  );
}
