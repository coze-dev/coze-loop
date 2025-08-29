import { type ListEvaluatorsRequest } from '@cozeloop/api-schema/evaluation';

export type FilterParams = Pick<
  ListEvaluatorsRequest,
  'search_name' | 'creator_ids' | 'evaluator_type' | 'order_bys'
>;
