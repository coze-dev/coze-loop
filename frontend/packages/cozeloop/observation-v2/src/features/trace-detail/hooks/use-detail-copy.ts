import { useCallback } from 'react';

import { handleCopy as copy } from '@cozeloop/components';

export const useDetailCopy = (moduleName?: string) => {
  const handleCopy = useCallback(
    (text: string, point?: string) => {
      copy(text);

      if (!moduleName) {
        return;
      }
    },
    [moduleName],
  );
  return handleCopy;
};
