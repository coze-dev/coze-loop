// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode, useCallback } from 'react';

import { get } from 'lodash-es';
import { type ISpec, type Datum } from '@visactor/vchart/esm/typings';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  Chart,
  ChartCardItemRender,
  ExperimentScoreTypeSelect,
  type CustomTooltipProps,
} from '@cozeloop/evaluate-components';
import {
  type AggregatorType,
  type ColumnAnnotation,
  type ColumnEvaluator,
} from '@cozeloop/api-schema/evaluation';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState, Tag } from '@coze-arch/coze-design';

import { setExperimentDetailLocalCache } from '@/utils/experiment-local-cache';
import { AnnotationInfo, EvaluatorInfo } from '@/components/info-tag';

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
    style: {
      fill: 'rgba(15, 21, 40, 0.82)',
      fontSize: 12,
    },
  },
  axes: [
    {
      orient: 'bottom',
      // 设置柱子间的间距百分比
      paddingInner: 0.3,
      label: {
        autoLimit: true,
        style: value => {
          const [_n, _ver] = ((value as string) || '').split(' ');
          return {
            react: {
              element: (
                <div className="bg-white flex items-center">
                  <div className="mr-1 text-xs font-normal leading-4 text-[rgba(15,21,40,0.82)]">
                    {_n}
                  </div>
                  {_ver ? (
                    <Tag size="small" color="primary" className="shrink-0">
                      {_ver}
                    </Tag>
                  ) : null}
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

const titleMap = {
  evaluator: I18n.t('evaluator_aggregate_score'),
  annotation: I18n.t('annotation_aggregate_score'),
};
interface ChartDataItem {
  name: string;
  score: number;
  id: Int64;
}

interface Columns {
  evaluator: ColumnEvaluator[];
  annotation: ColumnAnnotation[];
}

interface Props<T extends keyof Columns> {
  experimentID: string;
  type: T;
  columns?: Columns[T];
  values: ChartDataItem[];
  ready?: boolean;
  maxCount?: number;
  aggregatorType: AggregatorType;
  onAggregatorTypeChange?: (type: AggregatorType) => void;
}
export function AggregateChartBase<T extends keyof Columns>({
  experimentID,
  type,
  values,
  columns,
  ready,
  maxCount = 100,
  aggregatorType,
  onAggregatorTypeChange,
}: Props<T>) {
  const customTooltip = useCallback(
    (renderProps: CustomTooltipProps) => (
      <ComplexTooltipContent {...renderProps} type={type} columns={columns} />
    ),

    [columns, type],
  );

  const renderContent = (max?: number) =>
    ready && values.length === 0 ? (
      <div className="pt-10 pb-6">
        <EmptyState
          size="full_screen"
          icon={<IconCozIllusAdd />}
          title={I18n.t('no_data')}
          description={
            type === 'evaluator'
              ? I18n.t('refresh_after_experiment')
              : I18n.t('evaluate_complete_label_data_annotation_then_refresh')
          }
        />
      </div>
    ) : (
      <Chart
        spec={spec}
        values={max ? values.slice(0, max) : values}
        customTooltip={customTooltip}
      />
    );

  return (
    <ChartCardItemRender
      item={{
        id: '',
        title: titleMap[type] || I18n.t('aggregation_score'),
        content: renderContent(maxCount),
        fullContent: renderContent(),
      }}
      action={
        <ExperimentScoreTypeSelect
          value={aggregatorType}
          onChange={val => {
            onAggregatorTypeChange?.(val);
            setExperimentDetailLocalCache(`${experimentID}_${type}`, {
              overviewAggregatorType: val,
            });
          }}
        />
      }
    />
  );
}

function ComplexTooltipContent<T extends keyof Columns>(
  props: CustomTooltipProps & {
    type: T;
    columns?: Columns[T];
  },
) {
  const { params, type, columns, actualTooltip } = props;
  // 获取hover目标柱状图数据
  const datum: Datum | undefined = params?.datum?.id
    ? params?.datum
    : get(actualTooltip, 'data[0].data[0].datum[0]');

  if (!datum?.id) {
    return null;
  }

  let header: ReactNode;
  if (type === 'evaluator') {
    const evaluator = (columns as Columns['evaluator'])?.find(
      c => c.evaluator_version_id === datum?.id,
    );
    header = <EvaluatorInfo evaluator={evaluator} />;
  } else {
    const annotation = (columns as Columns['annotation'])?.find(
      c => c.tag_key_id === datum?.id,
    );
    header = <AnnotationInfo annotation={annotation} />;
  }

  return (
    <div className="text-xs flex flex-col gap-2 w-56">
      <div className="text-sm font-medium">{header}</div>
      <div className="flex items-center gap-2">
        <div className="w-2 h-2 bg-[var(--semi-color-primary)]" />
        <span className="text-muted-foreground">{I18n.t('score')}</span>
        <span className="font-semibold ml-auto">{datum?.score}</span>
      </div>
    </div>
  );
}
