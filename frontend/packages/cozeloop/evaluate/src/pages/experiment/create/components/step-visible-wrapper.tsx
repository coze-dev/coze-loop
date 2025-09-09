// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode } from 'react';

interface StepVisibleWrapperProps {
  /**
   * 是否可见
   */
  visible: boolean;
  /**
   * 子组件
   */
  children: ReactNode;
}

/**
 * 步骤可见性包装器
 *
 * 该组件用于包装各个步骤的表单组件，使其在不是当前步骤时隐藏但仍然保持渲染
 * 这样可以保持表单状态，同时控制UI显示
 */
export const StepVisibleWrapper = ({
  visible,
  children,
}: StepVisibleWrapperProps) => (
  <div className={visible ? '' : 'hidden'}>{children}</div>
);
