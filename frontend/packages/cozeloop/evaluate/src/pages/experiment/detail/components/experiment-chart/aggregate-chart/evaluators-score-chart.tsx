// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useMemo, useState } from 'react';

import {
  AggregatorType,
  type ColumnEvaluator,
  type EvaluatorAggregateResult,
} from '@cozeloop/api-schema/evaluation';

import { getExperimentDetailLocalCache } from '@/utils/experiment-local-cache';

import { getAggregateResultMap, type AggregateDataResultMap } from '../utils';
import { AggregateChartBase } from './aggregate-chart-base';
interface ChartDataItem {
  name: string;
  score: number;
  id: Int64;
}

export function EvaluatorsScoreChart({
  selectedIDs = [],
  evaluatorAggregateResult,
  evaluators,
  ready,
  experimentID,
}: {
  selectedIDs: Int64[];
  evaluatorAggregateResult: EvaluatorAggregateResult[] | undefined;
  evaluators: ColumnEvaluator[] | undefined;
  ready?: boolean;
  experimentID: string;
}) {
  const [aggregatorType, setAggregatorType] = useState<AggregatorType>(
    getExperimentDetailLocalCache(`${experimentID}_evaluator`)
      ?.overviewAggregatorType ?? AggregatorType.Average,
  );
  const [evaluatorResultMap, setEvaluatorResultMap] =
    useState<AggregateDataResultMap>({});

  useEffect(() => {
    const newMap = getAggregateResultMap(
      'evaluator',
      evaluatorAggregateResult ?? [],
    );
    setEvaluatorResultMap(newMap);
  }, [evaluatorAggregateResult]);

  const chartValues = useMemo(() => {
    const evaluatorIdSet = new Set(selectedIDs);
    const values: ChartDataItem[] =
      evaluators
        ?.filter(e => evaluatorIdSet.has(e.evaluator_version_id ?? ''))
        ?.map(evaluator => {
          const versionId = evaluator?.evaluator_version_id ?? '';
          const data = evaluatorResultMap[aggregatorType]?.[versionId];
          const chartValue: ChartDataItem = {
            name: `${evaluator?.name ?? '-'} ${evaluator.version ?? ''}`,
            score: data?.value ?? 0,
            id: versionId,
          };
          return chartValue;
        }) ?? [];
    return values;
  }, [evaluatorResultMap, aggregatorType, evaluators, selectedIDs]);

  return (
    <AggregateChartBase
      experimentID={experimentID}
      type="evaluator"
      columns={evaluators}
      values={chartValues}
      ready={ready}
      aggregatorType={aggregatorType}
      onAggregatorTypeChange={setAggregatorType}
      maxCount={5}
    />
  );
}
