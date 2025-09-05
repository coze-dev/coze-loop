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
}: {
  errorMsg?: string;
  evaluatorResult: EvaluatorResult | undefined;
  className?: string;
}) {
  const isError = Boolean(errorMsg);
  return (
    <div
      className={classNames('py-6 px-8 rounded-[12px] coz-bg-plus', className)}
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
            {I18n.t('scores_only_previews_not_actual_results')}
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
          `${I18n.t('reason')}：${evaluatorResult?.reasoning ?? '-'}`}
      </Typography.Text>
    </div>
  );
}
