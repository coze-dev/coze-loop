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
      title: '状态',
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
