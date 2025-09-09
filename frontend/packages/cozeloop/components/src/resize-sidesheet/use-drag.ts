// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, useState } from 'react';

import { useMouseDownOffset } from '../hooks/use-mouse-down-offset';

const DEFAULT_WIDTH = 500;
const MAX_WIDTH = 800;
const MIN_WIDTH = 300;

export interface DragOptions {
  defaultWidth?: number;
  maxWidth?: number;
  minWidth?: number;
}

export const useDrag = (options: DragOptions = {}) => {
  const {
    defaultWidth = DEFAULT_WIDTH,
    maxWidth = MAX_WIDTH,
    minWidth = MIN_WIDTH,
  } = options;
  const [sidePaneWidth, setSidePaneWidth] = useState(defaultWidth);
  const prevWidthRef = useRef(sidePaneWidth);
  const { ref, isActive } = useMouseDownOffset(({ offsetX }) => {
    const newWidth = prevWidthRef.current - offsetX;
    setSidePaneWidth([maxWidth, newWidth, minWidth].sort((a, b) => a - b)[1]);
  });
  useEffect(() => {
    prevWidthRef.current = sidePaneWidth;
    document.body.style.cursor = isActive ? 'col-resize' : '';
    document.body.style.userSelect = isActive ? 'none' : 'auto';
  }, [isActive]);
  return {
    sidePaneWidth,
    containerRef: ref,
    isActive,
  };
};
