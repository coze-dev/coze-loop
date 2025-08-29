import { type TooltipProps } from '@coze-arch/coze-design';
import { Tooltip } from '@coze-arch/coze-design';

export interface TooltipWhenDisabledProps extends TooltipProps {
  disabled?: boolean;
}

export function TooltipWhenDisabled({
  children,
  disabled,
  ...rest
}: TooltipWhenDisabledProps) {
  if (disabled) {
    return <Tooltip {...rest}>{children}</Tooltip>;
  }
  return <>{children}</>;
}
