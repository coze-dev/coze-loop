// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  EvalTargetType,
  PromptUserQueryFieldKey,
} from '@cozeloop/api-schema/evaluation';

import {
  type CreateExperimentValues,
  ExtCreateStep,
  type EvalTargetDefinition,
} from '../../../types/evaluate-target';
import { PromptEvalTargetSelect } from '../../../components/selectors/evaluate-target';
import PromptTargetPreview from './prompt-target-preview';
import { PromptFieldMappingPreview } from './prompt-field-mapping-preview';
import { PromptEvalTargetView } from './prompt-eval-target-view';
import PluginEvalTargetForm from './plugin-eval-target-form';

const getEvalTargetValidFields = (values: CreateExperimentValues) => {
  const { evalTargetMapping = {}, target_runtime_param } = values;
  const result = ['evalTarget', 'evalTargetVersion', 'evalTargetMapping'];
  // 动态参数校验
  if (target_runtime_param) {
    result.push('target_runtime_param');
  }

  Object.keys(evalTargetMapping).forEach(key => {
    // evalTargetMapping.input
    result.push(`evalTargetMapping.${key}`);
  });
  // user_query 校验
  result.push(`evalTargetMapping.${PromptUserQueryFieldKey}`);

  return result;
};

export const promptEvalTargetDefinitionPayload: EvalTargetDefinition = {
  type: EvalTargetType.CozeLoopPrompt,
  name: 'Prompt',
  selector: PromptEvalTargetSelect,
  preview: PromptTargetPreview,
  extraValidFields: {
    [ExtCreateStep.EVAL_TARGET]: getEvalTargetValidFields,
  },
  evalTargetFormSlotContent: PluginEvalTargetForm,
  evalTargetView: PromptEvalTargetView,
  viewSubmitFieldMappingPreview: PromptFieldMappingPreview,
  targetInfo: {
    color: 'primary',
    tagColor: 'primary',
  },
};
