import { useState } from 'react';

import { type PromptDevelopProps } from './type';
import { PlaygroundContainer } from './playground';
import { DevelopContainer } from './develop';
import { PromptDevProvider } from './components/prompt-provider';

const DEFAULT_GROUP_NUM = 2;

export function PromptDevelop(props: PromptDevelopProps) {
  const { isPlayground, wrapperClassName, ...rest } = props;
  const [groupNum, setGroupNum] = useState(DEFAULT_GROUP_NUM);

  if (isPlayground) {
    return (
      <PromptDevProvider
        isPlayground={isPlayground}
        groupNum={groupNum}
        setGroupNum={setGroupNum}
        {...rest}
      >
        <PlaygroundContainer wrapperClassName={wrapperClassName} />
      </PromptDevProvider>
    );
  }

  return (
    <PromptDevProvider
      isPlayground={isPlayground}
      groupNum={groupNum}
      setGroupNum={setGroupNum}
      {...rest}
    >
      <DevelopContainer wrapperClassName={wrapperClassName} />
    </PromptDevProvider>
  );
}
