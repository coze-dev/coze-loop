// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { IconCozCheckMarkFill } from '@coze-arch/coze-design/icons';
import { Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

interface StepNavProps {
  currentStep: string;
  stepItems: {
    key: string;
    label: string;
    icon?: React.ReactNode;
  }[];
  onStepChange?: (step: string) => void;
  clickToChange?: boolean;
}

export function StepNav({
  currentStep,
  stepItems,
  onStepChange,
  clickToChange,
}: StepNavProps) {
  const activeStepIndex = stepItems.findIndex(item => item.key === currentStep);
  return (
    <div className={styles['step-nav']}>
      {stepItems.map((item, index) => (
        <Typography.Text
          key={item.key}
          className={classNames(styles['tab-step'], {
            [styles['tab-active']]:
              currentStep === item.key || index <= activeStepIndex,
            'cursor-pointer': clickToChange,
          })}
          icon={
            index < activeStepIndex ? (
              <span className={styles['tab-icon']}>
                <IconCozCheckMarkFill />
              </span>
            ) : (
              <span className={styles['tab-icon']}>
                {item?.icon ?? index + 1}
              </span>
            )
          }
          onClick={() => clickToChange && onStepChange?.(item.key)}
        >
          {item.label}
        </Typography.Text>
      ))}
    </div>
  );
}
