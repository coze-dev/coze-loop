// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { devtools } from 'zustand/middleware';
import { create } from 'zustand';

// 定义状态类型
interface EvaluationFlagState {
  // 评估分析空间配置
  enableEvaluationAnalysis: boolean;
}

// 定义操作类型
interface EvaluationFlagAction {
  // 设置评估分析空间列表
  setEnableEvaluationAnalysis: (enableEvaluationAnalysis: boolean) => void;
}

// 定义事件枚举，方便后续扩展
export enum EvaluationFlagEvent {
  SPACES_UPDATED = 'spaces-updated',
}

const IS_DEV_MODE = process.env.NODE_ENV === 'development';

// 创建并导出全局状态
export const useEvaluationFlagStore = create<
  EvaluationFlagState & EvaluationFlagAction
>()(
  devtools(
    set => ({
      // 初始状态
      enableEvaluationAnalysis: false,

      // 设置评估分析空间列表
      setEnableEvaluationAnalysis: enableEvaluationAnalysis => {
        set({ enableEvaluationAnalysis });
      },
    }),
    { name: 'cozeloop.evaluation.flag', enabled: IS_DEV_MODE },
  ),
);
