// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback, useEffect, useMemo, useState } from 'react';

import { get } from 'lodash-es';
import { type ISpec, type Datum } from '@visactor/vchart/esm/typings';
import { TypographyText } from '@cozeloop/shared-components';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  Chart,
  ChartCardItemRender,
  ExperimentScoreTypeSelect,
  type CustomTooltipProps,
} from '@cozeloop/evaluate-components';
import {
  type AggregateData,
  AggregatorType,
  type EvaluatorAggregateResult,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState, Tag } from '@coze-arch/coze-design';

import {
  getExperimentDetailLocalCache,
  setExperimentDetailLocalCache,
} from '@/utils/experiment-local-cache';

interface ChartDataItem {
  name: string;
  score: number;
  id: Int64;
}

const spec: ISpec = {
  type: 'bar',
  xField: 'name',
  yField: 'score',
  barMaxWidth: 200,
  padding: 24,
  label: {
    visible: true,
    position: 'top',
    overlap: false,
  },
  axes: [
    {
      orient: 'bottom',
      // 设置柱子间的间距百分比
      paddingInner: 0.3,
      label: {
        style: value => {
          const [_n, _ver] = ((value as string) || '').split(' ');
          return {
            react: {
              element: (
                <div className="bg-white flex items-center">
                  <div className="mr-1 text-xs font-normal leading-4 text-[rgba(15,21,40,0.82)]">
                    {_n}
                  </div>
                  <Tag size="small" color="primary" className="shrink-0">
                    {_ver}
                  </Tag>
                </div>
              ),
            },
          };
        },
      },
    },
    {
      orient: 'left',
      innerOffset: {
        top: 20,
      },
    },
  ],
};

type EvaluatorResultMap = Record<
  Int64,
  Record<Int64, AggregateData | undefined>
>;

function getEvaluatorAggregateResultMap(
  resultMap: EvaluatorResultMap,
  evaluatorAggregateResults: EvaluatorAggregateResult[],
) {
  evaluatorAggregateResults.forEach(evaluatorResult => {
    const versionId = evaluatorResult?.evaluator_version_id ?? '';
    evaluatorResult.aggregator_results?.forEach(item => {
      const type = item?.aggregator_type ?? '';
      if (!resultMap[type]) {
        resultMap[type] = {};
      }
      resultMap[type][versionId] = item?.data;
    });
  });
  return resultMap;
}

// eslint-disable-next-line complexity
function ComplexTooltipContent(
  props: CustomTooltipProps & {
    experiment: Experiment | undefined;
  },
) {
  const { params, experiment, actualTooltip } = props;
  // 获取hover目标柱状图数据
  const datum: Datum | undefined = params?.datum?.id
    ? params?.datum
    : get(actualTooltip, 'data[0].data[0].datum[0]');
  const evaluator = experiment?.evaluators?.find(
    e => e.current_version?.id === datum?.id,
  );

  if (!datum?.id) {
    return null;
  }
  return (
    <div className="text-xs flex flex-col gap-2 w-56">
      <div className="text-sm font-medium">
        <TypographyText style={{ maxWidth: 160, marginRight: 4 }}>
          {evaluator?.name ?? '-'}
        </TypographyText>
        {evaluator?.current_version?.version ? (
          <Tag size="small" color="primary" className="shrink-0 font-normal">
            {evaluator?.current_version?.version}
          </Tag>
        ) : null}
      </div>
      <div className="flex items-center gap-2">
        <div className="w-2 h-2 bg-[var(--semi-color-primary)]" />
        <span className="text-muted-foreground">{I18n.t('score')}</span>
        <span className="font-semibold ml-auto">{datum?.score}</span>
      </div>
    </div>
  );
}

export default function EvaluatorsScoreChart({
  selectedEvaluatorIds = [],
  evaluatorAggregateResult,
  experiment,
  spaceID,
  ready,
}: {
  selectedEvaluatorIds: Int64[];
  evaluatorAggregateResult: EvaluatorAggregateResult[] | undefined;
  experiment: Experiment | undefined;
  spaceID: Int64;
  ready?: boolean;
}) {
  const experimentID = experiment?.id ?? '';
  const [aggregatorType, setAggregatorType] = useState<AggregatorType>(
    getExperimentDetailLocalCache(experimentID)?.overviewAggregatorType ??
      AggregatorType.Average,
  );
  const [evaluatorResultMap, setEvaluatorResultMap] =
    useState<EvaluatorResultMap>({});

  useEffect(() => {
    const newMap: EvaluatorResultMap = {};
    getEvaluatorAggregateResultMap(newMap, evaluatorAggregateResult ?? []);
    setEvaluatorResultMap(newMap);
  }, [evaluatorAggregateResult]);

  const chartValues = useMemo(() => {
    const evaluatorIdSet = new Set(selectedEvaluatorIds);
    const values: ChartDataItem[] =
      experiment?.evaluators
        ?.filter(e => evaluatorIdSet.has(e.current_version?.id ?? ''))
        ?.map(evaluator => {
          const versionId = evaluator?.current_version?.id ?? '';
          const data = evaluatorResultMap[aggregatorType]?.[versionId];
          const chartValue: ChartDataItem = {
            // name: <EvaluatorPreview spaceID={spaceID} evaluator={evaluator} />,
            name: `${evaluator?.name ?? '-'} ${evaluator.current_version?.version ?? ''}`,
            score: data?.value ?? 0,
            id: versionId,
          };
          return chartValue;
        }) ?? [];
    return values;
  }, [evaluatorResultMap, aggregatorType, experiment, selectedEvaluatorIds]);

  const customTooltip = useCallback(
    (renderProps: CustomTooltipProps) => (
      <ComplexTooltipContent {...renderProps} experiment={experiment} />
    ),

    [experiment, spaceID],
  );
  return (
    <ChartCardItemRender
      item={{
        id: '',
        title: I18n.t('aggregation_score'),
        content:
          ready && evaluatorAggregateResult?.length === 0 ? (
            <div className="pt-10 pb-6">
              <EmptyState
                size="full_screen"
                icon={<IconCozIllusAdd />}
                title={I18n.t('no_data')}
                description={I18n.t('refresh_after_experiment')}
              />
            </div>
          ) : (
            <Chart
              spec={spec}
              values={chartValues}
              customTooltip={customTooltip}
            />
          ),
      }}
      action={
        <ExperimentScoreTypeSelect
          value={aggregatorType}
          onChange={val => {
            setAggregatorType(val);
            setExperimentDetailLocalCache(experimentID, {
              overviewAggregatorType: val,
            });
          }}
        />
      }
    />
  );
}
