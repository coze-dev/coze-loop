// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';

import { usePromptStore } from '@/store/use-prompt-store';
import { usePromptMockDataStore } from '@/store/use-mockdata-store';
import { useBasicStore } from '@/store/use-basic-store';
import { usePrompt } from '@/hooks/use-prompt';

import { usePromptDevProviderContext } from './components/prompt-provider';
import { PromptLayout } from './components/prompt-layout';

export function DevelopContainer({
  wrapperClassName,
}: {
  wrapperClassName?: string;
}) {
  const { spaceID, promptID, readonly, queryVersion, onPromptLoaded } =
    usePromptDevProviderContext();
  const [getPromptLoading, setGetPromptLoading] = useState(true);
  const { promptByVersionService } = usePrompt({
    spaceID,
    promptID,
    regiesterSub: !readonly,
  });

  const { clearStore: clearPromptStore } = usePromptStore(
    useShallow(state => ({
      clearStore: state.clearStore,
      promptInfo: state.promptInfo,
    })),
  );

  const { clearBasicStore, setBasicReadonly } = useBasicStore(
    useShallow(state => ({
      clearBasicStore: state.clearBasicStore,
      setBasicReadonly: state.setReadonly,
    })),
  );

  const { clearMockdataStore } = usePromptMockDataStore(
    useShallow(state => ({
      clearMockdataStore: state.clearMockdataStore,
    })),
  );

  useEffect(() => {
    if (promptID) {
      promptByVersionService
        .runAsync({
          version: queryVersion,
          withCommit: true,
        })
        .then(res => {
          onPromptLoaded?.(res.prompt);
          setGetPromptLoading(false);
          setBasicReadonly(readonly || Boolean(queryVersion));
        })
        .catch(e => {
          console.error(e);
          setGetPromptLoading(false);
        });
    }
    return () => {
      clearPromptStore();
      clearBasicStore();
      clearMockdataStore();
    };
  }, [promptID, queryVersion, readonly]);

  return (
    <PromptLayout
      wrapperClassName={wrapperClassName}
      getPromptLoading={getPromptLoading}
    />
  );
}
