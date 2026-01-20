// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { type EvaluatorResult } from '@cozeloop/api-schema/evaluation';
import {
  IconCozCrossCircleFill,
  IconCozCheckMarkCircleFill,
} from '@coze-arch/coze-design/icons';
import { Typography } from '@coze-arch/coze-design';

export function EvaluatorTestRunResult({
  evaluatorResult,
  errorMsg,
  className,
  containerStyle,
}: {
  errorMsg?: string;
  evaluatorResult: EvaluatorResult | undefined;
  className?: string;
  containerStyle?: React.CSSProperties;
}) {
  const isError = Boolean(errorMsg);
  return (
    <div
      className={classNames('py-6 px-8 rounded-[12px] coz-bg-plus', className)}
      style={containerStyle}
    >
      <div
        className={classNames(
          'flex items-center gap-2 mb-3 text-xxl',
          isError ? 'text-[#D0292F]' : 'text-[#00815C]',
        )}
      >
        {isError ? <IconCozCrossCircleFill /> : <IconCozCheckMarkCircleFill />}
        <span className="font-bold">
          {isError ? I18n.t('debug_failure') : I18n.t('debugging_succeeded')}
        </span>
      </div>
      {!isError ? (
        <div className="mb-2 text-[16px] leading-[28px] coz-fg-primary font-medium">
          <span className="coz-fg-primary font-bold text-xxl">
            {I18n.t('cozeloop_open_evaluate_score_placeholder1', {
              placeholder1: evaluatorResult?.score,
            })}
          </span>
          <span className="coz-fg-dim text-[13px] ml-2">
            {I18n.t('score_only_for_preview')}
          </span>
        </div>
      ) : null}
      <Typography.Text
        className={classNames('text-sm font-normal coz-fg-primary')}
        ellipsis={{
          showTooltip: true,
          rows: 3,
        }}
      >
        {errorMsg ||
          `${I18n.t('evaluate_reason')}${evaluatorResult?.reasoning ?? '-'}`}
      </Typography.Text>
    </div>
  );
}
