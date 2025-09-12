// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention */
import { type ReactNode } from 'react';

import { type BenefitConfig } from '@cozeloop/biz-hooks-adapter';

export { type BenefitConfig } from '@cozeloop/biz-hooks-adapter';

export enum GuardActionType {
  /** 隐藏 */
  HIDDEN = 1,
  /** 只读 */
  READONLY = 2,
  /** 可操作 */
  ACTION = 3,
  /** 拦截 */
  GUARD = 4,
}

/**
 * 守护点位
 */
export enum GuardPoint {
  /** Demo 空间标识 */
  'base.navbar.demo_tag' = 'base.navbar.demo_tag',
  'base.navbar.subscribe' = 'base.navbar.subscribe',
  'base.navbar.auth' = 'base.navbar.auth',
  'base.navbar.account_setting' = 'base.navbar.account_setting',
  'base.navbar.space_manage' = 'base.navbar.space_manage',
  'base.navbar.auto_task' = 'base.navbar.auto_task',
  'pe.prompts.create' = 'pe.prompts.create',
  'pe.prompts.delete' = 'pe.prompts.delete',
  'pe.prompts.history' = 'pe.prompts.history',
  'pe.prompt.global' = 'pe.prompt.global',
  'pe.prompt.edit_meta' = 'pe.prompt.edit_meta',
  'pe.prompt.optimize' = 'pe.prompt.optimize',
  'pe.prompt.execute' = 'pe.prompt.execute',
  'pe.prompt.smart_optimize' = 'pe.prompt.smart_optimize',
  'pe.prompt.retry' = 'pe.prompt.retry',
  'pe.playground.quick_create' = 'pe.playground.quick_create',
  'eval.datasets.create_experiment' = 'eval.datasets.create_experiment',
  'eval.datasets.create' = 'eval.datasets.create',
  'eval.datasets.edit' = 'eval.datasets.edit',
  'eval.datasets.copy' = 'eval.datasets.copy',
  'eval.datasets.delete' = 'eval.datasets.delete',
  'eval.dataset_create.create' = 'eval.dataset_create.create',
  'eval.dataset.edit_meta' = 'eval.dataset.edit_meta',
  'eval.dataset.commit' = 'eval.dataset.commit',
  'eval.dataset.batch_delete' = 'eval.dataset.batch_delete',
  'eval.dataset.export' = 'eval.dataset.export',
  'eval.dataset.import' = 'eval.dataset.import',
  'eval.dataset.add' = 'eval.dataset.add',
  'eval.dataset.edit' = 'eval.dataset.edit',
  'eval.dataset.delete' = 'eval.dataset.delete',
  'eval.dataset.edit_col' = 'eval.dataset.edit_col',
  'eval.dataset.create_experiment' = 'eval.dataset.create_experiment',
  'eval.dataset_exp.copy' = 'eval.dataset_exp.copy',
  'eval.dataset_exp.delete' = 'eval.dataset_exp.delete',
  'eval.dataset_exp.execute' = 'eval.dataset_exp.execute',
  'eval.evaluators.create' = 'eval.evaluators.create',
  'eval.evaluators.edit' = 'eval.evaluators.edit',
  'eval.evaluators.copy' = 'eval.evaluators.copy',
  'eval.evaluators.delete' = 'eval.evaluators.delete',
  'eval.evaluator_create.debug' = 'eval.evaluator_create.debug',
  'eval.evaluator_create.preview_debug' = 'eval.evaluator_create.preview_debug',
  'eval.evaluator_create.create' = 'eval.evaluator_create.create',
  'eval.evaluator_create.global' = 'eval.evaluator_create.global',
  'eval.evaluator.global' = 'eval.evaluator.global',
  'eval.evaluator.edit_meta' = 'eval.evaluator.edit_meta',
  'eval.evaluator.commit' = 'eval.evaluator.commit',
  'eval.experiments.create' = 'eval.experiments.create',
  'eval.experiments.copy' = 'eval.experiments.copy',
  'eval.experiments.delete' = 'eval.experiments.delete',
  'eval.experiments.retry' = 'eval.experiments.retry',
  'eval.experiments.compare' = 'eval.experiments.compare',
  'eval.experiments.batch_delete' = 'eval.experiments.batch_delete',
  'eval.experiment_create.confirm' = 'eval.experiment_create.confirm',
  'eval.experiment.edit_meta' = 'eval.experiment.edit_meta',
  'eval.experiment.compare' = 'eval.experiment.compare',
  'eval.experiment.edit_result' = 'eval.experiment.edit_result',
  'ob.trace.custom_view' = 'ob.trace.custom_view',
  'ob.trace.annotation' = 'ob.trace.annotation',
  'ob.auto_task.create' = 'ob.auto_task.create',
  'ob.auto_task.evaluator_debug' = 'ob.auto_task.evaluator_debug',
  'ob.auto_task.submit_task_form' = 'ob.auto_task.submit_task_form',
  'ob.auto_task.task_write_action' = 'ob.auto_task.task_write_action',
  'ob.data_reflux.create' = 'ob.data_reflux.create',
  'ob.data_reflux.submit_data_reflux' = 'ob.data_reflux.submit_data_reflux',
  'pe.prompts.search_by_creator' = 'pe.prompts.search_by_creator',
  'eval.datasets.search_by_creator' = 'eval.datasets.search_by_creator',
  'eval.evaluators.search_by_creator' = 'eval.evaluators.search_by_creator',
  'eval.experiments.search_by_creator' = 'eval.experiments.search_by_creator',
  'data.label.create' = 'data.label.create',
  'data.label.edit' = 'data.label.edit',
}

/**
 * 定义每个点位的行为
 */
export type GuardConfig<T> = Record<GuardPoint, (context: T) => GuardResult>;

export interface GuardResult {
  type: GuardActionType;
  config?: GuardModalConfig;
}

/**
 * 约定守卫弹窗配置
 */
export interface GuardModalConfig {
  title: ReactNode;
  content: ReactNode;
  btnText: string;
  onConfirm?: () => void | Promise<void>;
  // 打开弹窗前校验是否能
  preCheck?: (context: ContextConfig) => boolean;
  // 确认后，触发原有的点击事件
  triggerOriginalClick?: boolean;
}

/**
 * 收集权益、路由等信息，用于上下文
 */
export interface ContextConfig {
  benefit?: BenefitConfig;
  // 所属模块
  app: string;
  // 是否示例空间
  isDemoSpace: boolean;
}

export interface GuardProps<T> {
  key: GuardPoint;
  context: T;
  configs: GuardConfig<T>;
}

// 权限结果接口，包含类型和状态
export interface GuardResultData {
  type: GuardActionType;
  readonly: boolean;
  hidden: boolean;
  guard: boolean;
  config?: {
    title: React.ReactNode;
    content: React.ReactNode;
    btnText: string;
    onConfirm?: () => void | Promise<void>;
    triggerOriginalClick?: boolean;
  };
  preprocess: (callback?: () => void) => void;
}

export type GuardResultDataMap = Record<GuardPoint, GuardResultData>;

// 权限策略接口
export interface GuardStrategy {
  // 检查单个权限点
  checkGuardPoint: (point: GuardPoint) => GuardResultData;

  // 刷新权限数据
  refreshGuardData: () => void;

  // 是否正在加载
  loading: boolean;
}
