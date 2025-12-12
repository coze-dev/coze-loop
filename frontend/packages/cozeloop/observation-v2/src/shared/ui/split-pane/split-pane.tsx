import React, { useState, useEffect } from 'react';

import cls from 'classnames';

import { useMouseDownOffset } from '@/shared/hooks/use-mouse-down-offset';

import styles from './index.module.less';

type PaneType = React.ReactNode;

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
    <div className={cls('flex w-full h-full', className)}>
      <div
        className={cls(
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
        className={cls(
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
