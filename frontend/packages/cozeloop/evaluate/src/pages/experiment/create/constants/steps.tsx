// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint } from '@cozeloop/guard';

export interface StepConfig {
  title: ReactNode;
  nextStepText?: string;
  hiddenStepBar?: boolean;
  isLast?: boolean;
  guardPoint: string;
  optional?: boolean;
  tooltip?: string;
}

export const STEPS: StepConfig[] = [
  {
    title: I18n.t('basic_info'),
    nextStepText: I18n.t('next_step_evaluation_set'),
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
  },
  {
    title: I18n.t('evaluation_set'),
    nextStepText: I18n.t('next_step_evaluation_object'),
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
  },
  {
    title: I18n.t('evaluation_object'),
    nextStepText: I18n.t('next_step_evaluator'),
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
    optional: true,
    tooltip: I18n.t('evaluate_skip_target_execution_config'),
  },
  {
    title: I18n.t('evaluator'),
    nextStepText: I18n.t('confirm_experiment_config'),
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
    optional: true,
    tooltip: I18n.t('evaluate_skip_evaluator_config_manual_labeling'),
  },
  {
    hiddenStepBar: true,
    title: I18n.t('confirm_experiment_config'),
    nextStepText: I18n.t('initiate_experiment'),
    isLast: true,
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
  },
];

// 步骤事件映射
export const STEP_EVENT_MAP = {
  0: 'next_evaluateSet',
  1: 'next_evaluateTarget',
  2: 'next_evaluator',
  3: 'next_confirm_config',
  4: 'next_launch_experiment',
} as const;

// 赶时间先这样, 后面换种优雅的写法
export const stepNameMap: Record<
  number,
  | ['next_evaluateSet', 'basic_info']
  | ['next_evaluateTarget', 'evaluate_set']
  | ['next_evaluator', 'evaluate_target']
  | ['next_confirm_config', 'evaluator']
  | ['next_launch_experiment', 'view_submit']
> = {
  0: ['next_evaluateSet', 'basic_info'],
  1: ['next_evaluateTarget', 'evaluate_set'],
  2: ['next_evaluator', 'evaluate_target'],
  3: ['next_confirm_config', 'evaluator'],
  4: ['next_launch_experiment', 'view_submit'],
};
