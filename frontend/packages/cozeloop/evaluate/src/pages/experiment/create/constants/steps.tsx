import { type ReactNode } from 'react';

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
    title: '基础信息',
    nextStepText: '下一步：评测集',
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
  },
  {
    title: '评测集',
    nextStepText: '下一步：评测对象',
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
  },
  {
    title: '评测对象',
    nextStepText: '下一步：评估器',
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
    optional: true,
    tooltip:
      '跳过评测对象执行配置，适用于评测集已包含agent实际输出的评测场景。',
  },
  {
    title: '评估器',
    nextStepText: '确认实验配置',
    guardPoint: GuardPoint['eval.experiment_create.confirm'],
    optional: true,
    tooltip:
      '选择跳过评估器配置，实验仅获agent实际输出，适用于人工标注的评测场景。',
  },
  {
    hiddenStepBar: true,
    title: '确认实验配置',
    nextStepText: '发起实验',
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
