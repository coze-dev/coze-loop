import { I18n } from '@cozeloop/i18n-adapter';
import { ItemErrorType } from '@cozeloop/api-schema/data';
import {
  type DatasetIOJobProgress,
  type ItemErrorGroup,
} from '@cozeloop/api-schema/data';
import { Typography } from '@coze-arch/coze-design';

import { ErrorTypeMap } from '@/const';

export const ImportResultInfo = ({
  progress,
  errors,
}: {
  progress?: DatasetIOJobProgress;
  errors?: ItemErrorGroup[];
}) => (
  <div>
    <div className="flex gap-2 items-center">
      <Typography.Text className="flex-1 leading-[16px]">
        {I18n.t('status_success')}
        <Typography.Text className="!font-medium mx-1">
          {progress?.added || 0}
        </Typography.Text>
        {I18n.t('cozeloop_open_evaluate_items_failed')}
        <Typography.Text className="!font-medium mx-1">
          {Number(progress?.processed) - Number(progress?.added) || 0}
        </Typography.Text>
        {I18n.t('tiao')}
      </Typography.Text>
    </div>
    {errors?.length ? (
      <div className="mt-2 rounded-[4px] p-2 coz-mg-secondary border border-solid border-[var(--coz-stroke-primary)]">
        <Typography.Text size="small" className="coz-fg-secondary">
          {I18n.t('cozeloop_open_evaluate_execution_failed_reasons')}
        </Typography.Text>
        {errors.map(log => (
          <div className="flex items-center">
            <span className="rounded-[50%] w-[4px] h-[4px] mx-2 bg-[black]"></span>
            <Typography.Text size="small" className="!coz-fg-secondary">
              {ErrorTypeMap[log?.type || ItemErrorType.InternalError]}
              <Typography.Text
                size="small"
                className="!font-semibold !coz-fg-primary"
              >
                {log?.error_count && log?.error_count > 0
                  ? `${I18n.t('cozeloop_open_evaluate_placeholder1_items_in_brackets', { placeholder1: log?.error_count })}`
                  : ''}
              </Typography.Text>
            </Typography.Text>
          </div>
        ))}
      </div>
    ) : null}
  </div>
);
