// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createContext, useContext, type ReactNode } from 'react';

import { GuardActionType, type GuardStrategy } from '../types';

// 默认策略实现
const defaultStrategyImpl: GuardStrategy = {
  checkGuardPoint: () => ({
    type: GuardActionType.ACTION,
    readonly: false,
    hidden: false,
    guard: false,
    preprocess: (callback?: () => void) => {
      // 开源版直接调用回调，不做拦截
      callback?.();
    },
  }),

  refreshGuardData: async () => {
    // 等待异步操作完成
    await Promise.resolve();
  },

  loading: false,
};

// 创建上下文对象，提供默认策略
const GuardContext = createContext<GuardStrategy>(defaultStrategyImpl);

// Provider组件
export const GuardProvider: React.FC<{
  strategy?: GuardStrategy;
  children: ReactNode;
}> = ({ strategy = defaultStrategyImpl, children }) => (
  <GuardContext.Provider value={strategy}>{children}</GuardContext.Provider>
);

// 使用上下文的Hook
export const useGuardStrategy = (): GuardStrategy => useContext(GuardContext);
