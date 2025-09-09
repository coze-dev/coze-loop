// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cs from 'classnames';
import { Tag, type TagProps } from '@coze-arch/coze-design';

export const LoopTag = ({ children, ...rest }: TagProps) => (
  <Tag
    {...rest}
    className={cs(
      '!rounded-[3px] !px-[8px] !font-normal !h-[20px]',
      rest.className,
    )}
  >
    {children}
  </Tag>
);
