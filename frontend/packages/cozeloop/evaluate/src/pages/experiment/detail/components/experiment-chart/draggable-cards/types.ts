import {
  type OptionDistributionItem,
  type ScoreDistributionItem,
} from '@cozeloop/api-schema/evaluation';

export interface ChartItemValue {
  name: string;
  count: string;
  dimension: string;
  percentage: number;
  percentageStr: string;
}

export type DistributionItem = (
  | ScoreDistributionItem
  | OptionDistributionItem
) & {
  prefix?: string;
  dimension: string;
};

export type DistributionMap = Record<string, DistributionItem>;
