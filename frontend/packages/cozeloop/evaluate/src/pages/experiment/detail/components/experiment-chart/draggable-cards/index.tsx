// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  ChartCardItemRender,
  DraggableGrid,
  type ChartCardItem,
} from '@cozeloop/evaluate-components';
import {
  AggregatorType,
  type EvaluatorAggregateResult,
  type ColumnAnnotation,
  type AnnotationAggregateResult,
  type ColumnEvaluator,
  DataType,
} from '@cozeloop/api-schema/evaluation';
import { tag } from '@cozeloop/api-schema/data';

import { AnnotationInfo, EvaluatorInfo } from '@/components/info-tag';

import { type DistributionMap } from './types';
import { EvaluatorChartCard } from './evaluator-chart-card';
import { AnnotateChartCard } from './annotate-chart-card';

/**
 * 提取实验评估器统计分数的分布（如1分的8个，0.6分的4个等）
 * 第一层map_key为评估器版本id
 * 第二层map key为得分，值为得分的数量
 */
function getEvaluatorScoreMap(results: EvaluatorAggregateResult[] = []) {
  const map: Record<Int64, DistributionMap> = {};
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
          map[versionId][scoreItem.score] = {
            ...scoreItem,
            prefix: I18n.t('score'),
            dimension: scoreItem.score,
          };
        },
      );
    });
  });
  return map;
}

/**
 * 提取实验标注统计分数的分布（如1分的8个，0.6分的4个等）
 * 第一层map_key为标注项id
 * 第二层map key为得分，值为得分的数量
 */
function getAnnotationOptionMap(results: AnnotationAggregateResult[] = []) {
  const map: Record<Int64, DistributionMap | undefined> = {};
  results.forEach(result => {
    const tagKeyId = result?.tag_key_id ?? '';
    result?.aggregator_results?.forEach(item => {
      if (item.aggregator_type !== AggregatorType.Distribution) {
        return;
      }
      if (item.data?.data_type === DataType.OptionDistribution) {
        item.data.option_distribution?.option_distribution_items?.forEach(
          optionItem => {
            if (!map[tagKeyId]) {
              map[tagKeyId] = {};
            }
            map[tagKeyId][optionItem.option] = {
              ...optionItem,
              dimension: optionItem.option,
              score: '',
            };
          },
        );
      } else if (item.data?.data_type === DataType.ScoreDistribution) {
        item.data?.score_distribution?.score_distribution_items?.forEach(
          scoreItem => {
            if (!map[tagKeyId]) {
              map[tagKeyId] = {};
            }
            map[tagKeyId][scoreItem.score] = {
              ...scoreItem,
              prefix: I18n.t('score'),
              dimension: scoreItem.score,
              option: '',
            };
          },
        );
      }
    });
  });
  return map;
}

export function DraggableCard({
  spaceID,
  evaluators = [],
  annotations,
  evaluatorAggregateResult,
  annotationAggregateResult,
  ready,
  chartType = 'bar',
}: {
  spaceID: Int64;
  evaluators: ColumnEvaluator[];
  annotations?: ColumnAnnotation[];
  evaluatorAggregateResult?: EvaluatorAggregateResult[];
  annotationAggregateResult?: AnnotationAggregateResult[];
  ready?: boolean;
  chartType?: 'bar' | 'pie';
}) {
  const [items, setItems] = useState<ChartCardItem[]>([]);

  useEffect(() => {
    const evaluatorScoreMap = getEvaluatorScoreMap(evaluatorAggregateResult);
    const annotationOptionMap = getAnnotationOptionMap(
      annotationAggregateResult,
    );

    const evaluatorItems = evaluators.map(evaluator => {
      const versionId = evaluator?.evaluator_version_id ?? '';
      const item: ChartCardItem = {
        id: versionId.toString(),
        title: (
          <EvaluatorInfo
            evaluator={evaluator}
            tagProps={{ className: 'font-normal' }}
          />
        ),
        tooltip: evaluator?.description,
        content: (
          <EvaluatorChartCard
            ready={ready}
            chartType={chartType}
            evaluator={evaluator}
            evaluatorScoreMap={evaluatorScoreMap}
            maxCount={5}
          />
        ),
        fullContent: (
          <EvaluatorChartCard
            ready={ready}
            chartType={chartType}
            evaluator={evaluator}
            evaluatorScoreMap={evaluatorScoreMap}
          />
        ),
      };
      return item;
    });

    const annotationItems =
      annotations
        ?.filter(
          annotation => annotation.content_type !== tag.TagContentType.FreeText,
        )
        ?.map(annotation => {
          const tagKeyId = annotation.tag_key_id || '';
          const item: ChartCardItem = {
            id: tagKeyId.toString(),
            title: (
              <div className="flex items-center">
                <AnnotationInfo
                  className="flex-1 overflow-hidden"
                  annotation={annotation}
                  tagProps={{ className: 'font-normal' }}
                />
              </div>
            ),
            tooltip: annotation?.description,
            content: (
              <AnnotateChartCard
                ready={ready}
                chartType={chartType}
                annotation={annotation}
                annotationOptionMap={annotationOptionMap}
                maxCount={5}
              />
            ),
            fullContent: (
              <AnnotateChartCard
                ready={ready}
                chartType={chartType}
                annotation={annotation}
                annotationOptionMap={annotationOptionMap}
              />
            ),
          };
          return item;
        }) || [];

    setItems([...evaluatorItems, ...annotationItems]);
  }, [
    ready,
    evaluators,
    evaluatorAggregateResult,
    annotations,
    annotationAggregateResult,
    spaceID,
  ]);

  return (
    <DraggableGrid<ChartCardItem>
      items={items}
      itemRender={ChartCardItemRender}
      onItemsChange={setItems}
    />
  );
}
