// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { QueryType } from '@cozeloop/api-schema/observation';
import { tag } from '@cozeloop/api-schema/data';

export const AUTO_EVAL_FEEDBACK_PREFIX = 'evaluator_version_';
export const AUTO_EVAL_FEEDBACK = 'feedback_auto_evaluator';
export const MANUAL_FEEDBACK_PREFIX = 'manual_feedback_';
export const MANUAL_FEEDBACK = 'feedback_manual';
export const API_FEEDBACK = 'feedback_openapi';
export const API_FEEDBACK_PREFIX = 'feedback_openapi_';
export const META_DATA_STRING = 'meta_data_string';
export const META_DATA_NUMBER = 'meta_data_number';
export const METADATA = 'metadata';

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

export enum FeedbackApiType {
  Category = 'category',
  Number = 'number',
  Boolean = 'boolean',
}

export const API_FEEDBACK_OPERATORS = {
  [FeedbackApiType.Category]: [
    QueryType.In,
    QueryType.not_In,
    QueryType.NotExist,
    QueryType.Exist,
  ],
  [FeedbackApiType.Number]: [
    QueryType.Lte,
    QueryType.Gte,
    QueryType.Eq,
    QueryType.NotExist,
    QueryType.Exist,
  ],
  [FeedbackApiType.Boolean]: [
    QueryType.In,
    QueryType.not_In,
    QueryType.NotExist,
    QueryType.Exist,
  ],
};

export enum MetadataType {
  String = 'string',
  Number = 'number',
}

export const METADATA_OPERATORS = {
  [MetadataType.String]: [QueryType.Match, QueryType.Exist, QueryType.NotExist],
  [MetadataType.Number]: [
    QueryType.Gte,
    QueryType.Lte,
    QueryType.Exist,
    QueryType.NotExist,
  ],
};
