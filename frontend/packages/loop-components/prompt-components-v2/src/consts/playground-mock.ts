// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Role, ToolType, VariableType } from '@cozeloop/api-schema/prompt';

import { type PromptState } from '@/store/use-prompt-store';
import { type PromptMockDataState } from '@/store/use-mockdata-store';

export const mockMockSet: PromptMockDataState = {
  historicMessage: [],
};

export const mockInfo: PromptState = {
  modelConfig: {},
  tools: [
    {
      type: ToolType.Function,
      function: {
        name: 'get_weather',
        description: 'Determine weather in my location',
        parameters:
          '{"type":"object","properties":{"location":{"type":"string","description":"The city and state e.g. San Francisco, CA"},"unit":{"type":"string","enum":["c","f"]}},"required":["location"]}',
      },
    },
  ],

  variables: [
    {
      key: 'departure',
      type: VariableType.String,
      desc: '',
    },
    {
      desc: '',
      type: VariableType.String,
      key: 'destination',
    },
    {
      desc: '',
      type: VariableType.String,
      key: 'people_num',
    },
    {
      desc: '',
      type: VariableType.String,
      key: 'days_num',
    },
    {
      type: VariableType.String,
      key: 'travel_theme',

      desc: '',
    },
  ],

  promptInfo: {},
  messageList: [
    {
      key: '1',
      content: I18n.t('prompt_playground_mock_system'),
      role: Role.System,
    },
    {
      key: '2',
      content: I18n.t('prompt_playground_mock_user'),

      role: Role.User,
    },
  ],
};
