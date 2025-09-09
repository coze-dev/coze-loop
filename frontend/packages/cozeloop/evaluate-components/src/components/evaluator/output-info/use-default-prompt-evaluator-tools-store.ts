// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { create } from 'zustand';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

interface DefaultPromptEvaluatorToolsState {
  toolsDescription?: {
    score?: string;
    reason?: string;
  };
}

interface DefaultPromptEvaluatorToolsAction {
  fetchData: (force?: boolean) => void;
}

export const useDefaultPromptEvaluatorToolsStore = create<
  DefaultPromptEvaluatorToolsState & DefaultPromptEvaluatorToolsAction
>((set, get) => ({
  fetchData: async (force?: boolean) => {
    const { toolsDescription } = get();
    if (!toolsDescription || force) {
      const { tools } = await StoneEvaluationApi.GetDefaultPromptEvaluatorTools(
        {},
      );
      const parametersStr = tools?.[0]?.function?.parameters;
      if (parametersStr) {
        try {
          const parameters = JSON.parse(parametersStr);

          const score = parameters?.properties?.score?.description;
          const reason = parameters?.properties?.reason?.description;
          if (score || reason) {
            set({
              toolsDescription: {
                score,
                reason,
              },
            });
          }
        } catch (e) {
          console.warn('get default prompt evaluator tools error', e);
        }
      }
    }
  },
}));
