import {
  type AggregateData,
  AggregatorType,
  type AnnotationAggregateResult,
  type EvaluatorAggregateResult,
  type ScoreDistributionItem,
} from '@cozeloop/api-schema/evaluation';

import { type DistributionItem } from '../draggable-cards/types';

/**
 * 提取实验评估器统计分数的分布（如1分的8个，0.6分的4个等）
 * 第一层map_key为评估器版本id
 * 第二层map key为得分，值为得分的数量
 */
export function getEvaluatorScoreMap(results: EvaluatorAggregateResult[] = []) {
  const map: Record<
    Int64,
    Record<number | string, ScoreDistributionItem> | undefined
  > = {};
  results.forEach(result => {
    const versionId = result?.evaluator_version_id ?? '';
    result?.aggregator_results?.forEach(item => {
      if (item.aggregator_type !== AggregatorType.Distribution) {
        return;
      }
      item.data?.score_distribution?.score_distribution_items?.forEach(
        scoreItem => {
          if (!map[versionId]) {
            map[versionId] = {};
          }
          map[versionId][scoreItem.score] = scoreItem;
        },
      );
    });
  });
  return map;
}

export type AggregateDataResultMap = Record<
  Int64,
  Record<Int64, AggregateData | undefined>
>;

interface AggregateResult {
  annotation: AnnotationAggregateResult[];
  evaluator: EvaluatorAggregateResult[];
}

/**
 * 获取聚合结果映射，以聚合器类型为key，方便切换不同聚合器时，结果显示
 * @param type 聚合结果类型
 * @param aggregateResults 聚合结果
 * @returns
 */
export function getAggregateResultMap<T extends keyof AggregateResult>(
  type: T,
  aggregateResults: AggregateResult[T],
) {
  const resultMap: AggregateDataResultMap = {};
  aggregateResults.forEach(
    (item: AnnotationAggregateResult | EvaluatorAggregateResult) => {
      const key =
        type === 'annotation'
          ? (item as AnnotationAggregateResult).tag_key_id
          : (item as EvaluatorAggregateResult).evaluator_version_id;
      item.aggregator_results?.forEach(result => {
        const aggregatorType = result?.aggregator_type ?? '';
        if (!resultMap[aggregatorType]) {
          resultMap[aggregatorType] = {};
        }
        resultMap[aggregatorType][key] = result?.data;
      });
    },
  );
  return resultMap;
}

export const CHART_MORE_KEY = '_cozeloop_more_';
/**
 * 将数据分为两部分，前max个，剩余的合并为更多
 * @param data
 * @param max
 * @returns
 */
export function splitData(data: [string, DistributionItem][], max?: number) {
  if (typeof max === 'undefined' || max >= data.length) {
    return {
      data,
      more: [],
    };
  }

  const more = data.slice(max);
  const moreCount = more.reduce((acc, item) => acc + Number(item[1].count), 0);
  const morePercentage = more.reduce(
    (acc, item) => acc + item[1].percentage,
    0,
  );

  const moreItem: [string, DistributionItem] = [
    CHART_MORE_KEY,
    {
      dimension: CHART_MORE_KEY,
      option: CHART_MORE_KEY,
      count: String(moreCount),
      percentage: morePercentage,
    },
  ];

  return {
    data: [...data.slice(0, max), moreItem],
    more,
  };
}

export function getScorePercentage(score: number | undefined) {
  if (typeof score !== 'number') {
    return '-';
  }
  const percent = score * 100;
  if (percent % 1 === 0) {
    return `${percent}%`;
  }
  return `${percent?.toFixed(1)}%`;
}
