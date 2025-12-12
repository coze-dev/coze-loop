import { UserInputSpanDefinition } from './user-input';
import { RetrieverSpanDefinition } from './retriever';
import { QuerySpanDefinition } from './query-span';
import { PromptSpanDefinition } from './prompt';
import { ModelSpanDefinition } from './model';
import { DefaultSpanDefinition } from './default';

const modelSpanDefinition = new ModelSpanDefinition();
const promptSpanDefinition = new PromptSpanDefinition();
export const defaultSpanDefinition = new DefaultSpanDefinition();
export const retrieverSpanDefinition = new RetrieverSpanDefinition();
export const querySpanDefinition = new QuerySpanDefinition();
export const userInputSpanDefinition = new UserInputSpanDefinition();

export const BUILT_IN_SPAN_DEFINITIONS = [
  modelSpanDefinition,
  promptSpanDefinition,
  defaultSpanDefinition,
  retrieverSpanDefinition,
  querySpanDefinition,
  userInputSpanDefinition,
];
