import {
  IconCozCheckMarkCircleFillPalette,
  IconCozCrossCircleFill,
  IconCozClockFill,
} from '@coze-arch/coze-design/icons';

import { useTraceStore } from '@/features/trace-list/stores/trace';

export const useCustomComponents = () => {
  const { customParams } = useTraceStore();

  return {
    StatusSuccessIcon:
      customParams?.StatusSuccessIcon ?? IconCozCheckMarkCircleFillPalette,
    StatusErrorIcon: customParams?.StatusErrorIcon ?? IconCozCrossCircleFill,
    LatencyIcon: customParams?.LatencyIcon ?? IconCozClockFill,
  };
};
