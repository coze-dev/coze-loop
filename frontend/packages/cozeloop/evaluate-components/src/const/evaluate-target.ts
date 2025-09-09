// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  ContentType,
  EvalTargetType,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';

export const evalTargetTypeMap = {
  [EvalTargetType.CozeBot]: I18n.t(
    'cozeloop_open_evaluate_coze_intelligent_agent',
  ),
  [EvalTargetType.CozeLoopPrompt]: 'Prompt',
};

export const evalTargetTypeOptions = [
  {
    label: evalTargetTypeMap[EvalTargetType.CozeBot],
    value: EvalTargetType.CozeBot,
  },
  {
    label: evalTargetTypeMap[EvalTargetType.CozeLoopPrompt],
    value: EvalTargetType.CozeLoopPrompt,
  },
];

export const COZE_BOT_INPUT_FIELD_NAME = 'input';
export const COMMON_OUTPUT_FIELD_NAME = 'actual_output';

export const DEFAULT_TEXT_STRING_SCHEMA: FieldSchema = {
  content_type: ContentType.Text,
  text_schema: '{"type": "string"}',
};

export const DEFAULT_MULTIPART_SCHEMA_OBJ = [
  {
    type: 'string',
    key: 'KaMFvtuHmC4upw3LnQlOX',
    propertyKey: 'type',
    children: [],
    additionalProperties: false,
    isRequired: true,
  },
  {
    type: 'string',
    key: '4w8PdWnCT7f-xS_lPx59m',
    propertyKey: 'text',
    children: [],
    additionalProperties: false,
    isRequired: false,
  },
  {
    type: 'object',
    key: '95uan6HCRtOCXj59ElrTL',
    propertyKey: 'image_url',
    children: [
      {
        type: 'string',
        key: '40Imj5mUX0QilRydFBeVQ',
        propertyKey: 'url',
        children: [],
        additionalProperties: false,
        isRequired: true,
      },
    ],
    additionalProperties: false,
    isRequired: false,
  },
];
export const DEFAULT_MULTIPART_SCHEMA = JSON.stringify(
  {
    type: 'object',
    properties: {
      type: {
        type: 'string',
        properties: {},
        additionalProperties: false,
      },
      text: {
        type: 'string',
        properties: {},
        additionalProperties: false,
      },
      image_url: {
        type: 'object',
        properties: {
          url: {
            type: 'string',
            properties: {},
            additionalProperties: false,
          },
        },
        additionalProperties: false,
      },
    },
    additionalProperties: false,
    required: ['type'],
  },
  null,
  2,
);
