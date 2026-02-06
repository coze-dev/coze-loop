// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { Scenario } from '@cozeloop/api-schema/llm-manage';
import { LlmManageApi } from '@cozeloop/api-schema';

export const useModelList = (spaceID: string, scenario?: Scenario) => {
  const service = useRequest(() =>
    LlmManageApi.ListModels({
      workspace_id: spaceID,
      page_size: 100,
      page_token: '0',
      scenario: scenario || Scenario.scenario_prompt_debug,
    }),
  );
  return service;
};
