import { useState } from 'react';

import { useLatest } from 'ahooks';
import {
  EvaluateModelConfigEditor,
  OutputInfo,
  wait,
} from '@cozeloop/evaluate-components';
import { Scenario, type Model } from '@cozeloop/api-schema/llm-manage';
import { type Evaluator, EvaluatorType } from '@cozeloop/api-schema/evaluation';
import { FormSelect, useFormState, withField } from '@coze-arch/coze-design';

import { multiModelValidate } from './validate-rules';
import { PromptField } from './prompt-field';

const FormModelConfig = withField(EvaluateModelConfigEditor);

export function ConfigContent({
  refreshEditorModelKey,
  disabled,
}: {
  refreshEditorModelKey?: number;
  disabled?: boolean;
}) {
  const formState = useFormState<Evaluator>();
  const [model, setModel] = useState<Model | undefined>();
  const modelRef = useLatest(model);
  const multiModalVariableEnable =
    model?.ability?.multi_modal === true && !disabled;

  return (
    <>
      <FormSelect
        label="评估器类型"
        field="evaluator_type"
        initValue={EvaluatorType.Prompt}
        fieldClassName="hidden"
      />
      <FormModelConfig
        refreshModelKey={refreshEditorModelKey}
        label="模型选择"
        disabled={disabled}
        field="current_version.evaluator_content.prompt_evaluator.model_config"
        scenario={Scenario.scenario_evaluator}
        onModelChange={setModel}
        rules={[
          { required: true, message: '请选择模型' },
          {
            asyncValidator: async (_, _val, callback) => {
              await wait(100);
              const messages =
                formState.values?.current_version?.evaluator_content
                  ?.prompt_evaluator?.message_list ?? [];
              const res = multiModelValidate(messages, modelRef.current);
              callback(res);
            },
          },
        ]}
      />
      <PromptField
        disabled={disabled}
        refreshEditorKey={refreshEditorModelKey}
        multiModalVariableEnable={multiModalVariableEnable}
      />
      <OutputInfo />
    </>
  );
}
