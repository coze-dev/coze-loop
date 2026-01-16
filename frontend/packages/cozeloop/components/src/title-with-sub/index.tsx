// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { Space, Typography } from '@coze-arch/coze-design';

interface TitleWithSubProps {
  title: string;
  subTitle?: string;
  className?: string;
}

export function TitleWithSub({
  title,
  subTitle,
  className,
}: TitleWithSubProps) {
  return (
    <Space align="center" spacing="tight" className={className}>
      <Typography.Title className="!text-sm" heading={6}>
        {title}
      </Typography.Title>
      {subTitle ? (
        <Typography.Text size="small" type="tertiary">
          {subTitle}
        </Typography.Text>
      ) : null}
    </Space>
  );
}
