// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useState, type ReactNode, useEffect } from 'react';

import classNames from 'classnames';
import { useMouseDownOffset } from '@cozeloop/hooks';

import styles from './index.module.less';

type PaneType = ReactNode;

interface SplitPaneProps {
  left: PaneType;
  right: PaneType;
  target?: 'left' | 'right';
  defaultWidth: number;
  maxWidth: number;
  minWidth: number;
  className?: string;
}
export const SplitPane = ({
  left,
  right,
  defaultWidth,
  target = 'left',
  maxWidth,
  minWidth,
  className,
}: SplitPaneProps) => {
  const [width, setWidth] = useState(defaultWidth);
  const [prevWidth, setPrevWidth] = useState<number>(defaultWidth);
  const { ref, isActive } = useMouseDownOffset(({ offsetX }) => {
    const newWidth =
      target === 'left' ? prevWidth + offsetX : prevWidth - offsetX;
    setWidth([minWidth, newWidth, maxWidth].sort((a, b) => a - b)[1]);
  });

  useEffect(() => {
    setPrevWidth(width);
    document.body.style.cursor = isActive ? 'col-resize' : '';
    document.body.style.userSelect = isActive ? 'none' : 'auto';
  }, [isActive]);

  return (
    <div className={classNames('flex w-full h-full', className)}>
      <div
        className={classNames(
          'analytics-content-box bg-white h-full overflow-hidden',
          styles['split-pane_container'],
          {
            'pointer-events-none select-none': isActive,
          },
        )}
        style={target === 'left' ? { width } : { flex: 1 }}
      >
        {left}
      </div>
      <div
        className="cursor-col-resize w-0.5 box-border mx-[3px] my-2.5 transition hover:bg-[#336df4]"
        style={{ background: isActive ? '#336df4' : undefined }}
        ref={ref}
      ></div>
      <div
        className={classNames(
          'analytics-content-box bg-white h-full overflow-hidden',
          styles['split-pane_container'],
          {
            'pointer-events-none select-none': isActive,
          },
        )}
        style={target === 'right' ? { width } : { flex: 1 }}
      >
        {right}
      </div>
    </div>
  );
};
