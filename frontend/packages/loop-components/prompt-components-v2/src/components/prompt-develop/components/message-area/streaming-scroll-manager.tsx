// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useCallback, useEffect, useRef, useState } from 'react';

import { useDebounceFn } from 'ahooks';
import { scrollToBottom } from '@cozeloop/toolkit';

interface ScrollState {
  isAtBottom: boolean;
  isScrolling: boolean;
  shouldAutoScroll: boolean;
  scrollPosition: number;
  scrollHeight: number;
  clientHeight: number;
  isStreaming: boolean;
}

interface StreamingScrollManagerProps {
  containerRef: React.RefObject<HTMLDivElement>;
  isStreaming: boolean;
  streamingText?: string;
  onScrollStateChange?: (state: ScrollState) => void;
}

export function useStreamingScrollManager({
  containerRef,
  isStreaming,
  streamingText,
  onScrollStateChange,
}: StreamingScrollManagerProps) {
  const [scrollState, setScrollState] = useState<ScrollState>({
    isAtBottom: true,
    isScrolling: false,
    shouldAutoScroll: true,
    scrollPosition: 0,
    scrollHeight: 0,
    clientHeight: 0,
    isStreaming,
  });

  const lastIsAtBottomRef = useRef(false);
  const userScrollTimeoutRef = useRef<NodeJS.Timeout>();
  const lastScrollTopRef = useRef(0);
  const isScrollingProgrammaticallyRef = useRef(false);
  const streamingScrollTimeoutRef = useRef<NodeJS.Timeout>();
  const isUserInteractingRef = useRef(false);

  // 检查是否在底部
  const checkIsAtBottom = useCallback(() => {
    if (!containerRef.current) {
      return false;
    }

    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const SCROLL_THRESHOLD = 10; // 10px 的容错范围
    return scrollHeight - clientHeight <= scrollTop + SCROLL_THRESHOLD;
  }, [containerRef]);

  const handleScrollToBottom = () => {
    if (!containerRef.current) {
      return;
    }
    // 重置状态
    if (userScrollTimeoutRef.current) {
      clearTimeout(userScrollTimeoutRef.current);
    }
    if (streamingScrollTimeoutRef.current) {
      clearTimeout(streamingScrollTimeoutRef.current);
    }
    isUserInteractingRef.current = false;
    setScrollState(prev => {
      const newState = {
        ...prev,
        isAtBottom: true,
        isScrolling: false,
      };
      onScrollStateChange?.(newState);
      return newState;
    });
    scrollToBottom(containerRef);
  };

  // 处理用户交互事件
  const handleUserInteraction = useCallback(() => {
    isUserInteractingRef.current = true;

    // 清除之前的定时器
    if (userScrollTimeoutRef.current) {
      clearTimeout(userScrollTimeoutRef.current);
    }

    // 设置新的定时器
    const USER_INTERACTION_DEBOUNCE_DELAY = 200;
    userScrollTimeoutRef.current = setTimeout(() => {
      isUserInteractingRef.current = false;
    }, USER_INTERACTION_DEBOUNCE_DELAY);
  }, []);

  const { run: handleScrollStop } = useDebounceFn(
    () => {
      setScrollState(prev => {
        const newState = {
          ...prev,
          isScrolling: false,
        };
        onScrollStateChange?.(newState);
        return newState;
      });
    },
    { wait: 300 },
  );
  // 滚动事件处理
  const handleScroll = useCallback(() => {
    if (!containerRef.current || isScrollingProgrammaticallyRef.current) {
      return;
    }

    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const isAtBottom = checkIsAtBottom();
    lastScrollTopRef.current = scrollTop;
    setScrollState(prev => {
      const newIsAtBottom =
        isStreaming && !isUserInteractingRef.current
          ? prev.isAtBottom
          : isAtBottom;
      const shouldAutoScroll =
        newIsAtBottom && !isUserInteractingRef.current && isStreaming;
      const newState = {
        isAtBottom: newIsAtBottom,
        isScrolling: true,
        shouldAutoScroll,
        scrollPosition: scrollTop,
        scrollHeight,
        clientHeight,
        isStreaming,
      };
      onScrollStateChange?.(newState);
      return newState;
    });

    handleScrollStop();
  }, [isStreaming]);

  // 监听用户交互事件
  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return;
    }

    // 监听用户滚动相关事件
    const userInteractionEvents = [
      'wheel', // 鼠标滚轮滚动
      'mousedown', // 鼠标按下（可能拖动滚动条）
      'mousemove', // 鼠标移动（拖动滚动条时）
      'touchstart', // 触摸开始（移动设备）
      'touchmove', // 触摸移动（移动设备）
      'keydown', // 键盘事件（方向键等）
    ];

    const handleUserEvent = (event: Event) => {
      // 对于 mousemove 事件，只有在鼠标按下时才认为是用户操作
      if (event.type === 'mousemove') {
        if (!(event as MouseEvent).buttons) {
          return;
        }
      }

      // 对于键盘事件，只监听滚动相关的键
      if (event.type === 'keydown') {
        const keyEvent = event as KeyboardEvent;
        const scrollKeys = [
          'ArrowUp',
          'ArrowDown',
          'PageUp',
          'PageDown',
          'Home',
          'End',
          ' ',
        ];
        if (!scrollKeys.includes(keyEvent.key)) {
          return;
        }
      }

      handleUserInteraction();
    };

    userInteractionEvents.forEach(eventType => {
      container.addEventListener(eventType, handleUserEvent, { passive: true });
    });

    return () => {
      userInteractionEvents.forEach(eventType => {
        container.removeEventListener(eventType, handleUserEvent);
      });
    };
  }, []);

  // 监听滚动事件
  useEffect(() => {
    const container = containerRef.current;
    if (!container) {
      return;
    }

    container.addEventListener('scroll', handleScroll, { passive: true });

    // 初始化检查
    handleScroll();

    return () => {
      container.removeEventListener('scroll', handleScroll);
      if (userScrollTimeoutRef.current) {
        clearTimeout(userScrollTimeoutRef.current);
      }
      if (streamingScrollTimeoutRef.current) {
        clearTimeout(streamingScrollTimeoutRef.current);
      }
    };
  }, [isStreaming]);

  // 流式输出时的自动滚动
  useEffect(() => {
    if (isStreaming) {
      if (scrollState.shouldAutoScroll) {
        lastIsAtBottomRef.current = true;
        scrollToBottom(containerRef);
      } else {
        lastIsAtBottomRef.current = false;
      }
    }
  }, [
    isStreaming,
    streamingText,
    scrollState.shouldAutoScroll,
    scrollState.isAtBottom,
  ]);

  // 流式输出状态变化
  useEffect(() => {
    if (!isStreaming) {
      // 重置状态
      if (userScrollTimeoutRef.current) {
        clearTimeout(userScrollTimeoutRef.current);
      }
      if (streamingScrollTimeoutRef.current) {
        clearTimeout(streamingScrollTimeoutRef.current);
      }

      if (lastIsAtBottomRef.current) {
        setTimeout(() => {
          handleScrollToBottom();
        }, 100);
      }
    }
  }, [isStreaming]);

  return {
    scrollState,
    checkIsAtBottom,
    handleScrollToBottom,
  };
}
