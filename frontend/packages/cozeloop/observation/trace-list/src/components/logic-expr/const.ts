// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { QueryType } from '@cozeloop/api-schema/observation';
import { tag } from '@cozeloop/api-schema/data';

export const AUTO_EVAL_FEEDBACK_PREFIX = 'evaluator_version_';
export const AUTO_EVAL_FEEDBACK = 'feedback_auto_evaluator';
export const MANUAL_FEEDBACK_PREFIX = 'manual_feedback_';
export const MANUAL_FEEDBACK = 'feedback_manual';
const { TagContentType } = tag;

export const MANUAL_FEEDBACK_OPERATORS = {
  [TagContentType.Boolean]: [QueryType.In, QueryType.not_In],
  [TagContentType.Categorical]: [QueryType.In, QueryType.not_In],
  [TagContentType.FreeText]: [QueryType.Exist, QueryType.Match],
  [TagContentType.ContinuousNumber]: [
    QueryType.Lte,
    QueryType.Gte,
    QueryType.Lt,
    QueryType.Gt,
    QueryType.Exist,
  ],
};
