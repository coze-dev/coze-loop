/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useCallback, useRef, useState } from 'react';

import { type EvaluationSetItemTableData } from '@cozeloop/evaluate-components';
import {
  type FieldData,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { Modal, Form } from '@coze-arch/coze-design';

import { StepVisibleWrapper } from '@/pages/experiment/create/components/step-visible-wrapper';

import type { ModalState, TestDataItem, TestDataModalProps } from '../types';
import { StepIndicator } from '../../../pages/experiment/create/components/step-navigator/step-indicator';
import StepTwoEvaluateTarget from './step-two-evaluate-target';
import StepThreeGenerateOutput from './step-three-generate-output';
import StepOneEvaluateSet from './step-one-evaluate-set';

import styles from './index.module.less';

// start_aigc
/**
 * 转换测试数据格式，将复杂的嵌套结构简化为只包含 content_type 和 text 的格式
 */
const transformTestDataItem = (item: TestDataItem) => {
  // 转换 evaluate_dataset_fields
  const fromEvalSetFields = item?.evaluate_dataset_fields || {};
  const transformedFromEvalSetFields: Record<string, FieldData> = Object.keys(
    fromEvalSetFields,
  ).reduce(
    (acc, key) => {
      const field = fromEvalSetFields[key];
      const content = field?.content;
      if (content) {
        acc[key] = {
          content_type: content.content_type,
          text: content.text,
        };
        if (content?.multi_part) {
          acc[key].multi_part = content.multi_part;
        }
      }
      return acc;
    },
    {} satisfies Record<string, FieldData>,
  );

  // 转换 evaluate_target_output_fields
  const fromEvalTargetFields = item?.evaluate_target_output_fields || {};
  const transformedFromEvalTargetFields: Record<string, FieldData> =
    Object.keys(fromEvalTargetFields).reduce(
      (acc, key) => {
        const field = fromEvalTargetFields[key];
        const content = field?.content;
        if (content) {
          acc[key] = {
            content_type: content.content_type,
            text: content.text,
          };
          if (content?.multi_part) {
            acc[key].multi_part = content.multi_part;
          }
        }
        return acc;
      },
      {} satisfies Record<string, FieldData>,
    );

  return {
    evaluate_dataset_fields: transformedFromEvalSetFields,
    evaluate_target_output_fields: transformedFromEvalTargetFields,
    ext: item.ext || {},
  };
};
// end_aigc

const steps = [
  { title: '评测集', guardPoint: '' },
  { title: '评测对象', guardPoint: '' },
  { title: '生成模拟输出', guardPoint: '' },
];

const TestDataModal: React.FC<TestDataModalProps> = ({
  visible,
  onClose,
  onImport,
  prevCount,
}) => {
  const formRef = useRef<Form<ModalState>>(null);
  const [localStep, setLocalStep] = useState<number>(0);
  const [fieldSchemas, setFieldSchemas] = useState<FieldSchema[]>([]);

  const [evaluationSetData, setEvaluationSetData] = useState<
    EvaluationSetItemTableData[]
  >([]);

  const resetModal = useCallback(() => {
    formRef.current?.formApi?.reset();
    formRef.current?.formApi?.setValues({
      currentStep: 0,
      selectedItems: undefined,
    });
    setEvaluationSetData([]);
    setLocalStep(0);
  }, []);

  const handleClose = useCallback(() => {
    resetModal();
    onClose();
  }, [resetModal, onClose]);

  const handlePrevStep = useCallback(() => {
    const formApi = formRef.current?.formApi;
    const currentStep = formApi?.getValue('currentStep') || 0;
    if (currentStep > 0) {
      const newStep = currentStep - 1;
      formApi?.setValue('currentStep', newStep);
      setLocalStep(newStep);
    }
  }, []);

  const handleNextStep = useCallback(() => {
    const formApi = formRef.current?.formApi;
    const currentStep = formApi?.getValue('currentStep') || 0;
    if (currentStep < 2) {
      const newStep = currentStep + 1;
      formApi?.setValue('currentStep', newStep);
      setLocalStep(newStep);
    }
  }, []);

  const handleImport = useCallback(
    (
      data: TestDataItem[],
      originSelectedData?: EvaluationSetItemTableData[],
    ) => {
      const importPayload = data.map(transformTestDataItem);
      onImport(importPayload, originSelectedData);
      resetModal();
    },
    [onImport, resetModal],
  );

  return (
    <Modal
      className={styles.evalSetTestDataModal}
      title="构造测试数据"
      visible={visible}
      onCancel={handleClose}
      hasScroll={false}
      width={1120}
      footer={null}
    >
      <StepIndicator steps={steps} currentStep={localStep} />
      <Form ref={formRef} initValues={{ currentStep: 0 }}>
        {/* 使用复用的步骤指示器 */}

        {/* 步骤内容 */}
        <StepVisibleWrapper visible={localStep === 0}>
          <StepOneEvaluateSet
            formRef={formRef}
            fieldSchemas={fieldSchemas}
            setFieldSchemas={setFieldSchemas}
            evaluationSetData={evaluationSetData}
            setEvaluationSetData={setEvaluationSetData}
            onImport={handleImport}
            onNextStep={handleNextStep}
            prevCount={prevCount}
          />
        </StepVisibleWrapper>
        <StepVisibleWrapper visible={localStep === 1}>
          <StepTwoEvaluateTarget
            formRef={formRef}
            evaluationSetData={evaluationSetData}
            fieldSchemas={fieldSchemas}
            setFieldSchemas={setFieldSchemas}
            onPrevStep={handlePrevStep}
            onNextStep={handleNextStep}
          />
        </StepVisibleWrapper>
        <StepVisibleWrapper visible={localStep === 2}>
          <StepThreeGenerateOutput
            formRef={formRef}
            fieldSchemas={fieldSchemas}
            onPrevStep={handlePrevStep}
            onImport={handleImport}
            evaluationSetData={evaluationSetData}
            setEvaluationSetData={setEvaluationSetData}
          />
        </StepVisibleWrapper>
      </Form>
    </Modal>
  );
};

export default TestDataModal;
