import { createContext, useContext } from 'react';

import { type CreateExperimentValues } from '@/types/evaluate-target';

export const ExptCreateFormCtx = createContext<{
  nextStepLoading: boolean;
  setNextStepLoading?: (loading: boolean) => void;
  createExperimentValues?: CreateExperimentValues;
  setCreateExperimentValues?: React.Dispatch<
    React.SetStateAction<CreateExperimentValues>
  >;
}>({
  nextStepLoading: false,
  setNextStepLoading: undefined,
  createExperimentValues: undefined,
  setCreateExperimentValues: undefined,
});

export const useExptCreateFormCtx = () => useContext(ExptCreateFormCtx);
