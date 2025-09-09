// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { isEqual } from 'lodash-es';
import { useRequest } from 'ahooks';
import { type ISpec } from '@visactor/vchart';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type Experiment,
  type Evaluator,
  AggregatorType,
  type AggregateData,
} from '@cozeloop/api-schema/evaluation';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState, Spin } from '@coze-arch/coze-design';

import { EvaluatorSelectLocalData } from '../selectors/evaluator-select-local-data';
import { ExperimentScoreTypeSelect } from '../evaluator-score-type-select';
import { EvaluatorExperimentsChartTooltip } from '../evaluator-experiments-chart-tooltip';
import { type ItemRenderProps, DraggableGrid } from '../draggable-grid';
import {
  type CustomTooltipProps,
  ChartCardItemRender,
  Chart,
  type ChartCardItem,
} from '../chart';
import {
  getExperimentNameWithIndex,
  uniqueExperimentsEvaluators,
} from '../../../utils/experiment';
import { EvaluatorPreview } from '../../../components/previews/evaluator-preview';
import { RefreshButton } from '../../../components/common';
import {
  type EvaluatorAggregateResultMap,
  fetchEvaluatorAggregateResult,
} from './tools';

interface ChartDataItem {
  name: string;
  score: number;
  id: Int64;
}

const specBar: ISpec = {
  type: 'bar',
  color: [
    '#6D62EB',
    '#00B2B2',
    '#00B2B2',
    '#00B2B2',
    '#00B2B2',
    '#00B2B2',
    '#00B2B2',
    '#00B2B2',
    '#00B2B2',
    '#00B2B2',
    '#00B2B2',
  ],
  xField: 'name',
  yField: 'score',
  // barMaxWidth: 40,
  axes: [
    {
      orient: 'bottom',
      // 设置柱子间的间距百分比
      paddingInner: 0.55,
      label: {
        // 标签相关配置
        style: {
          fontSize: 12,
        },
      },
    },
    {
      orient: 'left',
      label: {
        // 标签相关配置
        style: {
          fontSize: 12,
        },
      },
      innerOffset: {
        top: 12,
      },
    },
  ],
  legends: {
    visible: true,
    orient: 'bottom',
  },
  seriesField: 'name',
  label: {
    visible: true,
    position: 'top',
    overlap: false,
  },
};

const specLine: ISpec = {
  type: 'line',
  color: ['#6D62EB'],
  xField: 'name',
  yField: 'score',
};

