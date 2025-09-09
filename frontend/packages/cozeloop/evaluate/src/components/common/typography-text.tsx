// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Theme, Typography, type Ellipsis } from '@coze-arch/coze-design';

export default function TypographyText({
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
