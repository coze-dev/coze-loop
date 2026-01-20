// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { type EventParams } from '@visactor/vchart';
import { I18n } from '@cozeloop/i18n-adapter';
import { type ColumnEvaluator } from '@cozeloop/api-schema/evaluation';

import { CHART_MORE_KEY, getScorePercentage, splitData } from '../utils';
import { type DistributionMap, type ChartItemValue } from './types';
import { ChartCardBase } from './chart-card-base';

interface Props {
  evaluator: ColumnEvaluator;
  chartType: 'pie' | 'bar';
  ready?: boolean;
  evaluatorScoreMap?: Record<Int64, DistributionMap | undefined>;
  // 最大显示数量
  maxCount?: number;

  onClick?: (e: EventParams) => void;
}
export function EvaluatorChartCard({
  evaluator,
  chartType,
  ready,
  evaluatorScoreMap = {},
  maxCount = 100,
  onClick,
}: Props) {
  const values = useMemo(() => {
    const versionId = evaluator?.evaluator_version_id ?? '';
    const scoreCountMap = evaluatorScoreMap[versionId] ?? {};
    const originalData = Object.entries(scoreCountMap).sort(
      (a, b) => Number(b[1].count) - Number(a[1].count),
    );
    const { data } = splitData(originalData, maxCount);

    const nameMap = { [CHART_MORE_KEY]: I18n.t('analytics_subtitle_others') };
    const result: ChartItemValue[] = data.map(([score, item]) => ({
      name: nameMap[score] ?? score,
      dimension: score,
      count: item.count,
      percentage: item.percentage,
      percentageStr: getScorePercentage(item.percentage),
      item: {
        ...item,
        dimension: nameMap[score] ?? score,
      },
    }));
    return result;
  }, [ready, evaluator, evaluatorScoreMap, maxCount]);

  return (
    <ChartCardBase
      type="evaluator"
      chartType={chartType}
      ready={ready}
      values={values}
      onClick={onClick}
    />
  );
}
