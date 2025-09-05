/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useMemo, useRef, useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type ColumnAnnotation,
  type ColumnEvaluator,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import { tag } from '@cozeloop/api-schema/data';
import {
  IconCozAnalytics,
  IconCozPieChart,
} from '@coze-arch/coze-design/icons';
import { Radio, RadioGroup, Spin } from '@coze-arch/coze-design';

import {
  getExperimentDetailLocalCache,
  setExperimentDetailLocalCache,
} from '@/utils/experiment-local-cache';
import { batchGetExperimentAggrResult } from '@/request/experiment';

import { DraggableCard } from './draggable-cards';
import {
  MetricSelectLocalData,
  type MetricValueType,
} from './components/metric-select-local-data';
import { AnnotationsScoreChart, EvaluatorsScoreChart } from './aggregate-chart';

export default function ExperimentChart({
  spaceID,
  experiment,
  experimentID,
  columnAnnotations,
  columnEvaluators,
  loading = false,
}: {
  spaceID: string;
  experiment: Experiment | undefined;
  experimentID: string;
  columnEvaluators?: ColumnEvaluator[];
  columnAnnotations?: ColumnAnnotation[];
  loading?: boolean;
}) {
  const [selectedMetrics, setSelectedMetrics] = useState<MetricValueType[]>([]);
  const currentExperimentRef = useRef<Experiment | undefined>();

  const [chartType, setChartType] = useState<'bar' | 'pie'>('bar');
  const service = useRequest(
    async () => {
      const res = await batchGetExperimentAggrResult({
        workspace_id: spaceID,
        experiment_ids: [experimentID ?? 0],
      });

      const result = res.expt_aggregate_result?.[0];
      return {
        evaluatorAggregateResult: result?.evaluator_results ?? {},
        annotationAggregateResult: result?.annotation_results ?? {},
      };
    },
    { refreshDeps: [experimentID] },
  );

  const experimentAggregateResult = useMemo(
    () => ({
      evaluators:
        Object.values(service.data?.evaluatorAggregateResult ?? {}) ?? [],
      annotations:
        Object.values(service.data?.annotationAggregateResult ?? {}) ?? [],
    }),
    [service.data],
  );

  const allMetricIds = useMemo(
    () => ({
      evaluatorIds:
        columnEvaluators?.map(e => e.evaluator_version_id ?? '') ?? [],
      annotationIds: columnAnnotations?.map(a => a.tag_key_id ?? '') ?? [],
    }),
    [columnAnnotations, columnEvaluators],
  );
  useEffect(() => {
    // 相同实验id不刷新评估器选中状态
    if (
      currentExperimentRef.current &&
      currentExperimentRef.current.id === experiment?.id
    ) {
      return;
    }
    currentExperimentRef.current = experiment;
    const metrics = getExperimentDetailLocalCache(experimentID)?.metricsValue;

    const evaluatorMetrics =
      columnEvaluators?.map(item => ({
        type: 'evaluator' as const,
        id: item.evaluator_version_id ?? '',
      })) ?? [];
    const annotationMetrics =
      columnAnnotations
        ?.filter(item => item.content_type !== tag.TagContentType.FreeText)
        .map(item => ({
          type: 'annotation' as const,
          id: item.tag_key_id ?? '',
        })) ?? [];
    setSelectedMetrics(metrics ?? [...evaluatorMetrics, ...annotationMetrics]);
  }, [columnEvaluators, columnAnnotations]);

  return (
    <div className=" flex flex-col gap-4">
      <Spin spinning={loading || service.loading}>
        <div className="flex items-center text-sm font-semibold mb-3 h-[32px]">
          {I18n.t('overview')}
        </div>
        <div className="flex gap-2">
          <div className="flex-1">
            <EvaluatorsScoreChart
              selectedIDs={allMetricIds.evaluatorIds}
              evaluatorAggregateResult={experimentAggregateResult.evaluators}
              ready={!service.loading && !loading}
              evaluators={columnEvaluators}
              experimentID={experimentID}
            />
          </div>
          <div className="flex-1">
            <AnnotationsScoreChart
              selectedIDs={allMetricIds.annotationIds}
              annotationAggregateResult={experimentAggregateResult.annotations}
              ready={!service.loading && !loading}
              annotations={columnAnnotations}
              experimentID={experimentID}
            />
          </div>
        </div>
        <div className="flex items-center h-[32px] mt-5 mb-3">
          <div className="text-sm font-semibold mr-auto">
            {I18n.t('score_details_data_item_distribution')}
          </div>
          <RadioGroup
            type="button"
            value={chartType}
            onChange={e => setChartType(e.target.value as 'bar' | 'pie')}
            className="mr-2"
          >
            <Radio value="pie">
              <div className="flex items-center">
                <IconCozPieChart className="text-[14px]" />
              </div>
            </Radio>
            <Radio value="bar">
              <div className="flex items-center">
                <IconCozAnalytics className="text-[14px]" />
              </div>
            </Radio>
          </RadioGroup>
          <MetricSelectLocalData
            multiple
            maxTagCount={1}
            style={{ minWidth: 220 }}
            evaluators={columnEvaluators}
            annotations={columnAnnotations}
            value={selectedMetrics}
            onChange={val => {
              setSelectedMetrics(val);
              setExperimentDetailLocalCache(experimentID, {
                metricsValue: val,
              });
            }}
          />
        </div>
        <DraggableCard
          spaceID={spaceID}
          chartType={chartType}
          ready={!service.loading && !loading}
          evaluators={
            columnEvaluators?.filter(item =>
              selectedMetrics.find(
                id =>
                  id.type === 'evaluator' &&
                  id.id === item.evaluator_version_id,
              ),
            ) ?? []
          }
          evaluatorAggregateResult={experimentAggregateResult.evaluators}
          annotations={
            columnAnnotations?.filter(item =>
              selectedMetrics.find(
                id => id.type === 'annotation' && id.id === item.tag_key_id,
              ),
            ) ?? []
          }
          annotationAggregateResult={experimentAggregateResult.annotations}
        />
      </Spin>
    </div>
  );
}
