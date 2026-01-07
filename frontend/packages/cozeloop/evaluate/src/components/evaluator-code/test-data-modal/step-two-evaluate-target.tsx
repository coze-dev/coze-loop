// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

{
  /* start_aigc */
}
import { useCallback, useState } from 'react';

import {
  type EvalTargetDefinition,
  useEvalTargetDefinition,
} from '@cozeloop/evaluate-components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  ContentType,
  type EvalTargetType,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  Button,
  FormSelect,
  Toast,
  useFormState,
} from '@coze-arch/coze-design';

import type { ModalState, StepTwoEvaluateTargetProps } from '../types';

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

const StepTwoEvaluateTarget: React.FC<StepTwoEvaluateTargetProps> = ({
  formRef,
  onPrevStep,
  onNextStep,
  evaluationSetData,
}) => {
  const { spaceID } = useSpace();
  const [loading, setLoading] = useState<boolean>(false);
  const { getEvalTargetDefinitionList, getEvalTargetDefinition } =
    useEvalTargetDefinition();

  const formState = useFormState();

  const { values: formValues } = formState;

  const evalTargetTypeOptions = getEvalTargetDefinitionList()
    .filter(e => e.selector && !e?.disabledInCodeEvaluator)
    .map(eva => getOptionList(eva));

  const evalTargetDefinition = getEvalTargetDefinition?.(
    formValues.evalTargetType as string,
  );

  const handleEvalTargetTypeChange = (value: EvalTargetType) => {
    // 评测类型修改, 清空相关字段
    formRef.current?.formApi?.setValues({
      ...formValues,
      evalTargetType: value as EvalTargetType,
      evalTarget: undefined,
      evalTargetVersion: undefined,
    });
  };

  const handleOnFieldChange = useCallback(
    (key: string, value: unknown) => {
      if (key) {
        formRef.current?.formApi?.setValue(key as keyof ModalState, value);
      }
    },
    [formRef],
  );

  const geMockData = async () => {
    try {
      if (!formValues?.evalTarget || !formValues?.evalTargetVersion) {
        Toast.info({ content: '请选择评测对象和版本', top: 80 });
        return;
      }

      setLoading(true);
      const selectedItems = formValues?.selectedItems || new Set();
      const selectedData = evaluationSetData.filter(item =>
        selectedItems.has(item.item_id as string),
      );

      const mockResult = await StoneEvaluationApi.MockEvalTargetOutput({
        workspace_id: spaceID,
        source_target_id: formValues.evalTarget,
        target_type: formValues.evalTargetType,
        eval_target_version: formValues.evalTargetVersion,
      });

      const mockOutput = mockResult.mock_output;

      const transformData = selectedData.map(item => ({
        ext: {},
        evaluate_dataset_fields: item?.trunFieldData?.fieldDataMap || {},
        evaluate_target_output_fields: {
          actual_output: {
            key: 'actual_output',
            name: 'actual_output',
            content: {
              content_type: ContentType.Text,
              text: mockOutput?.actual_output,
              format: 1,
            },
          },
        },
      }));

      formRef.current?.formApi?.setValue('mockSetData', transformData);

      setLoading(false);
      onNextStep();
    } finally {
      setLoading(false);
    }
  };

  const targetType = formValues.evalTargetType;

  const TargetFormContent = evalTargetDefinition?.evalTargetFormSlotContent;

  return (
    <div className="h-[572px] flex flex-col">
      {/* 可滚动的内容区域 */}
      <div className="flex-1 overflow-y-auto pr-2">
        <div className="flex flex-col">
          {/* 使用标准的类型选择 */}
          <div>
            <FormSelect
              className="w-full"
              field="evalTargetType"
              label="类型"
              placeholder="请选择类型"
              optionList={evalTargetTypeOptions}
              showClear={true}
              onChange={value =>
                handleEvalTargetTypeChange(value as EvalTargetType)
              }
            />
          </div>

          {/* 根据类型渲染对应的表单内容 */}
          {targetType && TargetFormContent ? (
            <TargetFormContent
              formValues={formValues}
              createExperimentValues={formValues}
              onChange={handleOnFieldChange}
            />
          ) : null}
        </div>
      </div>

      {/* 固定在底部的操作按钮 */}
      <div className="flex-shrink-0 flex pt-4 ml-auto gap-1 border-t border-[var(--coz-border)]">
        <Button color="primary" onClick={onPrevStep} loading={loading}>
          上一步：评测集配置
        </Button>

        <Button onClick={geMockData} loading={loading}>
          下一步：生成模拟输出
        </Button>
      </div>
    </div>
  );
};

export default StepTwoEvaluateTarget;
{
  /* end_aigc */
}
