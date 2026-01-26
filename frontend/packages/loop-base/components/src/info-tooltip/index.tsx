// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import {
  IconCozInfoCircle,
  IconCozQuestionMarkCircle,
} from '@coze-arch/coze-design/icons';
import { Tooltip } from '@coze-arch/coze-design';

interface Props {
  content: string | React.ReactNode;
  className?: string;
  tooltipClassName?: string;
  icon?: React.ReactNode;
  useQuestion?: boolean;
}

export const InfoTooltip = ({
  content,
  className,
  tooltipClassName,
  icon,
  useQuestion = false,
}: Props) => {
  const defaultIcon = useQuestion ? (
    <IconCozQuestionMarkCircle className="coz-fg-secondary cursor-pointer hover:coz-fg-primary" />
  ) : (
    <IconCozInfoCircle className="coz-fg-secondary cursor-pointer hover:coz-fg-primary" />
  );
  return (
    <Tooltip
      content={
        <div style={{ maxHeight: 300, overflowY: 'auto' }}>{content}</div>
      }
      theme="dark"
      className={tooltipClassName}
    >
      <div className={classNames('h-[17px]', className)}>
        {icon ?? defaultIcon}
      </div>
    </Tooltip>
  );
};
