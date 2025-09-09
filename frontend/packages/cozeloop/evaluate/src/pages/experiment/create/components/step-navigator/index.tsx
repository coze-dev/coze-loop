// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type ReactNode } from 'react';

import { StepIndicator } from './step-indicator';
import { StepControls } from './step-controls';

export interface StepNavigatorProps {
  steps: string[];
  currentStep: number;
  onNext: () => void;
  onPrevious: () => void;
  isNextLoading?: boolean;
  children: ReactNode;
}

export const StepNavigator = ({
  steps,
  currentStep,
  onNext,
  onPrevious,
  isNextLoading = false,
  children,
}: StepNavigatorProps) => (
  <div className="h-full overflow-hidden flex flex-col">
    {/* 步骤指示器 */}
    <StepIndicator steps={steps as any} currentStep={currentStep} />

    {/* 表单内容 */}
    <div className="flex-1 overflow-y-auto p-6 pt-[20px] styled-scrollbar pr-[18px]">
      <div className="flex-1 w-[800px] mx-auto">{children}</div>
    </div>

    {/* 步骤控制器 */}
    <StepControls
      currentStep={currentStep}
      steps={steps as any}
      onNext={onNext}
      onPrevious={onPrevious}
      isNextLoading={isNextLoading}
    />
  </div>
);
