// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ListEvaluatorsRequest } from '@cozeloop/api-schema/evaluation';

export type FilterParams = Pick<
  ListEvaluatorsRequest,
  'search_name' | 'creator_ids' | 'evaluator_type' | 'order_bys'
>;
