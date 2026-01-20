// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { forwardRef, type CSSProperties, type ReactNode } from 'react';

import classNames from 'classnames';

export const Content = forwardRef<
  HTMLDivElement,
  {
    children?: ReactNode;
    style?: CSSProperties;
    className?: string;
    top?: ReactNode;
  }
>(({ children, style, className, top }, ref) => (
  <div className="w-full h-full flex flex-col">
    {top}
    <div
      ref={ref}
      className={classNames(
        'bg-white m-4 mb-0 p-4 rounded-md flex-1 flex flex-col overflow-y-auto',
        className,
      )}
      style={style}
    >
      {children}
    </div>
    {/* <div className="h-4 shrink-0" /> */}
  </div>
));
