// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type TooltipProps } from '@coze-arch/coze-design';
import { Tooltip } from '@coze-arch/coze-design';

export interface TooltipWhenDisabledProps extends TooltipProps {
  disabled?: boolean;
  needWrap?: boolean;
}

export function TooltipWhenDisabled({
  children,
  disabled,
  needWrap,
  ...rest
}: TooltipWhenDisabledProps) {
  if (disabled) {
    return (
      <Tooltip {...rest}>
        {needWrap ? <span>{children}</span> : children}
      </Tooltip>
    );
  }
  return <>{children}</>;
}
