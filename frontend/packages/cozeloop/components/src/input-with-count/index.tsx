// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { Input, type InputProps } from '@coze-arch/coze-design';

interface LimitCountProps {
  maxLen: number;
  len: number;
}
const LimitCount = ({ maxLen, len }: LimitCountProps) => (
  <span className="pl-2 pr-3 overflow-hidden text-xs coz-fg-secondary">
    <span>{len}</span>
    <span>/</span>
    <span>{maxLen}</span>
  </span>
);

export function InputWithCount(props: InputProps) {
  return (
    <Input
      {...props}
      suffix={
        Boolean(props.maxLength) && (
          <LimitCount
            maxLen={props.maxLength ?? 0}
            len={props.value?.toString().length ?? 0}
          />
        )
      }
    />
  );
}