// eslint-disable-next-line complexity
function EvaluatorChart({
  spaceID,
  experiments,
  evaluator,
  evaluatorAggregateResult = {},
  showAggregatorTypeSelect = true,
  action,
  chartType = 'bar',
  xFieldValueType = 'name',
  ready,
  cardHeaderStyle,
  cardBodyStyle,
  showEvalSetTooltip,
  showEvalTargetTooltip,
}: {
  spaceID: Int64;
  experiments: Experiment[] | undefined;
  evaluator: Evaluator | undefined;
  evaluatorAggregateResult:
    | Record<Int64, Record<Int64, AggregateData | undefined>>
    | undefined;
  showAggregatorTypeSelect?: boolean;
  action?: React.ReactNode;
  chartType?: 'bar' | 'line';
  xFieldValueType?: 'name' | 'id';
  ready?: boolean;
  cardHeaderStyle?: React.CSSProperties;
  cardBodyStyle?: React.CSSProperties;
  showEvalSetTooltip?: boolean;
  showEvalTargetTooltip?: boolean;
}) {
  const [aggregatorType, setAggregatorType] = useState<AggregatorType>(
    AggregatorType.Average,
  );

  const experimentScoreMap = evaluatorAggregateResult?.[aggregatorType];
  const evaluatorVersionId = `${evaluator?.current_version?.id ?? ''}`;

  const chartValues = useMemo(() => {
    const data: ChartDataItem[] = (experiments ?? [])
      .filter(e => experimentScoreMap?.[e.id ?? ''])
      .map((item, index) => ({
        name:
          xFieldValueType === 'id'
            ? `#${item.id?.toString()?.slice(-5) ?? '-'}`
            : getExperimentNameWithIndex(item, index, false),
        score: experimentScoreMap[item.id ?? '']?.value ?? 0,
        id: item.id ?? '',
      }));
    return data;
  }, [experiments, experimentScoreMap, xFieldValueType]);

  const spec = chartType === 'bar' ? specBar : specLine;
  if (
    xFieldValueType === 'id' &&
    spec.legends &&
    !Array.isArray(spec.legends)
  ) {
    spec.legends.visible = false;
  }

  const customTooltip = useCallback(
    (renderProps: CustomTooltipProps) => (
      <EvaluatorExperimentsChartTooltip
        {...renderProps}
        evaluator={evaluator}
        experiments={experiments ?? []}
        spaceID={spaceID}
        showEvalSet={showEvalSetTooltip}
        showEvalTarget={showEvalTargetTooltip}
      />
    ),
    [evaluator, experiments, spaceID],
  );

  return (
    <ChartCardItemRender
      cardBodyStyle={cardBodyStyle}
      cardHeaderStyle={cardHeaderStyle}
      item={{
        id: evaluatorVersionId,
        title: <EvaluatorPreview evaluator={evaluator} />,
        tooltip: evaluator?.description,
        content:
          ready && chartValues.length === 0 ? (
            <EmptyState
              size="full_screen"
              icon={<IconCozIllusAdd />}
              title={I18n.t('no_data')}
              description={I18n.t('refresh_after_experiment')}
            />
          ) : (
            <Chart
              spec={spec}
              values={chartValues}
              customTooltip={customTooltip}
            />
          ),
      }}
      action={
        <>
          {showAggregatorTypeSelect ? (
            <ExperimentScoreTypeSelect
              value={aggregatorType}
              onChange={setAggregatorType}
            />
          ) : null}
          {action}
        </>
      }
    />
  );
}

