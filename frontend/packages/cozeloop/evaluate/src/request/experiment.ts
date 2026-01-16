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
  type ListExptInsightAnalysisRecordRequest,
  type ListExptInsightAnalysisRecordResponse,
  type InsightAnalysisExperimentRequest,
  type InsightAnalysisExperimentResponse,
  type GetExptInsightAnalysisRecordRequest,
  type GetExptInsightAnalysisRecordResponse,
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

// 洞察分析：获取记录列表
export async function listExptInsightAnalysisRecord(
  params: ListExptInsightAnalysisRecordRequest,
): Promise<ListExptInsightAnalysisRecordResponse> {
  return StoneEvaluationApi.ListExptInsightAnalysisRecord(params);
}

// 洞察分析：触发分析
export async function insightAnalysisExperiment(
  params: InsightAnalysisExperimentRequest,
): Promise<InsightAnalysisExperimentResponse> {
  return StoneEvaluationApi.InsightAnalysisExperiment(params);
}

// 洞察分析：获取单条记录详情
export async function getExptInsightAnalysisRecord(
  params: GetExptInsightAnalysisRecordRequest,
): Promise<GetExptInsightAnalysisRecordResponse> {
  return StoneEvaluationApi.GetExptInsightAnalysisRecord(params);
}
