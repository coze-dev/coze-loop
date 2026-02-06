// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type ListTemplatesV2Request,
  type ListTemplatesV2Response,
  type ListEvaluatorTagsRequest,
  type ListEvaluatorTagsResponse,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

export async function listTemplatesV2(
  params: ListTemplatesV2Request,
): Promise<ListTemplatesV2Response> {
  return StoneEvaluationApi.ListTemplatesV2(params);
}

export async function listEvaluatorTags(
  params: ListEvaluatorTagsRequest,
): Promise<ListEvaluatorTagsResponse> {
  return StoneEvaluationApi.ListEvaluatorTags(params);
}
