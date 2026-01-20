// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Typography } from '@coze-arch/coze-design';

export const FormSectionLayout = ({
  children,
  title,
  className,
}: {
  children: React.ReactNode;
  title: React.ReactNode;
  className?: string;
}) => (
  <div className="flex flex-col ">
    <Typography.Title heading={6} className={className}>
      {title}
    </Typography.Title>
    {children}
  </div>
);
