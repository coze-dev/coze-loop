// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWithDisabled } from '@cozeloop/components';
import {
  IconCozCheckMarkFill,
  IconCozQuestionMarkCircle,
} from '@coze-arch/coze-design/icons';
import { Tag } from '@coze-arch/coze-design';

import { type StepConfig } from '../../constants/steps';

interface StepIndicatorProps {
  steps: StepConfig[];
  currentStep: number;
}

export const StepIndicator: React.FC<StepIndicatorProps> = ({
  steps,
  currentStep,
}) => (
  <div className="flex-shrink-0 m-6 mt-[12px] mb-0 flex flex-row items-center justify-center gap-9">
    {steps.map((item, index) => {
      const isCurrent = index === currentStep;
      const isDone = index < currentStep;

      if (item.hiddenStepBar) {
        return null;
      }

      return (
        <div
          className="flex flex-row items-center gap-2 text-[16px] font-medium"
          key={index}
        >
          <div
            className={classNames(
              'w-6 h-6 rounded-full flex items-center justify-center',
              isDone
                ? 'coz-mg-hglt coz-fg-hglt'
                : isCurrent
                  ? 'coz-mg-hglt-plus coz-fg-hglt-plus'
                  : 'coz-mg-primary coz-fg-secondary',
            )}
          >
            {isDone ? (
              <IconCozCheckMarkFill className="w-4 h-4 coz-fg-hglt" />
            ) : (
              index + 1
            )}
          </div>
          <div className={isCurrent ? 'text-brand-9' : 'coz-fg-secondary'}>
            {item.title}
          </div>
          {item.optional ? (
            <TooltipWithDisabled
              content={item.tooltip}
              disabled={!item.tooltip}
              theme="dark"
            >
              <Tag color="grey" suffixIcon={<IconCozQuestionMarkCircle />}>
                {I18n.t('optional')}
              </Tag>
            </TooltipWithDisabled>
          ) : null}
        </div>
      );
    })}
  </div>
);
