// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */
import React, {
  useCallback,
  useEffect,
  useRef,
  useState,
  useLayoutEffect,
  useMemo,
} from 'react';

import { nanoid } from 'nanoid';

import { type DebugMessage } from '@/store/use-mockdata-store';

import { MessageSkeleton } from './message-skeleton';
import { MessageItem } from './index';

interface VirtualMessageItemProps {
  item: DebugMessage;
  estimatedHeight?: number;
  rootMargin?: string;
  threshold?: number | number[];
  forceRender?: boolean;
  onHeightChange?: (height: number) => void;
  onVisibilityChange?: (visible: boolean) => void;
  [key: string]: any;
}

// 全局高度缓存
const heightCache = new Map<string, number>();
const visibilityCache = new Map<string, boolean>();

export const VirtualMessageItem = React.memo(function VirtualMessageItem({
  item,
  estimatedHeight = 120,
  rootMargin = '50px',
  threshold = [0, 0.01, 0.1, 0.5, 1],
  forceRender = false,
  onHeightChange,
  onVisibilityChange,
  ...props
}: VirtualMessageItemProps) {
  const [isVisible, setIsVisible] = useState(forceRender);
  const [realHeight, setRealHeight] = useState<number | null>(null);

  const containerRef = useRef<HTMLDivElement>(null);
  const messageRef = useRef<HTMLDivElement>(null);
  const observerRef = useRef<IntersectionObserver | null>(null);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);

  // 生成缓存键
  const cacheKey = useMemo(() => item?.id ?? nanoid(), [item?.id]);

  // 检查缓存
  useEffect(() => {
    const cachedHeight = heightCache.get(cacheKey);
    const cachedVisibility = visibilityCache.get(cacheKey);

    if (cachedHeight) {
      setRealHeight(cachedHeight);
    }

    if (cachedVisibility !== undefined) {
      setIsVisible(cachedVisibility);
    }
  }, [cacheKey]);

  // 更新高度
  const updateHeight = useCallback(() => {
    if (!messageRef.current) {
      return;
    }

    const height = messageRef.current.offsetHeight;
    if (height > 0 && height !== realHeight) {
      setRealHeight(height);
      heightCache.set(cacheKey, height);
      onHeightChange?.(height);
    }
  }, [realHeight, cacheKey]);

  // 处理可见性变化
  const handleVisibilityChange = useCallback(
    (visible: boolean) => {
      setIsVisible(visible);
      visibilityCache.set(cacheKey, visible);
      onVisibilityChange?.(visible);

      if (visible) {
        // 延迟一帧确保 DOM 已渲染
        requestAnimationFrame(() => {
          updateHeight();
        });
      }
    },
    [cacheKey, realHeight, forceRender],
  );

  // 设置 IntersectionObserver
  useEffect(() => {
    if (!containerRef.current) {
      return;
    }

    const observer = new IntersectionObserver(
      entries => {
        const entry = entries[0];
        const visible = entry.isIntersecting && entry.intersectionRatio > 0;
        handleVisibilityChange(visible);
      },
      {
        rootMargin,
        threshold,
      },
    );

    observer.observe(containerRef.current);
    observerRef.current = observer;

    return () => {
      observer.disconnect();
      observerRef.current = null;
    };
  }, [rootMargin, threshold, cacheKey, realHeight, forceRender]);

  // 设置 ResizeObserver
  useEffect(() => {
    if (!messageRef.current || !isVisible) {
      return;
    }

    const resizeObserver = new ResizeObserver(() => {
      updateHeight();
    });

    resizeObserver.observe(messageRef.current);
    resizeObserverRef.current = resizeObserver;

    return () => {
      resizeObserver.disconnect();
      resizeObserverRef.current = null;
    };
  }, [isVisible, realHeight, cacheKey, forceRender]);

  // 当消息变为可见时立即更新高度
  useLayoutEffect(() => {
    if (isVisible && messageRef.current) {
      updateHeight();
    }
  }, [isVisible, realHeight, cacheKey, forceRender]);

  // 计算当前高度
  const currentHeight = realHeight || estimatedHeight;

  // 容器样式
  const containerStyle = useMemo(() => {
    const baseStyle = {
      minHeight: isVisible ? 'auto' : `${currentHeight}px`,
    };

    if (!isVisible) {
      return {
        ...baseStyle,
        overflow: 'hidden',
      };
    }

    return baseStyle;
  }, [isVisible, currentHeight]);

  // 渲染内容
  const renderContent = () => {
    if (isVisible || forceRender) {
      return (
        <div ref={messageRef}>
          <MessageItem item={item} {...props} />
        </div>
      );
    }

    return (
      <MessageSkeleton
        messageType={item?.role}
        estimatedHeight={currentHeight}
      />
    );
  };

  return (
    <div ref={containerRef} style={containerStyle}>
      {renderContent()}
    </div>
  );
});
