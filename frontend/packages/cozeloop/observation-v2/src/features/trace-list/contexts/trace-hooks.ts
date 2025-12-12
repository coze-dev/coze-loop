import { useState, useCallback } from 'react';

import { type FieldMeta } from '@cozeloop/api-schema/observation';

export const useTraceActions = () => {
  const [fieldMetas, setFieldMetasState] = useState<
    Record<string, FieldMeta | undefined> | undefined
  >(undefined);

  const setFieldMetas = useCallback((e?: Record<string, FieldMeta>) => {
    setFieldMetasState(e);
  }, []);

  return {
    fieldMetas,
    setFieldMetas,
  };
};
