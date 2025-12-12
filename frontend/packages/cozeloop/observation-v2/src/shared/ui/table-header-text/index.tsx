import React from 'react';

import cls from 'classnames';
import { Typography } from '@coze-arch/coze-design';

interface TableHeaderText {
  className?: string;
  children: React.ReactElement | string;
  align?: 'left' | 'right';
}
export const TableHeaderText = ({
  children,
  className,
  align = 'left',
}: TableHeaderText) => (
  <div
    className={cls('max-w-full w-full flex items-center', className, {
      'justify-start': align === 'left',
      'justify-end': align === 'right',
    })}
  >
    <Typography.Text
      className={cls(className, 'max-w-full w-full')}
      ellipsis={{
        showTooltip: {
          opts: {
            theme: 'dark',
          },
        },
      }}
      style={{
        fontWeight: 'inherit',
        fontSize: 'inherit',
        color: 'inherit',
      }}
    >
      {children}
    </Typography.Text>
  </div>
);
