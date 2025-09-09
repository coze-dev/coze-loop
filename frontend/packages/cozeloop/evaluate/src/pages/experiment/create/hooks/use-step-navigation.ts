// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention */
import { useState } from 'react';

export interface UseStepNavigationOptions {
  initialStep?: number;
  // 预留接口
  onStepChange?: (newStep: number, prevStep: number) => void;
  onStepComplete?: (step: number) => void;
}

// 单纯用于实验创建表单的业务步骤处理, 需要抽取公共逻辑再抽象
export const useStepNavigation = ({
  initialStep = 0,
}: UseStepNavigationOptions = {}) => {
  const [step, _setStep] = useState(initialStep);

  const setStep = (newStep: number) => {
    _setStep(newStep);
  };

  const goNext = () => {
    const nextStep = step + 1;
    setStep(nextStep);
  };

  const goPrevious = () => {
    const prevStep = step - 1;
    setStep(prevStep);
  };

  return {
    step,
    setStep,
    goNext,
    goPrevious,
  };
};
