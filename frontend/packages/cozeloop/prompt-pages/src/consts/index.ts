// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-magic-numbers */

import { Role, VariableType } from '@cozeloop/api-schema/prompt';

export const VARIABLE_MAX_LEN = 50;
export const UPDATED_DRAFT_CODE = 600503308;
export const CALL_SLEEP_TIME = 400;

export const MAX_FILE_SIZE_MB = 20;
export const MAX_FILE_SIZE = MAX_FILE_SIZE_MB * 1024;
export const MAX_IMAGE_FILE = 20;

export enum PromptStorageKey {
  PLAYGROUND_INFO = 'playground-info',
  PLAYGROUND_MOCKSET = 'playground-mockset',
}

export enum MessageType {
  System = 1,
  User = 2,
  Assistant = 3,
  Tool = 4,
  Placeholder = 20,
}

export const MESSAGE_TYPE_MAP = {
  [MessageType.System]: Role.System,
  [MessageType.User]: Role.User,
  [MessageType.Assistant]: Role.Assistant,
  [MessageType.Tool]: Role.Tool,
  [MessageType.Placeholder]: Role.Placeholder,
};

export enum MessageListRoundType {
  Multi = 'multi',
  Single = 'single',
}

export enum MessageListGroupType {
  Single = 'single',
  Multi = 'multi',
}

export const VARIABLE_TYPE_ARRAY_MAP = {
  [VariableType.String]: 'String',
  [VariableType.Integer]: 'Integer',
  [VariableType.Float]: 'Float',
  [VariableType.Boolean]: 'Boolean',
  [VariableType.Object]: 'Object',
  [VariableType.Array_String]: 'Array<String>',
  [VariableType.Array_Integer]: 'Array<Integer>',
  [VariableType.Array_Float]: 'Array<Float>',
  [VariableType.Array_Boolean]: 'Array<Boolean>',
  [VariableType.Array_Object]: 'Array<Object>',
  [VariableType.Placeholder]: 'Placeholder',
  [VariableType.MultiPart]: '多模态',
};

export const VARIABLE_TYPE_ARRAY_TAG = {
  [VariableType.String]: '1',
  [VariableType.Integer]: '2',
  [VariableType.Float]: '4',
  [VariableType.Boolean]: '3',
  [VariableType.Object]: '6',
  [VariableType.Array_String]: '99',
  [VariableType.Array_Integer]: '100',
  [VariableType.Array_Float]: '102',
  [VariableType.Array_Boolean]: '101',
  [VariableType.Array_Object]: '103',
  [VariableType.Placeholder]: 'Placeholder',
};
