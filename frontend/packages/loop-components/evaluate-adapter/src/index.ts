// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type EvaluateAdapters } from '@cozeloop/adapter-interfaces/evaluate';

import { EvaluateTargetPromptDynamicParams } from './evaluate-target-prompt-dynamic-params';

// 创建符合 EvaluateAdapters 类型约束的导出对象
const evaluateAdapters: EvaluateAdapters = {
  EvaluateTargetPromptDynamicParams,
};

export default evaluateAdapters;

export { EvaluateTargetPromptDynamicParams };

export { EVALUATE_MULTIPART_DATA_ABILITY_CONFIG } from './constants';
