// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import classNames from 'classnames';
import {
  type RadioGroupProps,
  RadioGroup as SemiRadioGroup,
} from '@coze-arch/coze-design';

export function LoopRadioGroup({ className, ...props }: RadioGroupProps) {
  return (
    <SemiRadioGroup
      className={classNames('!bg-semi-fill-0 !p-0', className)}
      type="button"
      {...props}
    />
  );
}
