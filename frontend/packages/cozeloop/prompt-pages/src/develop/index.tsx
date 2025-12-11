// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useParams, useSearchParams } from 'react-router-dom';
import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { useBreadcrumb } from '@cozeloop/hooks';
import { GuardPoint, useGuard } from '@cozeloop/guard';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { usePrompt } from '@/hooks/use-prompt';
import { PromptDev } from '@/components/prompt-dev';

export function PromptDevelop() {
  const globalDisabled = useGuard({ point: GuardPoint['pe.prompt.global'] });
  const { promptID } = useParams<{
    promptID: string;
  }>();
  const [searchParams] = useSearchParams();
  const queryVersion = searchParams.get('version') || undefined;
  const [getPromptLoading, setGetPromptLoading] = useState(true);

  const { getPromptByVersion } = usePrompt({ promptID, regiesterSub: true });
  const { clearStore: clearPromptStore, promptInfo } = usePromptStore(
    useShallow(state => ({
      clearStore: state.clearStore,
      promptInfo: state.promptInfo,
    })),
  );

  const { clearStore: clearBasicStore, setBasicReadonly } = useBasicStore(
    useShallow(state => ({
      clearStore: state.clearStore,
      setBasicReadonly: state.setReadonly,
    })),
  );

  const { clearMockdataStore } = usePromptMockDataStore(
    useShallow(state => ({
      clearMockdataStore: state.clearMockdataStore,
    })),
  );

  useBreadcrumb({
    text: promptInfo?.prompt_basic?.display_name || '',
  });

  useEffect(() => {
    if (promptID) {
      getPromptByVersion(queryVersion, true, globalDisabled.data.readonly).then(
        () => {
          setGetPromptLoading(false);
          setBasicReadonly(
            globalDisabled.data.readonly || Boolean(queryVersion),
          );
        },
      );
    }
    return () => {
      clearPromptStore();
      clearBasicStore();
      clearMockdataStore();
    };
  }, [promptID, queryVersion, globalDisabled.data.readonly]);

  return <PromptDev getPromptLoading={getPromptLoading} />;
}
