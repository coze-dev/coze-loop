import { EvalTargetType } from '@cozeloop/api-schema/evaluation';

import {
  type CreateExperimentValues,
  ExtCreateStep,
  type EvalTargetDefinition,
} from '../../../types/evaluate-target';
import { PromptEvalTargetSelect } from '../../../components/selectors/evaluate-target';
import PromptTargetPreview from './prompt-target-preview';
import { PromptEvalTargetView } from './prompt-eval-target-view';
import PluginEvalTargetForm from './plugin-eval-target-form';

const getEvalTargetValidFields = (values: CreateExperimentValues) => {
  const { evalTargetMapping = {} } = values;
  const result = ['evalTarget', 'evalTargetVersion', 'evalTargetMapping'];

  Object.keys(evalTargetMapping).forEach(key => {
    // evalTargetMapping.input
    result.push(`evalTargetMapping.${key}`);
  });
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
  targetInfo: {
    color: 'primary',
    tagColor: 'primary',
  },
};
