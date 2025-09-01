import { useEffect, useMemo, useState } from 'react';

import {
  AggregatorType,
  type AnnotationAggregateResult,
  type ColumnAnnotation,
} from '@cozeloop/api-schema/evaluation';
import { tag } from '@cozeloop/api-schema/data';

import { getExperimentDetailLocalCache } from '@/utils/experiment-local-cache';

import { getAggregateResultMap, type AggregateDataResultMap } from '../utils';
import { AggregateChartBase } from './aggregate-chart-base';

interface ChartDataItem {
  name: string;
  score: number;
  id: Int64;
}

export function AnnotationsScoreChart({
  selectedIDs,
  annotationAggregateResult,
  annotations,
  ready,
  experimentID,
}: {
  selectedIDs: Int64[];
  annotationAggregateResult: AnnotationAggregateResult[] | undefined;
  annotations: ColumnAnnotation[] | undefined;
  ready?: boolean;
  experimentID: string;
}) {
  const [aggregatorType, setAggregatorType] = useState<AggregatorType>(
    getExperimentDetailLocalCache(`${experimentID}_annotation`)
      ?.overviewAggregatorType ?? AggregatorType.Average,
  );
  const [annotationResultMap, setAnnotationResultMap] =
    useState<AggregateDataResultMap>({});

  useEffect(() => {
    const newMap = getAggregateResultMap(
      'annotation',
      annotationAggregateResult ?? [],
    );
    setAnnotationResultMap(newMap);
  }, [annotationAggregateResult]);

  const chartValues = useMemo(() => {
    const annotationIdSet = new Set(selectedIDs);
    const values = (annotations || [])
      .filter(
        item =>
          annotationIdSet.has(item.tag_key_id ?? '') &&
          item.content_type === tag.TagContentType.ContinuousNumber,
      )
      .map(annotation => {
        const tagKeyId = annotation?.tag_key_id ?? '';
        const data = annotationResultMap[aggregatorType]?.[tagKeyId];
        if (typeof data === 'undefined') {
          return;
        }
        const chartValue: ChartDataItem = {
          name: `${annotation.tag_key_name}`,
          score: data?.value ?? 0,
          id: tagKeyId,
        };
        return chartValue;
      })
      .filter(Boolean) as ChartDataItem[];
    return values;
  }, [annotationResultMap, aggregatorType, annotations, selectedIDs]);
  if (chartValues.length === 0) {
    return null;
  }
  return (
    <AggregateChartBase
      experimentID={experimentID}
      type="annotation"
      columns={annotations}
      values={chartValues}
      ready={ready}
      aggregatorType={aggregatorType}
      onAggregatorTypeChange={setAggregatorType}
      maxCount={5}
    />
  );
}
