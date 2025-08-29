import { useMemo } from 'react';

import { type EventParams } from '@visactor/vchart';
import { type ColumnAnnotation } from '@cozeloop/api-schema/evaluation';

import { CHART_MORE_KEY, getScorePercentage, splitData } from '../utils';
import { type DistributionMap, type ChartItemValue } from './types';
import { ChartCardBase } from './chart-card-base';

interface Props {
  annotation: ColumnAnnotation;
  chartType: 'pie' | 'bar';
  ready?: boolean;
  annotationOptionMap?: Record<Int64, DistributionMap | undefined>;
  // 最大显示数量
  maxCount?: number;

  onClick?: (e: EventParams) => void;
}
export function AnnotateChartCard({
  annotation,
  chartType,
  ready,
  annotationOptionMap = {},
  maxCount = 100,
  onClick,
}: Props) {
  const values = useMemo(() => {
    const tagValueMap = (annotation.tag_values || []).reduce(
      (cur, item) => {
        cur[item.tag_value_id ?? ''] = item.tag_value_name || '';
        return cur;
      },
      { [CHART_MORE_KEY]: '其他' } as unknown as Record<string, string>,
    );

    const tagKeyId = annotation.tag_key_id ?? '';
    const optionCountMap = annotationOptionMap[tagKeyId] ?? {};

    const originalData = Object.entries(optionCountMap).sort(
      (a, b) => Number(b[1].count) - Number(a[1].count),
    );

    const { data } = splitData(originalData, maxCount);

    const result: ChartItemValue[] = data.map(([option, item]) => ({
      name: tagValueMap[option],
      dimension: option,
      count: item.count,
      percentage: item.percentage,
      percentageStr: getScorePercentage(item.percentage),
      item: {
        ...item,
        dimension: tagValueMap[item.dimension] ?? item.dimension,
      },
    }));
    return result;
  }, [ready, annotation, annotationOptionMap, maxCount]);

  return (
    <ChartCardBase
      type="annotate"
      chartType={chartType}
      ready={ready}
      values={values}
      onClick={onClick}
    />
  );
}
