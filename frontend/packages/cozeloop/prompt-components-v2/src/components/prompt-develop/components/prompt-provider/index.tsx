// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createContext, useContext } from 'react';

import { type PromptDevelopProps } from '../../type';

interface PromptDevProviderContextType extends PromptDevelopProps {
  children?: React.ReactNode;
  groupNum: number;
  setGroupNum?: (num: number) => void;
}

export const PromptDevProviderContext =
  createContext<PromptDevProviderContextType>({
    spaceID: '',
    promptID: '',
    groupNum: 2,
  });

// Provider component
export function PromptDevProvider({
  children,
  ...rest
}: PromptDevProviderContextType) {
  return (
    <PromptDevProviderContext.Provider value={{ ...rest }}>
      {children}
    </PromptDevProviderContext.Provider>
  );
}

export function usePromptDevProviderContext() {
  return useContext(PromptDevProviderContext);
}
