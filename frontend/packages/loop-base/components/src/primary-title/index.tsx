// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import classNames from 'classnames';
import { Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

interface PrimaryTitleProps {
  title: string;
  className?: string;
}

export function PrimaryTitle({ title, className }: PrimaryTitleProps) {
  return (
    <Typography.Text className={classNames(styles['primary-title'], className)}>
      {title}
    </Typography.Text>
  );
}
