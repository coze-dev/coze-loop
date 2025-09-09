// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type BatchGetExperimentsRequest,
  type BatchGetExperimentsResponse,
  type SubmitExperimentRequest,
  type SubmitExperimentResponse,
  type CheckExperimentNameRequest,
  type CheckExperimentNameResponse,
  type BatchGetExperimentAggrResultRequest,
  type BatchGetExperimentAggrResultResponse,
  type BatchGetExperimentResultRequest,
  type BatchGetExperimentResultResponse,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

export async function submitExperiment(
  params: SubmitExperimentRequest,
): Promise<SubmitExperimentResponse> {
  return StoneEvaluationApi.SubmitExperiment(params);
}

export async function batchGetExperiment(
  params: BatchGetExperimentsRequest,
): Promise<BatchGetExperimentsResponse> {
  return StoneEvaluationApi.BatchGetExperiments(params);
}

export async function checkExperimentName(
  params: CheckExperimentNameRequest,
): Promise<CheckExperimentNameResponse> {
  return StoneEvaluationApi.CheckExperimentName(params);
}

export async function batchGetExperimentAggrResult(
  params: BatchGetExperimentAggrResultRequest,
): Promise<BatchGetExperimentAggrResultResponse> {
  return StoneEvaluationApi.BatchGetExperimentAggrResult(params);
}

export async function batchGetExperimentResult(
  params: BatchGetExperimentResultRequest,
): Promise<BatchGetExperimentResultResponse> {
  return StoneEvaluationApi.BatchGetExperimentResult(params);
}
