// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { get } from 'lodash-es';
import { type Datum } from '@visactor/vchart/esm/typings';
import { I18n } from '@cozeloop/i18n-adapter';
import { type CustomTooltipProps } from '@cozeloop/evaluate-components';
import {
  type OptionDistributionItem,
  type ScoreDistributionItem,
} from '@cozeloop/api-schema/evaluation';

import { getScorePercentage } from '../utils';

export function ComplexTooltipContent(props: CustomTooltipProps) {
  const { params, actualTooltip } = props;
  // 获取hover目标柱状图数据
  const datum: Datum | undefined = params?.datum?.item
    ? params?.datum
    : get(actualTooltip, 'data[0].data[0].datum[0]');
  const item:
    | ((ScoreDistributionItem | OptionDistributionItem) & {
        prefix: string;
        dimension: string;
      })
    | undefined = datum?.item;
  const prefixBgColor = actualTooltip?.title?.shapeFill;
  if (!item) {
    return null;
  }

  return (
    <div className="w-[220px] flex flex-col gap-2">
      <div className="text-sm font-medium">
        {I18n.t('placeholder0_details', { placeholder0: item.prefix })}
      </div>
      <div className="flex items-center gap-2 text-xs">
        <div className="w-2 h-2" style={{ backgroundColor: prefixBgColor }} />
        <span>
          {item.prefix} {item.dimension}
        </span>
        <span className="font-semibold ml-auto">
          <span className="font-medium text-[var(--coz-fg-primary)]">
            {item.count ?? '-'}
          </span>
          <span className="text-[var(--coz-fg-secondary)]">
            {I18n.t('tiao')}({getScorePercentage(item.percentage)})
          </span>
        </span>
      </div>
    </div>
  );
}
