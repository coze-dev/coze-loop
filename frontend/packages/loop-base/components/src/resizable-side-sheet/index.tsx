// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useEffect, useRef, useState } from 'react';

import classNames from 'classnames';
import { useMouseDownOffset } from '@cozeloop/hooks';
import { type SideSheetReactProps, SideSheet } from '@coze-arch/coze-design';

interface ResizableSideSheetProps extends SideSheetReactProps {
  minSize?: number;
  maxSize?: number;
  defaultSize?: number;
  onResizeEnd?: (size: number) => void;
}

const SIZE_MAP = {
  small: 448,
  medium: 684,
  large: 920,
};
/** 仅支持 left 模式 */
export const ResizableSideSheet = (props: ResizableSideSheetProps) => {
  const {
    minSize = Number.MIN_VALUE,
    defaultSize,
    maxSize = Number.MAX_VALUE,
    size = 'small',
    onResizeEnd,
    className,
    children,
    ...restProps
  } = props;

  const [sidePaneWidth, setSidePaneWidth] = useState(
    defaultSize ?? SIZE_MAP[size],
  );
  const prevWidthRef = useRef(sidePaneWidth);

  const { ref, isActive } = useMouseDownOffset(({ offsetX }) => {
    const newWidth = prevWidthRef.current - offsetX;
    setSidePaneWidth([maxSize, newWidth, minSize].sort((a, b) => a - b)[1]);
  });

  useEffect(() => {
    prevWidthRef.current = sidePaneWidth;
    document.body.style.cursor = isActive ? 'col-resize' : '';
    document.body.style.userSelect = isActive ? 'none' : 'auto';
    if (!isActive) {
      onResizeEnd?.(sidePaneWidth);
    }
  }, [isActive]);
  return (
    <SideSheet
      className={classNames('relative', className)}
      size={size}
      width={sidePaneWidth}
      {...restProps}
    >
      <div
        ref={ref}
        className={classNames(
          'absolute h-full w-1 bg-white top-0 left-0 hover:cursor-col-resize hover:bg-blue-400 transition',
          {
            'bg-blue-400 cursor-col-resize': isActive,
          },
        )}
      />
      {children}
    </SideSheet>
  );
};
