// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */

import { ErrorBoundary } from 'react-error-boundary';
import { useState, type MutableRefObject } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  ExptCreateFormCtx,
  ExtCreateStep,
  useEvalTargetDefinition,
} from '@cozeloop/evaluate-components';
import { RouteBackAction, PageError } from '@cozeloop/components';
import { useNavigateModule, useSpace } from '@cozeloop/biz-hooks-adapter';
import { Form, Spin } from '@coze-arch/coze-design';

import { submitExperiment } from '@/request/experiment';

import {
  calcNextStepRenderValue,
  getCurrentTime,
  getSubmitValues,
  getValidateFields,
} from './tools';
import { useStepNavigation } from './hooks/use-step-navigation';
import { useLeaveGuard } from './hooks/use-leave-guard';
import { useFormData } from './hooks/use-form-data';
import { useExptPageInit } from './hooks/use-expt-page-init';
import { stepNameMap, STEPS } from './constants/steps';
import { ViewSubmitForm } from './components/view-submit-form';
import { StepVisibleWrapper } from './components/step-visible-wrapper';
import { StepIndicator } from './components/step-navigator/step-indicator';
import { StepControls } from './components/step-navigator/step-controls';
import { EvaluatorForm } from './components/evaluator-form';
import { EvaluateTargetForm } from './components/evaluate-target-form';
import { EvaluateSetForm } from './components/evaluate-set-form';
import { BaseInfoForm } from './components/base-info-form';

const reportStep = (params: {
  newTimeRef: MutableRefObject<number>;
  step: number;
  copyExperimentID?: string;
}) => {
  const { newTimeRef, step, copyExperimentID } = params;
  const duration = getCurrentTime() - newTimeRef.current;
  sendEvent(EVENT_NAMES.cozeloop_experiment_configure, {
    click_step: stepNameMap[step][0],
    duration,
  });
  sendEvent(EVENT_NAMES.cozeloop_experiment_create_step_cost, {
    step: stepNameMap[step][1],
    method: copyExperimentID ? 'copy' : 'create',
    duration,
  });

  newTimeRef.current = getCurrentTime();
};

const BackComponent = () => (
  <div className="px-6 py-3 h-[56px] flex-shrink-0 flex flex-row items-center">
    <RouteBackAction defaultModuleRoute="evaluation/experiments" />
    <span className="ml-2 text-[18px] font-medium coz-fg-plus">
      {I18n.t('new_experiment')}
    </span>
  </div>
);

