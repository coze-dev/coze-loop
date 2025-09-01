import { useState } from 'react';

import { type prompt } from '@cozeloop/api-schema/prompt';
import { type RuntimeParam } from '@cozeloop/api-schema/evaluation';
import { get } from '@cozeloop/adapter';
import { withField } from '@coze-arch/coze-design';

import { EvaluateTargetPromptDynamicParams as EvaluateTargetPromptDynamicParamsBase } from '@/adapter';

import { DynamicParamsField } from './dynamic-params-field';
interface Props {
  promptDetail?: prompt.Prompt;
  disabled?: boolean;
  initialValue?: RuntimeParam;
  onChange?: (v?: RuntimeParam) => void;
}

export const EvalTargetDynamicParams = ({
  promptDetail,
  disabled,
  initialValue,
  onChange,
}: Props) => {
  const [FormEvaluateTargetPromptDynamicParams] = useState(() => {
    const EvaluateTargetPromptDynamicParams =
      get('eval.experiments', 'EvaluateTargetPromptDynamicParams') ||
      EvaluateTargetPromptDynamicParamsBase;
    return withField(EvaluateTargetPromptDynamicParams);
  });

  if (!promptDetail) {
    return null;
  }

  return (
    <DynamicParamsField open={!!initialValue}>
      {FormEvaluateTargetPromptDynamicParams ? (
        <FormEvaluateTargetPromptDynamicParams
          noLabel
          field="target_runtime_param"
          initValue={initialValue}
          disabled={disabled}
          prompt={promptDetail}
          promptID={promptDetail.id}
          promptVersion={promptDetail.prompt_commit?.commit_info?.version}
          onChange={onChange}
        />
      ) : null}
    </DynamicParamsField>
  );
};
