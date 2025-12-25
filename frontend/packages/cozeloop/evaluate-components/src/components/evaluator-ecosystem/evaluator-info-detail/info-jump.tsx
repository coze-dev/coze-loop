// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { IconCozLongArrowTopRight } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';

interface InfoJumpProps {
  text: string;
  url: string;
  className?: string;
}

export function InfoJump({ text, url, className }: InfoJumpProps) {
  return (
    <div className={classNames('flex items-center', className)}>
      <span>{text}</span>
      <Button
        size="mini"
        style={{ width: '20px', height: '20px' }}
        color="secondary"
        icon={<IconCozLongArrowTopRight className="coz-fg-dim" />}
        onClick={e => {
          e.stopPropagation();
          window.open(url);
        }}
      />
    </div>
  );
}
