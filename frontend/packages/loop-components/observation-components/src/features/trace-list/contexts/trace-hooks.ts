// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
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
