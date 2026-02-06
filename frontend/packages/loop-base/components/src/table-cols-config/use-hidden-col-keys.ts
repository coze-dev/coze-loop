// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState, useEffect } from 'react';

import { safeJsonParse } from '@cozeloop/toolkit';

import { type ColKey } from './type';

const ITEM_KEY = 'FORNAX_TABLE_HIDDEN_COLS';

export function useHiddenColKeys({
  localStorageKey,
  defaultHiddenColKeys,
}: {
  localStorageKey?: string;
  defaultHiddenColKeys?: ColKey[];
}) {
  const handleSetHiddenColKeys = () => {
    if (localStorageKey) {
      const res = safeJsonParse<Record<string, ColKey[] | undefined>>(
        localStorage.getItem(ITEM_KEY),
      );
      if (res) {
        const list = res[`${localStorageKey}`];
        if (list) {
          return list;
        }
      }
    }

    return defaultHiddenColKeys || [];
  };

  const [hiddenColKeys, setHiddenColKeys] = useState<ColKey[]>(
    handleSetHiddenColKeys,
  );

  useEffect(() => {
    setHiddenColKeys(handleSetHiddenColKeys());
  }, [defaultHiddenColKeys]);

  useEffect(() => {
    if (localStorageKey) {
      const res =
        safeJsonParse<Record<string, ColKey[] | undefined>>(
          localStorage.getItem(ITEM_KEY),
        ) || {};
      res[`${localStorageKey}`] = hiddenColKeys.length
        ? hiddenColKeys
        : undefined;
      localStorage.setItem(ITEM_KEY, JSON.stringify(res));
    }
  }, [hiddenColKeys]);

  return [hiddenColKeys, setHiddenColKeys] as const;
}
