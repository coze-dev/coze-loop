// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback } from 'react';

import { handleCopy as copy } from '@cozeloop/components';

export const useDetailCopy = (moduleName?: string) => {
  const handleCopy = useCallback(
    (text: string) => {
      copy(text);

      if (!moduleName) {
        return;
      }
    },
    [moduleName],
  );
  return handleCopy;
};
