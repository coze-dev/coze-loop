// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createContext, useContext, type ReactNode } from 'react';

import { type Experiment } from '@cozeloop/api-schema/evaluation';

export const ExperimentContext = createContext<Experiment | undefined>(
  undefined,
);

export function useExperiment() {
  const experiment = useContext<Experiment | undefined>(ExperimentContext);
  return experiment;
}

export function ExperimentContextProvider({
  experiment,
  children,
}: {
  experiment: Experiment | undefined;
  children: ReactNode;
}) {
  return (
    <ExperimentContext.Provider value={experiment}>
      {children}
    </ExperimentContext.Provider>
  );
}
