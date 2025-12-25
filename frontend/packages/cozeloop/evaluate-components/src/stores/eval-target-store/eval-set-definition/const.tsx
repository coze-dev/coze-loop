// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { type EvalTargetType } from '@cozeloop/api-schema/evaluation';

import { type EvalTargetDefinition } from '../../../types/evaluate-target';
import { SetEvalTargetView } from './set-eval-target-view';
import PluginEvalTargetForm from './eval-target-form-content';

const setTransformEvaluatorEvalTargetSchemas = () => [];

export const evalSetDefinitionPayload: EvalTargetDefinition = {
  type: 5 as EvalTargetType,
  name: I18n.t('evaluation_set'),
  selector: () => <div>123</div>,
  description: I18n.t(
    'cozeloop_open_evaluate_select_previous_eval_set_as_target',
  ),
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
