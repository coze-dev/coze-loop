// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  EvaluatorPreview,
  uniqueExperimentsEvaluators,
  getLogicFieldName,
  type LogicField,
} from '@cozeloop/evaluate-components';
import { FieldType, type Experiment } from '@cozeloop/api-schema/evaluation';

import { ExperimentItemRunStatusSelect } from '@/components/experiment';

export function getExperimentContrastLogicFields(
  experiments: Experiment[],
): LogicField[] {
  const evaluators = uniqueExperimentsEvaluators(experiments);
  const evaluatorFields: LogicField[] = evaluators.map(evaluator => {
    const versionId = evaluator?.current_version?.id?.toString() ?? '';
    const field: LogicField = {
      title: <EvaluatorPreview evaluator={evaluator} className="ml-2" />,
      name: getLogicFieldName(FieldType.EvaluatorScore, versionId),
      type: 'number',
    };
    return field;
  });
  return [
    {
      title: I18n.t('status'),
      name: getLogicFieldName(FieldType.TurnRunState, 'turn_status'),
      type: 'options',
      // 禁用等于和不等于操作符
      disabledOperations: ['equals', 'not-equals'],
      setter: ExperimentItemRunStatusSelect,
      setterProps: {
        className: 'w-full',
        prefix: '',
        maxTagCount: 2,
        showClear: false,
        showIcon: false,
      },
    },
    ...evaluatorFields,
  ];
}
