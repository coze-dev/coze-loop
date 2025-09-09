// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type EvaluateTargetValues,
  CreateExperimentValues,
} from '@cozeloop/evaluate-components';
import {
  type Evaluator,
  type EvaluatorVersion,
} from '@cozeloop/api-schema/evaluation';

import { type OptionSchema } from '@/components/mapping-item-field/types';

export interface EvaluatorPro {
  evaluator?: Evaluator;
  evaluatorVersion?: EvaluatorVersion;
  evaluatorVersionDetail?: EvaluatorVersion;
  // key: 评估器字段名，value: 评测目标字段名
  evaluatorMapping?: Record<string, OptionSchema>;
}
export { CreateExperimentValues };

export type BaseInfoValues = Pick<CreateExperimentValues, 'name' | 'desc'>;

export type EvaluateSetValues = Pick<
  CreateExperimentValues,
  | 'eval_set_id'
  | 'eval_set_version_id'
  | 'evaluationSet'
  | 'evaluationSetVersion'
  | 'evaluationSetVersionDetail'
>;

export type EvaluatorValues = Pick<
  CreateExperimentValues,
  'evaluator_version_ids' | 'evaluator_field_mapping' | 'evaluatorProList'
>;

export type CommonFormRef = {
  validate?: () => Promise<
    BaseInfoValues | EvaluateSetValues | EvaluateTargetValues | EvaluatorValues
  >;
  getFormApi?: () => { getValues: () => CreateExperimentValues };
} | null;

export { type EvaluateTargetValues };
