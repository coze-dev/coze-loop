import { usePlayground } from '@/hooks/use-playground';

import { usePromptDevProviderContext } from './components/prompt-provider';
import { PromptLayout } from './components/prompt-layout';

export function PlaygroundContainer({
  wrapperClassName,
}: {
  wrapperClassName?: string;
}) {
  const { spaceID, readonly, promptID } = usePromptDevProviderContext();
  const { initPlaygroundLoading } = usePlayground({
    spaceID,
    promptID,
    useMockData: readonly,
  });
  return (
    <PromptLayout
      wrapperClassName={wrapperClassName}
      getPromptLoading={initPlaygroundLoading}
    />
  );
}
