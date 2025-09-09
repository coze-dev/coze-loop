// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type AggregateData,
  type EvaluatorAggregateResult,
  type BatchGetExperimentAggrResultRequest,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
/**
 * 对比试验中所有评估器聚合指标数据
 * - 第一层map_key为评估器版本id
 * - 第二层map_key为统计方式（平均分、最高分等）-
 * - 第三层map_key为实验id
 */
export type EvaluatorAggregateResultMap = Record<
  Int64,
  Record<Int64, Record<Int64, AggregateData | undefined>>
>;

/**
 * 计算对比试验中所有评估器聚合指标数据
 * - 第一层map_key为评估器版本id
 * - 第二层map_key为统计方式（平均分、最高分等）-
 * - 第三层map_key为实验id
 */
function getEvaluatorAggregateResultMap(
  resultMap: EvaluatorAggregateResultMap,
  results: EvaluatorAggregateResult[],
  experimentId: Int64,
) {
  results.forEach(result => {
    const versionId = result.evaluator_version_id ?? '';
    if (!resultMap[versionId]) {
      resultMap[versionId] = {};
    }
    const evaluatorResultMap = resultMap[versionId];

    result.aggregator_results?.forEach(aggregatorResult => {
      const scoreType = aggregatorResult.aggregator_type ?? '';
      if (!evaluatorResultMap[scoreType]) {
        evaluatorResultMap[scoreType] = {};
      }
      evaluatorResultMap[scoreType][experimentId] = aggregatorResult.data;
    });
  });
}

export async function fetchEvaluatorAggregateResult(
  params: BatchGetExperimentAggrResultRequest,
) {
  const res = await StoneEvaluationApi.BatchGetExperimentAggrResult(params);
  const resultMap: EvaluatorAggregateResultMap = {};
  res.expt_aggregate_result?.forEach(result => {
    const experimentId = result?.experiment_id ?? '';
    const results = Object.values(result.evaluator_results ?? {});
    getEvaluatorAggregateResultMap(resultMap, results, experimentId);
  });
  return resultMap;
}
