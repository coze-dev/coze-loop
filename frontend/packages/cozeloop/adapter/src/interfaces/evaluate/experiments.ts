// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ComponentType } from 'react';

import { type prompt } from '@cozeloop/api-schema/prompt';
import { type Model } from '@cozeloop/api-schema/llm-manage';
import { type RuntimeParam } from '@cozeloop/api-schema/evaluation';
export interface EvaluateTargetPromptDynamicParamsProps {
  promptID?: string;
  promptVersion?: string;
  prompt?: prompt.Prompt;
  disabled?: boolean;
  value?: RuntimeParam;
  onChange?: (val?: RuntimeParam) => void;
  onModelChange?: (model?: Model) => void;
}

export interface EvaluateExperimentsAdapters {
  /**
   * 评测 Prompt 动态参数
   */
  EvaluateTargetPromptDynamicParams: ComponentType<EvaluateTargetPromptDynamicParamsProps>;
}
