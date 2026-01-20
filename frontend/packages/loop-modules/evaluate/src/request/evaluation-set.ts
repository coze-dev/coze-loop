// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type GetEvaluationSetVersionRequest,
  type GetEvaluationSetVersionResponse,
  type GetEvaluationSetRequest,
  type GetEvaluationSetResponse,
  type ListEvaluationSetsRequest,
  type ListEvaluationSetsResponse,
  type ListEvaluationSetVersionsRequest,
  type ListEvaluationSetVersionsResponse,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

export async function getEvaluationSetVersion(
  params: GetEvaluationSetVersionRequest,
): Promise<GetEvaluationSetVersionResponse> {
  return StoneEvaluationApi.GetEvaluationSetVersion(params);
}

export async function listEvaluationSets(
  params: ListEvaluationSetsRequest,
): Promise<ListEvaluationSetsResponse> {
  return StoneEvaluationApi.ListEvaluationSets(params);
}

export async function listEvaluationSetVersions(
  params: ListEvaluationSetVersionsRequest,
): Promise<ListEvaluationSetVersionsResponse> {
  return StoneEvaluationApi.ListEvaluationSetVersions(params);
}

export async function getEvaluationSet(
  params: GetEvaluationSetRequest,
): Promise<GetEvaluationSetResponse> {
  return StoneEvaluationApi.GetEvaluationSet(params);
}
