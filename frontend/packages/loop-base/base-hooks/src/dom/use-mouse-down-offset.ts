// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState, useRef, useCallback } from 'react';

import { useLatest, useMemoizedFn } from 'ahooks';

interface Offset {
  offsetX: number;
  offsetY: number;
}

export const useMouseDownOffset = (callback: (offset: Offset) => void) => {
  const [isActive, setIsActive] = useState(false);
  const targetRef = useRef<HTMLElement | null>(null);
  const [startPosition, setStartPosition] = useState<Offset | null>(null);
  const callbackRef = useLatest(callback);

  const ref = useCallback((node: HTMLElement | null) => {
    if (targetRef.current) {
      targetRef.current.removeEventListener('mousedown', handleMouseDown);
    }
    targetRef.current = node; // 更新 ref
    if (targetRef.current) {
      targetRef.current.addEventListener('mousedown', handleMouseDown);
    }
  }, []);

  const handleMouseDown = useMemoizedFn((event: MouseEvent) => {
    setIsActive(true);
    setStartPosition({ offsetX: event.clientX, offsetY: event.clientY });
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
  });

  const handleMouseMove = useMemoizedFn((event: MouseEvent) => {
    if (startPosition) {
      callbackRef.current({
        offsetX: event.clientX - startPosition.offsetX,
        offsetY: event.clientY - startPosition.offsetY,
      });
    }
  });

  const handleMouseUp = useMemoizedFn(() => {
    setIsActive(false);
    setStartPosition(null);
    window.removeEventListener('mousemove', handleMouseMove);
    window.removeEventListener('mouseup', handleMouseUp);
  });

  return { isActive, ref };
};
