// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { get } from 'lodash-es';
import { type Datum } from '@visactor/vchart/esm/typings';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type Evaluator,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import { Divider } from '@coze-arch/coze-design';

import { TypographyText } from '../text-ellipsis';
import { EvaluatorPreview } from '../previews/evaluator-preview';
import { EvalTargetPreview } from '../previews/eval-target-preview';
import { EvaluationSetPreview } from '../previews/eval-set-preview';
import { getExperimentNameWithIndex } from '../../utils/experiment';
import { type CustomTooltipProps } from './chart';

// eslint-disable-next-line complexity
export function EvaluatorExperimentsChartTooltip(
  props: CustomTooltipProps & {
    evaluator: Evaluator | undefined;
    experiments: Experiment[];
    spaceID: Int64;
    showEvalTarget?: boolean;
    showEvalSet?: boolean;
  },
) {
  const {
    params,
    evaluator,
    experiments,
    spaceID,
    actualTooltip,
    showEvalTarget = true,
    showEvalSet = true,
  } = props;
  // 获取hover目标柱状图数据
  const datum: Datum | undefined = params?.datum?.id
    ? params?.datum
    : get(actualTooltip, 'data[0].data[0].datum[0]');
  const experiment = experiments?.find(e => e?.id === datum?.id);
  const experimentIndex = experiments?.findIndex(e => e?.id === datum?.id);

  const prefixBgColor = actualTooltip?.title?.shapeFill;

  return (
    <div className="text-xs flex flex-col px-2 py-1 gap-2 w-56 text-[var(--coz-fg-primary)]">
      <div className="text-sm font-medium overflow-hidden">
        <TypographyText>
          {getExperimentNameWithIndex(experiment, experimentIndex, true)}
        </TypographyText>
      </div>
      <div className="flex items-center gap-2">
        <div className={'w-2 h-2'} style={{ backgroundColor: prefixBgColor }} />
        <span className="text-muted-foreground">{evaluator?.name}</span>
        <span className="font-semibold ml-auto">{datum?.score}</span>
      </div>

      <Divider />

      {showEvalTarget ? (
        <div className="flex items-center gap-2 justify-between">
          <span>{I18n.t('evaluation_object')}</span>
          <div className="font-medium text-[var(--coz-fg-plus)]">
            <EvalTargetPreview
              spaceID={spaceID}
              evalTarget={experiment?.eval_target}
              size="small"
            />
          </div>
        </div>
      ) : null}

      {showEvalSet ? (
        <div className="flex items-center gap-2 justify-between">
          <span>{I18n.t('evaluation_set')}</span>
          <div className="font-medium text-[var(--coz-fg-plus)]">
            <EvaluationSetPreview evalSet={experiment?.eval_set} />
          </div>
        </div>
      ) : null}

      <div className="flex relative w-full items-center gap-2 justify-between">
        <span>{I18n.t('evaluator')}</span>
        <div
          className="font-medium relative text-[var(--coz-fg-plus)] flex  justify-end"
          style={{ width: '164px' }}
        >
          <EvaluatorPreview evaluator={evaluator} />
        </div>
      </div>
    </div>
  );
}
