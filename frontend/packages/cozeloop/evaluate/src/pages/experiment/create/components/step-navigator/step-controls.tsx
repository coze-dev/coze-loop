import React from 'react';

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
              text: '最大并发执行条数',
              extra: (
                <Tooltip
                  content="实验支持并发执行评测集中的条目，但受限于评测对象的并发度和调用评估器的模型 TPM 限制。这里设置理想的最大执行条数。"
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
            最大并发执行条数最多支持 {EVAL_EXPERIMENT_CONCUR_COUNT_MAX} 条。
          </div>
        </div>

        <div>
          {currentStep > 0 && (
            <Button color="primary" onClick={onPrevious} className="mr-2">
              上一步
            </Button>
          )}
          {currentStepConfig.optional ? (
            <Button
              color="primary"
              onClick={() => onSkip?.()}
              disabled={isSkipDisabled}
            >
              跳过
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
