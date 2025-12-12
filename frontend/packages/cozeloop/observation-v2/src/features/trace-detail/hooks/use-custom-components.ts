import {
  IconCozCheckMarkCircleFillPalette,
  IconCozCrossCircleFill,
} from '@coze-arch/coze-design/icons';

import { NodeDetailEmpty, RunTreeEmpty } from '@/shared/ui/empty-status';

import { useTraceDetailContext } from './use-trace-detail-context';

export const useCustomComponents = () => {
  const { customParams } = useTraceDetailContext();

  return {
    StatusSuccessIcon:
      customParams?.StatusSuccessIcon || IconCozCheckMarkCircleFillPalette,
    StatusErrorIcon: customParams?.StatusErrorIcon || IconCozCrossCircleFill,
    RunTreeEmpty: customParams?.RunTreeEmpty || RunTreeEmpty,
    NodeDetailEmpty: customParams?.NodeDetailEmpty || NodeDetailEmpty,
  };
};
