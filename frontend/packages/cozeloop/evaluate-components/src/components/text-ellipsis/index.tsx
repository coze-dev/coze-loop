// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cs from 'classnames';
import { type Theme, Typography, type Ellipsis } from '@coze-arch/coze-design';
export const TextEllipsis = ({
  children,
  emptyText = '-',
  className,
  theme = 'dark',
}: {
  children: React.ReactNode;
  emptyText?: string;
  className?: string;
  theme?: 'dark' | 'light';
}) => (
  <Typography.Text
    ellipsis={{
      showTooltip: {
        opts: {
          theme,
        },
      },
    }}
    className={cs('!text-[13px] !coz-fg-plus text-left', className)}
  >
    {children || emptyText}
  </Typography.Text>
);

export function TypographyText({
  ellipsis = {},
  children,
  style = {},
  className,
  tooltipTheme,
}: {
  children: React.ReactNode;
  ellipsis?: Ellipsis;
  style?: React.CSSProperties;
  className?: string;
  tooltipTheme?: Theme;
}) {
  return (
    <Typography.Text
      className={className}
      style={{
        fontSize: 'inherit',
        color: 'inherit',
        fontWeight: 'inherit',
        lineHeight: 'inherit',
        ...style,
      }}
      ellipsis={{
        rows: 1,
        showTooltip: { opts: { theme: tooltipTheme ?? 'dark' } },
        ...ellipsis,
      }}
    >
      {children}
    </Typography.Text>
  );
}
