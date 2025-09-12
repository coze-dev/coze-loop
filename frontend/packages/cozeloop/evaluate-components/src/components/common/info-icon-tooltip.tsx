// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tooltip } from '@coze-arch/coze-design';

export function InfoIconTooltip({
  tooltip,
  className,
  zIndex,
}: {
  tooltip?: React.ReactNode;
  className?: string;
  zIndex?: number;
}) {
  return (
    <Tooltip content={tooltip} theme="dark" zIndex={zIndex}>
      <div className={classNames('flex items-center', className)}>
        <IconCozInfoCircle className="coz-fg-secondary cursor-pointer hover:coz-fg-primary" />
      </div>
    </Tooltip>
  );
}
