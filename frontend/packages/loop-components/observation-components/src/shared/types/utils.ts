// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode, type MouseEvent } from 'react';

export type ValueOf<T> = T[keyof T];

/**
 * 基础组件 Props 接口
 */
export interface BaseComponentProps {
  className?: string;
}

/**
 * 可关闭组件 Props 接口
 */
export interface ClosableProps {
  showClose?: boolean;
  onClose?: () => void;
}

/**
 * 多选项处理的通用类型
 */
export interface MultipleSelectProps {
  index: number;
  disabled: boolean;
  onClose: (tagContent: ReactNode, e: MouseEvent<Element>) => void;
}
