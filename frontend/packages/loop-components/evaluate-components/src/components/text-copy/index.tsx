// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import cn from 'classnames';
import { handleCopy as copy } from '@cozeloop/components';
import { IconCozCopy, IconCozCheckMark } from '@coze-arch/coze-design/icons';
import { Button } from '@coze-arch/coze-design';
export const TextCopy = ({
  content,
  className,
}: {
  content: string;
  className?: string;
}) => {
  const [copying, setCopying] = useState(false);
  const handleCopy = () => {
    copy(content);
    navigator.clipboard.writeText(content);
    setTimeout(() => {
      setCopying(false);
    }, 3000);
  };
  return !copying ? (
    <Button
      icon={<IconCozCopy />}
      onClick={handleCopy}
      color="secondary"
      size="small"
      className={cn(
        'cursor-pointer !w-[20px] !min-w-[20px] !h-[20px] !p-0 !rounded-[4px] !text-[14px]',
        className,
      )}
    />
  ) : (
    <IconCozCheckMark />
  );
};
