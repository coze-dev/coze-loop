/* eslint-disable @typescript-eslint/naming-convention -- skip */
export type { ApiOption, ApiResponse } from './api/config';
export * from './api/idl';

export { $notification, type ApiBizErrorEvent } from './notification';

import {
  evaluationEvalSet,
  evaluationEvalTarget,
  evaluationEvaluator,
  evaluationExpt,
  dataDataset,
  llmManage,
  foundationUpload,
  promptManage,
  promptDebug,
  foundationAuthn,
  foundationUser,
  foundationSpace,
  dateTag,
} from './api/idl';

export const StoneEvaluationApi = {
  ...evaluationEvalSet,
  ...evaluationEvalTarget,
  ...evaluationEvaluator,
  ...evaluationExpt,
};

export const DataApi = {
  ...dataDataset,
  ...dateTag,
};

export const LlmManageApi = {
  ...llmManage,
};
export const FoundationApi = {
  ...foundationUpload,
  ...foundationAuthn,
  ...foundationUser,
  ...foundationSpace,
};

export const StonePromptApi = {
  ...promptManage,
  ...promptDebug,
};
