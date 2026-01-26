// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useBlocker } from 'react-router-dom';
import { useEffect } from 'react';

import { useI18n } from '../provider';

interface UnsaveLeaveWarningProps {
  /** 是否阻塞 */
  block: boolean;
  /** 自定义提示消息*/
  message?: string;
}

/**
 * 离开页面时的警告
 * 触发时机：关闭浏览器、关闭浏览器标签页、刷新页面、导航回退离开等
 */
export const useUnsaveLeaveWarning = ({
  block,
  message,
}: UnsaveLeaveWarningProps) => {
  const i18n = useI18n();
  const warnMessage =
    message || i18n.t('unsave_leave_confirm_warning') || 'Close confirm';

  useEffect(() => {
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      if (block) {
        event.preventDefault();
        event.returnValue = warnMessage; // 显示自定义消息（部分浏览器可能不支持）
      }
    };

    // 监听浏览器关闭标签页或刷新事件
    window.addEventListener('beforeunload', handleBeforeUnload);

    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, [block, warnMessage]);

  // 处理 React Router 导航离开
  useBlocker(() => {
    if (block) {
      // 弹出确认框
      return !window.confirm(warnMessage);
    }
    return true;
  });
};
