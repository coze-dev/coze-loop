// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type EvalTargetDefinition,
  useEvalTargetDefinition,
} from '@cozeloop/evaluate-components';
import { type EvalTargetType } from '@cozeloop/api-schema/evaluation';
import { type Form, FormSelect, useFormState } from '@coze-arch/coze-design';

import { type CreateExperimentValues } from '@/types/experiment/experiment-create';

import { evaluateTargetValidators } from '../validators/evaluate-target';

export interface EvaluateTargetFormProps {
  formRef: React.RefObject<Form<CreateExperimentValues>>;
  createExperimentValues: CreateExperimentValues;
  setCreateExperimentValues: React.Dispatch<
    React.SetStateAction<CreateExperimentValues>
  >;
}

const getOptionList = (option: EvalTargetDefinition) => {
  const { name, type, description } = option;
  if (!description) {
    return {
      label: name,
      value: type,
    };
  }

  return {
    label: (
      <div className="flex">
        <div className="mr-1.5 option-text self-center">{name}</div>
        <div className="text-[13px] font-normal text-[var(--coz-fg-secondary)]">
          {description}
        </div>
      </div>
    ),

    value: type,
  };
};

export const EvaluateTargetForm = (props: EvaluateTargetFormProps) => {
  const { formRef, createExperimentValues, setCreateExperimentValues } = props;
  const formState = useFormState();

  const { values: formValues } = formState;

  const { getEvalTargetDefinitionList, getEvalTargetDefinition } =
    useEvalTargetDefinition();

  const pluginEvaluatorList = getEvalTargetDefinitionList();

  const evalTargetTypeOptions = pluginEvaluatorList
    .filter(e => e.selector)
    .map(eva => getOptionList(eva));

  const currentEvaluator = getEvalTargetDefinition?.(
    formValues.evalTargetType as string,
  );

  const handleEvalTargetTypeChange = (v: EvalTargetType) => {
    // 评测类型修改, 清空相关字段
    formRef.current?.formApi?.setValues({
      ...formValues,
      evalTargetType: v as EvalTargetType,
      evalTarget: undefined,
      evalTargetVersion: undefined,
      evalTargetMapping: undefined,
    });
    setCreateExperimentValues(prev => ({
      ...prev,
      evalTargetType: v,
      evalTarget: undefined,
      evalTargetVersion: undefined,
      evalTargetMapping: undefined,
    }));
  };

  const targetType = formValues.evalTargetType;

  const TargetFormContent = currentEvaluator?.evalTargetFormSlotContent;

  const handleOnFieldChange = (
    key: keyof CreateExperimentValues,
    value: unknown,
  ) => {
    if (key) {
      formRef.current?.formApi?.setValue(key, value);
    }
  };

  return (
    <>
      <FormSelect
        fieldClassName="evaluate-eval-target-type-form-select"
        className="w-full"
        field="evalTargetType"
        label={I18n.t('type')}
        placeholder={I18n.t('select_type')}
        optionList={evalTargetTypeOptions}
        showClear={true}
        rules={evaluateTargetValidators.evalTargetType}
        onChange={v => handleEvalTargetTypeChange(v as EvalTargetType)}
      />

      {targetType ? (
        <>
          {TargetFormContent ? (
            <TargetFormContent
              formValues={formState.values}
              createExperimentValues={createExperimentValues}
              onChange={handleOnFieldChange}
              setCreateExperimentValues={setCreateExperimentValues}
            />
          ) : null}
        </>
      ) : null}
    </>
  );
};
