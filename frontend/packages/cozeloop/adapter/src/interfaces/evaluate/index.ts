import { type EvaluateExperimentsAdapters } from './experiments';
import { type EvaluateEvaluatorsAdapters } from './evaluators';
import { type EvaluateDatasetsAdapters } from './datasets';

export interface EvaluateAdapters {
  'eval.experiments': EvaluateExperimentsAdapters;
  'eval.datasets': EvaluateDatasetsAdapters;
  'eval.evaluators': EvaluateEvaluatorsAdapters;
}

export { type EvaluateTargetPromptDynamicParamsProps } from './experiments';
