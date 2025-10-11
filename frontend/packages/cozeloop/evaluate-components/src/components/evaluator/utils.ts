import { EvaluatorType } from '@cozeloop/api-schema/evaluation';

export const getEvaluatorJumpUrl = ({
  evaluatorType = EvaluatorType.Prompt,
  evaluatorId = '',
  evaluatorVersionId = '',
}: {
  evaluatorType?: EvaluatorType;
  evaluatorId?: string;
  evaluatorVersionId?: string;
}) => {
  if (evaluatorType === EvaluatorType.Code) {
    return `evaluation/evaluators/code/${evaluatorId}?version=${evaluatorVersionId}`;
  } else {
    return `evaluation/evaluators/${evaluatorId}?version=${evaluatorVersionId}`;
  }
};
