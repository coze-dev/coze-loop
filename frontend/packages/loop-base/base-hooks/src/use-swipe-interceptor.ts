// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */

import { useEffect, useRef } from 'react';

import { useLatest } from 'ahooks';

// 通用滑动拦截 hook
interface UseSwipeInterceptorOptions {
  onSwipeDetected: () => Promise<void>; // 检测到滑动时的回调
  deps?: any; // 是否启用拦截
  createSuccess?: boolean;
  pageName?: string;
}

export function useSwipeInterceptor({
  onSwipeDetected,
  deps,
  createSuccess,
  pageName,
}: UseSwipeInterceptorOptions) {
  const createSuccessRef = useLatest(createSuccess);
  // 滑动拦截相关状态
  const isProcessingNavigation = useRef(false);
  const currentHistoryIndex = useRef(0);

  // 初始化历史记录状态
  useEffect(() => {
    // 记录当前历史记录位置
    currentHistoryIndex.current = window.history.length;

    // 添加一个历史记录条目，防止直接后退
    window.history.pushState(
      { page: pageName || '' },
      '',
      window.location.href,
    );

    return () => {
      // 清理时移除添加的历史记录
      if (window.history.length > 1 && !createSuccessRef.current) {
        window.history.back();
      }
    };
  }, []);

  // 拦截浏览器历史记录变化
  const handlePopState = (event: PopStateEvent) => {
    if (isProcessingNavigation.current || createSuccessRef.current) {
      return;
    }

    // 阻止默认的历史记录变化
    event.preventDefault();

    // 立即添加一个新的历史记录条目来阻止跳转
    window.history.pushState(
      { page: 'optimization-create' },
      '',
      window.location.href,
    );

    isProcessingNavigation.current = true;

    // 触发提示
    console.log('handlePopState');
    onSwipeDetected()
      .finally(() => {
        isProcessingNavigation.current = false;
      })
      .catch(() => {
        isProcessingNavigation.current = false;
      });
  };

  // 添加事件监听器
  useEffect(() => {
    // 添加 popstate 事件监听器（拦截浏览器历史记录变化）
    window.addEventListener('popstate', handlePopState);

    return () => {
      window.removeEventListener('popstate', handlePopState);
    };
  }, [deps]);
}