export default function ExperimentCreatePage() {
  const { spaceID } = useSpace();

  const { step, goNext, goPrevious } = useStepNavigation();
  const navigateModule = useNavigateModule();

  // 计时初始化 + 面包屑 + 插件初始化获取
  const { searchParams, startTimeRef, newTimeRef } = useExptPageInit();

  const [nextStepLoading, setNextStepLoading] = useState(false);

  const copyExperimentID = searchParams.get('copy_experiment_id');
  const evaluationSetID = searchParams.get('evaluation_set_id');
  const evaluationSetVersionID = searchParams.get('version_id');

  const { getEvalTargetDefinition } = useEvalTargetDefinition();

  const {
    formRef,
    initLoading,
    formData: createExperimentValues,
    setFormData: setCreateExperimentValues,
  } = useFormData({
    spaceID,
    copyExperimentID: copyExperimentID || undefined,
    evaluationSetID: evaluationSetID || undefined,
    evaluationSetVersionID: evaluationSetVersionID || undefined,
    initialData: {
      workspace_id: spaceID,
    },
  });

  // 页面离开保护
  const { setBlockLeave } = useLeaveGuard();

  // 点击上一步
  const handleOnClickPreStep = () => {
    // 保存当前步骤的值
    if (formRef?.current?.formApi) {
      const currentValues = formRef.current.formApi.getValues();
      const nextStepValues = calcNextStepRenderValue(
        createExperimentValues,
        currentValues,
      );
      setCreateExperimentValues(nextStepValues);
    }

    reportStep({
      newTimeRef,
      step,
      copyExperimentID: copyExperimentID || '',
    });
    goPrevious();
  };
  // 提交表单
  const handleSubmit = async () => {
    try {
      setNextStepLoading(true);
      const filteredValues = getSubmitValues(createExperimentValues);
      // transformCreateValues
      const currentEvaluatorForValidation = getEvalTargetDefinition?.(
        createExperimentValues.evalTargetType as string,
      );
      // 用户有自定义转换
      const transform = currentEvaluatorForValidation?.transformCreateValues;
      const payload = transform ? transform(filteredValues) : filteredValues;
      const res = await submitExperiment({
        ...payload,
        workspace_id: spaceID,
      });
      const experimentId = res.experiment?.id;
      if (!experimentId) {
        throw new Error('experimentId is undefined');
      }
      setBlockLeave(false);
      setTimeout(() => {
        sendEvent(EVENT_NAMES.cozeloop_experiment_create_total_cost, {
          duration: getCurrentTime() - startTimeRef.current,
          method: copyExperimentID ? 'copy' : 'create',
        });

        navigateModule(`evaluation/experiments/${experimentId}`, {
          replace: true,
        });
      }, 100);
    } catch (e) {
      console.error(I18n.t('submit_form_problems'), e);
    } finally {
      setNextStepLoading(false);
    }
  };

  // 点击下一步, 校验并进行一些字段转换
  const handleOnClickNextStep = async () => {
    if (formRef.current?.formApi) {
      if (step === ExtCreateStep.CREATE_EXPERIMENT) {
        // 提交表单
        await handleSubmit();
      } else {
        // 因为 evalTargetType 在当前表单不会马上存到 createExperimentValues中, 所以需要每次获取
        const formValues = formRef.current.formApi.getValues();
        const currentEvaluatorForValidation = getEvalTargetDefinition?.(
          formValues.evalTargetType as string,
        );
        // 数组或回调函数
        const extraValidFields =
          currentEvaluatorForValidation?.extraValidFields?.[step];

        const validateFields = getValidateFields({
          currentStep: step,
          extraFields: extraValidFields,
          values: formValues,
        });

        let values;
        try {
          values = await formRef.current.formApi
            .validate(validateFields)
            .catch(e => console.warn(e));
        } catch (e) {
          setNextStepLoading(false);
          console.error('xxx 遇到问题e', e);
        }
        // 普通下一步

        if (values) {
          // 更新全局状态，确保包含最新的表单值
          setCreateExperimentValues(prev => ({
            ...prev,
            ...values,
          }));
          // 设置下一步
          goNext();
        }
      }
    }
    // 上报 & 更新时间戳
    reportStep({
      newTimeRef,
      step,
      copyExperimentID: copyExperimentID || '',
    });
  };

  const handleOnClickSkip = () => {
    goNext();
  };

  return (
    <div className="h-full overflow-hidden flex flex-col">
      <ErrorBoundary fallback={<PageError />}>
        <ExptCreateFormCtx.Provider
          value={{ nextStepLoading, setNextStepLoading }}
        >
          <BackComponent />
          {/* 步骤指示器 */}
          <StepIndicator steps={STEPS} currentStep={step} />
          <Form
            ref={formRef}
            className="flex-1 min-h-0 flex flex-col"
            onValueChange={v => {
              setBlockLeave(true);
            }}
          >
            {({ formState }) => (
              <>
                <div className="flex-1 overflow-y-auto p-6 pt-[20px] styled-scrollbar pr-[18px]">
                  <div className="flex-1 w-[800px] mx-auto">
                    {initLoading ? (
                      <Spin spinning={true} wrapperClassName="w-full h-96" />
                    ) : (
                      <>
                        {/* 基础信息 */}
                        <StepVisibleWrapper
                          visible={step === ExtCreateStep.BASE_INFO}
                        >
                          <BaseInfoForm />
                        </StepVisibleWrapper>
                        {/* 评测集 */}
                        <StepVisibleWrapper
                          visible={step === ExtCreateStep.EVAL_SET}
                        >
                          <EvaluateSetForm
                            formRef={formRef}
                            createExperimentValues={createExperimentValues}
                            setCreateExperimentValues={
                              setCreateExperimentValues
                            }
                            setNextStepLoading={setNextStepLoading}
                          />
                        </StepVisibleWrapper>
                        {/* 评测对象 */}
                        <StepVisibleWrapper
                          visible={step === ExtCreateStep.EVAL_TARGET}
                        >
                          <EvaluateTargetForm
                            formRef={formRef}
                            createExperimentValues={createExperimentValues}
                            setCreateExperimentValues={
                              setCreateExperimentValues
                            }
                          />
                        </StepVisibleWrapper>
                        {/* 评估器 */}
                        <StepVisibleWrapper
                          visible={step === ExtCreateStep.EVALUATOR}
                        >
                          <EvaluatorForm
                            initValue={
                              createExperimentValues.evaluatorProList || []
                            }
                            evaluationSetVersionDetail={
                              createExperimentValues?.evaluationSetVersionDetail ||
                              {}
                            }
                            evalTargetVersionDetail={
                              createExperimentValues?.evalTargetVersionDetail ||
                              {}
                            }
                          />
                        </StepVisibleWrapper>
                        {/* 创建实验，预览里仅展示没有可修改表单值的表单项，仅在该步骤时再渲染即可  */}
                        {step === ExtCreateStep.CREATE_EXPERIMENT ? (
                          <ViewSubmitForm
                            createExperimentValues={createExperimentValues}
                          />
                        ) : null}
                      </>
                    )}
                  </div>
                </div>
                <StepControls
                  currentStep={step}
                  steps={STEPS}
                  onNext={handleOnClickNextStep}
                  onPrevious={handleOnClickPreStep}
                  onSkip={handleOnClickSkip}
                  isSkipDisabled={
                    (step === ExtCreateStep.EVAL_TARGET &&
                      Boolean(createExperimentValues?.evalTargetType)) ||
                    (step === ExtCreateStep.EVALUATOR &&
                      Boolean(formState.values.evaluatorProList?.length))
                  }
                  isNextLoading={nextStepLoading}
                />
              </>
            )}
          </Form>
        </ExptCreateFormCtx.Provider>
      </ErrorBoundary>
    </div>
  );
}
