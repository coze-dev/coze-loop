// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export { Guard } from './guard';
export {
  GuardPoint,
  GuardActionType,
  type GuardStrategy,
  type GuardResultData,
  type GuardResult,
  type GuardConfig,
  type ContextConfig,
} from './types';
export { useGuard, useGuards } from './hooks/use-guard';
export { GuardRoute } from './guard-route';

// 导出上下文相关内容
export { GuardProvider, useGuardStrategy } from './context';
