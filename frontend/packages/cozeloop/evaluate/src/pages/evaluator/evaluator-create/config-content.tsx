// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { useLatest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
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
        label={I18n.t('evaluator_type')}
        field="evaluator_type"
        initValue={EvaluatorType.Prompt}
        fieldClassName="hidden"
      />
      <FormModelConfig
        refreshModelKey={refreshEditorModelKey}
        label={I18n.t('model_selection')}
        disabled={disabled}
        field="current_version.evaluator_content.prompt_evaluator.model_config"
        scenario={Scenario.scenario_evaluator}
        onModelChange={setModel}
        rules={[
          { required: true, message: I18n.t('choose_model') },
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
