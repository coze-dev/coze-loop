import { useEffect } from 'react';

import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type DeleteManualAnnotationRequest } from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';

import { useAnnotationPanelContext } from '@/components/annotation-panel/annotation-panel-context.ts';

export const useDeleteAnnotation = () => {
  const { spaceID } = useSpace();
  const { setSaveLoading } = useAnnotationPanelContext();
  const { runAsync, loading } = useRequest(
    async (params: Omit<DeleteManualAnnotationRequest, 'workspace_id'>) => {
      await observabilityTrace.DeleteManualAnnotation({
        workspace_id: spaceID,
        ...params,
      });
    },
    {
      manual: true,
      onFinally: () => {
        setSaveLoading?.(false);
      },
    },
  );

  useEffect(() => {
    setSaveLoading?.(loading);
  }, [loading]);

  return { runAsync, loading };
};
