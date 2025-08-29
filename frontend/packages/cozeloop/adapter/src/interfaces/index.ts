import { type PEAdapters } from './pe';
import { type ObservationAdapters } from './observation';
import { type EvaluateAdapters } from './evaluate';

export type Adapters = PEAdapters & EvaluateAdapters & ObservationAdapters;

export { type EvaluateTargetPromptDynamicParamsProps } from './evaluate';
