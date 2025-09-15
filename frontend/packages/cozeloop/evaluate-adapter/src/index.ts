import { type EvaluateAdapters } from '@cozeloop/adapter-interfaces/evaluate';

import { EvaluateTargetPromptDynamicParams } from './evaluate-target-prompt-dynamic-params';

// 创建符合 EvaluateAdapters 类型约束的导出对象
const evaluateAdapters: EvaluateAdapters = {
  EvaluateTargetPromptDynamicParams,
};

export default evaluateAdapters;

export { EvaluateTargetPromptDynamicParams };
