import {
  IconCozIllusNone,
  IconCozIllusLock,
} from '@coze-arch/coze-design/illustrations';
import { Empty } from '@coze-arch/coze-design';

import { i18nService } from '@/i18n';
import { TRACE_EXPIRED_CODE } from '@/features/trace-detail/constants/code';
import { HorizontalTraceHeader } from '@/features/trace-detail/components/header';

export const getEmptyConfig = (statusCode: number) => {
  switch (statusCode) {
    case TRACE_EXPIRED_CODE:
      return {
        image: (
          <IconCozIllusNone className="text-[120px] w-[120px] h-[120px]" />
        ),
        description: i18nService.t('current_trace_expired_to_view'),
      };
    default:
      return {
        image: (
          <IconCozIllusLock className="text-[120px] w-[120px] h-[120px]" />
        ),
        description: i18nService.t('no_permission_to_view_trace'),
        title: i18nService.t('no_permission_to_view'),
      };
  }
};

interface TraceDetailErrorProps {
  statusCode: number;
  headerConfig?: {
    onClose?: () => void;
    minColWidth?: number;
  };
}
export const TraceDetailError = (props: TraceDetailErrorProps) => {
  const { statusCode } = props;
  const emptyConfig = getEmptyConfig(statusCode);
  return (
    <div className="w-full h-full">
      <div className="border-solid border-0 border-b border-[var(--coz-stroke-primary)]">
        <HorizontalTraceHeader
          showClose
          onClose={props.headerConfig?.onClose}
          showTraceId={false}
        />
      </div>
      <div className="flex-1 flex items-center justify-center h-full">
        <Empty {...emptyConfig} />
      </div>
    </div>
  );
};
