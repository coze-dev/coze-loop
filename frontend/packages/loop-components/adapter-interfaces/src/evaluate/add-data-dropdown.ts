// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type FC, type ReactNode } from 'react';

import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';

export type AddDataDropdownType = FC<{
  /** 评测集详情 */
  evaluationSet?: EvaluationSet;
  menuConfigs: Array<{
    label: string;
    icon: ReactNode;
    onClick: () => void;
  }>;
}>;
