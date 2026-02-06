// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { usePromptDetail } from '@cozeloop/evaluate-components';
import { EvaluateTargetPromptDynamicParams } from '@cozeloop/evaluate-adapter';
import {
  EvalTargetType,
  type EvalTarget,
  type RuntimeParam,
} from '@cozeloop/api-schema/evaluation';

interface Props {
  data: RuntimeParam;
  evalTarget?: EvalTarget;
}

export function PromptDynamicParams({ data, evalTarget }: Props) {
  const targetPrompt =
    evalTarget?.eval_target_version?.eval_target_content?.prompt;

  const { promptDetail } = usePromptDetail({
    promptId: targetPrompt?.prompt_id || '',
    version: targetPrompt?.version || '',
  });

  return evalTarget?.eval_target_type === EvalTargetType.CozeLoopPrompt ? (
    <EvaluateTargetPromptDynamicParams
      disabled
      value={data}
      prompt={promptDetail}
    />
  ) : null;
}
