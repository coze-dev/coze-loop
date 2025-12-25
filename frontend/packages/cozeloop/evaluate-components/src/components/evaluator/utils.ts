// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Evaluator, EvaluatorType } from '@cozeloop/api-schema/evaluation';

export const getEvaluatorJumpUrl = ({
  evaluatorType = EvaluatorType.Prompt,
  evaluatorId = '',
  evaluatorVersionId = '',
  isBuiltin = false,
}: {
  evaluatorType?: EvaluatorType;
  evaluatorId?: string;
  evaluatorVersionId?: string;
  isBuiltin?: boolean;
}) => {
  // 预置评估器
  if (isBuiltin) {
    return `evaluation/evaluators/${evaluatorId}?isPreEvaluator=true&version=${evaluatorVersionId}`;
  }

  // 自建评估器
  if (evaluatorType === EvaluatorType.Code) {
    return `evaluation/evaluators/code/${evaluatorId}?version=${evaluatorVersionId}`;
  } else {
    return `evaluation/evaluators/${evaluatorId}?version=${evaluatorVersionId}`;
  }
};

/** 包含预置评估器跳转 */
export const getEvaluatorJumpUrlV2 = (
  evaluator?: Evaluator,
  customVersion?: string,
) => {
  const { builtin, evaluator_type, evaluator_id, current_version } =
    evaluator ?? {};

  // 预置评估器
  if (builtin) {
    return `evaluation/evaluators/${evaluator_id}?isPreEvaluator=true&version=${customVersion ?? current_version?.id}`;
  }

  let link = '';

  // 自建评估器
  if (evaluator_type === EvaluatorType.Code) {
    link = `${link}evaluation/evaluators/code/${evaluator_id}`;
  } else {
    link = `${link}evaluation/evaluators/${evaluator_id}`;
  }

  if (current_version?.id) {
    link = `${link}?version=${current_version?.id}`;
  }
  return link;
};
