import classNames from 'classnames';
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
        <span className="font-bold">{isError ? '调试失败' : '调试成功'}</span>
      </div>
      {!isError ? (
        <div className="mb-2 text-[16px] leading-[28px] coz-fg-primary font-medium">
          <span className="coz-fg-primary font-bold text-xxl">
            {evaluatorResult?.score} 分
          </span>
          <span className="coz-fg-dim text-[13px] ml-2">
            得分仅预览效果，非实际结果。
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
        {errorMsg || `原因：${evaluatorResult?.reasoning ?? '-'}`}
      </Typography.Text>
    </div>
  );
}
