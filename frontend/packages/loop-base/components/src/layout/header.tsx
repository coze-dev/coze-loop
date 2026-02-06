// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { type ReactNode } from 'react';

import classNames from 'classnames';

interface Props {
  children?: ReactNode;
  className?: string;
}
export function Header({ children, className }: Props) {
  return (
    <div className={classNames('flex items-center pb-4', className)}>
      {children}
    </div>
  );
}
