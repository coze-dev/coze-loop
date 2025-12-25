// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { useUIStore, type BreadcrumbItemConfig } from '@cozeloop/stores';

export function useBreadcrumb(
  config: BreadcrumbItemConfig | BreadcrumbItemConfig[],
) {
  const { pushBreadcrumb, popBreadcrumb } = useUIStore(
    useShallow(store => ({
      pushBreadcrumb: store.pushBreadcrumb,
      popBreadcrumb: store.popBreadcrumb,
    })),
  );

  useEffect(() => {
    if (Array.isArray(config)) {
      config.forEach(item => {
        pushBreadcrumb(item);
      });
    } else {
      pushBreadcrumb(config);
    }

    return () => {
      if (Array.isArray(config)) {
        config.forEach(() => {
          popBreadcrumb();
        });
      } else {
        popBreadcrumb();
      }
    };
  }, [config]);
}
