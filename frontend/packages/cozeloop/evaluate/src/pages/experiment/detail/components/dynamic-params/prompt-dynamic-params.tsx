import {
  EvaluateTargetPromptDynamicParams as EvaluateTargetPromptDynamicParamsBase,
  usePromptDetail,
} from '@cozeloop/evaluate-components';
import {
  EvalTargetType,
  type EvalTarget,
  type RuntimeParam,
} from '@cozeloop/api-schema/evaluation';
import { useAdapter } from '@cozeloop/adapter';

interface Props {
  data: RuntimeParam;
  evalTarget?: EvalTarget;
}

export function PromptDynamicParams({ data, evalTarget }: Props) {
  const EvaluateTargetPromptDynamicParams =
    useAdapter('eval.experiments', 'EvaluateTargetPromptDynamicParams') ||
    EvaluateTargetPromptDynamicParamsBase;

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
