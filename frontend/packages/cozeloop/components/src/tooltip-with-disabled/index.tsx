// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type TooltipProps } from '@coze-arch/coze-design';
import { Tooltip } from '@coze-arch/coze-design';

export interface TooltipWithDisabledProps extends TooltipProps {
  disabled?: boolean;
}

export function TooltipWithDisabled({
  children,
  disabled,
  ...rest
}: TooltipWithDisabledProps) {
  if (disabled) {
    return <>{children}</>;
  }
  return <Tooltip {...rest}>{children}</Tooltip>;
}
