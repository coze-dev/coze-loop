// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type GuardPoint, Guard } from '@cozeloop/guard';
import { EVAL_EXPERIMENT_CONCUR_COUNT_MAX } from '@cozeloop/biz-config-adapter';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Button, FormInputNumber, Tooltip } from '@coze-arch/coze-design';

import { type StepConfig } from '../../constants/steps';

interface StepControlsProps {
  currentStep: number;
  steps: StepConfig[];
  onNext: () => void;
  onPrevious: () => void;
  onSkip?: () => void;
  isSkipDisabled?: boolean;
  isNextLoading?: boolean;
}

export const StepControls: React.FC<StepControlsProps> = ({
  currentStep,
  steps,
  onNext,
  onPrevious,
  onSkip,
  isSkipDisabled = false,
  isNextLoading = false,
}) => {
  const currentStepConfig = steps[currentStep];

  return (
    <div className="flex-shrink-0 p-6">
      <div className="w-[800px] mx-auto flex flex-row items-center justify-between gap-2">
        <div className="flex items-center">
          <FormInputNumber
            labelPosition="left"
            initValue={5}
            label={{
              text: I18n.t('max_concurrent_execution_count'),
              extra: (
                <Tooltip
                  content={I18n.t(
                    'evaluate_experiment_concurrent_execution_limitations',
                  )}
                  theme="dark"
                >
                  <IconCozInfoCircle />
                </Tooltip>
              ),
            }}
            field="item_concur_num"
            className="w-[100px]"
            min={1}
            max={EVAL_EXPERIMENT_CONCUR_COUNT_MAX}
          />

          <div className="coz-fg-dim ml-2">
            {I18n.t('max_concurrent_execution_count_limit', {
              EVAL_EXPERIMENT_CONCUR_COUNT_MAX,
            })}
          </div>
        </div>

        <div>
          {currentStep > 0 && (
            <Button color="primary" onClick={onPrevious} className="mr-2">
              {I18n.t('prev_step')}
            </Button>
          )}
          {currentStepConfig.optional ? (
            <Button
              color="primary"
              onClick={() => onSkip?.()}
              disabled={isSkipDisabled}
            >
              {I18n.t('skip')}
            </Button>
          ) : null}

          <Guard
            point={currentStepConfig.guardPoint as GuardPoint}
            ignore={!currentStepConfig.isLast}
          >
            <Button onClick={onNext} loading={isNextLoading} className="ml-2">
              {currentStepConfig.nextStepText}
            </Button>
          </Guard>
        </div>
      </div>
    </div>
  );
};
