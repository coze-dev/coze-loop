import { useMemo } from 'react';

import { type EventParams, type ISpec } from '@visactor/vchart';
import { Chart } from '@cozeloop/evaluate-components';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState } from '@coze-arch/coze-design';

import { type ChartItemValue } from './types';
import { ComplexTooltipContent } from './complex-tooltip-content';

interface Props {
  type: 'evaluator' | 'annotate';
  chartType: 'pie' | 'bar';
  ready?: boolean;
  values: ChartItemValue[];
  onClick?: (e: EventParams) => void;
}
export function ChartCardBase({
  type,
  chartType,
  ready,
  values,
  onClick,
}: Props) {
  const categoryMap = useMemo(
    () =>
      values.reduce(
        (cur, item) => {
          cur[item.dimension] = item;
          return cur;
        },
        {} as unknown as Record<string, ChartItemValue>,
      ),
    [values],
  );
  return !ready || values.length === 0 ? (
    <div className="pt-10 pb-6">
      <EmptyState
        size="full_screen"
        icon={<IconCozIllusAdd />}
        title="暂无数据"
        description={
          type === 'evaluator'
            ? '实验完成后，再刷新重试'
            : '完成该标签的所有数据标注后，再刷新重试'
        }
      />
    </div>
  ) : (
    <Chart
      className="h-[260px]"
      spec={
        chartType === 'pie' ? getSpecPie(categoryMap) : getSpecBar(categoryMap)
      }
      values={values}
      customTooltip={ComplexTooltipContent}
      onClick={onClick}
    />
  );
}

const getSpecPie = (categoryMap: Record<string, ChartItemValue>): ISpec => ({
  type: 'pie',
  crosshair: {
    xField: { visible: true },
  },
  color: ['#6D62EB', '#00B2B2', '#3377FF', '#FFB829', '#CA61FF', '#7DD600'],
  valueField: 'count',
  categoryField: 'dimension',
  outerRadius: 0.8,
  innerRadius: 0,
  height: 200,
  legends: {
    visible: true,
    orient: 'left',
    data: items =>
      items.map(item => {
        item.value = categoryMap[item.label].percentageStr;
        return item;
      }),
    item: {
      width: '30%',
      shape: {
        style: {
          symbolType: 'square',
        },
      },
      label: {
        style: {
          fill: 'rgba(15, 21, 40, 0.82)',
        },
        formatMethod(text) {
          return categoryMap[text]?.name ?? text;
        },
      },
      value: {
        style: {
          fill: 'rgba(32, 41, 69, 0.62)',
        },
      },
    },
  },
  label: {
    visible: true,
    rotate: false,
    formatter: '{percentageStr}',
    style: {
      fontSize: 12,
    },
  },
});

const getSpecBar = (categoryMap: Record<string, ChartItemValue>): ISpec => ({
  type: 'bar',
  xField: 'dimension',
  yField: 'percentage',
  axes: [
    {
      orient: 'left',
      label: {
        autoLimit: true,
        formatMethod: val => {
          if (typeof val === 'number') {
            return `${val * 100}%`;
          }
          return val;
        },
      },
    },
    {
      orient: 'bottom',
      sampling: false,
      label: {
        autoLimit: true,
        autoRotate: true,
        autoRotateAngle: [0, 90],
        autoHide: true,
        formatMethod(text) {
          return categoryMap[text as string]?.name ?? text;
        },
        style: {
          fill: 'rgba(15, 21, 40, 0.82)',
          fontSize: 12,
        },
      },
    },
  ],
  label: {
    visible: true,
    position: 'top',
    overlap: false,
    formatter: '{percentageStr}',
    style: {
      fill: 'rgba(15, 21, 40, 0.82)',
      fontSize: 12,
    },
  },
});
