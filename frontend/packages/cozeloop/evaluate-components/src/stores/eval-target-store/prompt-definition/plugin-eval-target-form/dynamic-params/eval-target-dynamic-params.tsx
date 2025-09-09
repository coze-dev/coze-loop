// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { useLatest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { type prompt, VariableType } from '@cozeloop/api-schema/prompt';
import { type Model } from '@cozeloop/api-schema/llm-manage';
import { type RuntimeParam } from '@cozeloop/api-schema/evaluation';
import { get } from '@cozeloop/adapter';
import { withField } from '@coze-arch/coze-design';

import { wait } from '@/utils';
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
  const [model, setModel] = useState<Model | undefined>();
  const modelRef = useLatest(model);

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
          onModelChange={setModel}
          promptVersion={promptDetail.prompt_commit?.commit_info?.version}
          onChange={onChange}
          rules={[
            {
              // 支持多模态的Prompt不能配置不支持多模态模型的校验
              asyncValidator: async (_, _val, callback) => {
                // 等待100ms，让onModelChange触发转状态变更，下面modelRe拿到最新的值
                await wait(100);
                const variables =
                  promptDetail.prompt_commit?.detail?.prompt_template
                    ?.variable_defs;
                const hasMultiModelVar = variables?.some(
                  variable =>
                    variable?.type && variable.type === VariableType.MultiPart,
                );
                if (
                  hasMultiModelVar &&
                  !modelRef.current?.ability?.multi_modal
                ) {
                  callback(I18n.t('model_not_support_multimodal'));
                }
                callback();
              },
            },
          ]}
        />
      ) : null}
    </DynamicParamsField>
  );
};
