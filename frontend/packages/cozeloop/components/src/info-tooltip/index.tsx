// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tooltip } from '@coze-arch/coze-design';

interface Props {
  content: string;
  className?: string;
}

export const InfoTooltip = ({ content, className }: Props) => (
  <Tooltip content={content} theme="dark">
    <div className={classNames('h-[17px]', className)}>
      <IconCozInfoCircle className="coz-fg-secondary cursor-pointer hover:coz-fg-primary" />
    </div>
  </Tooltip>
);
