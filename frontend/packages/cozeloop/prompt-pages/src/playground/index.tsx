// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { GuardPoint, useGuard } from '@cozeloop/guard';

import { useBasicStore } from '@/store/use-basic-store';
import { usePlayground } from '@/hooks/use-playground';
import { PromptDev } from '@/components/prompt-dev';

export function Playground() {
  const globalDisabled = useGuard({ point: GuardPoint['pe.prompt.global'] });
  const { initPlaygroundLoading } = usePlayground();

  const { clearStore: clearBasicStore, setBasicReadonly } = useBasicStore(
    useShallow(state => ({
      clearStore: state.clearStore,
      setBasicReadonly: state.setReadonly,
    })),
  );

  useEffect(() => {
    setBasicReadonly(globalDisabled.data.readonly);

    return () => clearBasicStore();
  }, [globalDisabled.data.readonly]);

  return <PromptDev getPromptLoading={initPlaygroundLoading} />;
}