// eslint-disable-next-line @coze-arch/max-line-per-function
export function ExperimentContrastChart({
  spaceID,
  experiments,
  experimentIds,
  onlyBaseExperimentEvluators,
  selectedEvaluatorsId,
  showActions = true,
  layout = 'grid',
  showAggregatorTypeSelect = true,
  loading,
  onRefresh,
  chartType,
  xFieldValueType,
  cardHeaderStyle,
  cardBodyStyle,
  showEvalSetTooltip,
  showEvalTargetTooltip,
}: {
  spaceID: Int64;
  experiments: Experiment[] | undefined;
  experimentIds: Int64[] | undefined;
  // 仅展示 base experiment 的 evaluator
  onlyBaseExperimentEvluators?: boolean;
  selectedEvaluatorsId?: Int64[];
  showActions?: boolean;
  layout?: 'horizontal' | 'grid';
  showAggregatorTypeSelect?: boolean;
  onRefresh?: () => void;
  loading?: boolean;
  chartType?: 'bar' | 'line';
  xFieldValueType?: 'name' | 'id';
  cardHeaderStyle?: React.CSSProperties;
  cardBodyStyle?: React.CSSProperties;
  showEvalSetTooltip?: boolean;
  showEvalTargetTooltip?: boolean;
}) {
  const [items, setItems] = useState<ChartCardItem[]>([]);
  const [selectedEvaluatorIds, setSelectedEvaluatorIds] = useState<Int64[]>(
    selectedEvaluatorsId ?? [],
  );
  const [resultDataMap, setResultDataMap] =
    useState<EvaluatorAggregateResultMap>({});

  const experimentIdsRef = useRef(experimentIds);

  const service = useRequest(async () => {
    const data = await fetchEvaluatorAggregateResult({
      workspace_id: spaceID,
      experiment_ids: experimentIds ?? [],
    });
    setResultDataMap(data);
    return data;
  });

  const evaluators = useMemo(() => {
    if (onlyBaseExperimentEvluators) {
      return experiments?.[0]?.evaluators;
    }
    const allEvaluators = uniqueExperimentsEvaluators(experiments ?? []);
    return allEvaluators;
  }, [experiments, onlyBaseExperimentEvluators]);

  useEffect(() => {
    if (!isEqual(experimentIdsRef.current, experimentIds)) {
      experimentIdsRef.current = experimentIds;
      service.run();
    }
  }, [experimentIds, service]);

  useEffect(() => {
    const evaluatorIdSet = new Set(selectedEvaluatorIds);
    const newItems: ChartCardItem[] =
      evaluators
        ?.filter(e => evaluatorIdSet.has(e.current_version?.id ?? ''))
        ?.map(evaluator => {
          const item: ChartCardItem = {
            id: `${evaluator?.current_version?.id ?? ''}`,
          };
          return item;
        }) ?? [];
    setItems(newItems);
  }, [evaluators, selectedEvaluatorIds]);

  useEffect(() => {
    setSelectedEvaluatorIds(
      evaluators?.map(e => e.current_version?.id ?? '') ?? [],
    );
  }, [evaluators]);

  useEffect(() => {
    if (selectedEvaluatorsId !== undefined) {
      setSelectedEvaluatorIds(selectedEvaluatorsId);
    }
  }, [selectedEvaluatorsId]);

  const itemRender = useCallback(
    ({ item, action }: ItemRenderProps<ChartCardItem>) => {
      const evaluatorVersionId = item.id;
      const evaluator = evaluators?.find(
        e => e.current_version?.id === evaluatorVersionId,
      );
      const result = resultDataMap[evaluatorVersionId] ?? {};
      return (
        <EvaluatorChart
          spaceID={spaceID}
          experiments={experiments}
          evaluator={evaluator}
          evaluatorAggregateResult={result}
          action={action}
          showAggregatorTypeSelect={showAggregatorTypeSelect}
          chartType={chartType}
          xFieldValueType={xFieldValueType}
          ready={!service.loading}
          cardHeaderStyle={cardHeaderStyle}
          cardBodyStyle={cardBodyStyle}
          showEvalSetTooltip={showEvalSetTooltip}
          showEvalTargetTooltip={showEvalTargetTooltip}
        />
      );
    },
    [
      experiments,
      evaluators,
      resultDataMap,
      spaceID,
      chartType,
      showAggregatorTypeSelect,
      xFieldValueType,
      service.loading,
      showEvalSetTooltip,
      showEvalTargetTooltip,
    ],
  );

  return (
    <div>
      {showActions ? (
        <div className="flex justify-end mb-3 gap-2">
          <EvaluatorSelectLocalData
            prefix={I18n.t('indicator')}
            placeholder={I18n.t('please_select_an_indicator')}
            multiple={true}
            maxTagCount={1}
            style={{ minWidth: 200 }}
            evaluators={evaluators}
            value={selectedEvaluatorIds}
            onChange={val => setSelectedEvaluatorIds(val as Int64[])}
          />
          <RefreshButton
            onRefresh={() => {
              service.refresh();
              onRefresh?.();
            }}
          />
        </div>
      ) : null}
      {layout === 'grid' ? (
        <Spin spinning={loading || service.loading}>
          <DraggableGrid<ChartCardItem>
            items={items}
            itemRender={itemRender}
            onItemsChange={setItems}
          />
        </Spin>
      ) : (
        <Spin spinning={loading || service.loading}>
          <div className="flex gap-4">
            {items.map(item => (
              <div className="w-[536px] shrink-0">
                {itemRender({ item, action: null })}
              </div>
            ))}
          </div>
        </Spin>
      )}
    </div>
  );
}
