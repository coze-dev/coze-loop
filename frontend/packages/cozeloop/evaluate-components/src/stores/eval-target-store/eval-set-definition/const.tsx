import { type EvalTargetType } from '@cozeloop/api-schema/evaluation';

import { type EvalTargetDefinition } from '../../../types/evaluate-target';
import { SetEvalTargetView } from './set-eval-target-view';
import PluginEvalTargetForm from './eval-target-form-content';

const setTransformEvaluatorEvalTargetSchemas = () => [];

export const evalSetDefinitionPayload: EvalTargetDefinition = {
  type: 5 as EvalTargetType,
  name: '评测集',
  selector: () => <div>123</div>,
  description:
    '选择上一步配置的评测集作为评测对象，适用于该评测集已包含agent输出的场景。',
  // preview: PromptTargetPreview,
  // extraValidFields: {
  //   [ExtCreateStep.EVAL_TARGET]: getEvalTargetValidFields,
  // },
  preview: () => <div>123</div>,
  evalTargetFormSlotContent: PluginEvalTargetForm,
  transformEvaluatorEvalTargetSchemas: setTransformEvaluatorEvalTargetSchemas,
  evalTargetView: SetEvalTargetView,
  viewSubmitFieldMappingPreview: () => <div />,
  targetInfo: {
    color: 'blue',
    tagColor: 'blue',
  },
};
